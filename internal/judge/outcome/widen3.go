package outcome

// widen3.go — OUTCOME interpreter phase 3 (r63): the harder effect
// kinds. Same independence contract: expected deltas from the AST +
// the BoardSpec, never from engine resolution.
//
//	COPY        — token copies of battlefield permanents (CreateToken
//	              with IsCopyOf): the copy's printed body joins the
//	              board sums (52 corpus shapes).
//	FOR-EACH    — draw_per / lose_life_per ModificationEffects with a
//	              whitelisted raw filter: delta scales by the count of
//	              matching objects on the harness board (72 shapes).
//	MULTI-PICK  — Choice with Pick=N>1 / OrMore ("choose two", escalate
//	              "choose one or more"): the expectation set expands to
//	              all legal option subsets, capped at maxDisjuncts (68).
//	REPLACEMENT — "enters with N +1/+1 counters" self-replacements (390
//	              shapes) checked through the ETB scenario in
//	              etb_counters.go.
//	X-DIVIDED   — already covered by phase 2 (amountVal + Divided);
//	              pinned by unit test this phase.
//	LAYER P/T   — becomes_pt/switch parse census came back as layer
//	              statics or absent; reported, deferred (needs the
//	              layer-engine harness, not effect resolution).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// perFilterCount resolves a whitelisted for-each filter string against
// the harness board. ok=false → out of scope.
func perFilterCount(spec BoardSpec, raw string) (int, bool) {
	f := strings.ToLower(strings.TrimSpace(raw))
	own := spec.OwnCreatures
	if spec.SrcIsCreature {
		own++
	}
	switch f {
	case "creature you control", "creatures you control":
		return own, true
	case "creature", "creatures":
		return own + spec.OppCreatures, true
	case "land you control", "lands you control":
		return spec.OwnTappedLands, true
	case "artifact you control", "artifacts you control":
		return 0, true
	case "tapped creature target opponent controls",
		"tapped creature an opponent controls":
		return 0, true // opponent creatures on the board are untapped
	case "creature target opponent controls",
		"creature an opponent controls":
		return spec.OppCreatures, true
	}
	return 0, false
}

// leafPhase3 extends the leaf dispatch with phase-3 kinds. Returns
// (handled, ok): handled=false → fall through to earlier phases.
func leafPhase3(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.ModificationEffect:
		switch e.ModKind {
		case "draw_per":
			if len(e.Args) == 0 {
				return true, false
			}
			filter, _ := e.Args[0].(string)
			n, ok := perFilterCount(spec, filter)
			if !ok {
				return true, false
			}
			if n > spec.LibrarySize {
				return true, false
			}
			d.HandBySeat[spec.Controller] += n
			d.LibraryBySeat[spec.Controller] -= n
			return true, true
		case "lose_life_per":
			if len(e.Args) == 0 {
				return true, false
			}
			filter, _ := e.Args[0].(string)
			n, ok := perFilterCount(spec, filter)
			if !ok {
				return true, false
			}
			d.LifeBySeat[spec.Controller] -= n
			return true, true
		}
		switch e.ModKind {
		case "regenerate_typed", "restriction", "cast_trigger_tail",
			"trigger_tail_fragment", "trigger_fragment_upkeep",
			"attach", "gain_energy":
			// r63 phase 5 — zero-delta resolutions: regeneration
			// shields, restriction markers ("can't block"), delayed/
			// cast-trigger registrations, equipment attach, and energy
			// bookkeeping change no snapshot-observable count. A wrong
			// implementation that draws/mills/destroys instead is
			// caught by the zero expectation.
			return true, true
		}
		return true, false // other ModificationEffects: out of scope

	case *gameast.CreateToken:
		if e.IsCopyOf == nil {
			return false, false // plain tokens: phase-1 path
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return true, false
		}
		// Copy of a battlefield creature: every scaffold creature is a
		// 4/4, so the copy's body is deterministic.
		if normBase(e.IsCopyOf.Base) != "creature" ||
			len(e.IsCopyOf.Extra) > 0 || len(e.IsCopyOf.CreatureTypes) > 0 {
			return true, false
		}
		if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
			return true, false
		}
		d.BattlefieldBySeat[spec.Controller] += n
		d.PowerSum += scaffoldCreaturePT * n
		d.ToughSum += scaffoldCreaturePT * n
		if e.Tapped {
			d.Tapped += n
		}
		return true, true
	}
	return false, false
}

// expandChoiceSubsets handles Choice with Pick=N>1 or OrMore by
// expanding the expectation set across all legal option subsets.
// Returns (deltas, handled, ok).
func expandChoiceSubsets(spec BoardSpec, e *gameast.Choice, prefixes []*Delta) ([]*Delta, bool, bool) {
	pick, isInt := e.Pick.IntVal()
	pickStr, isStr := e.Pick.StrVal()
	orMore := e.OrMore || (isStr && strings.HasSuffix(pickStr, "+"))
	if isInt && pick == 1 && !orMore {
		return nil, false, false // plain choose-one: phase-2 path
	}
	if !isInt && !orMore {
		return nil, true, false // "x" picks etc: out of scope
	}
	minPick := 1
	if isInt && pick > 0 {
		minPick = pick
	}
	n := len(e.Options)
	if n == 0 || n > 4 {
		return nil, true, false
	}
	var out []*Delta
	// Enumerate option subsets; size == minPick exactly, or >= minPick
	// when orMore.
	for mask := 1; mask < (1 << n); mask++ {
		size := 0
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				size++
			}
		}
		if size < minPick || (!orMore && size != minPick) {
			continue
		}
		branch := cloneAll(prefixes)
		bad := false
		for i := 0; i < n; i++ {
			if mask&(1<<i) == 0 {
				continue
			}
			var ok bool
			branch, ok = expand(spec, e.Options[i], branch)
			if !ok {
				bad = true
				break
			}
		}
		if bad {
			return nil, true, false // any unmodelable option: skip card
		}
		out = append(out, branch...)
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
	}
	if len(out) == 0 {
		return nil, true, false
	}
	return out, true, true
}
