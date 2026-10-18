package hat

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r174 regressions for the strings.ToLower removals in
// fastManaExtraYield / typeLineContains / handQualityBonus /
// ritualNetMana / cardHeuristic.
//
// Every test below pins BOTH directions: a value the predicate must
// answer true/nonzero for, and one it must answer false/zero for. The
// helper-level tests additionally pin the exact expression each call
// site replaced, so a future edit to foldContainsASCII /
// intByFoldedName that drifts from strings.ToLower semantics fails
// here rather than silently changing eval scores.

// ---------------------------------------------------------------------
// Helper equivalence — the thing all five call sites stand on
// ---------------------------------------------------------------------

func TestR174_FoldContainsASCII_MatchesToLowerContains(t *testing.T) {
	hays := []string{
		"", "creature", "Creature", "CREATURE", "Legendary Creature",
		"artifact", "Artifact Creature", "land", "Land", "enchantment",
		"Sol Ring", "Demonic Tutor", "Worldly Tutor", "DEMONIC TUTOR",
		"Vampiric tutor", "Tutorial", "Statute of Denial", "Rings of Brighthearth",
		// non-ASCII: must take the strings.ToLower fallback and still agree
		"Lim-Dûl's Vault", "Æther Vial", "Juzám Djinn", "Márton Stromgald",
		"Séance", "ニンジャ", "Legendary Creature — Dragon",
	}
	needles := []string{
		"", "creature", "Creature", "artifact", "land", "enchantment",
		"instant", "sorcery", "planeswalker", "aura", "equipment",
		"commander", "tutor", "Tutor", "TUTOR", "dûl", "DÛL", "æ", "Æ",
		"—", "dragon", "zzz",
	}
	for _, h := range hays {
		for _, n := range needles {
			want := strings.Contains(strings.ToLower(h), strings.ToLower(n))
			got := foldContainsASCII(h, n)
			if got != want {
				t.Errorf("foldContainsASCII(%q, %q) = %v, want %v", h, n, got, want)
			}
		}
	}
}

func TestR174_IntByFoldedName_MatchesToLowerMapIndex(t *testing.T) {
	m := map[string]int{
		"sol ring": 1, "mana vault": 2, "basalt monolith": 2,
		"dark ritual": 2, "seething song": 2,
	}
	names := []string{
		"Sol Ring", "sol ring", "SOL RING", "sOl RiNg", "Mana Vault",
		"Basalt Monolith", "Dark Ritual", "Seething Song",
		"Plains", "Black Lotus", "", "Mox Sapphire",
		// non-ASCII and over-length both take the allocating fallback
		"Lim-Dûl's Vault", "Æther Vial",
		strings.Repeat("A", foldKeyMax+1),
		strings.Repeat("A", foldKeyMax),
	}
	for _, n := range names {
		want := m[strings.ToLower(n)]
		got := intByFoldedName(m, n)
		if got != want {
			t.Errorf("intByFoldedName(m, %q) = %d, want %d", n, got, want)
		}
	}
}

// intByFoldedName must not allocate on the ASCII path — that is the
// entire point of the change. 1 alloc/op means the stack buffer
// escaped and the optimisation is gone.
func TestR174_IntByFoldedName_ZeroAlloc(t *testing.T) {
	m := map[string]int{"sol ring": 1}
	got := testing.AllocsPerRun(200, func() {
		_ = intByFoldedName(m, "Sol Ring")
		_ = intByFoldedName(m, "Lightning Bolt")
	})
	if got != 0 {
		t.Errorf("intByFoldedName allocated %.1f times per run, want 0", got)
	}
}

func TestR174_FoldContainsASCII_ZeroAlloc(t *testing.T) {
	got := testing.AllocsPerRun(200, func() {
		_ = foldContainsASCII("Demonic Tutor", "tutor")
		_ = foldContainsASCII("creature", "land")
	})
	if got != 0 {
		t.Errorf("foldContainsASCII allocated %.1f times per run, want 0", got)
	}
}

// ---------------------------------------------------------------------
// fastManaExtraYield — both directions
// ---------------------------------------------------------------------

func TestR174_FastManaExtraYield_BothDirections(t *testing.T) {
	// MATCH: canonical capitalisation, the shape the engine actually
	// produces from the oracle corpus.
	seat := &gameengine.Seat{Idx: 0}
	newTestPermanent(seat, newTestCardMinimal("Mana Vault", []string{"artifact"}, 0, nil), 0, 0)
	if got := fastManaExtraYield(seat); got != 2 {
		t.Errorf("Mana Vault: got %d want 2", got)
	}

	// MATCH with a case variant — the ToLower the fold replaced existed
	// to normalise exactly this.
	seat2 := &gameengine.Seat{Idx: 0}
	newTestPermanent(seat2, newTestCardMinimal("SOL RING", []string{"artifact"}, 0, nil), 0, 0)
	if got := fastManaExtraYield(seat2); got != 1 {
		t.Errorf("SOL RING: got %d want 1", got)
	}

	// NON-MATCH: a mox taps for 1, deliberately excluded from the map.
	seat3 := &gameengine.Seat{Idx: 0}
	newTestPermanent(seat3, newTestCardMinimal("Mox Sapphire", []string{"artifact"}, 0, nil), 0, 0)
	if got := fastManaExtraYield(seat3); got != 0 {
		t.Errorf("Mox Sapphire: got %d want 0", got)
	}

	// NON-MATCH: a non-ASCII name goes through the fallback and must
	// still miss rather than panic or over-match.
	seat4 := &gameengine.Seat{Idx: 0}
	newTestPermanent(seat4, newTestCardMinimal("Lim-Dûl's Vault", []string{"artifact"}, 0, nil), 0, 0)
	if got := fastManaExtraYield(seat4); got != 0 {
		t.Errorf("Lim-Dûl's Vault: got %d want 0", got)
	}
}

// ---------------------------------------------------------------------
// ritualNetMana — both directions
// ---------------------------------------------------------------------

func TestR174_RitualNetMana_BothDirections(t *testing.T) {
	if got := ritualNetMana(newTestCardMinimal("Dark Ritual", []string{"instant"}, 1, nil)); got != 2 {
		t.Errorf("Dark Ritual: got %d want 2", got)
	}
	if got := ritualNetMana(newTestCardMinimal("dark RITUAL", []string{"instant"}, 1, nil)); got != 2 {
		t.Errorf("dark RITUAL: got %d want 2", got)
	}
	// Desperate Ritual is intentionally NOT in the map (net 0 standalone).
	if got := ritualNetMana(newTestCardMinimal("Desperate Ritual", []string{"instant"}, 3, nil)); got != 0 {
		t.Errorf("Desperate Ritual: got %d want 0", got)
	}
	if got := ritualNetMana(newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil)); got != 0 {
		t.Errorf("Lightning Bolt: got %d want 0", got)
	}
}

// ---------------------------------------------------------------------
// typeLineContains — both directions
// ---------------------------------------------------------------------

func TestR174_TypeLineContains_BothDirections(t *testing.T) {
	c := newTestCardMinimal("Birds of Paradise", []string{"creature", "bird"}, 1, nil)
	if !typeLineContains(c, "creature") {
		t.Error("creature should match a creature card")
	}
	if typeLineContains(c, "land") {
		t.Error("land must NOT match a creature card")
	}

	// A mixed-case needle must still match: the function's contract is
	// case-insensitive and the fold preserves it.
	if !typeLineContains(c, "Creature") {
		t.Error("mixed-case needle should still match")
	}

	// A mixed-case TYPE TOKEN must still match. Card.Types is built
	// lowercase by deckparser, but the fold means a hand-built or
	// engine-appended mixed-case token behaves identically.
	mixed := &gameengine.Card{Name: "Hand Built", Types: []string{"Artifact", "CREATURE"}}
	if !typeLineContains(mixed, "artifact") {
		t.Error("mixed-case type token should match a lowercase needle")
	}
	if !typeLineContains(mixed, "creature") {
		t.Error("uppercase type token should match a lowercase needle")
	}
	if typeLineContains(mixed, "enchantment") {
		t.Error("enchantment must NOT match an artifact creature")
	}

	// Substring semantics are preserved (not word-equality): the
	// original used strings.Contains and several call sites rely on it.
	if !typeLineContains(c, "creat") {
		t.Error("substring needle should match, as strings.Contains did")
	}

	if typeLineContains(nil, "creature") {
		t.Error("nil card must be false")
	}
}

// ---------------------------------------------------------------------
// handQualityBonus — tutor name probe, both directions
// ---------------------------------------------------------------------

func TestR174_HandQualityBonus_TutorNameBothDirections(t *testing.T) {
	// MATCH by NAME only: oracle text deliberately lacks "search your
	// library for", so the +0.15 can only come from the name probe.
	seat := &gameengine.Seat{Idx: 0}
	seat.Hand = []*gameengine.Card{
		gyCardWithOracle("Worldly Tutor", []string{"instant"}, 1,
			"reveal a creature card from your library and put it on top"),
	}
	if got := handQualityBonus(seat); got != 0.15 {
		t.Errorf("name-only tutor: got %.4f want 0.15", got)
	}

	// MATCH with a case variant.
	seat2 := &gameengine.Seat{Idx: 0}
	seat2.Hand = []*gameengine.Card{
		gyCardWithOracle("WORLDLY TUTOR", []string{"instant"}, 1,
			"reveal a creature card from your library and put it on top"),
	}
	if got := handQualityBonus(seat2); got != 0.15 {
		t.Errorf("uppercase name tutor: got %.4f want 0.15", got)
	}

	// NON-MATCH: no "tutor" in the name, no search text.
	seat3 := &gameengine.Seat{Idx: 0}
	seat3.Hand = []*gameengine.Card{
		gyCardWithOracle("Lightning Bolt", []string{"instant"}, 1,
			"deal 3 damage to any target"),
	}
	if got := handQualityBonus(seat3); got != 0 {
		t.Errorf("non-tutor: got %.4f want 0", got)
	}

	// NON-MATCH with a non-ASCII name — exercises the fallback path.
	seat4 := &gameengine.Seat{Idx: 0}
	seat4.Hand = []*gameengine.Card{
		gyCardWithOracle("Juzám Djinn", []string{"creature"}, 3,
			"at the beginning of your upkeep, this creature deals 1 damage to you"),
	}
	if got := handQualityBonus(seat4); got != 0 {
		t.Errorf("non-ASCII non-tutor: got %.4f want 0", got)
	}
}

// ---------------------------------------------------------------------
// cardHeuristic — the TypeLine theme probe, both directions
// ---------------------------------------------------------------------

// r174CardWithTypeLine builds a card whose TypeLine is set in the
// mixed case the oracle corpus actually carries, which is what made
// the inline strings.ToLower allocate on every scoring pass.
func r174CardWithTypeLine(name, typeLine string, types []string, cmc int) *gameengine.Card {
	c := gyCardWithOracle(name, types, cmc, "vanilla body with no theme words")
	c.TypeLine = typeLine
	return c
}

func TestR174_CardHeuristic_TypeLineThemeBothDirections(t *testing.T) {
	gs := newTestGame(t, 2)

	mk := func(theme string) float64 {
		h := NewYggdrasilHat(&StrategyProfile{
			Archetype:       ArchetypeMidrange,
			CommanderThemes: []string{theme},
		}, 0)
		c := r174CardWithTypeLine("Shivan Dragon", "Creature — Dragon",
			[]string{"creature", "dragon"}, 6)
		return h.cardHeuristic(gs, 0, c)
	}

	// The theme matches the TYPE LINE only (the oracle body has no
	// theme words), so the +0.12 Lovelace bonus isolates the branch the
	// TypeLineLower swap sits in.
	withDragon := mk("dragon")
	withGoblin := mk("goblin")

	if withDragon <= withGoblin {
		t.Errorf("type-line theme match should score higher: dragon=%.4f goblin=%.4f",
			withDragon, withGoblin)
	}
	if delta := withDragon - withGoblin; delta < 0.119 || delta > 0.121 {
		t.Errorf("type-line theme bonus should be exactly +0.12, got %.4f", delta)
	}
}

// TypeLineLower is the memoised form of the expression cardHeuristic
// used inline. Pin the equality directly so the swap cannot drift.
func TestR174_TypeLineLower_EqualsInlineToLower(t *testing.T) {
	for _, tl := range []string{
		"", "Creature — Dragon", "Legendary Artifact Creature — Golem",
		"Land", "LAND", "Snow Land — Forest", "Planeswalker — Jace",
		"Legendary Creature — Human Shaman", "Enchantment — Aura",
	} {
		c := &gameengine.Card{Name: "x", TypeLine: tl}
		if got, want := gameengine.TypeLineLower(c), strings.ToLower(tl); got != want {
			t.Errorf("TypeLineLower(%q) = %q, want %q", tl, got, want)
		}
	}
}
