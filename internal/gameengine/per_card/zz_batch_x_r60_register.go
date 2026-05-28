package per_card

// Batch X (R60) — 5 high-impact unstubbed board-wipe handlers.
//
//   - Supreme Verdict ({1}{W}{W}{U} — uncounterable creature wipe)
//   - Pernicious Deed ({1}{B}{G} enchantment — X-cost mass MV-bounded wipe)
//   - Vandalblast ({R} sorcery — single artifact / overload mass-opp-artifact wipe)
//   - Anger of the Gods ({1}{R}{R} sorcery — 3 damage + exile-instead-of-graveyard)
//   - Merciless Eviction ({4}{W}{B} sorcery — modal mass-exile by type)
//
// Pattern mirrors zz_batch_v_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests. All five reuse the existing
// destroyAllCreatures / DestroyPermanent / ExilePermanent primitives;
// Supreme Verdict stamps CostMeta["cannot_be_countered"] = true at
// cast time via OnCast (mirrors Dovin's Veto / Lier shape).

func init() {
	RegisterBatchXR60(Global())
	AddResetHook(RegisterBatchXR60)
}

// RegisterBatchXR60 registers the batch-X R60 handlers.
func RegisterBatchXR60(r *Registry) {
	if r == nil {
		return
	}
	registerSupremeVerdict(r)
	registerPerniciousDeed(r)
	registerVandalblast(r)
	registerAngerOfTheGods(r)
	registerMercilessEviction(r)
}
