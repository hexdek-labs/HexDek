package gameengine

// exile_tag_battlefield_entry_r63_test.go — STATE_INTEGRITY frontier
// (r63), seed-11991338 game 1199 Heliod orphan.
//
// Knowledge Pool's "exile instead of cast" PARTIAL exiled the cast
// Heliod (stamping ExiledByTimestamp = KP's timestamp), but the spell
// then resolved onto the battlefield anyway. The §400.7c gap-walk
// purged the duplicate exile-slice reference, yet the stale exile tag
// rode along on the battlefield-resident permanent. Knowledge Pool's
// own LTB tag-sweep scans only exile zones, so it missed the
// battlefield Heliod; when Heliod later died into exile carrying the
// long-dead timestamp, the orphaned-linked-exile backstop fired every
// tick. A card on the battlefield is not in exile, so its exile
// linkage must clear at the battlefield-append chokepoint.

import (
	"math/rand"
	"testing"
)

func TestEnforceBattlefieldUnique_ClearsStaleExileTag(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	heliod := &Card{Name: "Heliod, Sun-Crowned", Owner: 0, Types: []string{"creature", "enchantment", "legendary"}}
	MintOGInstanceID(gs, heliod)
	// A now-gone source (e.g. Knowledge Pool, ts 47) stamped the card.
	heliod.ExiledByTimestamp = 47

	// The card lands on the battlefield (the chokepoint every append
	// path calls).
	EnforceBattlefieldUniqueInstanceID(gs, heliod, 0)

	if heliod.ExiledByTimestamp != 0 {
		t.Fatalf("battlefield entry must clear the stale exile linkage, got ExiledByTimestamp=%d", heliod.ExiledByTimestamp)
	}

	// Later the (now untagged) permanent dies into exile — no orphan.
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, heliod)
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Errorf("a permanent that entered the battlefield must not orphan when it later dies into exile: %v", err)
	}
}

// Pre-fix proof: without the clear, the card carries the tag onto the
// battlefield and orphans on the subsequent death-into-exile. (This
// test documents the exact pre-fix failure shape; it passes post-fix.)
func TestEnforceBattlefieldUnique_EmptyIDCardAlsoCleared(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// A legacy / unminted card still gets its stale exile tag cleared
	// at battlefield entry (the clear runs before the empty-ID no-op).
	card := &Card{Name: "Legacy Permanent", Owner: 0, Types: []string{"creature"}}
	card.ExiledByTimestamp = 99
	EnforceBattlefieldUniqueInstanceID(gs, card, 0)
	if card.ExiledByTimestamp != 0 {
		t.Fatalf("battlefield entry must clear the stale exile tag even for empty-ID cards, got %d", card.ExiledByTimestamp)
	}
}
