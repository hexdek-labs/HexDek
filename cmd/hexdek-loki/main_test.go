package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// known is the canonical name set pulled from the engine. We pin against
// the real registry rather than a fixture so a new invariant added to
// AllInvariants automatically participates in --invariant validation
// without requiring this test to be updated.
var known = invariantNames(gameengine.AllInvariants())

func TestInvariantNames_NonEmpty(t *testing.T) {
	if len(known) == 0 {
		t.Fatal("AllInvariants() returned no invariants — registry collapsed?")
	}
	for _, n := range known {
		if n == "" {
			t.Fatal("empty invariant name in registry")
		}
	}
}

func TestNormalizeInvariantKey_AcceptsKebabSnakeCamel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"ZoneConservation", "zoneconservation"},
		{"zone-conservation", "zoneconservation"},
		{"zone_conservation", "zoneconservation"},
		{"  Zone-Conservation  ", "zoneconservation"},
		{"ATTACHMENTCONSISTENCY", "attachmentconsistency"},
	}
	for _, c := range cases {
		if got := normalizeInvariantKey(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalizeInvariantName_EmptyIsMatchAll(t *testing.T) {
	for _, in := range []string{"", "   ", "\t"} {
		got, err := canonicalizeInvariantName(in, known)
		if err != nil {
			t.Fatalf("empty input %q should not error: %v", in, err)
		}
		if got != "" {
			t.Fatalf("empty input %q should canonicalize to empty (match-all), got %q", in, got)
		}
	}
}

func TestCanonicalizeInvariantName_AcceptsCamelCase(t *testing.T) {
	got, err := canonicalizeInvariantName("ZoneConservation", known)
	if err != nil {
		t.Fatalf("camelCase: %v", err)
	}
	if got != "ZoneConservation" {
		t.Fatalf("camelCase: got %q, want ZoneConservation", got)
	}
}

func TestCanonicalizeInvariantName_AcceptsKebabCase(t *testing.T) {
	got, err := canonicalizeInvariantName("zone-conservation", known)
	if err != nil {
		t.Fatalf("kebab-case: %v", err)
	}
	if got != "ZoneConservation" {
		t.Fatalf("kebab-case must canonicalize to camelCase, got %q", got)
	}
}

func TestCanonicalizeInvariantName_RejectsUnknown(t *testing.T) {
	_, err := canonicalizeInvariantName("bogus-invariant", known)
	if err == nil {
		t.Fatal("expected error for unknown invariant")
	}
	// Error message must list the valid names so the user can actually
	// recover from the typo without consulting source.
	msg := err.Error()
	for _, n := range known {
		if !strings.Contains(msg, n) {
			t.Errorf("error message missing valid name %q: %s", n, msg)
		}
	}
}

func TestCanonicalizeInvariantName_EveryRegisteredNameCanonicalizesToItself(t *testing.T) {
	// Catches any future invariant whose name (or normalization) collides
	// with another's normalized form.
	seen := map[string]string{}
	for _, name := range known {
		got, err := canonicalizeInvariantName(name, known)
		if err != nil {
			t.Errorf("registered name %q failed to canonicalize: %v", name, err)
			continue
		}
		if got != name {
			t.Errorf("registered name %q canonicalized to a different name %q", name, got)
		}
		key := normalizeInvariantKey(name)
		if prev, dup := seen[key]; dup {
			t.Errorf("normalized-key collision: %q and %q both normalize to %q", prev, name, key)
		}
		seen[key] = name
	}
}

func TestMatchesInvariantFilter_EmptyCanonicalMatchesAll(t *testing.T) {
	// Unfiltered run: every violation should be accepted.
	for _, n := range known {
		if !matchesInvariantFilter(n, "") {
			t.Errorf("empty filter must accept %q (legacy behavior)", n)
		}
	}
}

func TestMatchesInvariantFilter_RestrictsToCanonical(t *testing.T) {
	if !matchesInvariantFilter("ZoneConservation", "ZoneConservation") {
		t.Error("filter must accept its target invariant")
	}
	if matchesInvariantFilter("CardIdentity", "ZoneConservation") {
		t.Error("filter must reject non-matching invariants")
	}
}

// TestCheckChaosInvariants_FilterRespected exercises the full code path
// from a synthetic violating GameState through the filter to the result
// slice. Forces a ZoneConservation violation by leaving cards in a seat
// that the engine's census doesn't recognise (extra card pointer in the
// graveyard) — then runs the checker twice, once unfiltered and once
// with a filter that excludes ZoneConservation.
func TestCheckChaosInvariants_FilterRespected(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5

	// Plant a ZoneConservation violation: stash the same Card pointer in
	// two seats' graveyards. The invariant tallies pointers and fires
	// when the census doesn't add up.
	dup := &gameengine.Card{Name: "Test Duplicate", Owner: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, dup)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, dup)

	// Pre-check: confirm at least one ZoneConservation or CardIdentity
	// violation surfaces from RunAllInvariants on this synthetic state.
	// Graceful-degradation skip: this test exercises the filter pipeline,
	// not the invariant detector itself. If a future engine refactor makes
	// the duplicate-card-pointer plant no longer trip any invariant, this
	// test should skip (the filter contract is independently covered by
	// TestMatchesInvariantFilter_* above), not fail — the invariant suite
	// in internal/gameengine/invariants_test.go owns detector coverage.
	raw := gameengine.RunAllInvariants(gs)
	if len(raw) == 0 {
		t.Skip("synthetic state did not produce a violation; engine semantics may have shifted")
	}
	probeName := raw[0].Name

	// Unfiltered: violations recorded.
	invariantFilter = ""
	t.Cleanup(func() { invariantFilter = "" })
	resAll := &chaosGameResult{}
	checkChaosInvariants(gs, 0, 0, 0, nil, resAll)
	if len(resAll.Violations) == 0 {
		t.Fatal("unfiltered run should record at least one violation")
	}
	gotInProbe := false
	for _, v := range resAll.Violations {
		if v.InvariantName == probeName {
			gotInProbe = true
		}
	}
	if !gotInProbe {
		t.Fatalf("unfiltered run missed probe invariant %q (got %+v)", probeName, resAll.Violations)
	}

	// Filtered to a DIFFERENT invariant: should drop the probe violation.
	other := otherInvariantName(probeName)
	invariantFilter = other
	resFiltered := &chaosGameResult{}
	checkChaosInvariants(gs, 0, 0, 0, nil, resFiltered)
	for _, v := range resFiltered.Violations {
		if v.InvariantName == probeName {
			t.Errorf("filter=%q should have dropped %q violation, but it was recorded: %s",
				other, probeName, v.Message)
		}
	}

	// Filtered to the probe itself: should keep at least one entry.
	invariantFilter = probeName
	resProbe := &chaosGameResult{}
	checkChaosInvariants(gs, 0, 0, 0, nil, resProbe)
	if len(resProbe.Violations) == 0 {
		t.Fatalf("filter=%q should have kept probe violations", probeName)
	}
	for _, v := range resProbe.Violations {
		if v.InvariantName != probeName {
			t.Errorf("filter=%q leaked non-target invariant %q", probeName, v.InvariantName)
		}
	}
}

// otherInvariantName returns any known invariant name distinct from `name`.
// Used by the filter test to construct a "filter that excludes the probe".
func otherInvariantName(name string) string {
	for _, n := range known {
		if n != name {
			return n
		}
	}
	return ""
}
