package per_card

// R60 — once_per_turn_cast_from_graveyard primitive ships with Kess
// Dissident Mage (refactored to the new primitive in
// custom_kess_dissident_mage.go) and Maestros Ascendancy (new handler
// in maestros_ascendancy.go). Kess already had a generated stub call
// in batch_generated.go so it re-registers there; Maestros is new and
// needs its own hookup. Pattern mirrors zz_era2_register.go so the
// handler survives Reset() in tests.

func init() {
	RegisterR60(Global())
	AddResetHook(RegisterR60)
}

// RegisterR60 registers the R60 handlers (Maestros Ascendancy). Kess
// is registered via batch_generated.go → registerKessDissidentMage →
// registerKessDissidentMageCustom and does not need to be re-listed.
func RegisterR60(r *Registry) {
	if r == nil {
		return
	}
	registerMaestrosAscendancy(r)
}
