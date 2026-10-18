package hat

import (
	"os"
	"path/filepath"
	"testing"
)

// r170 — a nil *SelfPlayManager must not panic on the sample path.
//
// Found by driving the real grinder game loop for the r170 seed work:
// the hexapi fixture builds a Showmatch without the init that assigns
// sm.selfPlay, so WriteEnrichedSamples ran on a nil receiver, wrote the
// file, and then panicked inside RecordSamples. The grinder's recover()
// swallowed it, so the gate printed a stack trace inside a test that
// still reported PASS. It was noted twenty-four rounds ago and never
// filed, because nobody had a reproduction.
//
// A live server assigns sm.selfPlay unconditionally during init
// (hexapi/showmatch.go:739), so production does not reach this. These
// tests pin the nil-safety anyway, because the call sites do not guard
// and the next fixture to be written will hit it again.

func TestR170_RecordSamplesOnNilManagerIsANoOpNotAPanic(t *testing.T) {
	var sp *SelfPlayManager
	sp.RecordSamples(25) // must not panic
}

// The write is the useful half and must still happen without a manager
// — guarding the whole method instead of the training trigger would
// silently drop every sample a fixture produces.
func TestR170_NilManagerStillWritesTheSamplesFile(t *testing.T) {
	var sp *SelfPlayManager
	path := filepath.Join(t.TempDir(), "samples.jsonl")

	err := sp.WriteEnrichedSamples(path, []PivotEnrichedSample{{}})
	if err != nil {
		t.Fatalf("WriteEnrichedSamples on a nil manager: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("samples file was not written: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("samples file is empty — the write half was skipped")
	}
}

// A real manager must still count, or the guard above has disabled
// training rather than made it nil-safe.
func TestR170_ARealManagerStillCountsSamples(t *testing.T) {
	sp := NewSelfPlayManager(DefaultSelfPlayConfig(t.TempDir()))
	before := sp.SampleCount()
	sp.RecordSamples(7)
	if got := sp.SampleCount() - before; got != 7 {
		t.Fatalf("SampleCount delta = %d, want 7 — the nil guard is swallowing real calls", got)
	}
}
