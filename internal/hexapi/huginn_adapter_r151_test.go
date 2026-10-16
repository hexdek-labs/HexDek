package hexapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/huginn"
)

func r151Pairs(n int) []heimdall.CoTriggerPair {
	out := make([]heimdall.CoTriggerPair, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, heimdall.CoTriggerPair{
			CardA: "Lotus Cobra", CardB: "Scute Swarm", ImpactScore: 1.0, TurnWindow: 0,
		})
	}
	return out
}

func r151Decks() []string {
	return []string{"moxfield/a", "moxfield/b", "moxfield/c", "moxfield/d"}
}

func r151ObsPath(dir string) string {
	return filepath.Join(dir, "huginn", "raw_observations.json")
}

// Direction 1 — a game's worth of observations does NOT touch disk. Before
// r151 every single game read the whole history back and wrote it out again.
func TestHuginnAdapterR151_SmallBatchDoesNotTouchDisk(t *testing.T) {
	dir := t.TempDir()
	a := &huginnAdapter{dataDir: dir}

	a.IngestCoTriggers(r151Pairs(10), r151Decks())

	if _, err := os.Stat(r151ObsPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("adapter wrote to disk for a 10-pair batch (err=%v)", err)
	}
}

// Direction 2 — it DOES flush once the buffer fills, and everything buffered
// comes back.
func TestHuginnAdapterR151_FlushesOnceTheBufferFills(t *testing.T) {
	dir := t.TempDir()
	a := &huginnAdapter{dataDir: dir}

	for i := 0; i < huginnObsFlushEvery+10; i++ {
		a.IngestCoTriggers(r151Pairs(1), r151Decks())
	}

	got, err := huginn.ReadRawObservations(filepath.Join(dir, "huginn"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(got) != huginnObsFlushEvery {
		t.Fatalf("persisted: want %d, got %d", huginnObsFlushEvery, len(got))
	}
	if got[0].CardA != "Lotus Cobra" || got[0].CardB != "Scute Swarm" {
		t.Fatalf("observation mangled: %+v", got[0])
	}
	if got[0].EffectPattern == "" {
		t.Fatalf("effect pattern not synthesized")
	}
}

// A corrupt history must NOT silently eat the batch. This is the live .206
// failure: unparseable since 2026-06-07, four months of observations read,
// failed and dropped, with nothing logged.
func TestHuginnAdapterR151_CorruptHistoryDoesNotEatTheBatch(t *testing.T) {
	dir := t.TempDir()
	hdir := filepath.Join(dir, "huginn")
	if err := os.MkdirAll(hdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// The exact shape of the live file: a complete array, then the tail of a
	// longer document.
	corrupt := "[\n  {\n    \"card_a\": \"A\",\n    \"card_b\": \"B\"\n  }\n]\nBL\",\n      \"x\"\n    ]\n  }\n]\n"
	if err := os.WriteFile(r151ObsPath(dir), []byte(corrupt), 0o644); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	// Precondition: the seeded file really is unparseable, or this test is
	// vacuous.
	if _, err := huginn.ReadRawObservations(hdir); err == nil {
		t.Fatalf("precondition: seeded file parsed cleanly, test would be vacuous")
	}

	a := &huginnAdapter{dataDir: dir}
	for i := 0; i < huginnObsFlushEvery; i++ {
		a.IngestCoTriggers(r151Pairs(1), r151Decks())
	}

	got, err := huginn.ReadRawObservations(hdir)
	if err != nil {
		t.Fatalf("history still unparseable after a flush: %v", err)
	}
	if len(got) != huginnObsFlushEvery {
		t.Fatalf("batch lost to a corrupt history: want %d, got %d", huginnObsFlushEvery, len(got))
	}
}

// The on-disk history must stay bounded, or the read-modify-write grows
// without limit again — the slow-motion version of the thing being fixed.
func TestHuginnAdapterR151_HistoryIsCapped(t *testing.T) {
	dir := t.TempDir()
	a := &huginnAdapter{dataDir: dir}

	batches := (huginnObsCap / huginnObsFlushEvery) + 3
	for b := 0; b < batches; b++ {
		a.IngestCoTriggers(r151Pairs(huginnObsFlushEvery), r151Decks())
	}

	got, err := huginn.ReadRawObservations(filepath.Join(dir, "huginn"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(got) > huginnObsCap {
		t.Fatalf("history exceeded the cap: want <= %d, got %d", huginnObsCap, len(got))
	}
	if len(got) != huginnObsCap {
		t.Fatalf("expected the cap to be reached exactly: want %d, got %d", huginnObsCap, len(got))
	}
}
