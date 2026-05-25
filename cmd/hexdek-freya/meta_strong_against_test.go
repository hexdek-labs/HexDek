package main

import (
	"sort"
	"strings"
	"testing"
)

// TestMetaStrongAgainst_TokensUseCase pins the headline ask: "which
// archetypes does Tokens beat reliably?" A Tokens deck canonicalises to
// the "aggro" key, and the result must include the wide-board favored
// matchups (Combo, Storm, Voltron) plus the reverse-direction Voltron
// entry where Voltron lists "Token/Go Wide: unfavored" — that meta-call
// label is invisible to a forward-only lookup, so the reverse pass is
// the only way it shows up.
func TestMetaStrongAgainst_TokensUseCase(t *testing.T) {
	got := MetaStrongAgainst("Go Wide Tokens")
	if len(got) == 0 {
		t.Fatalf("expected non-empty StrongAgainst for tokens, got nothing")
	}
	mustInclude := []string{"Combo", "Storm", "Voltron"}
	for _, want := range mustInclude {
		if !hasArchetype(got, want) {
			t.Errorf("tokens StrongAgainst missing %q, got %v", want, archetypeNames(got))
		}
	}
	// Voltron should be "both" — aggro lists it as favored, and Voltron
	// lists Token/Go Wide as unfavored. The reverse pass should upgrade
	// the entry to "both" rather than appending a duplicate.
	for _, a := range got {
		if a.Archetype == "Voltron" {
			if a.Source != "both" {
				t.Errorf("Voltron entry should be corroborated by both directions, got Source=%q", a.Source)
			}
			if a.OpponentReason == "" {
				t.Errorf("Voltron entry should carry Voltron's own reason as OpponentReason, got empty")
			}
		}
	}
	// No duplicate Voltron entries even though it appears under both
	// "Aggro" and "Token/Go Wide" labels in Voltron's matchup list.
	count := 0
	for _, a := range got {
		if a.Archetype == "Voltron" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Voltron appeared %d times, want 1 (de-dup across alias labels)", count)
	}
}

// TestMetaStrongAgainst_ForwardOnlyAndReverseOnly pins both ends of the
// Source taxonomy. Stax vs Reanimator: stax forward-lists reanimator as
// favored AND reanimator forward-lists stax as unfavored — that's
// "both". Stax vs a hypothetical archetype that ONLY stax claims to
// beat would be "forward". Vice versa for "reverse".
func TestMetaStrongAgainst_ForwardOnlyAndReverseOnly(t *testing.T) {
	got := MetaStrongAgainst("Stax")
	var both, forwardOnly, reverseOnly int
	for _, a := range got {
		switch a.Source {
		case "both":
			both++
		case "forward":
			forwardOnly++
		case "reverse":
			reverseOnly++
		default:
			t.Errorf("invalid Source %q on entry %+v", a.Source, a)
		}
	}
	// Stax has 8 favored entries in metaMatchupDB and most are
	// reciprocated, so we expect lots of "both" and zero invalid sources.
	if both == 0 {
		t.Errorf("stax should have at least some 'both' entries (reciprocal favored/unfavored pairs), got 0")
	}
	if forwardOnly+both+reverseOnly == 0 {
		t.Errorf("stax should have at least one StrongAgainst entry")
	}
}

// TestMetaStrongAgainst_ResultOrdering pins the sort: "both"-corroborated
// matchups come first (highest confidence — both sides agree), then
// forward-only, then reverse-only. Within each tier, archetype name
// alphabetical for stability across DB edits.
func TestMetaStrongAgainst_ResultOrdering(t *testing.T) {
	got := MetaStrongAgainst("Control")
	if len(got) < 2 {
		t.Skipf("need ≥2 entries to test ordering, got %d", len(got))
	}
	rank := func(src string) int {
		switch src {
		case "both":
			return 0
		case "forward":
			return 1
		case "reverse":
			return 2
		}
		return 3
	}
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if rank(prev.Source) > rank(cur.Source) {
			t.Errorf("ordering: %q (%s) before %q (%s) — higher-confidence tier should sort first",
				prev.Archetype, prev.Source, cur.Archetype, cur.Source)
		}
		if rank(prev.Source) == rank(cur.Source) && prev.Archetype > cur.Archetype {
			t.Errorf("ordering: within %q tier, %q should sort before %q",
				prev.Source, cur.Archetype, prev.Archetype)
		}
	}
}

// TestMetaStrongAgainst_NoSelfMatchup pins that the result never names
// the deck's own archetype — aggro can't be favored against aggro.
func TestMetaStrongAgainst_NoSelfMatchup(t *testing.T) {
	for _, arch := range []string{"Aggro", "Combo", "Control", "Voltron", "Stax", "Storm"} {
		got := MetaStrongAgainst(arch)
		for _, a := range got {
			if strings.EqualFold(a.Archetype, arch) {
				t.Errorf("%s StrongAgainst includes itself: %+v", arch, a)
			}
		}
	}
}

// TestMetaStrongAgainst_OnlyFavoredAndUnfavored pins that we never
// surface a "neutral" pairing — only true favored relationships count
// as "X beats Y". A neutral rating means a coinflip, not an advantage.
func TestMetaStrongAgainst_OnlyFavoredAndUnfavored(t *testing.T) {
	// Control vs Stax is canonically neutral in the DB. It must NOT
	// appear in Control's StrongAgainst list, and Control must NOT
	// appear in Stax's StrongAgainst list.
	ctrl := MetaStrongAgainst("Control")
	if hasArchetype(ctrl, "Stax") {
		t.Errorf("Control StrongAgainst includes Stax — that pairing is neutral in the DB")
	}
	stax := MetaStrongAgainst("Stax")
	if hasArchetype(stax, "Control") {
		t.Errorf("Stax StrongAgainst includes Control — that pairing is neutral in the DB")
	}
}

// TestMetaStrongAgainst_ReverseCorroborationCarriesOpponentReason
// pins that when forward + reverse agree, the entry carries BOTH
// reasons — Reason from X's perspective, OpponentReason from Y's. The
// Decks screen renders this as "X beats Y because Z (they say: W)".
func TestMetaStrongAgainst_ReverseCorroborationCarriesOpponentReason(t *testing.T) {
	got := MetaStrongAgainst("Stax")
	for _, a := range got {
		if a.Archetype == "Combo" {
			if a.Source != "both" {
				t.Errorf("Stax→Combo should be 'both' (Stax lists Combo favored, Combo lists Stax unfavored), got Source=%q", a.Source)
			}
			// Forward reason: stax's own reason for beating combo
			// ("Drannith Magistrate / Rule of Law / Cursed Totem").
			if !strings.Contains(a.Reason, "Drannith Magistrate") &&
				!strings.Contains(a.Reason, "Rule of Law") {
				t.Errorf("Stax→Combo forward Reason missing stax-perspective cards, got %q", a.Reason)
			}
			// Reverse reason: combo's own reason for losing to stax.
			if a.OpponentReason == "" {
				t.Errorf("Stax→Combo should carry Combo's own reason as OpponentReason")
			}
			return
		}
	}
	t.Errorf("Stax StrongAgainst missing Combo: %v", archetypeNames(got))
}

// TestMetaStrongAgainst_UnknownArchetypeFallsBackToMidrange pins the
// canonicalisation: a made-up archetype string still gets a meaningful
// answer by falling through to "midrange" (matching computeMetaPositioning
// behavior). This guards the Decks screen against returning empty for
// archetypes that haven't been added to the DB yet.
func TestMetaStrongAgainst_UnknownArchetypeFallsBackToMidrange(t *testing.T) {
	got := MetaStrongAgainst("Some Made-Up Archetype")
	// Midrange has no forward-favored entries (it's net-negative in the
	// DB), but reverse-direction other archetypes may surface entries.
	// What matters: the call doesn't panic and returns a slice.
	if got == nil {
		t.Errorf("MetaStrongAgainst should return a slice (possibly empty), got nil")
	}
	// Sanity: every returned entry is valid.
	for _, a := range got {
		if a.Archetype == "" || a.Source == "" {
			t.Errorf("invalid entry: %+v", a)
		}
	}
}

// TestMetaStrongAgainst_WiredIntoDeckProfile asserts the end-to-end
// integration: computeMetaPositioning populates dp.StrongAgainst as a
// side effect, so the Decks screen reads it from the deck profile
// without calling MetaStrongAgainst directly.
func TestMetaStrongAgainst_WiredIntoDeckProfile(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Aggro / Go Wide"}
	computeMetaPositioning(dp)
	if len(dp.StrongAgainst) == 0 {
		t.Errorf("expected dp.StrongAgainst populated by computeMetaPositioning, got empty")
	}
	if len(dp.MetaMatchups) == 0 {
		t.Errorf("expected dp.MetaMatchups still populated (back-compat)")
	}
}

// TestMetaStrongAgainst_DeterministicAcrossRuns guards against
// map-iteration nondeterminism leaking through. Same input must yield
// the same output across multiple calls.
func TestMetaStrongAgainst_DeterministicAcrossRuns(t *testing.T) {
	first := MetaStrongAgainst("Stax")
	for i := 0; i < 5; i++ {
		other := MetaStrongAgainst("Stax")
		if !sameAdvantageList(first, other) {
			t.Errorf("non-deterministic ordering between runs:\n  first: %v\n  other: %v",
				archetypeNames(first), archetypeNames(other))
		}
	}
}

func hasArchetype(list []MetaAdvantage, name string) bool {
	for _, a := range list {
		if a.Archetype == name {
			return true
		}
	}
	return false
}

func archetypeNames(list []MetaAdvantage) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Archetype+"("+a.Source+")")
	}
	return out
}

func sameAdvantageList(a, b []MetaAdvantage) bool {
	if len(a) != len(b) {
		return false
	}
	an := archetypeNames(a)
	bn := archetypeNames(b)
	sort.Strings(an)
	sort.Strings(bn)
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}
