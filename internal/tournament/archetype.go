package tournament

import "strings"

// Archetype is a canonical Commander deck archetype label. Values
// mirror cmd/hexdek-freya/archetypes.go::Archetypes so a deck Freya
// classifies as "Combo / Infinite" can be passed through verbatim.
//
// The internal/tournament package can't import cmd/hexdek-freya (it's
// package main), so the names are duplicated rather than referenced.
// Keep this list in sync: when Freya gains a new archetype, add the
// matching constant here and (if applicable) extend opposingArchetypes
// with its natural counter-matchups.
type Archetype string

// Canonical archetype names. Match Freya verbatim, including casing,
// spacing, and the " / " separator. Callers that build Archetype values
// from arbitrary strings (e.g. a Freya report dump) should pass through
// NormalizeArchetype to absorb casing/whitespace drift.
const (
	ArchetypeUnknown        Archetype = ""
	ArchetypeVoltron        Archetype = "Voltron"
	ArchetypeAggroGoWide    Archetype = "Aggro / Go Wide"
	ArchetypeExtraCombats   Archetype = "Extra Combats"
	ArchetypeComboInfinite  Archetype = "Combo / Infinite"
	ArchetypeStorm          Archetype = "Storm"
	ArchetypeControl        Archetype = "Control"
	ArchetypeStax           Archetype = "Stax"
	ArchetypeAristocrats    Archetype = "Aristocrats"
	ArchetypeArtifacts      Archetype = "Artifacts"
	ArchetypeEnchantress    Archetype = "Enchantress"
	ArchetypeReanimator     Archetype = "Reanimator"
	ArchetypeLandsMatter    Archetype = "Lands Matter"
	ArchetypeTribal         Archetype = "Tribal"
	ArchetypeSuperfriends   Archetype = "Superfriends"
	ArchetypeMill           Archetype = "Mill"
	ArchetypeLifegain       Archetype = "Lifegain"
	ArchetypeDiscard        Archetype = "Discard / Hand Attack"
	ArchetypeBlinkFlicker   Archetype = "Blink / Flicker"
	ArchetypeSpellslinger   Archetype = "Spellslinger"
	ArchetypeCountersMatter Archetype = "Counters Matter"
	ArchetypeTheftClone     Archetype = "Theft / Clone"
	ArchetypeNinjutsu       Archetype = "Ninjutsu / Evasion"
)

// NormalizeArchetype lowercases + trims a free-form archetype string
// and re-maps to one of the canonical constants if it matches. Returns
// ArchetypeUnknown for unrecognized input. Intended for Freya-dump
// round-trips where casing or trailing whitespace might have drifted.
func NormalizeArchetype(s string) Archetype {
	clean := strings.ToLower(strings.TrimSpace(s))
	if clean == "" {
		return ArchetypeUnknown
	}
	for _, a := range allArchetypes {
		if strings.ToLower(string(a)) == clean {
			return a
		}
	}
	return ArchetypeUnknown
}

var allArchetypes = []Archetype{
	ArchetypeVoltron, ArchetypeAggroGoWide, ArchetypeExtraCombats,
	ArchetypeComboInfinite, ArchetypeStorm,
	ArchetypeControl, ArchetypeStax,
	ArchetypeAristocrats, ArchetypeArtifacts, ArchetypeEnchantress,
	ArchetypeReanimator, ArchetypeLandsMatter, ArchetypeTribal,
	ArchetypeSuperfriends, ArchetypeMill, ArchetypeLifegain,
	ArchetypeDiscard, ArchetypeBlinkFlicker, ArchetypeSpellslinger,
	ArchetypeCountersMatter, ArchetypeTheftClone, ArchetypeNinjutsu,
}

// opposingArchetypes is the static "natural counter" table. Entries
// are symmetric: if (a, b) is in the map, OpposingArchetypes returns
// true for both (a, b) and (b, a). Each row picks the matchups where
// the two archetypes have a clear strategic axis of conflict —
// not every plausible interaction, just the canonical
// rock-paper-scissors pairings any MTG player would name without
// thinking:
//
//   Combo  ↔ Stax     — Stax taxes/locks the combo turn; combo
//                       races stax before the lock completes
//   Combo  ↔ Control  — Control answers combo pieces with counters
//                       and removal; combo punches through with
//                       protection / redundancy
//   Storm  ↔ Stax     — Stax denies mana / cast triggers; Storm
//                       needs uninterrupted chains
//   Storm  ↔ Control  — Control counters key spells in the chain
//   AggroGoWide ↔ Control       — Boardwipes erase wide boards
//   AggroGoWide ↔ Stax          — Stax taxes cheap creatures
//   Voltron ↔ Theft/Clone       — Theft steals the suited-up
//                                 commander; Voltron has nothing
//                                 to fall back on
//   Reanimator ↔ Control        — Counters + graveyard hate; the
//                                 canonical graveyard-deck answer
//   Reanimator ↔ Stax           — Stax slows reanimation setup
//   Aristocrats ↔ Stax          — Stax punishes the sacrifice
//                                 engine's permanent churn
//   Spellslinger ↔ Stax         — Tax effects punish high
//                                 spell-density turns
//   Tribal ↔ Control            — Boardwipes erase the tribe
//   Enchantress ↔ Control       — Enchantment removal + counters
//   Mill ↔ Control              — Counters protect the win
//   Superfriends ↔ AggroGoWide  — Wide boards attack planeswalkers
//   Superfriends ↔ Voltron      — Single big creature attacks a
//                                 walker each turn
//
// New archetypes (or Freya additions) should be added here when their
// natural counters are obvious; leaving an archetype out just means
// "no preference" — its pods get seeded without opposition bias.
var opposingArchetypes = map[[2]Archetype]bool{
	pair(ArchetypeComboInfinite, ArchetypeStax):       true,
	pair(ArchetypeComboInfinite, ArchetypeControl):    true,
	pair(ArchetypeStorm, ArchetypeStax):               true,
	pair(ArchetypeStorm, ArchetypeControl):            true,
	pair(ArchetypeAggroGoWide, ArchetypeControl):      true,
	pair(ArchetypeAggroGoWide, ArchetypeStax):         true,
	pair(ArchetypeVoltron, ArchetypeTheftClone):       true,
	pair(ArchetypeReanimator, ArchetypeControl):       true,
	pair(ArchetypeReanimator, ArchetypeStax):          true,
	pair(ArchetypeAristocrats, ArchetypeStax):         true,
	pair(ArchetypeSpellslinger, ArchetypeStax):        true,
	pair(ArchetypeTribal, ArchetypeControl):           true,
	pair(ArchetypeEnchantress, ArchetypeControl):      true,
	pair(ArchetypeMill, ArchetypeControl):             true,
	pair(ArchetypeSuperfriends, ArchetypeAggroGoWide): true,
	pair(ArchetypeSuperfriends, ArchetypeVoltron):     true,
}

// pair returns a canonicalized (lo, hi) key so opposingArchetypes lookups
// are direction-independent. Lexicographic ordering picks the canonical
// form deterministically.
func pair(a, b Archetype) [2]Archetype {
	if string(a) < string(b) {
		return [2]Archetype{a, b}
	}
	return [2]Archetype{b, a}
}

// OpposingArchetypes reports whether two archetypes are natural counters.
// Symmetric: a vs b and b vs a return the same value. Unknown
// archetypes (ArchetypeUnknown / unrecognized strings) never oppose
// anything — there's nothing to oppose if we don't know what they are.
func OpposingArchetypes(a, b Archetype) bool {
	if a == ArchetypeUnknown || b == ArchetypeUnknown {
		return false
	}
	if a == b {
		return false
	}
	return opposingArchetypes[pair(a, b)]
}

// PodHasOpposingPair returns true if any two archetypes in the proposed
// pod oppose each other. Used by SeedPodWithOptions's bias function.
// O(n²) over pod size, which is fine for pods of ≤4 — the typical
// Commander format.
func PodHasOpposingPair(archetypes []Archetype, pod []int) bool {
	if len(archetypes) == 0 || len(pod) < 2 {
		return false
	}
	for i := 0; i < len(pod); i++ {
		if pod[i] < 0 || pod[i] >= len(archetypes) {
			continue
		}
		for j := i + 1; j < len(pod); j++ {
			if pod[j] < 0 || pod[j] >= len(archetypes) {
				continue
			}
			if OpposingArchetypes(archetypes[pod[i]], archetypes[pod[j]]) {
				return true
			}
		}
	}
	return false
}
