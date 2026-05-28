package main

import (
	"strings"
	"testing"
)

// makeFP builds a tiny DeckFingerprint inline for overlap tests.
func makePreconFP(name, commander string, isPrecon bool, cards ...string) *DeckFingerprint {
	fp := &DeckFingerprint{
		Name:         name,
		Commander:    commander,
		IsPrecon:     isPrecon,
		NonlandCards: map[string]bool{},
	}
	for _, c := range cards {
		fp.NonlandCards[strings.ToLower(c)] = true
	}
	return fp
}

// TestFindCanonicalPreconOverlap_MatchesByCommander pins the canonical
// behavior: a user deck for "Sigarda, Heron's Grace" finds the precon
// for the same commander and reports a shared-card count.
func TestFindCanonicalPreconOverlap_MatchesByCommander(t *testing.T) {
	user := makePreconFP("user_sigarda", "Sigarda, Heron's Grace", false,
		"Heroic Intervention", "Smothering Tithe", "Cultivate",
		"Sol Ring", "Arcane Signet", "Sigarda's Aid",
		"Sterling Grove", "Replenish", "Eladamri's Call")
	precon := makePreconFP("coven_counters", "Sigarda, Heron's Grace", true,
		"Heroic Intervention", "Sol Ring", "Arcane Signet",
		"Sigarda's Aid", "Sterling Grove",
		"Stocking Tiger", "Some Other Filler", "More Filler")

	got := FindCanonicalPreconOverlap(user, []*DeckFingerprint{precon})
	if got == nil {
		t.Fatal("expected an overlap result, got nil")
	}
	// Shared: Heroic Intervention / Sol Ring / Arcane Signet / Sigarda's Aid / Sterling Grove = 5
	if got.SharedCount != 5 {
		t.Errorf("SharedCount = %d, want 5", got.SharedCount)
	}
	if got.PreconName != "coven_counters" {
		t.Errorf("PreconName = %q, want %q", got.PreconName, "coven_counters")
	}
	if got.PreconCommander != "Sigarda, Heron's Grace" {
		t.Errorf("PreconCommander = %q, want %q", got.PreconCommander, "Sigarda, Heron's Grace")
	}
	if got.TargetNonland != 9 {
		t.Errorf("TargetNonland = %d, want 9", got.TargetNonland)
	}
	// 5/9 ≈ 55.6%
	if got.SharedPercent < 55.0 || got.SharedPercent > 56.0 {
		t.Errorf("SharedPercent = %.1f, want ~55.6", got.SharedPercent)
	}
}

// TestFindCanonicalPreconOverlap_IgnoresDifferentCommanders pins that
// only precons matching the target's commander are considered — a
// Sigarda user deck must not match against an Atraxa precon even if
// the card overlap is high (Sol Ring / Cultivate staples).
func TestFindCanonicalPreconOverlap_IgnoresDifferentCommanders(t *testing.T) {
	user := makePreconFP("user_sigarda", "Sigarda, Heron's Grace", false,
		"Sol Ring", "Arcane Signet", "Cultivate", "Heroic Intervention")
	atraxaPrecon := makePreconFP("breed_lethality", "Atraxa, Praetors' Voice", true,
		"Sol Ring", "Arcane Signet", "Cultivate")
	got := FindCanonicalPreconOverlap(user, []*DeckFingerprint{atraxaPrecon})
	if got != nil {
		t.Errorf("expected nil (no Sigarda precon in corpus), got %+v", got)
	}
}

// TestFindCanonicalPreconOverlap_SkipsNonPrecons pins that
// fingerprints with IsPrecon=false are skipped — a user-built deck
// matching the same commander is NOT a "canonical precon" reference.
func TestFindCanonicalPreconOverlap_SkipsNonPrecons(t *testing.T) {
	user := makePreconFP("user_sigarda", "Sigarda, Heron's Grace", false,
		"Sol Ring", "Cultivate")
	otherUserDeck := makePreconFP("friend_sigarda", "Sigarda, Heron's Grace", false,
		"Sol Ring", "Cultivate")
	got := FindCanonicalPreconOverlap(user, []*DeckFingerprint{otherUserDeck})
	if got != nil {
		t.Errorf("non-precon corpus entry should not match, got %+v", got)
	}
}

// TestFindCanonicalPreconOverlap_SkipsSelf pins the structural guard:
// scanning a precon fingerprint against a corpus that includes itself
// must not return a self-match. The use case is the --all-decks pass
// where the precon is in BOTH the target list and the precon subset.
func TestFindCanonicalPreconOverlap_SkipsSelf(t *testing.T) {
	precon := makePreconFP("coven_counters", "Sigarda, Heron's Grace", true,
		"Sol Ring", "Cultivate", "Heroic Intervention")
	got := FindCanonicalPreconOverlap(precon, []*DeckFingerprint{precon})
	if got != nil {
		t.Errorf("self-match must return nil, got %+v", got)
	}
}

// TestFindCanonicalPreconOverlap_PicksBestWhenMultipleMatch pins the
// multi-precon tiebreak. Atraxa appears in two WotC precons (C16 Breed
// Lethality + ONE Corrupting Influence variant); the overlap call
// returns the one sharing MORE cards with the user's deck — that's
// the precon the user's build most resembles.
func TestFindCanonicalPreconOverlap_PicksBestWhenMultipleMatch(t *testing.T) {
	user := makePreconFP("user_atraxa", "Atraxa, Praetors' Voice", false,
		"Atraxa, Praetors' Voice", "Inexorable Tide", "Doubling Season",
		"Cultivate", "Sol Ring", "Vorinclex, Monstrous Raider")

	precon1 := makePreconFP("breed_lethality", "Atraxa, Praetors' Voice", true,
		"Atraxa, Praetors' Voice", "Cultivate", "Sol Ring")
	precon2 := makePreconFP("corrupting_influence", "Atraxa, Praetors' Voice", true,
		"Atraxa, Praetors' Voice", "Inexorable Tide", "Cultivate", "Sol Ring",
		"Vorinclex, Monstrous Raider", "Doubling Season")

	got := FindCanonicalPreconOverlap(user, []*DeckFingerprint{precon1, precon2})
	if got == nil {
		t.Fatal("expected an overlap result")
	}
	if got.PreconName != "corrupting_influence" {
		t.Errorf("expected the higher-overlap precon (corrupting_influence, 6 shared), got %q (%d shared)",
			got.PreconName, got.SharedCount)
	}
	if got.SharedCount != 6 {
		t.Errorf("SharedCount = %d, want 6 (all 6 nonlands of user deck shared with precon2)", got.SharedCount)
	}
}

// TestFindCanonicalPreconOverlap_NilSafety pins the defensive nil
// guards: empty corpus, nil target, target with empty commander.
func TestFindCanonicalPreconOverlap_NilSafety(t *testing.T) {
	if got := FindCanonicalPreconOverlap(nil, nil); got != nil {
		t.Errorf("nil target should return nil, got %+v", got)
	}
	user := makePreconFP("user", "", false, "Sol Ring")
	if got := FindCanonicalPreconOverlap(user, nil); got != nil {
		t.Errorf("empty commander should return nil, got %+v", got)
	}
	user2 := makePreconFP("user", "Sigarda, Heron's Grace", false, "Sol Ring")
	if got := FindCanonicalPreconOverlap(user2, []*DeckFingerprint{}); got != nil {
		t.Errorf("empty corpus should return nil, got %+v", got)
	}
}

// TestFormatPreconOverlap_HumanReadable pins the user-facing one-liner
// shape: "shares 23 cards with the canonical {precon name} precon
// (37% stock)" matching the task spec.
func TestFormatPreconOverlap_HumanReadable(t *testing.T) {
	o := &PreconOverlap{
		PreconName:    "sigarda_coven_counters",
		SharedCount:   23,
		TargetNonland: 63,
		SharedPercent: 36.51,
	}
	got := FormatPreconOverlap(o)
	for _, want := range []string{"23 cards", "sigarda_coven_counters", "37% stock"} {
		if !strings.Contains(got, want) {
			t.Errorf("format %q missing %q", got, want)
		}
	}
}

// TestFormatPreconOverlap_NilOrZeroEmpty pins that nil overlaps and
// zero-shared overlaps emit empty string so the text renderer can
// skip the line without an empty-message printout.
func TestFormatPreconOverlap_NilOrZeroEmpty(t *testing.T) {
	if FormatPreconOverlap(nil) != "" {
		t.Errorf("nil overlap should format empty")
	}
	if FormatPreconOverlap(&PreconOverlap{PreconName: "x", SharedCount: 0}) != "" {
		t.Errorf("zero shared should format empty")
	}
}

// TestBuildDeckFingerprint_DetectsPreconPath pins that the IsPrecon
// field is set automatically based on report.DeckPath — the wizards/
// substring is the load-bearing signal.
func TestBuildDeckFingerprint_DetectsPreconPath(t *testing.T) {
	preconReport := &FreyaReport{DeckPath: "data/decks/wizards/coven_counters.txt"}
	userReport := &FreyaReport{DeckPath: "data/decks/lyon/my_sigarda.txt"}

	fpPrecon := BuildDeckFingerprint(&DeckProfile{}, preconReport)
	if !fpPrecon.IsPrecon {
		t.Errorf("wizards/ path should set IsPrecon=true")
	}
	fpUser := BuildDeckFingerprint(&DeckProfile{}, userReport)
	if fpUser.IsPrecon {
		t.Errorf("non-wizards path should keep IsPrecon=false")
	}
}

// TestFindCanonicalPreconOverlap_CommanderMatchIsCaseInsensitive pins
// that commander-name matching tolerates case + whitespace differences
// (different source feeds can have varying capitalization).
func TestFindCanonicalPreconOverlap_CommanderMatchIsCaseInsensitive(t *testing.T) {
	user := makePreconFP("user", "sigarda, heron's grace", false,
		"Sol Ring", "Cultivate")
	precon := makePreconFP("coven_counters", "  Sigarda, Heron's Grace  ", true,
		"Sol Ring", "Cultivate", "Sigarda's Aid")
	got := FindCanonicalPreconOverlap(user, []*DeckFingerprint{precon})
	if got == nil {
		t.Fatal("case-insensitive whitespace-trimmed match should succeed")
	}
	if got.SharedCount != 2 {
		t.Errorf("SharedCount = %d, want 2", got.SharedCount)
	}
}
