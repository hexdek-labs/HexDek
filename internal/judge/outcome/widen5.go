package outcome

// widen5.go — OUTCOME interpreter phase 5 (r63): the long-tail push.
// Same independence contract as every phase: expected deltas derive
// from the AST + the BoardSpec the harness itself built — NEVER from
// engine resolution code.
//
// New coverage classes this phase:
//
//	ZERO-DELTA RESOLUTIONS  effects whose resolution must change no
//	    observed count: damage-prevention shields (Prevent), regenerate
//	    shields, restriction markers ("can't block"), delayed/cast
//	    trigger registrations, equipment attach, energy bookkeeping,
//	    self-untap of the already-untapped source. A wrong
//	    implementation that draws/mills/buffs instead is caught.
//	GUARANTEED FAIL-TO-FIND  tutors whose query cannot match the
//	    stocked library (artifact/instant/subtype/positive-color
//	    queries against colorless, subtype-less fillers): legal
//	    fail-to-find, expected delta zero.
//	ZERO-CANDIDATE TARGETING removal/bounce restricted to board states
//	    the scaffold never holds (tapped/attacking/blocking creatures):
//	    the effect must fizzle with no legal target.
//	COLORLESS PASS-THROUGH   ColorExclude-only filters ("nonblack
//	    creature") behave as unrestricted on the colorless scaffold;
//	    positive ColorFilter never matches (zero delta).
//	ANY-TARGET ALT EMISSION  the parser's second any-target shape
//	    {base:"target", quantifier:"any"} joins the phase-4 player∨
//	    creature disjunction.
//	DISTRIBUTE COUNTERS      "distribute N +1/+1 counters among …":
//	    the SPLIT is policy, the TOTAL is rules.
//	TARGETED DRAW            "target player draws N": the seat pick is
//	    policy (disjunction over both seats); the amount is rules.
//	OWN-BOUNCE               "return target creature you control":
//	    the controller's own creature leaves for their hand.
//	PERMANENT-BOUNCE         set-valued: creature pick (P/T leaves) ∨
//	    non-creature pick.
//	COPY SPELL               a staged opponent stack spell is copied:
//	    live stack +1 (the copy is created on the stack, never popped
//	    by the harness).
//	WIDER TUTOR POOLS        creature-card queries draw from the
//	    filler pool; mandatory searches move min(count, pool).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// zeroCandidateExtras are filter adjectives the scaffold NEVER
// satisfies for creatures: no creature on the harness board is tapped,
// attacking, blocking, or flying.
var zeroCandidateExtras = map[string]bool{
	"tapped": true, "attacking": true, "blocking": true,
	"flying": true, "with flying": true, "attacking or blocking": true,
}

// extrasAllZeroCandidate reports whether every Extra adjective is in
// the zero-candidate whitelist (so the filter provably matches nothing
// on the scaffold).
func extrasAllZeroCandidate(extra []string) bool {
	if len(extra) == 0 {
		return false
	}
	for _, ex := range extra {
		if !zeroCandidateExtras[strings.ToLower(strings.TrimSpace(ex))] {
			return false
		}
	}
	return true
}

// colorlessNeverMatches reports whether a positive color filter is set
// (scaffold cards are colorless — they can never match it).
func colorlessNeverMatches(f gameast.Filter) bool {
	return len(f.ColorFilter) > 0
}

// cleanFilterExceptColorExclude is cleanFilter with ColorExclude
// permitted: colorless scaffold cards are never excluded by a
// "non<color>" restriction, so the filter behaves as unrestricted.
func cleanFilterExceptColorExclude(f gameast.Filter) bool {
	return len(f.Extra) == 0 && len(f.CreatureTypes) == 0 &&
		len(f.ColorFilter) == 0 && f.ManaValueOp == "" && !f.NonToken
}

// treeNeedsStackSpell reports whether the effect tree holds a kind the
// harness must stage a live opponent stack spell for (counterspells
// and spell-copying).
func treeNeedsStackSpell(eff gameast.Effect) bool {
	switch e := eff.(type) {
	case *gameast.CounterSpell, *gameast.CopySpell:
		return true
	case *gameast.Sequence:
		for _, it := range e.Items {
			if treeNeedsStackSpell(it) {
				return true
			}
		}
	case *gameast.Choice:
		for _, o := range e.Options {
			if treeNeedsStackSpell(o) {
				return true
			}
		}
	case *gameast.Optional_:
		if e.Body != nil {
			return treeNeedsStackSpell(e.Body)
		}
	case *gameast.Conditional:
		return (e.Body != nil && treeNeedsStackSpell(e.Body)) ||
			(e.ElseBody != nil && treeNeedsStackSpell(e.ElseBody))
	}
	return false
}

// expandSetValuedLeaf5 intercepts the phase-5 set-valued leaves.
// Returns (deltas, handled, ok).
func expandSetValuedLeaf5(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool, bool) {
	switch e := eff.(type) {
	case *gameast.Draw:
		// "target player draws N": WHICH player is policy (the r63
		// phase-5 audit caught the engine targeting the opponent —
		// reported as a policy smell, not a rules divergence); the
		// AMOUNT is rules. Disjunction over both seats: a wrong count
		// matches neither member.
		base := normBase(e.Target.Base)
		if base != "player" && base != "target_player" && base != "target player" {
			return nil, false, false
		}
		if !e.Target.Targeted || e.Target.Quantifier == "each" || e.Target.Quantifier == "all" {
			return nil, false, false
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.LibrarySize {
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			for _, seat := range []int{spec.Controller, spec.Opponent} {
				b := pre.clone()
				b.HandBySeat[seat] += n
				b.LibraryBySeat[seat] -= n
				out = append(out, b)
			}
		}
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true

	case *gameast.Bounce:
		// Permanent-base single bounce: the policy's pick is set-valued
		// across body classes — a creature pick takes its 4/4 with it,
		// a non-creature pick moves no P/T. Both land in the opponent's
		// hand (harmful targeting).
		base := normBase(e.Target.Base)
		if base != "permanent" {
			return nil, false, false
		}
		if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" ||
			e.Target.Quantifier == "up_to_n" || e.Target.Quantifier == "n" {
			return nil, true, false
		}
		if !cleanFilter(e.Target) || e.Target.YouControl {
			return nil, true, false
		}
		dest := e.To
		if dest == "" {
			dest = "owners_hand"
		}
		if dest != "owners_hand" && dest != "top_of_library" && dest != "bottom_of_library" {
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			for _, creaturePick := range []bool{false, true} {
				if creaturePick && spec.OppCreatures == 0 {
					continue
				}
				b := pre.clone()
				b.BattlefieldBySeat[spec.Opponent]--
				if dest == "owners_hand" {
					b.HandBySeat[spec.Opponent]++
				} else {
					b.LibraryBySeat[spec.Opponent]++
				}
				if creaturePick {
					b.PowerSum -= scaffoldCreaturePT
					b.ToughSum -= scaffoldCreaturePT
				} else {
					// Non-creature picks may take the tapped relic.
					tapped := b.clone()
					tapped.Tapped--
					out = append(out, tapped)
				}
				out = append(out, b)
			}
		}
		if len(out) == 0 || len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true
	}
	return nil, false, false
}

// leafPhase5 extends the leaf dispatch with phase-5 kinds. Returns
// (handled, ok): handled=false → fall through to earlier phases.
func leafPhase5(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.Prevent:
		// Damage-prevention shield: registration only, no observable
		// count may change at resolution.
		return true, true

	case *gameast.UntapEffect:
		// Self-untap: the scaffold source is already untapped, so the
		// resolution is a no-op (CR §701.21b untapping an untapped
		// permanent does nothing).
		base := normBase(e.Target.Base)
		if base == "self" || base == "it" || base == "this" {
			return true, true
		}
		return false, false

	case *gameast.Destroy:
		return leafRestrictedRemoval5(spec, e.Target, "graveyard", d)

	case *gameast.Exile:
		if e.Until != "" {
			return false, false
		}
		return leafRestrictedRemoval5(spec, e.Target, "exile", d)

	case *gameast.Bounce:
		// Own-bounce: "return (another) target creature you control to
		// its owner's hand" — the controller's own creature leaves.
		base := normBase(e.Target.Base)
		dest := e.To
		if dest == "" {
			dest = "owners_hand"
		}
		if base == "creature" && e.Target.YouControl && dest == "owners_hand" &&
			cleanFilter(e.Target) &&
			e.Target.Quantifier != "each" && e.Target.Quantifier != "all" &&
			e.Target.Quantifier != "up_to_n" && e.Target.Quantifier != "n" {
			own := spec.OwnCreatures
			if spec.SrcIsCreature {
				own++
			}
			if own == 0 {
				return true, false
			}
			d.BattlefieldBySeat[spec.Controller]--
			d.HandBySeat[spec.Controller]++
			d.PowerSum -= scaffoldCreaturePT
			d.ToughSum -= scaffoldCreaturePT
			return true, true
		}
		// Zero-candidate adjective bounce (tapped/attacking/blocking).
		if base == "creature" && extrasAllZeroCandidate(e.Target.Extra) &&
			len(e.Target.CreatureTypes) == 0 && len(e.Target.ColorFilter) == 0 &&
			e.Target.ManaValueOp == "" {
			return true, true // fizzles: zero delta
		}
		// Positive-color bounce: colorless scaffold never matches.
		if base == "creature" && colorlessNeverMatches(e.Target) &&
			len(e.Target.Extra) == 0 && len(e.Target.CreatureTypes) == 0 {
			return true, true
		}
		return false, false

	case *gameast.CounterMod:
		// "distribute N <kind> counters among …": the split across
		// targets is policy, the total is rules. The free-text base
		// must reference creatures (candidates exist on the scaffold).
		if e.Op != "distribute" {
			return false, false
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return true, false
		}
		if !strings.Contains(strings.ToLower(e.Target.Base), "creature") {
			return true, false
		}
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		ptShift := 0
		switch kind {
		case "+1/+1":
			ptShift = n
		case "-1/-1":
			ptShift = -n
		}
		d.CountersByKind[kind] += n
		d.PowerSum += ptShift
		d.ToughSum += ptShift
		return true, true

	case *gameast.CopySpell:
		// The harness staged one live opponent spell; copying it puts
		// one more item on the stack (the copy is created, not cast,
		// and the harness never pops it).
		if spec.StackSpells < 1 {
			return true, false
		}
		if !cleanFilter(e.Target) {
			return true, false
		}
		d.LiveStack++
		return true, true
	}
	return false, false
}

// leafRestrictedRemoval5 covers the phase-5 single-target removal
// shapes phase 1 refuses: provably-zero-candidate adjective filters,
// positive-color filters on the colorless scaffold, and ColorExclude
// pass-through. Returns (handled, ok); handled=false falls through to
// the phase-1/4 paths.
func leafRestrictedRemoval5(spec BoardSpec, f gameast.Filter, destZone string, d *Delta) (bool, bool) {
	if f.Quantifier == "each" || f.Quantifier == "all" ||
		f.Quantifier == "up_to_n" || f.Quantifier == "n" {
		return false, false
	}
	if f.YouControl {
		return false, false
	}
	base := normBase(f.Base)

	// Zero-candidate adjectives: "destroy target tapped creature" has
	// no legal target on the scaffold — the effect must fizzle.
	if base == "creature" && extrasAllZeroCandidate(f.Extra) &&
		len(f.CreatureTypes) == 0 && len(f.ColorFilter) == 0 && f.ManaValueOp == "" {
		return true, true // zero delta
	}

	// Positive color filter: colorless scaffold cards never match.
	if (base == "creature" || base == "artifact" || base == "enchantment" || base == "permanent") &&
		colorlessNeverMatches(f) &&
		len(f.Extra) == 0 && len(f.CreatureTypes) == 0 && f.ManaValueOp == "" {
		return true, true // zero delta
	}

	// ColorExclude pass-through: "nonblack creature" behaves as
	// unrestricted against colorless cards (Doom Blade class).
	if len(f.ColorExclude) > 0 && cleanFilterExceptColorExclude(f) {
		var candidates int
		creature := false
		switch base {
		case "creature":
			candidates, creature = spec.OppCreatures, true
		case "artifact":
			candidates = spec.OppArtifacts + spec.OppTappedArtifacts
		case "enchantment":
			candidates = spec.OppEnchants
		default:
			return true, false // "permanent" stays set-valued
		}
		if candidates == 0 {
			return true, false
		}
		d.BattlefieldBySeat[spec.Opponent]--
		switch destZone {
		case "graveyard":
			d.GraveyardBySeat[spec.Opponent]++
		case "exile":
			d.ExileBySeat[spec.Opponent]++
		default:
			return true, false
		}
		if creature {
			d.PowerSum -= scaffoldCreaturePT
			d.ToughSum -= scaffoldCreaturePT
		}
		return true, true
	}
	return false, false
}
