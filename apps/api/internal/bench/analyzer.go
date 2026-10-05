package bench

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Finding severities, ordered from best to worst.
const (
	SevGood     = "good"
	SevInfo     = "info"
	SevWarning  = "warning"
	SevCritical = "critical"
)

var severityRank = map[string]int{SevGood: 0, SevInfo: 1, SevWarning: 2, SevCritical: 3}

// Finding is one interpreted pattern. Observation states only measured facts;
// PossibleCauses are hypotheses a black-box test cannot confirm.
type Finding struct {
	ID             string   `json:"id"`
	Severity       string   `json:"severity"`
	Title          string   `json:"title"`
	Observation    string   `json:"observation"`
	PossibleCauses []string `json:"possibleCauses"`
	Suggestion     string   `json:"suggestion,omitempty"`
}

// Analysis is the interpreted verdict of a benchmark.
type Analysis struct {
	Verdict  string    `json:"verdict"` // healthy | info | warning | critical
	Headline string    `json:"headline"`
	Findings []Finding `json:"findings"`
	Notes    []string  `json:"notes"`
}

// Stop reasons passed to Analyze.
const (
	StopCompleted = ""
	StopCancelled = "cancelled"
	StopTimeLimit = "time_limit"
)

// AnalysisInput is everything the analyzer looks at.
type AnalysisInput struct {
	Summary          Summary
	Results          []RequestResult
	Concurrency      int
	Timeout          time.Duration
	MaxTestDuration  time.Duration
	MaxResponseBytes int64
	ProbeRegion      string
	StopReason       string
}

// Analyze applies rule-based checks to a finished (or stopped) benchmark.
func Analyze(in AnalysisInput) Analysis {
	s := in.Summary
	byN := slices.Clone(in.Results)
	slices.SortFunc(byN, func(a, b RequestResult) int { return a.N - b.N })

	var findings []Finding
	add := func(f *Finding) {
		if f != nil {
			findings = append(findings, *f)
		}
	}
	add(noResponses(s, in.Timeout))
	if s.Latency != nil {
		add(transportErrors(s, in.Timeout))
		add(rateLimited(s, byN))
		add(serverErrors(s))
		add(clientErrors(s))
		add(latencyLevel(s, in.ProbeRegion))
		cold := coldStart(byN, s, in.Concurrency)
		add(tailLatency(s, byN, in.Concurrency, cold))
		add(latencyTrend(byN, in.Concurrency))
		add(cold)
	}
	if s.CompletedRequests > 0 && s.FailedRequests == 0 {
		add(&Finding{
			ID: "all_succeeded", Severity: SevGood, Title: "All requests succeeded",
			Observation: fmt.Sprintf("All %d requests returned a successful (2xx/3xx) response.", s.CompletedRequests),
		})
	}
	slices.SortStableFunc(findings, func(a, b Finding) int { return severityRank[b.Severity] - severityRank[a.Severity] })
	if findings == nil {
		findings = []Finding{}
	}

	return Analysis{
		Verdict:  verdict(findings),
		Headline: headline(findings),
		Findings: findings,
		Notes:    notes(in),
	}
}

func verdict(findings []Finding) string {
	if len(findings) == 0 || findings[0].Severity == SevGood {
		return "healthy"
	}
	return findings[0].Severity
}

func headline(findings []Finding) string {
	if len(findings) == 0 {
		return "No requests completed"
	}
	top := findings[0]
	switch top.Severity {
	case SevCritical:
		return "Critical: " + lowerFirst(top.Title)
	case SevWarning:
		return "Needs investigation: " + lowerFirst(top.Title)
	case SevInfo:
		return "Mostly healthy: " + lowerFirst(top.Title)
	}
	return "Healthy: no problems detected"
}

func noResponses(s Summary, timeout time.Duration) *Finding {
	if s.Latency != nil || s.CompletedRequests == 0 {
		return nil
	}
	return &Finding{
		ID: "no_responses", Severity: SevCritical, Title: "No HTTP responses",
		Observation: fmt.Sprintf("None of the %d completed requests received an HTTP response (%s).",
			s.CompletedRequests, countsText(s.ErrorTypes, nil)),
		PossibleCauses: append(errorTypeCauses(s.ErrorTypes, timeout),
			"The API is down, or the host name or port is wrong",
			"A firewall or WAF blocks traffic from the profiler server"),
		Suggestion: "Check that the URL works from a public network. If requests time out, try a longer timeout.",
	}
}

func transportErrors(s Summary, timeout time.Duration) *Finding {
	n := 0
	for _, c := range s.ErrorTypes {
		n += c
	}
	// Body-read errors can carry a status code; count only requests with no response.
	n = min(n, s.CompletedRequests-s.LatencySampleSize)
	if n == 0 {
		return nil
	}
	share := float64(n) / float64(s.CompletedRequests)
	return &Finding{
		ID: "transport_errors", Severity: severityFor(share, 0.05), Title: "Requests failed without a response",
		Observation: fmt.Sprintf("%d of %d requests (%s) got no HTTP response: %s.",
			n, s.CompletedRequests, pct(share), countsText(s.ErrorTypes, nil)),
		PossibleCauses: errorTypeCauses(s.ErrorTypes, timeout),
		Suggestion:     "Check whether failures cluster at a point in the test, and compare with server and load balancer logs.",
	}
}

func rateLimited(s Summary, byN []RequestResult) *Finding {
	n := s.StatusCodes["429"]
	if n == 0 {
		return nil
	}
	first := 0
	for _, r := range byN {
		if r.Status == 429 {
			first = r.N
			break
		}
	}
	share := float64(n) / float64(s.CompletedRequests)
	return &Finding{
		ID: "rate_limited", Severity: SevWarning, Title: "Rate limiting detected",
		Observation: fmt.Sprintf("%d requests (%s) returned HTTP 429 Too Many Requests. The first one was request #%d.",
			n, pct(share), first),
		PossibleCauses: []string{
			fmt.Sprintf("The API enforces a rate limit below this test's rate (%.1f req/s)", s.Throughput),
			"A CDN or WAF throttling repeated requests from a single IP",
		},
		Suggestion: "Lower the concurrency, or check the API's rate-limit headers (Retry-After, X-RateLimit-*).",
	}
}

func serverErrors(s Summary) *Finding {
	n := countWhere(s.StatusCodes, func(c int) bool { return c >= 500 })
	if n == 0 {
		return nil
	}
	share := float64(n) / float64(s.CompletedRequests)
	causes := []string{}
	if s.StatusCodes["500"] > 0 {
		causes = append(causes, "Unhandled errors in the application for some requests")
	}
	if s.StatusCodes["502"]+s.StatusCodes["504"] > 0 {
		causes = append(causes, "A gateway or load balancer did not get a timely or valid response from an upstream service")
	}
	if s.StatusCodes["503"] > 0 {
		causes = append(causes, "The service was overloaded or temporarily unavailable")
	}
	causes = append(causes, "Failures that only appear under concurrent load (timeouts, exhausted connection pools)")
	return &Finding{
		ID: "server_errors", Severity: severityFor(share, 0.05), Title: "Server errors",
		Observation: fmt.Sprintf("%d requests (%s) returned server errors: %s.",
			n, pct(share), countsText(s.StatusCodes, func(c int) bool { return c >= 500 })),
		PossibleCauses: causes,
		Suggestion:     "Check the server's logs for the time of the test.",
	}
}

func clientErrors(s Summary) *Finding {
	is4xx := func(c int) bool { return c >= 400 && c < 500 && c != 429 }
	n := countWhere(s.StatusCodes, is4xx)
	if n == 0 {
		return nil
	}
	share := float64(n) / float64(s.LatencySampleSize)
	var causes []string
	if s.StatusCodes["401"]+s.StatusCodes["403"] > 0 {
		causes = append(causes, "Missing or invalid authentication headers")
	}
	if s.StatusCodes["404"] > 0 {
		causes = append(causes, "The URL path is wrong or the resource does not exist")
	}
	if s.StatusCodes["400"]+s.StatusCodes["422"] > 0 {
		causes = append(causes, "The request body or parameters are invalid")
	}
	if s.StatusCodes["405"] > 0 {
		causes = append(causes, "This endpoint does not support the selected method")
	}
	f := &Finding{
		ID: "client_errors", Severity: SevInfo, Title: "Some requests were rejected",
		Observation: fmt.Sprintf("%d responses (%s) were client errors: %s.",
			n, pct(share), countsText(s.StatusCodes, is4xx)),
		PossibleCauses: causes,
		Suggestion:     "Fix the request first. Error responses are often much faster or slower than real ones, so the latency numbers may not reflect real performance.",
	}
	if share >= 0.5 {
		f.Severity, f.Title = SevWarning, "Most requests were rejected"
	}
	return f
}

func latencyLevel(s Summary, region string) *Finding {
	p50 := s.Latency.P50
	causes := []string{
		"Slow database queries or missing indexes",
		"Calls to slow external services",
		"Heavy computation or serialization",
		fmt.Sprintf("Network distance between the profiler (region: %s) and the API", region),
	}
	if s.LatencySampleSize > 0 && s.BytesReceived/int64(s.LatencySampleSize) > 512<<10 {
		causes = append(causes, fmt.Sprintf("Large responses (%s on average)", bytesText(s.BytesReceived/int64(s.LatencySampleSize))))
	}
	suggestion := "Compare with server-side timings. If the server reports much less time, the difference is network distance or payload transfer."
	obs := fmt.Sprintf("Median latency is %s (average %s).", fmtMs(p50), fmtMs(s.Latency.Avg))
	switch {
	case p50 >= 3000:
		return &Finding{ID: "latency", Severity: SevCritical, Title: "Very slow responses", Observation: obs, PossibleCauses: causes, Suggestion: suggestion}
	case p50 >= 1000:
		return &Finding{ID: "latency", Severity: SevWarning, Title: "Slow responses", Observation: obs, PossibleCauses: causes, Suggestion: suggestion}
	case p50 >= 400:
		return &Finding{ID: "latency", Severity: SevInfo, Title: "Moderate latency", Observation: obs, PossibleCauses: causes, Suggestion: suggestion}
	}
	return &Finding{ID: "latency", Severity: SevGood, Title: "Fast responses", Observation: obs, PossibleCauses: []string{}}
}

func highTail(p50, p95 float64) bool {
	return p50 > 0 && p95 >= 2.5*p50 && p95-p50 >= 50
}

// tailLatency flags a P95 far above the median. When the tail disappears once
// the first wave of requests (new connections) is excluded, the cold-start
// finding explains it instead and this returns nil.
func tailLatency(s Summary, byN []RequestResult, concurrency int, cold *Finding) *Finding {
	l := s.Latency
	if s.LatencySampleSize < 20 || !highTail(l.P50, l.P95) {
		return nil
	}
	if cold != nil {
		if warm := warmDurations(byN, concurrency); len(warm) >= 20 {
			wp50, wp95 := Percentile(warm, 50), Percentile(warm, 95)
			if !highTail(wp50, wp95) {
				cold.Observation += fmt.Sprintf(" They raise P95 to %.1f× the median; without them P95 is %.1f× the median (%s).",
					l.P95/l.P50, wp95/wp50, fmtMs(wp95))
				return nil
			}
		}
	}
	obs := fmt.Sprintf("P95 (%s) is %.1f× the median (%s).", fmtMs(l.P95), l.P95/l.P50, fmtMs(l.P50))
	if s.LatencySampleSize >= 100 && l.P99 >= 5*l.P50 {
		obs += fmt.Sprintf(" P99 (%s) is %.1f× the median.", fmtMs(l.P99), l.P99/l.P50)
	}
	return &Finding{
		ID: "tail_latency", Severity: SevWarning, Title: "High tail latency", Observation: obs,
		PossibleCauses: []string{
			"Intermittent slow dependencies (database, cache misses, external APIs)",
			"Garbage collection pauses or resource contention",
			"Uneven load across server instances",
			"Queueing when concurrent requests exceed what the server handles in parallel",
		},
		Suggestion: "Look up the slowest requests in server logs or traces to see what they have in common.",
	}
}

// latencyTrend compares the median latency of the first and last quarter of
// requests (by request number), skipping the first wave of requests that
// paid for new connections.
func latencyTrend(byN []RequestResult, concurrency int) *Finding {
	var d []float64 // in request order, not sorted
	for _, r := range byN {
		if r.Status != 0 && r.N > concurrency {
			d = append(d, r.DurationMs)
		}
	}
	if len(d) < 20 {
		return nil
	}
	q := len(d) / 4
	first, last := median(d[:q]), median(d[len(d)-q:])
	if first <= 0 {
		return nil
	}
	ratio := last / first
	obs := fmt.Sprintf("Median latency went from %s in the first quarter of requests to %s in the last quarter (%.1f×).",
		fmtMs(first), fmtMs(last), ratio)
	switch {
	case ratio >= 1.5 && last-first >= 50:
		return &Finding{
			ID: "latency_rising", Severity: SevWarning, Title: "Latency rose during the test", Observation: obs,
			PossibleCauses: []string{
				"Requests queueing because the server cannot keep up with this concurrency",
				"Throttling that starts after a burst of requests",
				"Resource exhaustion under sustained load (connection pools, memory, CPU)",
			},
			Suggestion: "Run again with lower concurrency. If latency stays flat, the API is saturating at this load.",
		}
	case ratio <= 0.6 && first-last >= 50:
		return &Finding{
			ID: "latency_falling", Severity: SevInfo, Title: "Latency dropped during the test", Observation: obs,
			PossibleCauses: []string{
				"Caches warming up",
				"Lazy initialization or JIT compilation on the server",
				"Autoscaling adding capacity during the test",
			},
		}
	}
	return nil
}

// coldStart flags when the first wave of requests (one per worker, each on a
// new connection) is much slower than the overall median.
func coldStart(byN []RequestResult, s Summary, concurrency int) *Finding {
	var first []float64
	for _, r := range byN {
		if r.N > concurrency {
			break
		}
		if r.Status != 0 {
			first = append(first, r.DurationMs)
		}
	}
	if s.LatencySampleSize < 10 || len(first) == 0 || s.LatencySampleSize <= len(first) {
		return nil
	}
	m, p50 := median(first), s.Latency.P50
	if m < 2*p50 || m-p50 < 100 {
		return nil
	}
	return &Finding{
		ID: "cold_start", Severity: SevInfo, Title: "First requests were slower",
		Observation: fmt.Sprintf("The first %d requests had a median of %s, compared with %s overall.", len(first), fmtMs(m), fmtMs(p50)),
		PossibleCauses: []string{
			"New TCP and TLS connections: each concurrent worker opens one connection, and later requests reuse it",
			"Cold start of a serverless function or container",
			"Empty caches on the server",
		},
	}
}

// warmDurations returns sorted durations of responses after the first wave.
func warmDurations(byN []RequestResult, concurrency int) []float64 {
	var d []float64
	for _, r := range byN {
		if r.Status != 0 && r.N > concurrency {
			d = append(d, r.DurationMs)
		}
	}
	slices.Sort(d)
	return d
}

func notes(in AnalysisInput) []string {
	s := in.Summary
	out := []string{
		fmt.Sprintf("Latency was measured from the profiler server (region: %s) and includes the network path from there to the API.", in.ProbeRegion),
		fmt.Sprintf("Requests were sent by %d concurrent worker(s), each reusing its connection (keep-alive).", in.Concurrency),
	}
	switch {
	case s.LatencySampleSize > 0 && s.LatencySampleSize < 20:
		out = append(out, "Fewer than 20 responses were measured, so P90, P95 and P99 are not meaningful.")
	case s.LatencySampleSize >= 20 && s.LatencySampleSize < 100:
		out = append(out, "P99 is based on fewer than 100 responses and is approximate.")
	}
	switch in.StopReason {
	case StopCancelled:
		out = append(out, fmt.Sprintf("The test was cancelled after %d of %d requests. Results cover completed requests only.", s.CompletedRequests, s.TotalRequests))
	case StopTimeLimit:
		out = append(out, fmt.Sprintf("The test reached the %s time limit after %d of %d requests. Results cover completed requests only.",
			in.MaxTestDuration, s.CompletedRequests, s.TotalRequests))
	}
	for _, r := range in.Results {
		if r.Truncated {
			out = append(out, fmt.Sprintf("Some responses were larger than %s and were only partially read, so their durations are understated.", bytesText(in.MaxResponseBytes)))
			break
		}
	}
	return out
}

func errorTypeCauses(types map[string]int, timeout time.Duration) []string {
	var causes []string
	if types[ErrTimeout] > 0 {
		causes = append(causes, fmt.Sprintf("Some responses took longer than the %s timeout", timeout))
	}
	if types[ErrConnRefused]+types[ErrConnReset] > 0 {
		causes = append(causes, "The server refused or dropped connections (connection limits, crashes or restarts)")
	}
	if types[ErrTLS] > 0 {
		causes = append(causes, "TLS certificate or handshake problems")
	}
	if types[ErrDNS] > 0 {
		causes = append(causes, "DNS resolution failures")
	}
	if types[ErrBlocked] > 0 {
		causes = append(causes, "The API redirected to, or resolved to, a private address that the profiler refuses to contact")
	}
	if types[ErrTooManyRedirects] > 0 {
		causes = append(causes, "A redirect loop or a long redirect chain")
	}
	return causes
}

func severityFor(share, criticalAt float64) string {
	if share >= criticalAt {
		return SevCritical
	}
	return SevWarning
}

func countWhere(codes map[string]int, keep func(int) bool) int {
	n := 0
	for k, c := range codes {
		if code, err := strconv.Atoi(k); err == nil && keep(code) {
			n += c
		}
	}
	return n
}

// countsText renders counts like "500 ×2, 503 ×1", largest first. keep filters
// numeric keys; nil keeps everything.
func countsText(counts map[string]int, keep func(int) bool) string {
	type kv struct {
		k string
		c int
	}
	var items []kv
	for k, c := range counts {
		if keep != nil {
			code, err := strconv.Atoi(k)
			if err != nil || !keep(code) {
				continue
			}
		}
		items = append(items, kv{k, c})
	}
	slices.SortFunc(items, func(a, b kv) int {
		if a.c != b.c {
			return b.c - a.c
		}
		return strings.Compare(a.k, b.k)
	})
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = fmt.Sprintf("%s ×%d", it.k, it.c)
	}
	return strings.Join(parts, ", ")
}
