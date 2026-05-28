package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestAttachmentConsistency_ShufflePronounIntoLibraryDetachesAuras
// reproduces the layer-stress 1000-game sweep (PR #735) finding:
// game 661 turn 55 surfaced "Prison Term (seat 3) attached to
// Sparkhunter Masticore which is not on any battlefield."
//
// Root cause: the shuffle_pronoun_into_owner_library resolution
// handler at resolve_helpers.go (Dread / Sparkhunter Masticore /
// Vigor / Worldspine Wurm / Guile shape) removed the carrier
// permanent via gs.removePermanent(src) without calling detachAll.
// Auras / Equipment attached to the carrier retained their
// AttachedTo pointer at a permanent no longer on any battlefield;
// the next SBA pass triggered checkAttachmentConsistency.
//
// The Dread family is the canonical "leaves battlefield via
// shuffle-into-library" cycle. Like the 2026-05-08 CardIdentity
// fix at this same handler, the removal site was missing one of
// the canonical leave-play cleanup calls — specifically detachAll.
//
// The fix (PR this test ships in): add detachAll(gs, src)
// immediately after UnregisterContinuousEffectsForPermanent at
// the shuffle_pronoun_into_owner_library / shuffle_self_into_library
// removal site, matching the cleanup pattern of every other
// leave-play path (destroyPermSBA, sacrificePermSBA, etc.).
func TestAttachmentConsistency_ShufflePronounIntoLibraryDetachesAuras(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Carrier: a Dread / Sparkhunter-shape creature. The test only
	// needs the permanent + a Card with an Owner — the actual
	// engine resolution path doesn't depend on the source card's
	// AST having a shuffle trigger; we drive resolveResidualByText
	// directly with "shuffle_pronoun_into_owner_library" / src.
	carrier := &Permanent{
		Card:       &Card{Name: "Sparkhunter Masticore", Types: []string{"creature"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, carrier)

	// Aura attached to the carrier (Prison Term in the original
	// loki report).
	prisonTerm := &Permanent{
		Card:       &Card{Name: "Prison Term", Types: []string{"enchantment", "aura"}, Owner: 1},
		Controller: 1,
		Owner:      1,
		AttachedTo: carrier,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, prisonTerm)

	// Sanity check: attachment is wired up.
	if prisonTerm.AttachedTo != carrier {
		t.Fatal("test fixture: Prison Term should start attached to carrier")
	}

	// Drive the shuffle_pronoun_into_owner_library code path. This
	// is the exact resolution branch hit by Dread-shape triggers.
	resolveModificationEffect(gs, carrier, &gameast.ModificationEffect{
		ModKind: "shuffle_pronoun_into_owner_library",
	})

	// Carrier should no longer be on seat 0's battlefield.
	carrierOnBattlefield := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == carrier {
			carrierOnBattlefield = true
			break
		}
	}
	if carrierOnBattlefield {
		t.Errorf("carrier should have been removed from battlefield by shuffle-into-library handler")
	}

	// The canonical regression: Prison Term's AttachedTo MUST
	// have been nil'd by detachAll. Pre-fix this would still
	// point at carrier (a permanent no longer on any battlefield),
	// triggering checkAttachmentConsistency on the next SBA pass.
	if prisonTerm.AttachedTo != nil {
		t.Errorf("Prison Term: AttachedTo should be nil after carrier shuffled into library, got pointer to %q (carrier still off-battlefield — checkAttachmentConsistency would fire next SBA)",
			prisonTerm.AttachedTo.Card.Name)
	}
}

// TestAttachmentConsistency_ShuffleSelfIntoLibraryDetachesAuras
// covers the shuffle_self_into_library alias — the same shape as
// shuffle_pronoun_into_owner_library but hit by a different
// resolution path (e.g. some Dread variants use the alternate
// alias). Same detachment contract.
func TestAttachmentConsistency_ShuffleSelfIntoLibraryDetachesAuras(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	carrier := &Permanent{
		Card:       &Card{Name: "Worldspine Wurm", Types: []string{"creature"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, carrier)

	equip := &Permanent{
		Card:       &Card{Name: "Lightning Greaves", Types: []string{"artifact", "equipment"}, Owner: 1},
		Controller: 1,
		Owner:      1,
		AttachedTo: carrier,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, equip)

	resolveModificationEffect(gs, carrier, &gameast.ModificationEffect{
		ModKind: "shuffle_self_into_library",
	})

	if equip.AttachedTo != nil {
		t.Errorf("Lightning Greaves: AttachedTo should be nil after Worldspine Wurm shuffled into library via shuffle_self_into_library, got pointer (off-battlefield carrier — AttachmentConsistency leak)")
	}
}
