package main

import (
	"strings"
	"testing"
)

// R60 commander-synergy-deepen wave (2026-05-30): the original
// 14-theme matcher reported obviously-wrong synergy values on
// every named-tribe commander we surveyed (Ur-Dragon 6%, Krenko
// 9%, Marrow-Gnawer 20%, Kaalia 23%, Yuriko 37%, Voja 48%, Edgar
// Markov 51%). Root cause: the `tribal` theme patterns
// ("creature type", "all creatures of the chosen type",
// "creatures you control get") never matched real commander text
// like "Goblins you control" / "each Vampire" / "Dragon spells".
// This file pins:
//   1. detectCommanderTribes correctly extracts the tribal axis
//      from each surveyed commander's actual oracle text and
//      correctly returns nil for non-tribal commanders that
//      merely SHARE a creature subtype (Atraxa Phyrexian/Angel/
//      Horror, Korvold Dragon Noble).
//   2. hasTribeToken's boundary check rejects substring false
//      positives like "Dragonfly".
//   3. The 6 newly-added theme patterns (damage, cheat_into_play,
//      top_of_library, cost_reduction, power_matters, life_loss)
//      light up on canonical commander oracle text and are not
//      pattern-clobbered by the existing 14 themes.
//   4. End-to-end synergy% on a synthetic Krenko deck rises into
//      the 40%+ tribal range it should have read all along.

// commanderSynergyTestOracle stubs an oracleDB by name with both
// oracle text AND type line (the tribal detector needs TypeLine
// for the prior fold-in heuristic, which is currently disabled —
// but the synergy MATCH loop reads TypeLine, so the stub must
// populate both).
func commanderSynergyTestOracle(entries map[string]struct{ Oracle, TypeLine string }) *oracleDB {
	db := &oracleDB{byName: map[string]*oracleEntry{}}
	for name, body := range entries {
		e := &oracleEntry{Name: name, OracleText: body.Oracle, TypeLine: body.TypeLine}
		db.byName[normalizeName(name)] = e
		db.byName[strings.ToLower(strings.TrimSpace(name))] = e
	}
	return db
}

// TestDetectCommanderTribes_NamedTribes pins the tribal-axis
// extraction for each surveyed commander against their real
// Scryfall oracle text. Asserts the EXACT set of tribes — too
// few = synergy% will undercount; too many = unrelated decks
// will overcount.
func TestDetectCommanderTribes_NamedTribes(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
		want   []string
	}{
		{
			name:   "Voja, Jaws of the Conclave",
			oracle: "Vigilance, trample, ward {3}\nWhenever Voja attacks, put X +1/+1 counters on each creature you control, where X is the number of Elves you control. Draw a card for each Wolf you control.",
			want:   []string{"Elf", "Wolf"},
		},
		{
			name:   "Edgar Markov",
			oracle: "Eminence — Whenever you cast another Vampire spell, if Edgar is in the command zone or on the battlefield, create a 1/1 black Vampire creature token.\nFirst strike, haste\nWhenever Edgar attacks, put a +1/+1 counter on each Vampire you control.",
			want:   []string{"Vampire"},
		},
		{
			name:   "Krenko, Mob Boss",
			oracle: "{T}: Create X 1/1 red Goblin creature tokens, where X is the number of Goblins you control.",
			want:   []string{"Goblin"},
		},
		{
			name:   "Kaalia of the Vast",
			oracle: "Flying\nWhenever Kaalia attacks an opponent, you may put an Angel, Demon, or Dragon creature card from your hand onto the battlefield tapped and attacking that opponent.",
			want:   []string{"Angel", "Demon", "Dragon"},
		},
		{
			name:   "The Ur-Dragon",
			oracle: "Eminence — As long as The Ur-Dragon is in the command zone or on the battlefield, other Dragon spells you cast cost {1} less to cast.\nFlying\nWhenever one or more Dragons you control attack, draw that many cards, then you may put a permanent card from your hand onto the battlefield.",
			want:   []string{"Dragon"},
		},
		{
			name:   "Marrow-Gnawer",
			oracle: "All Rats have fear.\n{T}, Sacrifice a Rat: Create X 1/1 black Rat creature tokens, where X is the number of Rats you control.",
			want:   []string{"Rat"},
		},
		{
			name:   "Yuriko, the Tiger's Shadow",
			oracle: "Commander ninjutsu {U}{B}\nWhenever a Ninja you control deals combat damage to a player, reveal the top card of your library and put that card into your hand. Each opponent loses life equal to that card's mana value.",
			want:   []string{"Ninja"},
		},
	}
	for _, c := range cases {
		got := detectCommanderTribes(c.oracle, "")
		if len(got) != len(c.want) {
			t.Errorf("%s: tribes len got %d (%v) want %d (%v)", c.name, len(got), got, len(c.want), c.want)
			continue
		}
		for i, w := range c.want {
			if got[i] != w {
				t.Errorf("%s: tribes[%d] got %q want %q (full got=%v want=%v)", c.name, i, got[i], w, got, c.want)
			}
		}
	}
}

// TestDetectCommanderTribes_NonTribalCommandersReturnEmpty pins
// the negative case — non-tribal commanders whose TYPELINE
// includes a tracked creature subtype must NOT register as
// tribal. This is the regression that pushed Atraxa from 17% to
// a false 23% during dev when the TypeLine fold-in was on.
func TestDetectCommanderTribes_NonTribalCommandersReturnEmpty(t *testing.T) {
	cases := []struct {
		name     string
		oracle   string
		typeLine string
	}{
		{
			name:     "Atraxa, Praetors' Voice",
			oracle:   "Flying, vigilance, deathtouch, lifelink\nAt the beginning of your end step, proliferate. (Choose any number of permanents and/or players, then give each another counter of each kind already there.)",
			typeLine: "Legendary Creature — Phyrexian Angel Horror",
		},
		{
			name:     "Korvold, Fae-Cursed King",
			oracle:   "Flying\nWhenever Korvold enters or attacks, sacrifice another permanent.\nWhenever you sacrifice a permanent, put a +1/+1 counter on Korvold and draw a card.",
			typeLine: "Legendary Creature — Dragon Noble",
		},
	}
	for _, c := range cases {
		got := detectCommanderTribes(c.oracle, c.typeLine)
		if len(got) != 0 {
			t.Errorf("%s: expected no tribes, got %v", c.name, got)
		}
	}
}

// TestHasTribeToken_BoundaryGuards pins the boundary check —
// "Dragonfly" must not match the Dragon tribe, "Catfish" must
// not match Cat. Without the boundary guard, every commander
// would over-flag these substrings.
func TestHasTribeToken_BoundaryGuards(t *testing.T) {
	cases := []struct {
		text, tribe string
		want        bool
	}{
		{"Whenever a Dragon attacks, ...", "Dragon", true},
		{"Whenever a Dragonfly attacks, ...", "Dragon", false},
		{"Create three Goblins.", "Goblin", true},
		{"Create three Goblinoids.", "Goblin", false},
		{"Catfish swim by.", "Cat", false},
		{"This is a Cat.", "Cat", true},
		{"Each Elf you control...", "Elf", true},
		{"Each Elfwoman you control...", "Elf", false},
		// case-sensitivity: lowercase "rat" must not match the Rat tribe
		{"You may berate the opponent.", "Rat", false},
		{"All Rats have fear.", "Rat", true},
	}
	for _, c := range cases {
		got := hasTribeToken(c.text, c.tribe)
		if got != c.want {
			t.Errorf("hasTribeToken(%q, %q) = %v, want %v", c.text, c.tribe, got, c.want)
		}
	}
}

// TestNewThemePatterns_LightUpOnCanonicalOracle pins each of the
// 6 new themes against a canonical example. If any of these
// regress to no-match, deck synergy% will silently fall back to
// pre-R60 behavior.
func TestNewThemePatterns_LightUpOnCanonicalOracle(t *testing.T) {
	// each case = the lowercased oracle text we expect to trigger
	// the named theme.
	cases := []struct {
		theme  string
		oracle string
	}{
		{"damage", "niv-mizzet deals 1 damage to any target."},
		{"damage", "this creature deals damage equal to its power to target opponent."},
		{"cheat_into_play", "you may put a creature card from your hand onto the battlefield."},
		{"cheat_into_play", "you may cast that spell without paying its mana cost."},
		{"top_of_library", "look at the top three cards of your library."},
		{"top_of_library", "you may play the top card of your library."},
		{"cost_reduction", "this spell costs {1} less to cast."},
		{"cost_reduction", "dragon spells you cast cost {1} less to cast."},
		{"power_matters", "creature spells you cast with power 4 or greater cost {2} less."},
		{"power_matters", "target creature with the greatest power gets +2/+2."},
		{"life_loss", "each opponent loses 2 life."},
		{"life_loss", "whenever an opponent loses life, draw a card."},
	}
	for _, c := range cases {
		hit := false
		for _, tp := range commanderThemePatterns {
			if tp.Theme != c.theme {
				continue
			}
			for _, pat := range tp.Patterns {
				if strings.Contains(c.oracle, pat) {
					hit = true
					break
				}
			}
			break
		}
		if !hit {
			t.Errorf("theme %q failed to match canonical oracle text: %q", c.theme, c.oracle)
		}
	}
}

// TestNewThemes_PresentInCommanderThemePatterns guards against
// accidental deletion of any new theme entry during future
// refactors. Each name MUST appear exactly once.
func TestNewThemes_PresentInCommanderThemePatterns(t *testing.T) {
	required := []string{"damage", "cheat_into_play", "top_of_library", "cost_reduction", "power_matters", "life_loss"}
	for _, name := range required {
		count := 0
		for _, tp := range commanderThemePatterns {
			if tp.Theme == name {
				count++
			}
		}
		if count != 1 {
			t.Errorf("theme %q appears %d times in commanderThemePatterns, want exactly 1", name, count)
		}
	}
}

// TestComputeCommanderSynergy_KrenkoGoblinTribal is the
// end-to-end pin. Krenko (Goblin tribal commander) with a small
// deck of 4 goblin creatures + 1 non-goblin filler + 5 lands
// should:
//   - detect tribe ["Goblin"]
//   - add "tribal" to CommanderThemes
//   - hit 4/5 nonland match (80%)
//
// The 5/10 vs 4/5 distinction is the structural guarantee that
// LAND cards are excluded from the denominator — a regression
// where lands were counted would tank the % to 40%.
func TestComputeCommanderSynergy_KrenkoGoblinTribal(t *testing.T) {
	oracle := commanderSynergyTestOracle(map[string]struct{ Oracle, TypeLine string }{
		"Krenko, Mob Boss": {
			Oracle:   "{T}: Create X 1/1 red Goblin creature tokens, where X is the number of Goblins you control.",
			TypeLine: "Legendary Creature — Goblin Warrior",
		},
		"Goblin Chieftain":  {Oracle: "Goblin creatures you control have haste.", TypeLine: "Creature — Goblin Warrior"},
		"Goblin Matron":     {Oracle: "When this enters, search your library for a Goblin card.", TypeLine: "Creature — Goblin"},
		"Goblin Recruiter":  {Oracle: "When this enters, search your library for any number of Goblin cards.", TypeLine: "Creature — Goblin"},
		"Goblin Warchief":   {Oracle: "Goblin spells you cast cost {1} less. Goblins you control have haste.", TypeLine: "Creature — Goblin Warrior"},
		"Llanowar Elves":    {Oracle: "{T}: Add {G}.", TypeLine: "Creature — Elf Druid"},
		"Mountain":          {Oracle: "", TypeLine: "Basic Land — Mountain"},
		"Cavern of Souls":   {Oracle: "Naming a creature type.", TypeLine: "Land"},
		"Sulfurous Springs": {Oracle: "Tap to add mana.", TypeLine: "Land"},
		"Bloodstained Mire": {Oracle: "Fetch land.", TypeLine: "Land"},
		"Castle Embereth":   {Oracle: "...", TypeLine: "Land"},
	})

	report := &FreyaReport{
		Commander: "Krenko, Mob Boss",
		Profiles: []CardProfile{
			{Name: "Goblin Chieftain", TypeLine: "Creature — Goblin Warrior"},
			{Name: "Goblin Matron", TypeLine: "Creature — Goblin"},
			{Name: "Goblin Recruiter", TypeLine: "Creature — Goblin"},
			{Name: "Goblin Warchief", TypeLine: "Creature — Goblin Warrior"},
			{Name: "Llanowar Elves", TypeLine: "Creature — Elf Druid"},
			{Name: "Mountain", TypeLine: "Basic Land — Mountain", IsLand: true},
			{Name: "Cavern of Souls", TypeLine: "Land", IsLand: true},
			{Name: "Sulfurous Springs", TypeLine: "Land", IsLand: true},
			{Name: "Bloodstained Mire", TypeLine: "Land", IsLand: true},
			{Name: "Castle Embereth", TypeLine: "Land", IsLand: true},
		},
	}

	dp := &DeckProfile{}
	computeCommanderSynergy(dp, report, oracle)

	if len(dp.CommanderTribes) != 1 || dp.CommanderTribes[0] != "Goblin" {
		t.Errorf("CommanderTribes = %v, want [Goblin]", dp.CommanderTribes)
	}
	hasTribal := false
	for _, th := range dp.CommanderThemes {
		if th == "tribal" {
			hasTribal = true
		}
	}
	if !hasTribal {
		t.Errorf("CommanderThemes %v missing 'tribal'", dp.CommanderThemes)
	}
	// 4 goblins out of 5 nonland cards = 0.80
	if dp.SynergyCount != 4 {
		t.Errorf("SynergyCount = %d, want 4 (4 goblins)", dp.SynergyCount)
	}
	if dp.CommanderSynergy < 0.79 || dp.CommanderSynergy > 0.81 {
		t.Errorf("CommanderSynergy = %.3f, want ~0.80", dp.CommanderSynergy)
	}
}

// TestComputeCommanderSynergy_NonTribalCommanderNoTribalTheme
// pins the negative — a Korvold-style sacrifice commander whose
// type line happens to be "Dragon Noble" must NOT register tribal
// (and must NOT get tribal-axis synergy points for unrelated
// dragons in the deck).
func TestComputeCommanderSynergy_NonTribalCommanderNoTribalTheme(t *testing.T) {
	oracle := commanderSynergyTestOracle(map[string]struct{ Oracle, TypeLine string }{
		"Korvold, Fae-Cursed King": {
			Oracle:   "Flying\nWhenever Korvold enters or attacks, sacrifice another permanent.\nWhenever you sacrifice a permanent, put a +1/+1 counter on Korvold and draw a card.",
			TypeLine: "Legendary Creature — Dragon Noble",
		},
		"Random Dragon":   {Oracle: "Flying. {T}: Deal 1 damage.", TypeLine: "Creature — Dragon"},
		"Sakura-Tribe Elder": {Oracle: "Sacrifice this creature: Search your library for a basic land card.", TypeLine: "Creature — Snake Shaman"},
		"Swamp":           {Oracle: "", TypeLine: "Basic Land — Swamp"},
	})
	report := &FreyaReport{
		Commander: "Korvold, Fae-Cursed King",
		Profiles: []CardProfile{
			{Name: "Random Dragon", TypeLine: "Creature — Dragon"},
			{Name: "Sakura-Tribe Elder", TypeLine: "Creature — Snake Shaman"},
			{Name: "Swamp", TypeLine: "Basic Land — Swamp", IsLand: true},
		},
	}

	dp := &DeckProfile{}
	computeCommanderSynergy(dp, report, oracle)

	if len(dp.CommanderTribes) != 0 {
		t.Errorf("CommanderTribes = %v, want empty (Korvold is sacrifice, not Dragon tribal)", dp.CommanderTribes)
	}
	for _, th := range dp.CommanderThemes {
		if th == "tribal" {
			t.Errorf("CommanderThemes %v should not include 'tribal' for non-tribal commander", dp.CommanderThemes)
		}
	}
	// Korvold's themes: sacrifice (from "sacrifice another permanent"),
	// counters (from "+1/+1 counter"), drawing ("draw a card"),
	// combat ("attacks"). "damage" / "lands" theme are NOT detected on
	// Korvold's oracle, so:
	//   - Random Dragon "Flying. {T}: Deal 1 damage." → no Korvold-theme
	//     hit (no "sacrifice"/"counter"/"draw"/"attack" substring).
	//   - Sakura-Tribe Elder "Sacrifice this creature: search ..." →
	//     hits Korvold's `sacrifice` theme via "sacrifice".
	// So SynergyCount == 1 is the correct expectation. The fact that
	// Random Dragon's type happens to be Dragon DOES NOT count — this
	// is the structural pin against tribal-bleed on non-tribal cmdrs.
	if dp.SynergyCount != 1 {
		t.Errorf("SynergyCount = %d, want 1 (only Sakura-Tribe Elder hits 'sacrifice'; Random Dragon must NOT score as Dragon-tribal bleed)", dp.SynergyCount)
	}
}
