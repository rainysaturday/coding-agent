package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/inference"
)

// mockBackend serves scripted chat completions.
func mockBackend(t *testing.T, contents ...string) *httptest.Server {
	t.Helper()
	var idx int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "done"
		if idx < len(contents) {
			content = contents[idx]
		}
		idx++
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]interface{}{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 20, "completion_tokens": 5, "total_tokens": 25},
		})
	}))
}

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	mb := mockBackend(t, "web answer", "second answer")
	t.Cleanup(mb.Close)
	cfg := config.NewDefault()
	cfg.APIEndpoint = mb.URL
	cfg.APIKey = "test"
	cfg.Stream = false
	cfg.SkillsEnabled = false
	cfg.WorkingDir = t.TempDir()
	cfg.AutoCompress = false
	s := NewServer(cfg)
	web := httptest.NewServer(s.Handler())
	t.Cleanup(web.Close)
	return s, web
}

func post(t *testing.T, url string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	data, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestIndexPage(t *testing.T) {
	_, web := newTestServer(t)
	resp, err := http.Get(web.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	html := string(data)
	if !strings.Contains(html, "<html") {
		t.Fatal("not an html page")
	}
	if strings.Contains(html, "__MODEL__") || strings.Contains(html, "/*__THEME__*/") {
		t.Fatal("placeholders not substituted")
	}
}

func TestAssetsServed(t *testing.T) {
	_, web := newTestServer(t)
	for path, wantType := range map[string]string{
		"/assets/app.js":    "application/javascript",
		"/assets/app.css":   "text/css",
		"/assets/style.css": "text/css",
	} {
		resp, err := http.Get(web.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s status: %d", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, wantType) {
			t.Fatalf("%s content-type: %q", path, ct)
		}
	}
}

func TestChatAndStateFlow(t *testing.T) {
	_, web := newTestServer(t)

	code, out := post(t, web.URL+"/api/chat", map[string]string{"session": "s1", "prompt": "hello"})
	if code != 200 || out["session"] != "s1" {
		t.Fatalf("chat: %d %v", code, out)
	}

	// State reflects the completed run.
	resp, err := http.Get(web.URL + "/api/state?session=s1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st struct {
		Session string `json:"session"`
		State   struct {
			Model string `json:"model"`
			Stats struct {
				Iterations   int `json:"iterations"`
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"stats"`
		} `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.State.Stats.Iterations != 1 || st.State.Stats.InputTokens != 20 {
		t.Fatalf("state: %+v", st)
	}

	// History contains the prompt.
	resp2, err := http.Get(web.URL + "/api/history?session=s1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var hist struct {
		History []string `json:"history"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&hist)
	if len(hist.History) != 1 || hist.History[0] != "hello" {
		t.Fatalf("history: %+v", hist)
	}
}

func TestSEESStreamDeliversEvents(t *testing.T) {
	_, web := newTestServer(t)

	// Subscribe first.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, web.URL+"/api/events?session=s2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("sse content-type: %q", ct)
	}

	// Trigger a run in the background.
	go func() {
		time.Sleep(100 * time.Millisecond)
		post(t, web.URL+"/api/chat", map[string]string{"session": "s2", "prompt": "stream me"})
	}()

	sc := bufio.NewScanner(resp.Body)
	var types []string
	var content string
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) != nil {
			continue
		}
		types = append(types, ev.Type)
		if ev.Type == "content" {
			content += ev.Text
		}
		if ev.Type == "done" {
			break
		}
	}
	if content != "web answer" {
		t.Fatalf("streamed content: %q (events: %v)", content, types)
	}
}

func TestCommands(t *testing.T) {
	_, web := newTestServer(t)

	code, out := post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/stats"})
	if code != 200 || !strings.Contains(out["output"].(string), "Iterations") {
		t.Fatalf("/stats: %v", out)
	}

	_, out = post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/goal fix it"})
	if !strings.Contains(out["output"].(string), "Goal set") {
		t.Fatalf("/goal: %v", out)
	}
	_, out = post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/goal"})
	if !strings.Contains(out["output"].(string), "fix it") {
		t.Fatalf("/goal status: %v", out)
	}
	_, out = post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/goal-off"})
	if !strings.Contains(out["output"].(string), "deactivated") {
		t.Fatalf("/goal-off: %v", out)
	}

	_, out = post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/read-only"})
	if !strings.Contains(out["output"].(string), "enabled") {
		t.Fatalf("/read-only: %v", out)
	}
	if st := out["state"].(map[string]interface{}); st["read_only"] != true {
		t.Fatalf("state read_only: %v", st["read_only"])
	}

	_, out = post(t, web.URL+"/api/command", map[string]string{"session": "s3", "command": "/bogus"})
	if !strings.Contains(out["output"].(string), "Unknown command") {
		t.Fatalf("/bogus: %v", out)
	}
}

func TestMethodGuards(t *testing.T) {
	_, web := newTestServer(t)
	resp, err := http.Get(web.URL + "/api/chat")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/chat: %d", resp.StatusCode)
	}
}

func TestSessionIsolation(t *testing.T) {
	_, web := newTestServer(t)
	post(t, web.URL+"/api/chat", map[string]string{"session": "a", "prompt": "for a"})
	// Session b must not see a's context.
	resp, err := http.Get(web.URL + "/api/state?session=b")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st struct {
		State struct {
			Stats struct {
				Iterations int `json:"iterations"`
			} `json:"stats"`
		} `json:"state"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&st)
	if st.State.Stats.Iterations != 0 {
		t.Fatalf("session b contaminated: %+v", st)
	}
}

// keep inference import used if mocks change
var _ = inference.NewTextMessage
