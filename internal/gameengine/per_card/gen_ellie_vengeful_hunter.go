package per_card

// registerEllieVengefulHunter is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real activated ability (pay 2 life, sac another creature: Ellie
// deals 2 to target player + gains indestructible UEOT) lives in
// custom_ellie_vengeful_hunter.go and is wired via
// registerEllieVengefulHunterCustom (zz_handler_q45_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Ellie activation even though the custom handler already
// implements the pay/sac cost + damage + indestructible grant.
// Neutered here (R50 batch G) so the partial channel doesn't surface
// a duplicate parser-gap signal.
func registerEllieVengefulHunter(r *Registry) {
	_ = r
}
