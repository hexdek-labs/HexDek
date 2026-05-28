package per_card

// Batch V (R60) — 5 high-impact unstubbed protection handlers.
//
//   - Heroic Intervention ({1}{G}{G} instant — mass hexproof + indestructible)
//   - Mother of Runes ({W} 1/1 — tap, single creature protection from color)
//   - Selfless Spirit ({1}{W} 2/1 — sac, mass indestructible)
//   - Tamiyo's Safekeeping ({1}{G} instant — single perm hexproof + indestructible + 2 life)
//   - Akroma's Will ({3}{W} sorcery — mass anthem + protection from B/R)
//
// Pattern mirrors zz_batch_u_r60_register.go: init() registers
// against the global registry and adds a Reset hook so handlers
// survive per_card.Reset() in tests. All five rely on the existing
// Permanent.GrantedAbilities + Flags cleanup at the EOT cleanup
// pass (phases.go §514.2 wear-off) — no per_card delayed-trigger
// bookkeeping needed.

func init() {
	RegisterBatchVR60(Global())
	AddResetHook(RegisterBatchVR60)
}

// RegisterBatchVR60 registers the batch-V R60 handlers.
func RegisterBatchVR60(r *Registry) {
	if r == nil {
		return
	}
	registerHeroicIntervention(r)
	registerMotherOfRunes(r)
	registerSelflessSpirit(r)
	registerTamiyosSafekeeping(r)
	registerAkromasWill(r)
}
