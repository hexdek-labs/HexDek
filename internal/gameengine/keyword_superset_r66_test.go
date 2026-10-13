package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// r66 — the superset contract.
//
// r65 asserted that gs.HasKeywordOf had become a strict superset of
// p.HasKeyword, and that assertion was the whole basis for migrating
// call sites to the layer-aware query. It was false in four ways. These
// tests pin the three that were real gaps, and pin the fourth as a
// deliberate and documented DIVERGENCE rather than letting it read as
// an oversight.
//
// Each of A/B/C fails before keyword_source_unification_r66.go.

func r66Perm(gs *GameState, name string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	gs.InvalidateCharacteristicsCache()
	return p
}

// assertSuperset is the contract itself: whatever the layer-blind query
// reports as present, the layer-aware query must also report. The
// converse is NOT asserted — the layer query legitimately sees more
// (§613 grants) and legitimately sees less (a Humility strip).
func assertSuperset(t *testing.T, gs *GameState, p *Permanent, kw string) {
	t.Helper()
	if !p.HasKeyword(kw) {
		t.Fatalf("precondition: p.HasKeyword(%q) = false, test proves nothing", kw)
	}
	if !gs.HasKeywordOf(p, kw) {
		t.Errorf("superset violated: p.HasKeyword(%q)=true but gs.HasKeywordOf=false; chars.Keywords=%v",
			kw, GetEffectiveCharacteristics(gs, p).Keywords)
	}
}

// A — level-up band keywords are stored under a "level_bracket:" prefix
// (CR §711.2) so a band change strips exactly its own grants.
// HasKeyword trims the prefix; the layer base folded it verbatim.
func TestKeywordSupersetR66_LevelBracketGrantReachesLayerQuery(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r66Perm(gs, "Banded Leveler")
	p.GrantedAbilities = append(p.GrantedAbilities, "level_bracket:flying")
	gs.InvalidateCharacteristicsCache()

	assertSuperset(t, gs, p, "flying")

	// And the prefixed form must NOT itself read as a keyword — it is
	// bookkeeping about the grant's origin, not a keyword name.
	if gs.HasKeywordOf(p, "level_bracket:flying") {
		t.Error("the storage prefix leaked into the layer query as a keyword name")
	}
}

// B — keyword counters living in the CounterStacks store. HasKeyword
// reaches these through HasKeywordCounter; r65 folded only the legacy
// Permanent.Counters map.
func TestKeywordSupersetR66_CounterStackKeywordReachesLayerQuery(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r66Perm(gs, "Stacked Flyer")
	p.CounterStacks = append(p.CounterStacks, counters.CounterStack{Type: "flying", Count: 1})
	gs.InvalidateCharacteristicsCache()

	assertSuperset(t, gs, p, "flying")
}

// C — a DIVERGENT grant in that same store: a phyresis counter confers
// infect (CR §702.91), a keyword whose name differs from the counter's.
// Only a registry lookup finds it; a name match does not.
func TestKeywordSupersetR66_DivergentCounterStackGrantReachesLayerQuery(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r66Perm(gs, "Phyrexian Subject")
	p.CounterStacks = append(p.CounterStacks, counters.CounterStack{Type: "phyresis", Count: 1})
	gs.InvalidateCharacteristicsCache()

	assertSuperset(t, gs, p, "infect")

	// The counter's own type name is not a keyword.
	if gs.HasKeywordOf(p, "phyresis") {
		t.Error("counter TYPE name folded as a keyword; only the conferred keyword should fold")
	}
}

// D — the documented divergence. HasKeyword's raw `p.Counters[want] > 0`
// check answers true for ANY counter kind queried by its own name,
// including "+1/+1". That is a defect in the layer-blind query, not a
// gap in the fold: "+1/+1" is not a keyword and must never appear in
// chars.Keywords. Pinned so that if someone later "fixes the superset"
// by folding every counter kind, this fails and explains why not.
func TestKeywordSupersetR66_PlusOneCounterIsNotAKeyword(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r66Perm(gs, "Counted Bear")
	p.Counters = map[string]int{"+1/+1": 3}
	gs.InvalidateCharacteristicsCache()

	if !p.HasKeyword("+1/+1") {
		t.Log("note: the layer-blind query no longer reports +1/+1 as a keyword — " +
			"if that was fixed deliberately, this test can be retired")
	}
	if gs.HasKeywordOf(p, "+1/+1") {
		t.Error(`"+1/+1" folded into chars.Keywords; the fold must stay registry-resolved`)
	}
	for _, k := range GetEffectiveCharacteristics(gs, p).Keywords {
		if k == "+1/+1" {
			t.Error(`"+1/+1" present in chars.Keywords`)
		}
	}
}

// The fold must still sit UNDER the layers: a layer-6 strip has to
// reach a keyword that arrived from one of the newly folded surfaces,
// or r66 has merely recreated the split one level down.
func TestKeywordSupersetR66_NewSurfacesSitUnderTheLayers(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r66Perm(gs, "Humbled Leveler")
	p.GrantedAbilities = append(p.GrantedAbilities, "level_bracket:flying")
	p.CounterStacks = append(p.CounterStacks, counters.CounterStack{Type: "flying", Count: 1})
	gs.InvalidateCharacteristicsCache()

	if !gs.HasKeywordOf(p, "flying") {
		t.Fatal("precondition: expected flying before the strip")
	}

	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:     LayerAbility,
		HandlerID: "r66_test_humility",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, target *Permanent) bool { return target == p },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Keywords = nil
			chars.Abilities = nil
			chars.LostAllAbilities = true
		},
	})
	gs.InvalidateCharacteristicsCache()

	if gs.HasKeywordOf(p, "flying") {
		t.Error("layer-6 strip did not reach the r66-folded surfaces — the fold is sitting above the layers")
	}
}

// r66 — the behaviour change, at the level it actually matters.
//
// keywordActive() was a union of the two keyword queries, written in
// r63 when neither was a superset. With the superset contract above in
// place, the raw arm can only contribute where the layer query said NO
// — which is exactly a layer-6 "loses all abilities" strip. So the
// union's only remaining effect was to make Humility, Lignify and
// Darksteel Mutation fail to remove evasion in combat (CR §613.1f).
//
// This pins the fix end-to-end through the real block-legality path
// rather than through the keyword query alone.
func TestKeywordSupersetR66_HumilityStripRemovesEvasionInCombat(t *testing.T) {
	gs := newFixtureGame(t)

	// A flyer whose flying comes from the Flags stamp — the surface
	// per_card handlers actually use, and the one that had no owner
	// and no duration.
	atk := addBattlefield(gs, 0, "Stamped Flyer", 2, 2, "creature")
	atk.Flags = map[string]int{"kw:flying": 1}
	ground := addBattlefield(gs, 1, "Ground Bear", 2, 2, "creature")
	gs.InvalidateCharacteristicsCache()

	if CanBlockGS(gs, atk, ground) {
		t.Fatal("precondition: a ground creature must not be able to block a flyer")
	}

	// Humility-style layer-6 strip.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:     LayerAbility,
		HandlerID: "r66_test_humility_combat",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, target *Permanent) bool { return target == atk },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Keywords = nil
			chars.Abilities = nil
			chars.LostAllAbilities = true
		},
	})
	gs.InvalidateCharacteristicsCache()

	if !CanBlockGS(gs, atk, ground) {
		t.Error("stripped creature kept flying in combat: the keyword union is still " +
			"re-adding what a layer-6 effect removed")
	}
}

// Don't-over-correct control: collapsing the union must not cost
// combat the keywords it could already see. A §613 layer grant with no
// strip in play still confers evasion.
func TestKeywordSupersetR66_LayerGrantStillConfersEvasion(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Grounded Bear", 2, 2, "creature")
	ground := addBattlefield(gs, 1, "Other Bear", 2, 2, "creature")
	gs.InvalidateCharacteristicsCache()

	if !CanBlockGS(gs, atk, ground) {
		t.Fatal("precondition: two ground creatures, blocking must be legal")
	}

	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:     LayerAbility,
		HandlerID: "r66_test_grant_flying",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, target *Permanent) bool { return target == atk },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Keywords = append(chars.Keywords, "flying")
		},
	})
	gs.InvalidateCharacteristicsCache()

	if CanBlockGS(gs, atk, ground) {
		t.Error("a layer-granted flying did not reach combat")
	}
}
