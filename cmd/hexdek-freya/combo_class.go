package main

import (
	"strings"
)

// ClassifyComboHeuristic infers a combo class from a name + description
// + piece list when the curated `Class` field is empty. Used for the
// Commander Spellbook import path where combos arrive without a
// pre-tagged class. Returns ComboClassUnknown when no rule matches —
// callers should treat that as a fallback bucket rather than a bug.
//
// Classification proceeds in priority order: the more specific class
// wins. So "infinite drain via Aetherflux storm" tags as
// infinite_drain (the production), not storm_finisher (the vehicle),
// because drain is the killing primitive.
func ClassifyComboHeuristic(name, description string, pieces []string) string {
	hay := strings.ToLower(name + " | " + description)
	for _, p := range pieces {
		hay += " | " + strings.ToLower(p)
	}

	// Order matters: more specific patterns first. The classifier picks
	// the first match because the production semantics of these combos
	// are mostly singular — when two productions are simultaneously
	// possible (mana + ETB from Worldgorger), the operationally
	// dominant one (ETB triggers, which is what the deck assembles
	// outlets around) is what we tag.
	// Rule order matters: specific-shape rules fire before
	// generic-production rules so the classifier picks the operationally
	// dominant class. For example, Worldgorger Dragon + Animate Dead
	// produces both infinite ETB triggers AND infinite mana (via land
	// untap), but the deck assembles outlets around the ETB shape —
	// tagging it infinite_etb leads to better hat affinity weighting
	// than infinite_mana would.
	rules := []struct {
		class string
		match func(string) bool
	}{
		// 1. Win-condition flavors — most specific.
		{ComboClassLibraryExileWin, func(h string) bool {
			return containsAny(h, "thassa's oracle", "laboratory maniac", "jace, wielder", "demonic consultation", "tainted pact", "doomsday", "empty library", "no cards in your library")
		}},
		{ComboClassStormFinisher, func(h string) bool {
			return containsAny(h, "aetherflux reservoir", "storm count", "tendrils of agony", "grapeshot")
		}},

		// 2. Named-engine ETB shapes — fire before generic mana so
		// Worldgorger/Deadeye/Nim Deathmantle aren't miscategorised as
		// infinite_mana when their description mentions the side-effect
		// land untap. The mill/drain exclusion lets Altar-of-Dementia +
		// Karmic-Guide-style descriptions fall through to their kill
		// primitive (mill wins when the description names a mill outlet
		// even though Karmic Guide is in the engine list).
		{ComboClassInfiniteETB, func(h string) bool {
			if containsAny(h, "altar of dementia", "mill ", "drain each opponent") {
				return false
			}
			return containsAny(h, "worldgorger dragon", "karmic guide", "reveillark", "nim deathmantle", "displacer kitten", "phyrexian altar", "gravecrawler", "persist creature", "blink trigger", "infinite etb", "infinite death triggers", "infinite recursion")
		}},

		// 3. Payoffs / multipliers / sinks — these consume the
		// production of another combo and must claim their description
		// before the producing-rule does ("Staff of Domination + Infinite
		// Mana" is a sink line, not a producer line).
		{ComboClassManaSink, func(h string) bool {
			return containsAny(h, "staff of domination", "any mana sink")
		}},
		{ComboClassBlinkEngine, func(h string) bool {
			return containsAny(h, "conjurer's closet", "thassa, deep-dwelling", "ephemerate", "blink every end step", "flicker each end step")
		}},
		{ComboClassETBPayoff, func(h string) bool {
			return containsAny(h, "purphoros", "impact tremors", "warstorm surge", "terror of the peaks", "each creature etb deals", "every creature etb deals")
		}},
		{ComboClassETBDoubler, func(h string) bool {
			return containsAny(h, "panharmonicon", "yarok", "doubles all", "doubles etb", "trigger twice")
		}},

		// 4. Production-class detectors — generic shapes that fire when
		// no specific engine or sink has claimed the description.
		{ComboClassInfiniteDrain, func(h string) bool {
			return containsAny(h, "exquisite blood", "sanguine bond", "vito, thorn", "blight-priest", "defiant bloodlord", "gray merchant", "blood artist", "zulaport cutthroat", "drain each opponent", "lose life") &&
				containsAny(h, "drain", "lose life", "life loss", "exquisite blood", "sanguine")
		}},
		{ComboClassInfiniteMill, func(h string) bool {
			return containsAny(h, "altar of dementia", "grinding station", "underworld breach", "mill ", "mills ", "mill your", "mill each", "mill opponent")
		}},
		{ComboClassInfiniteTokens, func(h string) bool {
			return containsAny(h, "kiki-jiki", "splinter twin", "create a token copy", "create a copy", "create token", "create a creature token", "hasty tokens", "infinite tokens")
		}},
		{ComboClassInfiniteDamage, func(h string) bool {
			return containsAny(h, "walking ballista", "triskelion", "infinite damage", "deals damage", "remove a counter", "ping") &&
				!containsAny(h, "drain") // drain wins above
		}},
		{ComboClassInfiniteMana, func(h string) bool {
			return containsAny(h, "infinite mana", "infinite colorless", "untap all", "untap lands", "net +", "treasure", "basalt monolith", "dramatic reversal", "dockside", "palinchron", "peregrine drake")
		}},
		{ComboClassCombatFinisher, func(h string) bool {
			return containsAny(h, "craterhoof behemoth", "overrun", "triumph of the hordes", "infect lethal", "lethal swing", "trample lethal")
		}},
	}

	for _, r := range rules {
		if r.match(hay) {
			return r.class
		}
	}
	return ComboClassUnknown
}

// ClassifySpellbookImported applies the heuristic classifier to every
// entry in `imports` whose Class field is empty, mutating in place. The
// curated KnownCombos list ships with Class pre-populated; this fills
// in the import-time gap so allKnownCombos() returns a uniformly-
// classified set regardless of source.
func ClassifySpellbookImported(imports []KnownCombo) {
	for i := range imports {
		if imports[i].Class != "" {
			continue
		}
		imports[i].Class = ClassifyComboHeuristic(imports[i].Name, imports[i].Description, imports[i].Pieces)
	}
}
