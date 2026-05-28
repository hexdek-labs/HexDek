package per_card

// Batch AH (R60) — 5 high-impact unstubbed cards with combat / damage /
// discard / self-mill trigger payloads.
//
//   - Polukranos, World Eater ({2}{G}{G} 5/5 Hydra — {X}{X}{G}: fight
//     up to X opposing creatures, one per opponent)
//   - Hungering Hydra ({X}{G}{G} 0/0 Hydra — ETB with X +1/+1 counters,
//     grow on every damage event taken)
//   - Grim Hireling ({3}{B}{R} 4/4 Mercenary — combat-damage-to-player
//     trigger creates Treasure tokens)
//   - Glint-Horn Buccaneer ({2}{R} 2/2 Pirate — discard-a-card trigger
//     pings each opponent for 1 and may draw)
//   - Old Stickfingers ({X}{B}{G} 4/4 Horror — ETB-equivalent self-mill
//     until X creatures hit graveyard, reanimate all of them)
//
// All five were unstubbed prior to this batch; verified via filesystem
// scan against the per_card directory + grep for OnTrigger / OnETB /
// OnActivated bindings on each name. Each handler is gated on the
// controller-seat predicate appropriate to its oracle text ("Whenever
// YOU discard", "Whenever creatures YOU control deal damage", etc.).
//
// Pattern mirrors zz_batch_y_r60_register.go: init() registers against
// the global registry and adds a Reset hook so handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchAHR60(Global())
	AddResetHook(RegisterBatchAHR60)
}

// RegisterBatchAHR60 registers the batch-AH R60 handlers.
func RegisterBatchAHR60(r *Registry) {
	if r == nil {
		return
	}
	registerPolukranosWorldEater(r)
	registerHungeringHydra(r)
	registerGrimHireling(r)
	registerGlintHornBuccaneer(r)
	registerOldStickfingers(r)
}
