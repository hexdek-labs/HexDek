package per_card

// dev/percard-stubs-batchH-r50 registration entry point.
//
// Tribal-lord focused batch: each port either lights up an "Other X
// you control" anthem, a tribal payoff trigger, or a tribal cost-
// reduction the gen_*.go / hand-written sibling was already
// breadcrumb-flagging as unimplemented.
//
//   - Mabel, Heir to Cragflame      Other Mice +1/+1 Modification.
//   - Commander Mustard             Other Soldiers vig/trample/haste flags.
//   - Rienne, Angel of Rebirth      Other multicolored +1/+0 Modification.
//   - Kolodin, Triumph Caster       Mounts/Vehicles kw:haste anthem.
//   - The Swarmweaver               Delirium Insect/Spider +1/+1 + deathtouch.
//   - Gornog, the Red Reaper        Coward-tag on attack + warrior anthem +
//                                   "Cowards can't block Warriors" marker.
//   - Gisa, the Hellraiser          Skeleton/Zombie +1/+1 + menace anthem.
//   - Aphelia, Viper Whisperer      {4}{B} tribal-halving activation +
//                                   Gorgon/Snake combat-damage dispatcher.
//   - Tovolar, Dire Overlord        Night-flip Human Werewolf transform rider.
//
// Samut, the Driving Force's noncreature {X}-discount lands in-place in
// gameengine/cost_modifiers.go (reads seat.Flags["speed"] which the
// hand-written anthem refresh already keeps in sync).
//
// "zz_" prefix sorts after every gen_*/custom_*/zz_*/handlers file, so
// init() runs LAST in package init order. AddResetHook makes batch H
// survive Reset()-using tests.

func init() {
	RegisterBatchHR50(Global())
	AddResetHook(RegisterBatchHR50)
}

func RegisterBatchHR50(r *Registry) {
	if r == nil {
		return
	}
	registerMabelHeirToCragflameCustom(r)
	registerCommanderMustardCustom(r)
	registerRienneAngelOfRebirthCustom(r)
	registerKolodinTriumphCasterCustom(r)
	registerGornogTheRedReaperCustom(r)
	registerJodahTheUnifierCustom(r)
	registerGisaTheHellraiserCustom(r)
	registerApheliaViperWhispererCustom(r)
	registerTovolarDireOverlordCustom(r)
}
