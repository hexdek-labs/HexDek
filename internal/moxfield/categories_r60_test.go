package moxfield

import (
	"encoding/json"
	"strings"
	"testing"
)

// categories_r60_test.go — pins capture of Moxfield's Maybeboard and
// Considering boards. Building on #300 (Tags / Hubs), these are the
// other user-applied "category" surfaces the importer was discarding.
// Both emit `// Maybeboard:` / `// Considering:` comment lines that
// the downstream deckparser silently skips (already-recognized as
// sideboard-flavored section markers in textlist.go), so they don't
// pollute the playable deck but DO survive in the .txt file for
// human + tooling visibility.

const fixtureMaybeboard = `{
	"name": "Yuriko with a maybeboard",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"maybeboard": {
			"count": 3,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "Mystic Confluence", "prices": {"usd": "2.00"}}},
				"b": {"quantity": 2, "card": {"name": "Bitterblossom"}},
				"c": {"quantity": 1, "card": {"name": "Force of Will"}}
			}
		}
	}
}`

func TestFixture_MaybeboardEmittedWithMarker(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureMaybeboard), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Maybeboard: 1 Mystic Confluence",
		"// Maybeboard: 2 Bitterblossom",
		"// Maybeboard: 1 Force of Will",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing maybeboard line %q in:\n%s", want, out)
		}
	}
}

func TestFixture_MaybeboardDoesNotLeakIntoMainboard(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureMaybeboard), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// Negative guard: Bitterblossom must not appear as a bare mainboard
	// line — that would falsely add it to the playable Commander deck.
	for _, forbidden := range []string{
		"\n2 Bitterblossom\n",
		"\n1 Mystic Confluence\n",
		"\n1 Force of Will\n",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("maybeboard card leaked into mainboard channel %q in:\n%s",
				forbidden, out)
		}
	}
}

const fixtureConsidering = `{
	"name": "Korvold but considering swaps",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Korvold, Fae-Cursed King"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"considering": {
			"count": 2,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "Goblin Bombardment"}},
				"b": {"quantity": 1, "card": {"name": "Pitiless Plunderer"}}
			}
		}
	}
}`

func TestFixture_ConsideringEmittedWithMarker(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureConsidering), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Considering: 1 Goblin Bombardment",
		"// Considering: 1 Pitiless Plunderer",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing considering line %q in:\n%s", want, out)
		}
	}
}

// All six boards together: commanders + main + side + companion +
// maybeboard + considering. Pin output ordering AND that each card
// only appears in its own section.
const fixtureAllSixBoards = `{
	"name": "Edgar All Boards",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Edgar Markov"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"sideboard":  {"count": 1, "cards": {"s": {"quantity": 1, "card": {"name": "Tezzeret the Seeker"}}}},
		"companions": {"count": 1, "cards": {"comp": {"quantity": 1, "card": {"name": "Lurrus of the Dream-Den"}}}},
		"maybeboard": {"count": 1, "cards": {"mb":  {"quantity": 1, "card": {"name": "Bitterblossom"}}}},
		"considering": {"count": 1, "cards": {"con": {"quantity": 1, "card": {"name": "Force of Will"}}}}
	}
}`

func TestFixture_AllSixBoardsPresentAndOrdered(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureAllSixBoards), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}

	// Each board must produce its marker.
	for _, want := range []string{
		"COMMANDER: Edgar Markov",
		"\n1 Sol Ring\n",
		"// Sideboard: 1 Tezzeret the Seeker",
		"// Companion: 1 Lurrus of the Dream-Den",
		"// Maybeboard: 1 Bitterblossom",
		"// Considering: 1 Force of Will",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing marker %q in:\n%s", want, out)
		}
	}

	// Section ordering: commanders → mainboard → sideboard → companion
	// → maybeboard → considering. Pin it so a refactor that swaps the
	// writeEntries call order is caught.
	idxCommander := strings.Index(out, "COMMANDER: Edgar")
	idxMain := strings.Index(out, "\n1 Sol Ring\n")
	idxSide := strings.Index(out, "// Sideboard:")
	idxComp := strings.Index(out, "// Companion:")
	idxMaybe := strings.Index(out, "// Maybeboard:")
	idxConsid := strings.Index(out, "// Considering:")
	indices := []struct {
		name string
		idx  int
	}{
		{"COMMANDER", idxCommander},
		{"mainboard", idxMain},
		{"Sideboard", idxSide},
		{"Companion", idxComp},
		{"Maybeboard", idxMaybe},
		{"Considering", idxConsid},
	}
	for i := 1; i < len(indices); i++ {
		if indices[i-1].idx < 0 || indices[i].idx < 0 {
			t.Fatalf("missing section index in: %+v\n%s", indices, out)
		}
		if indices[i-1].idx >= indices[i].idx {
			t.Errorf("section order broken: %s@%d should precede %s@%d",
				indices[i-1].name, indices[i-1].idx,
				indices[i].name, indices[i].idx)
		}
	}
}

func TestFixture_NoMaybeboardConsideringNoMarkers(t *testing.T) {
	// Sanity: a deck without maybeboard/considering must NOT emit
	// empty `// Maybeboard:` / `// Considering:` lines.
	const noMC = `{
		"name": "Plain Deck",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(noMC), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "// Maybeboard:") {
		t.Errorf("plain deck should not emit `// Maybeboard:`:\n%s", out)
	}
	if strings.Contains(out, "// Considering:") {
		t.Errorf("plain deck should not emit `// Considering:`:\n%s", out)
	}
}

// Card-noise fields (prices, isFoil, scryfall_id, alt printings) on
// maybeboard entries must not leak — same guarantee that
// fixtures_r60_test.go pinned for mainboard.
func TestFixture_MaybeboardNoiseFieldsDontLeak(t *testing.T) {
	const fixture = `{
		"name": "Maybeboard With Noise",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Karador, Ghost Chieftain"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"maybeboard": {
				"count": 1,
				"cards": {
					"mb": {
						"quantity": 1,
						"isFoil": true,
						"isEtched": false,
						"useCmcOverride": true,
						"prices": {"usd": "55.00", "usd_foil": "120.00", "tix": "3.00"},
						"card": {
							"name": "Eternal Witness",
							"scryfall_id": "deadbeef-0000-0000-0000-000000000000",
							"set": "5dn",
							"collector_number": "62"
						}
					}
				}
			}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// Maybeboard: 1 Eternal Witness") {
		t.Errorf("missing maybeboard line in:\n%s", out)
	}
	for _, leaked := range []string{
		"isFoil", "isEtched", "useCmcOverride",
		"prices", "55.00", "120.00", "tix",
		"scryfall_id", "deadbeef", "collector_number", "5dn",
	} {
		if strings.Contains(out, leaked) {
			t.Errorf("noise field %q leaked from maybeboard into output:\n%s",
				leaked, out)
		}
	}
}

// Empty board (key present but zero cards) must not emit any lines.
func TestFixture_EmptyMaybeboardSilent(t *testing.T) {
	const fixture = `{
		"name": "Empty Maybeboard",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"maybeboard": {"count": 0, "cards": {}},
			"considering": {"count": 0, "cards": {}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "// Maybeboard:") {
		t.Errorf("empty maybeboard should not emit a marker:\n%s", out)
	}
	if strings.Contains(out, "// Considering:") {
		t.Errorf("empty considering should not emit a marker:\n%s", out)
	}
}

// Quantity-zero entries within a maybeboard/considering must be
// skipped the same way mainboard handles them.
func TestFixture_MaybeboardQuantityZeroSkipped(t *testing.T) {
	const fixture = `{
		"name": "Quantity-Zero Maybeboard",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"maybeboard": {
				"count": 1,
				"cards": {
					"good": {"quantity": 1, "card": {"name": "Bitterblossom"}},
					"zero": {"quantity": 0, "card": {"name": "Should Be Skipped"}}
				}
			}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "Should Be Skipped") {
		t.Errorf("quantity=0 maybeboard entry leaked:\n%s", out)
	}
	if !containsLine(out, "// Maybeboard: 1 Bitterblossom") {
		t.Errorf("non-zero maybeboard entry missing:\n%s", out)
	}
}

// Per-board ordering inside the maybeboard must be sorted by entry
// key (matching how every other board is iterated). Deterministic
// output matters because diff reviews compare deck files.
func TestFixture_MaybeboardSortedDeterministically(t *testing.T) {
	const fixture = `{
		"name": "Ordering",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"maybeboard": {
				"count": 3,
				"cards": {
					"zeta":  {"quantity": 1, "card": {"name": "Card Z"}},
					"alpha": {"quantity": 1, "card": {"name": "Card A"}},
					"mu":    {"quantity": 1, "card": {"name": "Card M"}}
				}
			}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	first, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// Entry keys "alpha" < "mu" < "zeta" → output lines for Card A, M, Z.
	idxA := strings.Index(first, "// Maybeboard: 1 Card A")
	idxM := strings.Index(first, "// Maybeboard: 1 Card M")
	idxZ := strings.Index(first, "// Maybeboard: 1 Card Z")
	if idxA < 0 || idxM < 0 || idxZ < 0 {
		t.Fatalf("missing entries:\n%s", first)
	}
	if !(idxA < idxM && idxM < idxZ) {
		t.Errorf("maybeboard not sorted: A@%d M@%d Z@%d", idxA, idxM, idxZ)
	}
	// Determinism: run multiple times.
	for i := 0; i < 20; i++ {
		out, err := formatDecklist(&r)
		if err != nil {
			t.Fatalf("formatDecklist iter %d: %v", i, err)
		}
		if out != first {
			t.Fatalf("non-deterministic at iter %d", i)
		}
	}
}

// Legacy top-level (pre-v3 wrapper) maybeboard/considering still work.
// Real production caches from before #300 won't have these fields at
// all, but if a future cache format does, it should parse cleanly.
func TestFixture_LegacyTopLevelMaybeboardConsidering(t *testing.T) {
	const fixture = `{
		"name": "Legacy Layout",
		"format": "commander",
		"commanders":  {"c":  {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}},
		"mainboard":   {"m":  {"quantity": 1, "card": {"name": "Sol Ring"}}},
		"maybeboard":  {"mb": {"quantity": 1, "card": {"name": "Bitterblossom"}}},
		"considering": {"co": {"quantity": 1, "card": {"name": "Force of Will"}}}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// Maybeboard: 1 Bitterblossom") {
		t.Errorf("legacy maybeboard missing in:\n%s", out)
	}
	if !containsLine(out, "// Considering: 1 Force of Will") {
		t.Errorf("legacy considering missing in:\n%s", out)
	}
}

// v3 boards take precedence over legacy top-level — same pattern as
// the existing fixture test for the other boards.
func TestFixture_V3MaybeboardShadowsLegacyTopLevel(t *testing.T) {
	const fixture = `{
		"name": "Both Layouts",
		"format": "commander",
		"maybeboard":  {"old": {"quantity": 1, "card": {"name": "Old Maybe Card"}}},
		"considering": {"old": {"quantity": 1, "card": {"name": "Old Consid Card"}}},
		"boards": {
			"commanders":  {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":   {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"maybeboard":  {"count": 1, "cards": {"new": {"quantity": 1, "card": {"name": "New Maybe Card"}}}},
			"considering": {"count": 1, "cards": {"new": {"quantity": 1, "card": {"name": "New Consid Card"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// Maybeboard: 1 New Maybe Card") {
		t.Errorf("v3 maybeboard missing:\n%s", out)
	}
	if !containsLine(out, "// Considering: 1 New Consid Card") {
		t.Errorf("v3 considering missing:\n%s", out)
	}
	if strings.Contains(out, "Old Maybe Card") {
		t.Errorf("legacy maybeboard should be shadowed by v3:\n%s", out)
	}
	if strings.Contains(out, "Old Consid Card") {
		t.Errorf("legacy considering should be shadowed by v3:\n%s", out)
	}
}

// Composing with #300 tags: a deck with hubs + maybeboard + considering
// should emit ALL the right markers in the right order.
func TestFixture_TagsPlusCategoriesCombined(t *testing.T) {
	const fixture = `{
		"name": "Tags + Categories",
		"format": "commander",
		"hubs": [{"name": "Combo"}, {"name": "Token"}],
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Krenko, Mob Boss"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Goblin Bombardment"}}}},
			"maybeboard": {"count": 1, "cards": {"mb": {"quantity": 1, "card": {"name": "Skullclamp"}}}},
			"considering": {"count": 1, "cards": {"co": {"quantity": 1, "card": {"name": "Purphoros, God of the Forge"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixture), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// Order: Tags → COMMANDER → mainboard → Maybeboard → Considering.
	idxTags := strings.Index(out, "// Tags: Combo, Token")
	idxCommander := strings.Index(out, "COMMANDER: Krenko")
	idxMain := strings.Index(out, "1 Goblin Bombardment")
	idxMaybe := strings.Index(out, "// Maybeboard:")
	idxConsid := strings.Index(out, "// Considering:")
	for _, p := range []struct {
		name string
		idx  int
	}{
		{"Tags", idxTags}, {"Commander", idxCommander}, {"Mainboard", idxMain},
		{"Maybeboard", idxMaybe}, {"Considering", idxConsid},
	} {
		if p.idx < 0 {
			t.Fatalf("missing %s marker in:\n%s", p.name, out)
		}
	}
	if !(idxTags < idxCommander && idxCommander < idxMain &&
		idxMain < idxMaybe && idxMaybe < idxConsid) {
		t.Errorf("composed ordering broken: Tags@%d Commander@%d Main@%d Maybe@%d Consid@%d",
			idxTags, idxCommander, idxMain, idxMaybe, idxConsid)
	}
}

// Sanity: textlist.go's ParseDecklist already treats `// Maybeboard:`
// and `// Considering:` as sideboard-flavored markers (skipped for the
// Commander format — they share the section detection path with
// `// Sideboard:` and `// Companion:`). Pin via a round-trip: write
// the .txt output, then re-parse it, and assert no maybeboard /
// considering / sideboard / companion card ends up in the parsed
// mainboard.
//
// Note: textlist.ParseDecklist does NOT recognize the `COMMANDER:`
// directive — that's the broader internal/deckparser's job — so we
// don't assert commander presence here. The "didn't get promoted to
// playable" property is what matters for this regression.
func TestFixture_DeckparserRoundTrip_MaybeboardConsideringSkipped(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureAllSixBoards), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	parsed, err := ParseDecklist(out)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	forbidden := map[string]string{
		"Bitterblossom":          "maybeboard",
		"Force of Will":          "considering",
		"Tezzeret the Seeker":    "sideboard",
		"Lurrus of the Dream-Den": "companion",
	}
	for _, e := range parsed.Mainboard {
		if section, bad := forbidden[e.Name]; bad {
			t.Errorf("%s card leaked into parsed mainboard: %+v", section, e)
		}
	}
	for _, e := range parsed.Commander {
		if section, bad := forbidden[e.Name]; bad {
			t.Errorf("%s card leaked into parsed commander: %+v", section, e)
		}
	}
	// Positive: Sol Ring (the only true mainboard entry) survives.
	found := false
	for _, e := range parsed.Mainboard {
		if e.Name == "Sol Ring" {
			found = true
		}
	}
	if !found {
		t.Errorf("Sol Ring mainboard missing after round-trip; parsed: %+v", parsed)
	}
}
