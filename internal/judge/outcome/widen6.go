package outcome

// widen6.go — OUTCOME interpreter phase 6 (r63): keyword grants become
// observable, plus the next long-tail classes. Same independence
// contract as every phase: expected deltas derive from the AST + the
// BoardSpec the harness itself built — NEVER from engine resolution.
//
// New coverage classes:
//
//	ABILITY GRANTS    a new observation dimension counts the
//	    GrantedAbilities entries across battlefields. "Target creature
//	    gains flying/deathtouch and lifelink/…" lands exactly one entry
//	    on one permanent (+1); "creatures you control gain …" lands one
//	    per matching permanent. Compound names are one entry by the
//	    engine's storage model — the AGGREGATE entry count is the
//	    rules-visible fact (one grant event per object).
//	SELF-REMOVAL      "destroy/exile this …" — the source leaves for
//	    the graveyard/exile.
//	ABSENT-BASE SWEEPS removal/tap of base types the scaffold provably
//	    lacks (forests, walls, auras, equipment …) must fizzle.
//	TAP ALL CREATURES every untapped creature (both sides + source)
//	    flips tapped.
//	EACH-OPPONENT DRAIN  the parser's {base:"opponent", quantifier:
//	    "each"} emission for "each opponent loses N life".
//	TARGETED DISCARD  "target player discards N": harmful targeting
//	    picks the opponent.
//	COUNTER DOUBLING  op "double" on a board with no counters is a
//	    provable no-op.
//	ABILITY-COUNTERING  Stifle-class counters fizzle against a stack
//	    holding only a spell.
//	ANY-GRAVEYARD REANIMATION  from_zone "any_graveyard": the source
//	    graveyard is a policy pick — disjunction over both seats.
//	UP-TO-ONE BOUNCE  declining is legal — fizzle ∪ the harmful pick
//	    variants.
//	CONTEXTUAL UNTAP  "untap it"/"untap enchanted creature": every
//	    plausible referent on the scaffold is already untapped — the
//	    resolution must be a no-op.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// absentBases are filter bases (incl. parser plural/subtype emissions)
// that provably match nothing on the harness scaffold.
var absentBases = map[string]bool{
	"forest": true, "forests": true, "island": true, "islands": true,
	"swamp": true, "swamps": true, "mountain": true, "mountains": true,
	"plains": true, "wall": true, "walls": true, "aura": true,
	"auras": true, "equipment": true, "planeswalker": true,
	"planeswalkers": true, "battle": true, "battles": true,
	"token": true, "tokens": true,
}

// expandSetValuedLeaf6 intercepts the phase-6 set-valued leaves.
func expandSetValuedLeaf6(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool, bool) {
	switch e := eff.(type) {
	case *gameast.TapEffect:
		// "tap all creatures [your opponents control / you control]":
		// the parser drops controller riders from the filter (r63
		// phase-6 audit: Tempest Caller's "target opponent controls"
		// arrives as a bare all-creatures filter, and the ENGINE still
		// taps only the opponent's side) — the matched side is
		// ambiguous from the AST alone. Disjunction over the three
		// legal side-scopes; a tap-nothing or wrong-amount bug matches
		// none of them.
		if normBase(e.Target.Base) != "creature" || !cleanFilter(e.Target) {
			return nil, false, false
		}
		if e.Target.Quantifier != "each" && e.Target.Quantifier != "all" {
			return nil, false, false
		}
		own := spec.OwnCreatures
		if spec.SrcIsCreature {
			own++
		}
		scopes := map[int]bool{}
		if e.Target.YouControl {
			scopes[own] = true
		} else if e.Target.OpponentControls {
			scopes[spec.OppCreatures] = true
		} else {
			scopes[own+spec.OppCreatures] = true
			scopes[own] = true
			scopes[spec.OppCreatures] = true
		}
		var out []*Delta
		for _, pre := range prefixes {
			for count := range scopes {
				if count == 0 {
					continue
				}
				b := pre.clone()
				b.Tapped += count
				out = append(out, b)
			}
		}
		if len(out) == 0 || len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true

	case *gameast.Reanimate:
		// from_zone "any_graveyard": WHICH graveyard is policy; both are
		// stocked with known 4/4 creature cards. The card returns to the
		// CONTROLLER's battlefield either way.
		if e.FromZone != "any_graveyard" {
			return nil, false, false
		}
		if !cleanFilter(e.Query) || spec.GraveyardCreatures < 1 {
			return nil, true, false
		}
		if e.Controller != "" && e.Controller != "you" {
			return nil, true, false
		}
		if len(e.WithModifications) > 0 {
			return nil, true, false
		}
		base := normBase(e.Query.Base)
		if base != "creature_card" && base != "creature card" && base != "creature" && base != "card" {
			return nil, true, false
		}
		tapped := 0
		switch e.Destination {
		case "", "battlefield":
		case "battlefield_tapped":
			tapped = 1
		default:
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			for _, gySeat := range []int{spec.Controller, spec.Opponent} {
				b := pre.clone()
				b.GraveyardBySeat[gySeat]--
				b.BattlefieldBySeat[spec.Controller]++
				b.PowerSum += scaffoldCreaturePT
				b.ToughSum += scaffoldCreaturePT
				b.Tapped += tapped
				out = append(out, b)
			}
		}
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true

	case *gameast.Bounce:
		// "return up to one target <base> to its owner's hand":
		// declining is legal — fizzle ∪ the harmful-pick variants.
		if e.Target.Quantifier != "up_to_n" {
			return nil, false, false
		}
		if n, ok := upToCount(e.Target); !ok || n != 1 {
			return nil, true, false
		}
		if !cleanFilter(e.Target) || e.Target.YouControl {
			return nil, true, false
		}
		dest := e.To
		if dest == "" {
			dest = "owners_hand"
		}
		if dest != "owners_hand" {
			return nil, true, false
		}
		base := normBase(e.Target.Base)
		var pool, tappedPool int
		creature := false
		switch base {
		case "creature":
			pool, creature = spec.OppCreatures, true
		case "artifact":
			pool, tappedPool = spec.OppArtifacts+spec.OppTappedArtifacts, spec.OppTappedArtifacts
		case "enchantment":
			pool = spec.OppEnchants
		case "land":
			pool = spec.OppLands
		default:
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			out = append(out, pre.clone()) // declined / fizzled
			if pool == 0 {
				continue
			}
			b := pre.clone()
			b.BattlefieldBySeat[spec.Opponent]--
			b.HandBySeat[spec.Opponent]++
			if creature {
				b.PowerSum -= scaffoldCreaturePT
				b.ToughSum -= scaffoldCreaturePT
			}
			out = append(out, b)
			if tappedPool > 0 {
				t := b.clone()
				t.Tapped--
				out = append(out, t)
			}
		}
		if len(out) == 0 || len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true
	}
	return nil, false, false
}

// upToCount extracts the N of an up_to_n filter (default 1).
func upToCount(f gameast.Filter) (int, bool) {
	if f.Count == nil {
		return 1, true
	}
	if v, ok := f.Count.IntVal(); ok && v > 0 {
		return v, true
	}
	return 0, false
}

// leafPhase6 extends the leaf dispatch with phase-6 kinds. Returns
// (handled, ok): handled=false → fall through to accumulate.
func leafPhase6(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.GrantAbility:
		return leafGrantAbility(spec, e, d)

	case *gameast.LoseLife:
		// Parser emission {base:"opponent", quantifier:"each"} for
		// "each opponent loses N life" (Agent of Masks class).
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 {
			return false, false
		}
		if normBase(e.Target.Base) == "opponent" &&
			(e.Target.Quantifier == "each" || e.Target.Quantifier == "all") {
			d.LifeBySeat[spec.Opponent] -= n
			return true, true
		}
		return false, false

	case *gameast.Discard:
		// "target player discards N": harmful targeting picks the
		// opponent (chosen_by only affects WHICH cards, not the count).
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.HandSize {
			return false, false
		}
		if normBase(e.Target.Base) == "player" && e.Target.Targeted &&
			cleanFilter(e.Target) &&
			e.Target.Quantifier != "each" && e.Target.Quantifier != "all" {
			d.HandBySeat[spec.Opponent] -= n
			d.GraveyardBySeat[spec.Opponent] += n
			return true, true
		}
		return false, false

	case *gameast.Destroy:
		base := normBase(e.Target.Base)
		if base == "self" || base == "this" || base == "it" {
			// Self-destruction rider: the source dies.
			d.BattlefieldBySeat[spec.Controller]--
			d.GraveyardBySeat[spec.Controller]++
			if spec.SrcIsCreature {
				d.PowerSum -= scaffoldCreaturePT
				d.ToughSum -= scaffoldCreaturePT
			}
			return true, true
		}
		if absentBases[base] && cleanFilter(e.Target) {
			return true, true // no such permanent exists: provable fizzle
		}
		return false, false

	case *gameast.Exile:
		if e.Until != "" {
			return false, false
		}
		base := normBase(e.Target.Base)
		if base == "self" || base == "this" || base == "it" {
			d.BattlefieldBySeat[spec.Controller]--
			d.ExileBySeat[spec.Controller]++
			if spec.SrcIsCreature {
				d.PowerSum -= scaffoldCreaturePT
				d.ToughSum -= scaffoldCreaturePT
			}
			return true, true
		}
		if absentBases[base] && cleanFilter(e.Target) {
			return true, true
		}
		return false, false

	case *gameast.CounterMod:
		// op "double" on a board that has no counters of the kind is a
		// provable no-op (2 × 0 = 0) — gated on no counters having been
		// added earlier in this sequence.
		if e.Op != "double" {
			return false, false
		}
		if len(d.CountersByKind) > 0 {
			return true, false
		}
		return true, true

	case *gameast.CounterSpell:
		// Stifle-class ability countering: the staged stack item is a
		// SPELL, so an ability-filtered counter must fizzle.
		base := normBase(e.Target.Base)
		if base == "activated" || base == "activated_ability" ||
			base == "triggered" || base == "triggered_ability" ||
			base == "ability" || base == "activated or triggered ability" {
			return true, true // zero delta
		}
		return false, false

	case *gameast.UntapEffect:
		// Contextual referents ("untap it", "untap enchanted creature"):
		// every plausible referent on the scaffold is already untapped
		// (the source is untapped; no auras exist) — the resolution must
		// be a no-op either way.
		base := normBase(e.Target.Base)
		if base == "that_thing" || base == "that" || base == "enchanted_creature" ||
			base == "enchanted creature" {
			return true, true
		}
		return false, false
	}
	return false, false
}

// leafGrantAbility folds a keyword/restriction grant into d. The
// engine stores one GrantedAbilities entry per granted object (a
// compound "trample and lifelink" is one entry), so the entry COUNT
// per object is the aggregate.
func leafGrantAbility(spec BoardSpec, e *gameast.GrantAbility, d *Delta) (bool, bool) {
	if strings.TrimSpace(e.AbilityName) == "" {
		return true, false
	}
	base := normBase(e.Target.Base)
	q := e.Target.Quantifier

	// Self-grant ("~ gains lifelink until end of turn").
	if base == "self" || base == "it" || base == "this" {
		d.AbilityGrants++
		return true, true
	}
	if base != "creature" || !cleanFilter(e.Target) {
		// Subtype/adjective-restricted grants ride the engine's
		// fallback-to-source when no candidate matches — a systemic
		// engine-lane issue (see r63 phase-6 report), not modeled here.
		return true, false
	}
	switch q {
	case "", "one":
		// A creature candidate always exists on the scaffold.
		d.AbilityGrants++
		return true, true
	case "each", "all":
		own := spec.OwnCreatures
		if spec.SrcIsCreature {
			own++
		}
		count := own + spec.OppCreatures
		if e.Target.YouControl {
			count = own
		} else if e.Target.OpponentControls {
			count = spec.OppCreatures
		}
		if count == 0 {
			return true, false
		}
		d.AbilityGrants += count
		return true, true
	}
	return true, false
}
