package outcome

// widen7.go — OUTCOME interpreter phase 7 (r63): the last broadly
// tractable buckets before the parser-bound ceiling. Same independence
// contract: expected deltas from the AST + the harness's own
// BoardSpec, never from engine resolution.
//
// New coverage classes:
//
//	LIBRARY-TOP EXILE   "exile the top N cards of your library"
//	    (Filter.Base "library_top", the impulse-draw family): library
//	    −N, exile +N for the controller. The round-1 probe caught the
//	    engine silently no-oping this entire shape — fixed engine-side
//	    this phase.
//	EACH-OPPONENT DAMAGE  the {base:"opponent", quantifier:"each"}
//	    emission ("deals N damage to each opponent").
//	CONTEXTUAL-SUBJECT DAMAGE  "deals N damage to that creature":
//	    whichever creature the referent resolves to, the marked-damage
//	    aggregate is +N (every scaffold creature is a live 4/4).
//	SELF-NAMED COUNTERS  the parser's base "thing" for "put a +1/+1
//	    counter on ~": the source gains the counters.
//	CONTEXTUAL-PLAYER DISCARD  "that player / defending player
//	    discards N": the referent seat is ambiguous from the AST —
//	    disjunction over both seats; a wrong COUNT matches neither.
//	GAIN CONTROL        the permanent changes sides: battlefield count
//	    −1 opponent / +1 controller, all global sums unchanged —
//	    pick-independent for any opponent permanent.
//	ZERO-DELTA MODKINDS transform_self (the scaffold source is not a
//	    DFC — CR §712 transforming a non-DFC does nothing),
//	    roll_table_row fragments, level_marker registrations, and the
//	    Replacement effect kind (registration only).
//	COUNTER-SPELL TAILS base "abilities" fizzles (no ability staged);
//	    "noncreature spell" extras match the staged instant; typed
//	    spell bases (artifact/creature/enchantment spell) fizzle
//	    against it.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// expandSetValuedLeaf7 intercepts the phase-7 set-valued leaves.
func expandSetValuedLeaf7(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool, bool) {
	switch e := eff.(type) {
	case *gameast.Discard:
		// Contextual player referents ("that player", "defending
		// player"): the seat is unresolvable from the AST alone —
		// disjunction over both seats.
		base := normBase(e.Target.Base)
		contextual := base == "that_player" || base == "that player"
		if !contextual && base == "player" && !e.Target.Targeted &&
			extrasAllContextualPlayer(e.Target.Extra) {
			contextual = true
		}
		if !contextual {
			return nil, false, false
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.HandSize {
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			for _, seat := range []int{spec.Controller, spec.Opponent} {
				b := pre.clone()
				b.HandBySeat[seat] -= n
				b.GraveyardBySeat[seat] += n
				out = append(out, b)
			}
		}
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true
	}
	return nil, false, false
}

// extrasAllContextualPlayer reports whether every Extra adjective marks
// a contextual player referent (defending player et al).
func extrasAllContextualPlayer(extra []string) bool {
	if len(extra) == 0 {
		return false
	}
	for _, ex := range extra {
		if ex != "defending" && ex != "defending player" && ex != "that" {
			return false
		}
	}
	return true
}

// leafPhase7 extends the leaf dispatch with phase-7 kinds. Returns
// (handled, ok): handled=false → fall through.
func leafPhase7(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.Exile:
		// "Exile the top N cards of your library" (impulse family).
		if normBase(e.Target.Base) != "library_top" || e.Until != "" {
			return false, false
		}
		n := 1
		if e.Target.Count != nil {
			if v, ok := e.Target.Count.IntVal(); ok && v > 0 {
				n = v
			}
		}
		if n > spec.LibrarySize {
			n = spec.LibrarySize
		}
		d.LibraryBySeat[spec.Controller] -= n
		d.ExileBySeat[spec.Controller] += n
		return true, true

	case *gameast.Damage:
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 || e.Divided {
			return false, false
		}
		base := normBase(e.Target.Base)
		// "deals N damage to each opponent" — the {base:"opponent",
		// quantifier:"each"} emission.
		if base == "opponent" &&
			(e.Target.Quantifier == "each" || e.Target.Quantifier == "all") {
			d.LifeBySeat[spec.Opponent] -= n
			return true, true
		}
		// Contextual creature subjects ("that creature"): whichever
		// creature the referent resolves to, every scaffold creature is
		// a live 4/4 — the marked-damage aggregate is +N.
		if base == "that" || base == "that_creature" || base == "that creature" ||
			base == "this_creature" || base == "this creature" {
			if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
				return true, false
			}
			d.MarkedDamage += n
			return true, true
		}
		return false, false

	case *gameast.CounterMod:
		// The parser's base "thing" for "put a +1/+1 counter on ~":
		// the source itself gains the counters.
		if e.Op != "put" && e.Op != "" {
			return false, false
		}
		if normBase(e.Target.Base) != "thing" {
			return false, false
		}
		if d.BattlefieldBySeat[0]+d.BattlefieldBySeat[1] < 0 {
			return true, false // source may have left earlier in the sequence
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return true, false
		}
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		d.CountersByKind[kind] += n
		if spec.SrcIsCreature {
			switch kind {
			case "+1/+1":
				d.PowerSum += n
				d.ToughSum += n
			case "-1/-1":
				d.PowerSum -= n
				d.ToughSum -= n
			}
		}
		return true, true

	case *gameast.GainControl:
		// The stolen permanent changes sides: battlefield −1 opponent /
		// +1 controller; every global sum (P/T, tapped, counters) is
		// side-blind, so the swap is pick-independent.
		if !cleanFilter(e.Target) || e.Target.YouControl {
			return false, false
		}
		if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" ||
			e.Target.Quantifier == "up_to_n" || e.Target.Quantifier == "n" {
			return true, false
		}
		switch normBase(e.Target.Base) {
		case "permanent", "creature", "artifact", "enchantment", "land":
			// A candidate of every base exists on the opponent's side.
			d.BattlefieldBySeat[spec.Opponent]--
			d.BattlefieldBySeat[spec.Controller]++
			return true, true
		}
		return true, false

	case *gameast.Replacement:
		return true, true // registration only: zero observable delta

	case *gameast.CounterSpell:
		base := normBase(e.Target.Base)
		// Plural "abilities" emission of the Stifle class.
		if base == "abilities" {
			return true, true // fizzles: only a spell is staged
		}
		if spec.StackSpells < 1 {
			return false, false
		}
		// "Counter target noncreature spell": the staged instant IS a
		// noncreature spell — the counter lands.
		if base == "spell" && extrasAllNoncreature(e.Target.Extra) &&
			len(e.Target.CreatureTypes) == 0 && len(e.Target.ColorFilter) == 0 &&
			e.Target.ManaValueOp == "" {
			d.LiveStack--
			return true, true
		}
		// Typed spell bases that cannot match the staged instant
		// (Annul class): the counter fizzles.
		if base == "artifact" || base == "enchantment" || base == "creature" ||
			base == "planeswalker" || base == "artifact_or_enchantment" {
			return true, true // zero delta
		}
		return false, false
	}
	return false, false
}

// extrasAllNoncreature reports whether every Extra adjective still
// matches the staged instant spell ("non-creature" variants).
func extrasAllNoncreature(extra []string) bool {
	if len(extra) == 0 {
		return false
	}
	for _, ex := range extra {
		if ex != "non-creature" && ex != "noncreature" {
			return false
		}
	}
	return true
}
