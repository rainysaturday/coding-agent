// Package tools implements the tool execution system for the coding agent.
// This file contains the schedule store and the schedule ("schedule ping") tool
// executor (requirement 048).
package tools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Schedule kinds.
const (
	// ScheduleKindOnce fires a single time at an absolute instant.
	ScheduleKindOnce = "once"
	// ScheduleKindCron fires repeatedly according to a cron expression.
	ScheduleKindCron = "cron"
	// ScheduleKindEvery fires repeatedly at a fixed interval.
	ScheduleKindEvery = "every"
)

// scheduleTimeLayout is the layout used for rendering schedule times.
const scheduleTimeLayout = "2006-01-02 15:04:05"

// minScheduleDelay is the smallest delay accepted for a scheduled message. It
// keeps a mistyped "in 0s" from turning the scheduler into a busy loop.
const minScheduleDelay = time.Second

// ScheduleEntry is a single scheduled message.
type ScheduleEntry struct {
	ID        int
	Name      string
	Message   string
	Kind      string
	Spec      string // the timing spec as provided by the caller
	Cron      *CronSchedule
	Interval  time.Duration
	Created   time.Time
	Next      time.Time
	LastFired time.Time
	FireCount int
	Limit     int // maximum deliveries; 0 means unlimited (one-shot entries are 1)
	Paused    bool
	Done      bool
}

// Active reports whether the entry can still fire.
func (e *ScheduleEntry) Active() bool {
	return !e.Paused && !e.Done
}

// Status returns a short human-readable lifecycle state.
func (e *ScheduleEntry) Status() string {
	switch {
	case e.Done:
		return "done"
	case e.Paused:
		return "paused"
	default:
		return "active"
	}
}

// Describe renders the timing rule, e.g. `cron "*/5 * * * *"`.
func (e *ScheduleEntry) Describe() string {
	switch e.Kind {
	case ScheduleKindOnce:
		if e.Next.IsZero() {
			return fmt.Sprintf("once at %s", e.Spec)
		}
		return fmt.Sprintf("once at %s", e.Next.Format(scheduleTimeLayout))
	case ScheduleKindCron:
		return fmt.Sprintf("cron %q", e.Cron.Expr())
	case ScheduleKindEvery:
		return fmt.Sprintf("every %s", formatDurationLoose(e.Interval))
	default:
		return e.Spec
	}
}

// rearm computes the next fire time after now for a recurring entry. It returns
// the zero time when the entry cannot fire again.
func (e *ScheduleEntry) rearm(now time.Time) time.Time {
	switch e.Kind {
	case ScheduleKindCron:
		if e.Cron == nil {
			return time.Time{}
		}
		return e.Cron.NextAfter(now)
	case ScheduleKindEvery:
		if e.Interval <= 0 {
			return time.Time{}
		}
		// Advance from the previous scheduled instant so long-running
		// deliveries do not drift the interval, catching up if needed.
		next := e.Next
		for !next.After(now) {
			next = next.Add(e.Interval)
		}
		return next
	default:
		return time.Time{}
	}
}

// ScheduleStore holds the session's scheduled messages. It is owned by the
// ToolExecutor and shared with the agent's scheduler goroutine, so every method
// is safe for concurrent use.
type ScheduleStore struct {
	mu       sync.Mutex
	entries  []*ScheduleEntry
	nextID   int
	onChange func() // invoked after any mutation so the scheduler can re-arm
}

// NewScheduleStore creates an empty store.
func NewScheduleStore() *ScheduleStore {
	return &ScheduleStore{
		entries: make([]*ScheduleEntry, 0),
		nextID:  1,
	}
}

// SetChangeNotifier registers a callback invoked after every mutation. The
// callback must not block: it typically only signals the scheduler.
func (ss *ScheduleStore) SetChangeNotifier(fn func()) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.onChange = fn
}

// notify invokes the change notifier. It must be called WITHOUT holding ss.mu:
// notifiers routinely read the store (e.g. Count), which would deadlock on the
// non-reentrant mutex.
func (ss *ScheduleStore) notify() {
	ss.mu.Lock()
	fn := ss.onChange
	ss.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Add inserts a fully prepared entry and assigns its ID. It returns a copy so
// callers cannot mutate the stored entry afterwards.
func (ss *ScheduleStore) Add(entry *ScheduleEntry) *ScheduleEntry {
	ss.mu.Lock()
	entry.ID = ss.nextID
	ss.nextID++
	if entry.Created.IsZero() {
		entry.Created = time.Now()
	}
	if entry.Kind == ScheduleKindOnce {
		entry.Limit = 1
	}
	ss.entries = append(ss.entries, entry)
	cp := *entry
	ss.mu.Unlock()

	ss.notify()
	return &cp
}

// Get returns a copy of the entry with the given ID, or nil.
func (ss *ScheduleStore) Get(id int) *ScheduleEntry {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, e := range ss.entries {
		if e.ID == id {
			cp := *e
			return &cp
		}
	}
	return nil
}

// Remove deletes the entry with the given ID and returns a copy of it, or nil.
func (ss *ScheduleStore) Remove(id int) *ScheduleEntry {
	ss.mu.Lock()
	var cp *ScheduleEntry
	for i, e := range ss.entries {
		if e.ID == id {
			ss.entries = append(ss.entries[:i], ss.entries[i+1:]...)
			snapshot := *e
			cp = &snapshot
			break
		}
	}
	ss.mu.Unlock()

	if cp == nil {
		return nil
	}
	ss.notify()
	return cp
}

// SetPaused pauses or resumes the entry with the given ID and returns a copy,
// or nil if it does not exist. Resuming an entry whose next fire time has
// already passed re-arms it immediately (one-shot) or from now (recurring).
func (ss *ScheduleStore) SetPaused(id int, paused bool) *ScheduleEntry {
	ss.mu.Lock()
	now := time.Now()
	var cp *ScheduleEntry
	for _, e := range ss.entries {
		if e.ID != id {
			continue
		}
		e.Paused = paused
		if !paused && (e.Next.IsZero() || !e.Next.After(now)) {
			if e.Kind == ScheduleKindOnce {
				e.Next = now
			} else if next := e.rearm(now); !next.IsZero() {
				e.Next = next
			}
		}
		snapshot := *e
		cp = &snapshot
		break
	}
	ss.mu.Unlock()

	if cp == nil {
		return nil
	}
	ss.notify()
	return cp
}

// SetNext overrides the next fire time of an entry. It exists so tests (and
// future persistence code) can restore or fast-forward a schedule.
func (ss *ScheduleStore) SetNext(id int, next time.Time) bool {
	ss.mu.Lock()
	found := false
	for _, entry := range ss.entries {
		if entry.ID == id {
			entry.Next = next
			found = true
			break
		}
	}
	ss.mu.Unlock()

	if !found {
		return false
	}
	ss.notify()
	return true
}

// List returns copies of all entries ordered by ID.
func (ss *ScheduleStore) List() []*ScheduleEntry {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	out := make([]*ScheduleEntry, 0, len(ss.entries))
	for _, e := range ss.entries {
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// Count returns the total number of stored entries.
func (ss *ScheduleStore) Count() int {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return len(ss.entries)
}

// CountActive returns the number of entries that can still fire.
func (ss *ScheduleStore) CountActive() int {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	n := 0
	for _, e := range ss.entries {
		if e.Active() {
			n++
		}
	}
	return n
}

// Clear removes every entry and returns how many were removed.
func (ss *ScheduleStore) Clear() int {
	ss.mu.Lock()
	n := len(ss.entries)
	ss.entries = make([]*ScheduleEntry, 0)
	ss.mu.Unlock()

	if n > 0 {
		ss.notify()
	}
	return n
}

// Due atomically collects the entries that are due at or before now, re-arms
// recurring ones and retires one-shot (or exhausted) ones. The returned entries
// are snapshots carrying the delivery bookkeeping (LastFired, FireCount), so a
// message can never be delivered twice.
func (ss *ScheduleStore) Due(now time.Time) []*ScheduleEntry {
	ss.mu.Lock()

	var fired []*ScheduleEntry
	kept := ss.entries[:0]
	changed := false

	for _, e := range ss.entries {
		if !e.Active() || e.Next.IsZero() || e.Next.After(now) {
			kept = append(kept, e)
			continue
		}

		changed = true
		e.LastFired = now
		e.FireCount++

		switch {
		case e.Kind == ScheduleKindOnce:
			e.Done = true
		case e.Limit > 0 && e.FireCount >= e.Limit:
			e.Done = true
		}

		if !e.Done {
			if next := e.rearm(now); next.IsZero() {
				e.Done = true
			} else {
				e.Next = next
			}
		}

		snapshot := *e
		fired = append(fired, &snapshot)

		if e.Done {
			continue // retire: drop it from the store
		}
		kept = append(kept, e)
	}

	ss.entries = kept
	ss.mu.Unlock()

	if changed {
		ss.notify()
	}
	return fired
}

// NextDueTime returns the earliest pending fire time among active entries.
func (ss *ScheduleStore) NextDueTime(now time.Time) (time.Time, bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	var best time.Time
	for _, e := range ss.entries {
		if !e.Active() || e.Next.IsZero() {
			continue
		}
		if best.IsZero() || e.Next.Before(best) {
			best = e.Next
		}
	}
	return best, !best.IsZero()
}

// FormatScheduleList renders schedule entries as one line each.
func FormatScheduleList(entries []*ScheduleEntry) string {
	if len(entries) == 0 {
		return "(no scheduled messages)"
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		var detail []string
		if e.Active() && !e.Next.IsZero() {
			if e.Kind != ScheduleKindOnce {
				detail = append(detail, fmt.Sprintf("next %s", e.Next.Format(scheduleTimeLayout)))
			}
			if until := time.Until(e.Next); until > 0 {
				detail = append(detail, "in "+formatDurationLoose(until.Round(time.Second)))
			}
		}
		if e.FireCount > 0 {
			detail = append(detail, fmt.Sprintf("fired %d", e.FireCount))
		}
		if e.Limit > 0 {
			detail = append(detail, fmt.Sprintf("%d/%d", e.FireCount, e.Limit))
		}

		line := fmt.Sprintf("#%d [%s] %s", e.ID, e.Status(), e.Describe())
		if len(detail) > 0 {
			line += " (" + strings.Join(detail, ", ") + ")"
		}
		if e.Name != "" {
			line += fmt.Sprintf(" name=%q", e.Name)
		}
		line += " :: " + e.Message
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// FormatScheduleEntry renders a single entry as one line.
func FormatScheduleEntry(entry *ScheduleEntry) string {
	if entry == nil {
		return ""
	}
	return strings.SplitN(FormatScheduleList([]*ScheduleEntry{entry}), "\n", 2)[0]
}

// formatDurationLoose renders a duration compactly (1d2h, 1h30m, 45m, 90s).
func formatDurationLoose(d time.Duration) string {
	if d < time.Minute {
		return d.String()
	}

	var parts []string
	for _, unit := range []struct {
		size   time.Duration
		suffix string
	}{
		{24 * time.Hour, "d"},
		{time.Hour, "h"},
		{time.Minute, "m"},
		{time.Second, "s"},
	} {
		if n := int64(d / unit.size); n > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", n, unit.suffix))
			d -= time.Duration(n) * unit.size
		}
	}
	if len(parts) == 0 {
		return d.String()
	}
	return strings.Join(parts, "")
}

// durationUnitWords maps spelled-out units to Go duration suffixes. Longer
// spellings must come first so "minutes" is rewritten before "minute".
var durationUnitWords = []struct {
	word   string
	suffix string
}{
	{"weeks", "w"}, {"week", "w"},
	{"days", "d"}, {"day", "d"},
	{"hours", "h"}, {"hour", "h"}, {"hrs", "h"}, {"hr", "h"},
	{"minutes", "m"}, {"minute", "m"}, {"mins", "m"}, {"min", "m"},
	{"seconds", "s"}, {"second", "s"}, {"secs", "s"}, {"sec", "s"},
}

// durationTokenRE matches a fully qualified duration string such as "10m",
// "2h30m" or "1.5h" (i.e. every number carries a unit).
var durationTokenRE = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?[a-zµ]+)+$`)

// durationPartRE matches a single "<number><unit>" token.
var durationPartRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)([a-zµ]+)`)

// durationUnits maps normalized unit suffixes to their length. Go's own
// ParseDuration has no days or weeks, so units are resolved here.
var durationUnits = map[string]time.Duration{
	"ns": time.Nanosecond,
	"us": time.Microsecond,
	"µs": time.Microsecond,
	"ms": time.Millisecond,
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  24 * time.Hour,
	"w":  7 * 24 * time.Hour,
}

// parseDurationLoose parses a human-friendly duration such as "10m", "2h30m",
// "90s", "3d" or "10 minutes". It returns a descriptive error otherwise.
func parseDurationLoose(spec string) (time.Duration, error) {
	trimmed := strings.ToLower(strings.TrimSpace(spec))
	if trimmed == "" {
		return 0, fmt.Errorf("empty duration")
	}

	normalized := trimmed
	for _, u := range durationUnitWords {
		normalized = strings.ReplaceAll(normalized, u.word, u.suffix)
	}
	normalized = strings.Join(strings.Fields(normalized), "")

	if !durationTokenRE.MatchString(normalized) {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 10m, 90s, 2h30m, \"10 minutes\")", spec)
	}

	var total time.Duration
	for _, part := range durationPartRE.FindAllStringSubmatch(normalized, -1) {
		unit, ok := durationUnits[part[2]]
		if !ok {
			return 0, fmt.Errorf("unknown time unit %q in %q", part[2], spec)
		}
		value, err := strconv.ParseFloat(part[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %v", spec, err)
		}
		total += time.Duration(value * float64(unit))
	}
	if total <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", spec)
	}
	return total, nil
}

// parseScheduleTime parses an absolute local time. Supported layouts are
// RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02", and a bare
// clock time ("09:00" / "09:00:00"), which rolls over to tomorrow when it has
// already passed today.
func parseScheduleTime(spec string) (time.Time, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, trimmed, time.Local); err == nil {
			return t, nil
		}
	}

	now := time.Now()
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.ParseInLocation(layout, trimmed, now.Location()); err == nil {
			cand := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
			if !cand.After(now) {
				cand = cand.AddDate(0, 0, 1)
			}
			return cand, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time %q (use e.g. 2026-05-01T09:00:00, \"2026-05-01 09:00\", or 09:00)", spec)
}

// executeSchedule manages scheduled messages that are injected into the
// conversation as user messages when they fire (requirement 048).
func (te *ToolExecutor) executeSchedule(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
		}
	}

	action = strings.ToLower(strings.TrimSpace(action))

	switch action {
	case "add":
		return te.executeScheduleAdd(params)
	case "list":
		return te.executeScheduleList()
	case "remove":
		return te.executeScheduleRemove(params)
	case "pause", "resume":
		return te.executeScheduleSetPaused(params, action == "pause")
	case "clear":
		return te.executeScheduleClear()
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid action: %s (must be one of: add, list, remove, pause, resume, clear)", action),
		}
	}
}

// executeScheduleAdd registers a new scheduled message and returns immediately.
func (te *ToolExecutor) executeScheduleAdd(params map[string]interface{}) *ToolResult {
	message, ok := params["message"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: message",
		}
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return &ToolResult{
			Success: false,
			Error:   "message cannot be empty",
		}
	}

	provided := 0
	for _, key := range []string{"in", "at", "cron", "every"} {
		if v, ok := params[key].(string); ok && strings.TrimSpace(v) != "" {
			provided++
		}
	}
	if provided != 1 {
		return &ToolResult{
			Success: false,
			Error:   "exactly one of in, at, cron, or every is required",
		}
	}

	now := time.Now()
	entry := &ScheduleEntry{
		Message: message,
		Created: now,
	}
	if name, ok := params["name"].(string); ok {
		entry.Name = strings.TrimSpace(name)
	}
	if limit, errMsg := optionalIntParamStrict(params, "limit"); errMsg != "" {
		return &ToolResult{Success: false, Error: errMsg}
	} else if limit < 0 {
		return &ToolResult{Success: false, Error: "limit must be a positive integer"}
	} else {
		entry.Limit = limit
	}

	switch {
	case strings.TrimSpace(stringParam(params, "in")) != "":
		spec := strings.TrimSpace(stringParam(params, "in"))
		delay, err := parseDurationLoose(spec)
		if err != nil {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		if delay < minScheduleDelay {
			delay = minScheduleDelay
		}
		entry.Kind = ScheduleKindOnce
		entry.Spec = spec
		entry.Next = now.Add(delay)

	case strings.TrimSpace(stringParam(params, "at")) != "":
		spec := strings.TrimSpace(stringParam(params, "at"))
		at, err := parseScheduleTime(spec)
		if err != nil {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		if !at.After(now) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("scheduled time %s is in the past (now is %s)", at.Format(scheduleTimeLayout), now.Format(scheduleTimeLayout)),
			}
		}
		entry.Kind = ScheduleKindOnce
		entry.Spec = spec
		entry.Next = at

	case strings.TrimSpace(stringParam(params, "cron")) != "":
		spec := strings.TrimSpace(stringParam(params, "cron"))
		cron, err := ParseCron(spec)
		if err != nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("invalid cron expression: %v", err)}
		}
		next := cron.NextAfter(now)
		if next.IsZero() {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("cron expression %q never fires within 5 years", spec),
			}
		}
		entry.Kind = ScheduleKindCron
		entry.Spec = spec
		entry.Cron = cron
		entry.Next = next

	case strings.TrimSpace(stringParam(params, "every")) != "":
		spec := strings.TrimSpace(stringParam(params, "every"))
		interval, err := parseDurationLoose(spec)
		if err != nil {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		if interval < minScheduleDelay {
			interval = minScheduleDelay
		}
		entry.Kind = ScheduleKindEvery
		entry.Spec = spec
		entry.Interval = interval
		entry.Next = now.Add(interval)
	}

	// A one-shot schedule fires exactly once, so a limit is meaningless. Reject
	// it rather than silently forcing it to 1 (the store's Add does the same
	// forcing, but an explicit error is clearer for the model and the user).
	if entry.Kind == ScheduleKindOnce && entry.Limit > 1 {
		return &ToolResult{
			Success: false,
			Error:   "limit does not apply to one-shot schedules (in/at fire once); use every/cron for repeated delivery",
		}
	}

	created := te.scheduleStore.Add(entry)

	return &ToolResult{
		Success: true,
		Output: fmt.Sprintf("Scheduled #%d (%s) next %s: %q",
			created.ID, created.Describe(), created.Next.Format(scheduleTimeLayout), created.Message),
		Extra: map[string]interface{}{
			"id":   created.ID,
			"kind": created.Kind,
			"next": created.Next.Format(time.RFC3339),
		},
	}
}

// executeScheduleList returns every scheduled message.
func (te *ToolExecutor) executeScheduleList() *ToolResult {
	entries := te.scheduleStore.List()

	active, paused := 0, 0
	for _, e := range entries {
		if e.Active() {
			active++
		}
		if e.Paused {
			paused++
		}
	}

	return &ToolResult{
		Success: true,
		Output:  FormatScheduleList(entries),
		Extra: map[string]interface{}{
			"totalSchedules":  len(entries),
			"activeSchedules": active,
			"pausedSchedules": paused,
		},
	}
}

// executeScheduleRemove deletes a scheduled message by ID.
func (te *ToolExecutor) executeScheduleRemove(params map[string]interface{}) *ToolResult {
	id, errMsg := scheduleIDParam(params)
	if errMsg != "" {
		return &ToolResult{Success: false, Error: errMsg}
	}

	entry := te.scheduleStore.Remove(id)
	if entry == nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("schedule #%d not found", id)}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Removed schedule #%d (%s): %q", entry.ID, entry.Describe(), entry.Message),
		Extra:   map[string]interface{}{"id": entry.ID},
	}
}

// executeScheduleSetPaused pauses or resumes a scheduled message.
func (te *ToolExecutor) executeScheduleSetPaused(params map[string]interface{}, paused bool) *ToolResult {
	id, errMsg := scheduleIDParam(params)
	if errMsg != "" {
		return &ToolResult{Success: false, Error: errMsg}
	}

	entry := te.scheduleStore.SetPaused(id, paused)
	if entry == nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("schedule #%d not found", id)}
	}

	verb := "Resumed"
	if paused {
		verb = "Paused"
	}
	out := fmt.Sprintf("%s schedule #%d (%s): %q", verb, entry.ID, entry.Describe(), entry.Message)
	if !paused && !entry.Next.IsZero() {
		out += fmt.Sprintf(" - next %s", entry.Next.Format(scheduleTimeLayout))
	}

	return &ToolResult{
		Success: true,
		Output:  out,
		Extra:   map[string]interface{}{"id": entry.ID, "paused": paused},
	}
}

// executeScheduleClear removes every scheduled message.
func (te *ToolExecutor) executeScheduleClear() *ToolResult {
	n := te.scheduleStore.Clear()
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Cleared %d scheduled message(s)", n),
		Extra:   map[string]interface{}{"removed": n},
	}
}

// optionalIntParamStrict parses an optional integer parameter. It returns an
// empty error message when the parameter is absent (value 0) or valid.
func optionalIntParamStrict(params map[string]interface{}, key string) (int, string) {
	if _, ok := params[key]; !ok {
		return 0, ""
	}
	return parseIntParamStrict(params, key)
}

// scheduleIDParam extracts the required id parameter, returning an error
// message (empty when valid).
func scheduleIDParam(params map[string]interface{}) (int, string) {
	if _, ok := params["id"]; !ok {
		return 0, "missing required parameter: id"
	}
	id, errMsg := parseIntParamStrict(params, "id")
	if errMsg != "" {
		return 0, errMsg
	}
	if id <= 0 {
		return 0, "id must be a positive integer"
	}
	return id, ""
}

// stringParam returns a string parameter or the empty string.
func stringParam(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// scheduleFlagKeys maps schedule command flags (with or without a leading "--")
// to schedule tool parameter names.
var scheduleFlagKeys = map[string]string{
	"in":    "in",
	"at":    "at",
	"cron":  "cron",
	"every": "every",
	"name":  "name",
	"limit": "limit",
	"id":    "id",
}

// scheduleTimingKeys are the flags that select when a schedule fires.
var scheduleTimingKeys = map[string]bool{"in": true, "at": true, "cron": true, "every": true}

// splitScheduleArgs splits a command line into tokens, honoring single and
// double quotes so cron expressions and messages with spaces survive.
func splitScheduleArgs(line string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	quote := rune(0)
	started := false

	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case (r == '\'' || r == '"') && !started:
			// Only a quote at the start of a token opens a quoted section;
			// elsewhere it is a literal (so messages like don't work survive
			// without quoting).
			quote = r
			started = true
		case r == ' ' || r == '\t':
			if started {
				tokens = append(tokens, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", line)
	}
	if started {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

// ParseScheduleArgs parses a CLI-style schedule command line into schedule tool
// parameters, so the user-facing /schedule command and the model-facing tool
// share one validation path. Flags come first (with or without a leading "--"),
// then the timing spec, then the message:
//
//	list
//	clear
//	add in 10m Check the build
//	add at "2026-05-01 09:00" Deploy the release
//	add cron "*/5 * * * *" Check the build
//	add every 30m Stand up
//	add --name standup --limit 3 every 1h Review the backlog
//	remove 3 | pause 3 | resume 3
//
// Everything after the first message word is message text, so messages may
// contain words such as "at" or "every" without quoting the whole sentence.
func ParseScheduleArgs(line string) (map[string]interface{}, error) {
	tokens, err := splitScheduleArgs(strings.TrimSpace(line))
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return map[string]interface{}{"action": "list"}, nil
	}

	action := strings.ToLower(tokens[0])
	switch action {
	case "add", "list", "remove", "pause", "resume", "clear":
	default:
		return nil, fmt.Errorf("unknown schedule action %q (use add, list, remove, pause, resume, or clear)", tokens[0])
	}

	params := map[string]interface{}{"action": action}
	var message []string
	timingSet := false
	idSet := false
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]

		if len(message) == 0 {
			key := strings.ToLower(strings.TrimPrefix(token, "--"))
			_, isTiming := scheduleTimingKeys[key]
			param, ok := scheduleFlagKeys[key]
			if param == "id" {
				idSet = true
			}
			if ok && !(isTiming && timingSet) {
				if i+1 >= len(tokens) {
					return nil, fmt.Errorf("missing value for %s", token)
				}
				i++
				value := tokens[i]
				if param == "limit" || param == "id" {
					n, err := strconv.Atoi(value)
					if err != nil {
						return nil, fmt.Errorf("%s expects a whole number, got %q", token, value)
					}
					params[param] = n
					continue
				}
				params[param] = value
				if isTiming {
					timingSet = true
				}
				continue
			}
			if (action == "remove" || action == "pause" || action == "resume") && !strings.HasPrefix(token, "--") {
				if idSet {
					return nil, fmt.Errorf("unexpected argument %q (id already given)", token)
				}
				n, err := strconv.Atoi(token)
				if err != nil {
					return nil, fmt.Errorf("%s expects a schedule id, got %q", action, token)
				}
				params["id"] = n
				idSet = true
				continue
			}
		}

		if strings.HasPrefix(token, "--") {
			return nil, fmt.Errorf("unknown or misplaced schedule flag %q (flags must come before the message)", token)
		}
		message = append(message, tokens[i:]...)
		break
	}

	if action == "remove" || action == "pause" || action == "resume" {
		if _, ok := params["id"]; !ok {
			return nil, fmt.Errorf("%s requires a schedule id, e.g. /schedule %s 3", action, action)
		}
	}

	if action == "add" {
		if len(message) == 0 {
			return nil, fmt.Errorf("add requires a timing spec and a message, e.g. /schedule add in 10m Check the build")
		}
		params["message"] = strings.Join(message, " ")
	} else if len(message) > 0 {
		return nil, fmt.Errorf("unexpected argument %q", message[0])
	}

	return params, nil
}
