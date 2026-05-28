package per_card

// Batch Y (R60) — 5 high-impact unstubbed commander handlers.
//
//   - Krenko, Tin Street Kingpin ({1}{R} 1/2 — attack-trigger
//     +X +1/+1 counters + X Goblin tokens, X = opponents)
//   - Talrand, Sky Summoner ({2}{U}{U} 2/2 — instant/sorcery cast
//     creates a 2/2 blue flying Drake)
//   - Sram, Senior Edificer ({1}{W} 1/2 — Aura/Equipment/Vehicle
//     cast cantrip)
//   - Pia Nalaar, Consul of Revival ({1}{R} 2/2 — nontoken artifact
//     creature dies → tapped 1/1 flying Thopter)
//   - Etali, Primal Storm ({4}{R}{R} 6/6 — attack-trigger exile top
//     of each player's library + free-cast grant on nonland exiles)
//
// The 5 commanders the prompt named (Krenko Mob Boss, Atraxa
// Praetors' / Grand Unifier, Edgar Markov, Tymna+Thrasios, Kinnan)
// are all already stubbed under their own files; this batch picks
// the next tier of unstubbed commanders covering aggro / spellslinger
// / Voltron / aristocrat / chaos archetypes.
//
// Pattern mirrors zz_batch_u_r60_register.go / zz_batch_w_r60_register.go:
// init() registers against the global registry and adds a Reset hook
// so handlers survive per_card.Reset() in tests.

func init() {
	RegisterBatchYR60(Global())
	AddResetHook(RegisterBatchYR60)
}

// RegisterBatchYR60 registers the batch-Y R60 handlers.
func RegisterBatchYR60(r *Registry) {
	if r == nil {
		return
	}
	registerKrenkoTinStreetKingpin(r)
	registerTalrand(r)
	registerSramSeniorEdificer(r)
	registerPiaNalaarConsulOfRevival(r)
	registerEtaliPrimalStorm(r)
}
