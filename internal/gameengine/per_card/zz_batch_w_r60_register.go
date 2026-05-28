package per_card

// Batch W (R60) — 5 high-impact unstubbed draw-engine handlers.
//
//   - Phyrexian Arena ({1}{B}{B} enchantment — upkeep draw + 1 life)
//   - Toski, Bearer of Secrets ({2}{G} 1/1 — combat-damage cantrip)
//   - Beast Whisperer ({2}{G}{G} 2/3 — creature-cast cantrip)
//   - Guardian Project ({2}{G} enchantment — unique-name nontoken
//     creature ETB cantrip)
//   - The Great Henge ({7}{G}{G} legendary artifact — ETB life-gain
//     equal to its power + recurring creature-ETB +1/+1 counter +
//     cantrip)
//
// The five suggested in the prompt (Rhystic Study / Mystic Remora /
// Sylvan Library / Smothering Tithe / Esper Sentinel) are already
// stubbed under their own files; this batch picks the next tier of
// draw engines covering the same archetype-defining role.
//
// Pattern mirrors zz_batch_u_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests.

func init() {
	RegisterBatchWR60(Global())
	AddResetHook(RegisterBatchWR60)
}

// RegisterBatchWR60 registers the batch-W R60 handlers.
func RegisterBatchWR60(r *Registry) {
	if r == nil {
		return
	}
	registerPhyrexianArena(r)
	registerToski(r)
	registerBeastWhisperer(r)
	registerGuardianProject(r)
	registerTheGreatHenge(r)
}
