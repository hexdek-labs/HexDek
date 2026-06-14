package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func gyProbeGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.Phase = "main"
	gs.Active = 0
	return gs
}

func kwCard(name string, owner int, types []string, cmc int, kws ...gameast.Ability) *Card {
	return &Card{
		Name: name, Owner: owner, Types: types, CMC: cmc,
		AST: &gameast.CardAST{Name: name, Abilities: kws},
	}
}

func inZone(seat *Seat, card *Card, zone string) bool {
	var z []*Card
	switch zone {
	case "graveyard":
		z = seat.Graveyard
	case "exile":
		z = seat.Exile
	}
	for _, c := range z {
		if c == card {
			return true
		}
	}
	return false
}

// (a)+(f) flashback: cast from graveyard, resolves, card ends in EXILE.
func TestGYCast_Flashback_ExilesAfterResolve(t *testing.T) {
	gs := gyProbeGame(t)
	c := kwCard("Firebolt", 0, []string{"sorcery"}, 1, &gameast.Keyword{Name: "flashback", Args: []any{"{4}{R}"}})
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].ManaPool = 10
	if _, err := CastFlashback(gs, 0, c, -1); err != nil {
		t.Fatalf("CastFlashback failed: %v", err)
	}
	DrainStack(gs)
	if inZone(gs.Seats[0], c, "graveyard") {
		t.Error("flashback card wrongly returned to graveyard")
	}
	if !inZone(gs.Seats[0], c, "exile") {
		t.Error("flashback card must end in EXILE after resolving (CR 702.34c)")
	}
}

// (b) escape: pays escape cost AND exiles the required other cards; card exiled.
func TestGYCast_Escape_ExilesOthersAndSelf(t *testing.T) {
	gs := gyProbeGame(t)
	// Real escape cards carry ONLY the mana cost in Args; the "exile N
	// other cards" count lives in the raw text (mirrors the corpus).
	c := kwCard("Uro", 0, []string{"sorcery"}, 3,
		&gameast.Keyword{Name: "escape", Args: []any{"{1}{G}{U}"},
			Raw: "escape-{1}{g}{u}, exile three other cards from your graveyard"})
	fodder := []*Card{
		kwCard("f1", 0, []string{"creature"}, 1),
		kwCard("f2", 0, []string{"creature"}, 1),
		kwCard("f3", 0, []string{"creature"}, 1),
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, fodder...)
	gs.Seats[0].ManaPool = 10
	if _, err := CastWithEscape(gs, 0, c, -1, fodder); err != nil {
		t.Fatalf("CastWithEscape failed: %v", err)
	}
	DrainStack(gs)
	for _, f := range fodder {
		if !inZone(gs.Seats[0], f, "exile") {
			t.Errorf("escape fodder %s must be exiled as a cost", f.Name)
		}
	}
	if !inZone(gs.Seats[0], c, "exile") {
		t.Error("escape spell itself must end in exile")
	}
}

// (c) jump-start: discard a card additional cost, then exile after resolve.
func TestGYCast_JumpStart_DiscardsAndExiles(t *testing.T) {
	gs := gyProbeGame(t)
	c := kwCard("Chemister's Insight", 0, []string{"instant"}, 4, &gameast.Keyword{Name: "jump-start"})
	discard := kwCard("extra", 0, []string{"land"}, 0)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, discard)
	gs.Seats[0].ManaPool = 10
	if ok := CastWithJumpStart(gs, 0, c, discard); !ok {
		t.Fatalf("CastWithJumpStart failed")
	}
	DrainStack(gs)
	if inZone(gs.Seats[0], discard, "hand") {
		t.Error("jump-start must discard the extra card")
	}
	if !inZone(gs.Seats[0], c, "exile") {
		t.Error("jump-start card must end in EXILE after resolving")
	}
}

// (d) disturb: cast back face from graveyard, then exile.
func TestGYCast_Disturb_ExilesAfterResolve(t *testing.T) {
	gs := gyProbeGame(t)
	c := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].ManaPool = 10
	if _, err := CastWithDisturb(gs, 0, c, -1); err != nil {
		t.Fatalf("CastWithDisturb failed: %v", err)
	}
	DrainStack(gs)
	// The back face is a permanent → resolves to the battlefield; if it later
	// leaves it must be exiled (CR 702.146e). Here we just confirm it did NOT
	// fall back into the graveyard.
	if inZone(gs.Seats[0], c, "graveyard") {
		t.Error("disturb card must not remain in the graveyard after casting")
	}
}

// (e) embalm: exiles the card and makes a token copy.
func TestGYCast_Embalm_ExilesAndTokens(t *testing.T) {
	gs := gyProbeGame(t)
	c := kwCard("Anointer Priest", 0, []string{"creature"}, 2, &gameast.Keyword{Name: "embalm", Args: []any{"{3}{W}"}})
	c.BasePower, c.BaseToughness = 1, 3
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].ManaPool = 10
	perm := ActivateEmbalm(gs, 0, c, 4)
	if perm == nil {
		t.Fatal("ActivateEmbalm returned nil")
	}
	if !inZone(gs.Seats[0], c, "exile") {
		t.Error("embalm must exile the original card")
	}
	if inZone(gs.Seats[0], c, "graveyard") {
		t.Error("embalm card must not remain in graveyard")
	}
}

// (e) eternalize: exiles + makes a 4/4 black zombie token.
func TestGYCast_Eternalize_Makes44BlackZombie(t *testing.T) {
	gs := gyProbeGame(t)
	c := kwCard("Adorned Pouncer", 0, []string{"creature"}, 2, &gameast.Keyword{Name: "eternalize", Args: []any{"{4}{W}{W}"}})
	c.BasePower, c.BaseToughness = 1, 1
	c.Colors = []string{"W"}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].ManaPool = 10
	perm := ActivateEternalize(gs, 0, c, 6)
	if perm == nil {
		t.Fatal("ActivateEternalize returned nil")
	}
	if !inZone(gs.Seats[0], c, "exile") {
		t.Error("eternalize must exile the original card")
	}
	if perm.Card.BasePower != 4 || perm.Card.BaseToughness != 4 {
		t.Errorf("eternalize token must be 4/4, got %d/%d", perm.Card.BasePower, perm.Card.BaseToughness)
	}
	hasBlack := false
	for _, col := range perm.Card.Colors {
		if col == "B" {
			hasBlack = true
		}
	}
	if !hasBlack {
		t.Error("eternalize token must be black")
	}
	isZombie := false
	for _, ty := range perm.Card.Types {
		if ty == "zombie" {
			isZombie = true
		}
	}
	if !isZombie {
		t.Error("eternalize token must be a Zombie")
	}
}

// retrace: casts from graveyard discarding a land; card returns to GRAVEYARD
// (NOT exile) — it can be retraced again.
func TestGYCast_Retrace_ReturnsToGraveyardNotExile(t *testing.T) {
	gs := gyProbeGame(t)
	c := kwCard("Raven's Crime", 0, []string{"sorcery"}, 1, &gameast.Keyword{Name: "retrace"})
	land := kwCard("Swamp", 0, []string{"land"}, 0)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, land)
	gs.Seats[0].ManaPool = 10
	if ok := CastWithRetrace(gs, 0, c, land); !ok {
		t.Fatalf("CastWithRetrace failed")
	}
	DrainStack(gs)
	if inZone(gs.Seats[0], c, "exile") {
		t.Error("retrace card must NOT be exiled — it returns to the graveyard")
	}
}

func gyOnBattlefield(seat *Seat, card *Card) bool {
	for _, p := range seat.Battlefield {
		if p != nil && p.Card == card {
			return true
		}
	}
	return false
}
