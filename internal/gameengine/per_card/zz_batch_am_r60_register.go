package per_card

// Batch AM (R60) — 5 high-impact unstubbed cards in the graveyard-cast /
// escape / reanimate family. These cards stress CR §108.4 (cast = the act
// of putting a card on the stack and paying its costs, regardless of
// source zone) and §400.7 (zone-change identity) because they cast or
// move cards across owner-vs-controller boundaries.
//
//   - Kroxa, Titan of Death's Hunger ({B}{R} 6/6 Elder Giant — ETB
//     sac-unless-escaped + each-opp-discard, upkeep sac-unless-opp-lost-life)
//   - Uro, Titan of Nature's Wrath ({G}{U} 6/6 Elder Giant — ETB
//     sac-unless-escaped + gain 3 / draw / extra land, attack re-fires
//     value clause)
//   - Lurrus of the Dream-Den ({W}{B}{B} 3/2 Cat Nightmare — once-per-turn
//     cast ≤2-CMC permanent from graveyard via the engine's
//     NewOncePerTurnGraveyardCastPermission primitive)
//   - Mizzix's Mastery ({4}{R} Sorcery — register free-cast graveyard
//     grants on all i/s cards on resolve; Hat picks)
//   - Reanimate ({B} Sorcery — §400.7 / §110.2a fix to the existing
//     reanimateResolve in commander_staples.go: the pre-r60 handler
//     moved the creature to its OWNER's battlefield but did not
//     reparent to the caster's seat per "under your control",
//     leaving the reanimated body on the opp's side. The fix runs
//     the same post-MoveCard reparenting pattern Nicol Bolas's
//     −4 ultimate uses.)
//
// Past in Flames and Snapcaster Mage (the prompt's other two examples)
// are already stubbed (oneshot_flashback_grants.go and
// tla_flashback_single_target_r60.go respectively).
//
// Pattern mirrors zz_batch_ah_r60_register.go: init() registers against
// the global registry and adds a Reset hook so handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchAMR60(Global())
	AddResetHook(RegisterBatchAMR60)
}

// RegisterBatchAMR60 registers the batch-AM R60 handlers.
func RegisterBatchAMR60(r *Registry) {
	if r == nil {
		return
	}
	registerKroxa(r)
	registerUro(r)
	registerLurrus(r)
	registerMizzixsMastery(r)
	// Reanimate is registered by commander_staples.go; batch AM only
	// fixes the §400.7 / §110.2a reparenting bug in the existing
	// reanimateResolve, no separate registration needed here.
}
