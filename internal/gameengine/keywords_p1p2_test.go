package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newP1P2Game(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	return gs
}

func addP1P2Battlefield(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string{}, types...),
	}
	p := &Permanent{
		Card:          card,
		Controller:    seat,
		Owner:         seat,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
		SummoningSick: false,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func addP1P2GraveyardCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
		CMC:   cost,
	}
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
	return c
}

func addP1P2HandCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
		CMC:   cost,
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// ---------------------------------------------------------------------------
// 1. Unearth tests — CR §702.84
// ---------------------------------------------------------------------------

func TestUnearth_ReturnFromGraveyardWithHaste(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	card := addP1P2GraveyardCard(gs, 0, "Dregscape Zombie", 2, "creature")

	perm := ApplyUnearth(gs, 0, card, 2)
	if perm == nil {
		t.Fatal("ApplyUnearth should return a non-nil permanent")
	}

	// Mana should be spent.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana remaining, got %d", gs.Seats[0].ManaPool)
	}

	// Should be on battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == perm {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("unearthed permanent should be on battlefield")
	}

	// Should have haste.
	if !perm.HasKeyword("haste") {
		t.Fatal("unearthed permanent should have haste")
	}

	// Should be marked as unearthed.
	if perm.Flags["unearthed"] != 1 {
		t.Fatal("unearthed permanent should have 'unearthed' flag")
	}

	// Should NOT be in graveyard anymore.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("card should not be in graveyard after unearth")
		}
	}
}

func TestUnearth_InsufficientMana(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 1

	card := addP1P2GraveyardCard(gs, 0, "Dregscape Zombie", 2, "creature")

	perm := ApplyUnearth(gs, 0, card, 3)
	if perm != nil {
		t.Fatal("ApplyUnearth should return nil with insufficient mana")
	}

	// Card should still be in graveyard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card should remain in graveyard when unearth fails")
	}
}

func TestUnearth_DelayedExileAtEOT(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	card := addP1P2GraveyardCard(gs, 0, "Dregscape Zombie", 2, "creature")

	perm := ApplyUnearth(gs, 0, card, 2)
	if perm == nil {
		t.Fatal("ApplyUnearth returned nil")
	}

	// Should have registered a delayed trigger.
	foundDT := false
	for _, dt := range gs.DelayedTriggers {
		if dt.TriggerAt == "end_of_turn" && dt.OneShot {
			foundDT = true
			break
		}
	}
	if !foundDT {
		t.Fatal("unearth should register an end_of_turn delayed trigger")
	}
}

func TestUnearth_ReplacementExilesInsteadOfGraveyard(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	card := addP1P2GraveyardCard(gs, 0, "Dregscape Zombie", 2, "creature")

	perm := ApplyUnearth(gs, 0, card, 2)
	if perm == nil {
		t.Fatal("ApplyUnearth returned nil")
	}

	// Should have registered a replacement effect.
	foundRepl := false
	for _, repl := range gs.Replacements {
		if repl.SourcePerm == perm && repl.EventType == "would_change_zone" {
			foundRepl = true
			break
		}
	}
	if !foundRepl {
		t.Fatal("unearth should register a would_change_zone replacement effect")
	}
}

func TestNewUnearthPermission(t *testing.T) {
	perm := NewUnearthPermission(3)
	if perm.Zone != ZoneGraveyard {
		t.Fatalf("expected zone %q, got %q", ZoneGraveyard, perm.Zone)
	}
	if perm.Keyword != "unearth" {
		t.Fatalf("expected keyword 'unearth', got %q", perm.Keyword)
	}
	if perm.ManaCost != 3 {
		t.Fatalf("expected mana cost 3, got %d", perm.ManaCost)
	}
}

// ---------------------------------------------------------------------------
// 2. Foretell tests — CR §702.143
// ---------------------------------------------------------------------------

func TestForetellExile_Success(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := addP1P2HandCard(gs, 0, "Saw It Coming", 3, "instant")

	ok := ForetellExile(gs, 0, card)
	if !ok {
		t.Fatal("ForetellExile should succeed")
	}

	// Mana spent.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana remaining, got %d", gs.Seats[0].ManaPool)
	}

	// Card should be in exile, face-down.
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("foretold card should be in exile")
	}
	if !card.FaceDown {
		t.Fatal("foretold card should be face-down")
	}

	// Card should NOT be in hand.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("foretold card should not be in hand")
		}
	}
}

func TestForetellExile_NotYourTurn(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 1 // NOT seat 0's turn.
	gs.Seats[0].ManaPool = 5

	card := addP1P2HandCard(gs, 0, "Saw It Coming", 3, "instant")

	ok := ForetellExile(gs, 0, card)
	if ok {
		t.Fatal("ForetellExile should fail when it's not your turn")
	}
}

func TestForetellExile_InsufficientMana(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1

	card := addP1P2HandCard(gs, 0, "Saw It Coming", 3, "instant")

	ok := ForetellExile(gs, 0, card)
	if ok {
		t.Fatal("ForetellExile should fail with insufficient mana")
	}
}

func TestForetellCastPermission(t *testing.T) {
	perm := NewForetellCastPermission(2)
	if perm.Zone != ZoneExile {
		t.Fatalf("expected zone %q, got %q", ZoneExile, perm.Zone)
	}
	if perm.Keyword != "foretell" {
		t.Fatalf("expected keyword 'foretell', got %q", perm.Keyword)
	}
	if perm.ManaCost != 2 {
		t.Fatalf("expected mana cost 2, got %d", perm.ManaCost)
	}
}

// ---------------------------------------------------------------------------
// 3. Entwine tests — CR §702.42
// ---------------------------------------------------------------------------

func TestCanPayEntwine(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 7

	// Spell costs 3, entwine costs 2, total 5 -- affordable.
	if !CanPayEntwine(gs, 0, 3, 2) {
		t.Fatal("should be able to pay entwine with 7 mana (need 5)")
	}

	// Not enough mana.
	gs.Seats[0].ManaPool = 4
	if CanPayEntwine(gs, 0, 3, 2) {
		t.Fatal("should NOT be able to pay entwine with 4 mana (need 5)")
	}
}

func TestPayEntwineCost(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	card := &Card{Name: "Tooth and Nail", CMC: 7}
	ok := PayEntwineCost(gs, 0, card, 2)
	if !ok {
		t.Fatal("PayEntwineCost should succeed with 5 mana for cost 2")
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana remaining, got %d", gs.Seats[0].ManaPool)
	}
}

func TestShouldEntwine_Greedy(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 10

	if !ShouldEntwine(gs, 0, 3, 2) {
		t.Fatal("greedy policy should always entwine when affordable")
	}
}

// ---------------------------------------------------------------------------
// 4. Buyback tests — CR §702.27
// ---------------------------------------------------------------------------

func TestNewBuybackCost_CanPay(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	cost := NewBuybackCost(3)
	if !cost.CanPayFn(gs, 0) {
		t.Fatal("should be able to pay buyback 3 with 5 mana")
	}

	gs.Seats[0].ManaPool = 2
	if cost.CanPayFn(gs, 0) {
		t.Fatal("should NOT be able to pay buyback 3 with 2 mana")
	}
}

func TestNewBuybackCost_Pay(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Seats[0].ManaPool = 5

	cost := NewBuybackCost(3)
	ok := cost.PayFn(gs, 0)
	if !ok {
		t.Fatal("buyback payment should succeed")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("expected 2 mana remaining, got %d", gs.Seats[0].ManaPool)
	}
}

func TestShouldReturnToHandOnResolve(t *testing.T) {
	item := &StackItem{
		CostMeta: map[string]interface{}{"buyback": true},
	}
	if !ShouldReturnToHandOnResolve(item) {
		t.Fatal("should detect buyback on stack item")
	}

	item2 := &StackItem{
		CostMeta: map[string]interface{}{"buyback": false},
	}
	if ShouldReturnToHandOnResolve(item2) {
		t.Fatal("should not detect buyback when set to false")
	}

	item3 := &StackItem{}
	if ShouldReturnToHandOnResolve(item3) {
		t.Fatal("should not detect buyback on empty CostMeta")
	}
}

// ---------------------------------------------------------------------------
// 5. Wither tests — CR §702.80
// ---------------------------------------------------------------------------

func TestApplyWitherDamageToCreature(t *testing.T) {
	gs := newP1P2Game(t)

	src := addP1P2Battlefield(gs, 0, "Boggart Ram-Gang", 3, 3, "creature")
	src.Flags["kw:wither"] = 1

	target := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	counters := ApplyWitherDamageToCreature(gs, src, target, 3)
	if counters != 3 {
		t.Fatalf("expected 3 -1/-1 counters, got %d", counters)
	}
	if target.Counters["-1/-1"] != 3 {
		t.Fatalf("expected 3 -1/-1 counters on target, got %d", target.Counters["-1/-1"])
	}
}

func TestHasWither(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Boggart Ram-Gang", 3, 3, "creature")
	p.Flags["kw:wither"] = 1

	if !HasWither(p) {
		t.Fatal("creature with kw:wither flag should have wither")
	}

	p2 := addP1P2Battlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	if HasWither(p2) {
		t.Fatal("creature without wither should not have wither")
	}
}

func TestShouldApplyWitherDamage(t *testing.T) {
	gs := newP1P2Game(t)
	src := addP1P2Battlefield(gs, 0, "Boggart Ram-Gang", 3, 3, "creature")
	src.Flags["kw:wither"] = 1
	target := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	if !ShouldApplyWitherDamage(src, target) {
		t.Fatal("should apply wither damage from wither creature to creature")
	}

	// Non-wither source.
	src2 := addP1P2Battlefield(gs, 0, "Lightning Bolt", 3, 3, "creature")
	if ShouldApplyWitherDamage(src2, target) {
		t.Fatal("should NOT apply wither from non-wither creature")
	}
}

// ---------------------------------------------------------------------------
// 6. Disturb tests — CR §702.146
// ---------------------------------------------------------------------------

func TestNewDisturbPermission(t *testing.T) {
	perm := NewDisturbPermission(4)
	if perm.Zone != ZoneGraveyard {
		t.Fatalf("expected zone %q, got %q", ZoneGraveyard, perm.Zone)
	}
	if perm.Keyword != "disturb" {
		t.Fatalf("expected keyword 'disturb', got %q", perm.Keyword)
	}
	if perm.ManaCost != 4 {
		t.Fatalf("expected mana cost 4, got %d", perm.ManaCost)
	}
}

func TestApplyDisturbETB_TransformsToBackFace(t *testing.T) {
	gs := newP1P2Game(t)

	frontAST := &gameast.CardAST{Name: "Faithful Mending"}
	backAST := &gameast.CardAST{Name: "Faithbound Judge"}

	card := &Card{
		Name:  "Faithful Mending",
		Owner: 0,
		AST:   frontAST,
		Types: []string{"creature"},
	}

	perm := &Permanent{
		Card:          card,
		Controller:    0,
		Owner:         0,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
		FrontFaceAST:  frontAST,
		BackFaceAST:   backAST,
		FrontFaceName: "Faithful Mending",
		BackFaceName:  "Faithbound Judge",
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	ApplyDisturbETB(gs, perm)

	// Should be transformed.
	if !perm.Transformed {
		t.Fatal("disturbed permanent should be transformed")
	}

	// Should use back face AST.
	if perm.Card.AST != backAST {
		t.Fatal("disturbed permanent should use back face AST")
	}

	// Should use back face name.
	if perm.Card.Name != "Faithbound Judge" {
		t.Fatalf("expected name 'Faithbound Judge', got %q", perm.Card.Name)
	}

	// Should be flagged as disturbed.
	if perm.Flags["disturbed"] != 1 {
		t.Fatal("disturbed permanent should have 'disturbed' flag")
	}

	// Should have registered a replacement effect.
	foundRepl := false
	for _, repl := range gs.Replacements {
		if repl.SourcePerm == perm && repl.EventType == "would_change_zone" {
			foundRepl = true
			break
		}
	}
	if !foundRepl {
		t.Fatal("disturb should register a would_change_zone replacement effect")
	}
}

// ---------------------------------------------------------------------------
// 7. Bushido tests — CR §702.45
// ---------------------------------------------------------------------------

func TestApplyBushido(t *testing.T) {
	gs := newP1P2Game(t)
	perm := addP1P2Battlefield(gs, 0, "Konda's Hatamoto", 2, 2, "creature")

	ApplyBushido(gs, perm, 2)

	if len(perm.Modifications) != 1 {
		t.Fatalf("expected 1 modification, got %d", len(perm.Modifications))
	}
	mod := perm.Modifications[0]
	if mod.Power != 2 || mod.Toughness != 2 {
		t.Fatalf("expected +2/+2, got +%d/+%d", mod.Power, mod.Toughness)
	}
	if mod.Duration != "until_end_of_turn" {
		t.Fatalf("expected duration 'until_end_of_turn', got %q", mod.Duration)
	}
}

func TestGetBushidoN(t *testing.T) {
	gs := newP1P2Game(t)

	// With bushido_n flag.
	p1 := addP1P2Battlefield(gs, 0, "Test Bushido 3", 2, 2, "creature")
	p1.Flags["kw:bushido"] = 1
	p1.Flags["bushido_n"] = 3
	if GetBushidoN(p1) != 3 {
		t.Fatalf("expected bushido 3, got %d", GetBushidoN(p1))
	}

	// With just kw:bushido flag (default 1).
	p2 := addP1P2Battlefield(gs, 0, "Test Bushido Default", 2, 2, "creature")
	p2.Flags["kw:bushido"] = 1
	if GetBushidoN(p2) != 1 {
		t.Fatalf("expected bushido 1 (default), got %d", GetBushidoN(p2))
	}

	// Without bushido.
	p3 := addP1P2Battlefield(gs, 0, "No Bushido", 2, 2, "creature")
	if GetBushidoN(p3) != 0 {
		t.Fatalf("expected bushido 0, got %d", GetBushidoN(p3))
	}
}

func TestFireBushidoTriggers(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Bushido Attacker", 2, 2, "creature")
	atk.Flags["kw:bushido"] = 1
	atk.Flags["bushido_n"] = 2

	blk := addP1P2Battlefield(gs, 1, "Blocker", 3, 3, "creature")

	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk},
	}

	FireBushidoTriggers(gs, []*Permanent{atk}, blockerMap)

	// Attacker should have +2/+2 modification.
	if len(atk.Modifications) != 1 {
		t.Fatalf("expected 1 modification on attacker, got %d", len(atk.Modifications))
	}
	if atk.Modifications[0].Power != 2 || atk.Modifications[0].Toughness != 2 {
		t.Fatal("attacker should get +2/+2 from bushido")
	}

	// Blocker has no bushido, so no modification.
	if len(blk.Modifications) != 0 {
		t.Fatalf("blocker without bushido should have 0 modifications, got %d", len(blk.Modifications))
	}
}

// ---------------------------------------------------------------------------
// 8. Flanking tests — CR §702.25
// ---------------------------------------------------------------------------

func TestApplyFlanking(t *testing.T) {
	gs := newP1P2Game(t)
	blocker := addP1P2Battlefield(gs, 1, "Goblin Piker", 2, 1, "creature")

	ApplyFlanking(gs, blocker)

	if len(blocker.Modifications) != 1 {
		t.Fatalf("expected 1 modification, got %d", len(blocker.Modifications))
	}
	mod := blocker.Modifications[0]
	if mod.Power != -1 || mod.Toughness != -1 {
		t.Fatalf("expected -1/-1, got %d/%d", mod.Power, mod.Toughness)
	}
}

func TestFireFlankingTriggers(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Flanking Attacker", 3, 3, "creature")
	atk.Flags["kw:flanking"] = 1

	// Blocker WITHOUT flanking gets -1/-1.
	blk1 := addP1P2Battlefield(gs, 1, "Normal Blocker", 2, 2, "creature")

	// Blocker WITH flanking is immune.
	blk2 := addP1P2Battlefield(gs, 1, "Flanking Blocker", 2, 2, "creature")
	blk2.Flags["kw:flanking"] = 1

	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk1, blk2},
	}

	FireFlankingTriggers(gs, []*Permanent{atk}, blockerMap)

	// blk1 should get -1/-1.
	if len(blk1.Modifications) != 1 {
		t.Fatalf("normal blocker should have 1 flanking modification, got %d", len(blk1.Modifications))
	}

	// blk2 should NOT get -1/-1 (has flanking itself).
	if len(blk2.Modifications) != 0 {
		t.Fatalf("flanking blocker should not get flanking debuff, got %d modifications", len(blk2.Modifications))
	}
}

func TestFlankingTriggers_NoFlankingAttacker(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Normal Attacker", 3, 3, "creature")
	blk := addP1P2Battlefield(gs, 1, "Normal Blocker", 2, 2, "creature")

	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk},
	}

	FireFlankingTriggers(gs, []*Permanent{atk}, blockerMap)

	// No modifications because attacker has no flanking.
	if len(blk.Modifications) != 0 {
		t.Fatalf("blocker should have 0 modifications when attacker has no flanking, got %d", len(blk.Modifications))
	}
}

// ---------------------------------------------------------------------------
// 9. Horsemanship tests — CR §702.30
// ---------------------------------------------------------------------------

func TestCanBlockHorsemanship_BlockerWithout(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Lu Bu", 4, 3, "creature")
	atk.Flags["kw:horsemanship"] = 1

	blk := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	if CanBlockHorsemanship(atk, blk) {
		t.Fatal("creature without horsemanship should NOT be able to block a creature with horsemanship")
	}
}

func TestCanBlockHorsemanship_BlockerWith(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Lu Bu", 4, 3, "creature")
	atk.Flags["kw:horsemanship"] = 1

	blk := addP1P2Battlefield(gs, 1, "Riding Tiger", 3, 3, "creature")
	blk.Flags["kw:horsemanship"] = 1

	if !CanBlockHorsemanship(atk, blk) {
		t.Fatal("creature with horsemanship SHOULD be able to block a creature with horsemanship")
	}
}

func TestCanBlockHorsemanship_AttackerWithout(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	blk := addP1P2Battlefield(gs, 1, "Wall of Stone", 0, 5, "creature")

	if !CanBlockHorsemanship(atk, blk) {
		t.Fatal("when attacker has no horsemanship, any blocker should work")
	}
}

func TestCanBlockP1P2_Integration(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Lu Bu", 4, 3, "creature")
	atk.Flags["kw:horsemanship"] = 1

	blk := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	if CanBlockP1P2(atk, blk) {
		t.Fatal("CanBlockP1P2 should return false for non-horsemanship blocker")
	}
}

func TestHorsemanship_InCanBlockGS(t *testing.T) {
	gs := newP1P2Game(t)

	atk := addP1P2Battlefield(gs, 0, "Lu Bu", 4, 3, "creature")
	atk.Flags["kw:horsemanship"] = 1

	blk := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Should fail via canBlock integration.
	if CanBlockGS(gs, atk, blk) {
		t.Fatal("canBlockGS should reject non-horsemanship blocker vs horsemanship attacker")
	}
}

// ---------------------------------------------------------------------------
// 10. Devoid tests — CR §702.114
// ---------------------------------------------------------------------------

func TestRegisterDevoidEffect(t *testing.T) {
	gs := newP1P2Game(t)

	perm := addP1P2Battlefield(gs, 0, "Eldrazi Skyspawner", 2, 2, "creature")
	perm.Card.Colors = []string{"U"} // Normally blue.
	perm.Flags["kw:devoid"] = 1

	RegisterDevoidEffect(gs, perm)

	// Should have a continuous effect registered.
	if len(gs.ContinuousEffects) == 0 {
		t.Fatal("devoid should register a continuous effect")
	}

	// The effect should target this permanent and set colors to empty.
	found := false
	for _, ce := range gs.ContinuousEffects {
		if ce.SourcePerm == perm && ce.Layer == LayerColor {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("devoid continuous effect should be on layer 5 (color)")
	}
}

func TestHasDevoid(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Eldrazi Skyspawner", 2, 2, "creature")
	p.Flags["kw:devoid"] = 1

	if !HasDevoid(p) {
		t.Fatal("creature with kw:devoid should have devoid")
	}

	p2 := addP1P2Battlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	if HasDevoid(p2) {
		t.Fatal("creature without devoid should not have devoid")
	}
}

func TestCardHasDevoid_WithAST(t *testing.T) {
	card := &Card{
		Name: "Eldrazi Skyspawner",
		AST: &gameast.CardAST{
			Name: "Eldrazi Skyspawner",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "devoid"},
			},
		},
	}
	if !CardHasDevoid(card) {
		t.Fatal("card with devoid keyword in AST should report CardHasDevoid")
	}

	card2 := &Card{
		Name: "Grizzly Bears",
		AST: &gameast.CardAST{
			Name:      "Grizzly Bears",
			Abilities: []gameast.Ability{},
		},
	}
	if CardHasDevoid(card2) {
		t.Fatal("card without devoid keyword should not report CardHasDevoid")
	}
}

func TestGetDevoidColors(t *testing.T) {
	card := &Card{
		Name:   "Eldrazi Skyspawner",
		Colors: []string{"U"},
		AST: &gameast.CardAST{
			Name: "Eldrazi Skyspawner",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "devoid"},
			},
		},
	}
	colors := GetDevoidColors(card)
	if len(colors) != 0 {
		t.Fatalf("devoid card should have no colors, got %v", colors)
	}

	card2 := &Card{
		Name:   "Grizzly Bears",
		Colors: []string{"G"},
		AST: &gameast.CardAST{
			Name:      "Grizzly Bears",
			Abilities: []gameast.Ability{},
		},
	}
	colors2 := GetDevoidColors(card2)
	if len(colors2) != 1 || colors2[0] != "G" {
		t.Fatalf("non-devoid card should keep colors, got %v", colors2)
	}
}

// ---------------------------------------------------------------------------
// 11. Shroud tests — CR §702.18
// ---------------------------------------------------------------------------

func TestHasShroud(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Blastoderm", 5, 5, "creature")
	p.Flags["kw:shroud"] = 1

	if !HasShroud(p) {
		t.Fatal("creature with kw:shroud should have shroud")
	}
}

func TestCanBeTargetedBy_Shroud(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Blastoderm", 5, 5, "creature")
	p.Flags["kw:shroud"] = 1

	// Shroud prevents targeting by ANYONE, including controller.
	if CanBeTargetedBy(p, 0) {
		t.Fatal("shroud should prevent targeting by controller")
	}
	if CanBeTargetedBy(p, 1) {
		t.Fatal("shroud should prevent targeting by opponent")
	}
}

func TestCanBeTargetedBy_Hexproof(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Thrun the Last Troll", 4, 4, "creature")
	p.Flags["kw:hexproof"] = 1

	// Hexproof allows controller targeting but blocks opponents.
	if !CanBeTargetedBy(p, 0) {
		t.Fatal("hexproof should allow targeting by controller")
	}
	if CanBeTargetedBy(p, 1) {
		t.Fatal("hexproof should prevent targeting by opponent")
	}
}

func TestCanBeTargetedBy_Normal(t *testing.T) {
	gs := newP1P2Game(t)
	p := addP1P2Battlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	if !CanBeTargetedBy(p, 0) {
		t.Fatal("normal creature should be targetable by controller")
	}
	if !CanBeTargetedBy(p, 1) {
		t.Fatal("normal creature should be targetable by opponent")
	}
}

// ---------------------------------------------------------------------------
// Combat integration tests
// ---------------------------------------------------------------------------

func TestCheckCombatKeywordsP1P2_Combined(t *testing.T) {
	gs := newP1P2Game(t)

	// Attacker with bushido 2 and flanking.
	atk := addP1P2Battlefield(gs, 0, "Samurai Flanker", 2, 2, "creature")
	atk.Flags["kw:bushido"] = 1
	atk.Flags["bushido_n"] = 2
	atk.Flags["kw:flanking"] = 1

	// Blocker without flanking.
	blk := addP1P2Battlefield(gs, 1, "Hapless Recruit", 1, 1, "creature")

	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk},
	}

	CheckCombatKeywordsP1P2(gs, []*Permanent{atk}, blockerMap)

	// Attacker should have bushido +2/+2.
	if len(atk.Modifications) != 1 || atk.Modifications[0].Power != 2 {
		t.Fatal("attacker should get +2/+2 bushido buff")
	}

	// Blocker should have -1/-1 from flanking.
	if len(blk.Modifications) != 1 || blk.Modifications[0].Power != -1 {
		t.Fatal("blocker should get -1/-1 flanking debuff")
	}
}

// ---------------------------------------------------------------------------
// P1P2 keyword constants tests
// ---------------------------------------------------------------------------

func TestAllP1P2Keywords(t *testing.T) {
	keywords := AllP1P2Keywords()
	if len(keywords) != 11 {
		t.Fatalf("expected 11 P1P2 keywords, got %d", len(keywords))
	}

	// Verify all expected keywords are present.
	expected := map[string]bool{
		"unearth": false, "foretell": false, "entwine": false,
		"buyback": false, "wither": false, "disturb": false,
		"bushido": false, "flanking": false, "horsemanship": false,
		"devoid": false, "shroud": false,
	}
	for _, kw := range keywords {
		if _, ok := expected[kw]; !ok {
			t.Fatalf("unexpected keyword %q in AllP1P2Keywords", kw)
		}
		expected[kw] = true
	}
	for kw, found := range expected {
		if !found {
			t.Fatalf("missing keyword %q from AllP1P2Keywords", kw)
		}
	}
}

func TestHasP1P2Keyword(t *testing.T) {
	gs := newP1P2Game(t)

	p := addP1P2Battlefield(gs, 0, "Test P1P2", 2, 2, "creature")
	p.Flags["kw:bushido"] = 1

	if !HasP1P2Keyword(p) {
		t.Fatal("creature with bushido should return true from HasP1P2Keyword")
	}

	p2 := addP1P2Battlefield(gs, 0, "Vanilla", 2, 2, "creature")
	if HasP1P2Keyword(p2) {
		t.Fatal("vanilla creature should return false from HasP1P2Keyword")
	}
}

// ---------------------------------------------------------------------------
// Wither combat integration test
// ---------------------------------------------------------------------------

func TestWither_CombatDamageToCreature(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0

	// Attacker with wither.
	atk := addP1P2Battlefield(gs, 0, "Boggart Ram-Gang", 3, 3, "creature")
	atk.Flags["kw:wither"] = 1

	// Blocker.
	blk := addP1P2Battlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk},
	}

	// Deal combat damage.
	DealCombatDamageStep(gs, []*Permanent{atk}, blockerMap, false)

	// Combat damage assignment: attacker has 3 power, blocker has 2 toughness.
	// §510.1c: lethal amount = toughness - marked_damage = 2 - 0 = 2.
	// Attacker assigns 2 (lethal) to blocker; remaining 1 is unassigned
	// (no trample). Blocker should have 2 -1/-1 counters from wither.
	if blk.Counters["-1/-1"] != 2 {
		t.Fatalf("blocker should have 2 -1/-1 counters from wither (lethal amount), got %d", blk.Counters["-1/-1"])
	}
	// Wither damage should NOT add to MarkedDamage.
	if blk.MarkedDamage != 0 {
		t.Fatalf("wither damage should not mark damage, got %d", blk.MarkedDamage)
	}
}

func TestWither_CombatDamageToPlayer_Normal(t *testing.T) {
	gs := newP1P2Game(t)
	gs.Active = 0

	// Attacker with wither — damage to players is NORMAL (not counters).
	atk := addP1P2Battlefield(gs, 0, "Boggart Ram-Gang", 3, 3, "creature")
	atk.Flags["kw:wither"] = 1
	atk.Flags["attacking"] = 1
	setAttackerDefender(atk, 1)

	blockerMap := map[*Permanent][]*Permanent{
		atk: {}, // Unblocked.
	}

	initialLife := gs.Seats[1].Life

	DealCombatDamageStep(gs, []*Permanent{atk}, blockerMap, false)

	// Player should take normal damage (life loss), not poison.
	if gs.Seats[1].Life != initialLife-3 {
		t.Fatalf("expected life %d, got %d (wither should deal normal damage to players)",
			initialLife-3, gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// Nil-safety tests
// ---------------------------------------------------------------------------

func TestNilSafety_AllFunctions(t *testing.T) {
	// These should all handle nils gracefully without panicking.
	ApplyUnearth(nil, 0, nil, 0)
	ForetellExile(nil, 0, nil)
	CanPayEntwine(nil, 0, 0, 0)
	PayEntwineCost(nil, 0, nil, 0)
	ShouldEntwine(nil, 0, 0, 0)
	ApplyWitherDamageToCreature(nil, nil, nil, 0)
	HasWither(nil)
	HasDevoid(nil)
	HasShroud(nil)
	HasHexproof(nil)
	CanBeTargetedBy(nil, 0)
	ApplyBushido(nil, nil, 0)
	GetBushidoN(nil)
	ApplyFlanking(nil, nil)
	CanBlockHorsemanship(nil, nil)
	CanBlockP1P2(nil, nil)
	ShouldApplyWitherDamage(nil, nil)
	CardHasDevoid(nil)
	GetDevoidColors(nil)
	HasP1P2Keyword(nil)
	ShouldReturnToHandOnResolve(nil)
	ShouldExileOnResolve(nil)
	CheckCombatKeywordsP1P2(nil, nil, nil)
	FireBushidoTriggers(nil, nil, nil)
	FireFlankingTriggers(nil, nil, nil)
}
