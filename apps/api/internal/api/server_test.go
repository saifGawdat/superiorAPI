package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"superiorapi/internal/bench"
	"superiorapi/internal/config"
	"superiorapi/internal/httpclient"
	"superiorapi/internal/security"
	"superiorapi/internal/testrun"
)

func newAPI(t *testing.T, allowPrivate bool) *httptest.Server {
	t.Helper()
	cfg := config.Config{
		AllowedOrigins: []string{"http://localhost:3000"}, ProbeRegion: "test",
		MaxRequests: 100, MaxConcurrency: 10, DefaultTimeout: 5 * time.Second, MaxTimeout: 10 * time.Second,
		MaxBodyBytes: 1024, MaxResponseBytes: 1 << 20, MaxRedirects: 5, MaxHeaders: 10,
		Limits: testrun.Limits{
			MaxActiveGlobal: 10, MaxActivePerClient: 1, MaxActivePerTarget: 5, MaxTestsPerHour: 100,
			MaxTestDuration: 10 * time.Second, ResultTTL: time.Minute,
		},
	}
	policy := security.Policy{AllowPrivate: allowPrivate}
	factory := func(c bench.Config) (*bench.Runner, func()) {
		client := httpclient.New(httpclient.Options{Policy: policy, Concurrency: c.Concurrency, Timeout: c.Timeout, MaxRedirects: 5})
		return &bench.Runner{Client: client, MaxResponseBytes: 1 << 20, UserAgent: "test"}, client.CloseIdleConnections
	}
	m := testrun.NewManager(factory, cfg.Limits, cfg.ProbeRegion, cfg.MaxResponseBytes)
	srv := httptest.NewServer(New(cfg, policy, m))
	t.Cleanup(func() { m.Shutdown(); srv.Close() })
	return srv
}

func target(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func errField(out map[string]any) (code, field string) {
	e, _ := out["error"].(map[string]any)
	code, _ = e["code"].(string)
	field, _ = e["field"].(string)
	return
}

// readSSE returns the event names received and the data of the last event.
func readSSE(t *testing.T, url string) ([]string, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	var events []string
	var last map[string]any
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			events = append(events, strings.TrimPrefix(line, "event: "))
		case strings.HasPrefix(line, "data: "):
			last = nil
			json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &last)
		}
	}
	return events, last
}

func TestFullFlowOverSSE(t *testing.T) {
	api, tgt := newAPI(t, true), target(t, 5*time.Millisecond)
	resp, out := post(t, api.URL+"/api/tests", map[string]any{
		"url": tgt.URL + "/users", "method": "GET", "requests": 30, "concurrency": 3,
		"headers": map[string]string{"Authorization": "Bearer secret-token"},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d: %v", resp.StatusCode, out)
	}
	id := out["testId"].(string)

	events, done := readSSE(t, api.URL+"/api/tests/"+id+"/events")
	if len(events) < 2 || events[0] != "progress" || events[len(events)-1] != "done" {
		t.Fatalf("events = %v", events)
	}
	if done["status"] != "completed" {
		t.Fatalf("done status = %v", done["status"])
	}
	summary := done["summary"].(map[string]any)
	if summary["completedRequests"].(float64) != 30 || summary["successfulRequests"].(float64) != 30 {
		t.Errorf("summary = %v", summary)
	}
	if done["analysis"] == nil || done["histogram"] == nil || len(done["requests"].([]any)) != 30 {
		t.Error("final result incomplete")
	}

	// GET returns the same final result, and secrets are never echoed back.
	r, err := http.Get(api.URL + "/api/tests/" + id)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	raw.ReadFrom(r.Body)
	r.Body.Close()
	if strings.Contains(raw.String(), "secret-token") {
		t.Error("header value leaked in result")
	}
	if !strings.Contains(raw.String(), `"headerNames":["Authorization"]`) {
		t.Errorf("header names missing: %s", raw.String()[:200])
	}

	// Streaming a finished test yields "done" immediately.
	events, _ = readSSE(t, api.URL+"/api/tests/"+id+"/events")
	if len(events) != 1 || events[0] != "done" {
		t.Errorf("events for finished test = %v", events)
	}
}

func TestStartValidation(t *testing.T) {
	api, tgt := newAPI(t, true), target(t, 0)
	cases := []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"url": "", "requests": 1, "concurrency": 1}, "url"},
		{map[string]any{"url": "ftp://x.com", "requests": 1, "concurrency": 1}, "url"},
		{map[string]any{"url": tgt.URL, "method": "TRACE", "requests": 1, "concurrency": 1}, "method"},
		{map[string]any{"url": tgt.URL, "requests": 0, "concurrency": 1}, "requests"},
		{map[string]any{"url": tgt.URL, "requests": 101, "concurrency": 1}, "requests"},
		{map[string]any{"url": tgt.URL, "requests": 5, "concurrency": 11}, "concurrency"},
		{map[string]any{"url": tgt.URL, "requests": 5, "concurrency": 6}, "concurrency"},
		{map[string]any{"url": tgt.URL, "requests": 5, "concurrency": 1, "timeoutMs": 20000}, "timeoutMs"},
		{map[string]any{"url": tgt.URL, "method": "POST", "requests": 1, "concurrency": 1, "body": strings.Repeat("x", 2000)}, "body"},
	}
	for _, c := range cases {
		resp, out := post(t, api.URL+"/api/tests", c.body)
		code, field := errField(out)
		if resp.StatusCode != http.StatusBadRequest || code != "invalid_request" || field != c.field {
			t.Errorf("%v: status=%d code=%s field=%s, want field %s", c.body, resp.StatusCode, code, field, c.field)
		}
	}
}

func TestStartRejectsPrivateTargets(t *testing.T) {
	api := newAPI(t, false)
	for _, url := range []string{"http://localhost/", "http://127.0.0.1/", "http://169.254.169.254/latest/meta-data/", "http://[::1]/", "http://10.0.0.1/"} {
		resp, out := post(t, api.URL+"/api/tests", map[string]any{"url": url, "requests": 1, "concurrency": 1})
		code, field := errField(out)
		if resp.StatusCode != http.StatusBadRequest || code != "target_not_allowed" || field != "url" {
			t.Errorf("%s: status=%d code=%s field=%s", url, resp.StatusCode, code, field)
		}
	}
}

func TestRateLimitAndCancel(t *testing.T) {
	api, tgt := newAPI(t, true), target(t, 50*time.Millisecond)
	body := map[string]any{"url": tgt.URL, "requests": 100, "concurrency": 1}
	resp, out := post(t, api.URL+"/api/tests", body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", resp.StatusCode)
	}
	id := out["testId"].(string)

	resp, out = post(t, api.URL+"/api/tests", body)
	if code, _ := errField(out); resp.StatusCode != http.StatusTooManyRequests || code != "rate_limited" {
		t.Errorf("second test: status=%d out=%v", resp.StatusCode, out)
	}

	resp, _ = post(t, api.URL+"/api/tests/"+id+"/cancel", nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("cancel status %d", resp.StatusCode)
	}
	_, done := readSSE(t, api.URL+"/api/tests/"+id+"/events")
	if done["status"] != "cancelled" {
		t.Errorf("status after cancel = %v", done["status"])
	}
}

func TestNotFoundLimitsAndCORS(t *testing.T) {
	api := newAPI(t, true)
	r, _ := http.Get(api.URL + "/api/tests/nope")
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown test status %d", r.StatusCode)
	}

	r, _ = http.Get(api.URL + "/api/limits")
	var limits map[string]any
	json.NewDecoder(r.Body).Decode(&limits)
	r.Body.Close()
	if limits["maxRequests"].(float64) != 100 || limits["probeRegion"] != "test" {
		t.Errorf("limits = %v", limits)
	}

	req, _ := http.NewRequest(http.MethodOptions, api.URL+"/api/tests", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	r, _ = http.DefaultClient.Do(req)
	if r.StatusCode != http.StatusNoContent || r.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("preflight: status=%d acao=%q", r.StatusCode, r.Header.Get("Access-Control-Allow-Origin"))
	}
	req.Header.Set("Origin", "https://evil.example")
	r, _ = http.DefaultClient.Do(req)
	if r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("unknown origin was allowed")
	}
}
