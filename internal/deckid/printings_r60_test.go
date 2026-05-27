package deckid

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// printings_r60_test.go — basic-land printing-variant collision audit
// follow-up to PR #557. ComputeHash must treat different printings of
// the SAME logical card (Forest from THB / MID / NEO; foil vs non-foil;
// etched-foil) as the same content, but must NOT collapse genuinely
// distinct cards (Snow-Covered Forest, Forest from Aether Hub-style
// supplemental sets that share a name root).
//
// Pre-fix audit found that `Card{Name: "Forest (THB) 270"}` and
// `Card{Name: "Forest"}` hashed to different deck IDs, even though
// every parser-driven path strips the printing tail before constructing
// the Card. The defensive CleanCardName pre-pass in normalizeName fixes
// the gap so callers that bypass the parser (DB rows, manual builds,
// external imports) still get content-addressing semantics.

// TestDeckHash_BasicLandPrintingVariantsCollapse — different printings
// of Forest must hash identically. The variants below mirror the real
// formats the parser sees from Moxfield ("Forest (THB) 270"), MTGO
// ("[MID] Forest"), and Archidekt foil markers ("*F*" / "*F-Etched*").
func TestDeckHash_BasicLandPrintingVariantsCollapse(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Atraxa, Praetors' Voice"}}
	variants := map[string]string{
		"plain":          "Forest",
		"moxfield_set":   "Forest (THB) 270",
		"moxfield_short": "Forest (NEO)",
		"mtgo_bracket":   "[MID] Forest",
		"foil":           "Forest *F*",
		"etched_foil":    "Forest *F-Etched*",
		"hashtag":        "Forest #ramp",
		"trail_bracket":  "Forest [Lands]",
		"mixed":          "[MID] Forest *F*",
	}
	baseline := ComputeHash(cmd, []*gameengine.Card{{Name: variants["plain"]}})
	for label, name := range variants {
		h := ComputeHash(cmd, []*gameengine.Card{{Name: name}})
		if h != baseline {
			t.Errorf("variant %q (%q) hashed to %s, want baseline %s — printing decoration leaked into deck ID",
				label, name, string(h)[:16], string(baseline)[:16])
		}
	}
}

// TestDeckHash_SnowCoveredForestStillDistinct — defends against
// over-stripping. Snow-Covered Forest is a genuinely different card
// (different oracle text, snow-mana payoffs, different SBA-relevant
// type line) and MUST hash differently from Forest.
func TestDeckHash_SnowCoveredForestStillDistinct(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Marit Lage's Slumber"}}
	plain := ComputeHash(cmd, []*gameengine.Card{{Name: "Forest"}})
	snow := ComputeHash(cmd, []*gameengine.Card{{Name: "Snow-Covered Forest"}})
	if plain == snow {
		t.Fatalf("Snow-Covered Forest collapsed to Forest — name-strip is too aggressive (both hash %s)",
			string(plain)[:16])
	}
}

// TestDeckHash_FullDeckPrintingMix — end-to-end stress: two decks with
// identical content but ONE deck has every basic land tagged with a
// random printing decoration. Hashes must match.
func TestDeckHash_FullDeckPrintingMix(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Omnath, Locus of Creation"}}
	clean := []*gameengine.Card{
		{Name: "Sol Ring"}, {Name: "Cyclonic Rift"}, {Name: "Mana Crypt"},
		{Name: "Forest"}, {Name: "Forest"}, {Name: "Forest"}, {Name: "Forest"},
		{Name: "Island"}, {Name: "Island"}, {Name: "Plains"}, {Name: "Mountain"},
	}
	decorated := []*gameengine.Card{
		{Name: "Sol Ring (LEA) 256"}, {Name: "Cyclonic Rift *F*"}, {Name: "[MH3] Mana Crypt"},
		{Name: "Forest (THB) 270"}, {Name: "[MID] Forest"}, {Name: "Forest *F-Etched*"}, {Name: "Forest #ramp"},
		{Name: "Island (NEO) 304"}, {Name: "Island *F*"}, {Name: "Plains [Lands]"}, {Name: "Mountain (LTR) 282"},
	}
	if ComputeHash(cmd, clean) != ComputeHash(cmd, decorated) {
		t.Fatalf("full-deck printing mix: clean hash != decorated hash — content-addressing broken across import paths")
	}
}

// TestDeckHash_NonbasicPrintingsCollapse — printing-variant collapsing
// must apply uniformly, not just to basics. Sol Ring from LEA vs C21
// is the same card.
func TestDeckHash_NonbasicPrintingsCollapse(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Urza, Lord High Artificer"}}
	a := ComputeHash(cmd, []*gameengine.Card{{Name: "Sol Ring"}, {Name: "Mana Crypt"}})
	b := ComputeHash(cmd, []*gameengine.Card{{Name: "Sol Ring (LEA) 256 *F*"}, {Name: "[MH3] Mana Crypt *F-Etched*"}})
	if a != b {
		t.Fatalf("nonbasic printing variants hashed differently")
	}
}
