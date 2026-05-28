package per_card

// Batch U (R60) — 5 high-impact unstubbed storm/ritual/finisher handlers.
//
//   - Cabal Ritual ({1}{B} instant — +3 mana, +5 at threshold)
//   - Pyretic Ritual ({1}{R} instant — +3 mana, sibling to Dark Ritual)
//   - Birgi, God of Storytelling ({1}{R} 3/2 — {R} on every cast)
//   - Heartless Hidetsugu ({2}{R}{R} 4/4 — tap to halve every player's life)
//   - Tendrils of Agony ({2}{B}{B} sorcery — storm + 2 drain / 2 gain)
//
// Pattern mirrors zz_batch_t_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests. All five reuse engine
// primitives (ManaPool / AddMana / DealDamage / LoseLife / GainLife);
// Tendrils relies on the existing storm pipeline in costs.go that
// pushes copies via ApplyStormCopies before priority resumes.

func init() {
	RegisterBatchUR60(Global())
	AddResetHook(RegisterBatchUR60)
}

// RegisterBatchUR60 registers the batch-U R60 handlers.
func RegisterBatchUR60(r *Registry) {
	if r == nil {
		return
	}
	registerCabalRitual(r)
	registerPyreticRitual(r)
	registerBirgiGodOfStorytelling(r)
	registerHeartlessHidetsugu(r)
	registerTendrilsOfAgony(r)
}
