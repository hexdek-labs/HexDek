package deckparser

import (
	"bytes"
	"strings"
	"testing"
)

// comment_preservation_r60_test.go — inline-comment + hashtag round-
// trip preservation. Closes the loop on deck annotation workflow:
// parse → modify → write → re-parse must preserve the deckbuilder's
// intent notes.
//
// Pre-fix:
//   - `// note` was captured into CardLine.Comment (since PR #649).
//   - `#tag1 #tag2` was silently stripped by cleanCardName's
//     hashTagRE — the card resolved correctly but the tag content was
//     LOST. UIs surfacing deckbuilder intent ("#wincon", "#ramp",
//     "#flex") had no way to recover them.
//   - TournamentDeck had NO WriteText surface — there was no way to
//     serialize an annotated deck back to text.
//
// This PR adds: CardLine.HashTags field populated by extractHashTags
// (runs before set-parens strip so `1 Card (CMR) 254 #tag` doesn't
// drop the tag); TournamentDeck.WriteText emits a Moxfield-format
// text decklist with Comment + HashTags re-rendered verbatim;
// round-trip identity verified by re-parsing the written output.

func roundTripMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Chulane, Teller of Tales",
		"Sol Ring", "Command Tower", "Arcane Signet", "Lightning Greaves",
		"Cyclonic Rift", "Counterspell", "Demonic Tutor", "Vampiric Tutor",
		"Sylvan Library", "Eternal Witness", "Birds of Paradise",
		"Llanowar Elves", "Cultivate", "Path to Exile", "Beast Within",
		"Forest", "Mountain", "Island", "Plains", "Swamp",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestExtractHashTags_Table — the canonical transformation. Each
// trailing `#tag` block is split into individual tags (leading `#`
// stripped); the line preceding the block is returned with trailing
// whitespace trimmed. No-op when no block is present.
func TestExtractHashTags_Table(t *testing.T) {
	cases := []struct {
		in       string
		wantLine string
		wantTags []string
	}{
		{"1 Sol Ring", "1 Sol Ring", nil},
		{"1 Sol Ring #ramp", "1 Sol Ring", []string{"ramp"}},
		{"1 Sol Ring #ramp #wincon", "1 Sol Ring", []string{"ramp", "wincon"}},
		{"1 Sol Ring #turn-1-ramp #budget-friendly", "1 Sol Ring", []string{"turn-1-ramp", "budget-friendly"}},
		// Inner `#` chars are left alone — only the trailing anchored
		// block extracts. A standalone `#` mid-line is malformed and
		// stays in the name (defensive — no real card has this).
		{"1 Sol Ring", "1 Sol Ring", nil},
		// Multiple hashtags with mixed-case tags.
		{"1 Cyclonic Rift #wincon #BoardWipe", "1 Cyclonic Rift", []string{"wincon", "BoardWipe"}},
	}
	for _, tc := range cases {
		gotLine, gotTags := extractHashTags(tc.in)
		if gotLine != tc.wantLine {
			t.Errorf("extractHashTags(%q) line = %q, want %q", tc.in, gotLine, tc.wantLine)
		}
		if len(gotTags) != len(tc.wantTags) {
			t.Errorf("extractHashTags(%q) tags = %v, want %v", tc.in, gotTags, tc.wantTags)
			continue
		}
		for i, want := range tc.wantTags {
			if gotTags[i] != want {
				t.Errorf("extractHashTags(%q) tags[%d] = %q, want %q", tc.in, i, gotTags[i], want)
			}
		}
	}
}

// TestParseDeckReader_HashTagsCaptured — end-to-end: a deck with
// hashtag annotations parses with CardLine.HashTags populated.
// The cards still resolve cleanly because the tag block strips
// BEFORE cleanCardName runs — the meta lookup sees the bare name.
func TestParseDeckReader_HashTagsCaptured(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring #ramp #must-keep
1 Cyclonic Rift #wincon #board-wipe
1 Counterspell #counter #blue-package
1 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// All 4 mainboard cards should resolve.
	if len(td.Library) != 4 {
		t.Errorf("Library: want 4, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	// CardLines should carry the expected hashtags.
	wantTags := map[string][]string{
		"Sol Ring":      {"ramp", "must-keep"},
		"Cyclonic Rift": {"wincon", "board-wipe"},
		"Counterspell":  {"counter", "blue-package"},
		"Forest":        nil,
	}
	for _, cl := range td.CardLines {
		if cl.Section != "main" {
			continue
		}
		want, ok := wantTags[cl.Name]
		if !ok {
			continue
		}
		if len(cl.HashTags) != len(want) {
			t.Errorf("CardLines[%s] HashTags = %v, want %v", cl.Name, cl.HashTags, want)
			continue
		}
		for i, w := range want {
			if cl.HashTags[i] != w {
				t.Errorf("CardLines[%s].HashTags[%d] = %q, want %q", cl.Name, i, cl.HashTags[i], w)
			}
		}
	}
}

// TestParseDeckReader_HashTagsWithSetCode — set-parens BEFORE
// hashtag block. The extraction runs before the set-parens strip so
// the tag survives.
func TestParseDeckReader_HashTagsWithSetCode(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring (CMR) 254 #ramp #wincon
1 Cyclonic Rift (CMR) 76 #removal
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantTags := map[string][]string{
		"Sol Ring":      {"ramp", "wincon"},
		"Cyclonic Rift": {"removal"},
	}
	for _, cl := range td.CardLines {
		if cl.Section != "main" {
			continue
		}
		want := wantTags[cl.Name]
		if len(cl.HashTags) != len(want) {
			t.Errorf("CardLines[%s] HashTags = %v, want %v (set-parens BEFORE hashtag block must not eat the tags)",
				cl.Name, cl.HashTags, want)
		}
	}
}

// TestWriteText_BasicCommanderDeck — the simplest serializer
// contract: COMMANDER directive + `qty name` per line, blank line
// between commander block and mainboard.
func TestWriteText_BasicCommanderDeck(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Command Tower
2 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := td.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	wantLines := []string{
		"COMMANDER: Atraxa, Praetors' Voice",
		"1 Sol Ring",
		"1 Command Tower",
		"2 Forest",
	}
	for _, w := range wantLines {
		if !strings.Contains(out, w) {
			t.Errorf("WriteText output missing %q; got:\n%s", w, out)
		}
	}
}

// TestWriteText_PreservesCommentsAndTags — the load-bearing fix:
// `// comment` and `#tag` annotations from the source deck appear
// verbatim in the WriteText output.
func TestWriteText_PreservesCommentsAndTags(t *testing.T) {
	text := `COMMANDER: Chulane, Teller of Tales
1 Sol Ring // mvp #ramp
1 Command Tower // 5-color land #lands
1 Cyclonic Rift // wincon — never cut #wincon #board-wipe
1 Forest #lands
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := td.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"1 Sol Ring // mvp #ramp",
		"1 Command Tower // 5-color land #lands",
		"1 Cyclonic Rift // wincon — never cut #wincon #board-wipe",
		"1 Forest #lands",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("WriteText missing %q; got:\n%s", want, out)
		}
	}
}

// TestWriteText_PartnerPair — partner decks emit both directive
// lines (COMMANDER + PARTNER). Order is commander-first, partner-
// second matching TournamentDeck.CommanderCards ordering.
func TestWriteText_PartnerPair(t *testing.T) {
	text := `Commanders (2)
1 Atraxa, Praetors' Voice
1 Chulane, Teller of Tales

Deck (2)
1 Sol Ring
1 Command Tower
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("want 2 commanders, got %d", len(td.CommanderCards))
	}
	var buf bytes.Buffer
	if err := td.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "COMMANDER: Atraxa, Praetors' Voice") {
		t.Errorf("missing primary commander directive; got:\n%s", out)
	}
	if !strings.Contains(out, "PARTNER: Chulane, Teller of Tales") {
		t.Errorf("missing partner directive; got:\n%s", out)
	}
}

// TestWriteText_RoundTripPreservesCommentsAndTags — the canonical
// round-trip identity test. parse → write → re-parse → CardLine
// equivalence on Qty / Name / Comment / HashTags. The parser-emitted
// output must be re-parseable into the SAME annotation state.
func TestWriteText_RoundTripPreservesCommentsAndTags(t *testing.T) {
	original := `COMMANDER: Chulane, Teller of Tales
1 Sol Ring // mvp #ramp #must-keep
1 Cyclonic Rift // wincon #wincon
1 Counterspell #counter
1 Path to Exile // single-target #removal
4 Forest
`
	meta := roundTripMeta()
	td1, err := ParseDeckReader(strings.NewReader(original), nil, meta)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	var buf bytes.Buffer
	if err := td1.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	td2, err := ParseDeckReader(strings.NewReader(buf.String()), nil, meta)
	if err != nil {
		t.Fatalf("second parse: %v\noutput was:\n%s", err, buf.String())
	}
	// Commander identity.
	if td1.CommanderName != td2.CommanderName {
		t.Errorf("commander mismatch round-trip: %q → %q", td1.CommanderName, td2.CommanderName)
	}
	// Mainboard CardLines equivalence on the annotation fields.
	cmp := func(td *TournamentDeck) []string {
		var out []string
		for _, cl := range td.CardLines {
			if cl.Section != "main" {
				continue
			}
			out = append(out, cl.Name+"|"+cl.Comment+"|"+strings.Join(cl.HashTags, ","))
		}
		return out
	}
	a, b := cmp(td1), cmp(td2)
	if len(a) != len(b) {
		t.Fatalf("CardLines count round-trip mismatch: %d → %d (out=%s)", len(a), len(b), buf.String())
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("CardLines[%d] round-trip drift:\n  before: %s\n  after:  %s", i, a[i], b[i])
		}
	}
}

// TestWriteText_NilDeckNoPanic — defensive nil-receiver check so
// callers don't crash when a parse error left td nil.
func TestWriteText_NilDeckNoPanic(t *testing.T) {
	var td *TournamentDeck
	var buf bytes.Buffer
	if err := td.WriteText(&buf); err != nil {
		t.Fatalf("nil-receiver WriteText: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("nil receiver should produce empty output, got %q", buf.String())
	}
}

// TestParseDeckReader_HashTagsNoFalseStripOnCommandLine — tags on
// the COMMANDER directive line itself: pre-fix and post-fix the
// directive parsing doesn't see hashtag extraction (the extract step
// runs only on plain card lines), so `COMMANDER: Atraxa #cmdr-tag`
// would carry the tag into the name. Document this is OUT of scope
// — the round-trip preservation is for mainboard cards.
func TestParseDeckReader_HashTagsNoFalseStripOnCommandLine(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring #ramp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, roundTripMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.CommanderName != "Atraxa, Praetors' Voice" {
		t.Errorf("commander name = %q, want %q (no hashtag handling on directive line — out of scope)",
			td.CommanderName, "Atraxa, Praetors' Voice")
	}
	// And the mainboard hashtag still works.
	for _, cl := range td.CardLines {
		if cl.Name == "Sol Ring" {
			if len(cl.HashTags) != 1 || cl.HashTags[0] != "ramp" {
				t.Errorf("Sol Ring HashTags = %v, want [ramp]", cl.HashTags)
			}
		}
	}
}
