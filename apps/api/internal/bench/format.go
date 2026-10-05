package bench

import (
	"fmt"
	"slices"
	"unicode"
	"unicode/utf8"
)

// fmtMs formats milliseconds the way the UI does: "8.2ms", "281ms", "1.42s".
func fmtMs(v float64) string {
	switch {
	case v < 10:
		return fmt.Sprintf("%.1fms", v)
	case v < 1000:
		return fmt.Sprintf("%.0fms", v)
	}
	return fmt.Sprintf("%.2fs", v/1000)
}

func pct(share float64) string {
	return fmt.Sprintf("%.1f%%", share*100)
}

func bytesText(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func median(v []float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	return Percentile(s, 50)
}

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}
