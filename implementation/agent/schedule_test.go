package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// mockChatServer is a minimal OpenAI-compatible chat completions endpoint used
// by the schedule tests. It always answers with a final (tool-call-free) reply
// and records the messages it was sent.
type mockChatServer struct {
	server *httptest.Server

	// delay slows every response down, widening the window between the
	// request's context snapshot and the reply landing in the context.
	delay time.Duration

	mu       sync.Mutex
	calls    int
	messages [][]inference.Message
}

func newMockChatServer(t *testing.T) *mockChatServer {
	t.Helper()
	m := &mockChatServer{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []inference.Message `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)

		m.mu.Lock()
		m.calls++
		m.messages = append(m.messages, req.Messages)
		m.mu.Unlock()

		if m.delay > 0 {
			time.Sleep(m.delay)
		}

		if strings.Contains(string(body), `stream`+`":`+`true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte(`data: {"choices":[{"delta":{"content":"scheduled answer"},"finish_reason":"stop"}],` +
				`"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n\n" +
				"data: [DONE]\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"t1","object":"chat.completion","created":1,"model":"test-model",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"scheduled answer"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockChatServer) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// sawContent reports whether any request contained a message with substr.
func (m *mockChatServer) sawContent(substr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msgs := range m.messages {
		for _, msg := range msgs {
			if strings.Contains(msg.Content, substr) {
				return true
			}
		}
	}
	return false
}

// newScheduleAgent returns an agent wired to a mock inference server.
func newScheduleAgent(t *testing.T) (*Agent, *mockChatServer) {
	t.Helper()
	return newScheduleAgentStreaming(t, false)
}

// newScheduleAgentStreaming builds an agent whose inference client is created
// with streaming enabled or disabled, matching how the real binary configures
// inference from --stream / --no-stream.
func newScheduleAgentStreaming(t *testing.T, streaming bool) (*Agent, *mockChatServer) {
	t.Helper()
	mock := newMockChatServer(t)

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = mock.server.URL + "/v1"
	cfg.Streaming = streaming
	cfg.Skills = false

	ag := NewAgent(cfg)
	t.Cleanup(func() {
		ag.StopScheduleScheduler()
		_ = ag.CloseDebugLogger()
	})
	return ag, mock
}

// waitFor polls cond until it holds or the timeout elapses.
func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// ---------------------------------------------------------------------------
// Run gate
// ---------------------------------------------------------------------------

func TestRunGateSerializesRuns(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	if ag.IsRunning() {
		t.Fatal("a fresh agent should not be running")
	}
	if !ag.tryAcquireRunSlot() {
		t.Fatal("the run slot should be free initially")
	}
	if !ag.IsRunning() {
		t.Error("IsRunning should be true while the slot is held")
	}
	if ag.tryAcquireRunSlot() {
		t.Fatal("the run slot must not be handed out twice")
	}

	// acquireRunSlot honours context cancellation while the slot is busy.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ag.acquireRunSlot(ctx); err == nil {
		t.Error("acquireRunSlot should fail once the context expires")
	}

	ag.releaseRunSlot()
	if ag.IsRunning() {
		t.Error("IsRunning should be false after the slot is released")
	}
	if !ag.tryAcquireRunSlot() {
		t.Error("the run slot should be reusable after release")
	}
	ag.releaseRunSlot()
}

// ---------------------------------------------------------------------------
// Injection into the conversation
// ---------------------------------------------------------------------------

func TestInjectScheduledMessage(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	chunks := make(chan inference.StreamingChunk, 16)
	ag.SetStreamCallback(func(c inference.StreamingChunk) { chunks <- c })

	var delivered []ScheduledDelivery
	ag.SetScheduleHooks(ScheduleHooks{
		OnDelivery: func(d ScheduledDelivery) { delivered = append(delivered, d) },
	})

	entry := &tools.ScheduleEntry{
		ID:        7,
		Name:      "build-check",
		Message:   "check the build",
		Kind:      tools.ScheduleKindOnce,
		LastFired: time.Now(),
		FireCount: 1,
	}
	ag.injectScheduledMessage(entry)

	conv := ag.GetConversation()
	if len(conv) != 1 {
		t.Fatalf("expected 1 context message, got %d", len(conv))
	}
	last := conv[0]
	if last.Role != "user" {
		t.Errorf("scheduled message should be injected as a user message, got role %q", last.Role)
	}
	if !strings.Contains(last.Content, "check the build") {
		t.Errorf("context message should contain the scheduled text, got %q", last.Content)
	}
	if !strings.Contains(last.Content, "[Scheduled message #7 (build-check)") {
		t.Errorf("context message should carry a labelled header, got %q", last.Content)
	}

	// The user sees a notification on the dedicated schedule content type.
	noticed := false
	for len(chunks) > 0 {
		c := <-chunks
		if c.ContentType == inference.StreamingContentTypeSchedule && strings.Contains(c.Text, "check the build") {
			noticed = true
		}
	}
	if !noticed {
		t.Error("expected a schedule-typed stream notification for the delivery")
	}

	if len(delivered) != 1 || delivered[0].ID != 7 || delivered[0].Message != "check the build" {
		t.Errorf("unexpected delivery hook payload: %+v", delivered)
	}
}

func TestDeliverDueSchedulesInjectsAndRearms(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	// Hold the run gate so this test only observes injection, not a run.
	if !ag.tryAcquireRunSlot() {
		t.Fatal("could not acquire the run slot")
	}
	defer ag.releaseRunSlot()

	store := ag.GetToolExecutor().GetScheduleStore()
	past := time.Now().Add(-time.Second)

	once := store.Add(&tools.ScheduleEntry{Message: "one-shot ping", Kind: tools.ScheduleKindOnce, Next: past})
	cronExpr, err := tools.ParseCron("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	recurring := store.Add(&tools.ScheduleEntry{Message: "cron ping", Kind: tools.ScheduleKindCron, Cron: cronExpr, Next: past})

	ag.deliverDueSchedules(context.Background())

	conv := ag.GetConversation()
	if len(conv) != 2 {
		t.Fatalf("expected 2 injected messages, got %d", len(conv))
	}
	for _, msg := range conv {
		if msg.Role != "user" {
			t.Errorf("expected user role, got %q", msg.Role)
		}
	}

	// The one-shot entry is retired; the cron entry is re-armed in the future.
	if store.Get(once.ID) != nil {
		t.Error("the fired one-shot schedule should be retired")
	}
	updated := store.Get(recurring.ID)
	if updated == nil {
		t.Fatal("the recurring schedule should still exist")
	}
	if !updated.Next.After(time.Now()) {
		t.Errorf("the recurring schedule should be re-armed in the future, got %v", updated.Next)
	}
	if updated.FireCount != 1 {
		t.Errorf("expected fire count 1, got %d", updated.FireCount)
	}
	if ag.ScheduleCount() != 1 {
		t.Errorf("expected 1 remaining schedule, got %d", ag.ScheduleCount())
	}
}

// ---------------------------------------------------------------------------
// Starting runs for scheduled messages
// ---------------------------------------------------------------------------

func TestScheduledRunStartsWhenIdle(t *testing.T) {
	ag, mock := newScheduleAgent(t)

	store := ag.GetToolExecutor().GetScheduleStore()
	store.Add(&tools.ScheduleEntry{
		Message: "do the nightly thing",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(-time.Second),
	})

	ag.deliverDueSchedules(context.Background())

	if !waitFor(func() bool { return mock.callCount() > 0 }, 5*time.Second) {
		t.Fatal("a scheduled run was never started")
	}
	if !mock.sawContent("do the nightly thing") {
		t.Error("the scheduled message was not sent to the model")
	}
	if !waitFor(func() bool { return !ag.IsRunning() }, 5*time.Second) {
		t.Fatal("the scheduled run did not finish")
	}
	if ag.ScheduleAwaitingRun() {
		t.Error("the awaiting-run flag should be cleared once the run has started")
	}
}

func TestScheduledRunDeferredWhileBusy(t *testing.T) {
	ag, mock := newScheduleAgent(t)

	// Simulate a user-initiated run holding the gate.
	if !ag.tryAcquireRunSlot() {
		t.Fatal("could not acquire the run slot")
	}

	store := ag.GetToolExecutor().GetScheduleStore()
	store.Add(&tools.ScheduleEntry{
		Message: "queued ping",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(-time.Second),
	})

	ag.deliverDueSchedules(context.Background())
	time.Sleep(150 * time.Millisecond)

	if mock.callCount() != 0 {
		t.Error("no run may start while another run holds the gate")
	}
	if !ag.ScheduleAwaitingRun() {
		t.Error("the injected message should still be awaiting a run")
	}
	conv := ag.GetConversation()
	if len(conv) == 0 || !strings.Contains(conv[len(conv)-1].Content, "queued ping") {
		t.Error("the message should already be visible in the context")
	}

	ag.releaseRunSlot()
}

func TestMaybeRunScheduledNoopWithoutPendingMessage(t *testing.T) {
	ag, mock := newScheduleAgent(t)
	ag.maybeRunScheduled(context.Background())
	time.Sleep(100 * time.Millisecond)
	if mock.callCount() != 0 {
		t.Error("no run should start without an injected scheduled message")
	}
}

func TestCancelScheduledRunWithoutRun(t *testing.T) {
	ag, _ := newScheduleAgent(t)
	if ag.CancelScheduledRun() {
		t.Error("CancelScheduledRun should report false when no scheduled run is active")
	}
}

// ---------------------------------------------------------------------------
// Scheduler lifecycle
// ---------------------------------------------------------------------------

func TestSchedulerLoopFiresDueSchedules(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	// Keep runs from starting so the test only observes delivery.
	if !ag.tryAcquireRunSlot() {
		t.Fatal("could not acquire the run slot")
	}
	defer ag.releaseRunSlot()

	store := ag.GetToolExecutor().GetScheduleStore()
	store.Add(&tools.ScheduleEntry{
		Message: "fire immediately",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(-time.Second),
	})

	ag.StartScheduleScheduler()
	// Starting twice must be a no-op.
	ag.StartScheduleScheduler()
	defer ag.StopScheduleScheduler()

	if !waitFor(func() bool { return store.Count() == 0 }, 3*time.Second) {
		t.Fatal("the scheduler never delivered the due schedule")
	}
	conv := ag.GetConversation()
	if len(conv) == 0 || !strings.Contains(conv[len(conv)-1].Content, "fire immediately") {
		t.Error("the scheduled message was not injected into the context")
	}

	// After StopScheduleScheduler the loop no longer delivers anything.
	ag.StopScheduleScheduler()
	store.Add(&tools.ScheduleEntry{
		Message: "should not fire",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(-time.Second),
	})
	time.Sleep(200 * time.Millisecond)
	if store.Count() != 1 {
		t.Error("a stopped scheduler must not deliver schedules")
	}
}

func TestSchedulerChangeNotifierWakesLoop(t *testing.T) {
	ag, _ := newScheduleAgent(t)
	if !ag.tryAcquireRunSlot() {
		t.Fatal("could not acquire the run slot")
	}
	defer ag.releaseRunSlot()

	ag.StartScheduleScheduler()
	defer ag.StopScheduleScheduler()

	store := ag.GetToolExecutor().GetScheduleStore()
	// Adding a schedule that is already due wakes the loop and delivers it.
	store.Add(&tools.ScheduleEntry{Message: "wake up", Kind: tools.ScheduleKindOnce, Next: time.Now()})

	if !waitFor(func() bool { return store.Count() == 0 }, 3*time.Second) {
		t.Fatal("adding a due schedule did not wake the scheduler")
	}
}

func TestScheduleHooksOnChangeFires(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	changes := make(chan int, 16)
	ag.SetScheduleHooks(ScheduleHooks{OnChange: func(count int) { changes <- count }})
	ag.StartScheduleScheduler()
	defer ag.StopScheduleScheduler()

	result := ag.GetToolExecutor().Execute(context.Background(), &tools.ToolCall{
		Name:       "schedule",
		Parameters: map[string]interface{}{"action": "add", "message": "count me", "every": "1h"},
	})
	if !result.Success {
		t.Fatalf("Execute failed: %s", result.Error)
	}

	select {
	case count := <-changes:
		if count != 1 {
			t.Errorf("expected change count 1, got %d", count)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the change notifier never fired")
	}
	if ag.ScheduleCount() != 1 {
		t.Errorf("expected ScheduleCount 1, got %d", ag.ScheduleCount())
	}
}

// ---------------------------------------------------------------------------
// The user-facing /schedule surface
// ---------------------------------------------------------------------------

func TestScheduleCommandSurface(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	out := ag.ScheduleCommand("add in 10m check the build")
	if !strings.Contains(out, "Scheduled #1") || !strings.Contains(out, "check the build") {
		t.Errorf("unexpected add output: %q", out)
	}
	if ag.ScheduleCount() != 1 {
		t.Fatalf("expected 1 schedule, got %d", ag.ScheduleCount())
	}

	out = ag.ScheduleCommand("add every 5m stand-up")
	if !strings.Contains(out, "Scheduled #2") {
		t.Errorf("unexpected add output: %q", out)
	}
	if ag.ScheduleActiveCount() != 2 {
		t.Errorf("expected 2 active schedules, got %d", ag.ScheduleActiveCount())
	}

	out = ag.ScheduleCommand("list")
	if !strings.Contains(out, "check the build") || !strings.Contains(out, "stand-up") {
		t.Errorf("list output missing entries: %q", out)
	}
	if ag.ScheduleList() == "" {
		t.Error("ScheduleList should render a listing")
	}

	out = ag.ScheduleCommand("pause 1")
	if !strings.Contains(out, "Paused schedule #1") {
		t.Errorf("unexpected pause output: %q", out)
	}
	if ag.ScheduleActiveCount() != 1 {
		t.Errorf("expected 1 active schedule after pause, got %d", ag.ScheduleActiveCount())
	}

	out = ag.ScheduleCommand("resume 1")
	if !strings.Contains(out, "Resumed schedule #1") {
		t.Errorf("unexpected resume output: %q", out)
	}

	out = ag.ScheduleCommand("remove 1")
	if !strings.Contains(out, "Removed schedule #1") {
		t.Errorf("unexpected remove output: %q", out)
	}
	if ag.ScheduleCount() != 1 {
		t.Errorf("expected 1 schedule after remove, got %d", ag.ScheduleCount())
	}

	if !ag.ScheduleRemove(2) {
		t.Error("ScheduleRemove(2) should report success")
	}
	if ag.ScheduleRemove(2) {
		t.Error("ScheduleRemove(2) should report failure for a missing schedule")
	}
	if ag.ScheduleCount() != 0 {
		t.Errorf("expected no schedules, got %d", ag.ScheduleCount())
	}

	ag.ScheduleCommand("add every 10m a")
	ag.ScheduleCommand("add every 10m b")
	if n := ag.ScheduleClear(); n != 2 {
		t.Errorf("ScheduleClear should report 2, got %d", n)
	}
	if ag.ScheduleCount() != 0 {
		t.Error("ScheduleClear should empty the store")
	}

	// Errors are surfaced to the user.
	if out := ag.ScheduleCommand("remove 999"); !strings.Contains(out, "not found") {
		t.Errorf("expected a not-found error, got %q", out)
	}
	if out := ag.ScheduleCommand("add"); !strings.Contains(out, "Error") {
		t.Errorf("expected an error for a missing message, got %q", out)
	}
	if out := ag.ScheduleCommand("bogus"); !strings.Contains(out, "Error") || !strings.Contains(out, "action") {
		t.Errorf("expected an invalid action error, got %q", out)
	}
}

// The model and the user share the same store: a schedule added through the
// tool is visible to the user-facing accessors and vice versa.
func TestModelAndUserShareTheScheduleStore(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	result := ag.GetToolExecutor().Execute(context.Background(), &tools.ToolCall{
		Name: "schedule",
		Parameters: map[string]interface{}{
			"action":  "add",
			"message": "from the model",
			"cron":    "0 9 * * *",
			"name":    "morning",
		},
	})
	if !result.Success {
		t.Fatalf("Execute failed: %s", result.Error)
	}

	if ag.ScheduleCount() != 1 {
		t.Fatalf("expected 1 schedule, got %d", ag.ScheduleCount())
	}
	listing := ag.ScheduleList()
	if !strings.Contains(listing, "from the model") || !strings.Contains(listing, "morning") {
		t.Errorf("user listing should show the model's schedule: %q", listing)
	}

	// The user removes it; the model then sees an empty list.
	if !ag.ScheduleRemove(1) {
		t.Fatal("user removal failed")
	}
	listResult := ag.GetToolExecutor().Execute(context.Background(), &tools.ToolCall{
		Name:       "schedule",
		Parameters: map[string]interface{}{"action": "list"},
	})
	if !listResult.Success {
		t.Fatalf("list failed: %s", listResult.Error)
	}
	if strings.Contains(listResult.Output, "from the model") {
		t.Error("the model should not see a schedule the user removed")
	}
}

// TestStatsIncludeScheduleCount checks that /stats reports how many scheduled
// messages can still fire.
func TestStatsIncludeScheduleCount(t *testing.T) {
	ag, _ := newScheduleAgent(t)
	store := ag.GetToolExecutor().GetScheduleStore()

	if got := ag.GetStats().Schedules; got != 0 {
		t.Fatalf("expected 0 schedules, got %d", got)
	}

	store.Add(&tools.ScheduleEntry{Message: "active one", Kind: tools.ScheduleKindOnce, Next: time.Now().Add(time.Minute)})
	store.Add(&tools.ScheduleEntry{Message: "paused one", Kind: tools.ScheduleKindEvery, Interval: time.Hour, Next: time.Now().Add(time.Minute), Paused: true})

	if got := ag.GetStats().Schedules; got != 1 {
		t.Errorf("expected 1 active schedule in stats, got %d", got)
	}
	// The raw total still counts the paused entry.
	if got := ag.ScheduleCount(); got != 2 {
		t.Errorf("expected ScheduleCount 2, got %d", got)
	}
}

// TestSchedulerPollFallbackDelivers checks that the scheduler re-evaluates on
// its own even when no change notification arrives, so a missed wake signal
// cannot stall pending deliveries.
func TestSchedulerPollFallbackDelivers(t *testing.T) {
	original := schedulePollInterval
	schedulePollInterval = 20 * time.Millisecond
	defer func() { schedulePollInterval = original }()

	ag, mock := newScheduleAgent(t)
	// The loop snapshots the interval at startup, so set it before starting.
	ag.StartScheduleScheduler()

	store := ag.GetToolExecutor().GetScheduleStore()
	// Silence the notifier so delivery can only happen through the periodic
	// re-check.
	store.SetChangeNotifier(nil)
	store.Add(&tools.ScheduleEntry{
		Message: "polled delivery",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(10 * time.Millisecond),
	})

	if !waitFor(func() bool { return mock.sawContent("polled delivery") }, 3*time.Second) {
		t.Fatal("expected the poll fallback to deliver the scheduled message")
	}
	if got := ag.ScheduleCount(); got != 0 {
		t.Errorf("expected the one-shot schedule to be retired, got %d left", got)
	}
}

func TestRunScheduledEmitsAnswerWhenNotStreaming(t *testing.T) {
	agent, _ := newScheduleAgent(t)

	var mu sync.Mutex
	var answers []string
	agent.streamCallback = func(chunk inference.StreamingChunk) {
		if chunk.ContentType == inference.StreamingContentTypeNormal {
			mu.Lock()
			answers = append(answers, chunk.Text)
			mu.Unlock()
		}
	}

	// Non-streaming inference does not stream the answer, so the scheduled run
	// must emit the final output itself; otherwise the reply is invisible.
	agent.runScheduled(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(answers) == 0 {
		t.Fatal("expected the scheduled run's answer to be emitted")
	}
	if !strings.Contains(strings.Join(answers, ""), "scheduled answer") {
		t.Errorf("emitted output = %q, want it to contain the model answer", answers)
	}
}

func TestRunScheduledDoesNotDuplicateStreamedAnswer(t *testing.T) {
	agent, _ := newScheduleAgentStreaming(t, true)

	var mu sync.Mutex
	var answers []string
	agent.streamCallback = func(chunk inference.StreamingChunk) {
		if chunk.ContentType == inference.StreamingContentTypeNormal {
			mu.Lock()
			answers = append(answers, chunk.Text)
			mu.Unlock()
		}
	}

	// Streaming runs already deliver the answer chunk by chunk, so the scheduler
	// must not emit it a second time.
	agent.runScheduled(context.Background())

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(answers, "")
	if !strings.Contains(joined, "scheduled answer") {
		t.Fatalf("streaming run should still deliver the answer, got %q", answers)
	}
	if strings.Count(joined, "scheduled answer") != 1 {
		t.Errorf("streaming run delivered the answer more than once: %q", joined)
	}
}

// TestScheduledDeliveryDuringInflightRequestIsNotSwallowed is the regression
// test for the swallowed-delivery race: a schedule that fires while a run's
// API request is in flight (after that request's context snapshot) must not be
// treated as answered by the reply that lands after it. The run's end must
// wake the scheduler and start a follow-up run that carries the message to the
// model (requirement 048: "picked up by that run's next iteration", or by a
// follow-up run when the current one is already past its last snapshot).
func TestScheduledDeliveryDuringInflightRequestIsNotSwallowed(t *testing.T) {
	mock := newMockChatServer(t)
	mock.delay = 300 * time.Millisecond

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = mock.server.URL + "/v1"
	cfg.Streaming = false
	cfg.Skills = false
	ag := NewAgent(cfg)
	defer func() {
		ag.StopScheduleScheduler()
		_ = ag.CloseDebugLogger()
	}()

	// The schedule fires ~150ms into the run, i.e. while the run's only (and
	// therefore final) API request is still in flight.
	store := ag.GetToolExecutor().GetScheduleStore()
	store.Add(&tools.ScheduleEntry{
		Message: "midflight ping",
		Kind:    tools.ScheduleKindOnce,
		Next:    time.Now().Add(150 * time.Millisecond),
	})

	ag.StartScheduleScheduler()

	userDone := make(chan struct{})
	go func() {
		ag.Run(context.Background(), "user prompt")
		close(userDone)
	}()

	<-userDone

	// The user run's reply was generated without the ping, so the scheduler
	// must start a follow-up run whose request contains it.
	if !waitFor(func() bool { return mock.sawContent("midflight ping") }, 3*time.Second) {
		t.Fatal("the follow-up run never carried the injected scheduled message to the model")
	}
	if !waitFor(func() bool { return !ag.ScheduleAwaitingRun() }, 3*time.Second) {
		t.Error("the delivery should be cleared from awaiting state once answered")
	}
}

// TestRunScheduledEndHookFiresAfterOutput pins the front-end ordering: the
// OnRunEnd hook (which repaints the TUI prompt bar) must run only after the
// scheduled run's answer has been emitted, otherwise the prompt is painted
// above the answer and a stale prompt line is left in the scrollback.
func TestRunScheduledEndHookFiresAfterOutput(t *testing.T) {
	ag, _ := newScheduleAgent(t)

	var mu sync.Mutex
	var events []string
	ag.streamCallback = func(chunk inference.StreamingChunk) {
		if chunk.ContentType == inference.StreamingContentTypeNormal {
			mu.Lock()
			events = append(events, "output")
			mu.Unlock()
		}
	}
	ag.SetScheduleHooks(ScheduleHooks{
		OnRunEnd: func(error) {
			mu.Lock()
			events = append(events, "runend")
			mu.Unlock()
		},
	})

	ag.runScheduled(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[0] != "output" || events[1] != "runend" {
		t.Errorf("event order = %v, want [output runend]", events)
	}
}
