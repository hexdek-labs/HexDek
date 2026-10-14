package gameengine

import "testing"

// keyword_fold_provenance_r89_test.go — the measurement behind the
// corrected comment in keyword_source_unification_r65.go.
//
// That comment claimed the VERBATIM fold "keeps provenance tags such as
// kw:menace_from_gisa from being reinterpreted as a keyword". The
// opposite is true: folding verbatim is what carries the tag into
// chars.Keywords under its own name. This test is the evidence, and it
// also pins the bound on the harm — no keyword query answers wrongly —
// so a future reader does not over- or under-react to it.

func TestKeywordFoldR89_ProvenanceTagInKwNamespaceBecomesAKeyword(t *testing.T) {
	p := &Permanent{
		Card:     &Card{Name: "R89 Probe", Types: []string{"creature"}},
		Flags:    map[string]int{"kw:menace_from_gisa": 1},
		Owner:    0,
		Counters: map[string]int{},
	}

	got := runtimeKeywordSources(p)
	found := false
	for _, kw := range got {
		if kw == "menace_from_gisa" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the provenance tag to be folded verbatim as a keyword; got %v. "+
			"If this now passes cleanly the fold changed — re-read the corrected comment in "+
			"keyword_source_unification_r65.go before deleting this test", got)
	}

	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	chars := GetEffectiveCharacteristics(gs, p)
	inChars := false
	for _, kw := range chars.Keywords {
		if kw == "menace_from_gisa" {
			inChars = true
		}
	}
	if !inChars {
		t.Errorf("the tag reaches chars.Keywords in practice, not only the raw fold; got %v", chars.Keywords)
	}

	// THE BOUND ON THE HARM. The fabricated entry does not make the real
	// keyword answer true on either authority. This half must stay green;
	// if it ever fails, a provenance tag has started granting a keyword
	// and the severity is completely different.
	if p.HasKeyword("menace") {
		t.Errorf("raw HasKeyword(menace) must stay false — a provenance tag must never grant the real keyword")
	}
	if gs.HasKeywordOf(p, "menace") {
		t.Errorf("layer-aware HasKeywordOf(menace) must stay false — a provenance tag must never grant the real keyword")
	}
}

// TestKeywordFoldR89_KwsrcMarkersStayOutOfTheFold is the contrast case:
// the r87 marker namespace does not have this problem, which is the
// whole reason it was chosen.
func TestKeywordFoldR89_KwsrcMarkersStayOutOfTheFold(t *testing.T) {
	p := &Permanent{
		Card:     &Card{Name: "R89 Probe", Types: []string{"creature"}},
		Flags:    map[string]int{},
		Counters: map[string]int{},
	}
	GrantKeywordFlag(p, "menace", "gisa_the_hellraiser")

	for _, kw := range runtimeKeywordSources(p) {
		if kw != "menace" {
			t.Errorf("only the real keyword may be folded, got %q (all: %v)", kw, runtimeKeywordSources(p))
		}
	}
}
