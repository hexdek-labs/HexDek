package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrilTheMiststalker wires Uril, the Miststalker.
//
// Oracle text (Scryfall, verified):
//
//	Hexproof
//	Uril, the Miststalker gets +2/+2 for each Aura attached to it.
//
// Implementation (R42b stub port):
//   - Hexproof: AST keyword pipeline.
//   - +2/+2 per Aura: not a real §613 layer-7c continuous effect at the
//     per_card layer — the engine doesn't yet expose a per-permanent
//     recompute hook tied to aura attach/detach. Approximation:
//     recompute on every Uril-bearing event we can hook (ETB on Uril,
//     nonland_permanent_etb, permanent_ltb). Each recompute scans
//     Uril's controller's battlefield for Auras with AttachedTo == Uril
//     and replaces the "uril_aura_buff" Modification with a fresh
//     entry sized to the current count (P=+2N, T=+2N, permanent
//     duration). The replace-don't-stack approach keeps the buff
//     correct under Aura churn (e.g. an Aura bounced from Uril → its
//     LTB fires → recompute drops the buff).
func registerUrilTheMiststalker(r *Registry) {
	r.OnETB("Uril, the Miststalker", urilTheMiststalkerETB)
	r.OnTrigger("Uril, the Miststalker", "nonland_permanent_etb", urilRecomputeOnAnyETB)
	r.OnTrigger("Uril, the Miststalker", "permanent_ltb", urilRecomputeOnAnyLTB)
}

const urilAuraBuffTag = "uril_aura_buff"

func urilTheMiststalkerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "uril_the_miststalker_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	count := urilRecomputeAuraBuff(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"auras": count,
		"buff":  2 * count,
	})
}

func urilRecomputeOnAnyETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	urilTriggerRecompute(gs, perm)
}

func urilRecomputeOnAnyLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	urilTriggerRecompute(gs, perm)
}

func urilTriggerRecompute(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "uril_aura_buff_recompute"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	count := urilRecomputeAuraBuff(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"auras": count,
		"buff":  2 * count,
	})
}

// urilRecomputeAuraBuff drops every existing "uril_aura_buff"
// Modification on perm and appends a fresh one sized to the current
// count of Auras attached to perm. Returns the Aura count.
func urilRecomputeAuraBuff(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	// Strip any previous Uril buff entries (tagged via Source-marker
	// stored in Duration since Modification has no Source field).
	mods := perm.Modifications[:0]
	for _, m := range perm.Modifications {
		if m.Duration == urilAuraBuffTag {
			continue
		}
		mods = append(mods, m)
	}
	perm.Modifications = mods

	count := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.AttachedTo != perm {
				continue
			}
			if !cardHasType(p.Card, "aura") {
				continue
			}
			count++
		}
	}
	if count > 0 {
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:     2 * count,
			Toughness: 2 * count,
			Duration:  urilAuraBuffTag,
			Timestamp: gs.NextTimestamp(),
		})
	}
	gs.InvalidateCharacteristicsCache()
	return count
}
