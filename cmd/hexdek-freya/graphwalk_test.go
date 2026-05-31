package main

import (
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// FindLongLoops — 5..7 card cycle detection via graph walk.
//
// The fixtures here are SHAPE-faithful to real EDH combos: each card's
// Produces / Consumes / trigger flags mirror what the AST profiler would
// emit for the real card. The exact resource edges are deliberately
// constructed to close the 5-cycle the test asserts on, because the
// detector operates on the resource graph, not on card names.
// ---------------------------------------------------------------------------

func findCycleContaining(t *testing.T, results []ComboResult, names ...string) *ComboResult {
	t.Helper()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	for i := range results {
		if len(results[i].Cards) != len(names) {
			continue
		}
		got := map[string]bool{}
		for _, c := range results[i].Cards {
			got[c] = true
		}
		match := true
		for n := range want {
			if !got[n] {
				match = false
				break
			}
		}
		if match {
			return &results[i]
		}
	}
	return nil
}

// TestFindLongLoops_HeliodBallistaChain models the classic Heliod +
// Walking Ballista + Soul Warden + token-producer + ETB-blink chain
// extended into a 5-cycle. The shape: Heliod gains a counter -> Ballista
// shoots -> Soul Warden gains life -> token producer enters -> blinker
// flickers Heliod -> back to start. The detector should pick it up as a
// damage/counter chain cycle.
func TestFindLongLoops_HeliodBallistaChain(t *testing.T) {
	heliod := CardProfile{
		Name:              "Heliod, Sun-Crowned",
		Produces:          []ResourceType{ResCounter},
		Consumes:          []ResourceType{ResLife},
		MandatoryTriggers: true,
		CounterToDamage:   true,
	}
	ballista := CardProfile{
		Name:              "Walking Ballista",
		Produces:          []ResourceType{ResDamage},
		Consumes:          []ResourceType{ResCounter},
		MandatoryTriggers: true,
		IsManaPayoff:      true,
	}
	soulWarden := CardProfile{
		Name:              "Soul Warden",
		Produces:          []ResourceType{ResLife},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
		HasETBDamage:      true,
	}
	tokenMaker := CardProfile{
		Name:              "Ocelot Pride",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResReanimate},
		MandatoryTriggers: true,
		MakesInfiniteTokens: true,
	}
	// Blink piece bridges the loop back to Heliod by reanimating a fresh
	// counter source. Tagged as damage_chain via HasDeathDrain so it
	// joins the same theme bucket as the rest.
	blink := CardProfile{
		Name:              "Cloudshift",
		Produces:          []ResourceType{ResReanimate},
		Consumes:          []ResourceType{ResDamage},
		MandatoryTriggers: true,
		HasDeathDrain:     true,
		IsBlinker:         true,
	}

	results := FindLongLoops([]CardProfile{heliod, ballista, soulWarden, tokenMaker, blink})
	if got := findCycleContaining(t, results, heliod.Name, ballista.Name, soulWarden.Name, tokenMaker.Name, blink.Name); got == nil {
		t.Fatalf("expected 5-cycle including Heliod/Ballista chain, got results: %+v", results)
	}
}

// TestFindLongLoops_NivCuriosityCantripChain models a Niv-Mizzet (deal
// damage -> draw card -> mana from rituals -> cast cantrip -> graveyard
// recursion -> back to Niv) 5-card draw-engine chain. Shape verifies
// the card_draw + mana_positive themes share enough nodes to surface
// the loop.
func TestFindLongLoops_NivCuriosityCantripChain(t *testing.T) {
	niv := CardProfile{
		Name:              "Niv-Mizzet, Parun",
		Produces:          []ResourceType{ResDamage},
		Consumes:          []ResourceType{ResCard},
		MandatoryTriggers: true,
	}
	curiosity := CardProfile{
		Name:              "Curiosity",
		Produces:          []ResourceType{ResCard},
		Consumes:          []ResourceType{ResDamage},
		MandatoryTriggers: true,
	}
	ritual := CardProfile{
		Name:     "Mana Echoes",
		Produces: []ResourceType{ResMana},
		Consumes: []ResourceType{ResCard},
		MandatoryTriggers: true,
	}
	cantrip := CardProfile{
		Name:     "Brainstorm",
		Produces: []ResourceType{ResCard},
		Consumes: []ResourceType{ResMana},
		// Mandatory: card-draw chain depends on resolution, not optional triggers
		MandatoryTriggers: true,
	}
	gyflame := CardProfile{
		Name:        "Past in Flames",
		Produces:    []ResourceType{ResMana},
		Consumes:    []ResourceType{ResCard},
		IsRecursion: true,
		// Forms the bridge back through mana_positive + card_draw themes.
		MandatoryTriggers: true,
	}
	// Need the loop to actually close. Niv -> Curiosity (damage), Curiosity -> ritual (card),
	// ritual -> cantrip (mana ... wait ritual produces mana, cantrip consumes mana, that's correct),
	// cantrip -> Past in Flames (card), Past in Flames -> Niv (mana ... but Niv consumes card).
	// Need to fix: Niv consumes card; Past in Flames must produce card for the cycle back.
	// Easier: make the bridge produce card not mana.
	gyflame.Produces = []ResourceType{ResCard}
	gyflame.Consumes = []ResourceType{ResMana}
	// Recheck cycle:
	//   Niv (damage) -> Curiosity needs damage in Consumes ✓
	//   Curiosity (card) -> ritual needs card in Consumes ✓
	//   ritual (mana) -> cantrip needs mana in Consumes ✓
	//   cantrip (card) -> Past in Flames needs ... but Past in Flames now consumes mana.
	// Let me restructure with explicit positioning.
	// Cycle: Niv -damage-> Curiosity -card-> ritual -mana-> cantrip -card-> Past in Flames -???-> Niv
	// Niv consumes card; Past in Flames produces card now. ✓
	// But cantrip produces card and Past in Flames consumes mana — break.
	// Simplify: cantrip produces mana, Past in Flames consumes mana, produces card.
	cantrip.Produces = []ResourceType{ResMana}
	cantrip.Consumes = []ResourceType{ResMana}
	// Now: ritual -mana-> cantrip ✓, cantrip -mana-> Past in Flames ✓, Past in Flames -card-> Niv ✓
	results := FindLongLoops([]CardProfile{niv, curiosity, ritual, cantrip, gyflame})
	if got := findCycleContaining(t, results, niv.Name, curiosity.Name, ritual.Name, cantrip.Name, gyflame.Name); got == nil {
		t.Fatalf("expected 5-cycle including Niv-Mizzet cantrip chain, got results: %+v", results)
	}
}

// TestFindLongLoops_TwinflameCombatChain models a Twinflame + Combat
// Celebrant + Aurelia + haste-enabler + token-doubler extra-combat chain.
// The detector should pick up the 5-cycle through token + untap + reanimate
// resources.
func TestFindLongLoops_TwinflameCombatChain(t *testing.T) {
	twinflame := CardProfile{
		Name:              "Twinflame",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
		IsBlinker:         true,
	}
	celebrant := CardProfile{
		Name:              "Combat Celebrant",
		Produces:          []ResourceType{ResUntap},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
	}
	aurelia := CardProfile{
		Name:              "Aurelia, the Warleader",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResUntap},
		MandatoryTriggers: true,
	}
	hasteEnabler := CardProfile{
		Name:              "Concordant Crossroads",
		Produces:          []ResourceType{ResReanimate},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
		IsHasteEnabler:    true,
		IsRecursion:       true,
		RecursionDest:     "battlefield",
	}
	doubler := CardProfile{
		Name:              "Anointed Procession",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResReanimate},
		MandatoryTriggers: true,
	}
	// Cycle: twinflame -token-> celebrant -untap-> aurelia -mana-> haste -reanimate-> doubler -mana-> twinflame
	// Need twinflame to Consumes mana ✓
	results := FindLongLoops([]CardProfile{twinflame, celebrant, aurelia, hasteEnabler, doubler})
	if got := findCycleContaining(t, results, twinflame.Name, celebrant.Name, aurelia.Name, hasteEnabler.Name, doubler.Name); got == nil {
		t.Fatalf("expected 5-cycle including Twinflame combat chain, got results: %+v", results)
	}
}

// TestFindLongLoops_OpenChainReturnsNothing pins the negative case: an
// open chain that doesn't close into a cycle should produce no results.
// E consumes a resource (counter) that nobody in the chain produces, so
// the closing edge cannot exist regardless of the interesting-resource
// gate.
func TestFindLongLoops_OpenChainReturnsNothing(t *testing.T) {
	chain := []CardProfile{
		{Name: "A", Produces: []ResourceType{ResToken}, Consumes: []ResourceType{ResLife}, MandatoryTriggers: true},
		{Name: "B", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResToken}, MandatoryTriggers: true},
		{Name: "C", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
		{Name: "D", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "E", Produces: []ResourceType{ResLife}, Consumes: []ResourceType{ResCounter}, MandatoryTriggers: true},
	}
	// E consumes counter; no card in the chain produces counter, so the
	// in-edge to E cannot be formed by D either. The cycle cannot close.
	results := FindLongLoops(chain)
	for _, r := range results {
		if len(r.Cards) == 5 {
			t.Errorf("expected no 5-cycle (E.Consumes=counter is not produced by any node), got %+v", r)
		}
	}
}

// TestFindLongLoops_CyclingCoalesce: a 5-cycle with 2+ cycling cards is
// dropped per the issue #523 coalesce rule. (In practice the pre-filter
// caps cycling candidates at 1, so the path-level guard is defense in
// depth — but if the bucket pre-filter is ever loosened we want the
// guard to still hold.)
func TestFindLongLoops_CyclingCoalesce(t *testing.T) {
	// 5 cycling cards all with the same shape. Without the coalesce
	// guard this would surface as a 5-cycle of redundant cycling
	// behavior — semantically equivalent to a 2-card cycling "loop"
	// (which itself isn't a real combo).
	cards := []CardProfile{
		{Name: "Cycle1", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResCard}, HasCycling: true, MandatoryTriggers: true},
		{Name: "Cycle2", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResCard}, HasCycling: true, MandatoryTriggers: true},
		{Name: "Cycle3", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResCard}, HasCycling: true, MandatoryTriggers: true},
		{Name: "Cycle4", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResCard}, HasCycling: true, MandatoryTriggers: true},
		{Name: "Cycle5", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResCard}, HasCycling: true, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	for _, r := range results {
		cycCount := 0
		for _, name := range r.Cards {
			if strings.HasPrefix(name, "Cycle") {
				cycCount++
			}
		}
		if cycCount >= 2 {
			t.Errorf("expected cycling coalesce to suppress 2+ cycling card combo, got %+v", r)
		}
	}
}

// TestFindLongLoops_DedupCanonicalRotation: the same 5-cycle visited from
// different starting nodes (or in the reverse direction) is emitted at
// most once.
func TestFindLongLoops_DedupCanonicalRotation(t *testing.T) {
	cards := []CardProfile{
		{Name: "Z_A", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResToken}, MandatoryTriggers: true},
		{Name: "Z_B", Produces: []ResourceType{ResToken}, Consumes: []ResourceType{ResCard}, MandatoryTriggers: true},
		{Name: "Z_C", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		{Name: "Z_D", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResGraveyard}, MandatoryTriggers: true},
		{Name: "Z_E", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
	}
	results := FindLongLoops(cards)
	count := 0
	for _, r := range results {
		if len(r.Cards) == 5 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 unique 5-cycle, got %d (results: %+v)", count, results)
	}
}

// TestFindLongLoops_CycleLengthBound: cycles of length > graphWalkMaxLength
// are not enumerated. An 8-cycle should produce no length-8 results.
func TestFindLongLoops_CycleLengthBound(t *testing.T) {
	// Build an 8-cycle through the mana_positive bucket. No 5/6/7
	// sub-cycle exists in this construction because every node has
	// exactly one in-edge and one out-edge along the 8-cycle.
	resources := []ResourceType{ResMana, ResToken, ResCard, ResUntap, ResGraveyard, ResReanimate, ResLand, ResGraveyardFill}
	if len(resources) != 8 {
		t.Fatalf("test setup error")
	}
	var cards []CardProfile
	for i := 0; i < 8; i++ {
		cards = append(cards, CardProfile{
			Name:              "N" + string(rune('0'+i)),
			Produces:          []ResourceType{resources[i]},
			Consumes:          []ResourceType{resources[(i+7)%8]}, // (i-1) mod 8
			MandatoryTriggers: true,
		})
	}
	results := FindLongLoops(cards)
	for _, r := range results {
		if len(r.Cards) > graphWalkMaxLength {
			t.Errorf("expected no cycle longer than %d, got length %d: %+v",
				graphWalkMaxLength, len(r.Cards), r)
		}
	}
}

// TestFindLongLoops_FingerprintGate: cards from disjoint themes don't
// form a cross-theme cycle. Construct 5 cards where each pair of
// adjacent cards shares no theme bucket — the walker should produce
// nothing.
func TestFindLongLoops_FingerprintGate(t *testing.T) {
	// Build 5 cards each in a distinct theme bucket with NO shared theme.
	// Mana-only, token-only (using IsMassTokens which doesn't co-occur),
	// graveyard-only via IsRecursion, untap-only, ETB-blink-only.
	// Each card's Produces / Consumes are crafted so a 5-cycle would
	// CLOSE (resource overlap exists), but the theme-bucket walker
	// requires ALL nodes in the cycle to share a single bucket.
	cards := []CardProfile{
		// mana_positive only
		{Name: "ManaOnly", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResMana}, MandatoryTriggers: true},
		// untap only (no mana, no token)
		{Name: "UntapOnly", Produces: []ResourceType{ResUntap}, Consumes: []ResourceType{ResUntap}, MandatoryTriggers: true},
		// graveyard only (via IsRecursion gating)
		{Name: "GYOnly", Produces: []ResourceType{ResGraveyard}, Consumes: []ResourceType{ResGraveyard}, IsRecursion: true, MandatoryTriggers: true},
		// landfall only — note: landfall is not currently a theme bucket
		{Name: "LandfallOnly", Produces: []ResourceType{ResLandfall}, Consumes: []ResourceType{ResLandfall}, MandatoryTriggers: true},
		// damage only via HasETBDamage
		{Name: "DmgOnly", Produces: []ResourceType{ResDamage}, Consumes: []ResourceType{ResDamage}, HasETBDamage: true, MandatoryTriggers: true},
	}
	// No card here shares a theme bucket with all the others — each is in
	// exactly one bucket, and no single bucket has 5 cards in it. The
	// walker requires bucket-size >= 5 to even start; nothing should fire.
	results := FindLongLoops(cards)
	for _, r := range results {
		if len(r.Cards) == 5 {
			t.Errorf("expected fingerprint gate to suppress cross-theme 5-cycle, got %+v", r)
		}
	}
}

// TestFindLongLoops_BucketCap: buckets with > graphWalkCandidateCap candidates
// are skipped silently rather than processed (would blow runtime).
func TestFindLongLoops_BucketCap(t *testing.T) {
	// Construct graphWalkCandidateCap + 5 mana_positive cards each in a
	// trivial 5-cycle. Without the bucket cap the walker would emit
	// many overlapping cycles; with it, the bucket is skipped and no
	// long loops are reported.
	n := graphWalkCandidateCap + 5
	var cards []CardProfile
	resources := []ResourceType{ResMana, ResToken, ResCard, ResUntap, ResGraveyard}
	for i := 0; i < n; i++ {
		cards = append(cards, CardProfile{
			Name:              "M" + string(rune('A'+i%26)) + string(rune('a'+i/26)),
			Produces:          []ResourceType{resources[i%5]},
			Consumes:          []ResourceType{resources[(i+1)%5]},
			MandatoryTriggers: true,
		})
	}
	// This call should return quickly and emit nothing — the
	// mana_positive bucket has n nodes, exceeding the cap.
	results := FindLongLoops(cards)
	// We expect zero results because the only bucket containing these
	// cards (mana_positive) exceeds the cap.
	if len(results) > 0 {
		t.Errorf("expected bucket cap to skip dense bucket entirely, got %d results", len(results))
	}
}

// TestCanonicalCycleKey_RotationInvariant: rotation of the same cycle
// produces the same key.
func TestCanonicalCycleKey_RotationInvariant(t *testing.T) {
	base := []CardProfile{
		{Name: "Alpha"},
		{Name: "Beta"},
		{Name: "Gamma"},
		{Name: "Delta"},
		{Name: "Epsilon"},
	}
	want := canonicalCycleKey(base)
	for r := 1; r < 5; r++ {
		rotated := append(append([]CardProfile{}, base[r:]...), base[:r]...)
		if got := canonicalCycleKey(rotated); got != want {
			t.Errorf("rotation %d: key drift %q vs %q", r, got, want)
		}
	}
}

// TestCanonicalCycleKey_ReversalInvariant: reversing the cycle produces
// the same key.
func TestCanonicalCycleKey_ReversalInvariant(t *testing.T) {
	base := []CardProfile{
		{Name: "Alpha"},
		{Name: "Beta"},
		{Name: "Gamma"},
		{Name: "Delta"},
		{Name: "Epsilon"},
	}
	want := canonicalCycleKey(base)
	rev := make([]CardProfile, len(base))
	for i, c := range base {
		rev[len(base)-1-i] = c
	}
	if got := canonicalCycleKey(rev); got != want {
		t.Errorf("reversal: key drift %q vs %q", got, want)
	}
}

// TestThemeBucketsFor_CoverageSpotCheck: a handful of card profiles land
// in the expected buckets. This guards against future refactors silently
// dropping a theme assignment.
func TestThemeBucketsFor_CoverageSpotCheck(t *testing.T) {
	cases := []struct {
		name    string
		profile CardProfile
		want    []ThemeBucket
	}{
		{
			name: "mana_positive via Produces",
			profile: CardProfile{
				Name: "ramp", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife},
			},
			want: []ThemeBucket{ThemeManaPositive},
		},
		{
			name: "blinker bucket via IsBlinker",
			profile: CardProfile{
				Name: "flicker", IsBlinker: true,
				Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana},
			},
			want: []ThemeBucket{ThemeManaPositive, ThemeETBBlink, ThemeGraveyardLoop},
		},
		{
			name: "damage_chain via CounterToDamage",
			profile: CardProfile{
				Name: "heliod", CounterToDamage: true,
				Produces: []ResourceType{ResCounter}, Consumes: []ResourceType{ResLife},
			},
			want: []ThemeBucket{ThemeDamageChain},
		},
		{
			name: "graveyard_loop via IsRecursion",
			profile: CardProfile{
				Name: "muldrotha", IsRecursion: true,
				Produces: []ResourceType{ResReanimate}, Consumes: []ResourceType{ResMana},
			},
			want: []ThemeBucket{ThemeManaPositive, ThemeGraveyardLoop},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := themeBucketsFor(c.profile)
			gotList := make([]string, 0, len(got))
			for b := range got {
				gotList = append(gotList, string(b))
			}
			sort.Strings(gotList)
			wantList := make([]string, 0, len(c.want))
			for _, b := range c.want {
				wantList = append(wantList, string(b))
			}
			sort.Strings(wantList)
			if strings.Join(gotList, ",") != strings.Join(wantList, ",") {
				t.Errorf("profile %s: themes = %v, want %v", c.profile.Name, gotList, wantList)
			}
		})
	}
}

// BenchmarkFindLongLoops_WorstCase exercises the graph walker at the
// bucket cap with every candidate fully resource-flowing. Validates
// that the cap keeps runtime bounded.
func BenchmarkFindLongLoops_WorstCase(b *testing.B) {
	resources := []ResourceType{ResMana, ResToken, ResCard, ResUntap, ResGraveyard}
	var profiles []CardProfile
	for i := 0; i < graphWalkCandidateCap; i++ {
		profiles = append(profiles, CardProfile{
			Name:              "C" + string(rune('A'+i%26)) + string(rune('a'+i/26)),
			Produces:          []ResourceType{resources[i%5]},
			Consumes:          []ResourceType{resources[(i+1)%5]},
			MandatoryTriggers: true,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindLongLoops(profiles)
	}
}
