package per_card

// registerTasigurTheGoldenFang is the auto-generated entry point
// retained so the batch_generated.go registry call keeps compiling.
// The real activated ability (mill 2 + opponent picks nonland from
// graveyard to return) lives in custom_tasigur_the_golden_fang.go,
// registered from registry.go::registerDefaults.
//
// The previous auto-gen body paid {4} + milled 2 then stopped, dropping
// the return-from-graveyard half (half-finished-features-r48 #7). It
// also collided with the custom on OnActivated("Tasigur, the Golden
// Fang", ...) — both registrations would fire, with the gen path's
// no-return version racing the custom path's full implementation.
// Neutered in R60 Versailles so the partial channel doesn't surface
// a duplicate and the custom handler is the sole owner.
func registerTasigurTheGoldenFang(r *Registry) {
	_ = r
}
