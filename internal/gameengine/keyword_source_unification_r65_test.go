package gameengine

import "testing"

// ---------------------------------------------------------------------------
// r65 — the keyword authority was split between two queries, neither of
// which was a superset of the other:
//
//	p.HasKeyword    — AST + GrantedAbilities + Flags["kw:"] + counters,
//	                  but layer-BLIND.
//	gs.HasKeywordOf — layer-correct, but BaseCharacteristics never folded
//	                  in Flags["kw:"] or keyword counters.
//
// These pin the fold: the runtime surfaces now reach chars.Keywords, so
// the layer-aware query can see everything the layer-blind one sees.
// ---------------------------------------------------------------------------

func r65Perm(gs *GameState, name string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

func TestKeywordUnificationR65_FlagStampReachesLayerQuery(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Flag Bear")
	p.Flags = map[string]int{"kw:flying": 1}
	gs.InvalidateCharacteristicsCache()

	if !p.HasKeyword("flying") {
		t.Fatal("setup: the flag surface should grant flying")
	}
	if !gs.HasKeywordOf(p, "flying") {
		t.Error("the layer-aware query must see a Flags-stamped keyword")
	}
}

func TestKeywordUnificationR65_KeywordCounterReachesLayerQuery(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Counter Bear")
	p.Counters = map[string]int{"flying": 1}
	gs.InvalidateCharacteristicsCache()

	if !gs.HasKeywordOf(p, "flying") {
		t.Error("the layer-aware query must see a flying counter (CR §122.1b)")
	}
}

// Don't-over-fold control: a +1/+1 counter is not a keyword, and must
// never appear in chars.Keywords. p.HasKeyword's raw-map check would
// answer true for "+1/+1"; the fold deliberately goes through the
// counters registry instead.
func TestKeywordUnificationR65_PTCounterIsNotAKeyword(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Fat Bear")
	p.Counters = map[string]int{"+1/+1": 3}
	gs.InvalidateCharacteristicsCache()

	for _, k := range GetEffectiveCharacteristics(gs, p).Keywords {
		if k == "+1/+1" {
			t.Fatal("a +1/+1 counter must not be folded into chars.Keywords")
		}
	}
}

// Don't-over-fold control: a provenance tag such as
// "kw:menace_from_gisa" records WHO granted a keyword. It must be
// folded verbatim (so it cannot satisfy a bare HasKeyword("menace")),
// exactly as the flag surface already treats it.
func TestKeywordUnificationR65_ProvenanceTagIsNotTheKeyword(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Tagged Bear")
	p.Flags = map[string]int{"kw:menace_from_gisa": 1}
	gs.InvalidateCharacteristicsCache()

	if gs.HasKeywordOf(p, "menace") {
		t.Error("a provenance tag must not be reinterpreted as the bare keyword")
	}
	if p.HasKeyword("menace") {
		t.Error("parity: the flag surface does not grant menace here either")
	}
}

// The fold is at the layer BASE, so a layer-6 "loses all abilities"
// effect strips a flag-granted keyword too — the Humility phantom the
// invariants note describes.
func TestKeywordUnificationR65_FoldSitsUnderTheLayers(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Humbled Bear")
	p.Flags = map[string]int{"kw:flying": 1}
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(p, "flying") {
		t.Fatal("setup: flag-granted flying should be visible")
	}

	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:          LayerAbility,
		SourcePerm:     p,
		SourceCardName: "Humility",
		HandlerID:      "r65-humility-probe",
		Duration:       DurationPermanent,
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Keywords = nil
			chars.Abilities = nil
		},
	})
	gs.InvalidateCharacteristicsCache()

	if gs.HasKeywordOf(p, "flying") {
		t.Error("a layer-6 ability strip must remove a flag-granted keyword")
	}
}

// Determinism: chars.Keywords feeds the LayerIdempotency invariant,
// which compares two independently computed slices for equality. Go map
// iteration is randomised, so the fold sorts.
func TestKeywordUnificationR65_FoldOrderIsDeterministic(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := r65Perm(gs, "Many Bear")
	p.Flags = map[string]int{
		"kw:flying": 1, "kw:trample": 1, "kw:lifelink": 1,
		"kw:vigilance": 1, "kw:menace": 1, "kw:reach": 1,
	}
	first := runtimeKeywordSources(p)
	for i := 0; i < 50; i++ {
		got := runtimeKeywordSources(p)
		if len(got) != len(first) {
			t.Fatalf("length drift: %v vs %v", got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("order drift on pass %d: %v vs %v", i, got, first)
			}
		}
	}
}
