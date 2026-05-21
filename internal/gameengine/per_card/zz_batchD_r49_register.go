package per_card

// dev/percard-stubs-batchD-r49 registration entry point.
//
// Stacks 6 additive custom handlers alongside their gen_*.go stubs:
//
//   - The Locust God                    — {2}{U}{R}: draw a card,
//                                          then discard a card.
//   - Firesong and Sunspeaker           — red I/S spells you control
//                                          have lifelink (modeled as
//                                          life gain on noncombat damage
//                                          events with the tracked red
//                                          I/S on the stack).
//   - Mendicant Core, Guidelight        — max-speed artifact spell copy
//                                          via PushStackItem.
//   - Sandman, Shifting Scoundrel       — characteristic-defining
//                                          power/toughness equal to the
//                                          controller's land count.
//   - Old One Eye                       — "Other creatures you control
//                                          have trample" anthem refresh.
//
// Cleopatra, Exiled Pharaoh's self-betrayal trigger and the four
// cost-modifier cases (Capitoline Triad, Lyse Hext, Magnus the Red,
// Morophon the Boundless) are landed in-place — Cleopatra's gen_*.go
// drops the self-exclude branch; the cost cases extend
// gameengine/cost_modifiers.go ScanCostModifiers. Those don't need a
// register hook.
//
// init() runs LAST in per_card package init order ("zz_" prefix sorts
// after every gen_* / custom_* / zz_* alphabetically before us). It
// also installs a Reset() hook so Reset()-using tests get the batchD
// handlers reinstated.

func init() {
	RegisterBatchDR49(Global())
	AddResetHook(RegisterBatchDR49)
}

func RegisterBatchDR49(r *Registry) {
	if r == nil {
		return
	}
	registerTheLocustGodCustom(r)
	registerFiresongAndSunspeakerCustom(r)
	registerMendicantCoreGuidelightCustom(r)
	registerSandmanShiftingScoundrelCustom(r)
	registerOldOneEyeCustom(r)
}
