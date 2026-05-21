package per_card

// registerUreniTheSongUnending is the auto-generated entry point retained
// so the batch_generated.go registry call keeps compiling. The real ETB
// (divided X damage where X = lands controlled) lives in
// custom_ureni_the_song_unending.go and is wired via
// zz_handler_q45_register.go's registerUreniTheSongUnendingCustom.
//
// The auto-gen body emitted a misleading partial breadcrumb on every
// Ureni ETB even though the custom handler already implemented the
// full divided-X damage. Neutered here (R49 batch C) so the partial
// channel doesn't get polluted with a fake gap.
func registerUreniTheSongUnending(r *Registry) {
	_ = r
}
