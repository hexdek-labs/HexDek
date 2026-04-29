package gameengine

import "strings"

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

// CR §702.33 — Kicker
func IsKicked(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	if v, ok := item.CostMeta["kicked"]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return false
}

func ApplyKicker(gs *GameState, item *StackItem) {
	if gs == nil || item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["kicked"] = true
	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "kicker",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"rule": "702.33",
		},
	})
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

func TickSuspend(gs *GameState) {
	if gs == nil {
		return
	}
	if gs.Flags == nil {
		return
	}

	prefix := "suspend_counters:"
	var toRemove []string
	var toCast []struct {
		seat int
		card *Card
	}

	for key, counters := range gs.Flags {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):]
		sepIdx := -1
		for i := len(rest) - 1; i >= 0; i-- {
			if rest[i] == ':' {
				sepIdx = i
				break
			}
		}
		if sepIdx < 0 {
			continue
		}
		cardName := rest[:sepIdx]
		seatStr := rest[sepIdx+1:]

		seatIdx := 0
		for _, ch := range seatStr {
			seatIdx = seatIdx*10 + int(ch-'0')
		}

		if seatIdx < 0 || seatIdx >= len(gs.Seats) {
			continue
		}

		counters--
		if counters <= 0 {
			toRemove = append(toRemove, key)
			seat := gs.Seats[seatIdx]
			for _, c := range seat.Exile {
				if c != nil && c.DisplayName() == cardName {
					toCast = append(toCast, struct {
						seat int
						card *Card
					}{seatIdx, c})
					break
				}
			}
		} else {
			gs.Flags[key] = counters
			gs.LogEvent(Event{
				Kind:   "suspend_tick",
				Seat:   seatIdx,
				Source: cardName,
				Amount: counters,
				Details: map[string]interface{}{
					"rule": "702.62",
				},
			})
		}
	}

	for _, key := range toRemove {
		delete(gs.Flags, key)
	}

	for _, entry := range toCast {
		seat := gs.Seats[entry.seat]
		exileIdx := -1
		for i, c := range seat.Exile {
			if c == entry.card {
				exileIdx = i
				break
			}
		}
		if exileIdx >= 0 {
			removeCardFromZone(gs, entry.seat, entry.card, "exile")
			item := &StackItem{
				Card:       entry.card,
				Controller: entry.seat,
				CastZone:   ZoneExile,
				CostMeta:   map[string]interface{}{"free_cast": true, "suspend": true},
			}
			PushStackItem(gs, item)
			gs.LogEvent(Event{
				Kind:   "suspend_cast",
				Seat:   entry.seat,
				Source: entry.card.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.62",
				},
			})
		}
	}
}

// CR §702.63 — Vanishing
func ApplyVanishing(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		return
	}
	timeCounters, has := perm.Counters["time"]
	if !has {
		return
	}
	perm.AddCounter("time", -1)
	timeCounters--

	if timeCounters <= 0 {
		SacrificePermanent(gs, perm, "vanishing (last time counter removed)")
		gs.LogEvent(Event{
			Kind:   "vanishing_sacrifice",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule": "702.63",
			},
		})
	} else {
		gs.LogEvent(Event{
			Kind:   "vanishing_tick",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: timeCounters,
			Details: map[string]interface{}{
				"rule": "702.63",
			},
		})
	}
}

// CR §702.32 — Fading
func ApplyFading(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		return
	}
	fadeCounters, has := perm.Counters["fade"]
	if !has {
		return
	}
	perm.AddCounter("fade", -1)
	fadeCounters--

	if fadeCounters <= 0 {
		SacrificePermanent(gs, perm, "fading (last fade counter removed)")
		gs.LogEvent(Event{
			Kind:   "fading_sacrifice",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule": "702.32",
			},
		})
	} else {
		gs.LogEvent(Event{
			Kind:   "fading_tick",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: fadeCounters,
			Details: map[string]interface{}{
				"rule": "702.32",
			},
		})
	}
}

// CR §702.30 — Echo
func CheckEcho(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["echo_paid"] > 0 {
		return
	}
	enteredTurn := perm.Flags["entered_turn"]
	if enteredTurn == 0 {
		perm.Flags["entered_turn"] = gs.Turn
		return
	}
	if gs.Turn <= enteredTurn {
		return
	}

	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	echoCost := perm.Card.CMC
	if perm.Flags != nil {
		if v, ok := perm.Flags["echo_cost"]; ok && v > 0 {
			echoCost = v
		}
	}

	if seat.ManaPool >= echoCost {
		seat.ManaPool -= echoCost
		SyncManaAfterSpend(seat)
		perm.Flags["echo_paid"] = 1
		gs.LogEvent(Event{
			Kind:   "echo_paid",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: echoCost,
			Details: map[string]interface{}{
				"rule": "702.30",
			},
		})
	} else {
		SacrificePermanent(gs, perm, "echo unpaid")
		gs.LogEvent(Event{
			Kind:   "echo_sacrifice",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: echoCost,
			Details: map[string]interface{}{
				"rule": "702.30",
			},
		})
	}
}

// CR §702.112 — Renown
func CheckRenown(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	if perm.Flags != nil && perm.Flags["renowned"] > 0 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["renowned"] = 1
	perm.AddCounter("+1/+1", n)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "renown",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.112",
		},
	})
}

// CR §702.134 — Mentor
func ApplyMentor(gs *GameState, attacker *Permanent) {
	if gs == nil || attacker == nil {
		return
	}
	if !attacker.HasKeyword("mentor") {
		return
	}
	seatIdx := attacker.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	atkPower := attacker.Power()
	var best *Permanent
	bestPower := -1

	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p == attacker || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			continue
		}
		if p.Flags["attacking"] <= 0 {
			continue
		}
		pw := p.Power()
		if pw < atkPower && pw > bestPower {
			best = p
			bestPower = pw
		}
	}

	if best == nil {
		return
	}

	best.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "mentor",
		Seat:   seatIdx,
		Source: attacker.Card.DisplayName(),
		Details: map[string]interface{}{
			"target":       best.Card.DisplayName(),
			"target_power": bestPower,
			"rule":         "702.134",
		},
	})
}

// CR §702.149 — Training
func ApplyTraining(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !perm.HasKeyword("training") {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	myPower := perm.Power()
	found := false

	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			continue
		}
		if p.Flags["attacking"] <= 0 {
			continue
		}
		if p.Power() > myPower {
			found = true
			break
		}
	}

	if !found {
		return
	}

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "training",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"rule": "702.149",
		},
	})
}

// CR §702.110 — Exploit
func CheckExploit(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	if !perm.HasKeyword("exploit") {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	var victim *Permanent
	lowestTough := 1<<31 - 1
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		t := p.Toughness()
		if t < lowestTough {
			lowestTough = t
			victim = p
		}
	}

	if victim == nil {
		return false
	}

	SacrificePermanent(gs, victim, "exploit")

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["exploited"] = 1

	gs.LogEvent(Event{
		Kind:   "exploit",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"sacrificed": victim.Card.DisplayName(),
			"rule":       "702.110",
		},
	})
	return true
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

func HasDecayed(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["decayed"] > 0
}

func CheckDecayedEndOfCombat(gs *GameState) {
	if gs == nil {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		var toSacrifice []*Permanent
		for _, p := range seat.Battlefield {
			if p == nil {
				continue
			}
			if !HasDecayed(p) {
				continue
			}
			if p.Flags != nil && p.Flags["attacking"] > 0 {
				toSacrifice = append(toSacrifice, p)
			}
		}
		for _, p := range toSacrifice {
			SacrificePermanent(gs, p, "decayed (attacked)")
			gs.LogEvent(Event{
				Kind:   "decayed_sacrifice",
				Seat:   p.Controller,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.147",
				},
			})
		}
	}
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

// §702.28 — Shadow
func HasShadow(perm *Permanent) bool {
	return perm != nil && perm.HasKeyword("shadow")
}

// §702.70 — Poisonous
func ApplyPoisonous(gs *GameState, attacker *Permanent, defenderSeat, n int) {
	if gs == nil || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	gs.Seats[defenderSeat].PoisonCounters += n
	gs.LogEvent(Event{
		Kind: "poison", Seat: attacker.Controller, Target: defenderSeat,
		Source: attacker.Card.DisplayName(), Amount: n,
		Details: map[string]interface{}{"rule": "702.70", "reason": "poisonous"},
	})
}

// §702.43 — Modular
func TriggerModular(gs *GameState, dying *Permanent) {
	if gs == nil || dying == nil || !dying.HasKeyword("modular") {
		return
	}
	n := dying.Counters["+1/+1"]
	if n <= 0 {
		return
	}
	seat := dying.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsCreature() && p.Card != nil &&
			strings.Contains(strings.ToLower(strings.Join(p.Card.Types, " ")), "artifact") {
			p.AddCounter("+1/+1", n)
			gs.LogEvent(Event{
				Kind: "modular", Seat: seat,
				Source: dying.Card.DisplayName(),
				Details: map[string]interface{}{
					"target":   p.Card.DisplayName(),
					"counters": n,
					"rule":     "702.43",
				},
			})
			break
		}
	}
}

// §702.37 — Morph (cost payment to turn face-up)
func PayMorphCost(gs *GameState, perm *Permanent, cost int) bool {
	if gs == nil || perm == nil {
		return false
	}
	if perm.Flags == nil || perm.Flags["face_down"] != 1 {
		return false
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	p := EnsureTypedPool(s)
	if p.Total() < cost {
		return false
	}
	if !PayGenericCost(gs, s, cost, "morph", "morph_turn_face_up", perm.Card.DisplayName()) {
		s.ManaPool -= cost
		SyncManaAfterSpend(s)
	}
	perm.Flags["face_down"] = 0
	perm.SummoningSick = false
	gs.LogEvent(Event{
		Kind: "morph_turn_face_up", Seat: seat,
		Source: perm.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{"rule": "702.37"},
	})
	return true
}

// CastFaceDown casts a card from hand as a face-down 2/2 creature for {3}.
// CR §702.37a: "You may cast this card face down as a 2/2 creature for {3}."
// The card is placed on the battlefield with Card.FaceDown = true (so the
// layers system treats it as a 2/2 colorless nameless creature) and
// Flags["face_down"] = 1 (so PayMorphCost can detect it for turning face-up).
func CastFaceDown(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Check mana: costs {3}.
	p := EnsureTypedPool(seat)
	if p.Total() < 3 {
		return false
	}

	// Pay {3}.
	if !PayGenericCost(gs, seat, 3, "morph", "cast_face_down", "face-down creature") {
		seat.ManaPool -= 3
		SyncManaAfterSpend(seat)
	}

	// Remove from hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return false
	}
	seat.Hand = append(seat.Hand[:handIdx], seat.Hand[handIdx+1:]...)

	// Mark the card as face-down (layers system reads Card.FaceDown).
	card.FaceDown = true

	// Create a face-down permanent.
	perm := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags: map[string]int{
			"face_down":      1, // read by PayMorphCost for turn-face-up
			"morph_creature": 1,
		},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind: "cast_face_down", Seat: seatIdx,
		Source: "face-down creature",
		Amount: 3,
		Details: map[string]interface{}{
			"rule": "702.37",
		},
	})
	return true
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

// FaceDownPower returns 2 for face-down creatures (the default 2/2).
func FaceDownPower() int { return 2 }

// FaceDownToughness returns 2 for face-down creatures (the default 2/2).
func FaceDownToughness() int { return 2 }

// §701.39 — Bolster
func Bolster(gs *GameState, seatIdx, n int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return
	}
	var weakest *Permanent
	minTough := 999999
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		t := p.Toughness()
		if t < minTough {
			minTough = t
			weakest = p
		}
	}
	if weakest == nil {
		return
	}
	weakest.AddCounter("+1/+1", n)
	gs.LogEvent(Event{
		Kind: "bolster", Seat: seatIdx,
		Source: weakest.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{"rule": "701.39"},
	})
}

// §701.43 — Exert
func ExertPermanent(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["exerted"] = 1
	perm.DoesNotUntap = true
	gs.LogEvent(Event{
		Kind: "exert", Seat: perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{"rule": "701.43"},
	})
}

// §701.44 — Explore
func Explore(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		return
	}
	top := s.Library[0]
	if top != nil && cardHasType(top, "land") {
		MoveCard(gs, top, seat, "library", "hand", "tutor-to-hand")
		gs.LogEvent(Event{
			Kind: "explore_land", Seat: seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{"revealed": top.DisplayName(), "rule": "701.44"},
		})
	} else {
		perm.AddCounter("+1/+1", 1)
		MoveCard(gs, top, seat, "library", "hand", "tutor-to-hand")
		gs.LogEvent(Event{
			Kind: "explore_nonland", Seat: seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{"revealed": top.DisplayName(), "rule": "701.44"},
		})
	}
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

// §701.53 — Incubate
func Incubate(gs *GameState, seatIdx, n int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	token := &Card{
		Name:  "Incubator Token",
		Owner: seatIdx,
		Types: []string{"token", "artifact", "incubator"},
	}
	perm := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{"+1/+1": n},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(Event{
		Kind: "incubate", Seat: seatIdx,
		Amount: n,
		Details: map[string]interface{}{"rule": "701.53"},
	})
}

