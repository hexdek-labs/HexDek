package main

import (
	"os"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestExtractFabricatedInstanceID_HappyPath(t *testing.T) {
	msg := `ZoneConservation: InstanceID "h1OGVR200096" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry`
	got := ExtractFabricatedInstanceID(msg)
	want := "h1OGVR200096"
	if got != want {
		t.Fatalf("ExtractFabricatedInstanceID got %q, want %q", got, want)
	}
}

func TestExtractFabricatedInstanceID_DisappearanceArmReturnsEmpty(t *testing.T) {
	// The disappearance arm has a different message shape and must NOT match —
	// the forensics tool only handles fabrication.
	msg := `ZoneConservation: InstanceID "h1TKVC000110" (creature token zombie Token) is minted and not ceased but is absent from every zone — card disappeared`
	got := ExtractFabricatedInstanceID(msg)
	if got != "" {
		t.Fatalf("disappearance-arm must return empty; got %q", got)
	}
}

func TestExtractFabricatedInstanceID_UnrelatedInvariantReturnsEmpty(t *testing.T) {
	msg := `CardIdentity: card "Spikeshell Harrier" appears in both seat 1 battlefield and seat 1 battlefield`
	got := ExtractFabricatedInstanceID(msg)
	if got != "" {
		t.Fatalf("non-ZoneConservation message must return empty; got %q", got)
	}
}

func TestDecodeInstanceID_HappyPath(t *testing.T) {
	d := DecodeInstanceID("h1OGVR200096")
	if !d.Valid {
		t.Fatalf("expected Valid=true, got %+v", d)
	}
	want := DecodedInstanceID{
		Raw: "h1OGVR200096", Seat: 1, Provenance: "OG",
		Visibility: "V", Color: "R", CMC: 2, Sequence: 96, Valid: true,
	}
	if d != want {
		t.Fatalf("decoded mismatch:\n got=%+v\nwant=%+v", d, want)
	}
}

func TestDecodeInstanceID_TokenProvenance(t *testing.T) {
	d := DecodeInstanceID("h3TKVC000128")
	if !d.Valid || d.Provenance != "TK" || d.Seat != 3 || d.Color != "C" || d.CMC != 0 || d.Sequence != 128 {
		t.Fatalf("token-provenance decode wrong: %+v", d)
	}
}

func TestDecodeInstanceID_Invalid(t *testing.T) {
	cases := []string{
		"",                 // empty
		"junk",             // too short
		"x1OGVR200096",     // missing 'h' prefix
		"h1XXVR200096",     // bad provenance
		"h1OGZR200096",     // bad visibility
		"h1OGV?200096",     // bad cmc digit
		"h1OGVR2XX0096",    // bad seq (non-numeric)
	}
	for _, c := range cases {
		d := DecodeInstanceID(c)
		if d.Valid {
			t.Errorf("DecodeInstanceID(%q): want Valid=false, got %+v", c, d)
		}
	}
}

func TestTraceFirstAppearance_MatchByInstanceID(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "game_setup", Seat: -1},
		{Kind: "enter_battlefield", Seat: 1, Source: "Goblin Bushwhacker",
			Details: map[string]interface{}{"instance_id": "h1OGVR200096"}},
		{Kind: "combat_damage", Seat: 1, Source: "Goblin Bushwhacker", Amount: 2},
	}
	first := TraceFirstAppearance(events, "h1OGVR200096", "Goblin Bushwhacker")
	if first.NotFound {
		t.Fatalf("expected match, got NotFound")
	}
	if first.EventIdx != 1 {
		t.Fatalf("want EventIdx=1, got %d", first.EventIdx)
	}
	if first.MatchKind != "instance_id" {
		t.Fatalf("want MatchKind=instance_id, got %q", first.MatchKind)
	}
}

func TestTraceFirstAppearance_FallbackToCardName(t *testing.T) {
	// No event carries the instance_id in Details — must fall back to
	// matching event.Source against cardName.
	events := []gameengine.Event{
		{Kind: "game_setup", Seat: -1},
		{Kind: "creature_attacks", Seat: 1, Source: "Goblin Bushwhacker"},
		{Kind: "combat_damage", Seat: 1, Source: "Goblin Bushwhacker", Amount: 2},
	}
	first := TraceFirstAppearance(events, "h1OGVR200096", "Goblin Bushwhacker")
	if first.NotFound {
		t.Fatalf("expected fallback match, got NotFound")
	}
	if first.EventIdx != 1 {
		t.Fatalf("want EventIdx=1 (first creature_attacks), got %d", first.EventIdx)
	}
	if first.MatchKind != "card_name" {
		t.Fatalf("want MatchKind=card_name, got %q", first.MatchKind)
	}
}

func TestTraceFirstAppearance_NotFound(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "game_setup", Seat: -1},
		{Kind: "card_drawn", Seat: 0, Source: "Lightning Bolt"},
	}
	first := TraceFirstAppearance(events, "h1OGVR200096", "Goblin Bushwhacker")
	if !first.NotFound {
		t.Fatalf("expected NotFound, got match at idx %d", first.EventIdx)
	}
	if first.MatchKind != "<none>" {
		t.Fatalf("NotFound result must have MatchKind=<none>, got %q", first.MatchKind)
	}
}

func TestAnalyzeReplay_Synthetic_FabricationFixture(t *testing.T) {
	r, err := LoadReplay("testdata/replay-synthetic-fabrication.json")
	if err != nil {
		t.Fatalf("LoadReplay: %v", err)
	}
	if r.GameIdx != 411 {
		t.Fatalf("GameIdx: got %d, want 411", r.GameIdx)
	}
	if len(r.Violations) != 3 {
		t.Fatalf("Violations: got %d, want 3 (2 fabrication, 1 disappearance)", len(r.Violations))
	}

	traces := AnalyzeReplay(r)
	// 2 fabrication violations (both on the same h1OGVR200096 ID, 2
	// different turns); 1 disappearance is skipped.
	if len(traces) != 2 {
		t.Fatalf("traces: got %d, want 2 (disappearance must be skipped)", len(traces))
	}

	for i, tr := range traces {
		if tr.InstanceID != "h1OGVR200096" {
			t.Fatalf("trace[%d].InstanceID: got %q, want h1OGVR200096", i, tr.InstanceID)
		}
		if tr.CardName != "Goblin Bushwhacker" {
			t.Fatalf("trace[%d].CardName: got %q, want Goblin Bushwhacker", i, tr.CardName)
		}
		if !tr.Decoded.Valid {
			t.Fatalf("trace[%d].Decoded.Valid: got false", i)
		}
		if tr.Decoded.Seat != 1 || tr.Decoded.Provenance != "OG" || tr.Decoded.Color != "R" || tr.Decoded.CMC != 2 || tr.Decoded.Sequence != 96 {
			t.Fatalf("trace[%d].Decoded: got %+v", i, tr.Decoded)
		}
		if tr.First.NotFound {
			t.Fatalf("trace[%d].First: expected match against synthetic event log", i)
		}
		// Synthetic event 4 (0-indexed) is the enter_battlefield with
		// the instance_id in Details — that's the strongest match.
		if tr.First.EventIdx != 4 {
			t.Fatalf("trace[%d].First.EventIdx: got %d, want 4 (enter_battlefield)", i, tr.First.EventIdx)
		}
		if tr.First.MatchKind != "instance_id" {
			t.Fatalf("trace[%d].First.MatchKind: got %q, want instance_id", i, tr.First.MatchKind)
		}
	}

	// Render should produce a readable block per trace with key fields.
	r0 := RenderTrace(traces[0])
	for _, want := range []string{
		"turn 23", "ZoneConservation", "h1OGVR200096",
		"seat=1", "prov=OG", "Goblin Bushwhacker", "event #4",
		"matched via instance_id", "enter_battlefield",
	} {
		if !strings.Contains(r0, want) {
			t.Errorf("RenderTrace output missing %q\n--- output ---\n%s", want, r0)
		}
	}
}

func TestAnalyzeReplay_EmptyReplay(t *testing.T) {
	got := AnalyzeReplay(nil)
	if got != nil {
		t.Fatalf("nil-replay must return nil; got %+v", got)
	}
	got = AnalyzeReplay(&Replay{})
	if len(got) != 0 {
		t.Fatalf("empty replay must return empty; got %+v", got)
	}
}

func TestLoadReplay_MissingFile(t *testing.T) {
	_, err := LoadReplay("testdata/does-not-exist.json")
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestLoadReplay_BadJSON(t *testing.T) {
	// Write a junk fixture next to the real one — covered by .gitignore-
	// equivalent t.TempDir to avoid leaving artifacts in testdata/.
	dir := t.TempDir()
	bad := dir + "/junk.json"
	if err := os.WriteFile(bad, []byte("{ not valid json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := LoadReplay(bad)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
}
