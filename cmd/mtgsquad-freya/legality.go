package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck legality validation for Commander format.
// ---------------------------------------------------------------------------

// LegalityReport is the result of all five legality checks.
type LegalityReport struct {
	Valid       bool             `json:"valid"`
	CardCount   CardCountCheck   `json:"card_count"`
	ColorID     ColorIdentityCheck `json:"color_identity"`
	Singleton   SingletonCheck   `json:"singleton"`
	BannedCards BannedCheck      `json:"banned_cards"`
	CommanderOK CommanderCheck   `json:"commander"`
	Warnings    []string         `json:"warnings,omitempty"`
	Errors      []string         `json:"errors,omitempty"`
}

// CardCountCheck validates the deck has exactly 100 cards (99+1 or 98+2 with partners).
type CardCountCheck struct {
	Valid      bool   `json:"valid"`
	Expected   int    `json:"expected"`
	Actual     int    `json:"actual"`
	HasPartner bool   `json:"has_partner"`
	Message    string `json:"message,omitempty"`
}

// ColorIdentityCheck validates every card in the 99 is within the commander's color identity.
type ColorIdentityCheck struct {
	Valid      bool                   `json:"valid"`
	CommanderColors []string          `json:"commander_colors"`
	Violations []ColorViolation       `json:"violations,omitempty"`
}

// ColorViolation represents a single card whose color identity exceeds the commander's.
type ColorViolation struct {
	CardName      string   `json:"card_name"`
	CardColors    []string `json:"card_colors"`
	AllowedColors []string `json:"allowed_colors"`
}

// SingletonCheck validates no card appears more than once (with exceptions).
type SingletonCheck struct {
	Valid      bool               `json:"valid"`
	Violations []SingletonViolation `json:"violations,omitempty"`
}

// SingletonViolation represents a card that appears more than once illegally.
type SingletonViolation struct {
	CardName string `json:"card_name"`
	Count    int    `json:"count"`
}

// BannedCheck flags any cards on the Commander banned list.
type BannedCheck struct {
	Valid      bool     `json:"valid"`
	BannedFound []string `json:"banned_found,omitempty"`
}

// CommanderCheck validates the commander is a legal commander card.
type CommanderCheck struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Commander banned list (as of April 2026)
// ---------------------------------------------------------------------------

// Stored as normalized names for matching against oracle lookups.
var commanderBannedList = map[string]bool{
	"ancestral recall":              true,
	"balance":                       true,
	"biorhythm":                     true,
	"black lotus":                   true,
	"braids, cabal minion":          true,
	"channel":                       true,
	"coalition victory":             true,
	"dockside extortionist":         true,
	"emrakul, the aeons torn":       true,
	"erayo, soratami ascendant":     true,
	"falling star":                  true,
	"fastbond":                      true,
	"flash":                         true,
	"gifts ungiven":                 true,
	"golos, tireless pilgrim":       true,
	"griselbrand":                   true,
	"hullbreacher":                  true,
	"iona, shield of emeria":        true,
	"jeweled lotus":                 true,
	"karakas":                       true,
	"leovold, emissary of trest":    true,
	"library of alexandria":         true,
	"limited resources":             true,
	"lutri, the spellchaser":        true,
	"mana crypt":                    true,
	"mox emerald":                   true,
	"mox jet":                       true,
	"mox pearl":                     true,
	"mox ruby":                      true,
	"mox sapphire":                  true,
	"nadu, winged wisdom":           true,
	"panoptic mirror":               true,
	"paradox engine":                true,
	"primeval titan":                true,
	"prophet of kruphix":            true,
	"recurring nightmare":           true,
	"rofellos, llanowar emissary":   true,
	"shahrazad":                     true,
	"sundering titan":               true,
	"sway of the stars":             true,
	"sylvan primordial":             true,
	"time vault":                    true,
	"time walk":                     true,
	"tinker":                        true,
	"tolarian academy":              true,
	"trade secrets":                 true,
	"upheaval":                      true,
	"worldfire":                     true,
	"yawgmoth's bargain":           true,
}

// Basic lands and cards exempt from the singleton rule.
var singletonExemptBasics = map[string]bool{
	"plains":   true,
	"island":   true,
	"swamp":    true,
	"mountain": true,
	"forest":   true,
	"wastes":   true,
}

// ---------------------------------------------------------------------------
// CheckLegality runs all five Commander legality checks.
// ---------------------------------------------------------------------------

func CheckLegality(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *LegalityReport {
	lr := &LegalityReport{Valid: true}

	commander := report.Commander

	// Determine if we have partner commanders.
	// Partners are detected by looking for "partner" in the commander's oracle text,
	// or by checking if the decklist has a PARTNER: line (tracked via the second
	// commander name in the qtyProfiles). For simplicity, we detect partners from
	// the commander's oracle text.
	hasPartner := false
	if commander != "" {
		cmdrEntry := oracle.lookup(commander)
		if cmdrEntry != nil {
			ot := strings.ToLower(cmdrEntry.OracleText)
			if len(cmdrEntry.CardFaces) > 0 && ot == "" {
				ot = strings.ToLower(cmdrEntry.CardFaces[0].OracleText)
			}
			if strings.Contains(ot, "partner") {
				hasPartner = true
			}
		}
	}

	// 1. Card count check.
	lr.CardCount = checkCardCount(report.TotalCards, hasPartner)
	if !lr.CardCount.Valid {
		lr.Valid = false
		lr.Errors = append(lr.Errors, lr.CardCount.Message)
	}

	// 2. Color identity check.
	lr.ColorID = checkColorIdentity(commander, qtyProfiles, oracle)
	if !lr.ColorID.Valid {
		lr.Valid = false
		for _, v := range lr.ColorID.Violations {
			lr.Errors = append(lr.Errors,
				fmt.Sprintf("color identity violation: %s has identity [%s], commander allows [%s]",
					v.CardName, strings.Join(v.CardColors, ""), strings.Join(v.AllowedColors, "")))
		}
	}

	// 3. Singleton check.
	lr.Singleton = checkSingleton(qtyProfiles, oracle)
	if !lr.Singleton.Valid {
		lr.Valid = false
		for _, v := range lr.Singleton.Violations {
			lr.Errors = append(lr.Errors,
				fmt.Sprintf("singleton violation: %s appears %d times", v.CardName, v.Count))
		}
	}

	// 4. Banned list check.
	lr.BannedCards = checkBanned(qtyProfiles, oracle)
	if !lr.BannedCards.Valid {
		lr.Valid = false
		for _, name := range lr.BannedCards.BannedFound {
			lr.Errors = append(lr.Errors, fmt.Sprintf("banned card: %s", name))
		}
	}

	// 5. Commander legality check.
	lr.CommanderOK = checkCommander(commander, oracle)
	if !lr.CommanderOK.Valid {
		lr.Valid = false
		lr.Errors = append(lr.Errors, lr.CommanderOK.Message)
	}

	// Warnings for edge cases.
	if commander == "" {
		lr.Warnings = append(lr.Warnings, "no commander specified -- some checks skipped")
	}
	if len(qtyProfiles) == 0 {
		lr.Warnings = append(lr.Warnings, "no quantity data available -- singleton and color identity checks may be incomplete")
	}

	return lr
}

// ---------------------------------------------------------------------------
// Individual checks
// ---------------------------------------------------------------------------

func checkCardCount(total int, hasPartner bool) CardCountCheck {
	expected := 100
	cc := CardCountCheck{
		Expected:   expected,
		Actual:     total,
		HasPartner: hasPartner,
	}
	if total == expected {
		cc.Valid = true
		if hasPartner {
			cc.Message = fmt.Sprintf("100 cards (98 + 2 partner commanders)")
		} else {
			cc.Message = fmt.Sprintf("100 cards (99 + 1 commander)")
		}
	} else {
		cc.Valid = false
		cc.Message = fmt.Sprintf("expected 100 cards, found %d", total)
	}
	return cc
}

func checkColorIdentity(commander string, qtyProfiles []CardProfileQty, oracle *oracleDB) ColorIdentityCheck {
	ci := ColorIdentityCheck{Valid: true}

	if commander == "" {
		return ci
	}

	cmdrEntry := oracle.lookup(commander)
	if cmdrEntry == nil {
		ci.CommanderColors = []string{}
		return ci
	}

	// Build the allowed color set from the commander's color identity.
	allowed := map[string]bool{}
	for _, c := range cmdrEntry.ColorIdentity {
		allowed[c] = true
	}
	ci.CommanderColors = cmdrEntry.ColorIdentity

	// Colorless commanders allow only colorless cards.
	for _, qp := range qtyProfiles {
		cardName := qp.Profile.Name
		// Skip the commander itself.
		if normalizeName(cardName) == normalizeName(commander) {
			continue
		}

		entry := oracle.lookup(cardName)
		if entry == nil {
			continue
		}

		// Check each color in the card's color identity.
		for _, c := range entry.ColorIdentity {
			if !allowed[c] {
				ci.Valid = false
				ci.Violations = append(ci.Violations, ColorViolation{
					CardName:      entry.Name,
					CardColors:    entry.ColorIdentity,
					AllowedColors: cmdrEntry.ColorIdentity,
				})
				break // one violation per card is enough
			}
		}
	}

	return ci
}

func checkSingleton(qtyProfiles []CardProfileQty, oracle *oracleDB) SingletonCheck {
	sc := SingletonCheck{Valid: true}

	for _, qp := range qtyProfiles {
		if qp.Qty <= 1 {
			continue
		}

		name := qp.Profile.Name
		lower := strings.ToLower(strings.TrimSpace(name))

		// Basic lands are exempt.
		if singletonExemptBasics[lower] {
			continue
		}

		// Snow-covered basics are also exempt.
		if strings.HasPrefix(lower, "snow-covered ") {
			base := strings.TrimPrefix(lower, "snow-covered ")
			if singletonExemptBasics[base] {
				continue
			}
		}

		// Cards with "a deck can have any number of" in oracle text are exempt.
		entry := oracle.lookup(name)
		if entry != nil {
			ot := strings.ToLower(entry.OracleText)
			if len(entry.CardFaces) > 0 && ot == "" {
				ot = strings.ToLower(entry.CardFaces[0].OracleText)
			}
			if strings.Contains(ot, "a deck can have any number of") {
				continue
			}
		}

		sc.Valid = false
		sc.Violations = append(sc.Violations, SingletonViolation{
			CardName: name,
			Count:    qp.Qty,
		})
	}

	return sc
}

func checkBanned(qtyProfiles []CardProfileQty, oracle *oracleDB) BannedCheck {
	bc := BannedCheck{Valid: true}

	seen := map[string]bool{}

	for _, qp := range qtyProfiles {
		name := qp.Profile.Name

		// Try to resolve the canonical name from oracle.
		entry := oracle.lookup(name)
		var checkName string
		if entry != nil {
			checkName = strings.ToLower(strings.TrimSpace(entry.Name))
		} else {
			checkName = strings.ToLower(strings.TrimSpace(name))
		}

		// Normalize curly quotes for matching.
		checkName = foldQuotes(checkName)

		if commanderBannedList[checkName] && !seen[checkName] {
			seen[checkName] = true
			bc.Valid = false
			displayName := name
			if entry != nil {
				displayName = entry.Name
			}
			bc.BannedFound = append(bc.BannedFound, displayName)
		}
	}

	return bc
}

func checkCommander(commander string, oracle *oracleDB) CommanderCheck {
	cc := CommanderCheck{Valid: true}

	if commander == "" {
		cc.Valid = false
		cc.Message = "no commander specified"
		return cc
	}

	entry := oracle.lookup(commander)
	if entry == nil {
		cc.Valid = false
		cc.Message = fmt.Sprintf("commander %q not found in oracle database", commander)
		return cc
	}

	tl := strings.ToLower(entry.TypeLine)
	ot := strings.ToLower(entry.OracleText)
	if len(entry.CardFaces) > 0 {
		if ot == "" {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		if tl == "" {
			tl = strings.ToLower(entry.CardFaces[0].TypeLine)
		}
	}

	// Legendary creature is the standard commander requirement.
	if strings.Contains(tl, "legendary") && strings.Contains(tl, "creature") {
		cc.Message = fmt.Sprintf("%s is a legendary creature", entry.Name)
		return cc
	}

	// Some cards explicitly say "can be your commander".
	if strings.Contains(ot, "can be your commander") {
		cc.Message = fmt.Sprintf("%s has 'can be your commander' text", entry.Name)
		return cc
	}

	// Planeswalker commanders need the specific text.
	if strings.Contains(tl, "planeswalker") {
		if strings.Contains(ot, "can be your commander") {
			cc.Message = fmt.Sprintf("%s is a planeswalker that can be your commander", entry.Name)
			return cc
		}
		cc.Valid = false
		cc.Message = fmt.Sprintf("%s is a planeswalker without 'can be your commander' text", entry.Name)
		return cc
	}

	cc.Valid = false
	cc.Message = fmt.Sprintf("%s is not a legendary creature and does not have 'can be your commander' text", entry.Name)
	return cc
}
