package outcome

// widen9.go — OUTCOME interpreter phase 9 (r63c): the next band of
// effect AST kinds the interpreter did not yet model — three whole
// effect TYPES (not ModKinds) that had zero in-scope coverage. Same
// independence contract: expected deltas from the AST + the harness's
// own BoardSpec, never from engine resolution.
//
// New coverage classes:
//
//	SET-LIFE     "your/target/each player's life total becomes N"
//	    (SetLife): the seat's life moves to the literal N from the
//	    scaffold's starting 20. Self / each-opponent bases are
//	    deterministic; a policy-targeted "target player" is a two-seat
//	    disjunction (a wrong N matches neither). Non-literal amounts
//	    ("becomes your starting life total") are out of scope.
//	COPY-PERMANENT (token) "create a token that's a copy of <creature>"
//	    (CopyPermanent, AsToken): every one of the 49 corpus shapes is a
//	    token copy. The engine copies a battlefield creature; on the
//	    scaffold every creature is a 4/4, so the new token adds +1
//	    battlefield / +4 power / +4 toughness for the controller. Only
//	    creature-ish / contextual copy targets are modeled (an explicitly
//	    noncreature copy target is a separate engine question).
//	EXTRA-TURN / EXTRA-COMBAT  (ExtraTurn / ExtraCombat): both increment
//	    a pending-phase counter — zero snapshot-observable delta. A
//	    resolver that drew/created/etc. instead is caught against the
//	    zero expectation.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffoldStartingLife is the life total every seat begins with on the
// harness board (gameengine.NewGameState default, CR §103.3 baseline).
const scaffoldStartingLife = 20

// copyableCreatureBase reports whether a CopyPermanent target base names
// a creature or a contextual referent that, on the scaffold, resolves to
// a creature. The engine copies a battlefield creature regardless of the
// filter, so an explicitly-noncreature base (artifact/enchantment/land/
// planeswalker) is a distinct question kept out of scope.
func copyableCreatureBase(b string) bool {
	switch normBase(b) {
	case "creature", "that", "this", "it", "another", "the", "enchanted",
		"of", "card", "":
		return true
	}
	return false
}

// leafPhase9 models the deterministic phase-9 effect types. Returns
// (handled, ok): handled=false → fall through.
func leafPhase9(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.SetLife:
		n, ok := e.Amount.IntVal()
		if !ok {
			return true, false // non-literal amount (e.g. "starting"): out of scope
		}
		delta := n - scaffoldStartingLife
		base := normBase(e.Target.Base)
		each := e.Target.Quantifier == "each" || e.Target.Quantifier == "all"
		switch {
		case base == "self" || base == "you" || base == "controller" || base == "":
			if each {
				return true, false // "each ... you control" is not a player set-life
			}
			if delta != 0 {
				d.LifeBySeat[spec.Controller] += delta
			}
			return true, true
		case base == "opponent" || base == "each_opponent":
			if delta != 0 {
				d.LifeBySeat[spec.Opponent] += delta
			}
			return true, true
		case (base == "player" || base == "each_player") && each:
			if delta != 0 {
				d.LifeBySeat[spec.Controller] += delta
				d.LifeBySeat[spec.Opponent] += delta
			}
			return true, true
		}
		// Single policy-targeted "target player": handled as a disjunction
		// in expandSetValuedLeaf9, never folded here.
		return true, false

	case *gameast.CopyPermanent:
		if !e.AsToken {
			// "<this> becomes a copy of <creature>": on the scaffold the
			// source is already a 4/4, so a creature-copy is a zero P/T
			// delta — but the layer-system interaction is out of this
			// band's scope; skip rather than assert a trivial zero.
			return true, false
		}
		if !copyableCreatureBase(e.Target.Base) {
			return true, false
		}
		// The engine copies a non-source battlefield creature; with none
		// available it no-ops (zero delta expected).
		if spec.OppCreatures+spec.OwnCreatures == 0 {
			return true, true
		}
		d.BattlefieldBySeat[spec.Controller]++
		d.PowerSum += scaffoldCreaturePT
		d.ToughSum += scaffoldCreaturePT
		return true, true

	case *gameast.ExtraTurn:
		return true, true // pending-turn counter: zero snapshot delta

	case *gameast.ExtraCombat:
		return true, true // pending-combat counter: zero snapshot delta
	}
	return false, false
}

// expandSetValuedLeaf9 intercepts the policy-targeted single-player
// SetLife as a two-seat disjunction. Returns (deltas, handled, ok).
func expandSetValuedLeaf9(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool, bool) {
	e, ok := eff.(*gameast.SetLife)
	if !ok {
		return nil, false, false
	}
	base := normBase(e.Target.Base)
	each := e.Target.Quantifier == "each" || e.Target.Quantifier == "all"
	// Only the single policy-targeted "target player" needs the
	// disjunction; the deterministic bases fall through to leafPhase9.
	if !(base == "player" && !each && e.Target.Targeted) {
		return nil, false, false
	}
	n, ok := e.Amount.IntVal()
	if !ok {
		return nil, true, false
	}
	delta := n - scaffoldStartingLife
	var out []*Delta
	for _, pre := range prefixes {
		for _, seat := range []int{spec.Controller, spec.Opponent} {
			b := pre.clone()
			if delta != 0 {
				b.LifeBySeat[seat] += delta
			}
			out = append(out, b)
		}
	}
	if len(out) > maxDisjuncts {
		return nil, true, false
	}
	return out, true, true
}
