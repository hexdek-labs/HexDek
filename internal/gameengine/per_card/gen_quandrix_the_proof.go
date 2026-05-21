package per_card

// registerQuandrixTheProof is the auto-generated entry point retained
// so the batch_generated.go registry call keeps compiling. The real
// implementation (ETB +1/+1 counter doubler + cast-from-command-zone
// counter distribution) lives in custom_quandrix_the_proof.go and is
// wired via registerQuandrixTheProofCustom (registered from
// registry.go).
//
// Note: the auto-gen oracle text in this stub describes a different
// printing (cascade-rider). The custom handler implements the
// Strixhaven Commander 2021 printing's counter-doubling effect. The
// auto-gen body emitted a misleading "static abilities handled by
// AST engine" partial breadcrumb on every Quandrix ETB regardless.
// Neutered here (R50 batch G) so the partial channel doesn't surface
// the fake AST gap.
func registerQuandrixTheProof(r *Registry) {
	_ = r
}
