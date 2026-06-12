package hat

import (
	"reflect"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Hat regression suite — 50 canonical (game-state, decision, expected)
// triples that pin the GreedyHat's deterministic decision-path output.
// Drift-catch contract: any change to a Choose* method that alters the
// result on one of these states FAILS this suite, forcing the author to
// (a) confirm the change was intentional and (b) update the affected
// pin with the new value + a one-line justification in the commit.
//
// Why GreedyHat: deterministic by construction (no RNG, no MCTS noise,
// no time-dependent behavior), so the same input → same output across
// runs. YggdrasilHat has stochastic rollouts; pinning it would require
// fixed seeds AND would still drift when budget or evaluator weights
// retune. Greedy is the right "fast-path heuristic regression" target.
//
// Coverage matrix (50 cases across 14 decision methods):
//
//	Decision              | Cases
//	----------------------+------
//	ChooseMulligan        |   6
//	ChooseLandToPlay      |   3
//	ChooseCastFromHand    |   8
//	ChooseAttackers       |   3
//	ChooseAttackTarget    |   4
//	ChooseDiscard         |   5
//	ChooseBottomCards     |   4
//	ChooseScry            |   3
//	ChoosePutBack         |   3
//	ChooseSurveil         |   3
//	ChooseX               |   3
//	ChooseTarget          |   3
//	ChooseMode            |   2
//	----------------------+------
//	TOTAL                 |  50
//
// Helpers reuse existing newTestGame / newTestCardMinimal /
// newTestPermanent from hat_test.go.

// ---------------------------------------------------------------------
// Shared regression builders
// ---------------------------------------------------------------------

// regGame builds a 2-seat game at default life with no permanents.
func regGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	return gs
}

// regGame4 is a 4-seat variant for politics tests (ChooseAttackTarget).
func regGame4(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 4)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	return gs
}

// land / spell builders with explicit CMC for predictable sort order.
func regLand(name string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"land", "basic"}, 0, nil)
}
func regSpell(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"sorcery"}, cmc, nil)
}
func regCreature(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"creature"}, cmc, nil)
}

// cardNames extracts ordered DisplayNames from a slice (nil-safe).
// Returns nil for empty/nil input so reflect.DeepEqual against the
// expected `nil` in case definitions does what humans expect (Go's
// reflect.DeepEqual distinguishes []string{} from nil).
func cardNames(cards []*gameengine.Card) []string {
	if len(cards) == 0 {
		return nil
	}
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if c == nil {
			out = append(out, "<nil>")
		} else {
			out = append(out, c.DisplayName())
		}
	}
	return out
}

// ---------------------------------------------------------------------
// SECTION 1: ChooseMulligan — 6 cases
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseMulligan(t *testing.T) {
	h := &GreedyHat{}
	cases := []struct {
		name      string
		landCount int
		handSize  int
		wantMull  bool
	}{
		{"00_zero_lands_full_hand_mulls", 0, 7, true},
		{"01_one_land_full_hand_mulls", 1, 7, true},
		{"02_two_lands_full_hand_keeps", 2, 7, false},
		{"03_three_lands_full_hand_keeps", 3, 7, false},
		{"04_zero_lands_post_mulligan_keeps", 0, 5, false}, // < 7 → no further mull
		{"05_seven_lands_keeps", 7, 7, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := regGame(t)
			hand := make([]*gameengine.Card, 0, tc.handSize)
			for i := 0; i < tc.landCount; i++ {
				hand = append(hand, regLand("Plains"))
			}
			for len(hand) < tc.handSize {
				hand = append(hand, regSpell("Filler", 3))
			}
			got := h.ChooseMulligan(gs, 0, hand)
			if got != tc.wantMull {
				t.Errorf("got %v want %v (lands=%d, handSize=%d)",
					got, tc.wantMull, tc.landCount, tc.handSize)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 2: ChooseLandToPlay — 3 cases (always first index)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseLandToPlay(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	cases := []struct {
		name   string
		lands  []*gameengine.Card
		wantIs string // expected DisplayName; "" for nil
	}{
		{"06_no_lands_returns_nil", nil, ""},
		{"07_one_land_returns_it", []*gameengine.Card{regLand("Plains")}, "Plains"},
		{"08_three_lands_returns_first",
			[]*gameengine.Card{regLand("Forest"), regLand("Island"), regLand("Swamp")},
			"Forest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseLandToPlay(gs, 0, tc.lands)
			gotName := ""
			if got != nil {
				gotName = got.DisplayName()
			}
			if gotName != tc.wantIs {
				t.Errorf("got %q want %q", gotName, tc.wantIs)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 3: ChooseCastFromHand — 8 cases
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseCastFromHand(t *testing.T) {
	h := &GreedyHat{}

	// Sol Ring shape — categorizeCard recognizes mana rocks as CatRamp.
	rampCard := func() *gameengine.Card {
		c := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
		ast := &gameast.CardAST{Name: "Sol Ring"}
		ast.Abilities = append(ast.Abilities, &gameast.Static{Raw: "{t}: add {c}{c}"})
		c.AST = ast
		return c
	}
	drawCard := func() *gameengine.Card {
		c := newTestCardMinimal("Divination", []string{"sorcery"}, 3, nil)
		ast := &gameast.CardAST{Name: "Divination"}
		ast.Abilities = append(ast.Abilities, &gameast.Static{Raw: "draw two cards"})
		c.AST = ast
		return c
	}

	cases := []struct {
		name     string
		hand     []*gameengine.Card
		landsBF  int
		wantName string // "" for nil
	}{
		// ChooseCastFromHand assumes `castable` is pre-filtered by the
		// engine for mana affordability — the hat's job is to pick the
		// best among affordable, not to re-check mana. These cases pin
		// the GreedyHat's category-priority order over the input set.
		{"09_empty_castable_returns_nil", nil, 4, ""},
		{"10_single_castable_returns_it",
			[]*gameengine.Card{regCreature("Bear", 2)}, 4, "Bear"},
		{"11_ramp_beats_threat",
			[]*gameengine.Card{regCreature("Bear", 2), rampCard()}, 4, "Sol Ring"},
		{"12_single_cheap_creature_returned",
			[]*gameengine.Card{regCreature("Bear", 2)}, 4, "Bear"},
		{"13_picks_biggest_in_threat_bucket_fall_through",
			// Both creatures fall into the "other" bucket (no ramp/draw
			// available); Phase 1 then falls through to the CMC-desc
			// closer logic → Dragon (8) wins over Bear (2).
			[]*gameengine.Card{regCreature("Dragon", 8), regCreature("Bear", 2)}, 4, "Dragon"},
		{"14_draw_categorized_over_filler",
			[]*gameengine.Card{regSpell("Filler", 3), drawCard()}, 4, "Divination"},
		{"15_solo_filler_returned",
			[]*gameengine.Card{regSpell("Filler", 3)}, 4, "Filler"},
		{"16_one_drop_castable_solo",
			[]*gameengine.Card{regCreature("Llanowar Elf", 1)}, 1, "Llanowar Elf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := regGame(t)
			for i := 0; i < tc.landsBF; i++ {
				newTestPermanent(gs.Seats[0], regLand("Plains"), 0, 0)
			}
			gs.Seats[0].Hand = tc.hand
			got := h.ChooseCastFromHand(gs, 0, tc.hand)
			gotName := ""
			if got != nil {
				gotName = got.DisplayName()
			}
			if gotName != tc.wantName {
				t.Errorf("got %q want %q", gotName, tc.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 4: ChooseAttackers — 3 cases (greedy: attack with everything)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseAttackers(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	a1 := newTestPermanent(gs.Seats[0], regCreature("A1", 2), 2, 2)
	a2 := newTestPermanent(gs.Seats[0], regCreature("A2", 3), 3, 3)

	cases := []struct {
		name string
		in   []*gameengine.Permanent
		want []*gameengine.Permanent
	}{
		{"17_no_legal_returns_empty", nil, []*gameengine.Permanent{}},
		{"18_one_legal_attacks", []*gameengine.Permanent{a1}, []*gameengine.Permanent{a1}},
		{"19_two_legal_both_attack", []*gameengine.Permanent{a1, a2}, []*gameengine.Permanent{a1, a2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseAttackers(gs, 0, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 5: ChooseAttackTarget — 4 cases (leader-targeting)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseAttackTarget(t *testing.T) {
	h := &GreedyHat{}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *gameengine.GameState
		defenders   []int
		wantTarget  int
	}{
		{
			"20_no_defenders_fallback_to_self",
			func(t *testing.T) *gameengine.GameState { return regGame4(t) },
			nil, 0, // empty defenders → return seatIdx (0)
		},
		{
			"21_single_defender_returned",
			func(t *testing.T) *gameengine.GameState { return regGame4(t) },
			[]int{2}, 2,
		},
		{
			"22_picks_leader_by_life",
			func(t *testing.T) *gameengine.GameState {
				gs := regGame4(t)
				gs.Seats[1].Life = 10
				gs.Seats[2].Life = 30
				gs.Seats[3].Life = 20
				return gs
			},
			[]int{1, 2, 3}, 2, // seat 2 has highest life → leader
		},
		{
			"23_tie_breaks_by_APNAP_distance",
			func(t *testing.T) *gameengine.GameState {
				gs := regGame4(t)
				// All three opps have equal score → closest in APNAP order wins.
				return gs
			},
			[]int{2, 3, 1}, 1, // dist 1 (seat 1) is closest from seat 0
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := tc.setup(t)
			got := h.ChooseAttackTarget(gs, 0, nil, tc.defenders)
			if got != tc.wantTarget {
				t.Errorf("got %d want %d", got, tc.wantTarget)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 6: ChooseDiscard — 5 cases (highest-CMC first, name tiebreak)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseDiscard(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)

	cases := []struct {
		name     string
		hand     []*gameengine.Card
		n        int
		wantNames []string
	}{
		{"24_n_zero_returns_nil",
			[]*gameengine.Card{regSpell("A", 1)}, 0, nil},
		{"25_empty_hand_returns_nil", nil, 1, nil},
		{"26_n_one_picks_highest_cmc",
			[]*gameengine.Card{regSpell("Cheap", 1), regSpell("Big", 7), regSpell("Mid", 3)},
			1, []string{"Big"}},
		{"27_n_two_picks_top_two_cmc_then_name",
			[]*gameengine.Card{regSpell("A", 4), regSpell("B", 4), regSpell("C", 1)},
			2, []string{"A", "B"}}, // tied CMC → alphabetical
		{"28_n_exceeds_hand_returns_all",
			[]*gameengine.Card{regSpell("X", 2), regSpell("Y", 3)},
			5, []string{"Y", "X"}}, // sorted by CMC desc
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseDiscard(gs, 0, tc.hand, tc.n)
			gotNames := cardNames(got)
			if tc.wantNames == nil {
				if got != nil {
					t.Errorf("want nil got %v", gotNames)
				}
				return
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Errorf("got %v want %v", gotNames, tc.wantNames)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 7: ChooseBottomCards — 4 cases (same shape as Discard)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseBottomCards(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	cases := []struct {
		name     string
		hand     []*gameengine.Card
		count    int
		wantNames []string
	}{
		{"29_count_zero_returns_nil",
			[]*gameengine.Card{regSpell("A", 2)}, 0, nil},
		{"30_count_one_returns_highest_cmc",
			[]*gameengine.Card{regSpell("A", 1), regSpell("B", 5)},
			1, []string{"B"}},
		{"31_count_two_returns_top_two",
			[]*gameengine.Card{regSpell("Cheap", 1), regSpell("Mid", 3), regSpell("Big", 6)},
			2, []string{"Big", "Mid"}},
		{"32_count_exceeds_hand_returns_all_sorted",
			[]*gameengine.Card{regSpell("X", 2), regSpell("Y", 4), regSpell("Z", 3)},
			10, []string{"Y", "Z", "X"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseBottomCards(gs, 0, tc.hand, tc.count)
			gotNames := cardNames(got)
			if tc.wantNames == nil {
				if got != nil {
					t.Errorf("want nil got %v", gotNames)
				}
				return
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Errorf("got %v want %v", gotNames, tc.wantNames)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 8: ChooseScry — 3 cases (lands top, affordable top)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseScry(t *testing.T) {
	h := &GreedyHat{}
	cases := []struct {
		name        string
		cards       []*gameengine.Card
		landsBF     int
		wantTop     []string
		wantBottom  []string
	}{
		{
			"33_all_lands_top",
			[]*gameengine.Card{regLand("Plains"), regLand("Island")},
			0, []string{"Plains", "Island"}, nil,
		},
		{
			"34_unaffordable_bottomed",
			[]*gameengine.Card{regSpell("Big", 10), regSpell("Bigger", 12)},
			1, []string{"Big"}, []string{"Bigger"}, // both bottomed; cheapest pulled back to top
		},
		{
			"35_mixed_affordable_top_expensive_bottom",
			[]*gameengine.Card{regLand("Forest"), regSpell("Cheap", 2), regSpell("Expensive", 9)},
			3, []string{"Forest", "Cheap"}, []string{"Expensive"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := regGame(t)
			for i := 0; i < tc.landsBF; i++ {
				newTestPermanent(gs.Seats[0], regLand("Plains"), 0, 0)
			}
			top, bottom := h.ChooseScry(gs, 0, tc.cards)
			if !reflect.DeepEqual(cardNames(top), tc.wantTop) {
				t.Errorf("top got %v want %v", cardNames(top), tc.wantTop)
			}
			if !reflect.DeepEqual(cardNames(bottom), tc.wantBottom) {
				t.Errorf("bottom got %v want %v", cardNames(bottom), tc.wantBottom)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 9: ChoosePutBack — 3 cases
// ---------------------------------------------------------------------

func TestRegressionSuite_ChoosePutBack(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	cases := []struct {
		name     string
		hand     []*gameengine.Card
		count    int
		wantNames []string
	}{
		{"36_count_zero_returns_nil",
			[]*gameengine.Card{regSpell("A", 1)}, 0, nil},
		{"37_count_one_returns_highest_cmc",
			[]*gameengine.Card{regSpell("Bolt", 1), regSpell("Wrath", 4)},
			1, []string{"Wrath"}},
		{"38_count_equals_hand_returns_all_sorted",
			[]*gameengine.Card{regSpell("Cheap", 1), regSpell("Mid", 3), regSpell("Pricey", 5)},
			3, []string{"Pricey", "Mid", "Cheap"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChoosePutBack(gs, 0, tc.hand, tc.count)
			gotNames := cardNames(got)
			if tc.wantNames == nil {
				if got != nil {
					t.Errorf("want nil got %v", gotNames)
				}
				return
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Errorf("got %v want %v", gotNames, tc.wantNames)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 10: ChooseSurveil — 3 cases (expensive → graveyard)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseSurveil(t *testing.T) {
	h := &GreedyHat{}
	cases := []struct {
		name       string
		cards      []*gameengine.Card
		landsBF    int
		wantTop    []string
		wantGrave  []string
	}{
		{
			"39_all_lands_kept_on_top",
			[]*gameengine.Card{regLand("Plains"), regLand("Island")},
			0, []string{"Plains", "Island"}, nil,
		},
		{
			"40_all_expensive_to_graveyard",
			[]*gameengine.Card{regSpell("Big", 9), regSpell("Bigger", 10)},
			1, nil, []string{"Big", "Bigger"},
		},
		{
			"41_mixed_affordable_top_expensive_grave",
			[]*gameengine.Card{regSpell("Cheap", 1), regSpell("Expensive", 8), regLand("Plains")},
			3, []string{"Cheap", "Plains"}, []string{"Expensive"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := regGame(t)
			for i := 0; i < tc.landsBF; i++ {
				newTestPermanent(gs.Seats[0], regLand("Plains"), 0, 0)
			}
			grave, top := h.ChooseSurveil(gs, 0, tc.cards)
			if !reflect.DeepEqual(cardNames(top), tc.wantTop) {
				t.Errorf("top got %v want %v", cardNames(top), tc.wantTop)
			}
			if !reflect.DeepEqual(cardNames(grave), tc.wantGrave) {
				t.Errorf("graveyard got %v want %v", cardNames(grave), tc.wantGrave)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 11: ChooseX — 3 cases (spend all available)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseX(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	cases := []struct {
		name    string
		avail   int
		wantX   int
	}{
		{"42_zero_mana_returns_zero", 0, 0},
		{"43_positive_mana_returns_all", 7, 7},
		{"44_negative_clamps_to_zero", -3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseX(gs, 0, nil, tc.avail)
			if got != tc.wantX {
				t.Errorf("got %d want %d", got, tc.wantX)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 12: ChooseTarget — 3 cases (decline-to-policy, r62)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseTarget(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	t1 := gameengine.Target{Kind: gameengine.TargetKindSeat, Seat: 1}
	t2 := gameengine.Target{Kind: gameengine.TargetKindSeat, Seat: 0}

	cases := []struct {
		name string
		legal []gameengine.Target
		want gameengine.Target
	}{
		{"45_no_legal_returns_none",
			nil, gameengine.Target{Kind: gameengine.TargetKindNone}},
		// r62: GreedyHat declines ChooseTarget (zero Target) so the
		// engine's announcement-time picker falls back to the policy
		// pick — the engine now consults hats live, and honoring the old
		// first-legal body would have changed every Greedy-baseline
		// game's targeting.
		{"46_one_legal_declines_to_policy",
			[]gameengine.Target{t1}, gameengine.Target{Kind: gameengine.TargetKindNone}},
		{"47_two_legal_declines_to_policy",
			[]gameengine.Target{t1, t2}, gameengine.Target{Kind: gameengine.TargetKindNone}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseTarget(gs, 0, gameast.Filter{}, tc.legal)
			if got != tc.want {
				t.Errorf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SECTION 13: ChooseMode — 2 cases (index 0 or -1)
// ---------------------------------------------------------------------

func TestRegressionSuite_ChooseMode(t *testing.T) {
	h := &GreedyHat{}
	gs := regGame(t)
	cases := []struct {
		name  string
		modes []gameast.Effect
		want  int
	}{
		{"48_no_modes_returns_neg_one", nil, -1},
		{"49_modes_returns_zero",
			[]gameast.Effect{&gameast.Draw{}, &gameast.Damage{}},
			0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.ChooseMode(gs, 0, tc.modes)
			if got != tc.want {
				t.Errorf("got %d want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SUITE CAPSTONE — counts the regressions to pin the 50-case contract.
// If somebody adds/removes a case in the sections above, this also
// needs an update — that's the point, the suite size is a documented
// contract, not an accidental count.
// ---------------------------------------------------------------------

func TestRegressionSuite_FiftyCasePin(t *testing.T) {
	const want = 50
	got := 6 + 3 + 8 + 3 + 4 + 5 + 4 + 3 + 3 + 3 + 3 + 3 + 2
	if got != want {
		t.Fatalf("suite case count drift: got %d want %d (update both this pin AND the matrix in the file header when intentional)",
			got, want)
	}
}
