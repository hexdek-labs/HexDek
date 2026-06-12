package outcome

// widen.go — r63 part 2: set-based (disjunctive) expected deltas for
// effect shapes whose outcome depends on a legal CHOICE the engine's
// policy makes — modal "choose one" (Choice), "you may" (Optional_),
// "if [condition]" (Conditional) — plus new leaf kinds: mill, discard,
// tap/untap, +X/+X pumps (Buff), divided damage, and {X} amounts pinned
// by the harness board.
//
// Independence note (the prime directive of this dimension): every
// expectation here derives from the AST + the BoardSpec the harness
// itself built — never from engine resolution code. Disjunctive
// expectations assert "the actual delta equals SOME legal mode's
// delta": weaker than a point assertion but still rules-parity (a
// wrong AMOUNT in any mode cannot match any member of the set).

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// maxDisjuncts caps the expectation-set size (Choice × Sequence
// cross-products); past the cap the card is skipped, never guessed.
const maxDisjuncts = 8

// amountVal resolves a NumberOrRef on the harness board: plain ints,
// or "x"/"X" pinned to spec.XValue. Anything else is out of scope.
func amountVal(spec BoardSpec, n gameast.NumberOrRef) (int, bool) {
	if v, ok := n.IntVal(); ok {
		return v, true
	}
	if s, ok := n.StrVal(); ok && (s == "x" || s == "X") && spec.XValue > 0 {
		return spec.XValue, true
	}
	return 0, false
}

// ExpectSet computes the set of legal expected deltas for eff. ok=false
// means out of scope (some branch is unmodelable). A single-outcome
// effect yields a one-element set.
func ExpectSet(spec BoardSpec, eff gameast.Effect) ([]*Delta, bool) {
	sets, ok := expand(spec, eff, []*Delta{NewDelta()})
	if !ok || len(sets) == 0 || len(sets) > maxDisjuncts {
		return nil, false
	}
	return sets, true
}

// clone deep-copies a delta.
func (d *Delta) clone() *Delta {
	c := NewDelta()
	for k, v := range d.LifeBySeat {
		c.LifeBySeat[k] = v
	}
	for k, v := range d.HandBySeat {
		c.HandBySeat[k] = v
	}
	for k, v := range d.LibraryBySeat {
		c.LibraryBySeat[k] = v
	}
	for k, v := range d.GraveyardBySeat {
		c.GraveyardBySeat[k] = v
	}
	for k, v := range d.ExileBySeat {
		c.ExileBySeat[k] = v
	}
	for k, v := range d.BattlefieldBySeat {
		c.BattlefieldBySeat[k] = v
	}
	for k, v := range d.CountersByKind {
		c.CountersByKind[k] = v
	}
	c.MarkedDamage = d.MarkedDamage
	c.Tapped = d.Tapped
	c.PowerSum = d.PowerSum
	c.ToughSum = d.ToughSum
	return c
}

// expand maps each prefix delta through eff, branching on choice-like
// nodes. Returns ok=false when ANY reachable branch is unmodelable
// (skipping the whole card keeps the comparison sound).
func expand(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool) {
	switch e := eff.(type) {
	case *gameast.Sequence:
		cur := prefixes
		for _, item := range e.Items {
			var ok bool
			cur, ok = expand(spec, item, cur)
			if !ok || len(cur) > maxDisjuncts {
				return nil, false
			}
		}
		return cur, true

	case *gameast.Choice:
		pick, ok := e.Pick.IntVal()
		if !ok || pick != 1 || e.OrMore || len(e.Options) == 0 {
			return nil, false // only plain "choose one" in this pass
		}
		var out []*Delta
		for _, opt := range e.Options {
			branched, ok := expand(spec, opt, cloneAll(prefixes))
			if !ok {
				return nil, false
			}
			out = append(out, branched...)
		}
		if len(out) > maxDisjuncts {
			return nil, false
		}
		return out, true

	case *gameast.Optional_:
		if e.Body == nil {
			return prefixes, true
		}
		taken, ok := expand(spec, e.Body, cloneAll(prefixes))
		if !ok {
			return nil, false
		}
		out := append(cloneAll(prefixes), taken...) // declined ∪ taken
		if len(out) > maxDisjuncts {
			return nil, false
		}
		return out, true

	case *gameast.Conditional:
		// Condition truth isn't independently evaluable in this pass —
		// expect EITHER branch (or no-op when ElseBody is nil). Catches
		// wrong amounts in any branch; wrong-branch selection is a
		// phase-3 refinement (board-evaluable conditions).
		var out []*Delta
		if e.Body != nil {
			taken, ok := expand(spec, e.Body, cloneAll(prefixes))
			if !ok {
				return nil, false
			}
			out = append(out, taken...)
		}
		if e.ElseBody != nil {
			alt, ok := expand(spec, e.ElseBody, cloneAll(prefixes))
			if !ok {
				return nil, false
			}
			out = append(out, alt...)
		} else {
			out = append(out, cloneAll(prefixes)...)
		}
		if len(out) > maxDisjuncts {
			return nil, false
		}
		return out, true
	}

	// Leaf effects: apply to every prefix in place.
	for _, d := range prefixes {
		if !leaf(spec, eff, d) {
			return nil, false
		}
	}
	return prefixes, true
}

func cloneAll(ds []*Delta) []*Delta {
	out := make([]*Delta, len(ds))
	for i, d := range ds {
		out[i] = d.clone()
	}
	return out
}

// leaf folds a single deterministic effect into d (phase-1 kinds via
// accumulate, plus the part-2 widened kinds).
func leaf(spec BoardSpec, eff gameast.Effect, d *Delta) bool {
	switch e := eff.(type) {
	case *gameast.Mill:
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.LibrarySize {
			return false
		}
		base := normBase(e.Target.Base)
		each := e.Target.Quantifier == "each" || e.Target.Quantifier == "all"
		switch {
		case base == "self" || base == "you" || base == "" || base == "controller":
			d.LibraryBySeat[spec.Controller] -= n
			d.GraveyardBySeat[spec.Controller] += n
			return true
		case base == "opponent" || base == "each_opponent":
			d.LibraryBySeat[spec.Opponent] -= n
			d.GraveyardBySeat[spec.Opponent] += n
			return true
		case (base == "player" || base == "each_player") && each:
			d.LibraryBySeat[spec.Controller] -= n
			d.GraveyardBySeat[spec.Controller] += n
			d.LibraryBySeat[spec.Opponent] -= n
			d.GraveyardBySeat[spec.Opponent] += n
			return true
		case base == "target_player" || base == "target player" || base == "player":
			// Policy-targeted single player: the harness policy targets
			// the opponent for harmful mills.
			d.LibraryBySeat[spec.Opponent] -= n
			d.GraveyardBySeat[spec.Opponent] += n
			return true
		}
		return false

	case *gameast.Discard:
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.HandSize {
			return false
		}
		switch normBase(e.Target.Base) {
		case "self", "you", "", "controller":
			d.HandBySeat[spec.Controller] -= n
			d.GraveyardBySeat[spec.Controller] += n
			return true
		case "opponent", "each_opponent":
			d.HandBySeat[spec.Opponent] -= n
			d.GraveyardBySeat[spec.Opponent] += n
			return true
		}
		return false

	case *gameast.TapEffect:
		if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" ||
			e.Target.Quantifier == "up_to_n" || e.Target.Quantifier == "n" {
			return false // fan-out tap: later
		}
		if len(e.Target.Extra) > 0 || len(e.Target.CreatureTypes) > 0 || len(e.Target.ColorFilter) > 0 {
			return false
		}
		switch normBase(e.Target.Base) {
		case "creature", "artifact", "land", "permanent":
			// At least one untapped candidate of every base exists on
			// the default board.
			d.Tapped++
			return true
		}
		return false

	case *gameast.UntapEffect:
		if len(e.Target.Extra) > 0 || len(e.Target.CreatureTypes) > 0 || len(e.Target.ColorFilter) > 0 {
			return false
		}
		base := normBase(e.Target.Base)
		if base == "self" || base == "it" || base == "this" {
			return false // source tap-state shapes: later
		}
		// Tapped pools by side. The beneficial untap policy only ever
		// untaps the controller's OWN permanents when the choice is
		// optional (single target / "up to N") — untapping the
		// opponent's board is never the play (Teferi -1 finding).
		ownTapped := map[string]int{
			"land": spec.OwnTappedLands, "permanent": spec.OwnTappedLands,
			"artifact": 0, "creature": 0,
		}
		anyTapped := map[string]int{
			"land":      spec.OwnTappedLands,
			"artifact":  spec.OppTappedArtifacts,
			"permanent": spec.OwnTappedLands + spec.OppTappedArtifacts,
			"creature":  0,
		}
		own, knownOwn := ownTapped[base]
		all, knownAll := anyTapped[base]
		if !knownOwn || !knownAll {
			return false
		}
		if e.Target.YouControl {
			all = own
		}
		switch e.Target.Quantifier {
		case "", "one":
			// Side-correct picks only: the controller untaps their own
			// tapped permanents; with none, the pick no-ops.
			if own > 0 {
				d.Tapped--
			}
			return true
		case "up_to_n", "n":
			n := 1
			if e.Target.Count != nil {
				if v, ok := e.Target.Count.IntVal(); ok && v > 0 {
					n = v
				}
			}
			pool := own
			if e.Target.YouControl {
				pool = own
			}
			if n > pool {
				n = pool
			}
			d.Tapped -= n
			return true
		case "each", "all":
			d.Tapped -= all
			return true
		}
		return false

	case *gameast.Buff:
		if e.Duration != "" && e.Duration != "until_end_of_turn" {
			return false
		}
		if e.Power == 0 && e.Toughness == 0 {
			return false
		}
		if len(e.Target.Extra) > 0 || len(e.Target.CreatureTypes) > 0 || len(e.Target.ColorFilter) > 0 {
			return false
		}
		q := e.Target.Quantifier
		base := normBase(e.Target.Base)
		ownCreatures := spec.OwnCreatures
		if spec.SrcIsCreature {
			ownCreatures++
		}
		allCreatures := ownCreatures + spec.OppCreatures
		switch {
		case (base == "self" || base == "it" || base == "this") && q != "each" && q != "all":
			d.PowerSum += e.Power
			d.ToughSum += e.Toughness
			return true
		case base == "creature" && (q == "" || q == "one"):
			if allCreatures == 0 {
				return false
			}
			d.PowerSum += e.Power
			d.ToughSum += e.Toughness
			return true
		case base == "creature" && (q == "each" || q == "all") && e.Target.YouControl:
			d.PowerSum += e.Power * ownCreatures
			d.ToughSum += e.Toughness * ownCreatures
			return true
		case base == "creature" && (q == "each" || q == "all") && e.Target.OpponentControls:
			d.PowerSum += e.Power * spec.OppCreatures
			d.ToughSum += e.Toughness * spec.OppCreatures
			return true
		case base == "creature" && (q == "each" || q == "all"):
			d.PowerSum += e.Power * allCreatures
			d.ToughSum += e.Toughness * allCreatures
			return true
		}
		return false

	case *gameast.Damage:
		// Divided damage among creatures: the SPLIT is policy, the
		// TOTAL is rules.
		if e.Divided {
			n, ok := amountVal(spec, e.Amount)
			if !ok || n <= 0 {
				return false
			}
			if normBase(e.Target.Base) != "creature" ||
				len(e.Target.Extra) > 0 || len(e.Target.CreatureTypes) > 0 {
				return false
			}
			if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
				return false
			}
			d.MarkedDamage += n
			return true
		}
	}

	// Fall through to the phase-1 single-delta accumulator (which also
	// honors the X-pinning via its own amount checks below).
	return accumulate(spec, eff, d)
}
