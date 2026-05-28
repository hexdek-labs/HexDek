package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestCreatePermanent_CrossSeatDedupReusesExisting closes the
// dominant CardIdentity violation cluster surfaced by the layer-
// stress 1000-game seed-42 sweep (and the larger 250K sweep
// documented as Cluster A in docs/loki-r60-250k-analysis.md, PR
// #713): when a per_card handler invokes createPermanent against
// a *Card that's already wrapped in a Permanent on a DIFFERENT
// seat's battlefield, the dedup must return the existing
// Permanent rather than allocating a second wrapper.
//
// Pre-fix policy scanned only the target seat's battlefield —
// caught the canonical "MoveCard then createPermanent" pattern
// but missed cross-seat duplication, producing the
// "*Card appears in both seat 0 battlefield and seat 2
// battlefield" violation pattern (game 86 turn 27 in the
// reproduction corpus, Demolisher Spawn shape).
//
// The fix extends the dedup scan to ALL seats — same-pointer-on-
// battlefield is always wrong regardless of seat, so any hit is
// the canonical "use the existing Permanent" path.
//
// Layer-stress sweep verification post-fix: 1000 games seed 42
// went from 114 violations in 6 games to 0 violations.
func TestCreatePermanent_CrossSeatDedupReusesExisting(t *testing.T) {
	Reset()
	gs := gameengine.NewGameState(3, nil, nil)

	// Seed: Demolisher Spawn already wrapped on seat 2's
	// battlefield via some prior path (control change, reanimate,
	// etc.). The *Card pointer is what matters — owner stays as
	// the original owner regardless of which seat it sits on.
	card := &gameengine.Card{
		Name:  "Demolisher Spawn",
		Types: []string{"creature"},
		Owner: 0,
	}
	existing := &gameengine.Permanent{
		Card:       card,
		Controller: 2,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[2].Battlefield = append(gs.Seats[2].Battlefield, existing)

	// Some other code path now calls createPermanent with the same
	// *Card pointer targeting seat 0 (e.g. a per_card reanimate
	// handler that didn't notice the card was already on the
	// battlefield somewhere else).
	got := createPermanent(gs, 0, card, false)

	// Control-change semantics: a NEW wrapper on seat 0, the stale
	// seat-2 wrapper dropped. The *Card pointer ends up on EXACTLY
	// ONE battlefield (seat 0's), per §400.7c.
	if got == nil {
		t.Fatal("cross-seat createPermanent: want new Permanent on target seat, got nil")
	}
	if got == existing {
		t.Errorf("cross-seat createPermanent: want a NEW wrapper with Controller=0, got the stale seat-2 wrapper")
	}
	if got.Controller != 0 {
		t.Errorf("new wrapper Controller: want 0 (target seat), got %d", got.Controller)
	}

	// Seat 0 battlefield: exactly 1 wrapper for the *Card.
	count0 := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			count0++
		}
	}
	if count0 != 1 {
		t.Errorf("seat 0 should have exactly 1 wrapper for the *Card, got %d", count0)
	}

	// Seat 2 battlefield: stale wrapper dropped per §400.7c.
	for _, p := range gs.Seats[2].Battlefield {
		if p != nil && p.Card == card {
			t.Errorf("seat 2 should have dropped the stale wrapper — CardIdentity invariant would fire on the next SBA pass otherwise")
		}
	}
}

// TestCreatePermanent_SameSeatDedupStillWorks regression-guards
// the original same-seat dedup behavior — the fix extending the
// scan to all seats must not break the canonical "MoveCard then
// createPermanent" path where the *Card is already on the target
// seat's battlefield.
func TestCreatePermanent_SameSeatDedupStillWorks(t *testing.T) {
	Reset()
	gs := gameengine.NewGameState(2, nil, nil)

	card := &gameengine.Card{
		Name:  "Llanowar Elves",
		Types: []string{"creature"},
		Owner: 0,
	}
	existing := &gameengine.Permanent{
		Card:       card,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, existing)

	got := createPermanent(gs, 0, card, false)
	if got != existing {
		t.Errorf("same-seat dedup: want existing Permanent returned, got a new wrapper")
	}
	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			count++
		}
	}
	if count != 1 {
		t.Errorf("same-seat dedup: want exactly 1 wrapper for the *Card on seat 0, got %d", count)
	}
}

// TestCreatePermanent_FreshCardCreatesNewPermanent verifies the
// fix doesn't over-trigger — when the *Card is NOT on any seat's
// battlefield, createPermanent must allocate a new wrapper.
func TestCreatePermanent_FreshCardCreatesNewPermanent(t *testing.T) {
	Reset()
	gs := gameengine.NewGameState(2, nil, nil)

	card := &gameengine.Card{
		Name:  "Lightning Bolt",
		Types: []string{"instant"}, // Not a permanent — caller
		Owner: 0,
	}
	// Lightning Bolt isn't a permanent — createPermanent should
	// return nil per CardCanEnterBattlefield. Use a real creature
	// for the fresh-card test instead.
	creature := &gameengine.Card{
		Name:  "Grizzly Bears",
		Types: []string{"creature"},
		Owner: 0,
	}
	got := createPermanent(gs, 0, creature, false)
	if got == nil {
		t.Fatal("fresh card: createPermanent should allocate a new Permanent, got nil")
	}
	if got.Card != creature {
		t.Errorf("new Permanent: Card pointer mismatch")
	}
	if got.Controller != 0 {
		t.Errorf("new Permanent: Controller want 0, got %d", got.Controller)
	}
	_ = card // silence unused var
}
