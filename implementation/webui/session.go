package webui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

// subscriber is a single SSE connection registered to a session.
type subscriber struct {
	ch      chan sseMessage
	stop    chan struct{}
	dropped int64 // count of events dropped while this subscriber's channel was full
}

// sseMessage is an already-framed SSE event ready to write to a subscriber.
type sseMessage struct {
	event string
	data  string
}

// Session represents one browser session: it owns an *agent.Agent and a hub of
// SSE subscribers. Runs are serialized with a run mutex to preserve context.
type Session struct {
	ID        string
	agent     *agent.Agent
	cfg       *config.Config
	mu        sync.Mutex // guards subscribers, history, running
	subs      map[*subscriber]struct{}
	history   []string
	maxHist   int
	running   bool
	runCancel context.CancelFunc
	theme     string
	lastActive time.Time // last activity, used for idle reaping
}

// DefaultSessionID is the shared session used by every anonymous connection.
// Because the web UI is reached through a trusted SSH channel, all clients
// (page refreshes, different browsers, different machines) intentionally
// converge on this one server-side session so they continue the same work.
const DefaultSessionID = "default"

// SessionManager creates, tracks, and reaps sessions.
type SessionManager struct {
	cfg      *config.Config
	mu       sync.Mutex
	sessions map[string]*Session
	theme    string
}

// NewSessionManager creates a manager that builds sessions from cfg.
func NewSessionManager(cfg *config.Config) *SessionManager {
	return &SessionManager{
		cfg:      cfg,
		sessions: make(map[string]*Session),
		theme:    cfg.Theme,
	}
}

// Get returns an existing session or creates a new one lazily.
//
// An empty id resolves to the shared DefaultSessionID so that every anonymous
// connection (and every page refresh / new browser / new machine) continues
// the same server-side session. The session's state (agent context, goal,
// history) lives entirely on the server and is shared by all clients that
// connect without an explicit session id.
func (m *SessionManager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == "" {
		id = DefaultSessionID
	}
	if s, ok := m.sessions[id]; ok {
		s.touch()
		return s
	}
	s := newSession(m.cfg, id, m.theme)
	m.sessions[id] = s
	return s
}

// Reap removes sessions idle for longer than timeout. It is safe to call
// periodically; it never removes the most recently used session and never
// reaps a session with a run in progress. Subscribers of a reaped session are
// closed so reconnecting browsers terminate cleanly.
func (m *SessionManager) Reap(timeout time.Duration) {
	cutoff := time.Now().Add(-timeout)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.mu.Lock()
		busy := s.running
		lastActive := s.lastActive
		s.mu.Unlock()
		if !busy && lastActive.Before(cutoff) {
			// Close subscribers so their SSE loops exit.
			s.removeAllSubscribers()
			delete(m.sessions, id)
		}
	}
}

// removeAllSubscribers deregisters and stops every SSE subscriber of the
// session. The caller must not hold s.mu.
func (s *Session) removeAllSubscribers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sub := range s.subs {
		delete(s.subs, sub)
		close(sub.stop)
	}
}

// newSession builds a Session with a fresh agent from cfg.
func newSession(cfg *config.Config, id, theme string) *Session {
	ag := agent.NewAgent(cfg)
	s := &Session{
		ID:         id,
		agent:      ag,
		cfg:        cfg,
		subs:       make(map[*subscriber]struct{}),
		history:    make([]string, 0),
		maxHist:    100,
		theme:      theme,
		lastActive: time.Now(),
	}
	if val := cfg.ContextFile; val != "" {
		_ = ag.LoadContext(val)
	}
	if cfg.Goal != "" {
		ag.SetGoal(cfg.Goal)
	}
	return s
}

// touch records that the session was recently active, so idle reaping leaves
// it alone.
func (s *Session) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActive = time.Now()
}

// addSubscriber registers an SSE subscriber and returns it.
func (s *Session) addSubscriber() *subscriber {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := &subscriber{
		ch:   make(chan sseMessage, 256),
		stop: make(chan struct{}),
	}
	s.subs[sub] = struct{}{}
	return sub
}

// removeSubscriber deregisters a subscriber.
func (s *Session) removeSubscriber(sub *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[sub]; ok {
		delete(s.subs, sub)
		close(sub.stop)
	}
}

// broadcast sends a framed event to all subscribers without blocking.
// If a subscriber's channel is full, the event is dropped and counted so the
// frontend can be told content was lost (I-09). Terminal events (state,
// result, error) are never dropped: the oldest queued event is evicted to make
// room so the subscriber still learns the run finished.
func (s *Session) broadcast(event, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := sseMessage{event: event, data: data}
	critical := event == "state" || event == "result" || event == "error" || event == "truncated"
	for sub := range s.subs {
		select {
		case sub.ch <- msg:
		default:
			if critical {
				// Evict the oldest queued (likely a chunk) so a terminal event
				// is always delivered.
				select {
				case <-sub.ch:
					atomic.AddInt64(&sub.dropped, 1)
				default:
				}
				select {
				case sub.ch <- msg:
				default:
				}
			} else {
				atomic.AddInt64(&sub.dropped, 1)
			}
		}
	}
}

// addToHistory records a prompt in the session's history.
func (s *Session) addToHistory(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActive = time.Now()
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return
	}
	s.history = append([]string{trimmed}, s.history...)
	if s.maxHist > 0 && len(s.history) > s.maxHist {
		s.history = s.history[:s.maxHist]
	}
}

// clearHistory empties the session history.
func (s *Session) clearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = make([]string, 0)
}

// reset clears all server-side session state: the agent's context, the goal,
// and the input history. It is used by "New Session" to start the shared
// session fresh without relying on any client-side storage.
func (s *Session) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agent.ClearContext()
	s.agent.ClearGoal()
	s.history = make([]string, 0)
}

// historySnapshot returns a copy of the session history.
func (s *Session) historySnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.history))
	copy(out, s.history)
	return out
}

// conversation returns the session's conversation messages as historyMessages
// so a reconnecting client can render the full existing conversation.
func (s *Session) conversation() historyResponse {
	msgs := s.agent.GetConversation()
	out := make([]historyMessage, 0, len(msgs))
	for _, m := range msgs {
		hm := historyMessage{Role: m.Role, Content: m.Content, Reasoning: m.Reasoning}
		for _, tc := range m.ToolCalls {
			if tc != nil {
				hm.ToolCalls = append(hm.ToolCalls, historyToolCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
		out = append(out, hm)
	}
	return historyResponse{Messages: out}
}

// isRunning reports whether a run is currently executing.
func (s *Session) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// state builds a stateEvent for the session.
func (s *Session) state() stateEvent {
	s.touch()
	ag := s.agent
	st := s.cfg
	return stateEvent{
		ContextSize:  ag.GetContextSize(),
		MaxContext:   ag.GetMaxContextSize(),
		ReadOnly:     ag.GetToolExecutor().IsReadOnly(),
		Goal:         ag.GetGoal(),
		GoalActive:   ag.IsGoalActive(),
		Running:      s.isRunning(),
		Theme:        s.theme,
		WebAddr:      st.WebAddr,
		WebPort:      st.WebPort,
		Model:        st.Model,
		APIEndpoint:  st.APIEndpoint,
		Streaming:    st.Streaming,
		HasContext:   ag.GetActualContextSize() > 0,
		HistoryCount: len(s.historySnapshot()),
	}
}

// run executes the agent with the given prompt and streams events to the hub.
func (s *Session) run(ctx context.Context, prompt string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("a run is already in progress for this session")
	}
	s.running = true
	s.lastActive = time.Now()
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.runCancel = nil
		s.mu.Unlock()
		s.broadcastEvent("state", s.state())
		s.broadcastEvent("done", doneEvent{OK: true})
	}()

	// Emit the updated state (running=true) so the UI can disable input.
	s.broadcastEvent("state", s.state())

	var result *agent.Result
	var err error
	// broadcastChunk forwards tool/status notifications (and, in streaming
	// mode, assistant tokens) to SSE subscribers.
	broadcastChunk := func(chunk inference.StreamingChunk) {
		s.broadcastEvent("chunk", chunkEvent{
			Text:        chunk.Text,
			ContentType: int(chunk.ContentType),
			IsToolCall:  chunk.IsToolCall,
		})
	}
	if s.cfg.Streaming {
		result, err = s.agent.RunStream(runCtx, prompt, broadcastChunk)
	} else {
		// Non-streaming: install a notification callback so tool cards still
		// stream and status lines do not leak to the server's stdout, while
		// inference stays non-streaming (I-10).
		s.agent.SetStreamCallback(broadcastChunk)
		result, err = s.agent.Run(runCtx, prompt)
	}
	if err != nil {
		s.broadcastEvent("error", errorEvent{Message: err.Error()})
		return err
	}

	// Publish final result and stats.
	s.broadcastEvent("result", resultEventFromResult(result, s.agent))
	s.broadcastEvent("stats", toStats(s.agent.GetStats()))
	return nil
}

// cancel cancels the current run, if any.
func (s *Session) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runCancel != nil {
		s.runCancel()
	}
}

// broadcastEvent marshals a payload and broadcasts it as an SSE event.
func (s *Session) broadcastEvent(name string, payload interface{}) {
	data, err := jsonMarshal(payload)
	if err != nil {
		return
	}
	s.broadcast(name, string(data))
}

// resultEventFromResult builds a resultEvent from an agent.Result and agent stats.
func resultEventFromResult(result *agent.Result, ag *agent.Agent) resultEvent {
	stats := ag.GetStats()
	re := resultEvent{
		Stats:      toStats(stats),
		TokenUsage: 0,
	}
	if result != nil {
		re.FinalOutput = result.FinalOutput
		re.Reasoning = result.Reasoning
		re.TokenUsage = result.TokenUsage
		for _, st := range result.Steps {
			se := stepEvent{Action: st.Action}
			if st.ToolCall != nil {
				tc := &toolCallInfo{Name: st.ToolCall.Name, Parameters: st.ToolCall.Parameters, Arguments: st.ToolCall.Arguments}
				se.ToolCall = tc
			}
			if st.ToolResult != nil {
				out := st.ToolResult.Output
				se.ToolResult = &out
			}
			re.Steps = append(re.Steps, se)
		}
	}
	return re
}
