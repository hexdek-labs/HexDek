package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Shared test helpers — Aetherdrift Mobilize + Harmonize
// ---------------------------------------------------------------------------

func newAetherGame4P(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	return NewGameState(4, rng, nil)
}

func newAetherGame2P(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	return NewGameState(2, rng, nil)
}

// mobilizeAttacker builds a creature with "Mobilize N" on the AST and drops
// it onto seat's battlefield with the given defender pre-recorded.
func mobilizeAttacker(gs *GameState, seat, defender int, n int) *Permanent {
	card := &Card{
		Name:          "Trial Driver",
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Trial Driver",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mobilize", Args: []any{float64(n)}},
			},
		},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{flagAttacking: 1},
	}
	setAttackerDefender(p, defender)
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// harmonizeEnchantment builds a Harmonize enchantment permanent and drops it
// onto seat's battlefield.
func harmonizeEnchantment(gs *GameState, seat int, name string) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"enchantment", "harmonize"},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// ===========================================================================
// §702.181 — Mobilize
// ===========================================================================

func TestHasMobilize_Detects(t *testing.T) {
	card := &Card{
		Name: "Trial Driver",
		AST: &gameast.CardAST{
			Name: "Trial Driver",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mobilize", Args: []any{float64(2)}},
			},
		},
	}
	if !HasMobilize(card) {
		t.Fatal("HasMobilize returned false for a card with the keyword")
	}
}

func TestHasMobilize_NegativeNoKeyword(t *testing.T) {
	card := &Card{
		Name: "Plain Bear",
		AST: &gameast.CardAST{
			Name:      "Plain Bear",
			Abilities: []gameast.Ability{},
		},
	}
	if HasMobilize(card) {
		t.Fatal("HasMobilize returned true for a card without the keyword")
	}
	if HasMobilize(nil) {
		t.Fatal("HasMobilize(nil) should be false")
	}
}

func TestMobilizeCount_ParsesArg(t *testing.T) {
	card := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mobilize", Args: []any{float64(3)}},
			},
		},
	}
	if got := MobilizeCount(card); got != 3 {
		t.Fatalf("MobilizeCount: want 3, got %d", got)
	}
}

func TestMobilizeCount_DefaultsToOneWhenMissing(t *testing.T) {
	// No args printed — §702.181a treats "Mobilize" as Mobilize 1.
	card := &Card{
		CMC: 0, // ensure keywordArgCost fallback to CMC stays at 0 (then bumps to 1).
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "mobilize"},
			},
		},
	}
	if got := MobilizeCount(card); got != 1 {
		t.Fatalf("MobilizeCount default: want 1, got %d", got)
	}
}

func TestApplyMobilize_CreatesNWarriorTokens(t *testing.T) {
	gs := newAetherGame4P(t)
	atk := mobilizeAttacker(gs, 0, 1, 2)

	ApplyMobilize(gs, atk, 0)

	tokenCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			tokenCount++
		}
	}
	if tokenCount != 2 {
		t.Fatalf("expected 2 mobilize tokens, got %d", tokenCount)
	}
}

func TestApplyMobilize_TokensTappedAndAttackingSameDefender(t *testing.T) {
	gs := newAetherGame4P(t)
	atk := mobilizeAttacker(gs, 0, 2, 1) // attacking seat 2

	ApplyMobilize(gs, atk, 0)

	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Flags == nil || p.Flags["mobilize_token"] != 1 {
			continue
		}
		found = true
		if !p.Tapped {
			t.Error("mobilize token should be tapped")
		}
		if p.Flags[flagAttacking] != 1 {
			t.Error("mobilize token should be attacking")
		}
		if def, ok := AttackerDefender(p); !ok || def != 2 {
			t.Errorf("mobilize token should attack seat 2, got def=%d ok=%v", def, ok)
		}
		if p.Card == nil {
			t.Fatal("mobilize token missing Card")
		}
		if p.Card.BasePower != 1 || p.Card.BaseToughness != 1 {
			t.Errorf("token P/T want 1/1, got %d/%d", p.Card.BasePower, p.Card.BaseToughness)
		}
		isWarrior := false
		isToken := false
		isRed := false
		for _, ty := range p.Card.Types {
			if ty == "warrior" {
				isWarrior = true
			}
			if ty == "token" {
				isToken = true
			}
		}
		for _, c := range p.Card.Colors {
			if c == "R" {
				isRed = true
			}
		}
		if !isWarrior {
			t.Error("token should have 'warrior' subtype (CR §702.181a)")
		}
		if !isToken {
			t.Error("token should be flagged as a 'token' type for IsToken()")
		}
		if !isRed {
			t.Error("token should be red (CR §702.181a)")
		}
		// CR §508.1g — mobilize tokens enter attacking, not via
		// declare-attackers, so they carry the entered-attacking carve-out.
		if p.Flags[flagEnteredAttacking] != 1 {
			t.Error("mobilize token should be marked entered-attacking (§508.1g carve-out)")
		}
	}
	if !found {
		t.Fatal("no mobilize token was created")
	}
}

func TestApplyMobilize_TokensSacrificedAtEndOfTurn(t *testing.T) {
	gs := newAetherGame4P(t)
	atk := mobilizeAttacker(gs, 0, 1, 2)

	ApplyMobilize(gs, atk, 0)

	// Tokens are present pre-end-step.
	pre := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			pre++
		}
	}
	if pre != 2 {
		t.Fatalf("expected 2 tokens before end step, got %d", pre)
	}

	// Fire the next end step.
	fired := FireDelayedTriggers(gs, "ending", "end")
	if fired == 0 {
		t.Fatal("expected the mobilize end-of-turn delayed trigger to fire")
	}

	post := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			post++
		}
	}
	if post != 0 {
		t.Fatalf("mobilize tokens should have been sacrificed at end of turn, %d remain", post)
	}

	// CR §702.181a — the tokens are SACRIFICED (not exiled), so a "sacrifice"
	// event fires for each (routing them to the graveyard, where dies/LTB
	// payoffs observe them). Per CR §704.5d the token Card structs then cease
	// to exist as an SBA, so we assert on the events, not the graveyard slice.
	sacEvents := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "sacrifice" {
			continue
		}
		if name, _ := ev.Details["target_card"].(string); name == "Warrior Token" {
			sacEvents++
		}
	}
	if sacEvents != 2 {
		t.Fatalf("expected 2 'sacrifice' events for Warrior Token, got %d", sacEvents)
	}
}

func TestApplyMobilize_LogsEventWithRule(t *testing.T) {
	gs := newAetherGame4P(t)
	atk := mobilizeAttacker(gs, 0, 1, 2)

	ApplyMobilize(gs, atk, 0)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "mobilize" && ev.Amount == 2 {
			if rule, _ := ev.Details["rule"].(string); rule == "702.181" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected a mobilize event with rule 702.181 and amount 2")
	}
}

func TestApplyMobilize_NoKeywordIsNoop(t *testing.T) {
	gs := newAetherGame4P(t)
	// Build an attacker without the keyword.
	card := &Card{
		Name: "Plain Beast", Owner: 0, BasePower: 2, BaseToughness: 2,
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: "Plain Beast"},
	}
	atk := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{flagAttacking: 1},
	}
	setAttackerDefender(atk, 1)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atk)

	before := len(gs.Seats[0].Battlefield)
	ApplyMobilize(gs, atk, 0)
	after := len(gs.Seats[0].Battlefield)
	if before != after {
		t.Fatalf("ApplyMobilize on non-keyword attacker should not change battlefield (before=%d after=%d)", before, after)
	}
}

func TestApplyMobilize_NilSafe(t *testing.T) {
	// Should not panic.
	ApplyMobilize(nil, nil, 0)
	gs := newAetherGame4P(t)
	ApplyMobilize(gs, nil, 0)
}

func TestFireMobilizeTriggers_FiresOnlyForKeywordCarriers(t *testing.T) {
	gs := newAetherGame4P(t)
	carrier := mobilizeAttacker(gs, 0, 1, 1)
	// Sibling attacker without mobilize:
	plain := &Permanent{
		Card: &Card{
			Name: "Plain Attacker", Owner: 0, BasePower: 1, BaseToughness: 1,
			Types: []string{"creature"},
			AST:   &gameast.CardAST{Name: "Plain Attacker"},
		},
		Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{flagAttacking: 1},
	}
	setAttackerDefender(plain, 2)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, plain)

	FireMobilizeTriggers(gs, 0, []*Permanent{carrier, plain})

	tokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			tokens++
		}
	}
	if tokens != 1 {
		t.Fatalf("expected exactly 1 mobilize token (from the one carrier), got %d", tokens)
	}
}

// ===========================================================================
// §702.180 — Harmonize
// ===========================================================================

func TestHasHarmonize_DetectsEnchantmentSubtype(t *testing.T) {
	gs := newAetherGame2P(t)
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")
	if !HasHarmonize(perm) {
		t.Fatal("HasHarmonize returned false for an enchantment with the subtype")
	}
}

func TestHasHarmonize_RequiresEnchantmentType(t *testing.T) {
	gs := newAetherGame2P(t)
	// 'harmonize' tag on a creature should not count.
	card := &Card{
		Name: "Pretender", Owner: 0,
		Types: []string{"creature", "harmonize"},
	}
	p := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	if HasHarmonize(p) {
		t.Fatal("HasHarmonize should require the 'enchantment' card type")
	}
}

func TestHasHarmonizeCard_OffBattlefield(t *testing.T) {
	card := &Card{Types: []string{"enchantment", "harmonize"}}
	if !HasHarmonizeCard(card) {
		t.Fatal("HasHarmonizeCard returned false for a harmonize enchantment card")
	}
	if HasHarmonizeCard(&Card{Types: []string{"enchantment"}}) {
		t.Fatal("HasHarmonizeCard returned true for a plain enchantment")
	}
	if HasHarmonizeCard(nil) {
		t.Fatal("HasHarmonizeCard(nil) should be false")
	}
}

func TestHarmonizeActivate_SearchesAndMovesToHand(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")

	// Stack the library so the first match is obvious.
	target := &Card{Name: "Sought Card", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*Card{
		{Name: "Filler 1", Owner: 0, Types: []string{"sorcery"}},
		target,
		{Name: "Filler 2", Owner: 0, Types: []string{"instant"}},
	}

	ok := HarmonizeActivate(gs, 0, perm, 2, func(c *Card) bool {
		return cardHasType(c, "creature")
	})
	if !ok {
		t.Fatal("HarmonizeActivate should have succeeded")
	}

	// Mana paid.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("mana pool: want 3 after paying 2, got %d", gs.Seats[0].ManaPool)
	}

	// Card now in hand.
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == target {
			inHand = true
		}
	}
	if !inHand {
		t.Error("target card should be in hand after Harmonize")
	}

	// Library no longer contains it.
	for _, c := range gs.Seats[0].Library {
		if c == target {
			t.Error("target should have been removed from library")
		}
	}

	// Event logged with rule.
	logged := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "harmonize_activate" {
			if rule, _ := ev.Details["rule"].(string); rule == "702.180" {
				logged = true
				break
			}
		}
	}
	if !logged {
		t.Error("expected a harmonize_activate event with rule 702.180")
	}
}

func TestHarmonizeActivate_NilFilterPicksFirstCard(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")

	first := &Card{Name: "First", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Library = []*Card{first, {Name: "Second", Owner: 0}}

	if !HarmonizeActivate(gs, 0, perm, 1, nil) {
		t.Fatal("HarmonizeActivate should succeed with nil filter")
	}
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0] != first {
		t.Errorf("nil filter should grab the first library card; hand=%v", gs.Seats[0].Hand)
	}
}

func TestHarmonizeActivate_InsufficientManaFails(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")
	gs.Seats[0].Library = []*Card{{Name: "X", Owner: 0}}

	if HarmonizeActivate(gs, 0, perm, 3, nil) {
		t.Fatal("HarmonizeActivate should fail when mana < cost")
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana should be untouched on failure, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Error("no card should be drawn on failure")
	}
}

func TestHarmonizeActivate_SorcerySpeedGate(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 1 // Not the controller's turn.
	gs.Seats[0].ManaPool = 5
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")
	gs.Seats[0].Library = []*Card{{Name: "X", Owner: 0}}

	if HarmonizeActivate(gs, 0, perm, 1, nil) {
		t.Fatal("HarmonizeActivate should be blocked outside the controller's turn (sorcery speed)")
	}
}

func TestHarmonizeActivate_RejectsNonHarmonizePerm(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3
	// Plain enchantment, no harmonize subtype.
	card := &Card{Name: "Plain Enchant", Owner: 0, Types: []string{"enchantment"}}
	p := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	gs.Seats[0].Library = []*Card{{Name: "X", Owner: 0}}

	if HarmonizeActivate(gs, 0, p, 1, nil) {
		t.Fatal("HarmonizeActivate should reject permanents without the Harmonize subtype")
	}
}

func TestHarmonizeActivate_NoMatchStillPaysAndShuffles(t *testing.T) {
	gs := newAetherGame2P(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 4
	perm := harmonizeEnchantment(gs, 0, "Harmonic Resonator")
	// Library has no creature; filter wants creature.
	gs.Seats[0].Library = []*Card{
		{Name: "Filler", Owner: 0, Types: []string{"sorcery"}},
		{Name: "Filler2", Owner: 0, Types: []string{"instant"}},
	}

	if !HarmonizeActivate(gs, 0, perm, 2, func(c *Card) bool {
		return cardHasType(c, "creature")
	}) {
		t.Fatal("HarmonizeActivate should report success even when nothing was found")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("cost should still be paid; want 2, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should remain empty when no card matched; got %d", len(gs.Seats[0].Hand))
	}
	// Confirm the event records found=<none>.
	gotNone := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "harmonize_activate" {
			if v, _ := ev.Details["found"].(string); v == "<none>" {
				gotNone = true
				break
			}
		}
	}
	if !gotNone {
		t.Error("expected event detail found=<none> when no card matched")
	}
}

func TestHarmonizeActivate_NilSafe(t *testing.T) {
	if HarmonizeActivate(nil, 0, nil, 0, nil) {
		t.Fatal("HarmonizeActivate(nil...) should be false")
	}
	gs := newAetherGame2P(t)
	if HarmonizeActivate(gs, 0, nil, 0, nil) {
		t.Fatal("HarmonizeActivate with nil perm should be false")
	}
}
