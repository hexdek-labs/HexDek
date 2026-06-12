package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// announce_targets_r62_test.go — r62 announcement-time targeting.
//
// Pins the keystone fix from the r61 architecture review (reports 04 + 05):
// targets for required single-target spells/abilities are now chosen at
// ANNOUNCEMENT (CR §601.2c / §602.2b), consulting seat.Hat.ChooseTarget per
// §608.2a, stamped onto StackItem.Targets, and honored at resolution via
// the announced-target consumption path in PickTarget. This activates the
// announcement machinery (ward §702.21, heroic §702.123, the populated-path
// §608.2b gate) for AI-driven casts that pass nil targets.

// announceStubHat overrides exactly one decision — ChooseTarget — and
// declines everything else. Embedding GreedyHatStub keeps the interface
// burden at one method.
type announceStubHat struct {
	GreedyHatStub
	pick func(gs *GameState, seatIdx int, filter gameast.Filter, legal []Target) Target
}

func (h *announceStubHat) ChooseTarget(gs *GameState, seatIdx int, filter gameast.Filter, legal []Target) Target {
	if h.pick != nil {
		return h.pick(gs, seatIdx, filter, legal)
	}
	return Target{}
}

// TestAnnounce_RemovalStampsTargetsAndKeepsPolicyPick: a hatless cast of
// "Destroy target creature" with nil targets must (a) stamp announcement
// targets, (b) keep the pre-r62 policy outcome (highest-power enemy dies).
func TestAnnounce_RemovalStampsTargetsAndKeepsPolicyPick(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3

	own := addCreature(gs, 0, "My Bear", 2, 2)
	weak := addCreature(gs, 1, "Weak Goblin", 1, 1)
	strong := addCreature(gs, 1, "Big Dragon", 6, 6)

	doom := addHandCardWithEffect(gs, 0, "Doom Blade", 2,
		&gameast.Destroy{
			Target: gameast.Filter{Base: "creature", Targeted: true},
		}, "instant")

	if err := CastSpell(gs, 0, doom, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Policy pick = highest-power enemy creature → Big Dragon dies.
	if permanentOnBattlefield(gs, strong) {
		t.Fatalf("policy removal should have destroyed the highest-power enemy (Big Dragon)")
	}
	if !permanentOnBattlefield(gs, weak) {
		t.Fatalf("policy removal hit the WRONG creature (Weak Goblin died)")
	}
	if !permanentOnBattlefield(gs, own) {
		t.Fatalf("removal must not hit the caster's own creature")
	}
}

// TestAnnounce_HatChoiceOverridesPolicyAndResolutionHonorsIt is the core
// §608.2a contract: when a Hat is present, ITS choice — not the engine
// policy — is announced AND is the target the resolution actually hits.
func TestAnnounce_HatChoiceOverridesPolicyAndResolutionHonorsIt(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3

	weak := addCreature(gs, 1, "Weak Goblin", 1, 1)
	strong := addCreature(gs, 1, "Big Dragon", 6, 6)

	// Hat policy: always pick the WEAKEST enemy — the opposite of the
	// engine policy (highest power), so any pass-through is detectable.
	gs.Seats[0].Hat = &announceStubHat{pick: func(_ *GameState, _ int, _ gameast.Filter, legal []Target) Target {
		best := Target{}
		bestPow := 1 << 30
		for _, l := range legal {
			if l.Kind == TargetKindPermanent && l.Permanent != nil && l.Permanent.Power() < bestPow {
				best = l
				bestPow = l.Permanent.Power()
			}
		}
		return best
	}}

	doom := addHandCardWithEffect(gs, 0, "Doom Blade", 2,
		&gameast.Destroy{
			Target: gameast.Filter{Base: "creature", Targeted: true},
		}, "instant")

	if err := CastSpell(gs, 0, doom, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if permanentOnBattlefield(gs, weak) {
		t.Fatalf("resolution ignored the hat-announced target: Weak Goblin survived")
	}
	if !permanentOnBattlefield(gs, strong) {
		t.Fatalf("resolution fell back to the policy pick (Big Dragon died) instead of honoring the hat-announced target")
	}
}

// TestAnnounce_WardFiresOnLivePath: ward (§702.21) keys off item.Targets,
// which the lazy path always left empty — ward was a free pass for every
// AI-cast removal spell. With announcement-time stamping, an unpayable
// ward cost must counter the spell and the target must survive.
func TestAnnounce_WardFiresOnLivePath(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2 // exactly the spell's cost: nothing left for ward

	warded := addCreature(gs, 1, "Warded Wurm", 6, 6, "ward")
	warded.Flags["ward_cost"] = 2

	doom := addHandCardWithEffect(gs, 0, "Doom Blade", 2,
		&gameast.Destroy{
			Target: gameast.Filter{Base: "creature", Targeted: true},
		}, "instant")

	if err := CastSpell(gs, 0, doom, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if !permanentOnBattlefield(gs, warded) {
		t.Fatalf("ward did not protect: Warded Wurm was destroyed (ward never fired on the live path)")
	}
	if countEvents(gs, "ward_counter") == 0 {
		t.Fatalf("expected a ward_counter event (unpayable ward counters the spell per §702.21c)")
	}
}

// TestAnnounce_NoLegalTargetKeepsLazyFizzle: when no legal target exists at
// announcement, Targets stays empty and the r61 PR-3 lazy fizzle gate
// applies unchanged — the cast itself still succeeds (card is spent).
func TestAnnounce_NoLegalTargetKeepsLazyFizzle(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	// No creatures anywhere.
	doom := addHandCardWithEffect(gs, 0, "Doom Blade", 2,
		&gameast.Destroy{
			Target: gameast.Filter{Base: "creature", Targeted: true},
		}, "instant")

	if err := CastSpell(gs, 0, doom, nil); err != nil {
		t.Fatalf("CastSpell should still succeed with no legal target (lazy fizzle handles it): %v", err)
	}
	if countEvents(gs, "fizzle") == 0 {
		t.Fatalf("expected the §608.2b fizzle event on resolution")
	}
	inGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == doom {
			inGrave = true
		}
	}
	if !inGrave {
		t.Fatalf("fizzled spell should be in owner's graveyard")
	}
}

// TestAnnounce_ActivatedAbilityHonorsHatChoice pins the §602.2b half: an
// activated ability with a required single target consults the Hat at
// activation and resolution honors the choice.
func TestAnnounce_ActivatedAbilityHonorsHatChoice(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	weak := addCreature(gs, 1, "Weak Goblin", 1, 1)
	strong := addCreature(gs, 1, "Big Dragon", 6, 6)

	gs.Seats[0].Hat = &announceStubHat{pick: func(_ *GameState, _ int, _ gameast.Filter, legal []Target) Target {
		for _, l := range legal {
			if l.Kind == TargetKindPermanent && l.Permanent == weak {
				return l
			}
		}
		return Target{}
	}}

	// "{1}: Destroy target creature" on an artifact.
	pinger := addBattlefield(gs, 0, "Murder Engine", 0, 0, "artifact")
	pinger.Card.AST = &gameast.CardAST{
		Name: "Murder Engine",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Mana: &gameast.ManaCost{Symbols: []gameast.ManaSymbol{{Generic: 1}}}},
				Effect: &gameast.Destroy{
					Target: gameast.Filter{Base: "creature", Targeted: true},
				},
			},
		},
	}

	if err := ActivateAbility(gs, 0, pinger, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	if permanentOnBattlefield(gs, weak) {
		t.Fatalf("activated ability ignored the hat-announced target: Weak Goblin survived")
	}
	if !permanentOnBattlefield(gs, strong) {
		t.Fatalf("activated ability fell back to policy (Big Dragon died) instead of honoring the hat choice")
	}
}
