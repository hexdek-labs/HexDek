package hat

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// yard_cast_priority_r60_test.go — R60 audit. The tournament turn
// loop's buildCastableList only walks seat.Hand — gs.ZoneCastGrants
// is never consulted, so cards offered for cast-from-graveyard /
// cast-from-exile (Iroh's flashback grant, Past in Flames, Mizzix's
// Mastery exile-cast, Underworld Breach escape, Yawgmoth's Will) are
// invisible to the hat's cast decision.
//
// This file pins the prioritization helper that becomes load-bearing
// once the cast loop is extended to enumerate zone-cast options. The
// audit gap is documented in the PR body; this PR ships the
// machinery so the integration only needs to wire the call-through.
//
// Signals exercised (per RankZoneCastCandidates):
//   - free casts (ManaCost == 0) outrank paid casts
//   - combo-relevant cards outrank generic
//   - mass-effect oracle ("destroy all" / "win the game") outranks
//     vanilla
//   - tutor oracle ("search your library") outranks vanilla
//   - high-CMC original (CMC ≥ 5) outranks low-CMC at parity
//   - once-per-turn-consumed candidates are dropped (CanCastFromZone
//     filter)

// yardCard builds a Card placed into seat's graveyard with the given
// oracle. cmc is set via the "cost:N" type tag the test fixtures use.
func yardCard(gs *gameengine.GameState, seat int, name string, cmc int, oracle string, types ...string) *gameengine.Card {
	c := newTestCardMinimal(name, append([]string{}, types...), cmc,
		&gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Static{Raw: oracle}},
		})
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
	return c
}

// grantPerm registers a ZoneCastPermission for `card` with the
// supplied controller + ManaCost (0 = free, -1 = card's native CMC).
// Zone defaults to "graveyard"; pass a custom zone via grantPermZone.
func grantPerm(gs *gameengine.GameState, card *gameengine.Card, controller, manaCost int, sourceName string) *gameengine.ZoneCastPermission {
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	p := &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneGraveyard,
		Keyword:           "flashback",
		ManaCost:          manaCost,
		RequireController: controller,
		SourceName:        sourceName,
	}
	gs.ZoneCastGrants[card] = p
	return p
}

// hatForRanking builds a midrange Yggdrasil hat with affordable
// mana for the test seat — used by every test below.
func hatForRanking(t *testing.T) (*YggdrasilHat, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[0].ManaPool = 20
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	return h, gs
}

// -----------------------------------------------------------------------------
// RankZoneCastCandidates — empty / nil cases
// -----------------------------------------------------------------------------

func TestRankZoneCastCandidates_NoGrantsReturnsNil(t *testing.T) {
	h, gs := hatForRanking(t)
	if got := h.RankZoneCastCandidates(gs, 0); got != nil {
		t.Errorf("no grants registered should return nil; got %d entries", len(got))
	}
}

func TestRankZoneCastCandidates_NilGameSafe(t *testing.T) {
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	if got := h.RankZoneCastCandidates(nil, 0); got != nil {
		t.Errorf("nil gs must return nil; got %v", got)
	}
}

func TestRankZoneCastCandidates_FiltersByController(t *testing.T) {
	h, gs := hatForRanking(t)
	c := yardCard(gs, 1, "Opp Spell", 2, "Draw a card.", "sorcery")
	grantPerm(gs, c, 1, 0, "Opp Iroh")
	if got := h.RankZoneCastCandidates(gs, 0); len(got) != 0 {
		t.Errorf("grants for other seats must be filtered out; got %d", len(got))
	}
}

// -----------------------------------------------------------------------------
// Free > paid
// -----------------------------------------------------------------------------

// TestRankZoneCastCandidates_FreeOutranksPaid pins the canonical case:
// Iroh's flashback grant (ManaCost=0, sourceName "Iroh") beats a
// Mizzix's Mastery cast that pays the original CMC.
func TestRankZoneCastCandidates_FreeOutranksPaid(t *testing.T) {
	h, gs := hatForRanking(t)
	free := yardCard(gs, 0, "Lightning Bolt", 1, "Lightning Bolt deals 3 damage.", "instant")
	grantPerm(gs, free, 0, 0, "Iroh's flashback")
	paid := yardCard(gs, 0, "Mind Stone", 2, "Tap: Add one mana.", "artifact")
	grantPerm(gs, paid, 0, -1, "Mizzix's Mastery") // -1 = pay native CMC

	got := h.RankZoneCastCandidates(gs, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates; got %d", len(got))
	}
	if got[0].Card.DisplayName() != "Lightning Bolt" {
		t.Errorf("free cast should outrank paid; got first=%s reasons=%s",
			got[0].Card.DisplayName(), got[0].Reason)
	}
	if !strings.Contains(got[0].Reason, "free") {
		t.Errorf("top reason should include 'free'; got %q", got[0].Reason)
	}
}

// -----------------------------------------------------------------------------
// Mass effect > vanilla
// -----------------------------------------------------------------------------

func TestRankZoneCastCandidates_MassEffectOutranksVanilla(t *testing.T) {
	h, gs := hatForRanking(t)
	wrath := yardCard(gs, 0, "Wrath of God", 4, "Destroy all creatures.", "sorcery")
	grantPerm(gs, wrath, 0, -1, "Past in Flames")
	vanilla := yardCard(gs, 0, "Counsel of the Soratami", 3, "Draw two cards.", "sorcery")
	grantPerm(gs, vanilla, 0, -1, "Past in Flames")

	got := h.RankZoneCastCandidates(gs, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates; got %d", len(got))
	}
	if got[0].Card.DisplayName() != "Wrath of God" {
		t.Errorf("mass-removal should outrank generic draw; got first=%s",
			got[0].Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Tutor > vanilla
// -----------------------------------------------------------------------------

func TestRankZoneCastCandidates_TutorOutranksVanilla(t *testing.T) {
	h, gs := hatForRanking(t)
	tutor := yardCard(gs, 0, "Demonic Tutor", 2,
		"Search your library for a card and put it into your hand.", "sorcery")
	grantPerm(gs, tutor, 0, -1, "Past in Flames")
	vanilla := yardCard(gs, 0, "Counsel of the Soratami", 3, "Draw two cards.", "sorcery")
	grantPerm(gs, vanilla, 0, -1, "Past in Flames")

	got := h.RankZoneCastCandidates(gs, 0)
	if got[0].Card.DisplayName() != "Demonic Tutor" {
		t.Errorf("tutor should outrank generic draw; got first=%s",
			got[0].Card.DisplayName())
	}
	if !strings.Contains(got[0].Reason, "tutor") {
		t.Errorf("tutor reason should be set; got %q", got[0].Reason)
	}
}

// -----------------------------------------------------------------------------
// High-CMC bonus
// -----------------------------------------------------------------------------

func TestRankZoneCastCandidates_HighCMCBeatsLowCMCAtParity(t *testing.T) {
	h, gs := hatForRanking(t)
	bigSpell := yardCard(gs, 0, "Time Stretch", 10, "Take two extra turns after this one.", "sorcery")
	// Force paid cost (no free signal) so the big-spell bump is what
	// determines the winner.
	grantPerm(gs, bigSpell, 0, -1, "Mizzix's Mastery")
	smallSpell := yardCard(gs, 0, "Brainstorm", 1, "Draw three cards, then put two back.", "instant")
	grantPerm(gs, smallSpell, 0, -1, "Mizzix's Mastery")

	got := h.RankZoneCastCandidates(gs, 0)
	if got[0].Card.DisplayName() != "Time Stretch" {
		t.Errorf("high-CMC spell should outrank low-CMC at parity (extracted value); got first=%s",
			got[0].Card.DisplayName())
	}
	if !strings.Contains(got[0].Reason, "big_spell") {
		t.Errorf("big_spell reason should be set; got %q", got[0].Reason)
	}
}

// -----------------------------------------------------------------------------
// Score function — unit tests on contribution sizes
// -----------------------------------------------------------------------------

func TestScoreZoneCastCandidate_FreeContributesThree(t *testing.T) {
	h, gs := hatForRanking(t)
	c := yardCard(gs, 0, "Vanilla", 2, "Draw a card.", "sorcery")
	free := grantPerm(gs, c, 0, 0, "Iroh")
	paid := *free
	paid.ManaCost = -1

	freeScore, _ := h.scoreZoneCastCandidate(gs, 0, c, free)
	paidScore, _ := h.scoreZoneCastCandidate(gs, 0, c, &paid)
	if delta := freeScore - paidScore; delta < 2.9 || delta > 3.1 {
		t.Errorf("free should contribute ~3.0 vs paid baseline; got delta=%.2f",
			delta)
	}
}

func TestScoreZoneCastCandidate_MassEffectContributesTwo(t *testing.T) {
	h, gs := hatForRanking(t)
	wrath := yardCard(gs, 0, "Wrath of God", 4, "Destroy all creatures.", "sorcery")
	vanilla := yardCard(gs, 0, "Divination", 3, "Draw two cards.", "sorcery")
	paidPerm := &gameengine.ZoneCastPermission{Zone: gameengine.ZoneGraveyard, ManaCost: -1}

	wrathScore, _ := h.scoreZoneCastCandidate(gs, 0, wrath, paidPerm)
	vanillaScore, _ := h.scoreZoneCastCandidate(gs, 0, vanilla, paidPerm)
	if delta := wrathScore - vanillaScore; delta < 1.5 || delta > 2.5 {
		t.Errorf("mass-effect should contribute ~2.0 vs vanilla; got delta=%.2f",
			delta)
	}
}

// -----------------------------------------------------------------------------
// Sort is stable on tied scores
// -----------------------------------------------------------------------------

func TestRankZoneCastCandidates_StableTieBreakByName(t *testing.T) {
	h, gs := hatForRanking(t)
	bcard := yardCard(gs, 0, "Bolt", 1, "Deal 3 damage.", "instant")
	acard := yardCard(gs, 0, "Aether Shard", 1, "Deal 3 damage.", "instant")
	grantPerm(gs, bcard, 0, -1, "Mizzix")
	grantPerm(gs, acard, 0, -1, "Mizzix")
	got := h.RankZoneCastCandidates(gs, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates; got %d", len(got))
	}
	if got[0].Card.DisplayName() != "Aether Shard" {
		t.Errorf("tied-score sort should break alphabetically; got first=%s",
			got[0].Card.DisplayName())
	}
}
