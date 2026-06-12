package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// r61 PR-5 — cast-time cost-mechanic family wired into CastSpell.
//
// Pins the full cast pipeline for each newly-wired mechanic: Hat hook
// (ChooseOptionalCost) → cost payment → CostMeta stamp / copy push →
// resolution-time consumer (buyback return-to-hand, overload fan-out,
// replicate copies, surge/spectacle alt-cost, conspire/casualty copies,
// bargain flag). Each mechanic gets a "pay" and a "decline" case so the
// baseline (hat returns 0) is provably unchanged.
// -----------------------------------------------------------------------------

// payHat pays every optional cost to the max — exercises the "yes" branch of
// every mechanic. Embeds GreedyHatStub so it inherits every other Hat method.
type payHat struct{ GreedyHatStub }

func (*payHat) ChooseOptionalCost(gs *GameState, seatIdx int, card *Card, kind string, cost, max int) int {
	return max
}

// declineHat declines every optional cost (mirrors the conservative default).
type declineHat struct{ GreedyHatStub }

func (*declineHat) ChooseOptionalCost(gs *GameState, seatIdx int, card *Card, kind string, cost, max int) int {
	return 0
}

// instantSpellCard builds an instant with no resolution effect plus the given
// keyword AST node(s). Base mana cost via the cost:N test convention.
func instantSpellCard(name string, baseCost int, colors []string, kws ...gameast.Ability) *Card {
	return &Card{
		Name:   name,
		Owner:  0,
		Colors: colors,
		Types:  []string{"instant", "cost:" + itoa(baseCost)},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: kws,
		},
	}
}

func stackHasCopies(gs *GameState, name string) int {
	n := 0
	for _, it := range gs.Stack {
		if it != nil && it.IsCopy && it.Card != nil && it.Card.Name == name {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Buyback (CR §702.27) — additional mana cost → return to hand on resolve.
// ---------------------------------------------------------------------------

func TestBuyback_Paid_ReturnsToHand(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 5 // 2 base + 3 buyback
	card := instantSpellCard("Whispers", 2, nil,
		&gameast.Keyword{Name: "buyback", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// Spell resolved (DrainStack) — buyback should route it back to hand.
	if len(gs.Stack) != 0 {
		t.Fatalf("stack should be empty after resolve, got %d", len(gs.Stack))
	}
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			inHand = true
		}
	}
	if !inHand {
		t.Errorf("bought-back spell should return to hand; hand=%d graveyard=%d",
			len(gs.Seats[0].Hand), len(gs.Seats[0].Graveyard))
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 5 mana spent (2 base + 3 buyback), pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestBuyback_Declined_GoesToGraveyard(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 5 // could afford buyback, but hat declines
	card := instantSpellCard("Whispers", 2, nil,
		&gameast.Keyword{Name: "buyback", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("declined-buyback spell must NOT return to hand")
		}
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected only 2 base mana spent, pool=%d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Overload (CR §702.96) — alternative cost → single-target fans out to "each".
// ---------------------------------------------------------------------------

// End-to-end: an overloaded damage spell cast through CastSpell fans out to
// every opponent creature on resolution. We attach the damage effect via the
// card's AST so collectSpellEffect picks it up.
func TestOverload_CastSpell_EndToEnd_DamageFansOut(t *testing.T) {
	gs := newKWCombatGame4P(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 7
	a := addKWCombatBattlefield(gs, 1, "A", 1, 2, "creature")
	b := addKWCombatBattlefield(gs, 2, "B", 1, 2, "creature")
	c := addKWCombatBattlefield(gs, 3, "C", 1, 2, "creature")

	// Activated-with-empty-cost wrapping the damage = the spell-body shape
	// collectSpellEffect recognizes for instants.
	dmg := &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
		Target: gameast.Filter{Base: "creature", Targeted: true},
	}
	card := &Card{
		Name:  "Electrickery",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
		AST: &gameast.CardAST{
			Name: "Electrickery",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "overload", Args: []interface{}{float64(5)}},
				&gameast.Activated{Effect: dmg},
			},
		},
	}
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	for _, p := range []*Permanent{a, b, c} {
		if p.MarkedDamage != 1 {
			t.Errorf("%s should take 1 damage under overload, got %d", p.Card.Name, p.MarkedDamage)
		}
	}
	if IsOverloadActive(gs) {
		t.Error("overload flag should be cleared after resolution")
	}
}

func TestOverload_CastSpell_Declined_SingleTarget(t *testing.T) {
	gs := newKWCombatGame4P(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 7
	a := addKWCombatBattlefield(gs, 1, "A", 1, 2, "creature")
	b := addKWCombatBattlefield(gs, 2, "B", 1, 2, "creature")
	c := addKWCombatBattlefield(gs, 3, "C", 1, 2, "creature")

	dmg := &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
		Target: gameast.Filter{Base: "creature", Targeted: true},
	}
	card := &Card{
		Name:  "Electrickery",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
		AST: &gameast.CardAST{
			Name: "Electrickery",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "overload", Args: []interface{}{float64(5)}},
				&gameast.Activated{Effect: dmg},
			},
		},
	}
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 6 {
		t.Errorf("declined overload should pay only the 1 base cost, pool=%d", gs.Seats[0].ManaPool)
	}
	total := a.MarkedDamage + b.MarkedDamage + c.MarkedDamage
	if total != 1 {
		t.Errorf("non-overloaded damage should hit exactly one creature, total=%d", total)
	}
}

// ---------------------------------------------------------------------------
// Replicate (CR §702.56) — additional mana cost N times → N copies.
// ---------------------------------------------------------------------------

func TestReplicate_Paid_MakesNCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 8 // 2 base + 3 replicate × 2 copies = 8
	card := instantSpellCard("Pyromatics", 2, nil,
		&gameast.Keyword{Name: "replicate", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}

	// Snapshot copies present on the stack DURING resolution is hard post-drain;
	// instead pre-push so we can count copies before DrainStack runs. Use the
	// direct ApplyReplicate-via-CastSpell path and assert the replicate_pay/
	// replicate_copy events fired and mana was charged.
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// 2 base + 2 copies × 3 = 8 → pool 0.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected all 8 mana spent (2 base + 2×3 replicate), pool=%d", gs.Seats[0].ManaPool)
	}
	copies := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "replicate_copy" {
			copies++
		}
	}
	if copies != 2 {
		t.Errorf("expected 2 replicate copies, got %d", copies)
	}
}

func TestReplicate_Declined_NoCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 8
	card := instantSpellCard("Pyromatics", 2, nil,
		&gameast.Keyword{Name: "replicate", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 6 {
		t.Errorf("declined replicate should pay only 2 base, pool=%d", gs.Seats[0].ManaPool)
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "replicate_copy" {
			t.Fatal("declined replicate must not create copies")
		}
	}
}

// ---------------------------------------------------------------------------
// Surge (CR §702.117) — alternative cost, legal only after another cast.
// ---------------------------------------------------------------------------

func TestSurge_Active_PaysAltCost(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Turn.SpellsCast = 1 // a prior spell this turn → surge active
	card := instantSpellCard("Brutal Expulsion", 6, nil,
		&gameast.Keyword{Name: "surge", Args: []interface{}{float64(4)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// Surge alt cost 4 paid instead of base 6 → pool 6.
	if gs.Seats[0].ManaPool != 6 {
		t.Errorf("surge should pay the 4 alt cost (not 6 base), pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestSurge_Inactive_PaysBaseCost(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Turn.SpellsCast = 0 // no prior spell → surge not available
	card := instantSpellCard("Brutal Expulsion", 6, nil,
		&gameast.Keyword{Name: "surge", Args: []interface{}{float64(4)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("surge-inactive should pay 6 base cost, pool=%d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Spectacle (CR §702.107) — alternative cost, legal only if an opp lost life.
// ---------------------------------------------------------------------------

func TestSpectacle_Active_PaysAltCost(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10
	gs.Seats[1].Turn.LifeLost = 2 // opponent lost life → spectacle active
	card := instantSpellCard("Light Up the Stage", 3, nil,
		&gameast.Keyword{Name: "spectacle", Args: []interface{}{float64(1)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 9 {
		t.Errorf("spectacle should pay the 1 alt cost (not 3 base), pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestSpectacle_Inactive_PaysBaseCost(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10
	gs.Seats[1].Turn.LifeLost = 0 // no opp life loss → spectacle unavailable
	card := instantSpellCard("Light Up the Stage", 3, nil,
		&gameast.Keyword{Name: "spectacle", Args: []interface{}{float64(1)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 7 {
		t.Errorf("spectacle-inactive should pay 3 base cost, pool=%d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Conspire (CR §702.78) — tap two color-sharing creatures → copy.
// ---------------------------------------------------------------------------

func TestConspire_Paid_TapsTwoAndCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 3
	c1 := addKWCombatBattlefield(gs, 0, "Goblin 1", 2, 2, "creature")
	c1.Card.Colors = []string{"R"}
	c2 := addKWCombatBattlefield(gs, 0, "Goblin 2", 2, 2, "creature")
	c2.Card.Colors = []string{"R"}

	card := instantSpellCard("Lightning Bolt", 3, []string{"R"},
		&gameast.Keyword{Name: "conspire"})
	gs.Seats[0].Hand = []*Card{card}

	// Prevent DrainStack from immediately resolving by checking copy events.
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if !c1.Tapped || !c2.Tapped {
		t.Errorf("conspire should tap two creatures, c1.Tapped=%v c2.Tapped=%v", c1.Tapped, c2.Tapped)
	}
	conspired := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "conspire" {
			conspired = true
		}
	}
	if !conspired {
		t.Error("conspire copy event should have fired")
	}
}

func TestConspire_Declined_NoTapNoCopy(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 3
	c1 := addKWCombatBattlefield(gs, 0, "Goblin 1", 2, 2, "creature")
	c1.Card.Colors = []string{"R"}
	c2 := addKWCombatBattlefield(gs, 0, "Goblin 2", 2, 2, "creature")
	c2.Card.Colors = []string{"R"}

	card := instantSpellCard("Lightning Bolt", 3, []string{"R"},
		&gameast.Keyword{Name: "conspire"})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if c1.Tapped || c2.Tapped {
		t.Error("declined conspire must not tap creatures")
	}
}

// ---------------------------------------------------------------------------
// Casualty (CR §702.153) — sacrifice a creature of power ≥ N → copy.
// ---------------------------------------------------------------------------

func TestCasualty_Paid_SacrificesAndCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 2
	victim := addKWCombatBattlefield(gs, 0, "Big Creature", 3, 3, "creature")

	card := instantSpellCard("Bortuk Bonerattle", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(2)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// Victim should be sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("casualty should sacrifice the chosen creature")
		}
	}
	copied := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "casualty_copy" {
			copied = true
		}
	}
	if !copied {
		t.Error("casualty should copy the spell")
	}
}

func TestCasualty_Declined_NoSacrifice(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 2
	victim := addKWCombatBattlefield(gs, 0, "Big Creature", 3, 3, "creature")

	card := instantSpellCard("Bortuk Bonerattle", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(2)}})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			found = true
		}
	}
	if !found {
		t.Error("declined casualty must not sacrifice")
	}
}

// ---------------------------------------------------------------------------
// Bargain (CR §702.176) — sacrifice an artifact/enchantment/token → flag.
// ---------------------------------------------------------------------------

func TestBargain_Paid_SacrificesAndStampsFlag(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 3
	art := addKWCombatBattlefield(gs, 0, "Treasure", 0, 0, "artifact")

	card := instantSpellCard("Torch the Tower", 3, nil,
		&gameast.Keyword{Name: "bargain"})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == art {
			t.Error("bargain should sacrifice the artifact")
		}
	}
	paid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "bargain_paid" {
			paid = true
		}
	}
	if !paid {
		t.Error("bargain_paid event should have fired")
	}
}

func TestBargain_Declined_NoSacrifice(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 3
	art := addKWCombatBattlefield(gs, 0, "Treasure", 0, 0, "artifact")

	card := instantSpellCard("Torch the Tower", 3, nil,
		&gameast.Keyword{Name: "bargain"})
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == art {
			found = true
		}
	}
	if !found {
		t.Error("declined bargain must not sacrifice")
	}
}

// ---------------------------------------------------------------------------
// Helper-level pins.
// ---------------------------------------------------------------------------

func TestCostFamily_HelperDetectors(t *testing.T) {
	conspire := instantSpellCard("X", 1, []string{"R"}, &gameast.Keyword{Name: "conspire"})
	if !HasConspire(conspire) {
		t.Error("HasConspire should detect the conspire keyword")
	}
	plain := instantSpellCard("Y", 1, nil)
	if HasConspire(plain) {
		t.Error("HasConspire false positive on plain card")
	}
}

func TestConspireEligibleCount(t *testing.T) {
	gs := newKWCombatGame(t)
	card := instantSpellCard("X", 1, []string{"R"}, &gameast.Keyword{Name: "conspire"})
	if conspireEligibleCount(gs, 0, card) != 0 {
		t.Error("no creatures → 0 eligible")
	}
	c1 := addKWCombatBattlefield(gs, 0, "G1", 1, 1, "creature")
	c1.Card.Colors = []string{"R"}
	if conspireEligibleCount(gs, 0, card) != 1 {
		t.Error("one color-sharing creature → 1 eligible")
	}
	c2 := addKWCombatBattlefield(gs, 0, "G2", 1, 1, "creature")
	c2.Card.Colors = []string{"R"}
	if conspireEligibleCount(gs, 0, card) < 2 {
		t.Error("two color-sharing creatures → ≥2 eligible")
	}
	// Tapped creatures don't count.
	c1.Tapped = true
	c2.Tapped = true
	if conspireEligibleCount(gs, 0, card) != 0 {
		t.Error("tapped creatures must not count as eligible")
	}
}
