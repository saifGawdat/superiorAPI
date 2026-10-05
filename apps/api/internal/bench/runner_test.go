package bench

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"superiorapi/internal/httpclient"
	"superiorapi/internal/security"
)

func newRunner(policy security.Policy, concurrency int, timeout time.Duration) *Runner {
	return &Runner{
		Client: httpclient.New(httpclient.Options{
			Policy: policy, Concurrency: concurrency, Timeout: timeout, MaxRedirects: 5,
		}),
		MaxResponseBytes: 1 << 20,
		UserAgent:        "superiorAPI-test",
	}
}

func collect(t *testing.T, r *Runner, ctx context.Context, cfg Config) []RequestResult {
	t.Helper()
	var mu sync.Mutex
	var out []RequestResult
	r.Run(ctx, cfg, func(res RequestResult) {
		mu.Lock()
		out = append(out, res)
		mu.Unlock()
	})
	return out
}

func TestRunRespectsConcurrency(t *testing.T) {
	var inFlight, maxInFlight atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			prev := maxInFlight.Load()
			if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		if r.URL.Query().Get("fail") != "" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	r := newRunner(security.Policy{AllowPrivate: true}, 4, 5*time.Second)
	results := collect(t, r, context.Background(), Config{
		URL: srv.URL, Method: http.MethodGet, Requests: 40, Concurrency: 4, Timeout: 5 * time.Second,
	})

	if len(results) != 40 {
		t.Fatalf("got %d results, want 40", len(results))
	}
	if got := maxInFlight.Load(); got > 4 {
		t.Errorf("max in-flight = %d, want <= 4", got)
	}
	seen := map[int]bool{}
	for _, res := range results {
		if !res.OK || res.Status != 200 || res.Bytes != 5 || res.DurationMs <= 0 {
			t.Errorf("unexpected result %+v", res)
		}
		seen[res.N] = true
	}
	if len(seen) != 40 {
		t.Errorf("request numbers not unique: %d distinct", len(seen))
	}
}

func TestRunRecordsHTTPFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	r := newRunner(security.Policy{AllowPrivate: true}, 2, 5*time.Second)
	results := collect(t, r, context.Background(), Config{URL: srv.URL, Method: "GET", Requests: 3, Concurrency: 2})
	for _, res := range results {
		if res.OK || res.Status != 429 || res.Error != "" {
			t.Errorf("unexpected result %+v", res)
		}
	}
}

func TestRunTimeoutAndRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	r := newRunner(security.Policy{AllowPrivate: true}, 1, 50*time.Millisecond)
	res := collect(t, r, context.Background(), Config{URL: srv.URL, Method: "GET", Requests: 1, Concurrency: 1})
	if len(res) != 1 || res[0].Error != ErrTimeout || res[0].Status != 0 {
		t.Errorf("timeout: got %+v", res)
	}

	// A port that nothing listens on.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	r = newRunner(security.Policy{AllowPrivate: true}, 1, 2*time.Second)
	res = collect(t, r, context.Background(), Config{URL: "http://" + addr, Method: "GET", Requests: 1, Concurrency: 1})
	if len(res) != 1 || res[0].Error != ErrConnRefused {
		t.Errorf("refused: got %+v", res)
	}
}

func TestRunBlocksPrivateDestinationsAtDial(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()

	r := newRunner(security.Policy{}, 1, 2*time.Second) // production policy
	res := collect(t, r, context.Background(), Config{URL: srv.URL, Method: "GET", Requests: 2, Concurrency: 1})
	for _, x := range res {
		if x.Error != ErrBlocked {
			t.Errorf("got %+v, want blocked", x)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("private server was reached %d times", hits.Load())
	}
}

func TestRunCapsRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()

	r := newRunner(security.Policy{AllowPrivate: true}, 1, 2*time.Second)
	res := collect(t, r, context.Background(), Config{URL: srv.URL + "/", Method: "GET", Requests: 1, Concurrency: 1})
	if len(res) != 1 || res[0].Error != ErrTooManyRedirects {
		t.Errorf("got %+v, want too_many_redirects", res)
	}
}

func TestRunCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var count atomic.Int32
	r := newRunner(security.Policy{AllowPrivate: true}, 2, 5*time.Second)
	done := make(chan struct{})
	go func() {
		r.Run(ctx, Config{URL: srv.URL, Method: "GET", Requests: 500, Concurrency: 2}, func(RequestResult) {
			if count.Add(1) == 5 {
				cancel()
			}
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
	if c := count.Load(); c < 5 || c > 10 {
		t.Errorf("got %d results after cancel, want 5..10", c)
	}
}

func TestRunSendsBodyAndHeaders(t *testing.T) {
	var gotCT, gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT, gotAuth, gotUA = r.Header.Get("Content-Type"), r.Header.Get("Authorization"), r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	body := `{"a":1}`
	r := newRunner(security.Policy{AllowPrivate: true}, 1, 2*time.Second)
	collect(t, r, context.Background(), Config{
		URL: srv.URL, Method: "POST", Body: &body, Requests: 1, Concurrency: 1,
		Headers: map[string]string{"Authorization": "Bearer t"},
	})
	if gotCT != "application/json" || gotAuth != "Bearer t" || gotUA != "superiorAPI-test" {
		t.Errorf("headers: ct=%q auth=%q ua=%q", gotCT, gotAuth, gotUA)
	}
}
