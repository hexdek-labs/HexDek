package huginn

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// r151Obs builds n observations whose serialized length depends on n, so that
// concurrent writers produce documents of DIFFERENT lengths. That difference
// is the whole mechanism: with a shared tmp path the shorter document lands on
// top of the longer one and the longer one's tail survives past its end, which
// is precisely the live file found on .206 (a complete array closing at line
// 25697 of 32695, then the middle of a longer document).
func r151Obs(n int) []RawObservation {
	out := make([]RawObservation, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, RawObservation{
			CardA:         "Lotus Cobra",
			CardB:         "Scute Swarm",
			ImpactScore:   1.0,
			TurnWindow:    0,
			EffectPattern: "Lotus Cobra produces synergy, Scute Swarm consumes synergy",
			DeckNames:     []string{"moxfield/a", "moxfield/b", "moxfield/c", "moxfield/d"},
			Timestamp:     "2026-10-16T20:15:21Z",
		})
	}
	return out
}

// Direction 1 — concurrent writers leave a PARSEABLE file. Two grinder workers
// both reach this writer in production.
func TestAtomicWriteJSONR151_ConcurrentWritersLeaveAParseableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw_observations.json")

	sizes := []int{4000, 40, 2500, 90, 1200, 25}
	var wg sync.WaitGroup
	for _, n := range sizes {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 12; i++ {
				if err := atomicWriteJSON(path, r151Obs(n)); err != nil {
					t.Errorf("write %d: %v", n, err)
					return
				}
			}
		}(n)
	}
	wg.Wait()

	var out []RawObservation
	if err := readJSON(path, &out); err != nil {
		t.Fatalf("file left unparseable by concurrent writers: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("file parsed but empty")
	}
	// The surviving document must be exactly one of the written documents,
	// not a splice of two.
	ok := false
	for _, n := range sizes {
		if len(out) == n {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("surviving document is a splice: %d records, none of %v", len(out), sizes)
	}
}

// Direction 2 — no stray temp files are left behind.
func TestAtomicWriteJSONR151_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw_observations.json")
	for i := 0; i < 8; i++ {
		if err := atomicWriteJSON(path, r151Obs(10)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range ents {
		if e.Name() != "raw_observations.json" {
			t.Fatalf("stray file left behind: %s", e.Name())
		}
	}
}

// CONTROL — the pre-r151 writer, verbatim, must go RED on the same workload.
// If this ever passes, the test above is not exercising the race.
//
// The mechanism needs both O_TRUNC opens to happen BEFORE the larger write
// completes: writer A opens (truncate), writer B opens (truncate), A writes
// 1MB, B writes 10KB at ITS OWN offset 0 — leaving B's complete short document
// followed by the tail of A's long one. Marshalling inside the loop
// desynchronised the goroutines enough to hide it, so the payloads are
// pre-marshalled and the writers start on a barrier.
func r151SharedTmpWriteBytes(path string, out []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func TestAtomicWriteJSONR151_ControlSharedTmpPathCorrupts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw_observations.json")

	bigN, smallN := 6000, 20
	big, err := marshalIndentForTest(r151Obs(bigN))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	small, err := marshalIndentForTest(r151Obs(smallN))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	corrupt := false
	for round := 0; round < 400 && !corrupt; round++ {
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, payload := range [][]byte{big, small} {
			wg.Add(1)
			go func(p []byte) {
				defer wg.Done()
				<-start
				_ = r151SharedTmpWriteBytes(path, p)
			}(payload)
		}
		close(start)
		wg.Wait()

		var out []RawObservation
		if err := readJSON(path, &out); err != nil {
			corrupt = true
			t.Logf("round %d: control corrupted the file as expected: %v", round, err)
			break
		}
		if len(out) != bigN && len(out) != smallN {
			corrupt = true
			t.Logf("round %d: control spliced two documents as expected: %d records", round, len(out))
		}
	}
	if !corrupt {
		t.Fatalf("CONTROL DID NOT GO RED: the shared-tmp writer survived the workload, so the race test above proves nothing")
	}
}
