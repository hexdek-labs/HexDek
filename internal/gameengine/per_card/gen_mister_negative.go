package per_card

// registerMisterNegative is the auto-generated entry point retained so
// the batch_generated.go registry call keeps compiling. The real ETB
// (Darkforce Inversion: exchange life totals with target opponent;
// draw cards equal to life lost) lives in
// custom_mister_negative.go and is wired via
// registerMisterNegativeCustom (zz_handler_q45_register.go).
//
// The auto-gen body emitted a misleading "static abilities handled by
// AST engine" partial breadcrumb on every Mister Negative ETB — but
// Darkforce Inversion is a one-shot ETB effect, not a static, and the
// custom handler already implements it. Neutered here (R49 batch C)
// so the partial channel doesn't surface a fake static-AST gap.
func registerMisterNegative(r *Registry) {
	_ = r
}
