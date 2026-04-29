package gameengine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// wordToInt converts English number words to integers.
// Returns (n, true) on success, (0, false) if not a number word.
func wordToInt(s string) (int, bool) {
	switch strings.ToLower(s) {
	case "a", "an", "one":
		return 1, true
	case "two":
		return 2, true
	case "three":
		return 3, true
	case "four":
		return 4, true
	case "five":
		return 5, true
	case "six":
		return 6, true
	case "seven":
		return 7, true
	case "eight":
		return 8, true
	case "nine":
		return 9, true
	case "ten":
		return 10, true
	}
	return 0, false
}

// resolveConditionalEffect attempts to parse and execute a conditional_effect
// Modification at runtime. The parser stored "if CONDITION, EFFECT" as raw
// text; this function decomposes it into a structured Condition + Effect and
// resolves it through the normal conditional path.
//
// Returns true if the conditional was handled, false if it fell through
// (caller should log as unhandled).
func resolveConditionalEffect(gs *GameState, src *Permanent, raw string) bool {
	raw = strings.TrimSpace(raw)

	condText, effText := splitConditional(raw)
	if condText == "" || effText == "" {
		return false
	}

	cond := parseConditionText(condText)
	eff := parseEffectText(effText, src)

	if cond == nil && eff == nil {
		return false
	}

	if cond == nil {
		cond = &gameast.Condition{Kind: "raw", Args: []interface{}{condText}}
	}

	if !evalCondition(gs, src, cond) {
		gs.LogEvent(Event{
			Kind:   "conditional_effect_false",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"condition": condText,
			},
		})
		return true
	}

	if eff != nil {
		gs.LogEvent(Event{
			Kind:   "conditional_effect_fires",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"condition": condText,
				"effect":    effText,
			},
		})
		ResolveEffect(gs, src, eff)
		return true
	}

	return false
}

// splitConditional splits "if CONDITION, EFFECT" into condition and effect text.
func splitConditional(raw string) (string, string) {
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "if ") {
		return "", ""
	}

	text := raw[3:]

	idx := strings.Index(text, ", ")
	if idx < 0 {
		return "", ""
	}

	cond := strings.TrimSpace(text[:idx])
	eff := strings.TrimSpace(text[idx+2:])

	return strings.ToLower(cond), strings.ToLower(eff)
}

var (
	reYouControl      = regexp.MustCompile(`^you control (?:a |an |another )?(.+)$`)
	reYouDontControl  = regexp.MustCompile(`^you (?:don't|do not) control (?:a |an )?(.+)$`)
	reControlNo       = regexp.MustCompile(`^you control no (.+)$`)
	reGraveyardCount  = regexp.MustCompile(`^there are (?:(\d+)|five|four|three|seven|ten) or more cards in your graveyard`)
	reGraveyardGE     = regexp.MustCompile(`^(?:you have |there are )(?:(\d+)|five|four|three|seven|ten) or more (?:cards? )?(?:in (?:your )?graveyard|(?:creature|instant|sorcery|mana value) cards? in your graveyard|mana values? among cards in your graveyard)`)
	reLifeGE          = regexp.MustCompile(`^(?:you have|your life total is) (\d+) or more life`)
	reLifeLE          = regexp.MustCompile(`^(?:you have|your life total is) (\d+) or less life`)
	reLifeMoreThan    = regexp.MustCompile(`^you have at least (\d+) life more than your starting life total`)
	reKicked          = regexp.MustCompile(`^(?:it|~|this creature|this permanent|this spell) was kicked`)
	reKickedTwice     = regexp.MustCompile(`^(?:it|~) was kicked twice`)
	reDescended       = regexp.MustCompile(`^you (?:have )?descended this turn`)
	reAttacked        = regexp.MustCompile(`^you attacked (?:with a creature )?this turn`)
	reOpponentDiscard = regexp.MustCompile(`^an opponent discarded a card this turn`)
	rePermanentLeft   = regexp.MustCompile(`^a permanent left the battlefield`)
	reFirstCombat     = regexp.MustCompile(`^it'?s the first combat phase`)
	reNotYourTurn     = regexp.MustCompile(`^it'?s not (?:your|their) turn`)
	reYourTurn        = regexp.MustCompile(`^it'?s your turn`)
	reSelfIs          = regexp.MustCompile(`^(?:it|~|this creature|this permanent|that creature) (?:is|was) (?:a |an )?(.+)$`)
	reCounterOnSelf   = regexp.MustCompile(`^(?:it|~|this creature) has (?:a |an |one or more )?(\+\d+/\+\d+|\-\d+/\-\d+|\w+) counters? on it`)
	reNoCounterOnSelf = regexp.MustCompile(`^(?:it|~|this creature) (?:has|had) no (\+\d+/\+\d+|\-\d+/\-\d+|\w+) counters? on it`)
	reManaSpent       = regexp.MustCompile(`^at least (\d+) (\w+) mana was spent`)
)

// parseConditionText converts condition text into a structured Condition.
func parseConditionText(cond string) *gameast.Condition {
	cond = strings.TrimSpace(strings.ToLower(cond))

	if reKickedTwice.MatchString(cond) {
		return &gameast.Condition{Kind: "kicked", Args: []interface{}{2}}
	}
	if reKicked.MatchString(cond) {
		return &gameast.Condition{Kind: "kicked", Args: []interface{}{1}}
	}

	if m := reYouControl.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "you_control", Args: []interface{}{m[1]}}
	}
	if m := reYouDontControl.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "you_dont_control", Args: []interface{}{m[1]}}
	}
	if m := reControlNo.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "you_dont_control", Args: []interface{}{m[1]}}
	}

	if m := reGraveyardCount.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "card_count_zone", Args: []interface{}{"graveyard", ">=", n}}
	}
	if m := reGraveyardGE.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "card_count_zone", Args: []interface{}{"graveyard", ">=", n}}
	}

	if m := reLifeGE.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "life_threshold", Args: []interface{}{">=", n}}
	}
	if m := reLifeLE.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "life_threshold", Args: []interface{}{"<=", n}}
	}
	if m := reLifeMoreThan.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "life_threshold", Args: []interface{}{">=", 20 + n}}
	}

	if reDescended.MatchString(cond) {
		return &gameast.Condition{Kind: "descended_this_turn"}
	}
	if reAttacked.MatchString(cond) {
		return &gameast.Condition{Kind: "you_attacked_this_turn"}
	}
	if reOpponentDiscard.MatchString(cond) {
		return &gameast.Condition{Kind: "opponent_discarded_this_turn"}
	}
	if rePermanentLeft.MatchString(cond) {
		return &gameast.Condition{Kind: "permanent_left_this_turn"}
	}
	if reFirstCombat.MatchString(cond) {
		return &gameast.Condition{Kind: "first_combat_phase"}
	}
	if reNotYourTurn.MatchString(cond) {
		return &gameast.Condition{Kind: "not_your_turn"}
	}
	if reYourTurn.MatchString(cond) {
		return &gameast.Condition{Kind: "your_turn"}
	}

	if m := reCounterOnSelf.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "counter_on_self", Args: []interface{}{m[1], ">=", 1}}
	}
	if m := reNoCounterOnSelf.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "counter_on_self", Args: []interface{}{m[1], "==", 0}}
	}

	if m := reManaSpent.FindStringSubmatch(cond); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Condition{Kind: "mana_spent", Args: []interface{}{m[2], n}}
	}

	if m := reSelfIs.FindStringSubmatch(cond); m != nil {
		return &gameast.Condition{Kind: "self_is_type", Args: []interface{}{m[1]}}
	}

	return &gameast.Condition{Kind: "raw", Args: []interface{}{cond}}
}

var (
	reDealDamage  = regexp.MustCompile(`^(?:it |~ |that (?:creature |permanent )?)?deals? (\d+) damage`)
	reDraw        = regexp.MustCompile(`^(?:you )?(?:may )?draw (?:a |(\d+) )?cards?`)
	reDrawCard    = regexp.MustCompile(`^(?:you )?draw a card`)
	reGainLife    = regexp.MustCompile(`^(?:you )?(?:may )?gain (\d+) life`)
	reLoseLife    = regexp.MustCompile(`^(?:each opponent|target opponent|that player) loses? (\d+) life`)
	reYouLoseLife = regexp.MustCompile(`^you lose (\d+) life`)
	rePutCounter  = regexp.MustCompile(`^put (?:a |an |(\d+) )?(\+\d+/\+\d+|\-\d+/\-\d+|\w+) counters? on`)
	reCreateToken = regexp.MustCompile(`^create (?:a |an |(\d+|two|three|four|five|six|seven|eight|nine|ten) )?(.+?) (?:creature |artifact )?tokens?`)
	reDestroy     = regexp.MustCompile(`^destroy (?:target |that )?`)
	reExile       = regexp.MustCompile(`^exile (?:target |that )?`)
	reUntap       = regexp.MustCompile(`^untap (?:all |each |target |that )?`)
	reReturn      = regexp.MustCompile(`^return (?:it|~|this (?:creature|card)|that (?:creature|card|permanent)|target )`)
	reSacrifice   = regexp.MustCompile(`^sacrifice (?:it|~|this|that|a |an )`)
	reDiscard     = regexp.MustCompile(`^(?:each player |each opponent |that player )?discards? (?:a |an? |(\d+|two|three|four|five|six|seven|eight|nine|ten) )?cards?`)
	reWinGame     = regexp.MustCompile(`^(?:you |that player )?wins? the game`)
	// Compound effects: "each opponent loses N life and you gain N life"
	reLoseGain = regexp.MustCompile(`^(?:each opponent|target (?:opponent|player)) loses? (\d+) life and you gain (\d+) life`)
	reDrawGain = regexp.MustCompile(`^you (?:draw a card|gain \d+ life) and (?:you )?(?:draw a card|gain \d+ life)`)
)

// parseEffectText attempts to convert effect text into a structured Effect.
func parseEffectText(eff string, src *Permanent) gameast.Effect {
	eff = strings.TrimSpace(strings.ToLower(eff))

	// Compound: "each opponent loses N life and you gain N life"
	if m := reLoseGain.FindStringSubmatch(eff); m != nil {
		loseN, _ := strconv.Atoi(m[1])
		gainN, _ := strconv.Atoi(m[2])
		return &gameast.Sequence{Items: []gameast.Effect{
			&gameast.LoseLife{Amount: *gameast.NumInt(loseN)},
			&gameast.GainLife{Amount: *gameast.NumInt(gainN)},
		}}
	}

	if reWinGame.MatchString(eff) {
		return &gameast.WinGame{}
	}

	if m := reDealDamage.FindStringSubmatch(eff); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.Damage{Amount: *gameast.NumInt(n)}
	}

	if reDrawCard.MatchString(eff) {
		return &gameast.Draw{Count: *gameast.NumInt(1)}
	}
	if m := reDraw.FindStringSubmatch(eff); m != nil {
		n := 1
		if m[1] != "" {
			n, _ = strconv.Atoi(m[1])
		}
		return &gameast.Draw{Count: *gameast.NumInt(n)}
	}

	if m := reGainLife.FindStringSubmatch(eff); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.GainLife{Amount: *gameast.NumInt(n)}
	}

	if m := reLoseLife.FindStringSubmatch(eff); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.LoseLife{Amount: *gameast.NumInt(n)}
	}
	if m := reYouLoseLife.FindStringSubmatch(eff); m != nil {
		n, _ := strconv.Atoi(m[1])
		return &gameast.LoseLife{Amount: *gameast.NumInt(n)}
	}

	if rePutCounter.MatchString(eff) {
		m := rePutCounter.FindStringSubmatch(eff)
		n := 1
		if m[1] != "" {
			n, _ = strconv.Atoi(m[1])
		}
		return &gameast.CounterMod{
			Op:          "put",
			Count:       *gameast.NumInt(n),
			CounterKind: m[2],
		}
	}

	if reCreateToken.MatchString(eff) && !strings.Contains(eff, "copies of") && !strings.Contains(eff, "copy of") {
		m := reCreateToken.FindStringSubmatch(eff)
		n := 1
		if m[1] != "" {
			if wn, ok := wordToInt(m[1]); ok {
				n = wn
			} else if parsed, err := strconv.Atoi(m[1]); err == nil {
				n = parsed
			}
		}
		return &gameast.CreateToken{Count: *gameast.NumInt(n), Types: []string{m[2]}}
	}

	if reDestroy.MatchString(eff) {
		return &gameast.Destroy{}
	}

	if reExile.MatchString(eff) {
		return &gameast.Exile{}
	}

	if reUntap.MatchString(eff) {
		return &gameast.UntapEffect{}
	}

	if reReturn.MatchString(eff) {
		return &gameast.Bounce{}
	}

	if reSacrifice.MatchString(eff) {
		return &gameast.Sacrifice{}
	}

	if m := reDiscard.FindStringSubmatch(eff); m != nil {
		n := 1
		if m[1] != "" {
			if wn, ok := wordToInt(m[1]); ok {
				n = wn
			} else if parsed, err := strconv.Atoi(m[1]); err == nil {
				n = parsed
			}
		}
		return &gameast.Discard{Count: *gameast.NumInt(n)}
	}

	return nil
}
