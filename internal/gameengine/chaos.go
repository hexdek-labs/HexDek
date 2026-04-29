// Package gameengine — chaos.go
//
// Random deck generator + nightmare board generator for the chaos
// gauntlet stress test. Generates Commander-legal decks from the full
// oracle corpus using pure RNG, and synthesizes random battlefield states
// for SBA/layer/trigger stress testing.
//
// Design goals:
//   - Cards that parse as UnknownEffect should still be playable (they
//     just don't DO anything when resolved). The engine should NOT crash.
//   - The chaos gauntlet tests RESILIENCE — can the engine handle cards
//     it's never been specifically tested with?
//   - Everything is wrapped in recover() at the caller level so a single
//     card crash doesn't stop the whole run.
package gameengine

import (
	"math/rand"
)

// ChaosCard holds the minimal card metadata needed for random deck
// generation. Loaded from Scryfall oracle-cards.json which has
// color_identity, type_line, and all the fields we need.
type ChaosCard struct {
	Name          string
	TypeLine      string
	Types         []string
	ManaCost      string
	CMC           int
	Colors        []string
	ColorIdentity []string
	Power         int
	Toughness     int
	IsLegendary   bool
	IsCreature    bool
	IsLand        bool
	IsBasicLand   bool
}

// ChaosCorpus holds the full oracle corpus categorized for random deck
// generation. Built once at startup, read-only after that.
type ChaosCorpus struct {
	// All cards in the corpus.
	All []*ChaosCard

	// LegendaryCreatures is the subset eligible to be commanders.
	LegendaryCreatures []*ChaosCard

	// NonLand cards (creatures, instants, sorceries, artifacts, enchantments, planeswalkers).
	NonLand []*ChaosCard

	// NonBasicLands is the set of all non-basic land cards.
	NonBasicLands []*ChaosCard

	// BasicLands maps color letter ("W","U","B","R","G") to the basic
	// land name ("Plains","Island","Swamp","Mountain","Forest").
	BasicLands map[string]string
}

// NewChaosCorpus categorizes the given cards into the buckets needed
// for random deck generation. Call once at startup.
func NewChaosCorpus(cards []*ChaosCard) *ChaosCorpus {
	cc := &ChaosCorpus{
		All: cards,
		BasicLands: map[string]string{
			"W": "Plains",
			"U": "Island",
			"B": "Swamp",
			"R": "Mountain",
			"G": "Forest",
		},
	}

	for _, c := range cards {
		if c.IsLegendary && c.IsCreature {
			cc.LegendaryCreatures = append(cc.LegendaryCreatures, c)
		}
		if !c.IsLand {
			cc.NonLand = append(cc.NonLand, c)
		}
		if c.IsLand && !c.IsBasicLand {
			cc.NonBasicLands = append(cc.NonBasicLands, c)
		}
	}
	return cc
}

// ChaosDeck is a randomly generated Commander deck.
type ChaosDeck struct {
	Commander *ChaosCard
	Cards     []string // card names in the 99 (including lands)
}

// GenerateChaosDeck builds a random 100-card Commander deck:
//   - 1 random legendary creature as commander
//   - 35-38 lands (basics matching commander's colors + random nonbasics)
//   - 62-64 random nonland cards matching the commander's color identity
//
// The deck is singleton (no card appears more than once). The commander
// is NOT included in the 99.
func GenerateChaosDeck(cc *ChaosCorpus, rng *rand.Rand) *ChaosDeck {
	if cc == nil || len(cc.LegendaryCreatures) == 0 {
		return nil
	}

	// Pick a random legendary creature as commander.
	commander := cc.LegendaryCreatures[rng.Intn(len(cc.LegendaryCreatures))]

	// Build color identity set for filtering.
	ciSet := make(map[string]bool, 5)
	for _, c := range commander.ColorIdentity {
		ciSet[c] = true
	}

	// If colorless commander, allow all colors for lands but only colorless spells.
	// Actually for Commander, a colorless commander means only colorless cards.
	// But that makes for extremely boring decks. For chaos purposes, we'll
	// follow the actual rules: only cards whose color identity is a subset
	// of the commander's color identity.

	// Determine land count: 35-38.
	landCount := 35 + rng.Intn(4) // 35, 36, 37, or 38

	// Determine how many basics vs nonbasics.
	// For colorless commanders: use Wastes (but we don't have that easily,
	// so just use more nonbasics).
	nonBasicCount := 5 + rng.Intn(6) // 5-10 nonbasics
	basicCount := landCount - nonBasicCount
	if basicCount < 0 {
		basicCount = 0
		nonBasicCount = landCount
	}

	used := make(map[string]bool) // singleton enforcement
	used[commander.Name] = true

	var deckCards []string

	// Add basic lands. Distribute evenly among commander's colors.
	if len(ciSet) > 0 && basicCount > 0 {
		colors := make([]string, 0, 5)
		for c := range ciSet {
			if _, ok := cc.BasicLands[c]; ok {
				colors = append(colors, c)
			}
		}
		if len(colors) > 0 {
			for i := 0; i < basicCount; i++ {
				color := colors[i%len(colors)]
				landName := cc.BasicLands[color]
				// Basics CAN appear multiple times in Commander (exception
				// to singleton rule). We still add them but don't mark as
				// "used" for singleton purposes.
				deckCards = append(deckCards, landName)
			}
		} else {
			// Colorless commander — no basic lands match. Add Wastes or
			// just use nonbasics.
			nonBasicCount += basicCount
			basicCount = 0
		}
	} else if basicCount > 0 {
		// Colorless commander — redistribute to nonbasics.
		nonBasicCount += basicCount
		basicCount = 0
	}

	// Add nonbasic lands. Filter to those whose color identity is a subset
	// of the commander's.
	eligible := make([]*ChaosCard, 0, len(cc.NonBasicLands))
	for _, land := range cc.NonBasicLands {
		if used[land.Name] {
			continue
		}
		if colorIdentitySubset(land.ColorIdentity, ciSet) {
			eligible = append(eligible, land)
		}
	}
	// Shuffle eligible nonbasic lands.
	rng.Shuffle(len(eligible), func(i, j int) { eligible[i], eligible[j] = eligible[j], eligible[i] })
	added := 0
	for _, land := range eligible {
		if added >= nonBasicCount {
			break
		}
		if used[land.Name] {
			continue
		}
		deckCards = append(deckCards, land.Name)
		used[land.Name] = true
		added++
	}

	// Add nonland cards matching color identity. Target: 99 - len(deckCards).
	nonLandTarget := 99 - len(deckCards)
	if nonLandTarget < 0 {
		nonLandTarget = 0
	}

	eligibleNonLand := make([]*ChaosCard, 0, len(cc.NonLand))
	for _, card := range cc.NonLand {
		if used[card.Name] {
			continue
		}
		if colorIdentitySubset(card.ColorIdentity, ciSet) {
			eligibleNonLand = append(eligibleNonLand, card)
		}
	}
	rng.Shuffle(len(eligibleNonLand), func(i, j int) {
		eligibleNonLand[i], eligibleNonLand[j] = eligibleNonLand[j], eligibleNonLand[i]
	})

	added = 0
	for _, card := range eligibleNonLand {
		if added >= nonLandTarget {
			break
		}
		if used[card.Name] {
			continue
		}
		deckCards = append(deckCards, card.Name)
		used[card.Name] = true
		added++
	}

	return &ChaosDeck{
		Commander: commander,
		Cards:     deckCards,
	}
}

// GenerateNightmareBoard creates a random battlefield state with
// `permsPerSeat` random permanents on each seat's battlefield. Returns
// the card names placed on each seat (indexed by seat). Used for
// stress-testing the layer system, SBAs, and trigger checks against
// combinations nobody designed test cases for.
func GenerateNightmareBoard(cc *ChaosCorpus, rng *rand.Rand, nSeats, permsPerSeat int) [][]string {
	if cc == nil || len(cc.All) == 0 {
		return nil
	}

	boards := make([][]string, nSeats)
	used := make(map[string]bool)

	for seat := 0; seat < nSeats; seat++ {
		boards[seat] = make([]string, 0, permsPerSeat)
		for p := 0; p < permsPerSeat; p++ {
			// Pick a random card that could be a permanent (not instant/sorcery).
			for attempts := 0; attempts < 100; attempts++ {
				card := cc.All[rng.Intn(len(cc.All))]
				if used[card.Name] {
					continue
				}
				// Skip instants and sorceries — they can't be permanents.
				isInstantOrSorcery := false
				for _, t := range card.Types {
					if t == "instant" || t == "sorcery" {
						isInstantOrSorcery = true
						break
					}
				}
				if isInstantOrSorcery {
					continue
				}
				boards[seat] = append(boards[seat], card.Name)
				used[card.Name] = true
				break
			}
		}
	}
	return boards
}

// ResolveETBChoiceDefaults applies safe default P/T to creatures that
// would otherwise enter as 0/0 due to unresolved "As ~ enters, choose"
// ETB abilities. In the real game, these cards let the controller pick
// a P/T mode (e.g. Primal Plasma: 3/3, 2/2 flying, or 1/6 defender).
// Without ETB resolution, they'd immediately die to SBA 704.5f.
//
// Strategy:
//   - */* creatures with "As ~ enters" choice text: pick the balanced
//     middle form (typically 2/2 or 3/3). Since we can't parse all
//     forms, set to max(1, printed_power) / max(1, printed_toughness).
//   - 0/0 creatures that "enter with +1/+1 counters": add a baseline
//     counter since we don't resolve mana-spend or board-count ETBs.
//   - All other 0/0 creatures: leave alone (they may have CDA-based
//     P/T from static abilities like Tarmogoyf that the layer system
//     handles).
//
// Returns true if a modification was applied.
func ResolveETBChoiceDefaults(perm *Permanent) bool {
	if perm == nil || perm.Card == nil {
		return false
	}

	card := perm.Card

	// Only process creatures.
	isCreature := false
	for _, t := range card.Types {
		if t == "creature" {
			isCreature = true
			break
		}
	}
	if !isCreature {
		return false
	}

	// Only process cards with 0/0 base P/T.
	if card.BasePower != 0 || card.BaseToughness != 0 {
		return false
	}

	// Check oracle text for ETB-choice patterns via AST.
	oracleText := OracleTextLower(card)

	// Pattern 1: "As ~ enters" + choice-based P/T (Primal Plasma, Primal Clay, etc.)
	// These cards let you pick a P/T configuration. Default to 3/3 as the
	// most common balanced form across the 13 known cards.
	if hasETBChoicePattern(oracleText) {
		card.BasePower = 3
		card.BaseToughness = 3
		return true
	}

	// Pattern 2: "enters with" + "+1/+1 counter" (Marath, Verazol, Ulasht, etc.)
	// These cards get counters based on mana spent or board state. In chaos
	// mode we can't compute that, so add a baseline counter.
	if hasETBCounterPattern(oracleText) {
		if perm.Counters == nil {
			perm.Counters = make(map[string]int)
		}
		if perm.Counters["+1/+1"] == 0 {
			perm.Counters["+1/+1"] = 3 // baseline 3 counters
		}
		return true
	}

	return false
}

// HasETBChoicePatternExported is the exported version of hasETBChoicePattern
// for use by the chaos gauntlet card builder (cmd/mtgsquad-loki).
func HasETBChoicePatternExported(oracle string) bool {
	return hasETBChoicePattern(oracle)
}

// hasETBChoicePattern returns true if oracle text matches "As ~ enters"
// combined with "choose" or "becomes your choice" — the pattern for
// cards that let you pick a P/T form (Primal Plasma, Corrupted Shapeshifter,
// Aquamorph Entity, etc.).
func hasETBChoicePattern(oracle string) bool {
	if oracle == "" {
		return false
	}
	// "as this creature enters" or "as ~ enters"
	hasEnters := contains(oracle, "as this creature enters") ||
		contains(oracle, "as it enters") ||
		contains(oracle, "as this enters") ||
		contains(oracle, "as ~ enters")
	if !hasEnters {
		return false
	}
	// Must have choice-related text
	return contains(oracle, "choose") ||
		contains(oracle, "becomes your choice") ||
		contains(oracle, "choose a") ||
		contains(oracle, "it becomes")
}

// hasETBCounterPattern returns true if oracle text matches "enters with"
// combined with "+1/+1 counter" — cards like Marath, Verazol, Ulasht.
func hasETBCounterPattern(oracle string) bool {
	if oracle == "" {
		return false
	}
	return contains(oracle, "enters with") &&
		contains(oracle, "+1/+1 counter")
}

// contains is a simple case-insensitive substring check. Oracle text
// from OracleTextLower is already lowercased.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// colorIdentitySubset returns true if every color in `cardCI` is
// present in `commanderCI`. A card with no color identity (colorless)
// is always a subset.
func colorIdentitySubset(cardCI []string, commanderCI map[string]bool) bool {
	for _, c := range cardCI {
		if !commanderCI[c] {
			return false
		}
	}
	return true
}
