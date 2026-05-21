package per_card

// registerGalazethPrismari is the auto-generated entry point retained
// so the batch_generated.go registry call keeps compiling. The real
// ETB (create a Treasure token) and the artifact-tap mana grant live
// in custom_galazeth_prismari.go and are wired via
// registerGalazethPrismariCustom (registered from registry.go).
//
// REPLACES BROKEN AUTO-GEN ETB: the previous gen body minted a 1/1
// "Creature Token" on every Galazeth ETB — neither a Treasure (per
// the printed oracle) nor a behavior recoverable from the oracle
// text. The custom handler already creates the Treasure correctly.
// Neutered here (R49 batch C) so each Galazeth ETB no longer adds a
// phantom 1/1 to the controller's battlefield alongside the proper
// Treasure.
func registerGalazethPrismari(r *Registry) {
	_ = r
}
