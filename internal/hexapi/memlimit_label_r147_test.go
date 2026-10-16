package hexapi

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestR147_MemLimitLabel(t *testing.T) {
	cases := []struct {
		name  string
		limit int64
		want  string
	}{
		{"unset is what runtime reports with no limit", math.MaxInt64, "unset"},
		{"zero or negative is not a limit", 0, "unset"},
		{"what .206 actually runs", 10 << 30, "10GiB"},
		{"the figure the line used to claim", 8e9, "7.5GiB"},
		{"whole GiB loses the trailing zero", 4 << 30, "4GiB"},
	}
	for _, c := range cases {
		if got := memLimitLabel(c.limit); got != c.want {
			t.Errorf("%s: memLimitLabel(%d) = %q, want %q", c.name, c.limit, got, c.want)
		}
	}
}

// The whole point of r147 is that 8GB was a claim nobody read. The real
// limit on .206 is 10GiB, so the label for the live value must not be
// the string the line used to print.
func TestR147_TheOldClaimIsNotTheNewAnswer(t *testing.T) {
	if got := memLimitLabel(10 << 30); strings.Contains(got, "8") {
		t.Fatalf("10GiB rendered as %q — still carrying the 8GB claim", got)
	}
}

// Derived source gate (r134/r141 pattern): a hardcoded memlimit in the
// grinder's startup line cannot drift back to the truth, because it is
// not reading anything. Nothing stops someone re-inlining one except a
// test that looks.
func TestR147_GrinderLineDoesNotHardcodeAMemLimit(t *testing.T) {
	src, err := os.ReadFile("showmatch.go")
	if err != nil {
		t.Fatalf("read showmatch.go: %v", err)
	}
	for _, bad := range []string{"memlimit=8GB", "memlimit=10GiB", "memlimit=8GiB"} {
		if strings.Contains(string(src), bad) {
			t.Errorf("showmatch.go hardcodes %q — it must render debug.SetMemoryLimit(-1)", bad)
		}
	}
	if !strings.Contains(string(src), "memLimitLabel(debug.SetMemoryLimit(-1))") {
		t.Error("the grinder startup line should render the live limit via memLimitLabel")
	}
}
