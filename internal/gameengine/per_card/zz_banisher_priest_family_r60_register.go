package per_card

// Phase 3 LTB-return family (docs/instanceid-system-v2-r60.md §4.2 + §7).
// Registers Banisher Priest / Oblivion Ring / Detention Sphere / Faceless
// Butcher against the global registry, plus an AddResetHook so handlers
// survive per_card.Reset() in tests.

func init() {
	RegisterBanisherPriestFamilyR60(Global())
	AddResetHook(RegisterBanisherPriestFamilyR60)
}

// RegisterBanisherPriestFamilyR60 wires the 4 CR §406.7 LTB-return cards
// shipped under InstanceID Phase 3.
func RegisterBanisherPriestFamilyR60(r *Registry) {
	if r == nil {
		return
	}
	registerBanisherPriest(r)
	registerOblivionRing(r)
	registerDetentionSphere(r)
	registerFacelessButcher(r)
}
