package tournament

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// ---------------------------------------------------------------------------
// Combat Hat integration: PokerHat mode differentiates attacker selection
// ---------------------------------------------------------------------------

func TestPokerHat_HOLD_AttacksSelectively(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 3

	holdHat := hat.NewPokerHatWithMode(hat.ModeHold)
	gs.Seats[0].Hat = holdHat
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	// Seat 0: a vanilla 1/1 with no evasion — HOLD should skip it when
	// the opponent has a bigger blocker that would kill it in trade.
	vanilla := newEnginePerm(gs.Seats[0], "Savannah Lions", []string{"creature"}, 1, 1, 1)

	// Seat 1: a 5/5 untapped blocker — bigger than attacker, so
	// isOpenForAttacker returns false (defender has power >= attacker power).
	blocker := newEnginePerm(gs.Seats[1], "Serra Angel", []string{"creature"}, 5, 5, 5)
	_ = blocker

	legal := []*gameengine.Permanent{vanilla}
	chosen := holdHat.ChooseAttackers(gs, 0, legal)

	// HOLD's pickSafeAttackers only attacks if the attacker is evasive
	// or the opponent is "open" (no blocker with power >= attacker power)
	// or life <= 30. With a 5/5 blocker, 40 life, and no evasion, the
	// 1/1 should NOT attack.
	if len(chosen) != 0 {
		t.Errorf("HOLD should not attack 1/1 into 5/5 blocker at 40 life; got %d attackers", len(chosen))
	}
}

func TestPokerHat_RAISE_AttacksAll(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 3

	raiseHat := hat.NewPokerHatWithMode(hat.ModeRaise)
	gs.Seats[0].Hat = raiseHat
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	vanilla := newEnginePerm(gs.Seats[0], "Grizzly Bears", []string{"creature"}, 2, 2, 2)
	newEnginePerm(gs.Seats[1], "Wall of Stone", []string{"creature"}, 0, 7, 0)

	legal := []*gameengine.Permanent{vanilla}
	chosen := raiseHat.ChooseAttackers(gs, 0, legal)

	if len(chosen) != 1 {
		t.Errorf("RAISE should attack with everything; got %d attackers, want 1", len(chosen))
	}
}

func TestGreedyHat_AttacksAll(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 3
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	h := &hat.GreedyHat{}
	gs.Seats[0].Hat = h

	a := newEnginePerm(gs.Seats[0], "Grizzly Bears", []string{"creature"}, 2, 2, 2)
	b := newEnginePerm(gs.Seats[0], "Hill Giant", []string{"creature"}, 3, 3, 3)
	legal := []*gameengine.Permanent{a, b}

	chosen := h.ChooseAttackers(gs, 0, legal)
	if len(chosen) != 2 {
		t.Errorf("GreedyHat should attack with all legal; got %d, want 2", len(chosen))
	}
}

// ---------------------------------------------------------------------------
// Blocker assignment: PokerHat RAISE skips blocking when non-lethal
// ---------------------------------------------------------------------------

func TestPokerHat_RAISE_SkipsNonLethalBlocking(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 5

	raiseHat := hat.NewPokerHatWithMode(hat.ModeRaise)
	gs.Seats[1].Hat = raiseHat
	gs.Seats[1].Life = 40

	atk := newEnginePerm(gs.Seats[0], "Grizzly Bears", []string{"creature"}, 2, 2, 2)
	newEnginePerm(gs.Seats[1], "Hill Giant", []string{"creature"}, 3, 3, 3)

	blocks := raiseHat.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})

	// 2 damage < 40 life → RAISE shouldn't block.
	for _, blockers := range blocks {
		if len(blockers) > 0 {
			t.Error("RAISE should skip blocking when incoming damage is non-lethal")
		}
	}
}

func TestPokerHat_RAISE_BlocksLethalDamage(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 5

	raiseHat := hat.NewPokerHatWithMode(hat.ModeRaise)
	gs.Seats[1].Hat = raiseHat
	gs.Seats[1].Life = 2

	atk := newEnginePerm(gs.Seats[0], "Grizzly Bears", []string{"creature"}, 2, 2, 2)
	newEnginePerm(gs.Seats[1], "Hill Giant", []string{"creature"}, 3, 3, 3)

	blocks := raiseHat.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})

	// 2 damage >= 2 life → RAISE must block.
	totalBlockers := 0
	for _, blockers := range blocks {
		totalBlockers += len(blockers)
	}
	if totalBlockers == 0 {
		t.Error("RAISE should block when incoming damage is lethal")
	}
}

// ---------------------------------------------------------------------------
// Response thresholds: PokerHat mode gates counterspell usage
// ---------------------------------------------------------------------------

func TestPokerHat_ResponseThresholdByMode(t *testing.T) {
	// Build a counterspell card with proper AST structure.
	counterAST := &gameast.CardAST{
		Name: "Cancel",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}},
			},
		},
	}
	counterCard := &gameengine.Card{
		Name:  "Cancel",
		AST:   counterAST,
		Types: []string{"instant", "cost:3"},
	}

	// CMC 4 spell → stackItemScore = 4.
	// HOLD threshold = 2 → 4 >= 2 → counter
	// CALL threshold = 3 → 4 >= 3 → counter
	// RAISE threshold = 6 → 4 < 6 → NO counter
	midCard := &gameengine.Card{
		Name:  "Wrath of God",
		AST:   &gameast.CardAST{Name: "Wrath of God"},
		Types: []string{"sorcery", "cost:4"},
	}
	midStackItem := &gameengine.StackItem{
		Controller: 1,
		Card:       midCard,
		Effect:     &gameast.Destroy{Target: gameast.Filter{Base: "creature"}},
	}

	tests := []struct {
		name        string
		mode        hat.PlayerMode
		wantCounter bool
	}{
		{"HOLD counters CMC-4 spell", hat.ModeHold, true},
		{"CALL counters CMC-4 spell", hat.ModeCall, true},
		{"RAISE skips CMC-4 spell", hat.ModeRaise, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
			gs.Seats[0].Life = 40
			gs.Seats[1].Life = 40

			ph := hat.NewPokerHatWithMode(tc.mode)
			gs.Seats[0].Hat = ph
			gs.Seats[0].Hand = []*gameengine.Card{counterCard}
			gs.Seats[0].ManaPool = 5
			gameengine.EnsureTypedPool(gs.Seats[0])
			gameengine.AddMana(gs, gs.Seats[0], "any", 5, "test")

			resp := ph.ChooseResponse(gs, 0, midStackItem)
			got := resp != nil
			if got != tc.wantCounter {
				t.Errorf("mode=%s: got counter=%v, want %v", tc.mode, got, tc.wantCounter)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mode transitions: PokerHat transitions through HOLD → CALL → RAISE
// ---------------------------------------------------------------------------

func TestPokerHat_ModeTransitions(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(42)), nil)
	gs.CommanderFormat = true

	ph := hat.NewPokerHatWithMode(hat.ModeCall)
	gs.Seats[0].Hat = ph
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40

	// Start in CALL. Dropping to 3 life should trigger RAISE.
	gs.Seats[0].Life = 3
	for i := 0; i < 10; i++ {
		gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 0, Amount: 1})
	}

	if ph.Mode != hat.ModeRaise {
		t.Errorf("expected RAISE on low life; got %v", ph.Mode)
	}
}

func TestPokerHat_LateGameForceRaise(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 25

	ph := hat.NewPokerHatWithMode(hat.ModeCall)
	gs.Seats[0].Hat = ph
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	// Log enough events to clear cooldown, then trigger re-evaluate.
	for i := 0; i < 10; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})
	}

	if ph.Mode != hat.ModeRaise {
		t.Errorf("expected RAISE after turn 20; got %v", ph.Mode)
	}
}

// ---------------------------------------------------------------------------
// RAISE cascade: one seat's RAISE propagates to another
// ---------------------------------------------------------------------------

func TestPokerHat_RaiseCascade(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(42)), nil)

	ph1 := hat.NewPokerHatWithMode(hat.ModeCall)
	ph3 := hat.NewPokerHatWithMode(hat.ModeCall)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = ph1
	gs.Seats[2].Hat = &hat.GreedyHat{}
	gs.Seats[3].Hat = ph3

	gs.Seats[1].Life = 40
	// Seat 3 also low on life — so when it cascades to RAISE, the
	// re-evaluate path keeps RAISE locked (life <= raiseLifeThreshold).
	gs.Seats[3].Life = 8

	// Give seat 3 a big board so cascade triggers (bp >= 10).
	for i := 0; i < 5; i++ {
		newEnginePerm(gs.Seats[3], "Colossus", []string{"creature"}, 5, 5, 6)
	}

	// Warm up both hats past the mode-change cooldown (3 events).
	for i := 0; i < 5; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})
	}

	// Seat 1 transitions to RAISE (low life).
	gs.Seats[1].Life = 3
	for i := 0; i < 10; i++ {
		gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 1})
	}

	if ph1.Mode != hat.ModeRaise {
		t.Fatalf("seat 1 should be in RAISE; got %v", ph1.Mode)
	}

	// Seat 3 should have cascaded to RAISE (big board + low life keeps it locked).
	if ph3.Mode != hat.ModeRaise {
		t.Errorf("seat 3 should cascade to RAISE (board power=%d, life=%d); got %v",
			countBoardPower(gs.Seats[3]), gs.Seats[3].Life, ph3.Mode)
	}
}

// ---------------------------------------------------------------------------
// Zone-change through stack: countered spells fire MoveCard events
// ---------------------------------------------------------------------------

func TestZoneChange_CounteredSpellFiresEvents(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	card := &gameengine.Card{
		Name:  "Lightning Bolt",
		AST:   &gameast.CardAST{Name: "Lightning Bolt"},
		Types: []string{"instant", "cost:1"},
		Owner: 0,
	}

	// Push a spell onto the stack, mark it countered, resolve.
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		Effect:     &gameast.Damage{Amount: *gameast.NumInt(3)},
		Countered:  true,
	})

	gameengine.ResolveStackTop(gs)

	// The countered spell should end up in graveyard via MoveCard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Lightning Bolt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("countered spell should be in graveyard after ResolveStackTop")
	}

	// EventLog should contain a zone_change event for the countered spell.
	hasZoneChange := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "zone_change" {
			if from, ok := ev.Details["from_zone"].(string); ok && from == "stack" {
				hasZoneChange = true
				break
			}
		}
	}
	if !hasZoneChange {
		t.Error("expected zone_change event for countered spell moving stack→graveyard")
	}
}

// ---------------------------------------------------------------------------
// ChooseMode: GreedyHat picks index 0, PokerHat delegates to GreedyHat
// ---------------------------------------------------------------------------

func TestChooseMode_DefaultsToFirstMode(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)

	modes := []gameast.Effect{
		&gameast.Draw{Count: *gameast.NumInt(1)},
		&gameast.Damage{Amount: *gameast.NumInt(3)},
	}

	greedy := &hat.GreedyHat{}
	if got := greedy.ChooseMode(gs, 0, modes); got != 0 {
		t.Errorf("GreedyHat.ChooseMode: got %d, want 0", got)
	}

	poker := hat.NewPokerHat()
	if got := poker.ChooseMode(gs, 0, modes); got != 0 {
		t.Errorf("PokerHat.ChooseMode: got %d, want 0", got)
	}

	octo := &hat.OctoHat{}
	if got := octo.ChooseMode(gs, 0, modes); got != 0 {
		t.Errorf("OctoHat.ChooseMode: got %d, want 0", got)
	}
}

func TestChooseMode_EmptyReturnsNeg1(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)

	for _, h := range []gameengine.Hat{
		&hat.GreedyHat{},
		hat.NewPokerHat(),
		&hat.OctoHat{},
	} {
		if got := h.ChooseMode(gs, 0, nil); got != -1 {
			t.Errorf("%T.ChooseMode(nil modes): got %d, want -1", h, got)
		}
	}
}

// ---------------------------------------------------------------------------
// OrderReplacements: self-controlled first across all hat types
// ---------------------------------------------------------------------------

func TestOrderReplacements_SelfControlledFirst(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)

	mine := &gameengine.ReplacementEffect{ControllerSeat: 0, Timestamp: 2}
	theirs := &gameengine.ReplacementEffect{ControllerSeat: 1, Timestamp: 1}
	candidates := []*gameengine.ReplacementEffect{theirs, mine}

	for _, h := range []gameengine.Hat{
		&hat.GreedyHat{},
		hat.NewPokerHat(),
		&hat.OctoHat{},
	} {
		ordered := h.OrderReplacements(gs, 0, candidates)
		if len(ordered) != 2 {
			t.Fatalf("%T: got %d replacements, want 2", h, len(ordered))
		}
		if ordered[0].ControllerSeat != 0 {
			t.Errorf("%T: first replacement should be self-controlled (seat 0), got seat %d",
				h, ordered[0].ControllerSeat)
		}
	}
}

// ---------------------------------------------------------------------------
// Mixed-hat 500-game tournament: stability + zero crashes
// ---------------------------------------------------------------------------

func TestMixedHatTournament_500Games(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need at least 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
		paths = paths[:4]
	}
	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}

	hatFns := make([]HatFactory, nSeats)
	for i := range hatFns {
		switch i % 3 {
		case 0:
			hatFns[i] = func() gameengine.Hat { return &hat.GreedyHat{} }
		case 1:
			hatFns[i] = func() gameengine.Hat { return hat.NewPokerHat() }
		case 2:
			hatFns[i] = func() gameengine.Hat { return hat.NewPokerHatWithMode(hat.ModeHold) }
		}
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        500,
		Seed:          314,
		HatFactories:  hatFns,
		Workers:       4,
		CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 500 {
		t.Fatalf("expected 500 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	crashRate := float64(r.Crashes) / 500.0
	if crashRate > 0.05 {
		t.Errorf("crash rate %.1f%% exceeds 5%% threshold (crashes=%d)", crashRate*100, r.Crashes)
	}
	if r.AvgTurns <= 0 {
		t.Error("avg turns should be > 0")
	}
	t.Logf("500-game mixed-hat (%d seats): games=%d crashes=%d draws=%d avg_turns=%.1f gps=%.1f mode_changes=%d",
		nSeats, r.Games, r.Crashes, r.Draws, r.AvgTurns, r.GamesPerSecond, r.TotalModeChanges)
	t.Logf("  wins: %v", r.WinsByCommander)
}

// ---------------------------------------------------------------------------
// All-hat-types tournament: Greedy + Poker + Octo at 200 games
// ---------------------------------------------------------------------------

func TestAllHatTypes_Tournament(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need at least 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
		paths = paths[:4]
	}
	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}

	hatTypes := []HatFactory{
		func() gameengine.Hat { return &hat.GreedyHat{} },
		func() gameengine.Hat { return hat.NewPokerHat() },
		func() gameengine.Hat { return &hat.OctoHat{} },
		func() gameengine.Hat { return hat.NewPokerHatWithMode(hat.ModeHold) },
	}
	hatFns := make([]HatFactory, nSeats)
	for i := range hatFns {
		hatFns[i] = hatTypes[i%len(hatTypes)]
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        200,
		Seed:          271,
		HatFactories:  hatFns,
		Workers:       4,
		CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 200 {
		t.Fatalf("expected 200 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	crashRate := float64(r.Crashes) / 200.0
	if crashRate > 0.10 {
		t.Errorf("crash rate %.1f%% too high with OctoHat stress (crashes=%d)", crashRate*100, r.Crashes)
	}
	t.Logf("200-game all-hats (%d seats): games=%d crashes=%d draws=%d avg_turns=%.1f gps=%.1f",
		nSeats, r.Games, r.Crashes, r.Draws, r.AvgTurns, r.GamesPerSecond)
}

// ---------------------------------------------------------------------------
// OctoHat stress: 100 games with all-OctoHat seats
// ---------------------------------------------------------------------------

func TestOctoHatStress_100Games(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need at least 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
		paths = paths[:4]
	}
	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}

	cfg := TournamentConfig{
		Decks:  decks,
		NSeats: nSeats,
		NGames: 100,
		Seed:   666,
		HatFactories: []HatFactory{
			func() gameengine.Hat { return &hat.OctoHat{} },
		},
		Workers:       2,
		CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 100 {
		t.Fatalf("expected 100 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	crashRate := float64(r.Crashes) / 100.0
	if crashRate > 0.20 {
		t.Errorf("OctoHat crash rate %.1f%% exceeds 20%% threshold (crashes=%d)", crashRate*100, r.Crashes)
	}
	t.Logf("100-game OctoHat stress (%d seats): games=%d crashes=%d draws=%d avg_turns=%.1f",
		nSeats, r.Games, r.Crashes, r.Draws, r.AvgTurns)
}

// ---------------------------------------------------------------------------
// ChooseDiscard: GreedyHat discards highest CMC, OctoHat discards lowest
// ---------------------------------------------------------------------------

func TestChooseDiscard_Ordering(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)

	cheap := &gameengine.Card{Name: "Opt", Types: []string{"instant", "cost:1"}}
	mid := &gameengine.Card{Name: "Divination", Types: []string{"sorcery", "cost:3"}}
	expensive := &gameengine.Card{Name: "Bolas's Citadel", Types: []string{"artifact", "cost:6"}}
	hand := []*gameengine.Card{cheap, mid, expensive}

	// GreedyHat discards highest CMC first.
	greedy := &hat.GreedyHat{}
	gDiscard := greedy.ChooseDiscard(gs, 0, hand, 1)
	if len(gDiscard) != 1 || gDiscard[0].Name != "Bolas's Citadel" {
		t.Errorf("GreedyHat should discard highest CMC; got %v", cardNames(gDiscard))
	}

	// OctoHat discards lowest CMC first.
	octo := &hat.OctoHat{}
	oDiscard := octo.ChooseDiscard(gs, 0, hand, 1)
	if len(oDiscard) != 1 || oDiscard[0].Name != "Opt" {
		t.Errorf("OctoHat should discard lowest CMC; got %v", cardNames(oDiscard))
	}
}

// ---------------------------------------------------------------------------
// Attack target selection: Greedy targets leader, Poker targets open/leader
// ---------------------------------------------------------------------------

func TestAttackTargetSelection_LeaderFocus(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(42)), nil)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 20
	gs.Seats[3].Life = 40

	// Give seat 1 a huge board → highest leader score.
	for i := 0; i < 5; i++ {
		newEnginePerm(gs.Seats[1], "Dragon", []string{"creature"}, 5, 5, 6)
	}

	attacker := newEnginePerm(gs.Seats[0], "Attacker", []string{"creature"}, 3, 3, 3)
	legalDefenders := []int{1, 2, 3}

	greedy := &hat.GreedyHat{}
	target := greedy.ChooseAttackTarget(gs, 0, attacker, legalDefenders)

	// Greedy should target seat 1 (highest life + board power combined).
	if target != 1 {
		t.Errorf("GreedyHat should target the leader (seat 1); got seat %d", target)
	}
}

// ---------------------------------------------------------------------------
// ChooseScry: GreedyHat keeps affordable cards on top
// ---------------------------------------------------------------------------

func TestChooseScry_KeepsAffordable(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Life = 20
	gs.Seats[0].ManaPool = 3
	gameengine.EnsureTypedPool(gs.Seats[0])
	gameengine.AddMana(gs, gs.Seats[0], "any", 3, "test")

	cheap := &gameengine.Card{Name: "Opt", Types: []string{"instant", "cost:1"}}
	pricey := &gameengine.Card{Name: "Eldrazi", Types: []string{"creature", "cost:12"}}

	greedy := &hat.GreedyHat{}
	top, bottom := greedy.ChooseScry(gs, 0, []*gameengine.Card{cheap, pricey})

	if len(top) == 0 {
		t.Fatal("expected at least one card on top")
	}
	if top[0].Name != "Opt" {
		t.Errorf("cheap card should be on top; got %s", top[0].Name)
	}
	if len(bottom) == 0 || bottom[0].Name != "Eldrazi" {
		t.Error("expensive card should be on bottom")
	}
}

// ---------------------------------------------------------------------------
// ChooseSurveil: GreedyHat puts expensive cards in graveyard
// ---------------------------------------------------------------------------

func TestChooseSurveil_ExpensiveToGraveyard(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 2
	gameengine.EnsureTypedPool(gs.Seats[0])
	gameengine.AddMana(gs, gs.Seats[0], "any", 2, "test")

	cheap := &gameengine.Card{Name: "Opt", Types: []string{"instant", "cost:1"}}
	pricey := &gameengine.Card{Name: "Eldrazi", Types: []string{"creature", "cost:12"}}

	greedy := &hat.GreedyHat{}
	gy, top := greedy.ChooseSurveil(gs, 0, []*gameengine.Card{cheap, pricey})

	if len(gy) == 0 || gy[0].Name != "Eldrazi" {
		t.Error("expensive card should go to graveyard via surveil")
	}
	if len(top) == 0 || top[0].Name != "Opt" {
		t.Error("cheap card should stay on top via surveil")
	}
}

// ---------------------------------------------------------------------------
// Seed determinism with different hat types
// ---------------------------------------------------------------------------

func TestSeedDeterminism_WithPokerHat(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	decks := make([]*deckparser.TournamentDeck, 0, 2)
	for _, p := range paths[:2] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}

	cfg := TournamentConfig{
		Decks:  decks,
		NSeats: 2,
		NGames: 20,
		Seed:   42,
		HatFactories: []HatFactory{
			func() gameengine.Hat { return hat.NewPokerHat() },
			func() gameengine.Hat { return hat.NewPokerHat() },
		},
		Workers:       1,
		CommanderMode: true,
	}

	r1, err := Run(cfg)
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	r2, err := Run(cfg)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}

	if r1.Crashes != r2.Crashes {
		t.Fatalf("crashes differ: %d vs %d", r1.Crashes, r2.Crashes)
	}
	if r1.Draws != r2.Draws {
		t.Fatalf("draws differ: %d vs %d", r1.Draws, r2.Draws)
	}
	for _, name := range r1.CommanderNames {
		if r1.WinsByCommander[name] != r2.WinsByCommander[name] {
			t.Fatalf("wins for %q differ: %d vs %d",
				name, r1.WinsByCommander[name], r2.WinsByCommander[name])
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newEnginePerm(seat *gameengine.Seat, name string, types []string, power, toughness, cmc int) *gameengine.Permanent {
	card := &gameengine.Card{
		Name:          name,
		Types:         types,
		BasePower:     power,
		BaseToughness: toughness,
		AST:           &gameast.CardAST{Name: name},
	}
	if cmc > 0 {
		card.Types = append(card.Types, "cost:"+itoa(cmc))
	}
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat.Idx,
		Owner:      seat.Idx,
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

func countBoardPower(seat *gameengine.Seat) int {
	total := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			if pw := p.Power(); pw > 0 {
				total += pw
			}
		}
	}
	return total
}

func cardNames(cards []*gameengine.Card) []string {
	names := make([]string, len(cards))
	for i, c := range cards {
		if c != nil {
			names[i] = c.Name
		}
	}
	return names
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ---------------------------------------------------------------------------
// MCTSHat integration: runs through full tournament without crashing
// ---------------------------------------------------------------------------

func TestMCTSHat_TournamentIntegration(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
	}
	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}

	// Mix of MCTSHat (wrapping PokerHat) and plain PokerHat.
	factories := make([]HatFactory, nSeats)
	for i := 0; i < nSeats; i++ {
		if i%2 == 0 {
			factories[i] = func() gameengine.Hat {
				inner := hat.NewPokerHat()
				return hat.NewMCTSHat(inner, nil, 50)
			}
		} else {
			factories[i] = func() gameengine.Hat {
				return hat.NewPokerHat()
			}
		}
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        100,
		Seed:          99,
		HatFactories:  factories,
		Workers:       2,
		CommanderMode: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Crashes > 0 {
		t.Errorf("MCTSHat tournament had %d crashes", result.Crashes)
	}
	if result.Games != 100 {
		t.Errorf("expected 100 games, got %d", result.Games)
	}
}

func TestMCTSHat_RolloutIntegration(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
	}
	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}

	turnRunner := TurnRunnerForRollout()
	factories := make([]HatFactory, nSeats)
	for i := 0; i < nSeats; i++ {
		if i%2 == 0 {
			factories[i] = func() gameengine.Hat {
				inner := hat.NewPokerHat()
				mh := hat.NewMCTSHat(inner, nil, 200)
				mh.TurnRunner = turnRunner
				return mh
			}
		} else {
			factories[i] = func() gameengine.Hat {
				return hat.NewPokerHat()
			}
		}
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        20,
		Seed:          42,
		HatFactories:  factories,
		Workers:       1,
		CommanderMode: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Crashes > 0 {
		t.Errorf("MCTSHat rollout tournament had %d crashes", result.Crashes)
	}
	if result.Games < 1 {
		t.Error("expected at least 1 completed game")
	}
}
