package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Disguise (CR §702.166)
// ---------------------------------------------------------------------------

func newDisguiseGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(13))
	return NewGameState(2, rng, nil)
}

// disguiseHandCard builds a card with the disguise keyword printed at the
// given activation cost and seats it in `seat`'s hand. Printed stats /
// types follow a sensible "small monster" template so that, once flipped
// face-up, the layers system has something to read.
func disguiseHandCard(gs *GameState, seat int, name string, disguiseCost int) *Card {
	c := &Card{
		Name:          name,
		Owner:         seat,
		CMC:           5,
		BasePower:     4,
		BaseToughness: 4,
		Types:         []string{"creature", "human", "rogue"},
		Colors:        []string{"U"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "disguise", Args: []any{float64(disguiseCost)}},
				&gameast.Keyword{Name: "flying"},
			},
		},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// ===========================================================================
// HasDisguise / DisguiseCost
// ===========================================================================

func TestHasDisguise_Detects(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "disguise", Args: []any{float64(4)}},
			},
		},
	}
	if !HasDisguise(c) {
		t.Fatal("HasDisguise returned false for a card with the keyword")
	}
	if HasDisguise(&Card{AST: &gameast.CardAST{}}) {
		t.Fatal("HasDisguise returned true for a card without the keyword")
	}
	if HasDisguise(nil) {
		t.Fatal("HasDisguise(nil) should be false")
	}
}

func TestDisguiseCost_ParsesArg(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "disguise", Args: []any{float64(4)}},
			},
		},
	}
	if got := DisguiseCost(c); got != 4 {
		t.Fatalf("DisguiseCost: want 4, got %d", got)
	}
}

// ===========================================================================
// (a) Cast face-down spawns a 2/2 ward {2}
// ===========================================================================

func TestCastDisguiseFaceDown_Spawns2x2WardTwo(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost
	card := disguiseHandCard(gs, 0, "Hooded Conspirator", 4)

	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown error: %v", err)
	}
	if perm == nil {
		t.Fatal("CastDisguiseFaceDown returned nil permanent")
	}

	// Mana was paid.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana pool=0 after paying {3}, got %d", gs.Seats[0].ManaPool)
	}

	// Card removed from hand, placed on battlefield, marked face-down.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("card should be removed from hand, hand=%d", len(gs.Seats[0].Hand))
	}
	if !IsFaceDown(perm) {
		t.Error("permanent should be face-down")
	}
	if !card.FaceDown {
		t.Error("card.FaceDown should be true")
	}
	if perm.Flags["disguise_face_down"] != 1 {
		t.Error("permanent should be tagged with disguise_face_down=1")
	}

	// 2/2 via the §707.2 face-down characteristic override (layers.go
	// BaseCharacteristics).
	chars := BaseCharacteristics(perm)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("face-down disguise want 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	if !chars.FaceDown {
		t.Error("characteristics should reflect FaceDown=true")
	}

	// Ward {2}: HasKeyword path + ward_cost flag.
	if !perm.HasKeyword("ward") {
		t.Error("face-down disguise creature should HasKeyword('ward')")
	}
	if perm.Flags["ward_cost"] != DisguiseFaceDownWardCost {
		t.Errorf("ward_cost: want %d, got %d", DisguiseFaceDownWardCost, perm.Flags["ward_cost"])
	}

	// Stack was cleared after the in-place resolve — no leaked items.
	if len(gs.Stack) != 0 {
		t.Errorf("stack should be empty post-cast, got %d item(s)", len(gs.Stack))
	}
}

func TestCastDisguiseFaceDown_StampsCostMetaAltCost(t *testing.T) {
	// We can observe CostMeta by snapshotting the stack at push time —
	// CastDisguiseFaceDown pops the item before returning. Use a small
	// hook: peek by recording the event log entries that surface the
	// alt-cost trail (cast_face_down with alt_cost detail).
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost
	card := disguiseHandCard(gs, 0, "Skulking Killer", 4)

	if _, err := CastDisguiseFaceDown(gs, 0, card); err != nil {
		t.Fatalf("CastDisguiseFaceDown error: %v", err)
	}

	saw := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "cast_face_down" {
			continue
		}
		if v, _ := ev.Details["alt_cost"].(string); v == "disguise" {
			if rule, _ := ev.Details["rule"].(string); rule == "702.166a" {
				saw = true
				break
			}
		}
	}
	if !saw {
		t.Fatal("expected a cast_face_down event with alt_cost=disguise and rule=702.166a")
	}
}

func TestCastDisguiseFaceDown_RejectsCardWithoutKeyword(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 9
	c := &Card{
		Name:  "Plain Bear",
		Owner: 0,
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: "Plain Bear"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)

	perm, err := CastDisguiseFaceDown(gs, 0, c)
	if err == nil || perm != nil {
		t.Fatal("CastDisguiseFaceDown should reject a card without the disguise keyword")
	}
	if gs.Seats[0].ManaPool != 9 {
		t.Errorf("mana pool should be untouched on rejection, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastDisguiseFaceDown_RejectsInsufficientMana(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost - 1
	card := disguiseHandCard(gs, 0, "Underfunded", 3)

	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err == nil || perm != nil {
		t.Fatal("CastDisguiseFaceDown should fail when mana < {3}")
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("card should remain in hand on failure, hand=%d", len(gs.Seats[0].Hand))
	}
}

// ===========================================================================
// (b) TurnFaceUpDisguise pays the cost + reveals stats/abilities
// ===========================================================================

func TestTurnFaceUpDisguise_PaysCostAndRevealsCharacteristics(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost + 4 // {3} cast + {4} flip
	card := disguiseHandCard(gs, 0, "Hooded Conspirator", 4)

	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown error: %v", err)
	}
	// Mana left over: 4 for the flip.
	if gs.Seats[0].ManaPool != 4 {
		t.Fatalf("setup: expected 4 mana left after cast, got %d", gs.Seats[0].ManaPool)
	}

	// Pre-flip: characteristics are 2/2 with ward.
	pre := BaseCharacteristics(perm)
	if pre.Power != 2 || pre.Toughness != 2 || !pre.FaceDown {
		t.Fatalf("pre-flip want 2/2 face-down, got %d/%d face_down=%v", pre.Power, pre.Toughness, pre.FaceDown)
	}

	if !TurnFaceUpDisguise(gs, perm, 4) {
		t.Fatal("TurnFaceUpDisguise should succeed when cost is payable")
	}

	// Mana paid.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana pool=0 after paying disguise cost, got %d", gs.Seats[0].ManaPool)
	}

	// Card is face-up; permanent characteristics now reflect the printed
	// 4/4 flying.
	if perm.Card.FaceDown {
		t.Error("perm.Card.FaceDown should be cleared after flip")
	}
	if IsFaceDown(perm) {
		t.Error("IsFaceDown should be false after flip")
	}
	post := BaseCharacteristics(perm)
	if post.Power != 4 || post.Toughness != 4 {
		t.Errorf("post-flip want 4/4, got %d/%d", post.Power, post.Toughness)
	}
	if !perm.HasKeyword("flying") {
		t.Error("post-flip should reveal printed flying keyword")
	}

	// Disguise-granted ward {2} stripped.
	if perm.Flags["kw:ward"] == 1 {
		t.Error("disguise-granted ward flag should be cleared on flip")
	}
	if perm.Flags["ward_cost"] != 0 {
		t.Error("ward_cost flag should be cleared on flip")
	}
	if perm.Flags["disguise_face_down"] == 1 {
		t.Error("disguise_face_down flag should be cleared on flip")
	}
	if perm.Flags["face_down"] == 1 {
		t.Error("face_down flag should be cleared on flip")
	}

	// Event trail.
	gotEvent := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "disguise_turn_face_up" && ev.Amount == 4 {
			if rule, _ := ev.Details["rule"].(string); rule == "702.166b" {
				gotEvent = true
				break
			}
		}
	}
	if !gotEvent {
		t.Error("expected a disguise_turn_face_up event with rule 702.166b and amount 4")
	}
}

func TestTurnFaceUpDisguise_RejectsInsufficientMana(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost + 1 // can cast, but only {1} left
	card := disguiseHandCard(gs, 0, "Hooded Conspirator", 4)
	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("cast: %v", err)
	}

	if TurnFaceUpDisguise(gs, perm, 4) {
		t.Fatal("TurnFaceUpDisguise should fail when mana < disguise cost")
	}
	if !IsFaceDown(perm) {
		t.Error("permanent should still be face-down after failed flip")
	}
	if !perm.HasKeyword("ward") {
		t.Error("ward {2} should still be present after failed flip")
	}
}

func TestTurnFaceUpDisguise_RejectsNonDisguisePermanent(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	// Build a face-down MORPH permanent (no disguise marker).
	morphCard := &Card{
		Name: "Generic Morph", Owner: 0,
		BasePower: 3, BaseToughness: 3,
		Types:    []string{"creature"},
		FaceDown: true,
		AST: &gameast.CardAST{
			Name: "Generic Morph",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "morph", Args: []any{float64(2)}},
			},
		},
	}
	perm := &Permanent{
		Card: morphCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags: map[string]int{
			"face_down":      1,
			"morph_creature": 1,
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	if TurnFaceUpDisguise(gs, perm, 4) {
		t.Fatal("TurnFaceUpDisguise should reject a plain morph face-down permanent")
	}
	if !morphCard.FaceDown {
		t.Error("morph card should remain face-down after rejected disguise flip")
	}
}

func TestTurnFaceUpDisguise_NilSafe(t *testing.T) {
	if TurnFaceUpDisguise(nil, nil, 0) {
		t.Fatal("TurnFaceUpDisguise(nil, nil) should be false")
	}
	gs := newDisguiseGame(t)
	if TurnFaceUpDisguise(gs, nil, 0) {
		t.Fatal("TurnFaceUpDisguise(gs, nil) should be false")
	}
}

// ===========================================================================
// (c) Ward {2} triggers when targeted while face-down
// ===========================================================================

func TestDisguiseFaceDown_WardTwoCountersWhenCannotPay(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost
	card := disguiseHandCard(gs, 0, "Hooded Killer", 4)

	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown: %v", err)
	}

	// Opponent (seat 1) tries to target the face-down disguise with a spell
	// they only have {1} for — can't pay the ward {2}, so per
	// §702.21a (Ward) the spell is countered.
	gs.Seats[1].ManaPool = 1
	attackerSpell := &Card{Name: "Doom Blade", Owner: 1, Types: []string{"instant"}}
	item := &StackItem{
		Kind:       "spell",
		Controller: 1,
		Card:       attackerSpell,
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: perm},
		},
	}

	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("spell targeting a face-down disguise creature should be countered when caster can't pay ward {2}")
	}
	// Ward fired — verify the counter event references the disguise creature.
	sawCounter := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ward_counter" && ev.Amount == DisguiseFaceDownWardCost {
			sawCounter = true
			break
		}
	}
	if !sawCounter {
		t.Errorf("expected a ward_counter event with amount=%d", DisguiseFaceDownWardCost)
	}
	// Caster still has their {1} because counter happened instead of payment.
	if gs.Seats[1].ManaPool != 1 {
		t.Errorf("caster mana should be untouched when ward counters, got %d", gs.Seats[1].ManaPool)
	}
}

func TestDisguiseFaceDown_WardTwoPaidWhenCasterCanAfford(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost
	card := disguiseHandCard(gs, 0, "Hooded Killer", 4)

	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown: %v", err)
	}

	// Opponent has {5} — can afford ward {2}.
	gs.Seats[1].ManaPool = 5
	item := &StackItem{
		Kind:       "spell",
		Controller: 1,
		Card:       &Card{Name: "Murder", Owner: 1, Types: []string{"instant"}},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: perm},
		},
	}

	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("spell should NOT be countered when caster pays ward {2}")
	}
	if gs.Seats[1].ManaPool != 3 {
		t.Errorf("ward {2} should have been paid: expected mana=3, got %d", gs.Seats[1].ManaPool)
	}
	sawPaid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ward_paid" && ev.Amount == DisguiseFaceDownWardCost {
			sawPaid = true
			break
		}
	}
	if !sawPaid {
		t.Errorf("expected a ward_paid event with amount=%d", DisguiseFaceDownWardCost)
	}
}

func TestDisguiseFaceUp_WardNoLongerApplies(t *testing.T) {
	// After the disguise creature flips face-up, the §702.168a-granted
	// ward {2} must be gone. The printed AST in disguiseHandCard has no
	// ward, so HasKeyword("ward") must read false post-flip.
	gs := newDisguiseGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = DisguiseFaceDownCost + 4
	card := disguiseHandCard(gs, 0, "Hooded Conspirator", 4)
	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown: %v", err)
	}
	if !TurnFaceUpDisguise(gs, perm, 4) {
		t.Fatal("flip failed unexpectedly")
	}

	if perm.HasKeyword("ward") {
		t.Fatal("ward {2} should be gone after the disguise flips face-up")
	}

	// And an opponent's spell targeting the now-face-up creature with no
	// mana should NOT be countered by ward.
	gs.Seats[1].ManaPool = 0
	item := &StackItem{
		Kind:       "spell",
		Controller: 1,
		Card:       &Card{Name: "Bolt", Owner: 1, Types: []string{"instant"}},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: perm},
		},
	}
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Fatal("ward-from-disguise should not fire once the permanent is face-up")
	}
}
