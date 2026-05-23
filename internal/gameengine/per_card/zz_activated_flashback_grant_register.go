package per_card

// dev/activated-flashback-grant-r60 registration entry point.
//
// Hand-rolled activated-ability flashback-grant handlers (Dralnu Lich
// Lord, Sapphire Collector, Magus of the Will) wire themselves into the
// global registry via this init(). File name sorts after every gen_*
// and existing custom file so it runs LAST in the package's init order.
//
// RegisterActivatedFlashbackGrant is also added as a Reset() hook so the
// handlers survive Reset()-driven test isolation (registry.go Reset()
// rebuilds only the registerDefaults() set; init() does not re-fire,
// hence AddResetHook).

func init() {
	RegisterActivatedFlashbackGrant(Global())
	AddResetHook(RegisterActivatedFlashbackGrant)
}

// RegisterActivatedFlashbackGrant registers the R60 batch on a Registry.
func RegisterActivatedFlashbackGrant(r *Registry) {
	if r == nil {
		return
	}
	registerDralnuLichLord(r)
	registerSapphireCollector(r)
	registerMagusOfTheWill(r)
}
