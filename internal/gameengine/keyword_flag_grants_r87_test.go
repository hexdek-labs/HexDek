package gameengine

import "testing"

func r87Perm() *Permanent {
	return &Permanent{Card: &Card{Name: "Subject", Types: []string{"creature"}}, Flags: map[string]int{}}
}

// TestGrantKeywordFlagR87_RefcountsAcrossSources is the primitive's core
// contract: the shared Flags["kw:<x>"] survives until the LAST holder
// revokes.
func TestGrantKeywordFlagR87_RefcountsAcrossSources(t *testing.T) {
	p := r87Perm()

	if changed := GrantKeywordFlag(p, "haste", "a"); !changed {
		t.Errorf("first grant should report the keyword set changed")
	}
	if changed := GrantKeywordFlag(p, "haste", "b"); changed {
		t.Errorf("second grant must NOT report a change — the flag was already set")
	}
	if n := KeywordFlagGrantCount(p, "haste"); n != 2 {
		t.Fatalf("want 2 holders, got %d: %+v", n, p.Flags)
	}

	if changed := RevokeKeywordFlag(p, "haste", "a"); changed {
		t.Errorf("revoking one of two holders must not report the keyword removed")
	}
	if !p.HasKeyword("haste") {
		t.Errorf("b still holds the grant — haste must survive a's revoke: %+v", p.Flags)
	}

	if changed := RevokeKeywordFlag(p, "haste", "b"); !changed {
		t.Errorf("revoking the last holder should report the keyword removed")
	}
	if p.HasKeyword("haste") {
		t.Errorf("no holders left — haste must be gone: %+v", p.Flags)
	}
	if n := KeywordFlagGrantCount(p, "haste"); n != 0 {
		t.Errorf("want 0 holders, got %d: %+v", n, p.Flags)
	}
}

// TestGrantKeywordFlagR87_Idempotent pins that re-granting under the
// same marker does not inflate the count. These handlers run as
// repeated battlefield sweeps, so a non-idempotent grant would make the
// keyword un-revokable.
func TestGrantKeywordFlagR87_Idempotent(t *testing.T) {
	p := r87Perm()
	for i := 0; i < 5; i++ {
		GrantKeywordFlag(p, "flying", "src")
	}
	if n := KeywordFlagGrantCount(p, "flying"); n != 1 {
		t.Fatalf("want 1 holder after 5 sweeps, got %d: %+v", n, p.Flags)
	}
	RevokeKeywordFlag(p, "flying", "src")
	if p.HasKeyword("flying") {
		t.Errorf("one revoke must clear a repeatedly-granted keyword: %+v", p.Flags)
	}
}

// TestGrantKeywordFlagR87_AdoptsPreexistingRawStamp pins the
// transitional guarantee for the ~230 unmigrated raw stamp sites.
func TestGrantKeywordFlagR87_AdoptsPreexistingRawStamp(t *testing.T) {
	p := r87Perm()
	p.Flags["kw:trample"] = 1 // an unmigrated handler's raw stamp

	if changed := GrantKeywordFlag(p, "trample", "src"); changed {
		t.Errorf("the flag was already set — grant must not report a change")
	}
	if n := KeywordFlagGrantCount(p, "trample"); n != 2 {
		t.Fatalf("want the adopted pre-existing holder plus src = 2, got %d: %+v", n, p.Flags)
	}
	RevokeKeywordFlag(p, "trample", "src")
	if !p.HasKeyword("trample") {
		t.Errorf("a stamp src did not place must survive src's revoke: %+v", p.Flags)
	}
}

// TestGrantKeywordFlagR87_RevokeOfUnheldGrantIsInert pins that a
// migrated handler cannot delete a keyword it never granted — the
// failure mode the refcount exists to prevent, approached from the
// other side.
func TestGrantKeywordFlagR87_RevokeOfUnheldGrantIsInert(t *testing.T) {
	p := r87Perm()
	GrantKeywordFlag(p, "menace", "owner")

	if changed := RevokeKeywordFlag(p, "menace", "stranger"); changed {
		t.Errorf("a source with no marker must not report a removal")
	}
	if !p.HasKeyword("menace") {
		t.Errorf("owner's grant must be untouched by a stranger's revoke: %+v", p.Flags)
	}
}

// TestGrantKeywordFlagR87_MarkersAreNotKeywords pins that provenance
// stays out of the layer-computed keyword set. runtimeKeywordSources
// folds every "kw:"-prefixed key VERBATIM, which is why the markers use
// their own "kwsrc:" namespace — the pre-existing
// "kw:menace_from_gisa" convention lands in chars.Keywords as a keyword
// literally named "menace_from_gisa".
func TestGrantKeywordFlagR87_MarkersAreNotKeywords(t *testing.T) {
	p := r87Perm()
	GrantKeywordFlag(p, "ward", "src")

	for _, kw := range runtimeKeywordSources(p) {
		if kw != "ward" {
			t.Errorf("provenance leaked into the keyword set: %q (all: %v)", kw, runtimeKeywordSources(p))
		}
	}
}

// TestGrantKeywordFlagR87_NormalizesLikeHasKeyword pins that the stamp
// lands on the key the readers look up. HasKeyword looks up the literal
// "kw:"+strings.ToLower(strings.TrimSpace(name)), so a grant written in
// any other case would silently grant nothing.
func TestGrantKeywordFlagR87_NormalizesLikeHasKeyword(t *testing.T) {
	p := r87Perm()
	GrantKeywordFlag(p, "  First Strike  ", "src")

	if !p.HasKeyword("first strike") {
		t.Errorf("grant must normalize to the key HasKeyword reads: %+v", p.Flags)
	}
	if !RevokeKeywordFlag(p, "FIRST STRIKE", "src") {
		t.Errorf("revoke must normalize the same way: %+v", p.Flags)
	}
	if p.HasKeyword("first strike") {
		t.Errorf("keyword should be gone: %+v", p.Flags)
	}
}
