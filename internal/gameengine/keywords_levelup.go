package gameengine

// Level Up keyword — CR §702.87 / §711 (Leveler Cards)
//
// Level up is an activated ability from Rise of the Eldrazi. "Level up [cost]"
// means "[cost]: Put a level counter on this permanent. Activate only as a
// sorcery." (CR §702.87a)
//
// Each leveler card has two level symbols that define bracket ranges:
//   - LEVEL N1-N2: "As long as this creature has at least N1 level counters
//     on it, but no more than N2 level counters on it, it has base power and
//     toughness [P/T] and has [abilities]." (CR §711.2a)
//   - LEVEL N3+:  "As long as this creature has N3 or more level counters
//     on it, it has base power and toughness [P/T] and has [abilities]."
//     (CR §711.2b)
//
// When the level counter count is less than the first bracket's minimum,
// the creature uses its printed (uppermost) P/T box. (CR §711.5)
//
// Level up can be activated regardless of how many level counters are on
// the permanent — there is no cap. (CR §711.4)
//
// Implementation notes:
//   - The parser emits "level up" as a Keyword ability on the AST.
//   - Level brackets appear as Static abilities with modification.kind =
//     "class_level_band" and args = [lo, hi] or [lo, nil] for open-ended.
//   - The P/T and keywords following each band are separate abilities in
//     the AST. We parse them from oracle text for structured bracket data.
//   - Level counters are stored in perm.Counters["level"].

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// LevelBracket — parsed bracket data for a leveler card
// ---------------------------------------------------------------------------

// LevelBracket represents one level range on a leveler card.
type LevelBracket struct {
	MinLevel  int      // minimum level counters for this bracket
	MaxLevel  int      // maximum level counters (-1 means unbounded/"+")
	Power     int      // base P/T for this bracket
	Toughness int      // base P/T for this bracket
	Keywords  []string // keyword abilities granted at this bracket
}

// ---------------------------------------------------------------------------
// Level Up cost extraction
// ---------------------------------------------------------------------------

// LevelUpCost extracts the level up cost from a card's AST keywords.
// Returns (cost, true) if the card has "level up". The cost is parsed
// from the keyword's raw text (e.g. "level up {r}" -> 1, "level up {4}" -> 4).
// If the keyword has args, uses args[0] as the cost.
func LevelUpCost(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		if name != "level up" {
			continue
		}
		// Try args first.
		if len(kw.Args) > 0 {
			switch v := kw.Args[0].(type) {
			case float64:
				return int(v), true
			case int:
				return v, true
			}
		}
		// Parse from raw text: "level up {r}" or "level up {4}" or "level up {r}{r}".
		cost := parseLevelUpCostFromRaw(kw.Raw)
		return cost, true
	}
	return 0, false
}

// HasLevelUp returns true if the card has the "level up" keyword.
func HasLevelUp(card *Card) bool {
	_, has := LevelUpCost(card)
	return has
}

// levelUpCostRegex matches mana symbols in braces, e.g. {R}, {4}, {1}{G}.
var levelUpCostRegex = regexp.MustCompile(`\{([^}]+)\}`)

// parseLevelUpCostFromRaw extracts the mana cost from "level up {cost}" raw text.
// Returns the total CMC of the cost. Each colored symbol counts as 1; generic
// numbers are added directly.
func parseLevelUpCostFromRaw(raw string) int {
	// Find everything after "level up".
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "level up")
	if idx < 0 {
		return 0
	}
	costPart := raw[idx+len("level up"):]
	matches := levelUpCostRegex.FindAllStringSubmatch(costPart, -1)
	total := 0
	for _, m := range matches {
		sym := strings.ToLower(m[1])
		// Generic mana: a number.
		if n, err := strconv.Atoi(sym); err == nil {
			total += n
		} else {
			// Colored/hybrid symbol: counts as 1.
			total++
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// Level bracket parsing from oracle text
// ---------------------------------------------------------------------------

// levelBandRegex matches "LEVEL N1-N2" or "LEVEL N3+" headers in oracle text.
var levelBandRegex = regexp.MustCompile(`(?i)^LEVEL\s+(\d+)([+-])(\d*)$`)

// ptRegex matches a bare "P/T" line like "4/4" or "8/8".
var ptRegex = regexp.MustCompile(`^(\d+)/(\d+)$`)

// ParseLevelBrackets extracts all level brackets from a card's oracle text.
// Returns brackets sorted by MinLevel. The base (level 0) bracket is NOT
// returned — the card's printed BasePower/BaseToughness applies for that.
func ParseLevelBrackets(card *Card) []LevelBracket {
	if card == nil || card.AST == nil {
		return nil
	}

	// Parse from oracle text if available on the card itself.
	oracle := ""
	if card.AST != nil {
		// Try to get oracle text. We look for it in the raw text of the
		// keyword abilities. As a fallback, reconstruct from AST abilities.
		oracle = reconstructOracleForLevelUp(card)
	}

	if oracle == "" {
		// Fall back to AST-based parsing.
		return parseLevelBracketsFromAST(card)
	}

	return parseLevelBracketsFromOracle(oracle)
}

// reconstructOracleForLevelUp reconstructs oracle text from AST raw texts.
func reconstructOracleForLevelUp(card *Card) string {
	if card.AST == nil {
		return ""
	}
	var parts []string
	for _, ab := range card.AST.Abilities {
		switch v := ab.(type) {
		case *gameast.Static:
			if v.Raw != "" {
				parts = append(parts, v.Raw)
			}
		case *gameast.Keyword:
			if v.Raw != "" {
				parts = append(parts, v.Raw)
			}
		case *gameast.Activated:
			if v.Raw != "" {
				parts = append(parts, v.Raw)
			}
		case *gameast.Triggered:
			if v.Raw != "" {
				parts = append(parts, v.Raw)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// parseLevelBracketsFromOracle parses level brackets from oracle text lines.
func parseLevelBracketsFromOracle(oracle string) []LevelBracket {
	lines := strings.Split(oracle, "\n")
	var brackets []LevelBracket
	var current *LevelBracket

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for LEVEL header.
		m := levelBandRegex.FindStringSubmatch(line)
		if m != nil {
			// Save previous bracket if any.
			if current != nil {
				brackets = append(brackets, *current)
			}
			lo, _ := strconv.Atoi(m[1])
			current = &LevelBracket{MinLevel: lo, MaxLevel: -1}
			if m[2] == "-" && m[3] != "" {
				hi, _ := strconv.Atoi(m[3])
				current.MaxLevel = hi
			}
			// m[2] == "+" means open-ended, MaxLevel stays -1.
			continue
		}

		if current == nil {
			continue
		}

		// Check for P/T line.
		pm := ptRegex.FindStringSubmatch(line)
		if pm != nil {
			p, _ := strconv.Atoi(pm[1])
			t, _ := strconv.Atoi(pm[2])
			current.Power = p
			current.Toughness = t
			continue
		}

		// Everything else is keywords/abilities for this bracket.
		// Split on commas for multiple keywords.
		kwParts := strings.Split(line, ",")
		for _, kw := range kwParts {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				// Skip the level up ability itself.
				if strings.HasPrefix(strings.ToLower(kw), "level up") {
					continue
				}
				current.Keywords = append(current.Keywords, strings.ToLower(kw))
			}
		}
	}

	if current != nil {
		brackets = append(brackets, *current)
	}

	return brackets
}

// parseLevelBracketsFromAST extracts brackets from the AST's class_level_band
// Modification nodes and the abilities that follow them.
func parseLevelBracketsFromAST(card *Card) []LevelBracket {
	if card.AST == nil {
		return nil
	}
	var brackets []LevelBracket
	var current *LevelBracket

	for _, ab := range card.AST.Abilities {
		switch v := ab.(type) {
		case *gameast.Static:
			if v.Modification != nil && v.Modification.ModKind == "class_level_band" {
				// Save previous bracket.
				if current != nil {
					brackets = append(brackets, *current)
				}
				lo, hi := parseBandArgs(v.Modification.Args)
				current = &LevelBracket{MinLevel: lo, MaxLevel: hi}
				continue
			}
			// P/T line (parsed_effect_residual containing "bare P/T").
			if current != nil && v.Modification != nil {
				if v.Modification.ModKind == "spell_effect" && len(v.Modification.Args) > 0 {
					// Check for nested modification with bare P/T.
					for _, arg := range v.Modification.Args {
						if mod, ok := arg.(*gameast.Modification); ok {
							if mod.ModKind == "parsed_effect_residual" && len(mod.Args) > 0 {
								if s, ok := mod.Args[0].(string); ok && strings.HasPrefix(s, "bare P/T") {
									parseBaresPT(s, current)
								}
							}
						}
					}
				}
			}
		case *gameast.Keyword:
			// Keywords following a level band belong to that bracket.
			name := strings.ToLower(strings.TrimSpace(v.Name))
			if current != nil && name != "level up" {
				current.Keywords = append(current.Keywords, name)
			}
		}
	}

	if current != nil {
		brackets = append(brackets, *current)
	}

	return brackets
}

// parseBandArgs extracts (lo, hi) from class_level_band args.
// Args are [lo, hi] where hi can be nil (open-ended).
// Returns hi = -1 for open-ended brackets.
func parseBandArgs(args []interface{}) (int, int) {
	if len(args) < 1 {
		return 0, -1
	}
	lo := toInt(args[0])
	if len(args) < 2 || args[1] == nil {
		return lo, -1 // open-ended
	}
	hi := toInt(args[1])
	return lo, hi
}

// toInt converts a float64 or int to int.
func toInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}

// parseBaresPT parses "bare P/T N/N" into the current bracket.
func parseBaresPT(s string, bracket *LevelBracket) {
	// Format: "bare P/T 4/4"
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return
	}
	pt := parts[len(parts)-1] // "4/4"
	m := ptRegex.FindStringSubmatch(pt)
	if m != nil {
		p, _ := strconv.Atoi(m[1])
		t, _ := strconv.Atoi(m[2])
		bracket.Power = p
		bracket.Toughness = t
	}
}

// ---------------------------------------------------------------------------
// GetLevelBracket — find the matching bracket for a permanent
// ---------------------------------------------------------------------------

// GetLevelBracket returns the bracket matching the permanent's current level
// counter count. Returns nil if the permanent has no level brackets or if the
// level count is below the first bracket (use base P/T in that case).
func GetLevelBracket(perm *Permanent) *LevelBracket {
	if perm == nil || perm.Card == nil {
		return nil
	}
	brackets := ParseLevelBrackets(perm.Card)
	if len(brackets) == 0 {
		return nil
	}

	levelCount := 0
	if perm.Counters != nil {
		levelCount = perm.Counters["level"]
	}

	// Find the matching bracket (later brackets take priority if overlapping).
	var match *LevelBracket
	for i := range brackets {
		b := &brackets[i]
		if levelCount >= b.MinLevel {
			if b.MaxLevel == -1 || levelCount <= b.MaxLevel {
				match = b
			}
		}
	}
	return match
}

// GetLevelCount returns the number of level counters on a permanent.
func GetLevelCount(perm *Permanent) int {
	if perm == nil || perm.Counters == nil {
		return 0
	}
	return perm.Counters["level"]
}

// ---------------------------------------------------------------------------
// ActivateLevelUp — the activated ability handler
// ---------------------------------------------------------------------------

// ActivateLevelUp activates the level up ability on a creature permanent.
// CR §702.87a: "[Cost]: Put a level counter on this permanent. Activate
// only as a sorcery."
//
// Steps:
//  1. Verify the permanent has "level up" keyword
//  2. Check sorcery-speed restriction (controller's main phase, stack empty)
//  3. Pay the level-up cost
//  4. Add a "level" counter to the permanent
//  5. Invalidate characteristics cache (P/T may change)
//  6. Run SBAs (in case P/T change causes lethal)
//
// Returns error if activation is illegal.
func ActivateLevelUp(gs *GameState, seatIdx int, perm *Permanent) error {
	if gs == nil || perm == nil {
		return &CastError{Reason: "nil game or permanent"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if perm.Controller != seatIdx {
		return &CastError{Reason: "not controller"}
	}
	if perm.Card == nil {
		return &CastError{Reason: "nil card"}
	}

	// Verify the card has level up.
	cost, hasLevelUp := LevelUpCost(perm.Card)
	if !hasLevelUp {
		return &CastError{Reason: "no level up ability"}
	}

	// Sorcery-speed check: controller's main phase, stack empty.
	// CR §702.87a: "Activate only as a sorcery."
	if !isSorceryTiming(gs, seatIdx) {
		return &CastError{Reason: "sorcery_speed_only"}
	}

	// Pay the cost.
	seat := gs.Seats[seatIdx]
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Add a level counter.
	perm.AddCounter("level", 1)
	newLevel := perm.Counters["level"]

	// Invalidate characteristics cache — P/T and abilities may change.
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "level_up",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: newLevel,
		Details: map[string]interface{}{
			"cost": cost,
			"rule": "702.87a",
		},
	})

	// Run SBAs in case the P/T change causes lethal damage.
	StateBasedActions(gs)

	return nil
}

// isSorceryTiming returns true if the given seat can take sorcery-speed
// actions: it's their turn, during a main phase, and the stack is empty.
func isSorceryTiming(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	// Must be active player's turn.
	if gs.Active != seatIdx {
		return false
	}
	// Must be during a main phase.
	step := strings.ToLower(gs.Step)
	if step != "precombat_main" && step != "postcombat_main" {
		return false
	}
	// Stack must be empty.
	if len(gs.Stack) > 0 {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// ApplyLevelBracketEffects — continuous effect for layer resolution
// ---------------------------------------------------------------------------

// ApplyLevelBracketEffects applies the current level bracket's P/T and
// abilities to a leveler creature. This should be called:
//   - After level counter changes (from ActivateLevelUp)
//   - During layer resolution for ongoing characteristic computation
//
// For level up creatures, the bracket P/T is a base P/T override (layer 7b),
// not a modifier. The bracket replaces the printed P/T entirely.
// CR §711.2a/b: "it has base power and toughness [P/T]"
//
// Bracket keywords are granted as additional abilities (layer 6).
func ApplyLevelBracketEffects(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !HasLevelUp(perm.Card) {
		return
	}

	bracket := GetLevelBracket(perm)
	if bracket == nil {
		// Below first bracket — use printed base P/T (no modification needed).
		return
	}

	// Layer 7b: Set base P/T to bracket values.
	// We implement this as a Modification that overrides rather than adds.
	// The modification sets P/T to (bracket - base), so base + mod = bracket.
	deltaP := bracket.Power - perm.Card.BasePower
	deltaT := bracket.Toughness - perm.Card.BaseToughness

	// Check if we already have a level_bracket modification to avoid stacking.
	// Remove any existing level bracket modifications.
	filtered := perm.Modifications[:0]
	for _, m := range perm.Modifications {
		if m.Duration != "level_bracket" {
			filtered = append(filtered, m)
		}
	}
	perm.Modifications = filtered

	perm.Modifications = append(perm.Modifications, Modification{
		Power:     deltaP,
		Toughness: deltaT,
		Duration:  "level_bracket",
		Timestamp: perm.Timestamp,
	})

	// Layer 6: Grant bracket abilities.
	// Remove previous level-bracket-granted abilities.
	kept := perm.GrantedAbilities[:0]
	for _, g := range perm.GrantedAbilities {
		if !strings.HasPrefix(g, "level_bracket:") {
			kept = append(kept, g)
		}
	}
	perm.GrantedAbilities = kept

	// Add bracket keywords as granted abilities.
	for _, kw := range bracket.Keywords {
		perm.GrantedAbilities = append(perm.GrantedAbilities, "level_bracket:"+kw)
		// Also add the plain keyword name so HasKeyword checks work.
		perm.GrantedAbilities = append(perm.GrantedAbilities, kw)
	}

	gs.InvalidateCharacteristicsCache()
}

// ---------------------------------------------------------------------------
// RegisterLevelUpContinuousEffects — wire into the layer system
// ---------------------------------------------------------------------------

// RegisterLevelUpContinuousEffect registers a continuous effect for a leveler
// creature so the layer system picks up bracket P/T and abilities. Call this
// at ETB for any creature with the "level up" keyword.
func RegisterLevelUpContinuousEffect(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !HasLevelUp(perm.Card) {
		return
	}

	cardName := perm.Card.DisplayName()

	// Register a ContinuousEffect that applies level bracket data.
	// Layer 7, sublayer "b" — P/T setting ("becomes N/N").
	ce := &ContinuousEffect{
		Layer:          7,
		Sublayer:       "b",
		Timestamp:      perm.Timestamp,
		SourcePerm:     perm,
		SourceCardName: cardName,
		ControllerSeat: perm.Controller,
		HandlerID:      "level_up:" + cardName,
		Duration:       "permanent",
		Predicate: func(_ *GameState, target *Permanent) bool {
			return target == perm
		},
		ApplyFn: func(gs *GameState, target *Permanent, chars *Characteristics) {
			// Apply bracket P/T as base override.
			bracket := GetLevelBracket(target)
			if bracket == nil {
				return
			}
			chars.Power = bracket.Power
			chars.Toughness = bracket.Toughness
			// Grant bracket keywords.
			for _, kw := range bracket.Keywords {
				chars.Keywords = append(chars.Keywords, kw)
			}
		},
	}
	gs.ContinuousEffects = append(gs.ContinuousEffects, ce)
}

// ---------------------------------------------------------------------------
// IsLeveler — type check for leveler cards
// ---------------------------------------------------------------------------

// IsLeveler returns true if the permanent's card is a leveler (has level up).
func IsLeveler(perm *Permanent) bool {
	if perm == nil || perm.Card == nil {
		return false
	}
	return HasLevelUp(perm.Card)
}

// ---------------------------------------------------------------------------
// GetEffectiveLevelPT — convenience for getting bracket-adjusted P/T
// ---------------------------------------------------------------------------

// GetEffectiveLevelPT returns the base P/T for a leveler creature considering
// its current level bracket. If the creature is not a leveler or is below the
// first bracket, returns the card's base P/T. This does NOT include counters
// or other modifications — just the bracket override.
func GetEffectiveLevelPT(perm *Permanent) (power, toughness int) {
	if perm == nil || perm.Card == nil {
		return 0, 0
	}

	bracket := GetLevelBracket(perm)
	if bracket == nil {
		return perm.Card.BasePower, perm.Card.BaseToughness
	}
	return bracket.Power, bracket.Toughness
}
