package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

func itoaBatch(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// HasKickerKeyword reports whether the card carries a "kicker" or
// "multikicker" keyword (CR §702.33). Detection is AST-driven via the
// Keyword node Name so it works regardless of whether the parser stashed
// the cost as a numeric arg or raw text.
func HasKickerKeyword(card *Card) bool {
	return cardHasKeywordByName(card, "kicker") ||
		cardHasKeywordByName(card, "multikicker")
}

// IsMultikicker reports whether the card's kicker keyword is the
// "multikicker" variant — the cost may be paid any number of times
// (CR §702.33d) rather than at most once.
func IsMultikicker(card *Card) bool {
	return cardHasKeywordByName(card, "multikicker")
}

// KickerCost returns the generic mana cost of ONE kick for the card's
// kicker / multikicker keyword. Reads the cost arg off the keyword node
// via keywordArgCostStrict — mana-string args ("{3}", "{1}{b}") and
// numeric args are both honored. Returns 0 if the card has no kicker or
// the parser couldn't recover a cost — callers treat 0 as "no affordable
// kicker" so a parser miss reads as DECLINE, never as a CMC-guessed
// price (the pre-r61.1 fallback that kicked Burst Lightning for {1}).
func KickerCost(card *Card) int {
	if card == nil {
		return 0
	}
	if cardHasKeywordByName(card, "multikicker") {
		if c, ok := keywordArgCostStrict(card, "multikicker"); ok && c > 0 {
			return c
		}
	}
	if cardHasKeywordByName(card, "kicker") {
		if c, ok := keywordArgCostStrict(card, "kicker"); ok && c > 0 {
			return c
		}
	}
	return 0
}

// StampKickResult records the chosen kick count onto the StackItem's
// CostMeta in the single canonical shape every downstream consumer reads:
//
//   - CostMeta["kicked"]          bool  — was the spell kicked at all
//   - CostMeta["multikick_count"] int   — how many times (1 for single kicker)
//
// This mirrors the ChosenX → gs.Flags mirroring pattern. The matching
// permanent-side mirror happens at ETB (see MirrorKickFlagsToPermanent).
func StampKickResult(item *StackItem, kicks int) {
	if item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	if kicks <= 0 {
		item.CostMeta["kicked"] = false
		item.CostMeta["multikick_count"] = 0
		return
	}
	item.CostMeta["kicked"] = true
	item.CostMeta["multikick_count"] = kicks
}

// MirrorKickFlagsToPermanent copies the canonical kick metadata from the
// resolving StackItem's CostMeta onto the entering permanent's Flags, so
// "if kicked" ETB statics (Grunn), multikick-count consumers (Zethi,
// Everflowing Chalice), and the evalCondition "kicked" predicate all see
// the same value. Mirrors the ChosenX → gs.Flags["_cast_chosen_x"]
// pattern. Sets:
//
//   - perm.Flags["kicked"]          N (the kick count; 0 when unkicked)
//   - perm.Flags["multikick_count"] N
//
// Both keys carry the kick count: resolve.go's evalCondition "kicked" case
// reads perm.Flags["kicked"] as a count and compares it against the
// condition's needed-times arg (so "was kicked" = >=1 and "was kicked
// twice" = >=2 both work), while Zethi / Chalice read multikick_count.
func MirrorKickFlagsToPermanent(item *StackItem, perm *Permanent) {
	if item == nil || perm == nil || item.CostMeta == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	kicked := false
	if v, ok := item.CostMeta["kicked"]; ok {
		kicked, _ = v.(bool)
	}
	count := 0
	if v, ok := item.CostMeta["multikick_count"]; ok {
		if n, ok2 := asInt(v); ok2 {
			count = n
		}
	}
	if !kicked && count == 0 {
		// Card was not kicked (or carries no kicker) — leave flags unset.
		return
	}
	perm.Flags["kicked"] = count
	perm.Flags["multikick_count"] = count
}

// CR §702.79 — Persist
func CheckPersist(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !perm.HasKeyword("persist") {
		return
	}
	if perm.Counters != nil && perm.Counters["-1/-1"] > 0 {
		return
	}
	seatIdx := perm.Owner
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	token := &Card{
		Name:          perm.Card.Name,
		Owner:         seatIdx,
		BasePower:     perm.Card.BasePower,
		BaseToughness: perm.Card.BaseToughness,
		Types:         perm.Card.Types,
		Colors:        perm.Card.Colors,
		CMC:           perm.Card.CMC,
	}
	returned := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{"-1/-1": 1},
		Flags:      map[string]int{},
	}
	if perm.Card.AST != nil {
		token.AST = perm.Card.AST
	}
	seat.Battlefield = append(seat.Battlefield, returned)
	RegisterReplacementsForPermanent(gs, returned)
	FirePermanentETBTriggers(gs, returned)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "persist",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.79",
		},
	})
	_ = returned
}

// CR §702.93 — Undying
func CheckUndying(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !perm.HasKeyword("undying") {
		return
	}
	if perm.Counters != nil && perm.Counters["+1/+1"] > 0 {
		return
	}
	seatIdx := perm.Owner
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	token := &Card{
		Name:          perm.Card.Name,
		Owner:         seatIdx,
		BasePower:     perm.Card.BasePower,
		BaseToughness: perm.Card.BaseToughness,
		Types:         perm.Card.Types,
		Colors:        perm.Card.Colors,
		CMC:           perm.Card.CMC,
	}
	returned := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{"+1/+1": 1},
		Flags:      map[string]int{},
	}
	if perm.Card.AST != nil {
		token.AST = perm.Card.AST
	}
	seat.Battlefield = append(seat.Battlefield, returned)
	RegisterReplacementsForPermanent(gs, returned)
	FirePermanentETBTriggers(gs, returned)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "undying",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.93",
		},
	})
	_ = returned
}

// CR §702.24 — Cumulative Upkeep
func ApplyCumulativeUpkeep(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	perm.AddCounter("age", 1)
	ageCounters := 0
	if perm.Counters != nil {
		ageCounters = perm.Counters["age"]
	}

	costPerCounter := 1
	if perm.Flags != nil {
		if v, ok := perm.Flags["cumulative_upkeep_cost"]; ok && v > 0 {
			costPerCounter = v
		}
	}
	totalCost := ageCounters * costPerCounter

	if seat.ManaPool >= totalCost {
		seat.ManaPool -= totalCost
		SyncManaAfterSpend(seat)
		gs.LogEvent(Event{
			Kind:   "cumulative_upkeep_paid",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: totalCost,
			Details: map[string]interface{}{
				"age_counters": ageCounters,
				"rule":         "702.24",
			},
		})
	} else {
		SacrificePermanent(gs, perm, "cumulative upkeep unpaid")
		gs.LogEvent(Event{
			Kind:   "cumulative_upkeep_sacrifice",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: totalCost,
			Details: map[string]interface{}{
				"age_counters": ageCounters,
				"rule":         "702.24",
			},
		})
	}
}

// CR §702.62 — Suspend
func SuspendCard(gs *GameState, seatIdx int, card *Card, timeCounters int) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return
	}

	MoveCard(gs, card, seatIdx, "hand", "exile", "effect")

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}

	key := "suspend_counters:" + card.DisplayName() + ":" + itoaBatch(seatIdx)
	gs.Flags[key] = timeCounters

	gs.LogEvent(Event{
		Kind:   "suspend",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: timeCounters,
		Details: map[string]interface{}{
			"rule": "702.62",
		},
	})
}

// CR §702.135 — Afterlife
func TriggerAfterlife(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Owner
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	for i := 0; i < n; i++ {
		token := &Card{
			Name:          "Spirit Token",
			Owner:         seatIdx,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "spirit"},
			Colors:        []string{"W", "B"},
		}
		spiritPerm := &Permanent{
			Card:       token,
			Controller: seatIdx,
			Owner:      seatIdx,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{"kw:flying": 1},
		}
		seat.Battlefield = append(seat.Battlefield, spiritPerm)
		RegisterReplacementsForPermanent(gs, spiritPerm)
		FirePermanentETBTriggers(gs, spiritPerm)
	}

	gs.LogEvent(Event{
		Kind:   "afterlife",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.135",
		},
	})
}

// CR §702.147 — Decayed
func ApplyDecayed(perm *Permanent) {
	if perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["decayed"] = 1
}

// CR §702.152 — Blitz
func ApplyBlitz(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:haste"] = 1
	perm.Flags["blitz"] = 1

	seatIdx := perm.Controller

	blitzPerm := perm
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: perm.Card.DisplayName() + " (blitz)",
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			if alive(gs, blitzPerm) {
				SacrificePermanent(gs, blitzPerm, "blitz EOT sacrifice")
			}
		},
	})

	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "on_event",
		ControllerSeat: seatIdx,
		SourceCardName: perm.Card.DisplayName() + " (blitz dies)",
		OneShot:        true,
		ConditionFn: func(gs *GameState, ev *Event) bool {
			if ev == nil {
				return false
			}
			return ev.Kind == "dies" && ev.Source == blitzPerm.Card.DisplayName()
		},
		EffectFn: func(gs *GameState) {
			if seatIdx >= 0 && seatIdx < len(gs.Seats) {
				gs.drawOne(seatIdx)
				gs.LogEvent(Event{
					Kind:   "blitz_draw",
					Seat:   seatIdx,
					Source: blitzPerm.Card.DisplayName(),
					Details: map[string]interface{}{
						"rule": "702.152",
					},
				})
			}
		},
	})

	gs.LogEvent(Event{
		Kind:   "blitz",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.152",
		},
	})
}

// IsFaceDown returns true if the permanent is face-down (morph/manifest/disguise/cloak).
func IsFaceDown(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	// Check Card.FaceDown (canonical for layers system).
	if perm.Card != nil && perm.Card.FaceDown {
		return true
	}
	// Also check Flags for backward compat.
	if perm.Flags != nil && perm.Flags["face_down"] == 1 {
		return true
	}
	return false
}

// §701.50 — Connive
func Connive(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	drawn := 0
	for i := 0; i < n && len(s.Library) > 0; i++ {
		c := s.Library[0]
		MoveCard(gs, c, seat, "library", "hand", "connive")
		drawn++
	}
	discarded := 0
	for i := 0; i < n && len(s.Hand) > 0; i++ {
		last := s.Hand[len(s.Hand)-1]
		DiscardCard(gs, last, seat)
		if last != nil && !cardHasType(last, "land") {
			perm.AddCounter("+1/+1", 1)
		}
		discarded++
	}
	gs.LogEvent(Event{
		Kind: "connive", Seat: seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"drawn": drawn, "discarded": discarded, "rule": "701.50",
		},
	})
}

// CR §702.46 — Soulshift
// "Soulshift N" means "When this permanent is put into a graveyard from
// the battlefield, you may return target Spirit card with mana value N or
// less from your graveyard to your hand."
func CheckSoulshift(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !perm.HasKeyword("soulshift") {
		return
	}
	n := soulshiftN(perm)
	if n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	// Find best Spirit card in graveyard with mana value <= N.
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		isSpirit := false
		for _, t := range c.Types {
			if strings.EqualFold(t, "spirit") {
				isSpirit = true
				break
			}
		}
		if !isSpirit {
			continue
		}
		if c.CMC > n {
			continue
		}
		if c.CMC > bestCMC {
			bestCMC = c.CMC
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return
	}

	target := seat.Graveyard[bestIdx]
	seat.Graveyard = append(seat.Graveyard[:bestIdx], seat.Graveyard[bestIdx+1:]...)
	seat.Hand = append(seat.Hand, target)

	gs.LogEvent(Event{
		Kind:   "soulshift",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":     "702.46",
			"n":        n,
			"returned": target.DisplayName(),
		},
	})
}

func soulshiftN(p *Permanent) int {
	if p == nil || p.Card == nil || p.Card.AST == nil {
		return 0
	}
	for _, ab := range p.Card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "soulshift") {
			continue
		}
		if len(kw.Args) > 0 {
			switch v := kw.Args[0].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
	}
	return 0
}
