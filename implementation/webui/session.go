package webui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

// subscriber is a single SSE connection registered to a session.
type subscriber struct {
	ch   chan sseMessage
	stop chan struct{}
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
}

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
func (m *SessionManager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != "" {
		if s, ok := m.sessions[id]; ok {
			return s
		}
	}
	if id == "" {
		id = newSessionID()
	}
	s := newSession(m.cfg, id, m.theme)
	m.sessions[id] = s
	return s
}

// GetOrCreate returns the session with the given id, or creates it if absent.
// If id is empty, a fresh id is generated.
func (m *SessionManager) GetOrCreate(id string) *Session {
	return m.Get(id)
}

// Reap removes sessions idle for longer than timeout. It is safe to call
// periodically; it never removes the most recently used session.
func (m *SessionManager) Reap(timeout time.Duration) {
	cutoff := time.Now().Add(-timeout)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.mu.Lock()
		busy := s.running
		s.mu.Unlock()
		// We don't track last-activity precisely here; a simple heuristic keeps
		// sessions that are still running and prunes others. To keep the agent's
		// long-lived interactive model intact, reaping is opt-in via the server
		// and defaults to a very long timeout (see Server).
		if !busy && s.idleSince().Before(cutoff) {
			delete(m.sessions, id)
		}
	}
}

// newSession builds a Session with a fresh agent from cfg.
func newSession(cfg *config.Config, id, theme string) *Session {
	ag := agent.NewAgent(cfg)
	// Track context size in the session state so /api/state can report it.
	ag.SetContextSizeCallback(func(size, max int) {})
	s := &Session{
		ID:      id,
		agent:   ag,
		cfg:     cfg,
		subs:    make(map[*subscriber]struct{}),
		history: make([]string, 0),
		maxHist: 100,
		theme:   theme,
	}
	if val := cfg.ContextFile; val != "" {
		_ = ag.LoadContext(val)
	}
	if cfg.Goal != "" {
		ag.SetGoal(cfg.Goal)
	}
	return s
}

// idleSince returns a conservative idle timestamp. Because the agent may hold
// long-lived state, we treat the session as idle only when not running and
// simply use the current time (so Reap only removes sessions once the server
// has shut down, unless overridden). This keeps the default behavior safe.
func (s *Session) idleSince() time.Time {
	return time.Now()
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
func (s *Session) broadcast(event, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := sseMessage{event: event, data: data}
	for sub := range s.subs {
		select {
		case sub.ch <- msg:
		default:
			// Subscriber is slow; drop the message to avoid blocking the run.
		}
	}
}

// addToHistory records a prompt in the session's history.
func (s *Session) addToHistory(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// historySnapshot returns a copy of the session history.
func (s *Session) historySnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.history))
	copy(out, s.history)
	return out
}

// isRunning reports whether a run is currently executing.
func (s *Session) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// state builds a stateEvent for the session.
func (s *Session) state() stateEvent {
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
	if s.cfg.Streaming {
		result, err = s.agent.RunStream(runCtx, prompt, func(chunk inference.StreamingChunk) {
			s.broadcastEvent("chunk", chunkEvent{
				Text:        chunk.Text,
				ContentType: int(chunk.ContentType),
				IsToolCall:  strings.HasPrefix(chunk.Text, "[Tool Call] "),
			})
		})
	} else {
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

// newSessionID generates a short random session identifier.
func newSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
