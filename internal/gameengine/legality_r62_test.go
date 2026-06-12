package gameengine

// Regression tests for the ride-along rules-legality validator (r62,
// legality.go — owner design from 7174n1c).
//
// The validator watches each cast/activation AT THE MOMENT IT HAPPENS and
// re-derives legality independently of the engine's own gates. These tests
// pin three things:
//
//  1. DEFAULT OFF is really off: gs.Legality == nil changes nothing.
//  2. Each phase-1 check CATCHES its live bug class — including two
//     engine-allowed-but-illegal cast shapes that exist on main today
//     (CastSpell's sorcery gate never checks the caster is the ACTIVE
//     player, and non-flash permanent spells aren't timing-gated at all),
//     which is the "illegal casts observed while spectating" class.
//  3. Legal play produces ZERO violations (no false positives), including
//     the deliberate replay of the Chalice multikicker scenario and
//     mana-ability activations inside the cost window.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func legalityFixture(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.Seed = 7
	gs.Phase = "main"
	gs.Active = 0
	gs.Legality = NewLegalityValidator(gs.Seed)
	return gs
}

func violationsByRule(v *LegalityValidator, rule string) []LegalityViolation {
	var out []LegalityViolation
	for _, viol := range v.Violations {
		if viol.Rule == rule {
			out = append(out, viol)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 1. Default off
// ---------------------------------------------------------------------------

// With gs.Legality nil every hook is a nil-receiver no-op: casting works
// exactly as before and nothing accumulates anywhere. (The broader
// zero-behavior-change claim is pinned by the entire existing suite
// running with the field unset.)
func TestLegality_DefaultOff_NoOp(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.Phase = "main"
	if gs.Legality != nil {
		t.Fatal("validator must be nil by default")
	}
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 3
	card := &Card{Name: "Vanilla", Owner: 0, Types: []string{"creature", "cost:3"}, BasePower: 2, BaseToughness: 2}
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell with nil validator failed: %v", err)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("creature should be on battlefield, got %d perms", len(gs.Seats[0].Battlefield))
	}
}

// Rollout clones must NOT carry the validator — hypothetical MCTS lines
// would pollute the violation stream.
func TestLegality_RolloutCloneDropsValidator(t *testing.T) {
	gs := legalityFixture(t)
	clone := gs.CloneForRollout(rand.New(rand.NewSource(8)))
	if clone.Legality != nil {
		t.Error("CloneForRollout must not copy the Legality validator")
	}
}

// ---------------------------------------------------------------------------
// 2. Timing (CR §307.1 / §117.1a) — the "illegal casts" class
// ---------------------------------------------------------------------------

// A sorcery cast by the NON-ACTIVE player. The engine's own CastSpell gate
// checks phase + stack emptiness but never that the caster is the active
// player, so the cast goes through — and the validator must flag it.
// This is a live engine-allowed-but-illegal action on main today.
func TestLegality_Timing_NonActiveSorcery_Flagged(t *testing.T) {
	gs := legalityFixture(t)
	gs.Active = 0
	gs.Seats[1].Hat = &GreedyHatStub{}
	gs.Seats[1].ManaPool = 2
	card := &Card{Name: "Off-Turn Divination", Owner: 1, Types: []string{"sorcery", "cost:2"}}
	gs.Seats[1].Hand = []*Card{card}

	if err := CastSpell(gs, 1, card, nil); err != nil {
		t.Fatalf("engine rejected the cast (gate changed?): %v — update this test's premise", err)
	}
	viols := violationsByRule(gs.Legality, "307.1")
	if len(viols) == 0 {
		t.Fatalf("non-active-player sorcery cast not flagged; violations=%v", gs.Legality.Violations)
	}
	v := viols[0]
	if v.Seat != 1 || v.Seed != 7 || v.Action != "cast:Off-Turn Divination" {
		t.Errorf("violation repro fields wrong: %+v", v)
	}
}

// A non-flash CREATURE cast during combat. CastSpell timing-gates only
// literal sorceries, so this goes through — the validator must flag the
// §117.1a sorcery-timing violation for permanent spells.
func TestLegality_Timing_CreatureAtInstantSpeed_Flagged(t *testing.T) {
	gs := legalityFixture(t)
	gs.Phase = "combat"
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 3
	card := &Card{Name: "Combat Bear", Owner: 0, Types: []string{"creature", "cost:3"}}
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("engine rejected the cast (gate changed?): %v — update this test's premise", err)
	}
	if len(violationsByRule(gs.Legality, "117.1a")) == 0 {
		t.Fatalf("non-flash creature cast in combat not flagged; violations=%v", gs.Legality.Violations)
	}
}

// A FLASH creature in combat is legal — zero violations.
func TestLegality_Timing_FlashCreature_Clean(t *testing.T) {
	gs := legalityFixture(t)
	gs.Phase = "combat"
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 3
	card := &Card{
		Name: "Ambush Cat", Owner: 0, Types: []string{"creature", "cost:3"},
		AST: &gameast.CardAST{Name: "Ambush Cat", Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "flash"},
		}},
	}
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("flash creature in combat should be clean, got %d violations: %v", n, gs.Legality.Violations)
	}
}

// ---------------------------------------------------------------------------
// 3. Targets (CR §608.2c / §601.2c)
// ---------------------------------------------------------------------------

// "Destroy target creature" handed a NON-creature target. The engine's
// ValidateTargetsAtAnnouncement checks existence + protection but never
// filter satisfaction, so the cast goes through — the validator must flag
// the filter mismatch. (Live invalid-target class from spectating.)
func TestLegality_Targets_FilterMismatch_Flagged(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 2

	// An artifact (non-creature) on the opponent's battlefield.
	artifact := &Permanent{
		Card:       &Card{Name: "Mana Vault", Owner: 1, Types: []string{"artifact"}},
		Controller: 1, Owner: 1,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, artifact)

	// "Destroy target creature" — spell body via the empty-cost Activated
	// pattern collectSpellEffect reads.
	card := &Card{
		Name: "Bad Murder", Owner: 0, Types: []string{"instant", "cost:2"},
		AST: &gameast.CardAST{Name: "Bad Murder", Abilities: []gameast.Ability{
			&gameast.Activated{Effect: &gameast.Destroy{
				Target: gameast.Filter{Base: "creature", Targeted: true},
			}},
		}},
	}
	gs.Seats[0].Hand = []*Card{card}

	err := CastSpell(gs, 0, card, []Target{{Kind: TargetKindPermanent, Permanent: artifact, Seat: 1}})
	if err != nil {
		t.Fatalf("engine rejected the cast (filter validation added upstream?): %v — update this test's premise", err)
	}
	viols := violationsByRule(gs.Legality, "601.2c")
	if len(viols) == 0 {
		t.Fatalf("creature-filter spell aimed at an artifact not flagged; violations=%v", gs.Legality.Violations)
	}
}

// The same spell aimed at a real creature is clean.
func TestLegality_Targets_FilterSatisfied_Clean(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 2
	bear := &Permanent{
		Card:       &Card{Name: "Bear", Owner: 1, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 1, Owner: 1,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, bear)
	card := &Card{
		Name: "Good Murder", Owner: 0, Types: []string{"instant", "cost:2"},
		AST: &gameast.CardAST{Name: "Good Murder", Abilities: []gameast.Ability{
			&gameast.Activated{Effect: &gameast.Destroy{
				Target: gameast.Filter{Base: "creature", Targeted: true},
			}},
		}},
	}
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, []Target{{Kind: TargetKindPermanent, Permanent: bear, Seat: 1}}); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("legal targeted kill should be clean, got %d violations: %v", n, gs.Legality.Violations)
	}
}

// ---------------------------------------------------------------------------
// 4. Cost paid (CR §601.2f-h) — including the Chalice multikicker replay
// ---------------------------------------------------------------------------

// Unit pins on the check itself: the two failure directions it must
// recognize, in the exact shape of the live Chalice double-pay report
// (announced base 0 + 2 kicks @ {2} = 4, but 6 left the pool) and a
// free-cast leak (announced 3, nothing left the pool).
func TestLegality_CostPaid_DetectsBothDirections(t *testing.T) {
	doublePay := &LegalityObservation{
		Kind: "cast", Seat: 0, TurnAtAnnounce: 3,
		Card:               &Card{Name: "Everflowing Chalice", Types: []string{"artifact", "cost:0"}, AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "multikicker", Args: []interface{}{float64(2)}}}}},
		PoolBefore:         6,
		PoolAfter:          0, // 6 spent — double-deducted kicks
		BaseCostAtAnnounce: 0,
		Item: &StackItem{CostMeta: map[string]interface{}{
			"kicked": true, "multikick_count": 2,
		}},
	}
	if v := checkLegalityCostPaid(nil, doublePay); len(v) != 1 {
		t.Fatalf("double-pay (announced 4, spent 6) not flagged: %v", v)
	} else if v[0].Rule != "601.2f-h" {
		t.Errorf("wrong rule: %s", v[0].Rule)
	}

	freeCast := &LegalityObservation{
		Kind: "cast", Seat: 2, TurnAtAnnounce: 5,
		Card:               &Card{Name: "Freebie", Types: []string{"sorcery", "cost:3"}},
		PoolBefore:         5,
		PoolAfter:          5, // nothing deducted
		BaseCostAtAnnounce: 3,
		Item:               &StackItem{},
	}
	if v := checkLegalityCostPaid(nil, freeCast); len(v) != 1 {
		t.Fatalf("under-pay (announced 3, spent 0) not flagged: %v", v)
	}
}

// Deliberate replay of the spectator-reported Chalice multikicker
// scenario end-to-end: cast the Chalice fixture with 2 kicks through the
// REAL CastSpell pipeline with the validator riding along. The engine's
// accounting is correct post-r61-PR-4, so this must be CLEAN — and if the
// double-pay ever regresses, this test is the tripwire that names it.
func TestLegality_ChaliceMultikicker_Replay_Clean(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].Hat = &kickHat{}
	gs.Seats[0].ManaPool = 4 // base 0 + 2 kicks @ {2}
	card := makeChaliceCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if got := violationsByRule(gs.Legality, "601.2f-h"); len(got) != 0 {
		t.Errorf("Chalice multikicker double-pay regressed (or check FPs): %v", got)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected exactly 4 mana spent, pool=%d", gs.Seats[0].ManaPool)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("kicked Chalice cast should be fully clean, got: %v", gs.Legality.Violations)
	}
}

// Buyback replay through the real pipeline: 2 base + 3 buyback must read
// as announced-and-paid 5, zero violations.
func TestLegality_Buyback_Replay_Clean(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 5
	card := instantSpellCard("Paid Whispers", 2, nil,
		&gameast.Keyword{Name: "buyback", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("paid buyback cast should be clean, got: %v", gs.Legality.Violations)
	}
}

// ---------------------------------------------------------------------------
// 5. Activations — cost window with inline mana abilities
// ---------------------------------------------------------------------------

// A tap-for-mana activation ADDS mana inside the observation window; the
// NoteManaAdd crediting must keep the cost check clean (expected spend 0).
func TestLegality_Activation_ManaAbility_Clean(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].Hat = &GreedyHatStub{}
	perm := &Permanent{
		Card: &Card{
			Name: "Test Llanowar", Owner: 0, Types: []string{"creature"},
			AST: &gameast.CardAST{Name: "Test Llanowar", Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Tap: true},
					Effect: &gameast.AddMana{AnyColorCount: 1},
				},
			}},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	if err := ActivateAbility(gs, 0, perm, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("mana-ability activation should be clean, got: %v", gs.Legality.Violations)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("expected 1 green in pool after tap, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// 6. Extensibility — registering a custom check
// ---------------------------------------------------------------------------

func TestLegality_RegisterCustomCheck(t *testing.T) {
	gs := legalityFixture(t)
	// Phase 3 widened the observation stream (zone_change / etb), so a
	// kind-agnostic check legitimately fires on more than just the cast.
	// Pin the Register contract precisely: exactly one CAST observation.
	gs.Legality.Register(LegalityCheck{
		Name: "always-fires",
		Fn: func(_ *GameState, obs *LegalityObservation) []LegalityViolation {
			if obs.Kind != "cast" {
				return nil
			}
			return []LegalityViolation{{
				Turn: obs.TurnAtAnnounce, Seat: obs.Seat,
				Action: obs.ActionLabel(), Rule: "custom-1", Detail: "tripwire",
			}}
		},
	})
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].ManaPool = 1
	card := &Card{Name: "Ping", Owner: 0, Types: []string{"instant", "cost:1"}}
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if len(violationsByRule(gs.Legality, "custom-1")) != 1 {
		t.Errorf("custom registered check did not fire: %v", gs.Legality.Violations)
	}
}
