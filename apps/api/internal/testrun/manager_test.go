package testrun

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"superiorapi/internal/bench"
	"superiorapi/internal/httpclient"
	"superiorapi/internal/security"
)

func testManager(limits Limits) *Manager {
	factory := func(cfg bench.Config) (*bench.Runner, func()) {
		c := httpclient.New(httpclient.Options{
			Policy: security.Policy{AllowPrivate: true}, Concurrency: cfg.Concurrency, Timeout: cfg.Timeout, MaxRedirects: 5,
		})
		return &bench.Runner{Client: c, MaxResponseBytes: 1 << 20, UserAgent: "test"}, c.CloseIdleConnections
	}
	return NewManager(factory, limits, "test", 1<<20)
}

func defaultLimits() Limits {
	return Limits{
		MaxActiveGlobal: 10, MaxActivePerClient: 1, MaxActivePerTarget: 2, MaxTestsPerHour: 3,
		MaxTestDuration: 10 * time.Second, ResultTTL: time.Minute,
	}
}

func slowServer(t *testing.T, delay time.Duration) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func cfg(url string, requests, concurrency int) bench.Config {
	return bench.Config{
		URL: url, Method: "GET", Requests: requests, Concurrency: concurrency, Timeout: 5 * time.Second,
		Headers: map[string]string{"Authorization": "Bearer secret", "X-Trace": "1"},
	}
}

func wait(t *testing.T, test *Test) {
	t.Helper()
	select {
	case <-test.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("test did not finish")
	}
}

func TestManagerRunsToCompletion(t *testing.T) {
	srv := slowServer(t, 5*time.Millisecond)
	m := testManager(defaultLimits())
	test, err := m.Start(cfg(srv.URL, 20, 4), "1.2.3.4", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if s := test.Snapshot(); s.Status != StatusRunning || s.Summary != nil {
		t.Errorf("running snapshot: status=%s summary=%v", s.Status, s.Summary)
	}
	wait(t, test)

	r := test.Snapshot()
	if r.Status != StatusCompleted || r.Summary == nil || r.Analysis == nil || r.FinishedAt == nil {
		t.Fatalf("final snapshot incomplete: %+v", r)
	}
	if r.Summary.CompletedRequests != 20 || r.Summary.SuccessfulRequests != 20 || len(r.Requests) != 20 || len(r.Histogram) == 0 {
		t.Errorf("summary=%+v requests=%d", r.Summary, len(r.Requests))
	}
	for i, req := range r.Requests {
		if req.N != i+1 {
			t.Fatalf("requests not ordered by n at %d: %d", i, req.N)
		}
	}
	if r.Progress.Completed != 20 || len(r.Progress.Recent) != recentCount || r.Progress.P50 == nil {
		t.Errorf("progress = %+v", r.Progress)
	}
	if len(r.Config.HeaderNames) != 2 || r.Config.HeaderNames[0] != "Authorization" {
		t.Errorf("header names = %v", r.Config.HeaderNames)
	}
}

func TestManagerLimits(t *testing.T) {
	srv := slowServer(t, 50*time.Millisecond)
	m := testManager(defaultLimits())

	first, err := m.Start(cfg(srv.URL, 10, 1), "client-a", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	var rl *RateLimitError
	if _, err := m.Start(cfg(srv.URL, 1, 1), "client-a", "other.com"); !errors.As(err, &rl) {
		t.Errorf("second concurrent test for same client: err = %v", err)
	}
	second, err := m.Start(cfg(srv.URL, 10, 1), "client-b", "EXAMPLE.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(cfg(srv.URL, 1, 1), "client-c", "example.com"); !errors.As(err, &rl) {
		t.Errorf("third test against same target: err = %v", err)
	}
	first.Cancel()
	second.Cancel()
	wait(t, first)
	wait(t, second)

	// client-a has used 1 of 3 hourly tests; two more are allowed, then refused.
	for i := range 2 {
		x, err := m.Start(cfg(srv.URL, 1, 1), "client-a", "example.com")
		if err != nil {
			t.Fatalf("hourly test %d refused: %v", i, err)
		}
		wait(t, x)
	}
	if _, err := m.Start(cfg(srv.URL, 1, 1), "client-a", "example.com"); !errors.As(err, &rl) {
		t.Errorf("hourly limit not enforced: err = %v", err)
	}
	// After an hour the window frees up.
	m.now = func() time.Time { return time.Now().Add(61 * time.Minute) }
	x, err := m.Start(cfg(srv.URL, 1, 1), "client-a", "example.com")
	if err != nil {
		t.Errorf("hourly window did not roll over: %v", err)
	} else {
		wait(t, x)
	}
}

func TestManagerCancel(t *testing.T) {
	srv := slowServer(t, 20*time.Millisecond)
	m := testManager(defaultLimits())
	test, err := m.Start(cfg(srv.URL, 500, 2), "c", "h")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	test.Cancel()
	test.Cancel() // idempotent
	wait(t, test)

	r := test.Snapshot()
	if r.Status != StatusCancelled || r.Summary == nil || r.Summary.CompletedRequests >= 500 || r.Summary.CompletedRequests == 0 {
		t.Fatalf("status=%s summary=%+v", r.Status, r.Summary)
	}
	if r.Summary.FailedRequests != 0 {
		t.Errorf("cancelled in-flight requests were counted as failures: %d", r.Summary.FailedRequests)
	}
}

func TestManagerTimeLimit(t *testing.T) {
	srv := slowServer(t, 20*time.Millisecond)
	limits := defaultLimits()
	limits.MaxTestDuration = 150 * time.Millisecond
	m := testManager(limits)
	test, _ := m.Start(cfg(srv.URL, 500, 1), "c", "h")
	wait(t, test)
	r := test.Snapshot()
	if r.Status != StatusCancelled || r.Summary.CompletedRequests >= 500 {
		t.Errorf("status=%s completed=%d", r.Status, r.Summary.CompletedRequests)
	}
}

func TestManagerSweep(t *testing.T) {
	srv := slowServer(t, 0)
	m := testManager(defaultLimits())
	test, _ := m.Start(cfg(srv.URL, 1, 1), "c", "h")
	wait(t, test)

	m.Sweep()
	if _, ok := m.Get(test.ID); !ok {
		t.Fatal("fresh result was swept")
	}
	m.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	m.Sweep()
	if _, ok := m.Get(test.ID); ok {
		t.Error("expired result was not swept")
	}
	if len(m.history) != 0 {
		t.Errorf("stale history not pruned: %v", m.history)
	}
}

func TestNewIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := newID()
		if len(id) != 16 || seen[id] {
			t.Fatalf("bad or duplicate id %q", id)
		}
		seen[id] = true
	}
}
