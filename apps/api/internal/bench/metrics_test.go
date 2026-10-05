package bench

import (
	"testing"
	"time"
)

func seq(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i + 1)
	}
	return out
}

func TestPercentileNearestRank(t *testing.T) {
	d := seq(100)
	for p, want := range map[float64]float64{1: 1, 50: 50, 90: 90, 95: 95, 99: 99, 100: 100} {
		if got := Percentile(d, p); got != want {
			t.Errorf("P%v of 1..100 = %v, want %v", p, got, want)
		}
	}
	d = seq(10)
	for p, want := range map[float64]float64{50: 5, 90: 9, 95: 10, 99: 10} {
		if got := Percentile(d, p); got != want {
			t.Errorf("P%v of 1..10 = %v, want %v", p, got, want)
		}
	}
	if Percentile([]float64{42}, 99) != 42 || Percentile(nil, 50) != 0 {
		t.Error("edge cases failed")
	}
}

func TestSummarize(t *testing.T) {
	results := []RequestResult{
		{N: 1, DurationMs: 100, Status: 200, OK: true, Bytes: 10},
		{N: 2, DurationMs: 300, Status: 200, OK: true, Bytes: 10},
		{N: 3, DurationMs: 200, Status: 500, OK: false, Bytes: 5},
		{N: 4, DurationMs: 5000, Status: 0, OK: false, Error: ErrTimeout},
	}
	s := Summarize(results, 10, 2*time.Second)

	if s.TotalRequests != 10 || s.CompletedRequests != 4 || s.SuccessfulRequests != 2 || s.FailedRequests != 2 {
		t.Errorf("counts wrong: %+v", s)
	}
	if s.ErrorRate != 0.5 || s.Throughput != 2 || s.DurationMs != 2000 || s.BytesReceived != 25 {
		t.Errorf("rates wrong: %+v", s)
	}
	if s.StatusCodes["200"] != 2 || s.StatusCodes["500"] != 1 || s.ErrorTypes[ErrTimeout] != 1 || len(s.StatusCodes) != 2 {
		t.Errorf("breakdown wrong: %v %v", s.StatusCodes, s.ErrorTypes)
	}
	// The timeout is excluded from latency stats.
	l := s.Latency
	if s.LatencySampleSize != 3 || l.Min != 100 || l.Max != 300 || l.Avg != 200 || l.P50 != 200 {
		t.Errorf("latency wrong: %+v", l)
	}
	if s.SlowestRequest == nil || s.SlowestRequest.N != 2 {
		t.Errorf("slowest wrong: %+v", s.SlowestRequest)
	}

	empty := Summarize([]RequestResult{{N: 1, Error: ErrDNS}}, 1, time.Second)
	if empty.Latency != nil || empty.SlowestRequest != nil || empty.ErrorRate != 1 {
		t.Errorf("no-response summary wrong: %+v", empty)
	}
}

func checkHistogram(t *testing.T, d []float64, b []Bucket) {
	t.Helper()
	total := 0
	for i, x := range b {
		total += x.Count
		if x.ToMs <= x.FromMs {
			t.Errorf("bucket %d empty range %+v", i, x)
		}
		if i > 0 && b[i-1].ToMs != x.FromMs {
			t.Errorf("buckets %d/%d not contiguous: %+v %+v", i-1, i, b[i-1], x)
		}
	}
	if total != len(d) {
		t.Errorf("histogram counts %d values, want %d", total, len(d))
	}
	if len(b) > 14 {
		t.Errorf("too many buckets: %d", len(b))
	}
	if b[0].FromMs > d[0] || b[len(b)-1].ToMs < d[len(d)-1] {
		t.Errorf("range [%v,%v] does not cover [%v,%v]", b[0].FromMs, b[len(b)-1].ToMs, d[0], d[len(d)-1])
	}
}

func TestHistogram(t *testing.T) {
	d := seq(100) // 1..100
	b := Histogram(d)
	checkHistogram(t, d, b)

	// Narrow cluster far from zero should not start at 0.
	narrow := []float64{801, 805, 810, 850, 899}
	b = Histogram(narrow)
	checkHistogram(t, narrow, b)
	if b[0].FromMs < 800 {
		t.Errorf("narrow cluster histogram starts at %v", b[0].FromMs)
	}

	// One huge outlier gets an overflow bucket instead of squashing the rest.
	outlier := append(seq(99), 10000)
	b = Histogram(outlier)
	checkHistogram(t, outlier, b)
	if last := b[len(b)-1]; last.Count != 1 || last.ToMs != 10000 {
		t.Errorf("expected overflow bucket with the outlier, got %+v", last)
	}
	if b[0].ToMs-b[0].FromMs > 20 {
		t.Errorf("outlier squashed the buckets: width %v", b[0].ToMs-b[0].FromMs)
	}

	same := []float64{5, 5, 5}
	checkHistogram(t, same, Histogram(same))
	if Histogram(nil) != nil {
		t.Error("empty histogram should be nil")
	}
}
