package judge

import (
	"fmt"
	"strings"
)

// legality.go — the LEGALITY dimension's deck-construction checks
// (Hex Judge phase 3 fold; pattern per 44e84b1e).
//
// Two halves make up the LEGALITY dimension:
//
//  1. DECK CONSTRUCTION (this file): the five Commander deck checks —
//     card count (CR §903.5a), color identity (§903.5c), singleton
//     (§903.5b), the format banned list, and commander eligibility
//     (§903.3). Promoted verbatim from cmd/hexdek-freya/legality.go
//     (now deleted); freya is a driver that builds a DeckSubmission and
//     renders the report. Checks assert from first principles on a
//     neutral input shape — no oracle-DB or engine dependency; callers
//     resolve card data however they like.
//
//  2. ACTION LEGALITY (internal/gameengine/legality.go): the ride-along
//     validator's target/timing/cost/priority checks at announcement.
//     Its LegalityViolation.Canonical() stamps Dimension=legality and
//     routes through LogViolation at record time.
//
// Every failed deck check emits one canonical ValidationViolation
// through LogViolation (Surface=legality, Dimension=legality, Name=CR
// citation) in addition to populating the structured report — the
// report is the driver-facing rendering, the violation stream is the
// Judge-facing record.

// ---------------------------------------------------------------------------
// Input shape
// ---------------------------------------------------------------------------

// DeckCard is one resolved deck entry. Resolved=false marks cards the
// caller could not find in its card database — checks that need card
// data (color identity, oracle-text exemptions, banned-list canonical
// name) skip or fall back to the raw name for unresolved entries,
// mirroring the historical freya behavior.
type DeckCard struct {
	// Name as written in the deck list.
	Name string
	// CanonicalName from the caller's card database ("" if unresolved).
	CanonicalName string
	// Qty is the number of copies in the list.
	Qty int
	// ColorIdentity letters (W/U/B/R/G) per the card database.
	ColorIdentity []string
	// OracleText (front face when the card is multi-faced).
	OracleText string
	// TypeLine (front face when multi-faced).
	TypeLine string
	// Resolved is true when the caller found the card in its database.
	Resolved bool
}

// DeckSubmission is the Judge's neutral deck-legality input.
type DeckSubmission struct {
	// Commander entry (zero-value + Name=="" when none was specified).
	Commander DeckCard
	// Cards is the full list EXCLUDING the commander.
	Cards []DeckCard
	// TotalCards is the list total INCLUDING the commander(s).
	TotalCards int
}

// ---------------------------------------------------------------------------
// Report shape (field names + json tags preserved from the freya
// original so its JSON wire format and renderers are unchanged).
// ---------------------------------------------------------------------------

// DeckLegalityReport is the result of all five legality checks.
type DeckLegalityReport struct {
	Valid       bool               `json:"valid"`
	CardCount   CardCountCheck     `json:"card_count"`
	ColorID     ColorIdentityCheck `json:"color_identity"`
	Singleton   SingletonCheck     `json:"singleton"`
	BannedCards BannedCheck        `json:"banned_cards"`
	CommanderOK CommanderCheck     `json:"commander"`
	Warnings    []string           `json:"warnings,omitempty"`
	Errors      []string           `json:"errors,omitempty"`
}

// CardCountCheck validates the deck has exactly 100 cards (99+1, or
// 98+2 with partner commanders).
type CardCountCheck struct {
	Valid      bool   `json:"valid"`
	Expected   int    `json:"expected"`
	Actual     int    `json:"actual"`
	HasPartner bool   `json:"has_partner"`
	Message    string `json:"message,omitempty"`
}

// ColorIdentityCheck validates every card is within the commander's
// color identity (CR §903.5c).
type ColorIdentityCheck struct {
	Valid           bool             `json:"valid"`
	CommanderColors []string         `json:"commander_colors"`
	Violations      []ColorViolation `json:"violations,omitempty"`
}

// ColorViolation is a single card whose identity exceeds the commander's.
type ColorViolation struct {
	CardName      string   `json:"card_name"`
	CardColors    []string `json:"card_colors"`
	AllowedColors []string `json:"allowed_colors"`
}

// SingletonCheck validates no card appears more than once (CR §903.5b,
// with basic-land and "any number of" exemptions).
type SingletonCheck struct {
	Valid      bool                 `json:"valid"`
	Violations []SingletonViolation `json:"violations,omitempty"`
}

// SingletonViolation is a card that appears more than once illegally.
type SingletonViolation struct {
	CardName string `json:"card_name"`
	Count    int    `json:"count"`
}

// BannedCheck flags cards on the Commander banned list.
type BannedCheck struct {
	Valid       bool     `json:"valid"`
	BannedFound []string `json:"banned_found,omitempty"`
}

// CommanderCheck validates the commander is a legal commander (§903.3).
type CommanderCheck struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Commander banned list (as of April 2026) + singleton exemptions.
// ---------------------------------------------------------------------------

var commanderBannedList = map[string]bool{
	"ancestral recall":            true,
	"balance":                     true,
	"biorhythm":                   true,
	"black lotus":                 true,
	"braids, cabal minion":        true,
	"channel":                     true,
	"coalition victory":           true,
	"dockside extortionist":       true,
	"emrakul, the aeons torn":     true,
	"erayo, soratami ascendant":   true,
	"falling star":                true,
	"fastbond":                    true,
	"flash":                       true,
	"golos, tireless pilgrim":     true,
	"griselbrand":                 true,
	"hullbreacher":                true,
	"iona, shield of emeria":      true,
	"jeweled lotus":               true,
	"karakas":                     true,
	"leovold, emissary of trest":  true,
	"library of alexandria":       true,
	"limited resources":           true,
	"lutri, the spellchaser":      true,
	"mana crypt":                  true,
	"mox emerald":                 true,
	"mox jet":                     true,
	"mox pearl":                   true,
	"mox ruby":                    true,
	"mox sapphire":                true,
	"nadu, winged wisdom":         true,
	"panoptic mirror":             true,
	"paradox engine":              true,
	"primeval titan":              true,
	"prophet of kruphix":          true,
	"recurring nightmare":         true,
	"rofellos, llanowar emissary": true,
	"shahrazad":                   true,
	"sundering titan":             true,
	"sway of the stars":           true,
	"sylvan primordial":           true,
	"time vault":                  true,
	"time walk":                   true,
	"tinker":                      true,
	"tolarian academy":            true,
	"trade secrets":               true,
	"upheaval":                    true,
	"worldfire":                   true,
	"yawgmoth's bargain":          true,
}

// IsCommanderBanned reports whether a (canonical, lowercased-by-callee)
// card name is on the Commander banned list. Exported so drivers (loki
// pool hygiene, forge) can consult the ONE list. A card carried by the
// stale-ban override (IsUnbannedOverride) reads NOT banned even if it is
// still on commanderBannedList.
func IsCommanderBanned(name string) bool {
	n := legalityFoldQuotes(strings.ToLower(strings.TrimSpace(name)))
	if unbannedOverrides["commander"][n] {
		return false
	}
	return commanderBannedList[n]
}

// ---------------------------------------------------------------------------
// Stale-ban overrides — THE single canonical correction layer.
// ---------------------------------------------------------------------------

// unbannedOverrides lists cards whose cached Scryfall legality (or a stale
// hardcoded banlist entry) may still read "banned" but which have since
// been unbanned in that format. Keyed by format slug → lowercased,
// quote-folded card name. This is the ONE source consulted by every
// banned-determination site — judge deck-legality (checkDeckBanned /
// IsCommanderBanned), the moxfield import validator, and the
// commander-check CLI — so correcting a stale ban is a single edit that
// reads LEGAL everywhere.
//
// Temporary band-aid pending a full oracle legality-cache refresh (the
// canonical long-term fix — regenerate oracle-cards.json + the card_oracle
// legalities cache from current Scryfall data). Gifts Ungiven was unbanned
// in Commander (Sept 2024); #996 added an import-path override and #997
// pulled it from both hardcoded lists, but the override is the durable
// mechanism for the broader stale-ban wave (#997 noted ~10 more). Delete
// per-format entries once the underlying legality data is re-fetched.
var unbannedOverrides = map[string]map[string]bool{
	"commander": {
		// Unbanned 2024-09 (Game Changer). #996/#997/92daf2b0.
		"gifts ungiven": true,
		// Unbanned 2025-04-22 (all five moved to Game Changers).
		"sway of the stars":    true,
		"braids, cabal minion": true,
		"coalition victory":    true,
		"panoptic mirror":      true,
		// Unbanned 2026-02-09 (both Game Changers). NOTE: Lutri is
		// unbanned for deck inclusion / as a commander but REMAINS
		// banned as a Companion — a restriction the deck-construction
		// banlist does not model, so it reads legal here (correct for
		// "may this card be in the deck").
		"biorhythm":              true,
		"lutri, the spellchaser": true,
		// Worldfire was unbanned ahead of the Format Panel era and is
		// confirmed legal in the 2026-04-30 oracle snapshot.
		"worldfire": true,
	},
}

// IsUnbannedOverride reports whether (format, name) is a known stale-ban
// override — the card reads legal in that format despite any cached
// "banned" Scryfall status or stale hardcoded banlist entry. Name is
// normalized (trim, lowercase, curly-quote/dash fold) to match banlist
// canonicalization; format is lowercased.
func IsUnbannedOverride(format, name string) bool {
	m := unbannedOverrides[strings.ToLower(strings.TrimSpace(format))]
	if m == nil {
		return false
	}
	return m[legalityFoldQuotes(strings.ToLower(strings.TrimSpace(name)))]
}

var singletonExemptBasics = map[string]bool{
	"plains":   true,
	"island":   true,
	"swamp":    true,
	"mountain": true,
	"forest":   true,
	"wastes":   true,
}

// legalityFoldQuotes normalizes curly quotes/dashes for banned-list
// matching (deck lists arrive with typographic punctuation).
func legalityFoldQuotes(s string) string {
	r := strings.NewReplacer(
		"‘", "'", "’", "'", "′", "'",
		"“", "\"", "”", "\"",
		"–", "-", "—", "-",
	)
	return r.Replace(s)
}

// ---------------------------------------------------------------------------
// CheckDeckLegality — the LEGALITY dimension's deck-construction checks.
// ---------------------------------------------------------------------------

// CheckDeckLegality runs all five Commander deck checks and returns the
// structured report. Every failed check ALSO emits one canonical
// ValidationViolation through LogViolation (Surface=legality,
// Dimension=legality, Name=CR citation).
func CheckDeckLegality(sub DeckSubmission) *DeckLegalityReport {
	lr := &DeckLegalityReport{Valid: true}

	hasPartner := sub.Commander.Resolved &&
		strings.Contains(strings.ToLower(sub.Commander.OracleText), "partner")

	// 1. Card count (CR §903.5a).
	lr.CardCount = checkDeckCardCount(sub.TotalCards, hasPartner)
	if !lr.CardCount.Valid {
		lr.Valid = false
		lr.Errors = append(lr.Errors, lr.CardCount.Message)
		emitDeckViolation("903.5a", lr.CardCount.Message, map[string]interface{}{
			"expected": lr.CardCount.Expected,
			"actual":   lr.CardCount.Actual,
		})
	}

	// 2. Color identity (CR §903.5c).
	lr.ColorID = checkDeckColorIdentity(sub)
	if !lr.ColorID.Valid {
		lr.Valid = false
		for _, v := range lr.ColorID.Violations {
			msg := fmt.Sprintf("color identity violation: %s has identity [%s], commander allows [%s]",
				v.CardName, strings.Join(v.CardColors, ""), strings.Join(v.AllowedColors, ""))
			lr.Errors = append(lr.Errors, msg)
			emitDeckViolation("903.5c", msg, map[string]interface{}{
				"card": v.CardName,
			})
		}
	}

	// 3. Singleton (CR §903.5b).
	lr.Singleton = checkDeckSingleton(sub.Cards)
	if !lr.Singleton.Valid {
		lr.Valid = false
		for _, v := range lr.Singleton.Violations {
			msg := fmt.Sprintf("singleton violation: %s appears %d times", v.CardName, v.Count)
			lr.Errors = append(lr.Errors, msg)
			emitDeckViolation("903.5b", msg, map[string]interface{}{
				"card":  v.CardName,
				"count": v.Count,
			})
		}
	}

	// 4. Banned list (format rule).
	lr.BannedCards = checkDeckBanned(sub.Cards)
	if !lr.BannedCards.Valid {
		lr.Valid = false
		for _, name := range lr.BannedCards.BannedFound {
			msg := "banned card: " + name
			lr.Errors = append(lr.Errors, msg)
			emitDeckViolation("banlist", msg, map[string]interface{}{
				"card": name,
			})
		}
	}

	// 5. Commander eligibility (CR §903.3).
	lr.CommanderOK = checkDeckCommander(sub.Commander)
	if !lr.CommanderOK.Valid {
		lr.Valid = false
		lr.Errors = append(lr.Errors, lr.CommanderOK.Message)
		emitDeckViolation("903.3", lr.CommanderOK.Message, map[string]interface{}{
			"commander": sub.Commander.Name,
		})
	}

	// Warnings for edge cases (report-only; not violations).
	if sub.Commander.Name == "" {
		lr.Warnings = append(lr.Warnings, "no commander specified -- some checks skipped")
	}
	if len(sub.Cards) == 0 {
		lr.Warnings = append(lr.Warnings, "no quantity data available -- singleton and color identity checks may be incomplete")
	}

	return lr
}

func emitDeckViolation(rule, msg string, ctx map[string]interface{}) {
	LogViolation(ValidationViolation{
		Surface:   SurfaceLegality,
		Dimension: DimensionLegality,
		Name:      rule,
		Severity:  SeverityCritical,
		Message:   msg,
		Seat:      -1, // deck-construction: no seat
		Context:   ctx,
	})
}

// ---------------------------------------------------------------------------
// Individual checks (semantics preserved verbatim from the freya
// originals; only the input shape changed).
// ---------------------------------------------------------------------------

func checkDeckCardCount(total int, hasPartner bool) CardCountCheck {
	cc := CardCountCheck{
		Expected:   100,
		Actual:     total,
		HasPartner: hasPartner,
	}
	if total == cc.Expected {
		cc.Valid = true
		if hasPartner {
			cc.Message = "100 cards (98 + 2 partner commanders)"
		} else {
			cc.Message = "100 cards (99 + 1 commander)"
		}
	} else {
		cc.Valid = false
		cc.Message = fmt.Sprintf("expected 100 cards, found %d", total)
	}
	return cc
}

func checkDeckColorIdentity(sub DeckSubmission) ColorIdentityCheck {
	ci := ColorIdentityCheck{Valid: true}

	if sub.Commander.Name == "" {
		return ci
	}
	if !sub.Commander.Resolved {
		ci.CommanderColors = []string{}
		return ci
	}

	allowed := map[string]bool{}
	for _, c := range sub.Commander.ColorIdentity {
		allowed[c] = true
	}
	ci.CommanderColors = sub.Commander.ColorIdentity

	for _, card := range sub.Cards {
		if !card.Resolved {
			continue
		}
		for _, c := range card.ColorIdentity {
			if !allowed[c] {
				ci.Valid = false
				name := card.CanonicalName
				if name == "" {
					name = card.Name
				}
				ci.Violations = append(ci.Violations, ColorViolation{
					CardName:      name,
					CardColors:    card.ColorIdentity,
					AllowedColors: sub.Commander.ColorIdentity,
				})
				break // one violation per card is enough
			}
		}
	}

	return ci
}

func checkDeckSingleton(cards []DeckCard) SingletonCheck {
	sc := SingletonCheck{Valid: true}

	for _, card := range cards {
		if card.Qty <= 1 {
			continue
		}

		lower := strings.ToLower(strings.TrimSpace(card.Name))

		// Basic lands are exempt.
		if singletonExemptBasics[lower] {
			continue
		}
		// Snow-covered basics too.
		if strings.HasPrefix(lower, "snow-covered ") {
			if singletonExemptBasics[strings.TrimPrefix(lower, "snow-covered ")] {
				continue
			}
		}
		// "A deck can have any number of ..." cards are exempt.
		if card.Resolved &&
			strings.Contains(strings.ToLower(card.OracleText), "a deck can have any number of") {
			continue
		}

		sc.Valid = false
		sc.Violations = append(sc.Violations, SingletonViolation{
			CardName: card.Name,
			Count:    card.Qty,
		})
	}

	return sc
}

func checkDeckBanned(cards []DeckCard) BannedCheck {
	bc := BannedCheck{Valid: true}
	seen := map[string]bool{}

	for _, card := range cards {
		checkName := card.CanonicalName
		if checkName == "" {
			checkName = card.Name
		}
		checkName = legalityFoldQuotes(strings.ToLower(strings.TrimSpace(checkName)))

		// A stale-ban override reads the card legal even if it is still
		// on commanderBannedList (defensive: future re-adds / drift).
		if unbannedOverrides["commander"][checkName] {
			continue
		}
		if commanderBannedList[checkName] && !seen[checkName] {
			seen[checkName] = true
			bc.Valid = false
			displayName := card.CanonicalName
			if displayName == "" {
				displayName = card.Name
			}
			bc.BannedFound = append(bc.BannedFound, displayName)
		}
	}

	return bc
}

func checkDeckCommander(cmdr DeckCard) CommanderCheck {
	cc := CommanderCheck{Valid: true}

	if cmdr.Name == "" {
		cc.Valid = false
		cc.Message = "no commander specified"
		return cc
	}
	if !cmdr.Resolved {
		cc.Valid = false
		cc.Message = fmt.Sprintf("commander %q not found in oracle database", cmdr.Name)
		return cc
	}

	tl := strings.ToLower(cmdr.TypeLine)
	ot := strings.ToLower(cmdr.OracleText)
	display := cmdr.CanonicalName
	if display == "" {
		display = cmdr.Name
	}

	// Legendary creature is the standard commander requirement.
	if strings.Contains(tl, "legendary") && strings.Contains(tl, "creature") {
		cc.Message = fmt.Sprintf("%s is a legendary creature", display)
		return cc
	}
	// Some cards explicitly say "can be your commander".
	if strings.Contains(ot, "can be your commander") {
		cc.Message = fmt.Sprintf("%s has 'can be your commander' text", display)
		return cc
	}
	// Planeswalker commanders need the specific text.
	if strings.Contains(tl, "planeswalker") {
		cc.Valid = false
		cc.Message = fmt.Sprintf("%s is a planeswalker without 'can be your commander' text", display)
		return cc
	}

	cc.Valid = false
	cc.Message = fmt.Sprintf("%s is not a legendary creature and does not have 'can be your commander' text", display)
	return cc
}
