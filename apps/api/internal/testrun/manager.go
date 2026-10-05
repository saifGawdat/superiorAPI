package testrun

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"superiorapi/internal/bench"
)

// RateLimitError means a test was refused by an abuse limit. Its message is
// safe to show to the user.
type RateLimitError struct{ msg string }

func (e *RateLimitError) Error() string { return e.msg }

// Limits caps how much load the profiler generates and for how long results live.
type Limits struct {
	MaxActiveGlobal    int           // concurrent tests across all clients
	MaxActivePerClient int           // concurrent tests per client IP
	MaxActivePerTarget int           // concurrent tests against one host, across clients
	MaxTestsPerHour    int           // tests started per client IP per rolling hour
	MaxTestDuration    time.Duration // a test is stopped after this long
	ResultTTL          time.Duration // finished results are kept this long
}

// RunnerFactory builds a runner for a test and returns a cleanup function.
type RunnerFactory func(cfg bench.Config) (*bench.Runner, func())

// Manager owns all tests in memory.
type Manager struct {
	newRunner        RunnerFactory
	limits           Limits
	region           string
	maxResponseBytes int64
	now              func() time.Time

	mu             sync.Mutex
	tests          map[string]*Test
	active         int
	activeByClient map[string]int
	activeByTarget map[string]int
	history        map[string][]time.Time // test start times per client
	wg             sync.WaitGroup
}

// NewManager creates a manager.
func NewManager(newRunner RunnerFactory, limits Limits, region string, maxResponseBytes int64) *Manager {
	return &Manager{
		newRunner: newRunner, limits: limits, region: region, maxResponseBytes: maxResponseBytes,
		now:            time.Now,
		tests:          map[string]*Test{},
		activeByClient: map[string]int{},
		activeByTarget: map[string]int{},
		history:        map[string][]time.Time{},
	}
}

// Start validates abuse limits and launches a test in the background.
// targetHost should be the lowercase host name of the target URL.
func (m *Manager) Start(cfg bench.Config, clientKey, targetHost string) (*Test, error) {
	targetHost = strings.ToLower(targetHost)
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	recent := m.recentStarts(clientKey, now)
	switch {
	case m.activeByClient[clientKey] >= m.limits.MaxActivePerClient:
		return nil, &RateLimitError{"You already have a test running. Wait for it to finish or cancel it."}
	case len(recent) >= m.limits.MaxTestsPerHour:
		return nil, &RateLimitError{fmt.Sprintf("You have reached the limit of %d tests per hour. Try again later.", m.limits.MaxTestsPerHour)}
	case m.activeByTarget[targetHost] >= m.limits.MaxActivePerTarget:
		return nil, &RateLimitError{"Another test against this host is already running. Try again when it finishes."}
	case m.active >= m.limits.MaxActiveGlobal:
		return nil, &RateLimitError{"The profiler is busy right now. Try again in a minute."}
	}

	t := newTest(newID(), cfg, m.region, clientKey, targetHost, now)
	m.tests[t.ID] = t
	m.active++
	m.activeByClient[clientKey]++
	m.activeByTarget[targetHost]++
	m.history[clientKey] = append(recent, now)

	m.wg.Add(1)
	go m.run(t)
	return t, nil
}

func (m *Manager) run(t *Test) {
	defer m.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), m.limits.MaxTestDuration)
	defer cancel()
	go func() {
		select {
		case <-t.userCancel:
			cancel()
		case <-ctx.Done():
		}
	}()

	runner, cleanup := m.newRunner(t.cfg)
	start := time.Now()
	runner.Run(ctx, t.cfg, t.record)
	elapsed := time.Since(start)
	cleanup()

	stop := bench.StopCompleted
	select {
	case <-t.userCancel:
		stop = bench.StopCancelled
	default:
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			stop = bench.StopTimeLimit
		}
	}
	t.finish(elapsed, stop, analysisParams{m.limits.MaxTestDuration, m.maxResponseBytes})

	m.mu.Lock()
	m.active--
	decr(m.activeByClient, t.clientKey)
	decr(m.activeByTarget, t.targetHost)
	m.mu.Unlock()
	close(t.done)
}

// Get returns a test by ID.
func (m *Manager) Get(id string) (*Test, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tests[id]
	return t, ok
}

// Sweep removes finished tests older than the TTL and stale rate-limit history.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for id, t := range m.tests {
		t.mu.Lock()
		finishedAt := t.finishedAt
		t.mu.Unlock()
		if !finishedAt.IsZero() && now.Sub(finishedAt) > m.limits.ResultTTL {
			delete(m.tests, id)
		}
	}
	for client := range m.history {
		if len(m.recentStarts(client, now)) == 0 {
			delete(m.history, client)
		}
	}
}

// RunJanitor calls Sweep every interval until ctx is done.
func (m *Manager) RunJanitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Sweep()
		}
	}
}

// Shutdown cancels all running tests and waits for them to stop.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	for _, t := range m.tests {
		t.Cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

// recentStarts returns the client's test start times within the last hour.
// Callers must hold m.mu.
func (m *Manager) recentStarts(client string, now time.Time) []time.Time {
	starts := m.history[client]
	i := 0
	for i < len(starts) && now.Sub(starts[i]) >= time.Hour {
		i++
	}
	return starts[i:]
}

func decr(counts map[string]int, key string) {
	if counts[key] <= 1 {
		delete(counts, key)
		return
	}
	counts[key]--
}

var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// newID returns an unguessable 16-character ID (80 random bits).
func newID() string {
	b := make([]byte, 10)
	rand.Read(b)
	return idEncoding.EncodeToString(b)
}
