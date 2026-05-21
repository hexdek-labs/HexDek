package per_card

// registerGhenArcanumWeaver is the auto-generated entry point retained
// so the batch_generated.go registry call keeps compiling. The real
// activated ability ({R}{W}{B}, {T}, Sacrifice an enchantment: return
// target enchantment card from your graveyard to the battlefield) lives
// in custom_ghen_arcanum_weaver.go and is wired via
// registerGhenArcanumWeaverCustom (zz_activated_stubs_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Ghen activation even though the custom handler already
// implemented the reanimation. Neutered here (R49 batch C) so the
// partial channel reflects the real custom-side gap, not a duplicate.
func registerGhenArcanumWeaver(r *Registry) {
	_ = r
}
