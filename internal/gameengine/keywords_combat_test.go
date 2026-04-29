package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ============================================================================
// Test helpers for combat keywords
// ============================================================================

func newKWCombatGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(2, rng, nil)
}

func newKWCombatGame4P(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(4, rng, nil)
}

func addKWCombatBattlefield(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string{}, types...),
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

func addKWCombatBattlefieldWithKeyword(gs *GameState, seat int, name string, pow, tough int, keyword string, types ...string) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, pow, tough, types...)
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: keyword},
		},
	}
	return p
}

func addKWCombatBattlefieldWithKeywords(gs *GameState, seat int, name string, pow, tough int, keywords []string, types ...string) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, pow, tough, types...)
	abilities := make([]gameast.Ability, len(keywords))
	for i, kw := range keywords {
		abilities[i] = &gameast.Keyword{Name: kw}
	}
	p.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: abilities,
	}
	return p
}

func addKWCombatBattlefieldColored(gs *GameState, seat int, name string, pow, tough int, colors []string, types ...string) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, pow, tough, types...)
	p.Card.Colors = colors
	return p
}

func addKWCombatHandCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
		CMC:   cost,
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

func countKWCombatEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// ============================================================================
// COMBAT EVASION TESTS
// ============================================================================

// ---------------------------------------------------------------------------
// Intimidate
// ---------------------------------------------------------------------------

func TestIntimidate_BlockedByArtifact(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Menacing Ogre", 3, 3, "intimidate", "creature")
	atk.Card.Colors = []string{"R"}
	// Artifact creature can block.
	blk := addKWCombatBattlefield(gs, 1, "Steel Wall", 0, 4, "artifact", "creature")
	if !CanBlockIntimidate(atk, blk) {
		t.Error("artifact creature should be able to block intimidate")
	}
}

func TestIntimidate_BlockedBySameColor(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Menacing Ogre", 3, 3, "intimidate", "creature")
	atk.Card.Types = append(atk.Card.Types, "red") // color via type (engine convention)
	blk := addKWCombatBattlefield(gs, 1, "Red Bear", 2, 2, "creature", "red")
	if !CanBlockIntimidate(atk, blk) {
		t.Error("creature sharing a color should be able to block intimidate")
	}
}

func TestIntimidate_CantBeBlockedByDifferentColor(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Menacing Ogre", 3, 3, "intimidate", "creature")
	atk.Card.Types = append(atk.Card.Types, "red")
	blk := addKWCombatBattlefield(gs, 1, "Green Bear", 2, 2, "creature", "green")
	if CanBlockIntimidate(atk, blk) {
		t.Error("creature of a different color should NOT block intimidate")
	}
}

// ---------------------------------------------------------------------------
// Fear
// ---------------------------------------------------------------------------

func TestFear_BlockedByArtifact(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Fear Bearer", 2, 2, "fear", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Iron Golem", 3, 3, "artifact", "creature")
	if !CanBlockFear(atk, blk) {
		t.Error("artifact creature should be able to block fear")
	}
}

func TestFear_BlockedByBlack(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Fear Bearer", 2, 2, "fear", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Dark Ritual", 1, 1, "creature", "black")
	if !CanBlockFear(atk, blk) {
		t.Error("black creature should be able to block fear")
	}
}

func TestFear_CantBeBlockedByNonBlackNonArtifact(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Fear Bearer", 2, 2, "fear", "creature")
	blk := addKWCombatBattlefield(gs, 1, "White Knight", 2, 2, "creature", "white")
	if CanBlockFear(atk, blk) {
		t.Error("non-black, non-artifact creature should NOT block fear")
	}
}

// ---------------------------------------------------------------------------
// Shadow
// ---------------------------------------------------------------------------

func TestShadow_ShadowBlocksShadow(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Shadow Attacker", 2, 2, "shadow", "creature")
	blk := addKWCombatBattlefieldWithKeyword(gs, 1, "Shadow Blocker", 1, 3, "shadow", "creature")
	if !CanBlockShadow(atk, blk) {
		t.Error("shadow creature should be able to block shadow creature")
	}
}

func TestShadow_NonShadowCantBlockShadow(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Shadow Attacker", 2, 2, "shadow", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Normal Blocker", 3, 3, "creature")
	if CanBlockShadow(atk, blk) {
		t.Error("non-shadow creature should NOT block shadow creature")
	}
}

func TestShadow_ShadowCantBlockNonShadow(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefield(gs, 0, "Normal Attacker", 2, 2, "creature")
	blk := addKWCombatBattlefieldWithKeyword(gs, 1, "Shadow Blocker", 1, 3, "shadow", "creature")
	if CanBlockShadow(atk, blk) {
		t.Error("shadow creature should NOT block non-shadow creature")
	}
}

// ---------------------------------------------------------------------------
// Skulk
// ---------------------------------------------------------------------------

func TestSkulk_CantBeBlockedByGreaterPower(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Skulker", 2, 2, "skulk", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Big Blocker", 4, 4, "creature")
	if CanBlockSkulk(atk, blk) {
		t.Error("creature with greater power should NOT block skulk")
	}
}

func TestSkulk_CanBeBlockedByEqualPower(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Skulker", 2, 2, "skulk", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Equal Blocker", 2, 2, "creature")
	if !CanBlockSkulk(atk, blk) {
		t.Error("creature with equal power should be able to block skulk")
	}
}

func TestSkulk_CanBeBlockedByLesserPower(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Skulker", 3, 3, "skulk", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Small Blocker", 1, 1, "creature")
	if !CanBlockSkulk(atk, blk) {
		t.Error("creature with lesser power should be able to block skulk")
	}
}

// ---------------------------------------------------------------------------
// Daunt
// ---------------------------------------------------------------------------

func TestDaunt_CantBeBlockedByPower2OrLess(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Daunter", 4, 4, "daunt", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Small Blocker", 2, 2, "creature")
	if CanBlockDaunt(atk, blk) {
		t.Error("creature with power 2 should NOT block daunt")
	}
}

func TestDaunt_CanBeBlockedByPower3Plus(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Daunter", 4, 4, "daunt", "creature")
	blk := addKWCombatBattlefield(gs, 1, "Big Blocker", 3, 3, "creature")
	if !CanBlockDaunt(atk, blk) {
		t.Error("creature with power 3 should be able to block daunt")
	}
}

// ---------------------------------------------------------------------------
// Aggregated evasion check
// ---------------------------------------------------------------------------

func TestCanBlockCombatKeywords_AllPass(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefield(gs, 0, "Normal Attacker", 2, 2, "creature")
	blk := addKWCombatBattlefield(gs, 1, "Normal Blocker", 2, 2, "creature")
	if !CanBlockCombatKeywords(gs, atk, blk) {
		t.Error("normal creatures should be able to block each other")
	}
}

// ============================================================================
// COMBAT MODIFIER TESTS
// ============================================================================

// ---------------------------------------------------------------------------
// Banding
// ---------------------------------------------------------------------------

func TestBanding_HasBanding(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefieldWithKeyword(gs, 0, "Banded Warrior", 2, 2, "banding", "creature")
	if !HasBanding(p) {
		t.Error("creature should have banding")
	}
}

func TestBanding_DamageRedistribution(t *testing.T) {
	gs := newKWCombatGame(t)
	// Two banded creatures: 2/3 and 1/2, total 5 toughness.
	c1 := addKWCombatBattlefield(gs, 0, "Band1", 2, 3, "creature")
	c2 := addKWCombatBattlefield(gs, 0, "Band2", 1, 2, "creature")
	// Simulate 4 damage total (would kill c2 if all on c2).
	c1.MarkedDamage = 1
	c2.MarkedDamage = 3 // Would be lethal for c2 (toughness 2).

	ApplyBandingDamageRedistribution(gs, []*Permanent{c1, c2})

	// After redistribution, damage should be spread to minimize kills.
	// c1 can absorb up to 2 (toughness 3 - 1 = room for 2), c2 can absorb 1.
	// Total 4 damage, c1 should take 2, c2 should take 1, remainder on c1.
	if c1.MarkedDamage+c2.MarkedDamage != 4 {
		t.Errorf("total damage should be preserved: got %d + %d = %d",
			c1.MarkedDamage, c2.MarkedDamage, c1.MarkedDamage+c2.MarkedDamage)
	}
	if c2.MarkedDamage >= c2.Toughness() {
		t.Errorf("c2 should not have lethal damage after redistribution: marked=%d tough=%d",
			c2.MarkedDamage, c2.Toughness())
	}
	if countKWCombatEvents(gs, "banding_redistribute") != 1 {
		t.Error("expected 1 banding_redistribute event")
	}
}

// ---------------------------------------------------------------------------
// Rampage
// ---------------------------------------------------------------------------

func TestRampage_BuffPerBlockerBeyondFirst(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Rampager", 3, 3, "rampage", "creature")
	atk.Flags["rampage_n"] = 2 // Rampage 2.

	ApplyRampage(gs, atk, 3) // 3 blockers -> +2/+2 for each beyond first = +4/+4.

	totalPowerBuff := 0
	for _, m := range atk.Modifications {
		totalPowerBuff += m.Power
	}
	if totalPowerBuff != 4 {
		t.Errorf("rampage 2 with 3 blockers should give +4/+4, got +%d", totalPowerBuff)
	}
}

func TestRampage_NoBuffWithSingleBlocker(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Rampager", 3, 3, "rampage", "creature")

	ApplyRampage(gs, atk, 1) // Only 1 blocker -> no buff.

	if len(atk.Modifications) != 0 {
		t.Error("rampage should give no buff with only 1 blocker")
	}
}

// ---------------------------------------------------------------------------
// Battle Cry
// ---------------------------------------------------------------------------

func TestBattleCry_OtherAttackersGetBuff(t *testing.T) {
	gs := newKWCombatGame(t)
	src := addKWCombatBattlefieldWithKeyword(gs, 0, "Battle Cryer", 2, 2, "battle cry", "creature")
	other1 := addKWCombatBattlefield(gs, 0, "Attacker A", 3, 3, "creature")
	other2 := addKWCombatBattlefield(gs, 0, "Attacker B", 1, 1, "creature")
	attackers := []*Permanent{src, other1, other2}

	ApplyBattleCry(gs, 0, attackers)

	// Source should NOT get buff, others should each get +1/+0.
	if len(src.Modifications) != 0 {
		t.Error("battle cry source should NOT receive +1/+0")
	}
	if len(other1.Modifications) != 1 || other1.Modifications[0].Power != 1 {
		t.Error("other attacker should receive +1/+0 from battle cry")
	}
	if len(other2.Modifications) != 1 || other2.Modifications[0].Power != 1 {
		t.Error("other attacker should receive +1/+0 from battle cry")
	}
}

// ---------------------------------------------------------------------------
// Myriad
// ---------------------------------------------------------------------------

func TestMyriad_CreatesTokensForOtherOpponents(t *testing.T) {
	gs := newKWCombatGame4P(t)
	// Seat 0 attacks seat 1.
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Myriad Beast", 3, 3, "myriad", "creature")
	setAttackerDefender(atk, 1)

	ApplyMyriad(gs, atk, 0)

	// Should create tokens for seats 2 and 3 (the other opponents).
	tokenCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Flags != nil && p.Flags["myriad_token"] == 1 {
			tokenCount++
		}
	}
	if tokenCount != 2 {
		t.Errorf("myriad should create 2 tokens for other opponents, got %d", tokenCount)
	}
}

func TestMyriad_TokensAreTappedAndAttacking(t *testing.T) {
	gs := newKWCombatGame4P(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Myriad Beast", 3, 3, "myriad", "creature")
	setAttackerDefender(atk, 1)

	ApplyMyriad(gs, atk, 0)

	for _, p := range gs.Seats[0].Battlefield {
		if p.Flags != nil && p.Flags["myriad_token"] == 1 {
			if !p.Tapped {
				t.Error("myriad token should be tapped")
			}
			if p.Flags["attacking"] != 1 {
				t.Error("myriad token should be attacking")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Melee
// ---------------------------------------------------------------------------

func TestMelee_BuffPerOpponentAttacked(t *testing.T) {
	gs := newKWCombatGame4P(t)
	meleeCreature := addKWCombatBattlefieldWithKeyword(gs, 0, "Melee Fighter", 2, 2, "melee", "creature")
	other := addKWCombatBattlefield(gs, 0, "Normal Attacker", 3, 3, "creature")
	setAttackerDefender(meleeCreature, 1)
	setAttackerDefender(other, 2) // Attacking different opponent.
	attackers := []*Permanent{meleeCreature, other}

	ApplyMelee(gs, 0, attackers)

	// 2 distinct opponents attacked -> melee creature gets +2/+2.
	if len(meleeCreature.Modifications) != 1 {
		t.Fatalf("melee creature should have 1 modification, got %d", len(meleeCreature.Modifications))
	}
	if meleeCreature.Modifications[0].Power != 2 || meleeCreature.Modifications[0].Toughness != 2 {
		t.Errorf("melee should give +2/+2 for 2 opponents, got +%d/+%d",
			meleeCreature.Modifications[0].Power, meleeCreature.Modifications[0].Toughness)
	}
}

func TestMelee_NoBuff_NoMeleeKeyword(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefield(gs, 0, "Normal Guy", 2, 2, "creature")
	setAttackerDefender(atk, 1)
	attackers := []*Permanent{atk}

	ApplyMelee(gs, 0, attackers)

	if len(atk.Modifications) != 0 {
		t.Error("creature without melee should not receive buff")
	}
}

// ---------------------------------------------------------------------------
// Annihilator
// ---------------------------------------------------------------------------

func TestAnnihilator_DefenderSacrifices(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Eldrazi", 10, 10, "annihilator", "creature")
	atk.Flags["annihilator_n"] = 2

	// Defender has 3 permanents.
	addKWCombatBattlefield(gs, 1, "Perm1", 1, 1, "creature")
	addKWCombatBattlefield(gs, 1, "Perm2", 2, 2, "creature")
	addKWCombatBattlefield(gs, 1, "Perm3", 3, 3, "creature")

	setAttackerDefender(atk, 1)
	ApplyAnnihilator(gs, atk, 1)

	// Defender should have lost 2 permanents.
	remaining := len(gs.Seats[1].Battlefield)
	if remaining != 1 {
		t.Errorf("defender should have 1 permanent left after annihilator 2, got %d", remaining)
	}
}

func TestAnnihilator_GetN(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefieldWithKeyword(gs, 0, "Emrakul", 15, 15, "annihilator", "creature")
	p.Flags["annihilator_n"] = 6
	if GetAnnihilatorN(p) != 6 {
		t.Errorf("expected annihilator 6, got %d", GetAnnihilatorN(p))
	}
}

// ---------------------------------------------------------------------------
// Afflict
// ---------------------------------------------------------------------------

func TestAfflict_DefenderLosesLife(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Afflict Beast", 4, 4, "afflict", "creature")
	atk.Flags["afflict_n"] = 3
	setAttackerDefender(atk, 1)
	gs.Seats[1].Life = 20

	ApplyAfflict(gs, atk, 1)

	if gs.Seats[1].Life != 17 {
		t.Errorf("defender should lose 3 life from afflict 3, got life=%d", gs.Seats[1].Life)
	}
}

func TestAfflict_FireOnBlocked(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Afflict Beast", 4, 4, "afflict", "creature")
	atk.Flags["afflict_n"] = 2
	setAttackerDefender(atk, 1)
	gs.Seats[1].Life = 20

	blk := addKWCombatBattlefield(gs, 1, "Blocker", 3, 3, "creature")
	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk},
	}

	FireAfflictTriggers(gs, blockerMap)

	if gs.Seats[1].Life != 18 {
		t.Errorf("defender should lose 2 life from afflict, got life=%d", gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// Provoke
// ---------------------------------------------------------------------------

func TestProvoke_UntapsAndForces(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Provoker", 3, 3, "provoke", "creature")
	setAttackerDefender(atk, 1)

	// Defender has a tapped creature.
	target := addKWCombatBattlefield(gs, 1, "Tapped Defender", 4, 4, "creature")
	target.Tapped = true

	ApplyProvoke(gs, atk, 1)

	if target.Tapped {
		t.Error("provoke should untap the target creature")
	}
	if target.Flags["provoked_by"] == 0 {
		t.Error("target should have provoked_by flag set")
	}
}

// ---------------------------------------------------------------------------
// Trample over planeswalkers
// ---------------------------------------------------------------------------

func TestTrampleOverPW_Detection(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefieldWithKeyword(gs, 0, "Questing Beast", 4, 4, "trample over planeswalkers", "creature")
	if !HasTrampleOverPlaneswalkers(p) {
		t.Error("should detect trample over planeswalkers")
	}
}

func TestTrampleOverPW_NormalCreatureDoesNotHave(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Normal", 2, 2, "creature")
	if HasTrampleOverPlaneswalkers(p) {
		t.Error("normal creature should not have trample over planeswalkers")
	}
}

// ============================================================================
// COST MODIFICATION TESTS
// ============================================================================

// ---------------------------------------------------------------------------
// Spectacle
// ---------------------------------------------------------------------------

func TestSpectacle_Detection(t *testing.T) {
	card := &Card{
		Name: "Rix Maadi Reveler",
		AST: &gameast.CardAST{
			Name:      "Rix Maadi Reveler",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "spectacle", Args: []interface{}{float64(1)}}},
		},
	}
	if !HasSpectacle(card) {
		t.Error("should detect spectacle keyword")
	}
	if SpectacleCost(card) != 1 {
		t.Errorf("spectacle cost should be 1, got %d", SpectacleCost(card))
	}
}

func TestSpectacle_CanPay_OpponentLostLife(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Flags["opponent_lost_life_this_turn"] = 1

	if !CanPaySpectacle(gs, 0) {
		t.Error("spectacle should be payable when opponent lost life this turn")
	}
}

func TestSpectacle_CannotPay_NoLifeLoss(t *testing.T) {
	gs := newKWCombatGame(t)
	if CanPaySpectacle(gs, 0) {
		t.Error("spectacle should NOT be payable when no opponent lost life")
	}
}

// ---------------------------------------------------------------------------
// Surge
// ---------------------------------------------------------------------------

func TestSurge_Detection(t *testing.T) {
	card := &Card{
		Name: "Crush of Tentacles",
		AST: &gameast.CardAST{
			Name:      "Crush of Tentacles",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "surge", Args: []interface{}{float64(3)}}},
		},
	}
	if !HasSurge(card) {
		t.Error("should detect surge keyword")
	}
	if SurgeCost(card) != 3 {
		t.Errorf("surge cost should be 3, got %d", SurgeCost(card))
	}
}

func TestSurge_CanPay_SpellCastThisTurn(t *testing.T) {
	gs := newKWCombatGame(t)
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["spells_cast_this_turn"] = 1

	if !CanPaySurge(gs, 0) {
		t.Error("surge should be payable when a spell was cast this turn")
	}
}

// ---------------------------------------------------------------------------
// Improvise
// ---------------------------------------------------------------------------

func TestImprovise_CostReduction(t *testing.T) {
	gs := newKWCombatGame(t)
	// 3 untapped artifacts.
	addKWCombatBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	addKWCombatBattlefield(gs, 0, "Mana Vault", 0, 0, "artifact")
	tapped := addKWCombatBattlefield(gs, 0, "Tapped Art", 0, 0, "artifact")
	tapped.Tapped = true

	card := &Card{
		Name: "Metalwork Colossus",
		CMC:  11,
		AST: &gameast.CardAST{
			Name:      "Metalwork Colossus",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "improvise"}},
		},
	}

	if !HasImprovise(card) {
		t.Error("should detect improvise keyword")
	}
	reduction := ImproviseCostReduction(gs, 0)
	if reduction != 2 {
		t.Errorf("improvise should reduce by 2 (untapped artifacts), got %d", reduction)
	}
}

// ---------------------------------------------------------------------------
// Undaunted
// ---------------------------------------------------------------------------

func TestUndaunted_CostReduction4P(t *testing.T) {
	gs := newKWCombatGame4P(t)
	card := &Card{
		Name: "Sublime Exhalation",
		CMC:  6,
		AST: &gameast.CardAST{
			Name:      "Sublime Exhalation",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "undaunted"}},
		},
	}

	if !HasUndaunted(card) {
		t.Error("should detect undaunted keyword")
	}
	reduction := UndauntedReduction(gs, 0)
	if reduction != 3 { // 3 opponents in a 4-player game.
		t.Errorf("undaunted should reduce by 3 in 4-player, got %d", reduction)
	}
}

// ---------------------------------------------------------------------------
// Assist
// ---------------------------------------------------------------------------

func TestAssist_Detection(t *testing.T) {
	card := &Card{
		Name: "Huddle Up",
		AST: &gameast.CardAST{
			Name:      "Huddle Up",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "assist"}},
		},
	}
	if !HasAssist(card) {
		t.Error("should detect assist keyword")
	}
}

// ---------------------------------------------------------------------------
// Offering
// ---------------------------------------------------------------------------

func TestOffering_ReductionFromSacrifice(t *testing.T) {
	gs := newKWCombatGame(t)
	// A goblin with CMC 3 on the battlefield.
	goblin := addKWCombatBattlefield(gs, 0, "Goblin Chieftain", 2, 2, "creature", "goblin")
	goblin.Card.CMC = 3

	reduction := OfferingReduction(gs, 0, "goblin")
	if reduction != 3 {
		t.Errorf("offering should reduce by goblin's CMC (3), got %d", reduction)
	}
}

// ---------------------------------------------------------------------------
// AppendCombatCostModifiers
// ---------------------------------------------------------------------------

func TestAppendCombatCostModifiers_Improvise(t *testing.T) {
	gs := newKWCombatGame(t)
	addKWCombatBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	addKWCombatBattlefield(gs, 0, "Mox Opal", 0, 0, "artifact")

	card := &Card{
		Name: "Whir of Invention",
		CMC:  6,
		AST: &gameast.CardAST{
			Name:      "Whir of Invention",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "improvise"}},
		},
	}

	mods := AppendCombatCostModifiers(gs, card, 0, nil)
	if len(mods) != 1 {
		t.Fatalf("expected 1 cost modifier from improvise, got %d", len(mods))
	}
	if mods[0].Source != "improvise" {
		t.Errorf("expected source 'improvise', got %q", mods[0].Source)
	}
	if mods[0].Amount != 2 {
		t.Errorf("expected reduction of 2, got %d", mods[0].Amount)
	}
}

// ============================================================================
// CASTING RESTRICTION TESTS
// ============================================================================

// ---------------------------------------------------------------------------
// Miracle
// ---------------------------------------------------------------------------

func TestMiracle_Detection(t *testing.T) {
	card := &Card{
		Name: "Terminus",
		AST: &gameast.CardAST{
			Name:      "Terminus",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "miracle", Args: []interface{}{float64(1)}}},
		},
	}
	if !HasMiracle(card) {
		t.Error("should detect miracle keyword")
	}
	if MiracleCost(card) != 1 {
		t.Errorf("miracle cost should be 1, got %d", MiracleCost(card))
	}
}

func TestMiracle_CastForMiracleCost(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 5

	card := &Card{
		Name: "Terminus",
		CMC:  6,
		AST: &gameast.CardAST{
			Name:      "Terminus",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "miracle", Args: []interface{}{float64(1)}}},
		},
		Types: []string{"sorcery"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	err := CastWithMiracle(gs, 0, card)
	if err != nil {
		t.Fatalf("miracle cast should succeed: %v", err)
	}
	// Should have paid 1 mana, not 6.
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("should have paid 1 mana for miracle, mana pool=%d", gs.Seats[0].ManaPool)
	}
	// Card should be on stack.
	if len(gs.Stack) != 1 {
		t.Error("spell should be on the stack")
	}
}

// ---------------------------------------------------------------------------
// Splice
// ---------------------------------------------------------------------------

func TestSplice_Detection(t *testing.T) {
	card := &Card{
		Name: "Glacial Ray",
		AST: &gameast.CardAST{
			Name:      "Glacial Ray",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "splice", Args: []interface{}{float64(2)}}},
		},
	}
	if !HasSplice(card) {
		t.Error("should detect splice keyword")
	}
}

func TestSplice_ApplyOntoArcane(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 10

	arcaneSpell := &Card{
		Name:  "Lava Spike",
		Types: []string{"sorcery", "arcane"},
		CMC:   1,
	}

	spliceCard := &Card{
		Name: "Glacial Ray",
		CMC:  2,
		AST: &gameast.CardAST{
			Name:      "Glacial Ray",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "splice", Args: []interface{}{float64(2)}}},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spliceCard)

	ApplySplice(gs, 0, arcaneSpell)

	// Splice cost should have been paid.
	if gs.Seats[0].ManaPool != 8 {
		t.Errorf("splice cost of 2 should be paid, mana pool=%d", gs.Seats[0].ManaPool)
	}
	// Splice card should still be in hand.
	if len(gs.Seats[0].Hand) != 1 {
		t.Error("splice card should remain in hand")
	}
}

// ---------------------------------------------------------------------------
// Cipher
// ---------------------------------------------------------------------------

func TestCipher_EncodeOnCreature(t *testing.T) {
	gs := newKWCombatGame(t)
	card := &Card{
		Name: "Hands of Binding",
		AST: &gameast.CardAST{
			Name:      "Hands of Binding",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "cipher"}},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	host := addKWCombatBattlefield(gs, 0, "Invisible Stalker", 1, 1, "creature")

	EncodeCipher(gs, 0, card, host)

	if host.Flags["cipher_encoded"] != 1 {
		t.Error("host should have cipher_encoded flag")
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Error("card should be removed from hand")
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Error("card should be in exile")
	}
}

func TestCipher_FireTrigger(t *testing.T) {
	gs := newKWCombatGame(t)
	host := addKWCombatBattlefield(gs, 0, "Invisible Stalker", 1, 1, "creature")
	host.Flags["cipher_encoded"] = 1

	FireCipherTrigger(gs, host)

	if countKWCombatEvents(gs, "cipher_cast_copy") != 1 {
		t.Error("should fire cipher_cast_copy event")
	}
}

// ---------------------------------------------------------------------------
// Adventure
// ---------------------------------------------------------------------------

func TestAdventure_CastAdventureHalf(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 5

	card := &Card{
		Name: "Bonecrusher Giant",
		CMC:  3,
		AST: &gameast.CardAST{
			Name:      "Bonecrusher Giant",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "adventure"}},
		},
		Types: []string{"creature"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	err := CastAdventure(gs, 0, card, 2) // Adventure cost of 2.
	if err != nil {
		t.Fatalf("adventure cast should succeed: %v", err)
	}

	// Card should be in exile (not graveyard).
	if len(gs.Seats[0].Exile) != 1 {
		t.Error("card should be in exile after adventure")
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Error("card should be removed from hand")
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("should have paid 2 mana, pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestAdventure_CreatureCastPermission(t *testing.T) {
	perm := NewAdventureCreatureCastPermission(3)
	if perm.Zone != ZoneExile {
		t.Errorf("adventure creature should cast from exile, got %q", perm.Zone)
	}
	if perm.ManaCost != 3 {
		t.Errorf("adventure creature cost should be 3, got %d", perm.ManaCost)
	}
}

// ---------------------------------------------------------------------------
// Aftermath
// ---------------------------------------------------------------------------

func TestAftermath_CastPermission(t *testing.T) {
	perm := NewAftermathCastPermission(2)
	if perm.Zone != ZoneGraveyard {
		t.Errorf("aftermath should cast from graveyard, got %q", perm.Zone)
	}
	if !perm.ExileOnResolve {
		t.Error("aftermath should exile on resolve")
	}
	if perm.ManaCost != 2 {
		t.Errorf("aftermath cost should be 2, got %d", perm.ManaCost)
	}
}

// ---------------------------------------------------------------------------
// Retrace
// ---------------------------------------------------------------------------

func TestRetrace_CastPermission(t *testing.T) {
	perm := NewRetraceCastPermission(3)
	if perm.Zone != ZoneGraveyard {
		t.Errorf("retrace should cast from graveyard, got %q", perm.Zone)
	}
	if perm.Keyword != "retrace" {
		t.Errorf("retrace permission keyword should be 'retrace', got %q", perm.Keyword)
	}
}

func TestRetrace_CanCast_HasLandInHand(t *testing.T) {
	gs := newKWCombatGame(t)
	addKWCombatHandCard(gs, 0, "Forest", 0, "land")

	if !CanCastRetrace(gs, 0) {
		t.Error("retrace should be castable with a land in hand")
	}
}

func TestRetrace_CannotCast_NoLandInHand(t *testing.T) {
	gs := newKWCombatGame(t)
	addKWCombatHandCard(gs, 0, "Bolt", 1, "instant")

	if CanCastRetrace(gs, 0) {
		t.Error("retrace should NOT be castable without a land in hand")
	}
}

// ---------------------------------------------------------------------------
// Jump-start
// ---------------------------------------------------------------------------

func TestJumpStart_CastPermission(t *testing.T) {
	perm := NewJumpStartCastPermission(4)
	if perm.Zone != ZoneGraveyard {
		t.Errorf("jump-start should cast from graveyard, got %q", perm.Zone)
	}
	if !perm.ExileOnResolve {
		t.Error("jump-start should exile on resolve")
	}
}

func TestJumpStart_Detection(t *testing.T) {
	card := &Card{
		Name: "Chemister's Insight",
		AST: &gameast.CardAST{
			Name:      "Chemister's Insight",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "jump-start"}},
		},
	}
	if !HasJumpStart(card) {
		t.Error("should detect jump-start keyword")
	}
}

// ---------------------------------------------------------------------------
// Overload
// ---------------------------------------------------------------------------

func TestOverload_Detection(t *testing.T) {
	card := &Card{
		Name: "Cyclonic Rift",
		AST: &gameast.CardAST{
			Name:      "Cyclonic Rift",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "overload", Args: []interface{}{float64(7)}}},
		},
	}
	if !HasOverload(card) {
		t.Error("should detect overload keyword")
	}
	if OverloadCost(card) != 7 {
		t.Errorf("overload cost should be 7, got %d", OverloadCost(card))
	}
}

func TestOverload_CastWithOverload(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 10

	card := &Card{
		Name: "Cyclonic Rift",
		CMC:  2,
		AST: &gameast.CardAST{
			Name:      "Cyclonic Rift",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "overload", Args: []interface{}{float64(7)}}},
		},
		Types: []string{"instant"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	err := CastWithOverload(gs, 0, card)
	if err != nil {
		t.Fatalf("overload cast should succeed: %v", err)
	}

	// Should have paid 7 mana.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("should have paid 7 mana for overload, pool=%d", gs.Seats[0].ManaPool)
	}
	// Card should be on stack with overloaded flag.
	if len(gs.Stack) != 1 {
		t.Fatal("spell should be on the stack")
	}
	if !IsOverloaded(gs.Stack[0]) {
		t.Error("stack item should be marked as overloaded")
	}
}

func TestOverload_NotOverloaded(t *testing.T) {
	item := &StackItem{
		CostMeta: map[string]interface{}{"something_else": true},
	}
	if IsOverloaded(item) {
		t.Error("stack item without overloaded flag should not be overloaded")
	}
}

// ============================================================================
// PROTECTION VARIANT TESTS
// ============================================================================

// ---------------------------------------------------------------------------
// Protection from [type]
// ---------------------------------------------------------------------------

func TestProtectionFromType_Creatures(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Protected", 2, 2, "creature")
	p.Flags["prot_type:creature"] = 1

	protTypes := ProtectionTypes(p)
	if _, ok := protTypes["creature"]; !ok {
		t.Error("should have protection from creatures")
	}
	if !HasProtectionFromType(p, "creature") {
		t.Error("HasProtectionFromType should return true for creatures")
	}
}

func TestProtectionFromType_ASTParsing(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Spectra Ward", 0, 0, "enchantment")
	p.Card.AST = &gameast.CardAST{
		Name: "Spectra Ward",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "protection", Raw: "protection from creatures"},
		},
	}

	if !HasProtectionFromType(p, "creature") {
		t.Error("should parse protection from creatures from AST")
	}
}

// ---------------------------------------------------------------------------
// Hexproof from [color]
// ---------------------------------------------------------------------------

func TestHexproofFromColor_Detection(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	p.Flags["hexproof_from:B"] = 1

	colors := HexproofFromColors(p)
	if _, ok := colors["B"]; !ok {
		t.Error("should detect hexproof from black via flag")
	}
}

func TestHexproofFromColor_ASTParsing(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{
		Name: "Knight of Grace",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "hexproof", Raw: "hexproof from black"},
		},
	}

	colors := HexproofFromColors(p)
	if _, ok := colors["B"]; !ok {
		t.Error("should parse hexproof from black from AST")
	}
}

func TestHexproofFromColor_BlocksOpponentTargeting(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	p.Flags["hexproof_from:B"] = 1

	blackSpell := &Card{
		Name:   "Doom Blade",
		Types:  []string{"instant", "black"},
		Colors: []string{"B"},
	}

	// Opponent (seat 1) tries to target with a black spell.
	if CanBeTargetedByCombat(p, 1, blackSpell) {
		t.Error("hexproof from black should prevent black spell targeting by opponent")
	}
}

func TestHexproofFromColor_AllowsControllerTargeting(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	p.Flags["hexproof_from:B"] = 1

	blackSpell := &Card{
		Name:   "Dark Ritual",
		Types:  []string{"instant", "black"},
		Colors: []string{"B"},
	}

	// Controller (seat 0) can still target own creatures.
	if !CanBeTargetedByCombat(p, 0, blackSpell) {
		t.Error("hexproof from black should allow controller to target")
	}
}

func TestHexproofFromColor_AllowsOtherColorTargeting(t *testing.T) {
	gs := newKWCombatGame(t)
	p := addKWCombatBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	p.Flags["hexproof_from:B"] = 1

	redSpell := &Card{
		Name:   "Lightning Bolt",
		Types:  []string{"instant", "red"},
		Colors: []string{"R"},
	}

	// Opponent with a red spell should be able to target.
	if !CanBeTargetedByCombat(p, 1, redSpell) {
		t.Error("hexproof from black should allow red spell targeting by opponent")
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestCheckCombatKeywordsCombat_RunsAll(t *testing.T) {
	gs := newKWCombatGame(t)
	atk := addKWCombatBattlefieldWithKeyword(gs, 0, "Multi-Keyword", 4, 4, "rampage", "creature")
	atk.Flags["rampage_n"] = 1
	setAttackerDefender(atk, 1)

	blk1 := addKWCombatBattlefield(gs, 1, "Blocker1", 2, 2, "creature")
	blk2 := addKWCombatBattlefield(gs, 1, "Blocker2", 2, 2, "creature")
	blockerMap := map[*Permanent][]*Permanent{
		atk: {blk1, blk2},
	}

	CheckCombatKeywordsCombat(gs, []*Permanent{atk}, blockerMap)

	// Rampage should have fired.
	if countKWCombatEvents(gs, "rampage") != 1 {
		t.Error("rampage should have fired")
	}
}

func TestCheckAttackKeywordsCombat_BattleCryAndMelee(t *testing.T) {
	gs := newKWCombatGame4P(t)
	battleCryer := addKWCombatBattlefieldWithKeywords(gs, 0, "Battle Cryer", 2, 2,
		[]string{"battle cry", "melee"}, "creature")
	other := addKWCombatBattlefield(gs, 0, "Other Attacker", 3, 3, "creature")
	setAttackerDefender(battleCryer, 1)
	setAttackerDefender(other, 2)
	attackers := []*Permanent{battleCryer, other}

	CheckAttackKeywordsCombat(gs, 0, attackers)

	// Battle cry: other should get +1/+0.
	hasBC := false
	for _, m := range other.Modifications {
		if m.Power == 1 && m.Toughness == 0 {
			hasBC = true
		}
	}
	if !hasBC {
		t.Error("other attacker should have +1/+0 from battle cry")
	}

	// Melee: battle cryer should have +2/+2 (2 opponents attacked).
	hasMelee := false
	for _, m := range battleCryer.Modifications {
		if m.Power == 2 && m.Toughness == 2 {
			hasMelee = true
		}
	}
	if !hasMelee {
		t.Error("battle cryer should have +2/+2 from melee (2 opponents attacked)")
	}
}

func TestAllCombatKeywords_NonEmpty(t *testing.T) {
	kws := AllCombatKeywords()
	if len(kws) < 20 {
		t.Errorf("expected at least 20 combat keywords, got %d", len(kws))
	}
}

// ---------------------------------------------------------------------------
// Nil safety tests
// ---------------------------------------------------------------------------

func TestCombatKeywords_NilSafety(t *testing.T) {
	// These should not panic.
	CanBlockIntimidate(nil, nil)
	CanBlockFear(nil, nil)
	CanBlockShadow(nil, nil)
	CanBlockSkulk(nil, nil)
	CanBlockDaunt(nil, nil)
	CanBlockCombatKeywords(nil, nil, nil)
	HasBanding(nil)
	ApplyBandingDamageRedistribution(nil, nil)
	ApplyRampage(nil, nil, 0)
	FireRampageTriggers(nil, nil)
	ApplyBattleCry(nil, 0, nil)
	ApplyMyriad(nil, nil, 0)
	ApplyMelee(nil, 0, nil)
	ApplyAnnihilator(nil, nil, 0)
	ApplyAfflict(nil, nil, 0)
	ApplyProvoke(nil, nil, 0)
	HasTrampleOverPlaneswalkers(nil)
	HasSpectacle(nil)
	HasSurge(nil)
	HasImprovise(nil)
	HasAssist(nil)
	HasUndaunted(nil)
	HasOffering(nil)
	HasMiracle(nil)
	HasSplice(nil)
	HasCipher(nil)
	HasAdventure(nil)
	HasAftermath(nil)
	HasRetrace(nil)
	HasJumpStart(nil)
	HasOverload(nil)
	ProtectionTypes(nil)
	HexproofFromColors(nil)
}
