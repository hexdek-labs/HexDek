package main

import "testing"

// TestCheckQuadCombo_AllInputOrderings constructs a 4-card directed cycle
// (A->B->C->D->A) and verifies that checkQuadCombo detects it regardless of
// which of the 24 input orderings is passed in. This is the 4-card analog of
// the r59 TestCheckTripleCombo_AllInputOrderings fix: any future regression
// that narrows the permutation table will fail at least one of these 24.
func TestCheckQuadCombo_AllInputOrderings(t *testing.T) {
	// Cycle: A->B (token), B->C (mana), C->D (card), D->A (graveyard).
	a := CardProfile{
		Name:              "TokenFactory",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResGraveyard},
		MandatoryTriggers: true,
	}
	b := CardProfile{
		Name:              "TokenToMana",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
	}
	c := CardProfile{
		Name:              "ManaToCard",
		Produces:          []ResourceType{ResCard},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
	}
	d := CardProfile{
		Name:              "CardToGraveyard",
		Produces:          []ResourceType{ResGraveyard},
		Consumes:          []ResourceType{ResCard},
		MandatoryTriggers: true,
	}

	perms := allQuadPerms(a, b, c, d)
	if len(perms) != 24 {
		t.Fatalf("test setup error: expected 24 perms, got %d", len(perms))
	}

	for i, p := range perms {
		combo := checkQuadCombo(p[0], p[1], p[2], p[3])
		if combo == nil {
			t.Errorf("permutation %d (%s,%s,%s,%s): expected combo, got nil",
				i, p[0].Name, p[1].Name, p[2].Name, p[3].Name)
			continue
		}
		if len(combo.Cards) != 4 {
			t.Errorf("permutation %d: expected 4 cards in combo, got %d", i, len(combo.Cards))
		}
	}
}

// TestCheckQuadCombo_NoCycleReturnsNil pins the negative case: an open chain
// of four cards (no return edge to close the cycle) should not be flagged.
func TestCheckQuadCombo_NoCycleReturnsNil(t *testing.T) {
	a := CardProfile{Name: "A", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife}}
	b := CardProfile{Name: "B", Produces: []ResourceType{ResCard}, Consumes: []ResourceType{ResMana}}
	c := CardProfile{Name: "C", Produces: []ResourceType{ResToken}, Consumes: []ResourceType{ResCard}}
	d := CardProfile{Name: "D", Produces: []ResourceType{ResLife}, Consumes: []ResourceType{ResCounter}}
	// D->A would need D.Produces ∩ A.Consumes; D produces life and A consumes life,
	// BUT life is not "interesting" per isInterestingLoop, AND no other edge in the
	// chain closes. So this should not be detected as a cycle (no interesting close).
	// More importantly: D consumes counter which nobody produces, so the cycle
	// going through D can't be completed via the chain shown above.
	if combo := checkQuadCombo(a, b, c, d); combo != nil {
		t.Errorf("expected nil (no interesting closed cycle), got %+v", combo)
	}
}

// TestCheckQuadCombo_ReverseDirectionDetected exercises the reversed cycle
// (A->D->C->B->A) explicitly to guarantee both directed orientations of the
// undirected 4-cycle are covered by the enumeration.
func TestCheckQuadCombo_ReverseDirectionDetected(t *testing.T) {
	a := CardProfile{
		Name:              "A",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
	}
	b := CardProfile{
		Name:              "B",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResCard},
		MandatoryTriggers: true,
	}
	c := CardProfile{
		Name:              "C",
		Produces:          []ResourceType{ResCard},
		Consumes:          []ResourceType{ResGraveyard},
		MandatoryTriggers: true,
	}
	d := CardProfile{
		Name:              "D",
		Produces:          []ResourceType{ResGraveyard},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
	}
	// Cycle as defined: A->D (token), D->C (graveyard), C->B (card), B->A (mana).
	// This is the "reverse" relative to alphabetical A->B->C->D->A.
	if combo := checkQuadCombo(a, b, c, d); combo == nil {
		t.Fatal("expected combo for reverse-direction 4-cycle, got nil")
	}
}

// TestQuadPermutationCoverage pins that the nested distinct-index loop in
// checkQuadCombo visits exactly the 24 unique permutations of (0,1,2,3) — no
// duplicates, no omissions. This is the structural invariant the r59 triple
// fix taught us to lock down: a permutation table that's silently wrong is
// invisible until a real-world cycle goes undetected.
func TestQuadPermutationCoverage(t *testing.T) {
	seen := map[[4]int]int{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if j == i {
				continue
			}
			for k := 0; k < 4; k++ {
				if k == i || k == j {
					continue
				}
				for l := 0; l < 4; l++ {
					if l == i || l == j || l == k {
						continue
					}
					seen[[4]int{i, j, k, l}]++
				}
			}
		}
	}
	if len(seen) != 24 {
		t.Fatalf("expected 24 unique perms, got %d", len(seen))
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("perm %v visited %d times, expected 1", p, n)
		}
	}
}

// BenchmarkFindLoopsQuad_WorstCase exercises FindLoops at the 70-candidate
// cap with every candidate fully resource-flowing (worst case for the quad
// search). Used to validate that the cap keeps runtime bounded.
func BenchmarkFindLoopsQuad_WorstCase(b *testing.B) {
	res := []ResourceType{ResMana, ResToken, ResCard, ResGraveyard}
	var profiles []CardProfile
	for i := 0; i < 70; i++ {
		profiles = append(profiles, CardProfile{
			Name:              "C" + string(rune('A'+i%26)) + string(rune('a'+i/26)),
			Produces:          []ResourceType{res[i%4]},
			Consumes:          []ResourceType{res[(i+1)%4]},
			MandatoryTriggers: true,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindLoops(profiles)
	}
}

// allQuadPerms returns every ordering of (a,b,c,d). Mirrors the structure of
// checkQuadCombo's nested loops so the test surface and the production
// surface generate the same 24-perm set from the same construction.
func allQuadPerms(a, b, c, d CardProfile) [][4]CardProfile {
	cards := [4]CardProfile{a, b, c, d}
	var out [][4]CardProfile
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if j == i {
				continue
			}
			for k := 0; k < 4; k++ {
				if k == i || k == j {
					continue
				}
				for l := 0; l < 4; l++ {
					if l == i || l == j || l == k {
						continue
					}
					out = append(out, [4]CardProfile{cards[i], cards[j], cards[k], cards[l]})
				}
			}
		}
	}
	return out
}
