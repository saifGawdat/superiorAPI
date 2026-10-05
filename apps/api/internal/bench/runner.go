// Package bench runs benchmarks against a target API and analyzes the results.
package bench

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config describes one benchmark. It is validated before it reaches the runner.
type Config struct {
	URL         string
	Method      string
	Headers     map[string]string
	Body        *string
	Requests    int
	Concurrency int
	Timeout     time.Duration
}

// RequestResult is the measurement of a single request.
type RequestResult struct {
	N             int     `json:"n"`
	StartOffsetMs float64 `json:"startOffsetMs"`
	DurationMs    float64 `json:"durationMs"`
	Status        int     `json:"status"` // 0 when no HTTP response was received
	OK            bool    `json:"ok"`
	Bytes         int64   `json:"bytes"`
	Error         string  `json:"error,omitempty"`
	Truncated     bool    `json:"truncated,omitempty"` // body exceeded the read limit
}

// Runner executes benchmarks with a bounded worker pool.
type Runner struct {
	Client           *http.Client
	MaxResponseBytes int64
	UserAgent        string
}

// Run sends cfg.Requests requests using cfg.Concurrency workers and calls
// onResult (from worker goroutines, possibly concurrently) for every finished
// request. Requests interrupted because ctx was cancelled are not reported.
// It returns when all workers have stopped.
func (r *Runner) Run(ctx context.Context, cfg Config, onResult func(RequestResult)) {
	start := time.Now()
	jobs := make(chan int)

	var wg sync.WaitGroup
	for range cfg.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				if res, ok := r.do(ctx, cfg, n, start); ok {
					onResult(res)
				}
			}
		}()
	}

feed:
	for n := 1; n <= cfg.Requests; n++ {
		select {
		case jobs <- n:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
}

// do performs request n. The second return value is false when the request
// was aborted by test cancellation and should not be counted.
func (r *Runner) do(ctx context.Context, cfg Config, n int, testStart time.Time) (RequestResult, bool) {
	if ctx.Err() != nil {
		return RequestResult{}, false
	}
	var body io.Reader
	if cfg.Body != nil {
		body = strings.NewReader(*cfg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, body)
	if err != nil {
		return RequestResult{N: n, Error: "other"}, true
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", r.UserAgent)
	}
	if cfg.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", guessContentType(*cfg.Body))
	}

	t0 := time.Now()
	res := RequestResult{N: n, StartOffsetMs: ms(t0.Sub(testStart))}

	resp, err := r.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return RequestResult{}, false
		}
		res.DurationMs = ms(time.Since(t0))
		res.Error = classifyError(err)
		return res, true
	}

	// Read (and discard) the body so the duration covers the full response.
	read, err := io.Copy(io.Discard, io.LimitReader(resp.Body, r.MaxResponseBytes+1))
	resp.Body.Close()
	res.DurationMs = ms(time.Since(t0))
	res.Status = resp.StatusCode
	res.Bytes = min(read, r.MaxResponseBytes)
	res.Truncated = read > r.MaxResponseBytes
	res.OK = resp.StatusCode < 400

	if err != nil {
		if ctx.Err() != nil {
			return RequestResult{}, false
		}
		res.OK = false
		res.Error = classifyError(err)
	}
	return res, true
}

func guessContentType(body string) string {
	if json.Valid([]byte(body)) {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

// ms converts a duration to milliseconds rounded to 0.1ms.
func ms(d time.Duration) float64 {
	return float64(d.Round(100*time.Microsecond)) / float64(time.Millisecond)
}
