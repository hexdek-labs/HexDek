package heimdall

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// seed_binary_r60_test pins the binary-seed pack/unpack pipeline used
// by the Heimdall game-seed archive. The 28-byte record format is a
// stable contract — a regression would corrupt every existing seed
// file on disk. Targets the audit-flagged 0% file with round-trip
// invariants and per-piece boundary checks.

// TestEncodeDecode_RoundTripPreservesGameSeed verifies the full
// Encode→Decode pipeline restores every field bit-for-bit. The
// DeckIndex is shared across both halves so the deck-hash lookup
// works (Encode registers, Decode resolves).
func TestEncodeDecode_RoundTripPreservesGameSeed(t *testing.T) {
	di := NewDeckIndex(filepath.Join(t.TempDir(), "index.json"))
	original := GameSeed{
		RNGSeed:    0x1234567890ABCDEF,
		DeckKeys:   [4]string{"atraxa-v1", "voja-tribal", "korvold-aristo", "ur-dragon"},
		Winner:     2,
		Turns:      42,
		KillMethod: "combo",
	}
	encoded := EncodeSeed(original, di)
	if len(encoded) != SeedRecordSize {
		t.Fatalf("encoded length = %d, want %d", len(encoded), SeedRecordSize)
	}
	decoded := DecodeSeed(encoded, di)
	if !reflect.DeepEqual(decoded.DeckKeys, original.DeckKeys) {
		t.Errorf("DeckKeys lost: got %v, want %v", decoded.DeckKeys, original.DeckKeys)
	}
	if decoded.RNGSeed != original.RNGSeed {
		t.Errorf("RNGSeed lost: got %x, want %x", decoded.RNGSeed, original.RNGSeed)
	}
	if decoded.Winner != original.Winner {
		t.Errorf("Winner lost: got %d, want %d", decoded.Winner, original.Winner)
	}
	if decoded.Turns != original.Turns {
		t.Errorf("Turns lost: got %d, want %d", decoded.Turns, original.Turns)
	}
	if decoded.KillMethod != original.KillMethod {
		t.Errorf("KillMethod lost: got %q, want %q", decoded.KillMethod, original.KillMethod)
	}
}

// TestEncodeSeed_NoWinnerSentinel pins the no-winner encoding: a
// Winner outside [0, 3] (typically -1 for draws) packs as the 0xFF
// sentinel byte. On decode, 0xFF maps back to -1. This is the
// canonical "draw / no winner" signal in the binary format.
func TestEncodeSeed_NoWinnerSentinel(t *testing.T) {
	di := NewDeckIndex(filepath.Join(t.TempDir(), "index.json"))
	cases := []int{-1, -5, 4, 99} // all out of [0,3] range
	for _, w := range cases {
		seed := GameSeed{Winner: w, KillMethod: "combat"}
		buf := EncodeSeed(seed, di)
		if buf[24] != 0xFF {
			t.Errorf("Winner=%d should encode as 0xFF, got 0x%02X", w, buf[24])
		}
		decoded := DecodeSeed(buf, di)
		if decoded.Winner != -1 {
			t.Errorf("Winner=%d should decode to -1 (canonical no-winner), got %d", w, decoded.Winner)
		}
	}
}

// TestKillMethodEnumRoundTrip pins the kill-method enum mapping: each
// known method has a bidirectional string↔uint8 mapping, and unknown
// strings decode to KillUnknown ("unknown") rather than panicking or
// silently being lost.
func TestKillMethodEnumRoundTrip(t *testing.T) {
	known := []string{"combat", "commander", "combo", "mill", "poison", "timeout"}
	for _, s := range known {
		e := killMethodEnum(s)
		if killMethodString(e) != s {
			t.Errorf("round-trip broken for %q: got %q", s, killMethodString(e))
		}
	}
	// Unknown method
	if got := killMethodEnum("not-a-real-method"); got != KillUnknown {
		t.Errorf("unknown method should map to KillUnknown=%d, got %d", KillUnknown, got)
	}
	// Out-of-range enum value decodes to "unknown" (defensive)
	if got := killMethodString(99); got != "unknown" {
		t.Errorf("out-of-range enum 99 should decode to 'unknown', got %q", got)
	}
}

// TestDeckKeyHash_Deterministic verifies hashing is stable — same key
// produces the same hash across runs. Without this, deck-index lookup
// would fail across process restarts.
func TestDeckKeyHash_Deterministic(t *testing.T) {
	h1 := DeckKeyHash("atraxa-praetors-voice-v3")
	h2 := DeckKeyHash("atraxa-praetors-voice-v3")
	if h1 != h2 {
		t.Errorf("DeckKeyHash not deterministic: %08x vs %08x", h1, h2)
	}
	// Different keys produce different hashes (with very high probability).
	if DeckKeyHash("atraxa") == DeckKeyHash("voja") {
		t.Errorf("DeckKeyHash collision on different inputs")
	}
}

// TestDeckIndex_RegisterAndResolve pins the DeckIndex round-trip:
// Register returns a hash, Resolve maps that hash back to the original
// key. Registering twice for the same key is idempotent.
func TestDeckIndex_RegisterAndResolve(t *testing.T) {
	di := NewDeckIndex(filepath.Join(t.TempDir(), "index.json"))
	h := di.Register("test-deck-key")
	if got := di.Resolve(h); got != "test-deck-key" {
		t.Errorf("Resolve(%08x) = %q, want %q", h, got, "test-deck-key")
	}
	// Re-register same key: idempotent, same hash.
	if h2 := di.Register("test-deck-key"); h2 != h {
		t.Errorf("re-register changed hash: %08x vs %08x", h2, h)
	}
}

// TestDeckIndex_ResolveUnknownReturnsEmpty pins the unknown-hash path:
// resolving a hash that was never registered returns the empty string
// (not a panic, not a "missing" sentinel).
func TestDeckIndex_ResolveUnknownReturnsEmpty(t *testing.T) {
	di := NewDeckIndex(filepath.Join(t.TempDir(), "index.json"))
	if got := di.Resolve(0xDEADBEEF); got != "" {
		t.Errorf("unknown hash should resolve to empty string, got %q", got)
	}
}

// TestDeckIndex_FlushAndLoad pins the persistence round-trip: flushing
// to disk and constructing a new DeckIndex from the same path restores
// the entries. Necessary because seed files reference hashes; the
// index file is the lookup table consulted at read time.
func TestDeckIndex_FlushAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.json")
	di := NewDeckIndex(path)
	h := di.Register("persistent-key")
	di.Flush()

	// Confirm file was written.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("flush didn't write file: %v", err)
	}

	// Fresh DeckIndex pointing at the same path should load the entry.
	di2 := NewDeckIndex(path)
	if got := di2.Resolve(h); got != "persistent-key" {
		t.Errorf("reload broke: Resolve(%08x) = %q, want 'persistent-key'", h, got)
	}
}

// TestDeckIndex_FlushNoopWhenNotDirty verifies the dirty-flag
// optimization: if no Register was called after the last Flush, Flush
// is a no-op (doesn't rewrite the file unnecessarily).
func TestDeckIndex_FlushNoopWhenNotDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.json")
	di := NewDeckIndex(path)
	di.Register("key")
	di.Flush() // writes file, clears dirty

	// Capture mod time.
	statBefore, _ := os.Stat(path)
	di.Flush() // should be no-op
	statAfter, _ := os.Stat(path)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Errorf("Flush rewrote file when not dirty (ModTime changed)")
	}
}

// TestWriteAndReadBinarySeeds_RoundTrip pins the file-level pipeline:
// WriteBinarySeeds creates a file with the magic header and serialized
// records, ReadBinarySeeds reads it back and reconstructs the seeds.
// The header magic gate ensures non-Heimdall files don't get parsed.
func TestWriteAndReadBinarySeeds_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	di := NewDeckIndex(filepath.Join(dir, "index.json"))
	path := filepath.Join(dir, "seeds.bin")
	in := []GameSeed{
		{RNGSeed: 100, DeckKeys: [4]string{"a", "b", "c", "d"}, Winner: 0, Turns: 5, KillMethod: "combat"},
		{RNGSeed: 200, DeckKeys: [4]string{"a", "b", "c", "d"}, Winner: -1, Turns: 30, KillMethod: "timeout"},
	}
	size, err := WriteBinarySeeds(path, in, di)
	if err != nil {
		t.Fatalf("WriteBinarySeeds: %v", err)
	}
	// Header (8) + 2 records (28 each) = 64 bytes.
	if size != int64(seedHeaderSize+2*SeedRecordSize) {
		t.Errorf("file size = %d, want %d", size, seedHeaderSize+2*SeedRecordSize)
	}

	out, err := ReadBinarySeeds(path, 0, di) // limit=0 = read all
	if err != nil {
		t.Fatalf("ReadBinarySeeds: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("read len = %d, want %d", len(out), len(in))
	}
	for i := range in {
		if out[i].RNGSeed != in[i].RNGSeed || out[i].Winner != in[i].Winner ||
			out[i].Turns != in[i].Turns || out[i].KillMethod != in[i].KillMethod {
			t.Errorf("seed[%d] round-trip lost data:\n  in:  %+v\n  out: %+v", i, in[i], out[i])
		}
	}
}

// TestReadBinarySeeds_BadMagicErrors verifies the magic-byte gate
// rejects non-Heimdall files. Without this, ReadBinarySeeds would
// silently spit out garbage interpretations of unrelated bytes.
func TestReadBinarySeeds_BadMagicErrors(t *testing.T) {
	dir := t.TempDir()
	di := NewDeckIndex(filepath.Join(dir, "index.json"))
	path := filepath.Join(dir, "bad.bin")
	// Write 8 bytes of junk (wrong magic).
	if err := os.WriteFile(path, []byte{0, 1, 2, 3, 4, 5, 6, 7}, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := ReadBinarySeeds(path, 0, di)
	if err == nil {
		t.Errorf("expected magic-byte error, got nil")
	}
}

// TestReadBinarySeeds_LimitCapsCount verifies the limit parameter caps
// the returned slice — used by tooling to peek at the head of a large
// archive without loading the whole file.
func TestReadBinarySeeds_LimitCapsCount(t *testing.T) {
	dir := t.TempDir()
	di := NewDeckIndex(filepath.Join(dir, "index.json"))
	path := filepath.Join(dir, "seeds.bin")
	seeds := make([]GameSeed, 10)
	for i := range seeds {
		seeds[i] = GameSeed{RNGSeed: int64(i), DeckKeys: [4]string{"a", "b", "c", "d"}, Winner: 0}
	}
	if _, err := WriteBinarySeeds(path, seeds, di); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := ReadBinarySeeds(path, 3, di)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != 3 {
		t.Errorf("limit=3 should cap at 3, got %d", len(out))
	}
}
