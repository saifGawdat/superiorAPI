package bench

import (
	"math"
	"slices"
	"strconv"
	"time"
)

// Latency holds latency statistics in milliseconds.
type Latency struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// SlowestRequest identifies the slowest request that got an HTTP response.
type SlowestRequest struct {
	N          int     `json:"n"`
	DurationMs float64 `json:"durationMs"`
	Status     int     `json:"status"`
}

// Summary is the aggregate result of a benchmark.
type Summary struct {
	TotalRequests      int             `json:"totalRequests"`
	CompletedRequests  int             `json:"completedRequests"`
	SuccessfulRequests int             `json:"successfulRequests"`
	FailedRequests     int             `json:"failedRequests"`
	ErrorRate          float64         `json:"errorRate"` // fraction 0..1 of completed requests
	DurationMs         float64         `json:"durationMs"`
	Throughput         float64         `json:"throughput"` // completed requests per second
	Latency            *Latency        `json:"latency"`    // nil when no request got a response
	LatencySampleSize  int             `json:"latencySampleSize"`
	StatusCodes        map[string]int  `json:"statusCodes"`
	ErrorTypes         map[string]int  `json:"errorTypes"`
	BytesReceived      int64           `json:"bytesReceived"`
	SlowestRequest     *SlowestRequest `json:"slowestRequest"`
}

// Bucket is one histogram bin covering [FromMs, ToMs).
type Bucket struct {
	FromMs float64 `json:"fromMs"`
	ToMs   float64 `json:"toMs"`
	Count  int     `json:"count"`
}

// Percentile returns the p-th percentile (0 < p <= 100) of an ascending
// slice using the nearest-rank method, so the result is always an observed
// value. It returns 0 for an empty slice.
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(n)))
	return sorted[min(max(rank, 1), n)-1]
}

// ResponseDurations returns the sorted durations of requests that received an
// HTTP response. Transport errors are excluded: their duration is mostly the
// timeout or the time to fail, not the API's latency.
func ResponseDurations(results []RequestResult) []float64 {
	d := make([]float64, 0, len(results))
	for _, r := range results {
		if r.Status != 0 {
			d = append(d, r.DurationMs)
		}
	}
	slices.Sort(d)
	return d
}

// Summarize aggregates request results. total is the number of requests the
// test was configured to send; elapsed is the wall time of the test.
func Summarize(results []RequestResult, total int, elapsed time.Duration) Summary {
	s := Summary{
		TotalRequests:     total,
		CompletedRequests: len(results),
		DurationMs:        ms(elapsed),
		StatusCodes:       map[string]int{},
		ErrorTypes:        map[string]int{},
	}
	var slowest *RequestResult
	for i, r := range results {
		if r.OK {
			s.SuccessfulRequests++
		} else {
			s.FailedRequests++
		}
		if r.Status != 0 {
			s.StatusCodes[strconv.Itoa(r.Status)]++
			if slowest == nil || r.DurationMs > slowest.DurationMs {
				slowest = &results[i]
			}
		}
		if r.Error != "" {
			s.ErrorTypes[r.Error]++
		}
		s.BytesReceived += r.Bytes
	}
	if len(results) > 0 {
		s.ErrorRate = round(float64(s.FailedRequests)/float64(len(results)), 4)
	}
	if secs := elapsed.Seconds(); secs > 0 {
		s.Throughput = round(float64(len(results))/secs, 1)
	}
	if slowest != nil {
		s.SlowestRequest = &SlowestRequest{N: slowest.N, DurationMs: slowest.DurationMs, Status: slowest.Status}
	}

	d := ResponseDurations(results)
	s.LatencySampleSize = len(d)
	if len(d) > 0 {
		var sum float64
		for _, v := range d {
			sum += v
		}
		s.Latency = &Latency{
			Min: d[0],
			Max: d[len(d)-1],
			Avg: round(sum/float64(len(d)), 1),
			P50: Percentile(d, 50),
			P90: Percentile(d, 90),
			P95: Percentile(d, 95),
			P99: Percentile(d, 99),
		}
	}
	return s
}

// Histogram bins sorted latencies into roughly ten buckets of a "nice" width
// (1, 2, 2.5 or 5 × 10^n ms), starting near the minimum. When a few extreme
// outliers would squash the chart, values above P99 go into a single wider
// overflow bucket at the end.
func Histogram(sorted []float64) []Bucket {
	n := len(sorted)
	if n == 0 {
		return nil
	}
	lo, maxV := sorted[0], sorted[n-1]
	hi := maxV
	if n >= 20 {
		if p99 := Percentile(sorted, 99); maxV > p99*1.5 {
			hi = p99
		}
	}

	// Work in integer bucket indices so float rounding (0.3/0.1 = 2.9999…)
	// cannot produce empty, overlapping or zero-width buckets.
	width := niceStep((hi - lo) / 10)
	index := func(v float64) int { return int(math.Floor(v/width + 1e-9)) }
	edge := func(i int) float64 { return round(float64(i)*width, 2) }
	first := index(lo)
	k := index(hi) - first + 1

	buckets := make([]Bucket, k)
	for i := range buckets {
		buckets[i] = Bucket{FromMs: edge(first + i), ToMs: edge(first + i + 1)}
	}
	overflow := 0
	for _, v := range sorted {
		if i := index(v) - first; i < k {
			buckets[i].Count++
		} else {
			overflow++
		}
	}
	if overflow > 0 {
		if end := buckets[k-1].ToMs; maxV > end {
			buckets = append(buckets, Bucket{FromMs: end, ToMs: maxV, Count: overflow})
		} else {
			buckets[k-1].Count += overflow
		}
	}
	return buckets
}

func niceStep(raw float64) float64 {
	if raw <= 0.1 {
		return 0.1
	}
	exp := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, s := range []float64{1, 2, 2.5, 5, 10} {
		if raw/exp <= s {
			return s * exp
		}
	}
	return 10 * exp
}

// LivePercentiles returns P50 and P95 of the responses so far, or nils when
// no response has arrived yet.
func LivePercentiles(results []RequestResult) (p50, p95 *float64) {
	d := ResponseDurations(results)
	if len(d) == 0 {
		return nil, nil
	}
	a, b := Percentile(d, 50), Percentile(d, 95)
	return &a, &b
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
