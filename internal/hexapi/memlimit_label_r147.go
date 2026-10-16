package hexapi

import (
	"fmt"
	"math"
	"strings"
)

// r147 — the grinder's startup line used to print the literal string
// "memlimit=8GB". It read nothing. The wrapper on .206 exports
// GOMEMLIMIT=10GiB, so the line had been reporting a number that was
// never true on that host, and the comment justifying maxProcessHeap
// cited the same wrong figure.
//
// This is the "two readers of one concept drift apart" class, in its
// cheapest form: one of the readers was a string literal, so it could
// not drift back. A limit worth printing is worth reading.
//
// memLimitLabel renders a GOMEMLIMIT byte count for the log line.
// math.MaxInt64 is what runtime/debug reports when no limit is set.
func memLimitLabel(limit int64) string {
	if limit <= 0 || limit == math.MaxInt64 {
		return "unset"
	}
	gib := float64(limit) / (1 << 30)
	s := fmt.Sprintf("%.1f", gib)
	s = strings.TrimSuffix(s, ".0")
	return s + "GiB"
}
