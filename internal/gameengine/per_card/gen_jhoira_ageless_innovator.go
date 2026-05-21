package per_card

// registerJhoiraAgelessInnovator is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real activated ability ({T}: add two ingenuity counters, then
// optionally cheat an artifact with MV X from hand where X = counter
// count) lives in custom_jhoira_ageless_innovator.go and is wired via
// registerJhoiraAgelessInnovatorCustom
// (zz_activated_stubs_register.go).
//
// The auto-gen activated body emitted a misleading partial breadcrumb
// on every Jhoira activation even though the custom handler already
// implemented the counter+cheat logic. Neutered here (R49 batch C)
// so the partial channel doesn't surface a duplicate parser-gap.
func registerJhoiraAgelessInnovator(r *Registry) {
	_ = r
}
