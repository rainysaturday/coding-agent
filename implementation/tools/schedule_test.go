// Package tools implements the tool execution system for the coding agent.
// This file contains tests for the schedule store and schedule tool
// (requirement 048).
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseDurationLoose(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"90s", 90 * time.Second},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"1h", time.Hour},
		{"1.5h", 90 * time.Minute},
		{"10 minutes", 10 * time.Minute},
		{"2 hours", 2 * time.Hour},
		{"45 sec", 45 * time.Second},
		{"3d", 72 * time.Hour},
		{"1w", 168 * time.Hour},
		{"500ms", 500 * time.Millisecond},
		{"  10M  ", 10 * time.Minute},
	}

	for _, tc := range cases {
		got, err := parseDurationLoose(tc.in)
		if err != nil {
			t.Errorf("parseDurationLoose(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDurationLoose(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestParseDurationLooseInvalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"10",           // no unit
		"abc",          // not a number
		"soon",         // not a duration
		"-5m",          // negative
		"3 months",     // unsupported unit
		"10 minutes x", // junk
		"m",            // no number
	}

	for _, in := range cases {
		if d, err := parseDurationLoose(in); err == nil {
			t.Errorf("parseDurationLoose(%q) expected an error, got %s", in, d)
		}
	}
}

func TestParseScheduleTime(t *testing.T) {
	got, err := parseScheduleTime("2026-05-13T09:30:00Z")
	if err != nil {
		t.Fatalf("RFC3339 parse failed: %v", err)
	}
	if got.Year() != 2026 || got.Month() != time.May || got.Day() != 13 {
		t.Errorf("unexpected RFC3339 result: %s", got)
	}

	got, err = parseScheduleTime("2026-05-13 09:30")
	if err != nil {
		t.Fatalf("date-time parse failed: %v", err)
	}
	if got.Hour() != 9 || got.Minute() != 30 {
		t.Errorf("unexpected time component: %s", got)
	}

	got, err = parseScheduleTime("2026-05-13")
	if err != nil {
		t.Fatalf("date parse failed: %v", err)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("date-only should be midnight, got %s", got)
	}
}

func TestParseScheduleTimeClockRollover(t *testing.T) {
	now := time.Now()
	// A clock time far in the future relative to "now" stays today; one in the
	// past rolls over to tomorrow.
	target := now.Add(3 * time.Hour)
	if target.Day() == now.Day() {
		spec := target.Format("15:04")
		got, err := parseScheduleTime(spec)
		if err != nil {
			t.Fatalf("clock parse failed: %v", err)
		}
		if got.Day() != now.Day() {
			t.Errorf("clock time %q should resolve to today, got %s", spec, got)
		}
	}

	past := now.Add(-3 * time.Hour)
	if past.Day() != now.Day() {
		t.Skip("cannot construct a same-day past clock time right now")
	}
	spec := past.Format("15:04")
	got, err := parseScheduleTime(spec)
	if err != nil {
		t.Fatalf("clock parse failed: %v", err)
	}
	if !got.After(now) {
		t.Errorf("clock time %q should roll over to tomorrow (%s), got %s", spec, now.AddDate(0, 0, 1), got)
	}
}

func TestParseScheduleTimeInvalid(t *testing.T) {
	for _, in := range []string{"", "not-a-time", "13:70", "2026-13-01"} {
		if _, err := parseScheduleTime(in); err == nil {
			t.Errorf("parseScheduleTime(%q) expected an error", in)
		}
	}
}

func TestScheduleStoreAddAndList(t *testing.T) {
	store := NewScheduleStore()

	once := store.Add(&ScheduleEntry{Message: "ping", Kind: ScheduleKindOnce, Next: time.Now().Add(time.Minute)})
	cron, _ := ParseCron("*/5 * * * *")
	recurring := store.Add(&ScheduleEntry{Message: "tick", Kind: ScheduleKindCron, Cron: cron, Next: cron.NextAfter(time.Now())})

	if once.ID != 1 || recurring.ID != 2 {
		t.Errorf("expected sequential IDs, got %d and %d", once.ID, recurring.ID)
	}
	if once.Limit != 1 {
		t.Errorf("one-shot entries should have limit 1, got %d", once.Limit)
	}
	if got := store.Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
	if got := store.CountActive(); got != 2 {
		t.Errorf("CountActive() = %d, want 2", got)
	}
	if len(store.List()) != 2 {
		t.Fatalf("List() should return 2 entries")
	}

	// List must return copies: mutating one must not affect the store.
	listed := store.List()
	listed[0].Message = "mutated"
	if store.Get(once.ID).Message != "ping" {
		t.Errorf("List() leaked the stored entry")
	}
}

func TestScheduleStoreDueOnceRetires(t *testing.T) {
	store := NewScheduleStore()
	store.Add(&ScheduleEntry{Message: "ping", Kind: ScheduleKindOnce, Next: time.Now().Add(-time.Second)})

	due := store.Due(time.Now())
	if len(due) != 1 {
		t.Fatalf("expected 1 due entry, got %d", len(due))
	}
	if !due[0].Done || due[0].FireCount != 1 {
		t.Errorf("fired one-shot entry should be done with 1 fire, got done=%v fires=%d", due[0].Done, due[0].FireCount)
	}
	if store.Count() != 0 {
		t.Errorf("fired one-shot entry should be removed from the store")
	}
	if again := store.Due(time.Now()); len(again) != 0 {
		t.Errorf("a one-shot entry must never fire twice")
	}
}

func TestScheduleStoreDueCronRearms(t *testing.T) {
	store := NewScheduleStore()
	cron, err := ParseCron("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	store.Add(&ScheduleEntry{Message: "tick", Kind: ScheduleKindCron, Cron: cron, Next: now})

	due := store.Due(now)
	if len(due) != 1 {
		t.Fatalf("expected 1 due entry, got %d", len(due))
	}
	if store.Count() != 1 {
		t.Fatalf("recurring entry should stay in the store")
	}
	stored := store.List()[0]
	if !stored.Next.Equal(now.Add(5 * time.Minute)) {
		t.Errorf("expected re-arm to 10:05, got %s", stored.Next)
	}

	// The snapshot handed to the caller keeps the delivery bookkeeping.
	if due[0].FireCount != 1 || due[0].LastFired.IsZero() {
		t.Errorf("fired snapshot should carry fire bookkeeping")
	}
}

func TestScheduleStoreDueEveryDriftFree(t *testing.T) {
	store := NewScheduleStore()
	interval := time.Minute
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	store.Add(&ScheduleEntry{Message: "tick", Kind: ScheduleKindEvery, Interval: interval, Next: now})

	// A late collection (5 minutes past the scheduled instant) still advances by
	// whole intervals rather than restarting from "now".
	store.Due(now.Add(5 * time.Minute))
	stored := store.List()[0]
	if !stored.Next.Equal(now.Add(6 * time.Minute)) {
		t.Errorf("expected next at +6m, got %s", stored.Next)
	}
}

func TestScheduleStoreDueLimit(t *testing.T) {
	store := NewScheduleStore()
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	store.Add(&ScheduleEntry{
		Message:  "twice",
		Kind:     ScheduleKindEvery,
		Interval: time.Minute,
		Next:     now,
		Limit:    2,
	})

	if due := store.Due(now); len(due) != 1 {
		t.Fatalf("expected first delivery")
	}
	if store.Count() != 1 {
		t.Fatalf("entry with limit 2 should still be pending after 1 fire")
	}
	if due := store.Due(now.Add(time.Minute)); len(due) != 1 {
		t.Fatalf("expected second delivery")
	}
	if store.Count() != 0 {
		t.Errorf("entry should be retired after reaching its limit")
	}
}

func TestScheduleStorePausedNotDue(t *testing.T) {
	store := NewScheduleStore()
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	entry := store.Add(&ScheduleEntry{Message: "hold", Kind: ScheduleKindEvery, Interval: time.Minute, Next: now})

	if got := store.SetPaused(entry.ID, true); got == nil || !got.Paused {
		t.Fatalf("pause failed")
	}
	if due := store.Due(now.Add(time.Hour)); len(due) != 0 {
		t.Errorf("paused entries must not fire")
	}
	if store.CountActive() != 0 {
		t.Errorf("paused entries are not active")
	}

	resumed := store.SetPaused(entry.ID, false)
	if resumed == nil || resumed.Paused {
		t.Fatalf("resume failed")
	}
	if !resumed.Next.After(now.Add(time.Hour)) {
		t.Errorf("resuming a stale entry should re-arm it from now, got %s", resumed.Next)
	}
	if due := store.Due(time.Now().Add(time.Hour)); len(due) != 1 {
		t.Errorf("resumed entry should fire again")
	}
}

func TestScheduleStoreRemoveAndClear(t *testing.T) {
	store := NewScheduleStore()
	a := store.Add(&ScheduleEntry{Message: "a", Kind: ScheduleKindOnce, Next: time.Now()})
	store.Add(&ScheduleEntry{Message: "b", Kind: ScheduleKindOnce, Next: time.Now()})

	removed := store.Remove(a.ID)
	if removed == nil || removed.Message != "a" {
		t.Fatalf("Remove did not return the removed entry")
	}
	if store.Remove(a.ID) != nil {
		t.Errorf("removing a missing entry should return nil")
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 entry left")
	}
	if n := store.Clear(); n != 1 {
		t.Errorf("Clear() returned %d, want 1", n)
	}
	if store.Count() != 0 {
		t.Errorf("Clear should empty the store")
	}
}

func TestScheduleStoreNextDueTime(t *testing.T) {
	store := NewScheduleStore()
	if _, ok := store.NextDueTime(time.Now()); ok {
		t.Errorf("empty store should report no pending time")
	}
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	store.Add(&ScheduleEntry{Message: "late", Kind: ScheduleKindOnce, Next: now.Add(time.Hour)})
	store.Add(&ScheduleEntry{Message: "soon", Kind: ScheduleKindOnce, Next: now.Add(time.Minute)})
	store.Add(&ScheduleEntry{Message: "paused", Kind: ScheduleKindOnce, Next: now, Paused: true})

	next, ok := store.NextDueTime(now)
	if !ok || !next.Equal(now.Add(time.Minute)) {
		t.Errorf("expected the earliest active entry, got %s (ok=%v)", next, ok)
	}
}

func TestScheduleStoreChangeNotifier(t *testing.T) {
	store := NewScheduleStore()
	calls := 0
	store.SetChangeNotifier(func() { calls++ })

	entry := store.Add(&ScheduleEntry{Message: "x", Kind: ScheduleKindEvery, Interval: time.Minute, Next: time.Now()})
	store.SetPaused(entry.ID, true)
	store.SetPaused(entry.ID, false)
	store.Remove(entry.ID)
	if calls != 4 {
		t.Errorf("expected 4 change notifications, got %d", calls)
	}

	// Collecting when nothing is due must not notify.
	store.Due(time.Now().Add(time.Hour))
	if calls != 4 {
		t.Errorf("a no-op Due() must not notify, got %d calls", calls)
	}
}

// TestScheduleStoreNotifierMayReadStore guards against a deadlock: the change
// notifier runs while the store is mutated, so it must be invoked outside the
// store mutex. The agent's notifier reads the store (Count), which would
// otherwise block forever on the non-reentrant mutex.
func TestScheduleStoreNotifierMayReadStore(t *testing.T) {
	store := NewScheduleStore()
	var counts []int
	store.SetChangeNotifier(func() {
		counts = append(counts, store.Count())
	})

	done := make(chan struct{})
	go func() {
		entry := store.Add(&ScheduleEntry{Message: "x", Kind: ScheduleKindEvery, Interval: time.Minute, Next: time.Now()})
		store.SetPaused(entry.ID, true)
		store.SetPaused(entry.ID, false)
		store.SetNext(entry.ID, time.Now().Add(time.Hour))
		store.Due(time.Now().Add(2 * time.Hour))
		store.Remove(entry.ID)
		store.Add(&ScheduleEntry{Message: "y", Kind: ScheduleKindOnce, Next: time.Now()})
		store.Clear()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the change notifier deadlocked while reading the store")
	}
	if len(counts) == 0 {
		t.Fatal("expected change notifications")
	}
}

// scheduleCall is a test helper that runs the schedule tool through the
// executor, mirroring how the agent invokes it.
func scheduleCall(t *testing.T, te *ToolExecutor, args map[string]interface{}) *ToolResult {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"id":   "call-1",
		"type": "function",
		"function": map[string]interface{}{
			"name":      "schedule",
			"arguments": mustJSON(t, args),
		},
	})
	if err != nil {
		t.Fatalf("marshal tool call: %v", err)
	}
	tc, err := ParseToolCall(string(raw))
	if err != nil {
		t.Fatalf("parse tool call: %v", err)
	}
	return te.Execute(context.Background(), tc)
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func TestExecuteScheduleAddIn(t *testing.T) {
	te := NewToolExecutor()

	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "check the build",
		"in":      "10m",
		"name":    "build-check",
	})
	if !res.Success {
		t.Fatalf("add failed: %s", res.Error)
	}
	if !strings.Contains(res.Output, "Scheduled #1") || !strings.Contains(res.Output, "check the build") {
		t.Errorf("unexpected output: %s", res.Output)
	}
	if res.Extra["id"] != 1 {
		t.Errorf("expected id in Extra, got %v", res.Extra["id"])
	}

	entry := te.GetScheduleStore().Get(1)
	if entry == nil {
		t.Fatalf("entry not stored")
	}
	if entry.Kind != ScheduleKindOnce {
		t.Errorf("kind = %s, want once", entry.Kind)
	}
	until := time.Until(entry.Next)
	if until < 9*time.Minute || until > 11*time.Minute {
		t.Errorf("next fire time is not ~10 minutes away: %s", until)
	}
	if entry.Name != "build-check" {
		t.Errorf("name not stored: %q", entry.Name)
	}
}

func TestExecuteScheduleAddCronAndEvery(t *testing.T) {
	te := NewToolExecutor()

	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "standup reminder",
		"cron":    "0 9 * * 1-5",
	})
	if !res.Success {
		t.Fatalf("cron add failed: %s", res.Error)
	}
	entry := te.GetScheduleStore().Get(1)
	if entry.Kind != ScheduleKindCron || entry.Cron == nil {
		t.Fatalf("expected a cron entry, got %+v", entry)
	}
	if entry.Next.Weekday() > time.Friday {
		t.Errorf("cron entry next fire should be a weekday, got %s", entry.Next)
	}

	res = scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "poll CI",
		"every":   "30m",
		"limit":   float64(3),
	})
	if !res.Success {
		t.Fatalf("every add failed: %s", res.Error)
	}
	entry = te.GetScheduleStore().Get(2)
	if entry.Kind != ScheduleKindEvery || entry.Interval != 30*time.Minute || entry.Limit != 3 {
		t.Errorf("unexpected every entry: %+v", entry)
	}
}

func TestExecuteScheduleAddAt(t *testing.T) {
	te := NewToolExecutor()

	future := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04")
	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "deploy",
		"at":      future,
	})
	if !res.Success {
		t.Fatalf("at add failed: %s", res.Error)
	}

	res = scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "past",
		"at":      time.Now().Add(-time.Hour).Format("2006-01-02 15:04"),
	})
	if res.Success {
		t.Errorf("scheduling in the past should fail")
	}
}

func TestExecuteScheduleAddValidation(t *testing.T) {
	te := NewToolExecutor()
	store := te.GetScheduleStore()

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"no timing", map[string]interface{}{"action": "add", "message": "hi"}},
		{"two timings", map[string]interface{}{"action": "add", "message": "hi", "in": "5m", "cron": "* * * * *"}},
		{"empty message", map[string]interface{}{"action": "add", "message": "  ", "in": "5m"}},
		{"missing message", map[string]interface{}{"action": "add", "in": "5m"}},
		{"bad duration", map[string]interface{}{"action": "add", "message": "hi", "in": "soon"}},
		{"bad cron", map[string]interface{}{"action": "add", "message": "hi", "cron": "99 * * * *"}},
		{"impossible cron", map[string]interface{}{"action": "add", "message": "hi", "cron": "0 0 30 2 *"}},
	}

	for _, tc := range cases {
		res := scheduleCall(t, te, tc.args)
		if res.Success {
			t.Errorf("%s: expected failure, got output %q", tc.name, res.Output)
		}
	}

	if store.Count() != 0 {
		t.Errorf("failed adds must not create entries, found %d", store.Count())
	}
}

func TestExecuteScheduleAddMinimumDelay(t *testing.T) {
	te := NewToolExecutor()
	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "immediate",
		"in":      "1ms",
	})
	if !res.Success {
		t.Fatalf("add failed: %s", res.Error)
	}
	entry := te.GetScheduleStore().Get(1)
	if until := time.Until(entry.Next); until < 0 {
		t.Errorf("next fire time should not be in the past: %s", until)
	}
}

func TestExecuteScheduleListRemovePauseResumeClear(t *testing.T) {
	te := NewToolExecutor()

	if res := scheduleCall(t, te, map[string]interface{}{"action": "list"}); !res.Success {
		t.Fatalf("list failed: %s", res.Error)
	} else if !strings.Contains(res.Output, "no scheduled messages") {
		t.Errorf("empty list output = %q", res.Output)
	}

	scheduleCall(t, te, map[string]interface{}{"action": "add", "message": "one", "in": "5m"})
	scheduleCall(t, te, map[string]interface{}{"action": "add", "message": "two", "every": "10m"})

	res := scheduleCall(t, te, map[string]interface{}{"action": "list"})
	if !strings.Contains(res.Output, "one") || !strings.Contains(res.Output, "two") {
		t.Errorf("list output missing entries: %s", res.Output)
	}
	if res.Extra["totalSchedules"] != 2 {
		t.Errorf("expected totalSchedules=2, got %v", res.Extra["totalSchedules"])
	}

	if res := scheduleCall(t, te, map[string]interface{}{"action": "pause", "id": float64(1)}); !res.Success {
		t.Fatalf("pause failed: %s", res.Error)
	}
	if !te.GetScheduleStore().Get(1).Paused {
		t.Errorf("entry should be paused")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "resume", "id": float64(1)}); !res.Success {
		t.Fatalf("resume failed: %s", res.Error)
	}
	if te.GetScheduleStore().Get(1).Paused {
		t.Errorf("entry should be resumed")
	}

	if res := scheduleCall(t, te, map[string]interface{}{"action": "remove", "id": float64(2)}); !res.Success {
		t.Fatalf("remove failed: %s", res.Error)
	}
	if te.GetScheduleStore().Count() != 1 {
		t.Errorf("entry not removed")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "remove", "id": float64(99)}); res.Success {
		t.Errorf("removing an unknown id should fail")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "remove"}); res.Success {
		t.Errorf("remove without id should fail")
	}

	if res := scheduleCall(t, te, map[string]interface{}{"action": "clear"}); !res.Success {
		t.Fatalf("clear failed: %s", res.Error)
	}
	if te.GetScheduleStore().Count() != 0 {
		t.Errorf("clear did not empty the store")
	}
}

func TestExecuteScheduleInvalidAction(t *testing.T) {
	te := NewToolExecutor()
	if res := scheduleCall(t, te, map[string]interface{}{"action": "frobnicate"}); res.Success {
		t.Errorf("unknown action should fail")
	}
	if res := scheduleCall(t, te, map[string]interface{}{}); res.Success {
		t.Errorf("missing action should fail")
	}
}

func TestExecuteScheduleReadOnly(t *testing.T) {
	te := NewToolExecutor()
	// Seed an entry before switching to read-only so remove/clear have
	// something to act on.
	te.GetScheduleStore().Add(&ScheduleEntry{Message: "seed", Kind: ScheduleKindEvery, Interval: time.Minute, Next: time.Now().Add(time.Minute)})
	te.SetReadOnly(true)

	if res := scheduleCall(t, te, map[string]interface{}{"action": "add", "message": "hi", "in": "5m"}); res.Success {
		t.Errorf("add must be blocked in read-only mode")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "pause", "id": float64(1)}); res.Success {
		t.Errorf("pause must be blocked in read-only mode")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "resume", "id": float64(1)}); res.Success {
		t.Errorf("resume must be blocked in read-only mode")
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "list"}); !res.Success {
		t.Errorf("list must stay available in read-only mode: %s", res.Error)
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "remove", "id": float64(1)}); !res.Success {
		t.Errorf("remove must stay available in read-only mode: %s", res.Error)
	}
	if res := scheduleCall(t, te, map[string]interface{}{"action": "clear"}); !res.Success {
		t.Errorf("clear must stay available in read-only mode: %s", res.Error)
	}
}

func TestFormatScheduleList(t *testing.T) {
	if got := FormatScheduleList(nil); !strings.Contains(got, "no scheduled messages") {
		t.Errorf("empty list formatting = %q", got)
	}

	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.Local)
	cron, _ := ParseCron("*/5 * * * *")
	lines := FormatScheduleList([]*ScheduleEntry{
		{ID: 1, Message: "once", Kind: ScheduleKindOnce, Next: now.Add(time.Minute)},
		{ID: 2, Message: "cron", Kind: ScheduleKindCron, Cron: cron, Next: now.Add(time.Hour), FireCount: 3, Limit: 10, Name: "tick"},
		{ID: 3, Message: "paused", Kind: ScheduleKindEvery, Interval: time.Hour, Next: now, Paused: true},
	})

	for _, want := range []string{"#1 [active]", "once at 2026-05-13 10:01:00", "cron \"*/5 * * * *\"", "3/10", "name=\"tick\"", "#3 [paused]", "every 1h"} {
		if !strings.Contains(lines, want) {
			t.Errorf("formatted list missing %q:\n%s", want, lines)
		}
	}
}

func TestFormatDurationLoose(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{24 * time.Hour, "1d"},
		{90 * time.Minute, "1h30m"},
		{2 * time.Hour, "2h"},
		{45 * time.Minute, "45m"},
		{90 * time.Second, "1m30s"},
		{25 * time.Hour, "1d1h"},
		{500 * time.Millisecond, "500ms"},
	}
	for _, tc := range cases {
		if got := formatDurationLoose(tc.in); got != tc.want {
			t.Errorf("formatDurationLoose(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseScheduleArgs(t *testing.T) {
	cases := []struct {
		line string
		want map[string]interface{}
	}{
		{"", map[string]interface{}{"action": "list"}},
		{"list", map[string]interface{}{"action": "list"}},
		{"clear", map[string]interface{}{"action": "clear"}},
		{"remove 3", map[string]interface{}{"action": "remove", "id": 3}},
		{"pause 2", map[string]interface{}{"action": "pause", "id": 2}},
		{"resume --id 7", map[string]interface{}{"action": "resume", "id": 7}},
		{
			"add in 10m Check the build",
			map[string]interface{}{"action": "add", "in": "10m", "message": "Check the build"},
		},
		{
			"add every 30m Stand up",
			map[string]interface{}{"action": "add", "every": "30m", "message": "Stand up"},
		},
		{
			`add cron "*/5 * * * *" Check the build`,
			map[string]interface{}{"action": "add", "cron": "*/5 * * * *", "message": "Check the build"},
		},
		{
			`add at "2026-05-01 09:00" Deploy the release`,
			map[string]interface{}{"action": "add", "at": "2026-05-01 09:00", "message": "Deploy the release"},
		},
		{
			"add --name standup --limit 3 every 1h Review the backlog",
			map[string]interface{}{"action": "add", "name": "standup", "limit": 3, "every": "1h", "message": "Review the backlog"},
		},
		{
			"add in 10m every morning check CI at nine",
			map[string]interface{}{"action": "add", "in": "10m", "message": "every morning check CI at nine"},
		},
	}

	for _, tc := range cases {
		got, err := ParseScheduleArgs(tc.line)
		if err != nil {
			t.Errorf("ParseScheduleArgs(%q) returned error: %v", tc.line, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseScheduleArgs(%q) = %#v, want %#v", tc.line, got, tc.want)
			continue
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("ParseScheduleArgs(%q)[%q] = %v, want %v", tc.line, k, got[k], v)
			}
		}
	}
}

func TestParseScheduleArgsInvalid(t *testing.T) {
	cases := []string{
		"frobnicate",
		`add "unterminated`,
		"add in 10m",                 // no message
		"add",                        // no timing, no message
		"remove",                     // no id
		"remove abc",                 // id not a number
		"list extra",                 // unexpected argument
		"add in",                     // missing value
		"add --bogus 5m message",     // unknown flag
		"add limit x in 10m message", // limit not a number
		"remove 1 2",                 // extra positional
	}

	for _, line := range cases {
		if params, err := ParseScheduleArgs(line); err == nil {
			t.Errorf("ParseScheduleArgs(%q) expected an error, got %#v", line, params)
		}
	}
}

// TestExecuteScheduleAddReturnsImmediately locks in the requirement that a
// schedule call only registers the entry: it must return at once instead of
// waiting for (or blocking on) the future delivery.
func TestExecuteScheduleAddReturnsImmediately(t *testing.T) {
	te := NewToolExecutor()

	start := time.Now()
	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "check the build in ten minutes",
		"in":      "10m",
	})
	elapsed := time.Since(start)

	if !res.Success {
		t.Fatalf("add failed: %s", res.Error)
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("add must return immediately, took %s", elapsed)
	}

	entry := te.GetScheduleStore().Get(1)
	if entry == nil {
		t.Fatal("expected the schedule to be registered")
	}
	if until := time.Until(entry.Next); until < 9*time.Minute || until > 10*time.Minute+time.Second {
		t.Errorf("expected the entry to fire in ~10 minutes, got %s", until)
	}
	if res.Extra["id"] != 1 {
		t.Errorf("expected the new id in the result extras, got %v", res.Extra["id"])
	}
}

// Regression: an apostrophe inside an unquoted token must stay a literal;
// "don't" previously failed with "unterminated quote".
func TestParseScheduleArgsApostropheInMessage(t *testing.T) {
	got, err := ParseScheduleArgs("add in 5m don't forget the milk")
	if err != nil {
		t.Fatalf("apostrophe in message: %v", err)
	}
	if got["message"] != "don't forget the milk" {
		t.Errorf("message = %v, want %q", got["message"], "don't forget the milk")
	}
}

// Regression: a limit on a one-shot schedule must be rejected with a clear
// error instead of being silently forced to 1.
func TestExecuteScheduleAddRejectsLimitOnOneShot(t *testing.T) {
	te := NewToolExecutor()
	res := scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "ping",
		"in":      "10m",
		"limit":   3,
	})
	if res.Success {
		t.Fatal("expected limit with a one-shot schedule to be rejected")
	}
	if n := te.GetScheduleStore().Count(); n != 0 {
		t.Errorf("rejected add must not register an entry, got %d", n)
	}

	// limit == 1 matches one-shot semantics and stays acceptable.
	res = scheduleCall(t, te, map[string]interface{}{
		"action":  "add",
		"message": "ping",
		"in":      "10m",
		"limit":   1,
	})
	if !res.Success {
		t.Fatalf("limit 1 with a one-shot should be accepted: %s", res.Error)
	}
}
