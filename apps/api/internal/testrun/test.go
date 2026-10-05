// Package testrun manages the lifecycle of benchmark tests: starting them
// under abuse limits, tracking live progress, cancelling and expiring them.
package testrun

import (
	"slices"
	"sync"
	"time"

	"superiorapi/internal/bench"
)

// Test statuses.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

const recentCount = 12

// ConfigView is the public, secret-free view of a test's configuration.
// Header values and the body are never echoed back.
type ConfigView struct {
	URL         string   `json:"url"`
	Method      string   `json:"method"`
	Requests    int      `json:"requests"`
	Concurrency int      `json:"concurrency"`
	TimeoutMs   int64    `json:"timeoutMs"`
	HeaderNames []string `json:"headerNames"`
	HasBody     bool     `json:"hasBody"`
}

// Progress is a live snapshot of a running test.
type Progress struct {
	Completed int                   `json:"completed"`
	Succeeded int                   `json:"succeeded"`
	Failed    int                   `json:"failed"`
	Total     int                   `json:"total"`
	ElapsedMs float64               `json:"elapsedMs"`
	P50       *float64              `json:"p50"`
	P95       *float64              `json:"p95"`
	Recent    []bench.RequestResult `json:"recent"` // newest first
}

// Result is the full public state of a test.
type Result struct {
	ID          string                `json:"id"`
	Status      string                `json:"status"`
	Config      ConfigView            `json:"config"`
	ProbeRegion string                `json:"probeRegion"`
	StartedAt   time.Time             `json:"startedAt"`
	FinishedAt  *time.Time            `json:"finishedAt,omitempty"`
	Progress    Progress              `json:"progress"`
	Summary     *bench.Summary        `json:"summary,omitempty"`
	Histogram   []bench.Bucket        `json:"histogram,omitempty"`
	Analysis    *bench.Analysis       `json:"analysis,omitempty"`
	Requests    []bench.RequestResult `json:"requests,omitempty"` // ordered by n
}

// Test is one benchmark run. All exported methods are safe for concurrent use.
type Test struct {
	ID         string
	cfg        bench.Config
	view       ConfigView
	region     string
	clientKey  string
	targetHost string
	startedAt  time.Time
	done       chan struct{}
	cancelOnce sync.Once
	userCancel chan struct{}

	mu         sync.Mutex
	status     string
	results    []bench.RequestResult // completion order
	succeeded  int
	finishedAt time.Time
	final      *Result
	version    uint64
}

func newTest(id string, cfg bench.Config, region, clientKey, targetHost string, now time.Time) *Test {
	names := make([]string, 0, len(cfg.Headers))
	for k := range cfg.Headers {
		names = append(names, k)
	}
	slices.Sort(names)
	return &Test{
		ID: id, cfg: cfg, region: region, clientKey: clientKey, targetHost: targetHost,
		startedAt: now, done: make(chan struct{}), userCancel: make(chan struct{}),
		status: StatusRunning,
		view: ConfigView{
			URL: cfg.URL, Method: cfg.Method, Requests: cfg.Requests, Concurrency: cfg.Concurrency,
			TimeoutMs: cfg.Timeout.Milliseconds(), HeaderNames: names, HasBody: cfg.Body != nil,
		},
	}
}

// Done is closed when the test has finished and its result is final.
func (t *Test) Done() <-chan struct{} { return t.done }

// Cancel stops a running test. Already-finished tests are unaffected.
func (t *Test) Cancel() {
	t.cancelOnce.Do(func() { close(t.userCancel) })
}

func (t *Test) record(r bench.RequestResult) {
	t.mu.Lock()
	t.results = append(t.results, r)
	if r.OK {
		t.succeeded++
	}
	t.version++
	t.mu.Unlock()
}

// Progress returns the live progress and a version that changes whenever a
// new result arrives.
func (t *Test) Progress() (Progress, uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.progressLocked(), t.version
}

func (t *Test) progressLocked() Progress {
	elapsed := time.Since(t.startedAt)
	if !t.finishedAt.IsZero() {
		elapsed = t.finishedAt.Sub(t.startedAt)
	}
	p50, p95 := bench.LivePercentiles(t.results)
	recent := make([]bench.RequestResult, 0, recentCount)
	for i := len(t.results) - 1; i >= 0 && len(recent) < recentCount; i-- {
		recent = append(recent, t.results[i])
	}
	return Progress{
		Completed: len(t.results),
		Succeeded: t.succeeded,
		Failed:    len(t.results) - t.succeeded,
		Total:     t.cfg.Requests,
		ElapsedMs: float64(elapsed.Round(100*time.Microsecond)) / float64(time.Millisecond),
		P50:       p50,
		P95:       p95,
		Recent:    recent,
	}
}

// Snapshot returns the current public state: progress while running, the
// full report once finished.
func (t *Test) Snapshot() Result {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.final != nil {
		return *t.final
	}
	return Result{
		ID: t.ID, Status: t.status, Config: t.view, ProbeRegion: t.region,
		StartedAt: t.startedAt, Progress: t.progressLocked(),
	}
}

// finish computes the final report. stopReason is one of the bench.Stop* values.
func (t *Test) finish(elapsed time.Duration, stopReason string, a analysisParams) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.finishedAt = t.startedAt.Add(elapsed)
	t.status = StatusCompleted
	if stopReason != bench.StopCompleted {
		t.status = StatusCancelled
	}

	summary := bench.Summarize(t.results, t.cfg.Requests, elapsed)
	analysis := bench.Analyze(bench.AnalysisInput{
		Summary:          summary,
		Results:          t.results,
		Concurrency:      t.cfg.Concurrency,
		Timeout:          t.cfg.Timeout,
		MaxTestDuration:  a.maxTestDuration,
		MaxResponseBytes: a.maxResponseBytes,
		ProbeRegion:      t.region,
		StopReason:       stopReason,
	})
	byN := slices.Clone(t.results)
	slices.SortFunc(byN, func(x, y bench.RequestResult) int { return x.N - y.N })
	finishedAt := t.finishedAt

	t.final = &Result{
		ID: t.ID, Status: t.status, Config: t.view, ProbeRegion: t.region,
		StartedAt: t.startedAt, FinishedAt: &finishedAt,
		Progress:  t.progressLocked(),
		Summary:   &summary,
		Histogram: bench.Histogram(bench.ResponseDurations(t.results)),
		Analysis:  &analysis,
		Requests:  byN,
	}
	t.version++
}

type analysisParams struct {
	maxTestDuration  time.Duration
	maxResponseBytes int64
}
