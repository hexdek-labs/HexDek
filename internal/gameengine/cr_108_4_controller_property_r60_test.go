package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// stackPushControllerForCard scans gs.EventLog for the stack_push event
// emitted when `cardName` was pushed onto the stack, and returns the
// Seat (which PushStackItem stamps as item.Controller). Returns -1 if
// the event isn't found. This is the test inspection point — after
// CastSpell + DrainStack, the StackItem is gone from gs.Stack, but the
// push-time controller is preserved in the event log.
func stackPushControllerForCard(gs *GameState, cardName string) int {
	for _, ev := range gs.EventLog {
		if ev.Kind != "stack_push" {
			continue
		}
		if ev.Source != cardName {
			continue
		}
		return ev.Seat
	}
	return -1
}

// -----------------------------------------------------------------------------
// CR §108.4 spell-controller property — r60
// -----------------------------------------------------------------------------
//
// CR §108.4: "A spell's controller is, by default, the player who put it on
// the stack." (CR §108.4a, CR §601.2a "the spell becomes the topmost object
// on the stack ... and its controller is the player who cast it.")
//
// The invariant under test: every cast pipeline — CastSpell (hand),
// CastFlashback (graveyard), CastWithEscape (graveyard + exile cost),
// CastFromZone (exile / library via ZoneCastPermission grant), and the
// alt-cost variants that route through the same StackItem builders —
// MUST set StackItem.Controller to the caster's seat index, NOT the
// card's owner seat. The two diverge whenever a non-owner casts an
// owner's card: Praetor's Grasp puts an opponent's card into your
// exile, you cast it through a free-cast grant — Controller is you,
// not the opponent.
//
// Failure mode this property catches: a future cast-pipeline addition
// (e.g., a new alt-cost keyword, a new ZoneCastPermission shape) that
// stamps `Controller: card.Owner` instead of `Controller: seatIdx` in
// the resulting StackItem. The bug is silent at cast time (the spell
// resolves under the wrong controller) and surfaces downstream as
// "owner's triggers fire instead of caster's" / "owner's mana pool
// pays for X" / "owner's seat takes the wincon-trigger payoff."
//
// Methodology: each subtest constructs a 2-player game, places a card
// owned by seat 0 into the relevant source zone, casts via the path
// under test using seat 1 (or seat 0 for paths that can't cross-cast),
// and inspects gs.Stack[0].Controller against the caster seat.
// -----------------------------------------------------------------------------

func newCR108Game(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(108))
	return NewGameState(2, rng, nil)
}

func newCR108Instant(name string, owner, cmc int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newCR108FlashbackCard(name string, owner, cmc int, flashbackArg string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flashback", Args: []any{flashbackArg}},
			},
		},
	}
}

func newCR108EscapeCard(name string, owner, cmc int, manaCost string, exileCount int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{
					Name: "escape",
					Args: []any{manaCost, exileCount},
				},
			},
		},
	}
}

// -----------------------------------------------------------------------------
// Baseline: CastSpell from hand — caster == owner. Controller MUST be caster.
// -----------------------------------------------------------------------------
func TestCR108_4_CastSpell_HandPath_ControllerIsCaster(t *testing.T) {
	gs := newCR108Game(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	card := newCR108Instant("Lightning Bolt", 0, 1)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	got := stackPushControllerForCard(gs, "Lightning Bolt")
	if got != 0 {
		t.Fatalf("CR §108.4 — stack_push Seat=%d, want 0 (caster) for hand cast", got)
	}
}

// -----------------------------------------------------------------------------
// CastFlashback (graveyard path) — same-seat (owner casts own card).
// Verifies the flashback builder sets Controller=caster, not owner.
// (Caster and owner happen to coincide here — flashback is always self-
// cast from your own graveyard — but the property still asserts the
// builder uses seatIdx and not card.Owner. A regression that swapped
// the two would still pass this test trivially; the cross-seat
// distinction is exercised by the CastFromZone subtests below.)
// -----------------------------------------------------------------------------
func TestCR108_4_CastFlashback_GraveyardPath_ControllerIsCaster(t *testing.T) {
	gs := newCR108Game(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6
	card := newCR108FlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastFlashback(gs, 0, card, 5); err != nil {
		t.Fatalf("CastFlashback: %v", err)
	}
	got := stackPushControllerForCard(gs, "Past in Flames")
	if got != 0 {
		t.Fatalf("CR §108.4 — stack_push Seat=%d, want 0 (caster) for flashback", got)
	}
}

// -----------------------------------------------------------------------------
// CastWithEscape (graveyard path with exile-cards cost). Same-seat.
// -----------------------------------------------------------------------------
func TestCR108_4_CastWithEscape_GraveyardPath_ControllerIsCaster(t *testing.T) {
	gs := newCR108Game(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	card := newCR108EscapeCard("Uro, Titan of Nature's Wrath", 0, 3, "{1}{G}{U}", 5)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	// Pad graveyard with 5 fodder cards for the exile cost.
	var fodder []*Card
	for i := 0; i < 5; i++ {
		c := &Card{Name: "Fodder", Owner: 0, Types: []string{"land"}}
		fodder = append(fodder, c)
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	}

	if _, err := CastWithEscape(gs, 0, card, 3, fodder); err != nil {
		t.Fatalf("CastWithEscape: %v", err)
	}
	got := stackPushControllerForCard(gs, "Uro, Titan of Nature's Wrath")
	if got != 0 {
		t.Fatalf("CR §108.4 — stack_push Seat=%d, want 0 (caster) for escape", got)
	}
}

// -----------------------------------------------------------------------------
// CastFromZone — Praetor's Grasp / Hostage Taker style. Card OWNED by
// seat 0 (printed Owner=0), sitting in some exile zone with a
// ZoneCastPermission that grants seat 1 the right to cast it. Controller
// MUST be seat 1, NOT seat 0.
//
// This is the canonical anti-pattern the §108.4 property catches — a
// cast pipeline that stamped Controller=card.Owner would put the spell
// on the stack under seat 0's control even though seat 1 paid for it
// and chose targets.
// -----------------------------------------------------------------------------
func TestCR108_4_CastFromZone_NonOwnerCaster_ControllerIsCaster(t *testing.T) {
	gs := newCR108Game(t)
	gs.Active = 1
	gs.Seats[1].ManaPool = 5
	// Card owned by seat 0 (printed Owner=0). CastFromZone scans the
	// caster's zone (zone_cast.go:286 — `seat := gs.Seats[seatIdx]`),
	// so we place the card in seat 1's exile to match the contract.
	// This mirrors the Hostage Taker / Gonti exile-to-controller's-side
	// lifecycle; the cross-seat distinction here is owner vs controller
	// on the resulting StackItem, NOT zone ownership.
	card := newCR108Instant("Owner's Bomb", 0, 3)
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, card)

	// Grant seat 1 free-cast permission. RequireController=1 gates the
	// permission to seat 1 only.
	perm := NewFreeCastFromExilePermission(1, "TestGrant")
	RegisterZoneCastGrant(gs, card, perm)

	if _, err := CastFromZone(gs, 1, card, ZoneExile, perm, nil); err != nil {
		t.Fatalf("CastFromZone: %v", err)
	}
	got := stackPushControllerForCard(gs, "Owner's Bomb")
	if got != 1 {
		t.Fatalf("CR §108.4 — stack_push Seat=%d, want 1 (caster), NOT 0 (owner). "+
			"This is the Hostage-Taker / Etali-exile anti-pattern — owner is "+
			"seat 0, but the spell controller MUST be the caster (seat 1).", got)
	}
	if card.Owner != 0 {
		t.Fatalf("card.Owner should remain 0 (unchanged by cross-cast), got %d", card.Owner)
	}
}

// -----------------------------------------------------------------------------
// CastFromZone — Bolas's Citadel style. Library cast, life-cost-instead-
// of-mana. Same-seat (owner casts top of own library).
// -----------------------------------------------------------------------------
func TestCR108_4_CastFromZone_BolassCitadelLibraryPath_ControllerIsCaster(t *testing.T) {
	gs := newCR108Game(t)
	gs.Active = 0
	gs.Seats[0].Life = 40
	card := newCR108Instant("Library Spell", 0, 3)
	gs.Seats[0].Library = append(gs.Seats[0].Library, card)

	perm := &ZoneCastPermission{
		Zone:                  ZoneLibrary,
		Keyword:               "bolas_citadel",
		ManaCost:              -1, // use card's printed cost
		LifeCostInsteadOfMana: 3,  // CMC 3
		RequireController:     0,
		SourceName:            "Bolas's Citadel",
	}
	RegisterZoneCastGrant(gs, card, perm)

	if _, err := CastFromZone(gs, 0, card, ZoneLibrary, perm, nil); err != nil {
		t.Fatalf("CastFromZone (library): %v", err)
	}
	got := stackPushControllerForCard(gs, "Library Spell")
	if got != 0 {
		t.Fatalf("CR §108.4 — stack_push Seat=%d, want 0 (caster) for library cast", got)
	}
}

// -----------------------------------------------------------------------------
// Bug-class regression: a synthetic StackItem with Controller stamped
// to card.Owner instead of caster is detectable by inspecting Card.Owner
// against StackItem.Controller. We assert the invariant directly so any
// future cast-pipeline addition that picks the wrong field surfaces.
// This complements the per-path tests above by stating the contract
// independent of which cast function is used.
// -----------------------------------------------------------------------------
func TestCR108_4_PropertyCrossCheck_ControllerNeverDefaultsToOwner(t *testing.T) {
	cases := []struct {
		name      string
		cardName  string
		setup     func(t *testing.T) (*GameState, int) // returns gs and caster seat
		cardOwner int
	}{
		{
			name:     "hand-cast self",
			cardName: "Lightning Bolt",
			setup: func(t *testing.T) (*GameState, int) {
				gs := newCR108Game(t)
				gs.Active = 0
				gs.Seats[0].ManaPool = 5
				card := newCR108Instant("Lightning Bolt", 0, 1)
				gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
				if err := CastSpell(gs, 0, card, nil); err != nil {
					t.Fatalf("setup CastSpell: %v", err)
				}
				return gs, 0
			},
			cardOwner: 0,
		},
		{
			name:     "flashback graveyard",
			cardName: "Past in Flames",
			setup: func(t *testing.T) (*GameState, int) {
				gs := newCR108Game(t)
				gs.Active = 0
				gs.Seats[0].ManaPool = 6
				card := newCR108FlashbackCard("Past in Flames", 0, 4, "{4}{R}")
				gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
				if _, err := CastFlashback(gs, 0, card, 5); err != nil {
					t.Fatalf("setup CastFlashback: %v", err)
				}
				return gs, 0
			},
			cardOwner: 0,
		},
		{
			name:     "hostage-taker cross-cast",
			cardName: "Owner's Bomb",
			setup: func(t *testing.T) (*GameState, int) {
				gs := newCR108Game(t)
				gs.Active = 1
				gs.Seats[1].ManaPool = 5
				card := newCR108Instant("Owner's Bomb", 0, 3)
				// Caster's exile (CastFromZone reads caster's zone).
				gs.Seats[1].Exile = append(gs.Seats[1].Exile, card)
				perm := NewFreeCastFromExilePermission(1, "TestGrant")
				RegisterZoneCastGrant(gs, card, perm)
				if _, err := CastFromZone(gs, 1, card, ZoneExile, perm, nil); err != nil {
					t.Fatalf("setup CastFromZone: %v", err)
				}
				return gs, 1
			},
			cardOwner: 0,
		},
		{
			name:     "bolas citadel library",
			cardName: "Library Spell",
			setup: func(t *testing.T) (*GameState, int) {
				gs := newCR108Game(t)
				gs.Active = 0
				gs.Seats[0].Life = 40
				card := newCR108Instant("Library Spell", 0, 3)
				gs.Seats[0].Library = append(gs.Seats[0].Library, card)
				perm := &ZoneCastPermission{
					Zone:                  ZoneLibrary,
					Keyword:               "bolas_citadel",
					ManaCost:              -1,
					LifeCostInsteadOfMana: 3,
					RequireController:     0,
					SourceName:            "Bolas's Citadel",
				}
				RegisterZoneCastGrant(gs, card, perm)
				if _, err := CastFromZone(gs, 0, card, ZoneLibrary, perm, nil); err != nil {
					t.Fatalf("setup CastFromZone: %v", err)
				}
				return gs, 0
			},
			cardOwner: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs, caster := tc.setup(t)
			got := stackPushControllerForCard(gs, tc.cardName)
			if got == -1 {
				t.Fatalf("no stack_push event found for %q", tc.cardName)
			}
			if got != caster {
				t.Fatalf("CR §108.4 — stack_push Seat=%d, want %d (caster)", got, caster)
			}
			// The discriminating assertion: when caster != cardOwner,
			// Controller MUST track caster, NOT owner.
			if caster != tc.cardOwner && got == tc.cardOwner {
				t.Fatalf("§108.4 violation — Controller defaulted to card.Owner=%d "+
					"instead of caster=%d. Bug class: cast pipeline stamping "+
					"Controller from card.Owner.", tc.cardOwner, caster)
			}
		})
	}
}
