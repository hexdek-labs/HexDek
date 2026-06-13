package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// counterspells_mr2_r60.go — second batch of shard M-R counterspells
// that parsed to bare typed_spell_effect nodes (handled nowhere) and
// countered NOTHING. Reuses mrCounterFunc / findCounterableSpell from
// counterspells_mr_r60.go. For "unless controller pays {N}" cards we
// follow the engine convention (counter, assume no pay).
//
// One new self-registering file (init() + AddResetHook); no shared
// registry edits.
func init() {
	registerCounterspellsMR2R60(Global())
	AddResetHook(registerCounterspellsMR2R60)
}

func registerCounterspellsMR2R60(r *Registry) {
	if r == nil {
		return
	}
	// Unconditional hard counters.
	r.OnResolve("Neutralize", mrCounterFunc("Neutralize", "neutralize", nil, 0))
	r.OnResolve("Neutralizing Blast", mrCounterFunc("Neutralizing Blast", "neutralizing_blast", mrMulticolored, 0))
	// Counter-unless-pay {N} family (counter per engine convention).
	r.OnResolve("Mindstatic", mrCounterFunc("Mindstatic", "mindstatic", nil, 6))
	r.OnResolve("Miscalculation", mrCounterFunc("Miscalculation", "miscalculation", nil, 2))
	r.OnResolve("Miscast", mrCounterFunc("Miscast", "miscast", mrInstantOrSorcery, 3))
	r.OnResolve("Rethink", mrCounterFunc("Rethink", "rethink", nil, 1))
	r.OnResolve("Revolutionary Rebuff", mrCounterFunc("Revolutionary Rebuff", "revolutionary_rebuff", mrNonArtifact, 2))
	r.OnResolve("Override", mrCounterFunc("Override", "override", nil, 1))
	r.OnResolve("Oppressive Will", mrCounterFunc("Oppressive Will", "oppressive_will", nil, 1))
	r.OnResolve("Rakshasa's Disdain", mrCounterFunc("Rakshasa's Disdain", "rakshasas_disdain", nil, 1))
}

func mrInstantOrSorcery(si *gameengine.StackItem) bool {
	return isInstantSpell(si) || isSorcerySpell(si)
}

func mrNonArtifact(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	for _, t := range si.Card.Types {
		if t == "artifact" {
			return false
		}
	}
	return true
}

func mrMulticolored(si *gameengine.StackItem) bool {
	return si != nil && si.Card != nil && len(si.Card.Colors) >= 2
}
