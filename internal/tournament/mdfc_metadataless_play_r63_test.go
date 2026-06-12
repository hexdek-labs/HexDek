package tournament

// Regression for the METADATA-LESS MDFC back-face land leak
// (mdfc-backface r63) — the correctness-score baseline's single
// STATE-INTEGRITY offender (28 of 300 chaos games).
//
// mdfc_play_test.go covers the shape where the deckparser populated
// BackFace* metadata (SwapToBackFace does the full identity swap).
// The chaos harnesses (cmd/hexdek-loki / cmd/hexdek-correctness)
// historically built Cards WITHOUT those fields, so IsMDFC() was false,
// the swap no-op'd, and StripAdventureHalfTypes refused (front half has
// no permanent type) — the played land entered the battlefield carrying
// the front face's "sorcery"/"instant" type and tripped Feynman's
// permanent_types invariant. The engine-side fix
// (gameengine.StripToBackHalfTypes, composed into
// EnsureBattlefieldFrontFace) keeps the back-half types whenever only
// the back half has a permanent type, metadata or not.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// metadataLessMDFC mirrors how cmd/hexdek-loki's buildCardFromName
// historically built "Agadeem's Awakening // Agadeem, the Undercrypt":
// combined parser-leaked Types, NO BackFace* fields.
func metadataLessMDFC() *gameengine.Card {
	return &gameengine.Card{
		Name:     "Agadeem's Awakening // Agadeem, the Undercrypt",
		Types:    []string{"sorcery", "//", "land"},
		TypeLine: "sorcery // land",
		CMC:      3,
		Colors:   []string{"B"},
	}
}

func TestMDFC_MetadataLess_BackFaceLandPlayHasNoSpellType(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	seat := gs.Seats[0]
	seat.Hat = &hat.GreedyHat{}
	card := metadataLessMDFC()
	seat.Hand = append(seat.Hand, card)

	tryPlayLand(gs, 0)

	if len(seat.Battlefield) != 1 {
		t.Fatalf("expected 1 permanent on battlefield, got %d (hand size: %d)",
			len(seat.Battlefield), len(seat.Hand))
	}
	perm := seat.Battlefield[0]

	hasLand := false
	for _, typ := range perm.Card.Types {
		switch typ {
		case "instant", "sorcery":
			t.Errorf("battlefield permanent carries spell type %q (types: %v) — instants and sorceries cannot be permanents (CR §304.4/§307.1)",
				typ, perm.Card.Types)
		case "land":
			hasLand = true
		case "//":
			t.Errorf("parser pseudo-token %q leaked onto battlefield identity (types: %v)", typ, perm.Card.Types)
		}
	}
	if !hasLand {
		t.Errorf("back-face land permanent lost its land type (types: %v)", perm.Card.Types)
	}
}
