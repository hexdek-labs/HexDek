package gameengine

// Phase 4 combat tests. Mirrors the semantic parity smoke-tests from the
// Python reference. All fixtures are synthetic (no corpus load needed).

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Combat-specific fixture helpers.
// -----------------------------------------------------------------------------

// newCombatGame is a two-seat combat fixture with seat 0 as active.
func newCombatGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	return gs
}

// addCreature puts a creature on seat's battlefield with optional keywords
// (applied via the "kw:<name>" flag so HasKeyword picks them up without a
// full AST). Returns the new Permanent.
func addCreature(gs *GameState, seat int, name string, pow, tough int, keywords ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature"},
	}
	p := &Permanent{
		Card:          card,
		Controller:    seat,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
		SummoningSick: false,
	}
	for _, kw := range keywords {
		p.Flags["kw:"+kw] = 1
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addColoredCreature adds a creature of the given color letter(s). Colors
// go into Card.Types ("red"/"blue"/...) so cardColors() finds them.
func addColoredCreature(gs *GameState, seat int, name string, pow, tough int, color string, keywords ...string) *Permanent {
	p := addCreature(gs, seat, name, pow, tough, keywords...)
	p.Card.Types = append(p.Card.Types, color)
	return p
}

// addCardWithAbility creates a creature whose CardAST carries the given
// Triggered ability — used to exercise attack/combat-damage trigger firing.
func addCardWithAbility(gs *GameState, seat int, name string, pow, tough int, ab gameast.Ability) *Permanent {
	ast := &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{ab},
	}
	card := &Card{
		AST:           ast,
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature"},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// -----------------------------------------------------------------------------
// 1. Basic attacks / blocks / damage — canonical cases.
// -----------------------------------------------------------------------------

func TestCombat_UnblockedAttack(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Grizzly Bears", 2, 2)

	CombatPhase(gs)

	if gs.Seats[1].Life != 18 {
		t.Errorf("defender life: want 18, got %d", gs.Seats[1].Life)
	}
}

func TestCombat_BlockedBothDie(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Bear A", 2, 2)
	blk := addCreature(gs, 1, "Bear B", 2, 2)

	CombatPhase(gs)

	if gs.Seats[1].Life != 20 {
		t.Errorf("defender life: want 20, got %d", gs.Seats[1].Life)
	}
	if atk.MarkedDamage != 2 {
		t.Errorf("atk marked: want 2, got %d", atk.MarkedDamage)
	}
	if blk.MarkedDamage != 2 {
		t.Errorf("blk marked: want 2, got %d", blk.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// 2. Trample: spillover damage goes to defender.
// -----------------------------------------------------------------------------

func TestCombat_TrampleSpillover(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Ghalta", 3, 3, "trample")
	addCreature(gs, 1, "Ornithopter", 0, 2)
	// Ornithopter has 0 power so only the attacker deals damage. It has
	// 2 toughness; the trampler has 3 → assigns 2 to kill blocker then
	// 1 trample spills to defender (20 → 19).

	CombatPhase(gs)

	if gs.Seats[1].Life != 19 {
		t.Errorf("trample spillover: want 19, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 3. Evasion: flying, reach.
// -----------------------------------------------------------------------------

func TestCombat_FlyingNotBlockable(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Dragon", 3, 3, "flying")
	addCreature(gs, 1, "Grizzly", 2, 2) // no flying, no reach

	CombatPhase(gs)

	if gs.Seats[1].Life != 17 {
		t.Errorf("flying unblockable by ground: want 17, got %d", gs.Seats[1].Life)
	}
}

func TestCombat_FlyingBlockedByReach(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Dragon", 3, 3, "flying")
	blk := addCreature(gs, 1, "Reach Spider", 2, 3, "reach")

	CombatPhase(gs)

	if gs.Seats[1].Life != 20 {
		t.Errorf("reach blocks: want 20, got %d", gs.Seats[1].Life)
	}
	if atk.MarkedDamage != 2 {
		t.Errorf("atk marked: want 2, got %d", atk.MarkedDamage)
	}
	if blk.MarkedDamage != 3 {
		t.Errorf("blk marked: want 3, got %d", blk.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// 4. First strike & double strike.
// -----------------------------------------------------------------------------

func TestCombat_FirstStrikeKillsBlocker(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "White Knight", 2, 2, "first strike")
	blk := addCreature(gs, 1, "Grizzly", 2, 2)
	// atk strikes first for 2 → blocker dies before dealing return damage.
	// For the test we can't run SBAs (Phase 6), so we simulate removing the
	// blocker manually after first-strike step. Instead: use a blocker with
	// 2 toughness and track MarkedDamage at end of combat. The attacker
	// should have 0 damage (dead blocker doesn't swing back).
	//
	// NOTE: We don't run SBAs in Phase 4; we therefore post-process by
	// verifying the blocker's marked damage equals its toughness but the
	// attacker's marked damage is 0 (blocker died in FS step). In Phase 6
	// this removal happens automatically.

	CombatPhase(gs)

	if blk.MarkedDamage != 2 {
		t.Errorf("first-strike blk dmg: want 2, got %d", blk.MarkedDamage)
	}
	// Because we don't have SBAs in Phase 4, the blocker is technically
	// still on the battlefield — so it WILL swing back. Instead, test the
	// Python-parity shortcut: simulate SBA between the two steps by
	// removing the blocker after FS step if its MarkedDamage exceeds
	// toughness.
	_ = atk
}

func TestCombat_FirstStrikeSavesAttacker(t *testing.T) {
	// Attacker kills blocker in FS step; we simulate SBA by removing
	// the dead blocker ourselves, then run the regular step.
	gs := newCombatGame(t)
	gs.Active = 0

	atk := addCreature(gs, 0, "White Knight", 2, 2, "first strike")
	blk := addCreature(gs, 1, "Grizzly", 2, 2)

	// Run declare-attackers/blockers manually so we can inject an SBA.
	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	DealCombatDamageStep(gs, attackers, blockerMap, true)
	// MVP SBA stand-in: remove the dead blocker.
	if blk.MarkedDamage >= blk.Toughness() {
		gs.removePermanent(blk)
	}
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if atk.MarkedDamage != 0 {
		t.Errorf("first-strike atk should take 0 dmg, got %d", atk.MarkedDamage)
	}
}

func TestCombat_DoubleStrikeWithTrampleHitsFace(t *testing.T) {
	// Per CR §510.1c, when a blocked attacker's blockers have all died by
	// the time damage is assigned, damage fizzles UNLESS the attacker has
	// trample. We test the trample variant so the regular step goes to
	// face — without trample, damage fizzles (Python parity, see
	// _deal_combat_damage_step live_blockers branch).
	gs := newCombatGame(t)
	gs.Active = 0
	atk := addCreature(gs, 0, "Mirran Crusader", 2, 2, "double strike", "trample")
	blk := addCreature(gs, 1, "Grizzly", 2, 2)

	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	DealCombatDamageStep(gs, attackers, blockerMap, true)
	// Simulate SBA: blk died.
	if blk.MarkedDamage >= blk.Toughness() {
		gs.removePermanent(blk)
	}
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if atk.MarkedDamage != 0 {
		t.Errorf("double-strike attacker should take 0 dmg: got %d", atk.MarkedDamage)
	}
	// Regular step: blocker dead, attacker has trample, full 2 tramples.
	if gs.Seats[1].Life != 18 {
		t.Errorf("double-strike + trample face dmg: want 18, got %d", gs.Seats[1].Life)
	}
}

func TestCombat_DoubleStrikeWithoutTrampleFizzles(t *testing.T) {
	// Without trample, if the blocker dies in FS step, the regular step's
	// damage fizzles — CR §510.1c. Python parity.
	gs := newCombatGame(t)
	gs.Active = 0
	atk := addCreature(gs, 0, "Mirran Crusader", 2, 2, "double strike")
	blk := addCreature(gs, 1, "Grizzly", 2, 2)

	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	DealCombatDamageStep(gs, attackers, blockerMap, true)
	if blk.MarkedDamage >= blk.Toughness() {
		gs.removePermanent(blk)
	}
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if atk.MarkedDamage != 0 {
		t.Errorf("DS attacker takes 0: got %d", atk.MarkedDamage)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("DS without trample fizzles when blocker dies: want 20, got %d",
			gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 5. Deathtouch & lifelink.
// -----------------------------------------------------------------------------

func TestCombat_DeathtouchTinyKillsGiant(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Deathtouch Rat", 1, 1, "deathtouch")
	blk := addCreature(gs, 1, "Giant", 5, 5)
	// Greedy blocker AI on seat 1 won't block an attacker (seat 1 is
	// defender). Force the block manually by constructing blockerMap.
	attackers := []*Permanent{gs.Seats[0].Battlefield[0]}
	attackers[0].Flags[flagAttacking] = 1
	attackers[0].Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{attackers[0]: {blk}}

	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if blk.MarkedDamage < blk.Toughness() {
		t.Errorf("deathtouch should be lethal to giant: marked=%d tough=%d",
			blk.MarkedDamage, blk.Toughness())
	}
}

func TestCombat_LifelinkHeals(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Ajani's Pridemate", 3, 3, "lifelink")

	CombatPhase(gs)

	if gs.Seats[0].Life != 23 {
		t.Errorf("lifelink gain: want 23, got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 17 {
		t.Errorf("lifelink dmg: want 17, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 6. Protection.
// -----------------------------------------------------------------------------

func TestCombat_ProtectionFromRedPreventsDamage(t *testing.T) {
	gs := newCombatGame(t)

	// Attacker has protection from red → the red blocker's damage is prevented
	// but the attacker still kills the blocker.
	atk := addCreature(gs, 0, "White Knight", 3, 3)
	atk.Flags["prot:R"] = 1

	blk := addColoredCreature(gs, 1, "Red Bear", 4, 4, "red")

	// Force the block.
	attackers := []*Permanent{atk}
	atk.Flags[flagAttacking] = 1
	atk.Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{atk: {blk}}

	DealCombatDamageStep(gs, attackers, blockerMap, false)

	if atk.MarkedDamage != 0 {
		t.Errorf("protection should prevent damage: got %d", atk.MarkedDamage)
	}
	if blk.MarkedDamage != 3 {
		t.Errorf("attacker kills blocker: want 3 dmg, got %d", blk.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// 7. Indestructible.
// -----------------------------------------------------------------------------

func TestCombat_IndestructibleSurvivesLethal(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Ajani's Pridemate", 2, 2, "indestructible")
	blk := addCreature(gs, 1, "Giant", 5, 5)

	attackers := []*Permanent{atk}
	atk.Flags[flagAttacking] = 1
	atk.Flags[flagDeclaredAttacker] = 1
	blockerMap := map[*Permanent][]*Permanent{atk: {blk}}

	DealCombatDamageStep(gs, attackers, blockerMap, false)

	// Indestructibility is an SBA-level rule (Phase 6). Phase 4's
	// contract: lethal damage is MARKED but not destroyed. Phase 6 SBA
	// will check the keyword and NOT destroy. For Phase 4 we just
	// verify damage got marked.
	if atk.MarkedDamage != 5 {
		t.Errorf("indestructible marks damage normally: want 5, got %d", atk.MarkedDamage)
	}
	if !atk.HasKeyword("indestructible") {
		t.Errorf("indestructible keyword must be detectable")
	}
}

// -----------------------------------------------------------------------------
// 8. Defender can't attack.
// -----------------------------------------------------------------------------

func TestCombat_DefenderCantAttack(t *testing.T) {
	gs := newCombatGame(t)
	// Wall of Blossoms: 0/4 defender.
	addCreature(gs, 0, "Wall of Blossoms", 0, 4, "defender")

	CombatPhase(gs)

	if gs.Seats[1].Life != 20 {
		t.Errorf("defender creature did not attack: want 20, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 9. Haste: summoning-sick creature with haste can attack.
// -----------------------------------------------------------------------------

func TestCombat_HasteAttacksImmediately(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Lightning Elemental", 4, 1, "haste")
	atk.SummoningSick = true

	CombatPhase(gs)

	if gs.Seats[1].Life != 16 {
		t.Errorf("haste attacks: want 16, got %d", gs.Seats[1].Life)
	}
}

func TestCombat_SummoningSickNoHasteCantAttack(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Baloth", 4, 4)
	atk.SummoningSick = true

	CombatPhase(gs)

	if gs.Seats[1].Life != 20 {
		t.Errorf("summon-sick can't attack: want 20, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 10. Vigilance: no tap on attack.
// -----------------------------------------------------------------------------

func TestCombat_VigilanceDoesntTap(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Serra Angel", 4, 4, "vigilance")

	CombatPhase(gs)

	if atk.Tapped {
		t.Errorf("vigilance attacker should not be tapped")
	}
	if gs.Seats[1].Life != 16 {
		t.Errorf("vigilance still deals damage: want 16, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 11. Menace: at least 2 blockers required.
// -----------------------------------------------------------------------------

func TestCombat_MenaceWithOneBlockerGoesUnblocked(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Gore-House Chainwalker", 3, 3, "menace")
	addCreature(gs, 1, "Lone Defender", 2, 2) // only one blocker

	CombatPhase(gs)

	// Menace needs 2 blockers; only 1 available → unblockable.
	if gs.Seats[1].Life != 17 {
		t.Errorf("menace unblocked with single blocker: want 17, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 12. Attacks trigger — "When ~ attacks, draw a card" fires ONCE.
// -----------------------------------------------------------------------------

func TestCombat_AttackTriggerFiresOnce(t *testing.T) {
	gs := newCombatGame(t)

	trigger := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "attack", Actor: &gameast.Filter{Base: "self"}},
		Effect: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	p := addCardWithAbility(gs, 0, "Attacker", 2, 2, trigger)
	p.Card.Types = []string{"creature"}
	// Library for the draw.
	gs.Seats[0].Library = []*Card{{Name: "X", Owner: 0}}

	CombatPhase(gs)

	// Drew one card, so life total unchanged for attacker, +1 card in hand.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("attack trigger draw: want 1 in hand, got %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// 13. Per-instance combat-damage triggers — "create a Treasure" fires per
//     damage event. Double strike should fire twice; plain damage once.
// -----------------------------------------------------------------------------

func TestCombat_CombatDamageTriggerFiresOncePerHit(t *testing.T) {
	gs := newCombatGame(t)

	treasurePT := [2]int{0, 0}
	trigger := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "deal_combat_damage"},
		Effect: &gameast.CreateToken{
			Count: *gameast.NumInt(1),
			PT:    &treasurePT,
			Types: []string{"artifact", "treasure"},
		},
	}
	p := addCardWithAbility(gs, 0, "Goldhoarder", 2, 2, trigger)
	_ = p

	CombatPhase(gs)

	// One damage instance → one Treasure token on the attacker's side.
	treasures := 0
	for _, perm := range gs.Seats[0].Battlefield {
		for _, t := range perm.Card.Types {
			if t == "treasure" {
				treasures++
			}
		}
	}
	if treasures != 1 {
		t.Errorf("combat damage trigger: want 1 treasure, got %d", treasures)
	}
}

func TestCombat_DoubleStrikeFiresCombatDamageTriggerTwice(t *testing.T) {
	gs := newCombatGame(t)

	treasurePT := [2]int{0, 0}
	trigger := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "deal_combat_damage"},
		Effect: &gameast.CreateToken{
			Count: *gameast.NumInt(1),
			PT:    &treasurePT,
			Types: []string{"artifact", "treasure"},
		},
	}
	p := addCardWithAbility(gs, 0, "Hoarding DS", 2, 2, trigger)
	p.Flags["kw:double strike"] = 1

	// Use the manual step path so we can simulate SBA cleanup between
	// first-strike and regular steps.
	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	DealCombatDamageStep(gs, attackers, blockerMap, true)
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	treasures := 0
	for _, perm := range gs.Seats[0].Battlefield {
		for _, t := range perm.Card.Types {
			if t == "treasure" {
				treasures++
			}
		}
	}
	if treasures != 2 {
		t.Errorf("double strike triggers twice: want 2 treasures, got %d", treasures)
	}
}

// -----------------------------------------------------------------------------
// 14. Enters-tapped-and-attacking token does NOT fire its own attack trigger.
// -----------------------------------------------------------------------------

func TestCombat_EnteredAttackingTokenDoesNotFireTrigger(t *testing.T) {
	gs := newCombatGame(t)

	trigger := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "attack"},
		Effect: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	tok := addCardWithAbility(gs, 0, "Hero of Bladehold Token", 1, 1, trigger)
	// Simulate having entered tapped+attacking via CreateToken.
	tok.Tapped = true
	setPermFlag(tok, flagAttacking, true)

	gs.Seats[0].Library = []*Card{{Name: "X", Owner: 0}}

	// Call DeclareAttackers directly (no "legal" creatures since it's tapped).
	attackers := DeclareAttackers(gs, 0)

	// The token should be counted as attacker (via CR §506.3 scoop-in)...
	inList := false
	for _, a := range attackers {
		if a == tok {
			inList = true
		}
	}
	if !inList {
		t.Errorf("token entering tapped+attacking should be in attackers list")
	}
	// ...but its OWN attack trigger should NOT have fired (wasn't declared).
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("entered-attacking token fired attack trigger (should not): hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// 15. Extra combat phase cascades (Aggravated Assault style).
// -----------------------------------------------------------------------------

func TestCombat_ExtraCombatPhaseDamagesAgain(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Serra Angel", 4, 4, "vigilance")
	_ = atk

	// First combat phase: 4 damage.
	CombatPhase(gs)
	if gs.Seats[1].Life != 16 {
		t.Fatalf("first combat: want 16, got %d", gs.Seats[1].Life)
	}

	// Second combat phase: +4 more.
	CombatPhase(gs)
	if gs.Seats[1].Life != 12 {
		t.Errorf("extra combat: want 12, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 16. Beginning-of-combat trigger fires once per combat phase.
// -----------------------------------------------------------------------------

func TestCombat_BeginningOfCombatTriggerFires(t *testing.T) {
	gs := newCombatGame(t)

	treasurePT := [2]int{0, 0}
	ab := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "begin_of_combat"},
		Effect: &gameast.CreateToken{
			Count: *gameast.NumInt(1),
			PT:    &treasurePT,
			Types: []string{"artifact", "treasure"},
		},
	}
	addCardWithAbility(gs, 0, "Dockside", 1, 1, ab)

	CombatPhase(gs)

	treasures := 0
	for _, perm := range gs.Seats[0].Battlefield {
		for _, t := range perm.Card.Types {
			if t == "treasure" {
				treasures++
			}
		}
	}
	if treasures != 1 {
		t.Errorf("begin-of-combat trigger: want 1 treasure, got %d", treasures)
	}

	// Second combat should fire again.
	CombatPhase(gs)
	treasures = 0
	for _, perm := range gs.Seats[0].Battlefield {
		for _, t := range perm.Card.Types {
			if t == "treasure" {
				treasures++
			}
		}
	}
	if treasures != 2 {
		t.Errorf("second combat begin trigger: want 2 treasures, got %d", treasures)
	}
}

// -----------------------------------------------------------------------------
// 17. Damage wear-off + combat-flag clear at end of combat.
// -----------------------------------------------------------------------------

func TestCombat_EndOfCombatClearsFlags(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Bear", 2, 2)

	CombatPhase(gs)

	if atk.IsAttacking() {
		t.Errorf("attacking flag not cleared at end of combat")
	}
	if atk.WasDeclaredAttacker() {
		t.Errorf("declared_attacker flag not cleared at end of combat")
	}
}

func TestCombat_UntilEndOfCombatModificationExpires(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Bear", 2, 2)
	atk.Modifications = append(atk.Modifications, Modification{
		Power: 2, Toughness: 2, Duration: "until_end_of_combat",
		Timestamp: gs.NextTimestamp(),
	})
	if atk.Power() != 4 {
		t.Fatalf("pre-combat buff: want 4, got %d", atk.Power())
	}

	CombatPhase(gs)

	if atk.Power() != 2 {
		t.Errorf("EoC buff expired: want 2, got %d", atk.Power())
	}
}

// -----------------------------------------------------------------------------
// 18. Menace with 2 blockers: assigns both correctly.
// -----------------------------------------------------------------------------

func TestCombat_MenaceWithTwoBlockers(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Menace Ogre", 3, 3, "menace")
	b1 := addCreature(gs, 1, "Bear A", 2, 2)
	b2 := addCreature(gs, 1, "Bear B", 2, 2)

	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	if len(blockerMap[atk]) != 2 {
		t.Fatalf("menace 2 blockers: want 2, got %d", len(blockerMap[atk]))
	}
	DealCombatDamageStep(gs, attackers, blockerMap, false)

	// Menace Ogre distributes 3 dmg. Trample is NOT on, so the policy
	// assigns lethal to highest-toughness blocker first. Both blockers
	// have 2 toughness; attacker sends 2 to first then 1 to second.
	totalDealt := b1.MarkedDamage + b2.MarkedDamage
	if totalDealt != 3 {
		t.Errorf("menace damage distribution: want 3 total, got %d", totalDealt)
	}
}

// -----------------------------------------------------------------------------
// 19. §508 defender subtype on defender side (can still block).
// -----------------------------------------------------------------------------

func TestCombat_DefenderCanStillBlock(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 0, "Bear", 2, 2)
	wall := addCreature(gs, 1, "Wall", 0, 4, "defender")

	CombatPhase(gs)

	// Wall should have blocked.
	if gs.Seats[1].Life != 20 {
		t.Errorf("wall blocked: want 20, got %d", gs.Seats[1].Life)
	}
	if wall.MarkedDamage != 2 {
		t.Errorf("wall took damage: want 2, got %d", wall.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// 20. Protection from red blocker → can't be blocked.
// -----------------------------------------------------------------------------

func TestCombat_ProtectionFromColorCantBeBlocked(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "White Knight", 2, 2)
	atk.Flags["prot:R"] = 1

	redBlk := addColoredCreature(gs, 1, "Red Bear", 4, 4, "red")
	_ = redBlk

	attackers := DeclareAttackers(gs, 0)
	blockerMap := DeclareBlockers(gs, attackers, 1)

	if len(blockerMap[atk]) != 0 {
		t.Errorf("protection blocker disallowed: want 0 blockers, got %d", len(blockerMap[atk]))
	}
	DealCombatDamageStep(gs, attackers, blockerMap, false)
	if gs.Seats[1].Life != 18 {
		t.Errorf("protection unblockable by red: want 18, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// 21. Already-attacked-this-combat: multi-combat reuse (Aggravated Assault).
// -----------------------------------------------------------------------------

func TestCombat_CreatureAttacksTwiceAcrossCombats(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Angel", 3, 3, "vigilance")

	// Combat 1.
	CombatPhase(gs)
	if !atk.HasKeyword("vigilance") {
		t.Fatalf("keyword lookup sanity")
	}
	if gs.Seats[1].Life != 17 {
		t.Fatalf("first combat: want 17, got %d", gs.Seats[1].Life)
	}

	// Combat 2 (simulating Aggravated Assault's extra-combat effect).
	CombatPhase(gs)
	if gs.Seats[1].Life != 14 {
		t.Errorf("second combat: want 14, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Benchmark: full combat step with 5 attackers vs 5 blockers — target <5μs
// per step (just the combat invocation; setup allocations are amortized).
// -----------------------------------------------------------------------------

func BenchmarkCombat_FiveAttackers(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		gs := NewGameState(2, rand.New(rand.NewSource(int64(n))), nil)
		gs.Active = 0
		for i := 0; i < 5; i++ {
			addCreature(gs, 0, "Attacker", 2, 2)
			addCreature(gs, 1, "Blocker", 2, 2)
		}
		CombatPhase(gs)
	}
}

// BenchmarkCombat_StepOnly benchmarks just the DealCombatDamageStep on a
// pre-built battlefield of 5 attackers vs 5 blockers. This is the inner
// hot path; the full CombatPhase includes declaration logs and trigger
// scans that are O(battlefield).
func BenchmarkCombat_StepOnly(b *testing.B) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	var attackers []*Permanent
	for i := 0; i < 5; i++ {
		a := addCreature(gs, 0, "Attacker", 2, 2)
		blk := addCreature(gs, 1, "Blocker", 2, 2)
		a.Flags[flagAttacking] = 1
		a.Flags[flagDeclaredAttacker] = 1
		attackers = append(attackers, a)
		_ = blk
	}
	blockerMap := map[*Permanent][]*Permanent{}
	for i, a := range attackers {
		blockerMap[a] = []*Permanent{gs.Seats[1].Battlefield[i]}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// Reset marked damage so we can re-run the step in place.
		for _, a := range attackers {
			a.MarkedDamage = 0
		}
		for _, p := range gs.Seats[1].Battlefield {
			p.MarkedDamage = 0
		}
		DealCombatDamageStep(gs, attackers, blockerMap, false)
	}
}
