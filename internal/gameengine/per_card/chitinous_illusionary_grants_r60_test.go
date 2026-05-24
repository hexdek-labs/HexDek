package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 follow-up to the loki round-2 + goldilocks-r60 cross-validated
// residual: Chitinous Crawler + Illusionary Mask both AST'd to the
// over-applied impulse_play modkind, causing the engine's
// `case "impulse_play":` arm to exile an unrelated library card and
// register a ZoneCastPermission against it with SourceTimestamp=0.
//
// Combined fix:
//   1. resolve_helpers.go impulse_play arm gates on args[0] — skips
//      library-top exile when the args mention "face down" / "from your
//      hand" / "from your graveyard".
//   2. Per-card handlers (this package) implement the correct semantic
//      for both cards.
//
// These tests pin both halves: the engine-side guard suppresses the
// stray grant for the wrong-semantic AST patterns, AND each per-card
// handler runs the right state mutation.

// -----------------------------------------------------------------------------
// Engine-side guard — impulse_play arm now skips library-top exile for
// the four non-library zone hints surfaced by the corpus audit.
// -----------------------------------------------------------------------------

// TestImpulsePlayGuard_SkipsFaceDownArg verifies the resolve_helpers.go
// guard for Illusionary Mask's exact AST args[0] string.
func TestImpulsePlayGuard_SkipsFaceDownArg(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Swamp")

	src := addPerm(gs, 0, "Illusionary Mask", "artifact")

	// Mimic the resolve.go ResolveEffect path that fires ModKind=impulse_play.
	resolveImpulsePlayForTest(gs, src, "you may cast that card face down as a 2/2 creature spell without paying its mana cost")

	// The library should be UNTOUCHED — the guard kicks in.
	if len(gs.Seats[0].Library) != 1 {
		t.Fatalf("library top should NOT have been exiled (got len=%d)", len(gs.Seats[0].Library))
	}
	if len(gs.ZoneCastGrants) != 0 {
		t.Fatalf("no ZoneCastPermission should be registered (got %d)", len(gs.ZoneCastGrants))
	}
}

// TestImpulsePlayGuard_SkipsFromYourGraveyardArg covers the Oskar / Ichor
// Aberration / Walk-In Closet family AND Chitinous Crawler's adjacent
// "from your graveyard" cost hint.
func TestImpulsePlayGuard_SkipsFromYourGraveyardArg(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Swamp")

	src := addPerm(gs, 0, "Oskar, Rubbish Reclaimer", "creature")

	resolveImpulsePlayForTest(gs, src, "you may cast it from your graveyard")

	if len(gs.Seats[0].Library) != 1 {
		t.Fatalf("library top should NOT have been exiled")
	}
	if len(gs.ZoneCastGrants) != 0 {
		t.Fatalf("no ZoneCastPermission should be registered")
	}
}

// TestImpulsePlayGuard_SkipsFromYourHandArg covers Maelstrom Archangel /
// Wildfire Eternal / Twinning Glass etc. + Illusionary Mask's hand-side.
func TestImpulsePlayGuard_SkipsFromYourHandArg(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Swamp")

	src := addPerm(gs, 0, "Maelstrom Archangel", "creature")

	resolveImpulsePlayForTest(gs, src, "you may cast a spell from your hand without paying its mana cost")

	if len(gs.Seats[0].Library) != 1 {
		t.Fatalf("library top should NOT have been exiled")
	}
	if len(gs.ZoneCastGrants) != 0 {
		t.Fatalf("no ZoneCastPermission should be registered")
	}
}

// TestImpulsePlayGuard_LibraryTopForCanonicalArg confirms the canonical
// "exile from library top, you may play it this turn" pattern (Jeska's
// Will, Chandra Fire Artisan, Bonehoard Dracosaur) still creates a
// grant — AND the grant now carries SourceTimestamp.
func TestImpulsePlayGuard_LibraryTopForCanonicalArg(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Swamp")

	src := addPerm(gs, 0, "Jeska's Will", "sorcery")
	src.Timestamp = 4242 // explicit so we can assert SourceTimestamp stamp

	resolveImpulsePlayForTest(gs, src, "you may play them this turn")

	if len(gs.Seats[0].Library) != 0 {
		t.Fatalf("library top should have been exiled (got len=%d)", len(gs.Seats[0].Library))
	}
	if len(gs.ZoneCastGrants) != 1 {
		t.Fatalf("expected 1 ZoneCastPermission, got %d", len(gs.ZoneCastGrants))
	}
	for _, p := range gs.ZoneCastGrants {
		if p.SourceTimestamp != 4242 {
			t.Errorf("SourceTimestamp not stamped: want 4242, got %d", p.SourceTimestamp)
		}
		if p.GrantTurn != gs.Turn {
			t.Errorf("GrantTurn: want %d, got %d", gs.Turn, p.GrantTurn)
		}
	}
}

// resolveImpulsePlayForTest invokes the engine's impulse_play arm via
// the public ResolveEffect entry — same dispatch path real activations
// use.
func resolveImpulsePlayForTest(gs *gameengine.GameState, src *gameengine.Permanent, argText string) {
	gameengine.ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "impulse_play",
		Args:    []interface{}{argText},
	})
}

// -----------------------------------------------------------------------------
// Chitinous Crawler — per_card handler
// -----------------------------------------------------------------------------

// TestChitinousCrawler_ExilesGraveyardCardAndRegistersGrant verifies the
// activated-ability handler picks a creature card from the controller's
// graveyard, moves it to exile, and registers a properly-stamped grant.
func TestChitinousCrawler_ExilesGraveyardCardAndRegistersGrant(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5

	// Seed graveyard with an eligible permanent card.
	gravePerm := &gameengine.Card{Name: "Sakura-Tribe Elder", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gravePerm)

	// Place Chitinous Crawler on the battlefield.
	src := addPerm(gs, 0, "Chitinous Crawler", "creature")
	src.Timestamp = 9001

	chitinousCrawlerActivated(gs, src, 2, nil)

	// Graveyard should be empty, exile should hold the chosen card.
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("graveyard should be drained by 1 card, got %d remaining", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Exile) != 1 || gs.Seats[0].Exile[0] != gravePerm {
		t.Errorf("chosen card should be in exile, got %v", gs.Seats[0].Exile)
	}

	// Grant should be registered with proper SourceTimestamp + GrantTurn.
	grant := gameengine.GetZoneCastGrant(gs, gravePerm)
	if grant == nil {
		t.Fatalf("ZoneCastPermission for the exiled card not registered")
	}
	if grant.SourceTimestamp != 9001 {
		t.Errorf("SourceTimestamp: want 9001, got %d", grant.SourceTimestamp)
	}
	if grant.GrantTurn != 5 {
		t.Errorf("GrantTurn: want 5, got %d", grant.GrantTurn)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.RequireController != 0 {
		t.Errorf("RequireController: want 0, got %d", grant.RequireController)
	}
	if src.Flags["chitinous_crawler_handled"] != 1 {
		t.Errorf("source flag not stamped")
	}
}

// TestChitinousCrawler_GrantExpiresAtEndOfTurn pins the lifecycle:
// after EOT cleanup runs, the grant is gone.
func TestChitinousCrawler_GrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5

	gravePerm := &gameengine.Card{Name: "Solemn Simulacrum", Owner: 0, Types: []string{"artifact", "creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gravePerm)

	src := addPerm(gs, 0, "Chitinous Crawler", "creature")
	src.Timestamp = 9001
	chitinousCrawlerActivated(gs, src, 2, nil)

	if gameengine.GetZoneCastGrant(gs, gravePerm) == nil {
		t.Fatalf("grant not registered at activation time")
	}

	// Simulate EOT cleanup.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")

	if gameengine.GetZoneCastGrant(gs, gravePerm) != nil {
		t.Errorf("grant should have been reaped by EOT cleanup")
	}
}

// TestChitinousCrawler_GrantExpiresOnSourceLTB pins the source-LTB path:
// when Chitinous Crawler leaves the battlefield, ExpireSourceGrants
// reaps the grant immediately via SourceTimestamp.
func TestChitinousCrawler_GrantExpiresOnSourceLTB(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5

	gravePerm := &gameengine.Card{Name: "Hornet Queen", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gravePerm)

	src := addPerm(gs, 0, "Chitinous Crawler", "creature")
	src.Timestamp = 9001
	chitinousCrawlerActivated(gs, src, 2, nil)

	if gameengine.GetZoneCastGrant(gs, gravePerm) == nil {
		t.Fatalf("grant not registered at activation time")
	}

	gameengine.ExpireSourceGrants(gs, 9001)

	if gameengine.GetZoneCastGrant(gs, gravePerm) != nil {
		t.Errorf("grant should have been reaped on source LTB via SourceTimestamp")
	}
}

// TestChitinousCrawler_NoEligibleGraveyardCard guards the empty-graveyard
// case: the handler emits a fail breadcrumb and does NOT register a stray
// grant.
func TestChitinousCrawler_NoEligibleGraveyardCard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5

	src := addPerm(gs, 0, "Chitinous Crawler", "creature")
	src.Timestamp = 9001
	chitinousCrawlerActivated(gs, src, 2, nil)

	if len(gs.ZoneCastGrants) != 0 {
		t.Errorf("no grant should be registered when graveyard is empty (got %d)", len(gs.ZoneCastGrants))
	}
	if src.Flags["chitinous_crawler_handled"] == 1 {
		t.Errorf("source flag should NOT be stamped on empty-graveyard fail")
	}
}

// -----------------------------------------------------------------------------
// Illusionary Mask — per_card handler
// -----------------------------------------------------------------------------

// TestIllusionaryMask_StampsHandledFlagOnEligibleHandCard verifies the
// handler picks an eligible creature card from hand (CMC ≤ X paid) and
// stamps the source flag. The full face-down cast is deferred (out of
// scope per the handler docblock); the test pins the minimum-correct
// behavior the R60 sweep ships.
func TestIllusionaryMask_StampsHandledFlagOnEligibleHandCard(t *testing.T) {
	gs := newGame(t, 2)

	// Hand contains a CMC-2 creature.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Grizzly Bears", Owner: 0, Types: []string{"creature", "cmc:2"},
	})

	src := addPerm(gs, 0, "Illusionary Mask", "artifact")
	src.Timestamp = 7777

	illusionaryMaskActivated(gs, src, 0, map[string]interface{}{"x_paid": 2})

	if src.Flags["illusionary_mask_handled"] != 1 {
		t.Errorf("illusionary_mask_handled flag not stamped")
	}
	// And critically: no stray ZoneCastPermission against a library card.
	if len(gs.ZoneCastGrants) != 0 {
		t.Errorf("Illusionary Mask handler must not leak a ZoneCastPermission, got %d", len(gs.ZoneCastGrants))
	}
}

// TestIllusionaryMask_NoEligibleHandCard guards the empty-hand /
// insufficient-mana case: the handler bails without stamping the flag
// or registering a grant.
func TestIllusionaryMask_NoEligibleHandCard(t *testing.T) {
	gs := newGame(t, 2)

	// Hand has only a high-CMC creature; X=1 paid, can't satisfy.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Emrakul, the Aeons Torn", Owner: 0, Types: []string{"creature", "cmc:15"},
	})

	src := addPerm(gs, 0, "Illusionary Mask", "artifact")
	illusionaryMaskActivated(gs, src, 0, map[string]interface{}{"x_paid": 1})

	if src.Flags["illusionary_mask_handled"] == 1 {
		t.Errorf("handled flag should NOT be stamped when no eligible card exists")
	}
	if len(gs.ZoneCastGrants) != 0 {
		t.Errorf("no grant should be registered, got %d", len(gs.ZoneCastGrants))
	}
}

// TestIllusionaryMask_FullActivationCycleStaysClean is the
// cross-validation: run the whole activation cycle (engine ResolveEffect
// for the impulse_play ModKind + per_card handler) and confirm no
// ZoneCastPermission survives — neither the engine guard nor the
// handler creates one.
func TestIllusionaryMask_FullActivationCycleStaysClean(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Swamp", "Forest", "Mountain")

	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Grizzly Bears", Owner: 0, Types: []string{"creature", "cmc:2"},
	})

	src := addPerm(gs, 0, "Illusionary Mask", "artifact")

	// Engine arm — should skip library-top exile because of the "face down" hint.
	resolveImpulsePlayForTest(gs, src, "you may cast that card face down as a 2/2 creature spell without paying its mana cost")
	// Per_card handler — should stamp flag, not create a grant.
	illusionaryMaskActivated(gs, src, 0, map[string]interface{}{"x_paid": 2})

	// Library untouched.
	if len(gs.Seats[0].Library) != 3 {
		t.Errorf("library top should NOT have been exiled (got %d cards)", len(gs.Seats[0].Library))
	}
	// No grant.
	if len(gs.ZoneCastGrants) != 0 {
		t.Errorf("no ZoneCastPermission should be registered (got %d)", len(gs.ZoneCastGrants))
	}
	// Invariant clean.
	if vs := gameengine.RunAllInvariants(gs); len(vs) > 0 {
		for _, v := range vs {
			if v.Name == "ZoneCastGrantExpiry" {
				t.Errorf("ZoneCastGrantExpiry invariant tripped: %s", v.Message)
			}
		}
	}
}
