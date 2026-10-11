package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/tools"
)

// Requirement 048: scheduled messages are manageable over the web API, and the
// number of registered schedules is reported in the session state so the
// frontend can show it on the prompt bar.

func TestCommand_ScheduleAddAndState(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{
		Session: "sched1",
		Command: "/schedule add --in 10m remind me to check the build",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !strings.Contains(resp.Output, "Scheduled #1") {
		t.Errorf("expected confirmation output, got %q", resp.Output)
	}
	if resp.State.ScheduleCount != 1 {
		t.Errorf("expected scheduleCount 1 in state, got %d", resp.State.ScheduleCount)
	}

	// The counter is visible through /api/state as well.
	rec = doJSON(t, mux, http.MethodGet, "/api/state?session=sched1", nil)
	var st stateEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if st.ScheduleCount != 1 {
		t.Errorf("expected scheduleCount 1 from /api/state, got %d", st.ScheduleCount)
	}
	if !strings.Contains(rec.Body.String(), `"scheduleCount":1`) {
		t.Errorf("expected scheduleCount in JSON payload, got %s", rec.Body.String())
	}
}

func TestCommand_ScheduleListRemoveAndErrors(t *testing.T) {
	srv, mux := newTestMux(t)

	// Adding an invalid schedule reports an error.
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{
		Session: "sched2", Command: "/schedule add --in notaduration msg",
	})
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !strings.Contains(resp.Output, "Error") {
		t.Errorf("expected error output, got %q", resp.Output)
	}

	// Register two schedules and list them.
	for _, cmd := range []string{
		"/schedule add --in 5m first message",
		"/schedule add --cron \"*/5 * * * *\" second message",
	} {
		rec = doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "sched2", Command: cmd})
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
	}
	rec = doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "sched2", Command: "/schedule list"})
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !strings.Contains(resp.Output, "first message") || !strings.Contains(resp.Output, "second message") {
		t.Errorf("expected both schedules listed, got %q", resp.Output)
	}
	if resp.State.ScheduleCount != 2 {
		t.Errorf("expected 2 schedules, got %d", resp.State.ScheduleCount)
	}

	// Removing one updates the count.
	rec = doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "sched2", Command: "/schedule remove 1"})
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.State.ScheduleCount != 1 {
		t.Errorf("expected 1 schedule after removal, got %d", resp.State.ScheduleCount)
	}

	// The session's scheduler is running so the remaining schedule still fires.
	sess := srv.sessions.Get("sched2")
	if !sess.agent.ScheduleRunning() {
		t.Error("expected the session scheduler to be running")
	}
}

// TestCommand_ScheduleAllowedDuringRun checks that managing schedules is not
// blocked by an in-flight run: schedules do not touch the conversation.
func TestCommand_ScheduleAllowedDuringRun(t *testing.T) {
	if !commandSafeDuringRun("/schedule list") {
		t.Error("expected /schedule to be safe during a run")
	}
}

// TestSession_ScheduleChangeBroadcastsState checks that a schedule change
// pushes a fresh state event (with the counter) to SSE subscribers.
func TestSession_ScheduleChangeBroadcastsState(t *testing.T) {
	srv := NewServer(config.DefaultConfig())
	sess := srv.sessions.Get("sched3")

	sub := sess.addSubscriber()
	defer sess.removeSubscriber(sub)

	sess.agent.GetToolExecutor().GetScheduleStore().Add(&tools.ScheduleEntry{
		Message: "check the build",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(10 * time.Minute),
	})

	select {
	case msg := <-sub.ch:
		if msg.event != "state" {
			t.Fatalf("expected a state event, got %q", msg.event)
		}
		var st stateEvent
		if err := json.Unmarshal([]byte(msg.data), &st); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if st.ScheduleCount != 1 {
			t.Errorf("expected scheduleCount 1 in broadcast, got %d", st.ScheduleCount)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a state broadcast after a schedule change")
	}
}

// TestSession_ScheduleDeliveryReachesSubscribers checks that a delivery that
// happens outside a run still reaches SSE subscribers as a chunk event with the
// schedule content type, so the browser can render it visibly.
func TestSession_ScheduleDeliveryReachesSubscribers(t *testing.T) {
	srv := NewServer(config.DefaultConfig())
	sess := srv.sessions.Get("sched4")
	sub := sess.addSubscriber()
	defer sess.removeSubscriber(sub)

	// The session installs a persistent stream callback; a scheduled delivery
	// goes through it.
	sess.agent.EmitScheduleNotice("Scheduled message #1 (at " + time.Now().Format(time.RFC3339) + ")\ncheck the build")

	select {
	case msg := <-sub.ch:
		if msg.event != "chunk" {
			t.Fatalf("expected a chunk event, got %q", msg.event)
		}
		var ce chunkEvent
		if err := json.Unmarshal([]byte(msg.data), &ce); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if ce.ContentType != 4 {
			t.Errorf("expected schedule content type (4), got %d", ce.ContentType)
		}
		if !strings.Contains(ce.Text, "Scheduled message #1") {
			t.Errorf("expected the delivery label in the chunk, got %q", ce.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the delivery to reach the subscriber")
	}
}

// TestAssets_AppJS_HandlesScheduleChunks checks the embedded frontend renders
// schedule deliveries and shows the counter.
func TestAssets_AppJS_HandlesScheduleChunks(t *testing.T) {
	appJS, ok := staticAsset("app.js")
	if !ok {
		t.Fatal("app.js asset missing")
	}
	app := string(appJS)
	for _, want := range []string{"contentType === 4", "msg-schedule", "scheduleCount", "Schedules: "} {
		if !strings.Contains(app, want) {
			t.Errorf("expected app.js to contain %q", want)
		}
	}
	indexBytes, ok := staticAsset("index.html")
	if !ok {
		t.Fatal("index.html asset missing")
	}
	index := string(indexBytes)
	if !strings.Contains(index, `id="schedule-count"`) {
		t.Error("expected index.html to contain the schedule counter element")
	}
}
