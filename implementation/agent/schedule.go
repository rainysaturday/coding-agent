// Package agent implements the main agent logic.
// This file implements the schedule ("schedule ping") scheduler: it watches the
// tool executor's schedule store, injects due messages into the conversation as
// user messages, and starts a run when the agent is idle (requirement 048).
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// ScheduledDelivery describes a scheduled message that fired and was injected
// into the conversation. It is a snapshot handed to front-ends (TUI, web UI).
type ScheduledDelivery struct {
	ID        int       `json:"id"`
	Name      string    `json:"name,omitempty"`
	Message   string    `json:"message"`
	Kind      string    `json:"kind"`
	At        time.Time `json:"at"`
	FireCount int       `json:"fire_count"`
}

// ScheduleHooks carries optional front-end callbacks for schedule events. They
// are invoked from the scheduler goroutine, so they must not block.
type ScheduleHooks struct {
	// OnDelivery fires after a scheduled message has been added to the
	// conversation context.
	OnDelivery func(delivery ScheduledDelivery)
	// OnChange fires whenever the schedule set changes (add, remove, pause,
	// resume, clear, or a delivery), carrying the number of stored schedules.
	OnChange func(count int)
	// OnRunStart fires before the scheduler starts a run for a scheduled
	// message, OnRunEnd after it finishes.
	OnRunStart func()
	OnRunEnd   func(err error)
}

// SetScheduleHooks installs front-end callbacks for schedule events.
func (a *Agent) SetScheduleHooks(hooks ScheduleHooks) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.scheduleHooks = hooks
}

// ---------------------------------------------------------------------------
// Run gate
// ---------------------------------------------------------------------------

// acquireRunSlot blocks until the agent's single run slot is free, then claims
// it. Runs are serialized so a scheduler-initiated run and a user-initiated run
// never mutate the conversation context at the same time.
func (a *Agent) acquireRunSlot(ctx context.Context) error {
	select {
	case a.runSlot <- struct{}{}:
		a.mu.Lock()
		a.running = true
		a.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// tryAcquireRunSlot claims the run slot without blocking, reporting whether it
// was free.
func (a *Agent) tryAcquireRunSlot() bool {
	select {
	case a.runSlot <- struct{}{}:
		a.mu.Lock()
		a.running = true
		a.mu.Unlock()
		return true
	default:
		return false
	}
}

// releaseRunSlot releases the run slot.
func (a *Agent) releaseRunSlot() {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	select {
	case <-a.runSlot:
	default:
	}
}

// IsRunning reports whether a run (user-initiated or scheduler-initiated) is
// currently in progress.
func (a *Agent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// scheduledAnsweredUnlocked reports whether the pending scheduled delivery has
// already been carried into an inference request. Caller must hold a.mu.
//
// A delivery is answered only once a context snapshot was taken after it was
// injected: the model cannot respond to a message it never received. The old
// heuristic ("some message follows the scheduled one in the context") wrongly
// counted an assistant reply that was generated while the delivery raced into
// the context after the run's final snapshot, silently swallowing it. Counter
// comparison also survives compression, which replaces context messages with
// copies and breaks pointer identity.
func (a *Agent) scheduledAnsweredUnlocked() bool {
	return a.lastScheduledSeq < a.apiSnapshotSeq
}

// noteRunEnd is called when a run finishes. If scheduled messages were injected
// during the run and nothing has answered them yet, the scheduler is woken so
// it can start a follow-up run.
func (a *Agent) noteRunEnd() {
	a.mu.Lock()
	pending := a.scheduleAwaitingRun
	if pending && a.scheduledAnsweredUnlocked() {
		a.scheduleAwaitingRun = false
		pending = false
	}
	a.mu.Unlock()

	if pending {
		a.wakeScheduler()
	}
}

// ---------------------------------------------------------------------------
// Scheduler lifecycle
// ---------------------------------------------------------------------------

// StartScheduleScheduler starts the background scheduler that delivers due
// scheduled messages. It is a no-op when the scheduler is already running.
// One-shot (non-interactive) sessions never start it, so a scheduled message
// can only fire in an interactive or web session.
func (a *Agent) StartScheduleScheduler() {
	a.mu.Lock()
	if a.scheduleStop != nil {
		a.mu.Unlock()
		return
	}
	a.scheduleStop = make(chan struct{})
	a.scheduleWake = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.scheduleCtx = ctx
	a.scheduleCancel = cancel
	hooks := a.scheduleHooks
	// The loop owns its own copies of the stop/wake channels, so stopping the
	// scheduler (which nils the agent fields) cannot race with it.
	stopCh := a.scheduleStop
	wakeCh := a.scheduleWake
	doneCh := make(chan struct{})
	a.scheduleDone = doneCh
	a.mu.Unlock()

	store := a.toolExecutor.GetScheduleStore()
	store.SetChangeNotifier(func() {
		a.wakeScheduler()
		if hooks.OnChange != nil {
			hooks.OnChange(store.Count())
		}
	})

	go func() {
		defer close(doneCh)
		a.scheduleLoop(ctx, stopCh, wakeCh)
	}()
}

// StopScheduleScheduler stops the scheduler and cancels any run it started.
func (a *Agent) StopScheduleScheduler() {
	a.mu.Lock()
	stop := a.scheduleStop
	cancel := a.scheduleCancel
	runCancel := a.scheduledRunCancel
	done := a.scheduleDone
	a.scheduleStop = nil
	a.scheduleWake = nil
	a.scheduleCtx = nil
	a.scheduleCancel = nil
	a.scheduledRunCancel = nil
	a.scheduleDone = nil
	a.mu.Unlock()

	if runCancel != nil {
		runCancel()
	}
	if cancel != nil {
		cancel()
	}
	if stop != nil {
		close(stop)
	}
	// Wait for the loop goroutine to exit so no scheduler survives the stop:
	// a loop that has not reached its first select yet would otherwise keep
	// running (and reading package state) after Stop returns.
	if done != nil {
		<-done
	}
}

// CancelScheduledRun cancels the run the scheduler is currently executing, if
// any. Schedules themselves are untouched.
func (a *Agent) CancelScheduledRun() bool {
	a.mu.Lock()
	cancel := a.scheduledRunCancel
	a.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// wakeScheduler nudges the scheduler loop to re-evaluate due schedules.
func (a *Agent) wakeScheduler() {
	a.mu.Lock()
	wake := a.scheduleWake
	a.mu.Unlock()
	if wake == nil {
		return
	}
	select {
	case wake <- struct{}{}:
	default:
	}
}

// schedulePollInterval bounds how long the scheduler may sleep before it
// re-evaluates the schedule set. Change notifications normally make the wait
// exact; this is the fallback.
var schedulePollInterval = 30 * time.Second

// scheduleLoop waits for the next due schedule and delivers it. It re-arms
// whenever the schedule set changes (add/remove/pause/resume/clear).
func (a *Agent) scheduleLoop(ctx context.Context, stop <-chan struct{}, wake <-chan struct{}) {
	store := a.toolExecutor.GetScheduleStore()

	// Read the poll interval once, so it is not mutated concurrently.
	pollInterval := schedulePollInterval

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	for {
		next, pending := store.NextDueTime(time.Now())

		// Wait until the earliest pending schedule is due, but never longer
		// than schedulePollInterval. The periodic re-check is a safety net: it
		// keeps deliveries on time if a wake signal is missed or the wall
		// clock jumps (suspend/resume) while a long wait is armed.
		delay := pollInterval
		if pending {
			delay = time.Until(next)
			if delay < 0 {
				delay = 0
			}
			if delay > pollInterval {
				delay = pollInterval
			}
		}

		timer.Stop()
		select {
		case <-timer.C:
		default:
		}
		timer.Reset(delay)

		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
			a.deliverDueSchedules(ctx)
		case <-wake:
			a.deliverDueSchedules(ctx)
		}
	}
}

// ---------------------------------------------------------------------------
// Delivery
// ---------------------------------------------------------------------------

// deliverDueSchedules injects every scheduled message that is due and, when the
// agent is idle, starts a run so the message is actually acted on.
func (a *Agent) deliverDueSchedules(ctx context.Context) {
	store := a.toolExecutor.GetScheduleStore()
	for _, entry := range store.Due(time.Now()) {
		a.injectScheduledMessage(entry)
	}
	a.maybeRunScheduled(ctx)
}

// scheduledMessageText renders a scheduled message as it appears in the
// conversation context: a labelled header followed by the message body.
func scheduledMessageText(entry *tools.ScheduleEntry) string {
	label := fmt.Sprintf("Scheduled message #%d", entry.ID)
	if entry.Name != "" {
		label += fmt.Sprintf(" (%s)", entry.Name)
	}
	at := entry.LastFired
	if at.IsZero() {
		at = time.Now()
	}
	return fmt.Sprintf("[%s at %s]\n%s", label, at.Format("2006-01-02 15:04:05"), entry.Message)
}

// injectScheduledMessage appends a scheduled message to the conversation as a
// user message and notifies the front-end, so the delivery is visible to the
// user as well as to the model.
func (a *Agent) injectScheduledMessage(entry *tools.ScheduleEntry) {
	content := scheduledMessageText(entry)

	a.mu.Lock()
	msg := &inference.Message{Role: "user", Content: content}
	a.context = append(a.context, msg)
	// Remember how many snapshots existed at injection time: the delivery is
	// answered once the counter advances past this value (req 048).
	a.lastScheduledSeq = a.apiSnapshotSeq
	a.scheduleAwaitingRun = true
	streamCallback := a.streamCallback
	hooks := a.scheduleHooks
	a.mu.Unlock()

	if a.debugLogger != nil {
		a.debugLogger.LogUserMessage(content, inference.EstimateTokens(content))
	}

	notice := fmt.Sprintf("[Scheduled message #%d] %s\n", entry.ID, entry.Message)
	if streamCallback != nil {
		streamCallback(inference.StreamingChunk{
			Text:        "\n" + notice,
			ContentType: inference.StreamingContentTypeSchedule,
		})
	} else {
		fmt.Printf("\n%s%s%s", colors.GetColor("cyan"), notice, colors.GetColor("reset"))
	}

	if hooks.OnDelivery != nil {
		hooks.OnDelivery(ScheduledDelivery{
			ID:        entry.ID,
			Name:      entry.Name,
			Message:   entry.Message,
			Kind:      entry.Kind,
			At:        entry.LastFired,
			FireCount: entry.FireCount,
		})
	}
}

// maybeRunScheduled starts a run for injected scheduled messages when no other
// run is in progress. If a run is already active it is left alone: the message
// is already in the context and the next iteration will see it.
func (a *Agent) maybeRunScheduled(ctx context.Context) {
	a.mu.Lock()
	pending := a.scheduleAwaitingRun
	if pending && a.scheduledAnsweredUnlocked() {
		a.scheduleAwaitingRun = false
		pending = false
	}
	a.mu.Unlock()

	if !pending {
		return
	}
	if !a.tryAcquireRunSlot() {
		return
	}
	go a.runScheduled(ctx)
}

// runScheduled runs the agentic loop over the current context, which already
// ends with the scheduled user message.
func (a *Agent) runScheduled(ctx context.Context) {
	defer a.releaseRunSlot()

	a.mu.Lock()
	a.scheduleAwaitingRun = false
	hooks := a.scheduleHooks
	stream := a.config != nil && a.config.Streaming
	parent := a.scheduleCtx
	a.mu.Unlock()

	if parent == nil {
		parent = ctx
	}
	runCtx, cancel := context.WithCancel(parent)
	a.mu.Lock()
	a.scheduledRunCancel = cancel
	a.mu.Unlock()
	defer func() {
		cancel()
		a.mu.Lock()
		a.scheduledRunCancel = nil
		a.mu.Unlock()
	}()

	if hooks.OnRunStart != nil {
		hooks.OnRunStart()
	}

	result, err := a.runLoop(runCtx, stream)

	if err != nil {
		if runCtx.Err() == nil {
			a.streamScheduleNotice(fmt.Sprintf("[Scheduled run failed] %v\n", err))
		}
	} else if !stream && result != nil && strings.TrimSpace(result.FinalOutput) != "" {
		// A scheduled run has no caller printing its result, so surface the
		// answer here. Streaming runs already streamed the answer chunk by
		// chunk; in non-streaming mode nothing would be shown without this.
		a.emitRunOutput(result.FinalOutput)
	}

	// Close the run only after all of its output has been written: the
	// front-end hook ends the output line and repaints the prompt bar, so
	// firing it before the final answer would leave a stale prompt line
	// above the answer (requirement 048).
	if hooks.OnRunEnd != nil {
		hooks.OnRunEnd(err)
	}
}

// emitRunOutput sends a run's final answer to the front-end as ordinary model
// output, using the direct-print fallback when no callback is installed.
func (a *Agent) emitRunOutput(text string) {
	a.mu.Lock()
	streamCallback := a.streamCallback
	a.mu.Unlock()

	if streamCallback != nil {
		streamCallback(inference.StreamingChunk{
			Text:        "\n" + text + "\n",
			ContentType: inference.StreamingContentTypeNormal,
		})
		return
	}
	fmt.Printf("\n%s\n", text)
}

// streamScheduleNotice sends a schedule-related notice to the front-end.
func (a *Agent) streamScheduleNotice(notice string) {
	a.mu.Lock()
	streamCallback := a.streamCallback
	a.mu.Unlock()

	if streamCallback != nil {
		streamCallback(inference.StreamingChunk{
			Text:        "\n" + notice,
			ContentType: inference.StreamingContentTypeSchedule,
		})
		return
	}
	fmt.Printf("\n%s%s%s", colors.GetColor("cyan"), notice, colors.GetColor("reset"))
}

// ---------------------------------------------------------------------------
// Accessors and the user-facing /schedule command
// ---------------------------------------------------------------------------

// ScheduleCount returns the number of stored scheduled messages.
// EmitScheduleNotice streams a schedule notice to the front-end using the
// schedule content type. Front-ends call it to surface schedule activity that
// happens outside an agent run (for example a delivery that was injected while
// the agent was idle).
func (a *Agent) EmitScheduleNotice(notice string) {
	a.streamScheduleNotice(notice)
}

// ScheduleRunning reports whether the agent's schedule scheduler is active.
func (a *Agent) ScheduleRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.scheduleStop != nil
}

func (a *Agent) ScheduleCount() int {
	return a.toolExecutor.GetScheduleStore().Count()
}

// ScheduleActiveCount returns the number of scheduled messages that can still
// fire.
func (a *Agent) ScheduleActiveCount() int {
	return a.toolExecutor.GetScheduleStore().CountActive()
}

// ScheduleList returns a human-readable listing of the scheduled messages.
func (a *Agent) ScheduleList() string {
	return tools.FormatScheduleList(a.toolExecutor.GetScheduleStore().List())
}

// ScheduleAwaitingRun reports whether scheduled messages have been injected but
// not yet answered by a run.
func (a *Agent) ScheduleAwaitingRun() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.scheduleAwaitingRun
}

// ScheduleRemove deletes a scheduled message by ID, reporting whether it
// existed.
func (a *Agent) ScheduleRemove(id int) bool {
	return a.toolExecutor.GetScheduleStore().Remove(id) != nil
}

// ScheduleClear removes every scheduled message and returns how many were
// removed.
func (a *Agent) ScheduleClear() int {
	return a.toolExecutor.GetScheduleStore().Clear()
}

// ScheduleCommand runs a user-facing /schedule command. The command line is
// parsed into schedule tool parameters and executed through the tool executor,
// so the user and the model share exactly the same operations and validation.
func (a *Agent) ScheduleCommand(args string) string {
	params, err := tools.ParseScheduleArgs(args)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	result := a.toolExecutor.Execute(context.Background(), &tools.ToolCall{
		Name:       "schedule",
		Parameters: params,
	})
	if !result.Success {
		return fmt.Sprintf("Error: %s", result.Error)
	}
	return strings.TrimRight(result.Output, "\n")
}
