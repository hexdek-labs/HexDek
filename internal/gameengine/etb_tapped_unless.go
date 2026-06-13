package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// etb_tapped_unless.go — generic handler for the `etb_tapped_unless` static
// ability KIND (CR §614.1d enters-the-battlefield-tapped replacement).
//
// "This land enters the battlefield tapped unless <condition>." ~105 lands
// (checklands, fastlands, slowlands, "a player has 13 or less life" lands,
// "two or more opponents" lands, legendary-matters lands, …) parse the unless
// clause to an inert `etb_tapped_unless` Modification node whose only arg is
// the raw English condition. Before this handler those lands hit the
// log-only static bucket in resolveModificationEffect and entered UNTAPPED
// for free — a real tempo/advantage bug shared by the whole class.
//
// ApplyEntersTappedUnless mirrors ApplyStaticETBCounters: it is invoked once
// at ETB (from both the cast path resolvePermanentSpellETB and the non-cast
// path FirePermanentETBTriggers), iterates the entering permanent's Static
// abilities for ModKind == "etb_tapped_unless", evaluates the condition, and
// taps the permanent when the condition is NOT met.
//
// Conservative default: when the condition text cannot be parsed, the land
// enters TAPPED. "Enters tapped unless X" makes the untapped state the
// exception; defaulting to tapped never hands a player a free untapped land
// it should not have — the worst case is a parseable-later land entering
// tapped (a minor symmetric tempo cost), never an illegal advantage.
func ApplyEntersTappedUnless(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	// Face-down permanents have no abilities (CR §708.4).
	if perm.Flags != nil && perm.Flags["face_down"] != 0 {
		return
	}
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind != "etb_tapped_unless" {
			continue
		}
		cond := ""
		if len(st.Modification.Args) > 0 {
			if s, ok := st.Modification.Args[0].(string); ok {
				cond = strings.ToLower(strings.TrimSpace(s))
			}
		}
		met, parsed := entersUntappedConditionMet(gs, perm, cond)
		if met {
			// Condition satisfied → land enters untapped (default). No-op,
			// but log for downstream analysis / parity.
			gs.LogEvent(Event{
				Kind:   "enters_untapped",
				Seat:   perm.Controller,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"mod_kind":  "etb_tapped_unless",
					"condition": cond,
					"parsed":    parsed,
				},
			})
			return
		}
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["enters_tapped"] = 1
		perm.Tapped = true
		gs.LogEvent(Event{
			Kind:   "enters_tapped",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"mod_kind":  "etb_tapped_unless",
				"condition": cond,
				"parsed":    parsed,
			},
		})
		return
	}
}

// entersUntappedConditionMet evaluates the raw "unless" condition of an
// etb_tapped_unless land. Returns (met, parsed): met=true means the land
// enters untapped; parsed=false means the text was not recognized (caller
// treats unrecognized as "enters tapped").
func entersUntappedConditionMet(gs *GameState, perm *Permanent, cond string) (met bool, parsed bool) {
	if cond == "" {
		return false, false
	}
	seat := perm.Controller

	// "it's your turn"
	if cond == "it's your turn" || cond == "its your turn" {
		return gs.Active == seat, true
	}

	// Life-total lands: "a player has 13 or less life".
	if strings.Contains(cond, "life") && strings.Contains(cond, "player has") {
		n, ok := firstNumber(cond)
		if !ok {
			return false, false
		}
		atMost := strings.Contains(cond, "or less") || strings.Contains(cond, "or fewer")
		for _, s := range gs.Seats {
			if s == nil || s.Lost {
				continue
			}
			if atMost && s.Life <= n {
				return true, true
			}
			if !atMost && s.Life >= n {
				return true, true
			}
		}
		return false, true
	}

	// "you have two or more opponents"
	if strings.Contains(cond, "opponents") && !strings.Contains(cond, "control") {
		n, ok := firstNumber(cond)
		if !ok {
			return false, false
		}
		living := 0
		for _, opp := range gs.Opponents(seat) {
			if gs.Seats[opp] != nil && !gs.Seats[opp].Lost {
				living++
			}
		}
		return compareCount(living, n, cond), true
	}

	// "your opponents control eight or more lands"
	if strings.HasPrefix(cond, "your opponents control") && strings.Contains(cond, "lands") {
		n, ok := firstNumber(cond)
		if !ok {
			return false, false
		}
		count := 0
		for _, opp := range gs.Opponents(seat) {
			count += countSeatLands(gs, opp, nil, "")
		}
		return compareCount(count, n, cond), true
	}

	// "you control ..." family.
	if strings.HasPrefix(cond, "you control") {
		// Legendary (green) creature.
		if strings.Contains(cond, "legendary") && strings.Contains(cond, "creature") {
			green := strings.Contains(cond, "green")
			return controllerHasLegendaryCreature(gs, seat, green), true
		}
		// Mount or vehicle.
		if strings.Contains(cond, "mount") || strings.Contains(cond, "vehicle") {
			return controllerControls(gs, seat, func(p *Permanent) bool {
				return cardHasType(p.Card, "mount") || cardHasType(p.Card, "vehicle")
			}), true
		}
		// Basic land(s).
		if strings.Contains(cond, "basic land") {
			n := 1
			if v, ok := firstNumber(cond); ok {
				n = v
			}
			count := countSeatLands(gs, seat, func(p *Permanent) bool {
				return cardHasType(p.Card, "basic")
			}, "")
			return compareCount(count, n, cond), true
		}
		// "[N] or more/fewer other <subtype>(s)" and "[N] or more/fewer other lands".
		if strings.Contains(cond, "other") {
			n, ok := firstNumber(cond)
			if !ok {
				return false, false
			}
			sub := firstBasicSubtype(cond) // "" → any land
			count := countSeatLands(gs, seat, func(p *Permanent) bool {
				if p == perm {
					return false
				}
				return sub == "" || cardHasType(p.Card, sub)
			}, "")
			return compareCount(count, n, cond), true
		}
		// "you control a <subtype> or a <subtype>" / "you control a <subtype>".
		if subs := basicSubtypesIn(cond); len(subs) > 0 {
			for _, sub := range subs {
				if controllerControls(gs, seat, func(p *Permanent) bool {
					return p.IsLand() && cardHasType(p.Card, sub)
				}) {
					return true, true
				}
			}
			return false, true
		}
	}

	return false, false
}

// --- small helpers ------------------------------------------------------

var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
}

// firstNumber returns the first integer in the text, recognizing digits and
// number words (one..fifteen).
func firstNumber(s string) (int, bool) {
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.'
	}) {
		if n, ok := numberWords[tok]; ok {
			return n, true
		}
		if v, ok := atoiSafe(tok); ok {
			return v, true
		}
	}
	return 0, false
}

// compareCount applies the "or more/greater" (>=) vs "or fewer/less" (<=)
// comparator found in cond to count vs n. Defaults to >= when neither phrase
// is present (e.g. "you control a basic land" with implicit n=1).
func compareCount(count, n int, cond string) bool {
	if strings.Contains(cond, "or fewer") || strings.Contains(cond, "or less") {
		return count <= n
	}
	return count >= n
}

var basicSubtypeList = []string{"plains", "island", "swamp", "mountain", "forest"}

// firstBasicSubtype returns the first basic land subtype mentioned, or "".
func firstBasicSubtype(cond string) string {
	for _, s := range basicSubtypeList {
		// match singular or plural (forest/forests, etc.)
		if strings.Contains(cond, s) {
			return s
		}
	}
	return ""
}

// basicSubtypesIn returns every basic land subtype mentioned in cond.
func basicSubtypesIn(cond string) []string {
	var out []string
	for _, s := range basicSubtypeList {
		if strings.Contains(cond, s) {
			out = append(out, s)
		}
	}
	return out
}

// countSeatLands counts lands controlled by seat matching pred (nil = all
// lands). The unused string param keeps the signature flexible for callers.
func countSeatLands(gs *GameState, seat int, pred func(*Permanent) bool, _ string) int {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		if pred == nil || pred(p) {
			n++
		}
	}
	return n
}

func controllerControls(gs *GameState, seat int, pred func(*Permanent) bool) bool {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return false
	}
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if pred(p) {
			return true
		}
	}
	return false
}

func controllerHasLegendaryCreature(gs *GameState, seat int, green bool) bool {
	return controllerControls(gs, seat, func(p *Permanent) bool {
		if !p.IsCreature() || !cardHasType(p.Card, "legendary") {
			return false
		}
		if !green {
			return true
		}
		for _, c := range p.Card.Colors {
			if strings.EqualFold(c, "G") {
				return true
			}
		}
		return false
	})
}
