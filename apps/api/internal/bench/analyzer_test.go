package bench

import (
	"strings"
	"testing"
	"time"
)

// gen builds n results; f returns (durationMs, status) for request number i (1-based).
func gen(n int, f func(i int) (float64, int)) []RequestResult {
	out := make([]RequestResult, n)
	for i := range out {
		d, status := f(i + 1)
		r := RequestResult{N: i + 1, DurationMs: d, Status: status, OK: status != 0 && status < 400}
		if status == 0 {
			r.Error = ErrTimeout
		}
		out[i] = r
	}
	return out
}

func analyze(results []RequestResult, concurrency int, stop string) Analysis {
	return Analyze(AnalysisInput{
		Summary:          Summarize(results, len(results), 10*time.Second),
		Results:          results,
		Concurrency:      concurrency,
		Timeout:          10 * time.Second,
		MaxTestDuration:  2 * time.Minute,
		MaxResponseBytes: 5 << 20,
		ProbeRegion:      "test",
		StopReason:       stop,
	})
}

func find(a Analysis, id string) *Finding {
	for i := range a.Findings {
		if a.Findings[i].ID == id {
			return &a.Findings[i]
		}
	}
	return nil
}

func ids(a Analysis) []string {
	var out []string
	for _, f := range a.Findings {
		out = append(out, f.ID+":"+f.Severity)
	}
	return out
}

func TestAnalyzeHealthy(t *testing.T) {
	a := analyze(gen(50, func(i int) (float64, int) { return 80 + float64(i%5), 200 }), 5, StopCompleted)
	if a.Verdict != "healthy" || a.Headline != "Healthy: no problems detected" {
		t.Errorf("verdict=%s headline=%q findings=%v", a.Verdict, a.Headline, ids(a))
	}
	if find(a, "all_succeeded") == nil || find(a, "latency").Severity != SevGood {
		t.Errorf("findings = %v", ids(a))
	}
}

func TestAnalyzeTailLatency(t *testing.T) {
	results := gen(100, func(i int) (float64, int) {
		if i%10 == 0 {
			return 600, 200
		}
		return 100, 200
	})
	a := analyze(results, 5, StopCompleted)
	f := find(a, "tail_latency")
	if f == nil || a.Verdict != SevWarning || a.Headline != "Needs investigation: high tail latency" {
		t.Fatalf("verdict=%s headline=%q findings=%v", a.Verdict, a.Headline, ids(a))
	}
	if !strings.Contains(f.Observation, "P95 (600ms) is 6.0× the median (100ms)") {
		t.Errorf("observation = %q", f.Observation)
	}
	if find(a, "latency_rising") != nil || find(a, "cold_start") != nil {
		t.Errorf("unexpected findings %v", ids(a))
	}
}

func TestAnalyzeRateLimit(t *testing.T) {
	results := gen(100, func(i int) (float64, int) {
		if i > 80 {
			return 100, 429
		}
		return 100, 200
	})
	a := analyze(results, 5, StopCompleted)
	f := find(a, "rate_limited")
	if f == nil || !strings.Contains(f.Observation, "20 requests (20.0%)") || !strings.Contains(f.Observation, "#81") {
		t.Fatalf("rate limit finding = %+v (all: %v)", f, ids(a))
	}
	if find(a, "client_errors") != nil || find(a, "server_errors") != nil {
		t.Errorf("429 must not count as a generic client/server error: %v", ids(a))
	}
}

func TestAnalyzeServerErrors(t *testing.T) {
	a := analyze(gen(100, func(i int) (float64, int) {
		if i%10 == 0 {
			return 100, 500
		}
		return 100, 200
	}), 5, StopCompleted)
	f := find(a, "server_errors")
	if f == nil || f.Severity != SevCritical || a.Verdict != SevCritical || a.Headline != "Critical: server errors" {
		t.Fatalf("verdict=%s headline=%q findings=%v", a.Verdict, a.Headline, ids(a))
	}
	if !strings.Contains(f.Observation, "500 ×10") {
		t.Errorf("observation = %q", f.Observation)
	}
}

func TestAnalyzeNoResponses(t *testing.T) {
	a := analyze(gen(10, func(int) (float64, int) { return 10000, 0 }), 2, StopCompleted)
	f := find(a, "no_responses")
	if f == nil || a.Verdict != SevCritical || !strings.Contains(f.Observation, "timeout ×10") {
		t.Fatalf("findings = %v", ids(a))
	}
	if len(a.Findings) != 1 {
		t.Errorf("expected only no_responses, got %v", ids(a))
	}
}

func TestAnalyzeRisingLatency(t *testing.T) {
	a := analyze(gen(100, func(i int) (float64, int) { return 100 + float64(i)*3, 200 }), 5, StopCompleted)
	if find(a, "latency_rising") == nil {
		t.Errorf("findings = %v", ids(a))
	}
}

func TestAnalyzeColdStart(t *testing.T) {
	a := analyze(gen(50, func(i int) (float64, int) {
		if i <= 5 {
			return 900, 200
		}
		return 100, 200
	}), 5, StopCompleted)
	f := find(a, "cold_start")
	if f == nil || f.Severity != SevInfo || a.Verdict != SevInfo {
		t.Fatalf("verdict=%s findings=%v", a.Verdict, ids(a))
	}
	// The slow first wave inflates P95, but that is explained by cold start,
	// not reported as a separate tail-latency problem.
	if find(a, "tail_latency") != nil || !strings.Contains(f.Observation, "without them P95 is 1.0×") {
		t.Errorf("findings=%v observation=%q", ids(a), f.Observation)
	}

	// A real tail among warm requests is still reported alongside cold start.
	a = analyze(gen(100, func(i int) (float64, int) {
		if i <= 5 || i%10 == 0 {
			return 900, 200
		}
		return 100, 200
	}), 5, StopCompleted)
	if find(a, "tail_latency") == nil || find(a, "cold_start") == nil {
		t.Errorf("findings=%v", ids(a))
	}
}

func TestAnalyzeClientErrors(t *testing.T) {
	a := analyze(gen(30, func(int) (float64, int) { return 50, 401 }), 2, StopCompleted)
	f := find(a, "client_errors")
	if f == nil || f.Severity != SevWarning || f.Title != "Most requests were rejected" {
		t.Fatalf("findings = %v", ids(a))
	}
	if !strings.Contains(strings.Join(f.PossibleCauses, " "), "authentication") {
		t.Errorf("causes = %v", f.PossibleCauses)
	}
}

func TestAnalyzeNotes(t *testing.T) {
	a := analyze(gen(10, func(int) (float64, int) { return 50, 200 }), 2, StopCancelled)
	all := strings.Join(a.Notes, "\n")
	for _, want := range []string{"region: test", "Fewer than 20 responses", "cancelled after 10 of 10"} {
		if !strings.Contains(all, want) {
			t.Errorf("notes missing %q:\n%s", want, all)
		}
	}
}

// Observations must state facts only; speculation belongs in PossibleCauses.
func TestObservationsAreFacts(t *testing.T) {
	cases := [][]RequestResult{
		gen(100, func(i int) (float64, int) { return 100 + float64(i)*30, 200 }),
		gen(100, func(i int) (float64, int) {
			if i%7 == 0 {
				return 2000, 503
			}
			return 1500, 200
		}),
	}
	for _, results := range cases {
		for _, f := range analyze(results, 5, StopCompleted).Findings {
			low := strings.ToLower(f.Observation)
			for _, word := range []string{"database", "may ", "probably", "because", "likely"} {
				if strings.Contains(low, word) {
					t.Errorf("finding %s observation speculates (%q): %q", f.ID, word, f.Observation)
				}
			}
		}
	}
}
