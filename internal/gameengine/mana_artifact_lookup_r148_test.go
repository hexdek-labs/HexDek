package gameengine

import "testing"

// r148 — isSignet/isTalisman/isMox each built their lookup table inside
// the function, so every call allocated. The profile put isSignet alone
// at 1.42GB. These are now package-level tables and the predicates are
// pure lookups, so a call must allocate nothing at all.
//
// This is the test that actually proves the fix: it fails if anyone moves
// a table back inside its predicate, which a membership test would not
// notice.
func TestR148_ManaArtifactPredicatesDoNotAllocate(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"isSignet", func() { _ = isSignet("Azorius Signet") }},
		{"isTalisman", func() { _ = isTalisman("Talisman of Progress") }},
		{"isMox", func() { _ = isMox("Mox Pearl") }},
		{"isSignet miss", func() { _ = isSignet("Sol Ring") }},
	}
	for _, c := range cases {
		if got := testing.AllocsPerRun(200, c.fn); got != 0 {
			t.Errorf("%s allocates %.0f times per call — the lookup table is being rebuilt", c.name, got)
		}
	}
}

// Membership must be exactly what it was before the slice-to-map
// conversion. "Mox Ruby" was listed TWICE in the old slice; a map
// collapses the duplicate, which must not change the answer.
func TestR148_MembershipUnchanged(t *testing.T) {
	signets := []string{
		"Azorius Signet", "Dimir Signet", "Rakdos Signet", "Gruul Signet",
		"Selesnya Signet", "Orzhov Signet", "Izzet Signet", "Golgari Signet",
		"Boros Signet", "Simic Signet",
	}
	for _, n := range signets {
		if !isSignet(n) {
			t.Errorf("isSignet(%q) = false, want true", n)
		}
	}
	talismans := []string{
		"Talisman of Progress", "Talisman of Dominance", "Talisman of Indulgence",
		"Talisman of Impulse", "Talisman of Unity", "Talisman of Hierarchy",
		"Talisman of Creativity", "Talisman of Resilience", "Talisman of Conviction",
		"Talisman of Curiosity",
	}
	for _, n := range talismans {
		if !isTalisman(n) {
			t.Errorf("isTalisman(%q) = false, want true", n)
		}
	}
	moxes := []string{
		"Mox Pearl", "Mox Sapphire", "Mox Jet", "Mox Ruby", "Mox Emerald",
		"Mox Diamond", "Mox Opal", "Mox Amber", "Mox Tantalite", "Chrome Mox",
		"Mox Pyrite", "Moxite",
	}
	for _, n := range moxes {
		if !isMox(n) {
			t.Errorf("isMox(%q) = false, want true", n)
		}
	}

	// Negative direction — these predicates gate real mana behaviour, so a
	// table that answered true for everything would pass every test above.
	for _, n := range []string{"", "Sol Ring", "Signet", "Mox", "Arcane Signet"} {
		if isSignet(n) || isTalisman(n) || isMox(n) {
			t.Errorf("%q matched a mana-artifact predicate but is in none of the three lists", n)
		}
	}
}
