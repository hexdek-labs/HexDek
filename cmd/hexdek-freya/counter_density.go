package main

import (
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Counter density analysis — Counters Matter theme rollup.
//
// Walks report.Profiles, classifies each PERMANENT card (creature /
// artifact / enchantment / planeswalker / battle) as one or both of:
//
//   Producer: puts +1/+1 counters on a creature you control or
//             enters with +1/+1 counters on itself. The canonical
//             "deck cares about counter placement" signal.
//
//   Payoff:   triggers off counter placement, scales with the number
//             of +1/+1 counters present, multiplies counter
//             placement (Hardened Scales / Doubling Season /
//             Branching Evolution / Conclave Mentor), or pays off
//             "modified" creatures (the modern shorthand for
//             "creature with counter / aura / equipment").
//
// Instants and sorceries are intentionally EXCLUDED — the theme
// framing is about persistent board presence and recurring
// triggers, not one-shot pumps (Berserk, Inspiring Call). A
// Triumph-of-the-Hordes proliferate spell is real Counters Matter
// support but doesn't constitute a permanent producer / payoff for
// theme detection purposes.
//
// A card can land in BOTH lists when it produces AND scales —
// Hangarback Walker enters with counters and dies for tokens equal
// to its counters; Walking Ballista enters with counters and pays
// them out for damage. Recording each independently lets downstream
// consumers see the structural overlap.
//
// Theme thresholds (chosen to align with the existing
// "Counters Matter" archetype fingerprint at ctx.counterCount >= 8
// but scoped strictly to permanents):
//
//   producers + payoffs >= 5  → CountersMatterTheme  (theme present)
//   producers + payoffs >= 10 → CountersMatterStrong (primary engine)
//
// 5 is the floor at which the density is structurally meaningful
// rather than an incidental Llanowar Reborn one-off; 10 marks
// dedicated counters-tribal builds (Hardened Scales-style or
// Atraxa-shell), where the deck's gameplan IS counter scaling.
// ---------------------------------------------------------------------------

const (
	countersMatterThemeFloor  = 5
	countersMatterStrongFloor = 10
)

func computeCounterDensity(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report == nil || oracle == nil {
		return
	}
	for _, p := range report.Profiles {
		if !isCounterRelevantPermanent(p) {
			continue
		}
		entry := oracle.lookup(p.Name)
		if entry == nil {
			continue
		}
		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		if ot == "" {
			continue
		}
		if isCounterProducerOracle(ot) {
			dp.CounterProducers = append(dp.CounterProducers, p.Name)
		}
		if isCounterPayoffOracle(ot) {
			dp.CounterPayoffs = append(dp.CounterPayoffs, p.Name)
		}
	}
	sort.Strings(dp.CounterProducers)
	sort.Strings(dp.CounterPayoffs)

	total := len(dp.CounterProducers) + len(dp.CounterPayoffs)
	if total >= countersMatterThemeFloor {
		dp.CountersMatterTheme = true
	}
	if total >= countersMatterStrongFloor {
		dp.CountersMatterStrong = true
	}
}

// isCounterRelevantPermanent returns true when p is a permanent card
// type (creature / artifact / enchantment / planeswalker / battle).
// Lands are excluded — they don't carry counter-producer / counter-
// payoff oracle text at a rate that justifies inclusion (and the
// few that do, like Llanowar Reborn, place a single one-shot counter
// and don't constitute a recurring engine).
func isCounterRelevantPermanent(p CardProfile) bool {
	if p.IsLand {
		return false
	}
	tl := strings.ToLower(p.TypeLine)
	if tl == "" {
		return false
	}
	if strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery") {
		return false
	}
	return strings.Contains(tl, "creature") ||
		strings.Contains(tl, "artifact") ||
		strings.Contains(tl, "enchantment") ||
		strings.Contains(tl, "planeswalker") ||
		strings.Contains(tl, "battle")
}

// isCounterProducerOracle returns true when the lowercased oracle
// text indicates the card places +1/+1 counters on creatures.
// Matches both ETB-with-counter shapes (Hangarback Walker, Walking
// Ballista, Champion of Lambholt) and active-placement shapes
// (Pir Imaginative Rascal, Hardened Scales' replacement-side
// placement — actually that's pure payoff; placement triggers like
// Bramble Sovereign / Tuskguard Captain).
//
// "+1/+1 counter" is the universal anchor — any card that mentions
// the literal phrase in a placement context qualifies. The
// permanent-type gate upstream prevents instants/sorceries from
// leaking through.
func isCounterProducerOracle(ot string) bool {
	if !strings.Contains(ot, "+1/+1 counter") {
		return false
	}
	placementCues := []string{
		"put a +1/+1 counter",
		"put two +1/+1 counter",
		"put three +1/+1 counter",
		"put x +1/+1 counter",
		"put that many +1/+1 counter",
		"enters with a +1/+1 counter",
		"enters with two +1/+1 counter",
		"enters with x +1/+1 counter",
		"enters the battlefield with a +1/+1 counter",
		"enters the battlefield with x +1/+1 counter",
		"enters with another +1/+1 counter",
	}
	for _, cue := range placementCues {
		if strings.Contains(ot, cue) {
			return true
		}
	}
	return false
}

// isCounterPayoffOracle returns true when the lowercased oracle text
// indicates the card scales with, triggers on, or multiplies +1/+1
// counter placement. Three families:
//
//  1. Trigger-on-placement: "whenever one or more +1/+1 counters"
//     placed, "whenever a +1/+1 counter is placed on" (Bow of
//     Nylea-style payoffs; Hardened Scales-style triggers).
//
//  2. Scale-with: "for each +1/+1 counter on", "equal to the number
//     of +1/+1 counters" (Walking Ballista's pay-down ability,
//     Cathars' Crusade's stacking).
//
//  3. Multiplier replacement: "if one or more +1/+1 counters would
//     be put on" + "put that many plus one" / "twice that many"
//     (Hardened Scales, Doubling Season, Branching Evolution,
//     Conclave Mentor, Innkeeper's Talent, Master Biomancer's
//     "enters with N +1/+1 counters" amplifier shape).
//
// "Modified" is the modern keyword shorthand for "has +1/+1 counter
// OR aura OR equipment" — included as a payoff cue because every
// printed "modified" payoff (Skrelv's Hive, Glistening Dawn, Halo
// Hunter, Anim Pakal) functions as a Counters Matter payoff in a
// counter-heavy deck.
func isCounterPayoffOracle(ot string) bool {
	if !strings.Contains(ot, "+1/+1 counter") && !strings.Contains(ot, "modified") {
		// Master Biomancer's payoff text is "enters with N
		// additional +1/+1 counters" — covered by the "+1/+1
		// counter" check above.
		return false
	}
	payoffCues := []string{
		// Trigger-on-placement
		"whenever one or more +1/+1 counter",
		"whenever a +1/+1 counter is placed",
		"whenever a +1/+1 counter is put",
		// Scale-with
		"for each +1/+1 counter on",
		"equal to the number of +1/+1 counter",
		"number of +1/+1 counter",
		// Multiplier replacement / amplifier
		"that many plus one +1/+1 counter",
		"twice that many +1/+1 counter",
		// Master-Biomancer style: creature ETBs gain extra counters
		"with an additional +1/+1 counter",
		"with that many additional +1/+1 counter",
		// Sac-counters payoffs and reset-on-LTB (Ozolith family)
		"remove a +1/+1 counter",
		"if it had +1/+1 counter",
		// Modified shorthand
		"modified creature",
		"creatures you control are modified",
		"a modified creature",
		"each modified creature",
	}
	for _, cue := range payoffCues {
		if strings.Contains(ot, cue) {
			return true
		}
	}
	return false
}
