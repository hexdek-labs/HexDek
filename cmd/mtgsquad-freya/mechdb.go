package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// MechanicDB -- auto-generated reference database built by scanning the
// entire oracle corpus. Loaded once at startup so Freya knows about EVERY
// mechanic, creature type, and keyword that exists in Magic.
// ---------------------------------------------------------------------------

// MechanicDB holds aggregate statistics about the oracle corpus.
type MechanicDB struct {
	// Creature type -> count of cards with that type (from type_line subtypes)
	CreatureTypes map[string]int

	// Scryfall keyword -> count of cards with that keyword
	Keywords map[string]int

	// Mechanic phrase category -> count of cards mentioning it
	Mechanics map[string]int

	// Mechanic category -> list of card names that belong to it
	CardsByMechanic map[string][]string

	// Cards that explicitly allow more than the normal copy limit
	UnlimitedCopies map[string]bool

	// Pre-built card lists for specific mechanic categories
	RingCards         []string
	DungeonCards      []string
	DayNightCards     []string
	SagaCards         []string
	VehicleCards      []string
	EquipmentCards    []string
	PlaneswalkerCards []string

	// Reverse index: card name -> set of mechanic categories it belongs to
	cardMechanics map[string]map[string]bool
}

// mechanicPhrase maps oracle text substrings to mechanic categories.
// All matching is done against lowercased oracle text.
var mechanicPhrases = map[string]string{
	"the ring tempts you":      "ring",
	"ring-bearer":              "ring",
	"your ring-bearer":         "ring",
	"venture into the dungeon": "dungeon",
	"the undercity":            "dungeon",
	"completed a dungeon":      "dungeon",
	"initiative":               "initiative",
	"daybound":                 "daynight",
	"nightbound":               "daynight",
	"it becomes day":           "daynight",
	"it becomes night":         "daynight",
	"rad counter":              "radiation",
	"energy counter":           "energy",
	"{e}":                      "energy",
	"poison counter":           "poison",
	"infect":                   "infect",
	"toxic":                    "toxic",
	"the monarch":              "monarch",
	"become the monarch":       "monarch",
	"city's blessing":          "ascend",
	"ascend":                   "ascend",
	"partner":                  "partner",
	"choose a background":      "background",
	"friends forever":          "friends_forever",
	"craft":                    "craft",
	"discover":                 "discover",
	"descend":                  "descend",
	"finality counter":         "finality",
	"stun counter":             "stun",
	"shield counter":           "shield",
	"incubator":                "incubate",
	"plot":                     "plot",
	"freerunning":              "freerunning",
	"saddle":                   "saddle",
	"mount":                    "mount",
	"offspring":                "offspring",
	"impending":                "impending",
	"spree":                    "spree",
}

// oracleMechEntry mirrors the Scryfall oracle-cards.json fields we need
// for mechanic scanning.
type oracleMechEntry struct {
	Name       string   `json:"name"`
	OracleText string   `json:"oracle_text"`
	TypeLine   string   `json:"type_line"`
	Layout     string   `json:"layout"`
	Keywords   []string `json:"keywords"`
	CardFaces  []struct {
		Name       string `json:"name"`
		OracleText string `json:"oracle_text"`
		TypeLine   string `json:"type_line"`
	} `json:"card_faces"`
}

// BuildMechanicDB scans oracle-cards.json and builds a MechanicDB.
func BuildMechanicDB(oraclePath string) (*MechanicDB, error) {
	f, err := os.Open(oraclePath)
	if err != nil {
		return nil, fmt.Errorf("open oracle: %w", err)
	}
	defer f.Close()

	var entries []oracleMechEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode oracle: %w", err)
	}

	db := &MechanicDB{
		CreatureTypes:   make(map[string]int),
		Keywords:        make(map[string]int),
		Mechanics:       make(map[string]int),
		CardsByMechanic: make(map[string][]string),
		UnlimitedCopies: make(map[string]bool),
		cardMechanics:   make(map[string]map[string]bool),
	}

	for i := range entries {
		e := &entries[i]
		if e.Name == "" {
			continue
		}
		// Skip non-playable layouts
		switch e.Layout {
		case "art_series", "token", "double_faced_token", "emblem":
			continue
		}

		name := e.Name
		oracleText := e.OracleText
		typeLine := e.TypeLine

		// For DFC/split cards, combine oracle text from all faces
		if oracleText == "" && len(e.CardFaces) > 0 {
			var parts []string
			for _, face := range e.CardFaces {
				if face.OracleText != "" {
					parts = append(parts, face.OracleText)
				}
			}
			oracleText = strings.Join(parts, "\n")
		}
		if typeLine == "" && len(e.CardFaces) > 0 {
			var parts []string
			for _, face := range e.CardFaces {
				if face.TypeLine != "" {
					parts = append(parts, face.TypeLine)
				}
			}
			typeLine = strings.Join(parts, " // ")
		}

		ot := strings.ToLower(oracleText)
		tl := strings.ToLower(typeLine)

		// ── Extract creature types from type_line ──
		extractMechDBCreatureTypes(tl, db)

		// ── Count Scryfall keywords ──
		for _, kw := range e.Keywords {
			db.Keywords[kw]++
		}

		// ── Scan oracle text for mechanic phrases ──
		cardMechs := make(map[string]bool)
		for phrase, category := range mechanicPhrases {
			if strings.Contains(ot, phrase) {
				if !cardMechs[category] {
					cardMechs[category] = true
					db.Mechanics[category]++
					db.CardsByMechanic[category] = append(db.CardsByMechanic[category], name)
				}
			}
		}

		// Store reverse index
		if len(cardMechs) > 0 {
			db.cardMechanics[name] = cardMechs
		}

		// ── Unlimited copies detection ──
		if strings.Contains(ot, "a deck can have any number") {
			db.UnlimitedCopies[name] = true
		}
		// "up to nine cards named" (Nazgul special case)
		if strings.Contains(ot, "up to nine cards named") ||
			strings.Contains(ot, "up to seven cards named") {
			db.UnlimitedCopies[name] = true
		}

		// ── Categorize into pre-built lists ──
		if cardMechs["ring"] {
			db.RingCards = append(db.RingCards, name)
		}
		if cardMechs["dungeon"] || cardMechs["initiative"] {
			db.DungeonCards = append(db.DungeonCards, name)
		}
		if cardMechs["daynight"] {
			db.DayNightCards = append(db.DayNightCards, name)
		}

		// Sagas
		if strings.Contains(tl, "saga") {
			db.SagaCards = append(db.SagaCards, name)
		}

		// Vehicles
		if strings.Contains(tl, "vehicle") {
			db.VehicleCards = append(db.VehicleCards, name)
		}

		// Equipment
		if strings.Contains(tl, "equipment") {
			db.EquipmentCards = append(db.EquipmentCards, name)
		}

		// Planeswalkers
		if strings.Contains(tl, "planeswalker") {
			db.PlaneswalkerCards = append(db.PlaneswalkerCards, name)
		}
	}

	return db, nil
}

// extractMechDBCreatureTypes parses creature types from a lowercased type_line
// and increments counts in the MechanicDB.
func extractMechDBCreatureTypes(typeLine string, db *MechanicDB) {
	// Split on " — " or " // " (for DFC type lines)
	// "legendary creature — human wizard" -> "human wizard"
	// "creature — elf // creature — wolf" -> "elf" and "wolf"
	parts := strings.Split(typeLine, " // ")
	for _, part := range parts {
		dashIdx := strings.Index(part, " — ")
		if dashIdx < 0 {
			dashIdx = strings.Index(part, " - ")
		}
		if dashIdx < 0 {
			continue
		}
		subtypePart := strings.TrimSpace(part[dashIdx+len(" — "):])
		if strings.Contains(part, " - ") && dashIdx == strings.Index(part, " - ") {
			subtypePart = strings.TrimSpace(part[dashIdx+3:])
		}

		// Only extract subtypes from creature/tribal type lines
		prefix := strings.TrimSpace(part[:dashIdx])
		isCreatureType := strings.Contains(prefix, "creature") ||
			strings.Contains(prefix, "kindred") ||
			strings.Contains(prefix, "tribal")
		if !isCreatureType {
			continue
		}

		words := strings.Fields(subtypePart)
		for _, w := range words {
			w = strings.TrimRight(w, ",.")
			if w == "" {
				continue
			}
			db.CreatureTypes[w]++
		}
	}
}

// CardMechanics returns the set of mechanic categories a card belongs to.
func (db *MechanicDB) CardMechanics(cardName string) map[string]bool {
	if db == nil || db.cardMechanics == nil {
		return nil
	}
	return db.cardMechanics[cardName]
}

// LogStats logs a summary of the MechanicDB at startup.
func (db *MechanicDB) LogStats() {
	log.Printf("  mechanic DB: %d creature types, %d keywords, %d mechanics, %d unlimited-copy cards",
		len(db.CreatureTypes), len(db.Keywords), len(db.Mechanics), len(db.UnlimitedCopies))
	log.Printf("    ring:%d dungeon:%d daynight:%d saga:%d vehicle:%d equipment:%d planeswalker:%d",
		len(db.RingCards), len(db.DungeonCards), len(db.DayNightCards),
		len(db.SagaCards), len(db.VehicleCards), len(db.EquipmentCards),
		len(db.PlaneswalkerCards))

	// Top 10 creature types
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range db.CreatureTypes {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	if len(sorted) > 10 {
		sorted = sorted[:10]
	}
	names := make([]string, len(sorted))
	for i, s := range sorted {
		names[i] = fmt.Sprintf("%s(%d)", s.Key, s.Value)
	}
	log.Printf("    top creature types: %s", strings.Join(names, ", "))
}

// ---------------------------------------------------------------------------
// FindMechanicSynergies -- detect mechanic-based synergies in a deck using
// the MechanicDB. This catches themed synergy groups that pairwise analysis
// misses: ring cards synergize with each other, energy cards form a package,
// unlimited-copies cards with high counts are engines, etc.
// ---------------------------------------------------------------------------

func FindMechanicSynergies(profiles []CardProfile, cardQtys map[string]int, db *MechanicDB) []ComboResult {
	if db == nil {
		return nil
	}

	var results []ComboResult

	// ── 1. Group deck cards by mechanic category ──
	deckMechanics := map[string][]string{} // mechanic -> card names in this deck
	for _, p := range profiles {
		mechs := db.CardMechanics(p.Name)
		for mech := range mechs {
			deckMechanics[mech] = append(deckMechanics[mech], p.Name)
		}
	}

	// ── 2. Flag mechanic groups with 3+ cards ──
	for mech, cards := range deckMechanics {
		if len(cards) < 3 {
			continue
		}

		// Deduplicate card names (in case of multiple copies counted once in profiles)
		seen := map[string]bool{}
		var unique []string
		for _, c := range cards {
			if !seen[c] {
				seen[c] = true
				unique = append(unique, c)
			}
		}
		if len(unique) < 3 {
			continue
		}

		// How many cards with this mechanic exist in all of Magic?
		totalInMagic := db.Mechanics[mech]

		// Show up to 5 representative cards
		showCards := unique
		if len(showCards) > 5 {
			showCards = showCards[:5]
		}

		mechLabel := mechanicLabel(mech)
		desc := fmt.Sprintf("%s theme: %d cards in deck (out of %d in Magic) -- %s",
			mechLabel, len(unique), totalInMagic,
			strings.Join(showCards, ", "))
		if len(unique) > 5 {
			desc += fmt.Sprintf(" (+%d more)", len(unique)-5)
		}

		results = append(results, ComboResult{
			Cards:       showCards,
			LoopType:    "synergy",
			Description: desc,
		})
	}

	// ── 3. Unlimited-copies engine detection ──
	if cardQtys != nil {
		for cardName := range db.UnlimitedCopies {
			qty := 0
			// Check all qty entries (case-insensitive match)
			for qName, qVal := range cardQtys {
				if normalizeName(qName) == normalizeName(cardName) {
					qty += qVal
				}
			}
			if qty >= 4 {
				results = append(results, ComboResult{
					Cards:    []string{cardName},
					LoopType: "synergy",
					Description: fmt.Sprintf("same-name engine: %dx %s (unlimited copies card)",
						qty, cardName),
				})
			}
		}
	}

	// ── 4. Tribal density detection ──
	// Count creature types across all deck cards, counting total instances
	// (respecting duplicates like 9x Nazgul) but deduplicating names for display.
	deckTribes := map[string][]string{}     // creature type -> all card names (with dupes)
	deckTribesUniq := map[string][]string{} // creature type -> unique card names
	deckTribesCount := map[string]int{}     // creature type -> total count
	for _, p := range profiles {
		for _, ct := range p.CreatureTypes {
			deckTribes[ct] = append(deckTribes[ct], p.Name)
			deckTribesCount[ct]++
		}
	}
	for tribe, cards := range deckTribes {
		seen := map[string]bool{}
		var unique []string
		for _, c := range cards {
			if !seen[c] {
				seen[c] = true
				unique = append(unique, c)
			}
		}
		deckTribesUniq[tribe] = unique
	}

	// Check for tribal concentration: 5+ creatures of same type
	for tribe, count := range deckTribesCount {
		if count < 5 {
			continue
		}
		uniqueCards := deckTribesUniq[tribe]

		// Check if there's a lord or payoff for this type in the deck
		hasLordOrPayoff := false
		for _, p := range profiles {
			if p.IsTribalLord || p.IsTribalPayoff {
				for _, ct := range p.CreatureTypes {
					if ct == tribe {
						hasLordOrPayoff = true
						break
					}
				}
			}
			if hasLordOrPayoff {
				break
			}
		}

		// How many cards of this type exist in all of Magic?
		totalInMagic := db.CreatureTypes[tribe]

		showCards := uniqueCards
		if len(showCards) > 5 {
			showCards = showCards[:5]
		}

		desc := ""
		if hasLordOrPayoff {
			desc = fmt.Sprintf("tribal synergy (%s): %d creatures (%d unique) + lord/payoff (%d total %s in Magic) -- %s",
				tribe, count, len(uniqueCards), totalInMagic, tribe,
				strings.Join(showCards, ", "))
		} else {
			desc = fmt.Sprintf("tribal density (%s): %d creatures (%d unique, no lord detected, %d total %s in Magic) -- %s",
				tribe, count, len(uniqueCards), totalInMagic, tribe,
				strings.Join(showCards, ", "))
		}
		if len(uniqueCards) > 5 {
			desc += fmt.Sprintf(" (+%d more)", len(uniqueCards)-5)
		}

		results = append(results, ComboResult{
			Cards:       showCards,
			LoopType:    "synergy",
			Description: desc,
		})
	}

	return results
}

// mechanicLabel returns a human-readable label for a mechanic category.
func mechanicLabel(mech string) string {
	labels := map[string]string{
		"ring":            "The Ring",
		"dungeon":         "Dungeon",
		"initiative":      "Initiative",
		"daynight":        "Day/Night",
		"radiation":       "Radiation",
		"energy":          "Energy",
		"poison":          "Poison",
		"infect":          "Infect",
		"toxic":           "Toxic",
		"monarch":         "Monarch",
		"ascend":          "Ascend",
		"partner":         "Partner",
		"background":      "Background",
		"friends_forever": "Friends Forever",
		"craft":           "Craft",
		"discover":        "Discover",
		"descend":         "Descend",
		"finality":        "Finality",
		"stun":            "Stun",
		"shield":          "Shield",
		"incubate":        "Incubate",
		"plot":            "Plot",
		"freerunning":     "Freerunning",
		"saddle":          "Saddle",
		"mount":           "Mount",
		"offspring":       "Offspring",
		"impending":       "Impending",
		"spree":           "Spree",
	}
	if label, ok := labels[mech]; ok {
		return label
	}
	// Simple title case without using deprecated strings.Title
	if len(mech) == 0 {
		return mech
	}
	return strings.ToUpper(mech[:1]) + mech[1:]
}
