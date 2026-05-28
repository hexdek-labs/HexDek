package per_card

// Batch AK (R60) — 3 high-impact previously-unstubbed cards stressing
// CR §108.4 spell-controller routing + the post-#685 owner-scoped
// MoveCard / ZoneCastGrant primitives that the §400.7c property test
// (cr_400_7c_property_r60_test.go) verifies.
//
// Original task scope was 5 cards (Sen Triplets / Praetor's Grasp /
// Insurrection / Word of Command / Mind's Dilation). Sen Triplets and
// Praetor's Grasp were already fully wired before this batch landed —
// Sen Triplets in gen_sen_triplets.go (ZoneCastPolicy-driven cast-from-
// opponent's-hand grant) and Praetor's Grasp in praetors_grasp.go
// (MoveCard ownerSeat-explicit exile + NewFreeCastFromExilePermission).
// The three new handlers shipped here:
//
//   - Insurrection ({5}{R}{R}{R} mass-Threaten with untap + haste)
//   - Word of Command ({B}{B} hand reveal + force-play)
//   - Mind's Dilation ({5}{U}{U} first-opponent-spell exile + free cast)
//
// Pattern mirrors zz_batch_p_r60_register.go: init() registers against
// the global registry and adds a Reset hook so handlers survive
// per_card.Reset() in tests.

func init() {
	RegisterBatchAKR60(Global())
	AddResetHook(RegisterBatchAKR60)
}

// RegisterBatchAKR60 registers the batch-AK R60 handlers.
func RegisterBatchAKR60(r *Registry) {
	if r == nil {
		return
	}
	registerInsurrection(r)
	registerWordOfCommand(r)
	registerMindsDilation(r)
}
