package main

import (
	"testing"
)

// hasFinisherEvasionOrProtection: full coverage of the two curated
// keyword sets plus the explicit "trample alone fails" pin.
func TestHasFinisherEvasionOrProtection_KeywordCoverage(t *testing.T) {
	cases := []struct {
		name        string
		oracleText  string
		wantPass    bool
	}{
		// True-evasion keywords — each individually qualifies.
		{"flying", "flying", true},
		{"menace", "menace", true},
		{"fear", "fear", true},
		{"shadow", "shadow", true},
		{"horsemanship", "horsemanship", true},
		{"intimidate", "intimidate", true},
		{"skulk", "skulk", true},
		{"cant be blocked phrasing", "this creature can't be blocked.", true},
		{"unblockable legacy phrasing", "this creature is unblockable.", true},

		// Protection keywords — each individually qualifies.
		{"hexproof", "hexproof", true},
		{"shroud", "shroud", true},
		{"indestructible", "indestructible", true},
		{"ward", "ward {3}", true},
		{"protection from color", "protection from red", true},

		// Trample alone MUST fail — the core motivating case for r60.
		// A 6/6 trampler in 4-player EDH is chumped for 1 and the rest
		// is a 5-turn clock against one of three opponents — not a
		// finisher.
		{"trample alone fails", "trample", false},
		// Double strike alone fails — high damage per swing but blocker
		// chains stop it. Bumps to evasion-or-protection territory only
		// when paired with another keyword.
		{"double strike alone fails", "double strike", false},

		// Vanilla / value text never qualifies.
		{"vanilla beater", "", false},
		{"value etb", "when this creature enters, draw a card.", false},
		{"ramp body", "when this creature enters, search your library for a basic land card.", false},

		// Combined keywords still qualify (defends against accidental
		// short-circuit / regex regressions).
		{"flying + trample", "flying, trample", true},
		{"trample + indestructible", "trample, indestructible", true},
	}
	for _, tc := range cases {
		got := hasFinisherEvasionOrProtection(tc.oracleText)
		if got != tc.wantPass {
			t.Errorf("%s: hasFinisherEvasionOrProtection(%q) = %v, want %v",
				tc.name, tc.oracleText, got, tc.wantPass)
		}
	}
}

// classifyCardWithPower is a small helper that drives ClassifyCard
// through the IsCmdDamageThreat detection arm. Pins power as a
// string (matches Scryfall's representation) and assembles minimal
// other inputs.
func classifyCardWithPower(name, ot, typeLine, power string) CardProfile {
	return ClassifyCard(name, ot, typeLine, "", 7, power)
}

// IsCmdDamageThreat must NOT fire on a 6-power vanilla beater with
// no evasion or protection. Pre-r60 this case fell through to false
// (the trample/evasion check failed) UNLESS the power was ≥10 (which
// would unconditionally pass). The r60 dropped the unconditional
// arm — this test pins that drop.
func TestIsCmdDamageThreat_VanillaSixPowerRejected_R60(t *testing.T) {
	p := classifyCardWithPower("Test Beater",
		"", // no oracle text — pure vanilla
		"Legendary Creature — Beast", "6")
	if p.IsCmdDamageThreat {
		t.Error("6-power vanilla legendary creature must NOT be a commander-damage threat (no evasion, no protection)")
	}
}

// A 6-power trampler with NO other keyword must fail the gate.
// This is the headline r60 case — pre-r60 trample alone qualified,
// r60 drops it (chump-blockable in 4-player politics).
func TestIsCmdDamageThreat_TramplerAloneRejected_R60(t *testing.T) {
	p := classifyCardWithPower("Trampler",
		"trample",
		"Legendary Creature — Beast", "6")
	if p.IsCmdDamageThreat {
		t.Error("6-power trample-only legendary must NOT be a commander-damage threat (r60 evasion-or-protection gate)")
	}
}

// 10-power vanilla must also fail. Pre-r60 the unconditional
// pwr≥10 arm flagged it; r60 drops that arm because high power
// doesn't help if the creature dies to Path before connecting.
func TestIsCmdDamageThreat_TenPowerVanillaRejected_R60(t *testing.T) {
	p := classifyCardWithPower("Big Vanilla",
		"", // pure vanilla
		"Legendary Creature — Beast", "10")
	if p.IsCmdDamageThreat {
		t.Error("10-power vanilla legendary must NOT be a commander-damage threat (no evasion, no protection — removable on sight)")
	}
}

// Flying 6-power passes (true evasion, chump-resistant).
func TestIsCmdDamageThreat_FlyerPasses_R60(t *testing.T) {
	p := classifyCardWithPower("Dragon",
		"flying",
		"Legendary Creature — Dragon", "6")
	if !p.IsCmdDamageThreat {
		t.Error("6-power flyer must be a commander-damage threat (flying = true evasion)")
	}
}

// Indestructible 6-power passes (protection, removal-resistant).
func TestIsCmdDamageThreat_IndestructiblePasses_R60(t *testing.T) {
	p := classifyCardWithPower("Resilient Beater",
		"indestructible",
		"Legendary Creature — Beast", "6")
	if !p.IsCmdDamageThreat {
		t.Error("6-power indestructible legendary must be a commander-damage threat (survives removal)")
	}
}

// Ward 6-power passes (taxes removal — close enough to protection).
func TestIsCmdDamageThreat_WardPasses_R60(t *testing.T) {
	p := classifyCardWithPower("Taxed",
		"ward {3}",
		"Legendary Creature — Wizard", "6")
	if !p.IsCmdDamageThreat {
		t.Error("6-power ward legendary must be a commander-damage threat (ward taxes spot removal)")
	}
}

// Blightsteel Colossus shape: high power + indestructible (passes
// the protection arm). The canonical "10-power = finisher" case
// must continue to qualify under the new gate.
func TestIsCmdDamageThreat_BlightsteelCanonicalPasses_R60(t *testing.T) {
	p := classifyCardWithPower("Blightsteel Colossus",
		"infect, indestructible, trample",
		"Legendary Artifact Creature — Phyrexian Golem", "11")
	if !p.IsCmdDamageThreat {
		t.Error("Blightsteel-shape (high power + indestructible + trample + infect) must be a finisher — the canonical case")
	}
}

// Power < 6 always fails regardless of evasion/protection (the
// power threshold itself is unchanged from pre-r60).
func TestIsCmdDamageThreat_LowPowerRejected_R60(t *testing.T) {
	p := classifyCardWithPower("Small Flyer",
		"flying, hexproof",
		"Legendary Creature — Faerie", "4")
	if p.IsCmdDamageThreat {
		t.Error("4-power creature must NOT be a commander-damage threat regardless of keywords (power < 6 threshold)")
	}
}

// Non-legendary fails — IsCmdDamageThreat specifically targets
// commander-damage delivery.
func TestIsCmdDamageThreat_NonLegendaryRejected_R60(t *testing.T) {
	p := classifyCardWithPower("Dragon",
		"flying, indestructible",
		"Creature — Dragon", "8")
	if p.IsCmdDamageThreat {
		t.Error("non-legendary creature must NOT be a commander-damage threat (commander-damage requires legendary)")
	}
}

// End-to-end: FindFinishers count drops when a vanilla 10-power
// legendary is in the deck. Pre-r60: the vanilla 10/10 contributed
// 1 to the finisher count (via the dropped pwr≥10 unconditional
// arm). Post-r60: it doesn't.
func TestFindFinishers_VanillaBigCreatureNoLongerCounted_R60(t *testing.T) {
	vanilla := classifyCardWithPower("Vanilla Big",
		"",
		"Legendary Creature — Beast", "10")
	evasive := classifyCardWithPower("Real Threat",
		"flying, indestructible",
		"Legendary Creature — Dragon", "7")
	got := FindFinishers([]CardProfile{vanilla, evasive})
	// We expect ONE finisher (the evasive one), not two.
	cmdHits := 0
	for _, f := range got {
		for _, c := range f.Cards {
			if c == "Vanilla Big" {
				t.Errorf("vanilla 10/10 must NOT appear in finishers; got entry %+v", f)
			}
			if c == "Real Threat" {
				cmdHits++
			}
		}
	}
	if cmdHits == 0 {
		t.Error("evasive flying+indestructible 7-power threat must appear as a finisher")
	}
}
