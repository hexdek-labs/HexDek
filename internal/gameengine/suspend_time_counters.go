package gameengine

// suspend_time_counters.go — the upkeep countdowns for the time-counter
// keyword family, plus the §702.62g free-cast that fires when a suspended
// card's last time counter is removed.
//
// CR coverage:
//   - §702.62f  Suspend — "At the beginning of your upkeep, if this card is
//                suspended, remove a time counter from it."
//   - §702.62g  Suspend — "When the last time counter is removed ... cast it
//                without paying its mana cost if able ... it gains haste until
//                you lose control of it."
//   - §702.63b/c Vanishing — remove a time counter each upkeep; when the LAST
//                time counter is removed, sacrifice the permanent.
//   - §702.32b   Fading — remove a fade counter each upkeep; if you can't,
//                sacrifice the permanent.
//
// Before this file: SuspendCard exiled the card and stamped a counter that was
// never read; ApplyVanishingFadingETBCounters placed the starting time/fade
// counters but nothing counted them down. ~107 cards were inert past entry.
//
// The ticks are driven from the active seat's upkeep (tournament turn.go),
// mirroring how TickSagaChapters is driven from precombat main. They are
// gameengine-pure (no Hat dependency beyond the cast pipeline) so they can be
// unit-tested directly without the turn driver.

// TickTimeFadeCounters runs the §702.63 (vanishing) and §702.32 (fading)
// upkeep countdown for every battlefield permanent the seat controls. It is
// the low-risk half of the time-counter family — no cast pipeline, only a
// counter decrement and a canonical sacrifice. Returns the number of
// permanents sacrificed this pass.
//
// Vanishing (time counters): if the permanent has a time counter, remove one;
// if that removal emptied the last time counter, sacrifice it (§702.63c).
// Fading (fade counters): remove a fade counter; if there is none to remove,
// sacrifice it (§702.32b). The one-turn-of-life difference between the two
// keywords is faithful to the rules (a vanishing-N permanent is sacrificed on
// its Nth upkeep; a fading-N permanent on its (N+1)th).
func TickTimeFadeCounters(gs *GameState, activeSeat int) int {
	if gs == nil || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost {
		return 0
	}
	sacrificed := 0
	for _, p := range snapshotBattlefield(seat) {
		if p == nil || p.Card == nil || p.Controller != activeSeat {
			continue
		}
		if p.PhasedOut {
			continue
		}
		if p.Counters == nil {
			continue
		}
		// Vanishing — time counters (§702.63b/c).
		if p.Counters["time"] > 0 {
			p.AddCounter("time", -1)
			gs.LogEvent(Event{
				Kind:   "time_counter_removed",
				Seat:   activeSeat,
				Source: p.Card.DisplayName(),
				Amount: p.Counters["time"],
				Details: map[string]interface{}{
					"keyword": "vanishing",
					"rule":    "702.63b",
				},
			})
			// §702.63c — when the LAST time counter is removed, sacrifice.
			if p.Counters["time"] == 0 {
				sacrificePermSBA(gs, p, "vanishing", "702.63c", map[string]interface{}{
					"keyword": "vanishing",
				})
				sacrificed++
				continue
			}
		}
		// Fading — fade counters (§702.32b). "Remove a fade counter; if you
		// can't, sacrifice it." Distinct from vanishing: the sacrifice fires
		// on the upkeep where there is NO counter to remove.
		if hasFadingKeyword(p.Card) {
			if p.Counters["fade"] > 0 {
				p.AddCounter("fade", -1)
				gs.LogEvent(Event{
					Kind:   "fade_counter_removed",
					Seat:   activeSeat,
					Source: p.Card.DisplayName(),
					Amount: p.Counters["fade"],
					Details: map[string]interface{}{
						"keyword": "fading",
						"rule":    "702.32b",
					},
				})
			} else {
				sacrificePermSBA(gs, p, "fading", "702.32b", map[string]interface{}{
					"keyword": "fading",
				})
				sacrificed++
			}
		}
	}
	return sacrificed
}

// hasFadingKeyword reports whether the card carries the Fading keyword. Fade
// counters live under the "fade" key, but a fading permanent that has reached
// zero fade counters must STILL be sacrificed on its next upkeep ("if you
// can't [remove], sacrifice it") — so the fading branch cannot key off the
// counter count alone; it keys off the printed keyword.
func hasFadingKeyword(card *Card) bool {
	return cardHasKeywordByName(card, "fading")
}

// TickSuspendCounters runs the §702.62f upkeep decrement for every suspended
// card the seat controls (the cards sit in the seat's exile zone carrying the
// card.Meta["suspended"] marker, stamped by SuspendCard). When a card's last
// time counter is removed it is cast for free per §702.62g via
// CastSuspendedCard. Returns the number of cards cast this pass.
//
// The counter lives on the Card object (card.Meta), not a name-keyed
// gs.Flags string, so two same-named suspended cards count down independently.
func TickSuspendCounters(gs *GameState, activeSeat int) int {
	if gs == nil || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost {
		return 0
	}
	// Snapshot the exile slice — CastSuspendedCard mutates seat.Exile.
	snapshot := make([]*Card, len(seat.Exile))
	copy(snapshot, seat.Exile)

	cast := 0
	for _, c := range snapshot {
		if c == nil || c.Meta == nil {
			continue
		}
		if susp, _ := c.Meta["suspended"].(bool); !susp {
			continue
		}
		n, _ := c.Meta["suspend_counters"].(int)
		if n <= 0 {
			continue
		}
		n--
		c.Meta["suspend_counters"] = n
		gs.LogEvent(Event{
			Kind:   "suspend_tick",
			Seat:   activeSeat,
			Source: c.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"counters_remaining": n,
				"rule":               "702.62f",
			},
		})
		if n == 0 {
			// §702.62g — the last time counter was removed; cast for free.
			c.Meta["suspended"] = false
			if CastSuspendedCard(gs, activeSeat, c) {
				cast++
			}
		}
	}
	return cast
}

// CastSuspendedCard casts a suspended card from the seat's exile zone without
// paying its mana cost (CR §702.62g) and, if it resolves onto the battlefield
// as a creature, grants it haste "until you lose control of it" by stamping
// Flags["kw:haste"] on the resulting permanent (auto-cleared when it leaves).
// Returns true when the card was found in exile and pushed onto the stack.
//
// This mirrors the proven cascade.go free-cast block (collectSpellEffect ->
// StackItem{CastZone:"exile"} -> PushStackItem -> the full cast-time
// bookkeeping -> priority + resolve) so cast-triggers, storm, and a chained
// cascade/discover all fire exactly as a normal cast would, and so the
// exile->stack->battlefield move never duplicates the card.
func CastSuspendedCard(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	// Remove the card from exile FIRST — the cast pipeline will route it onto
	// the stack (and from there to the battlefield/graveyard); leaving it in
	// exile would duplicate the *Card across zones (ZoneConservation leak).
	idx := -1
	for i, c := range seat.Exile {
		if c == card {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	seat.Exile = append(seat.Exile[:idx], seat.Exile[idx+1:]...)

	gs.LogEvent(Event{
		Kind:   "suspend_cast_for_free",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.62g",
		},
	})

	// Mirror the cascade free-cast block (cascade.go §702.85a). The suspended
	// card is CAST, from exile, paying nothing.
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Effect:     eff,
		IsCopy:     false,
		CastZone:   "exile", // §702.62g casts the card from exile
	}
	PushStackItem(gs, item)

	// Full cast-time bookkeeping (storm / cast count / "whenever you cast"
	// triggers / its own cast keywords), mirroring CastSpell and cascade.
	IncrementCastCount(gs, seatIdx)
	RecordCast(gs, seatIdx, card, 0)
	fireCastTriggers(gs, seatIdx, card)
	if HasStormKeyword(card) {
		ApplyStormCopies(gs, item, seatIdx)
	}
	if cc := CascadeCount(card); cc > 0 {
		foundMV := manaCostOf(card)
		for i := 0; i < cc; i++ {
			ApplyCascade(gs, seatIdx, foundMV, card.DisplayName())
		}
	}
	if cardHasKeyword(card, "discover") {
		PerformDiscover(gs, seatIdx, manaCostOf(card))
	}
	InvokeCastHook(gs, item)

	// CR §117 — opponents receive priority before the suspended spell
	// resolves (it can be countered). Resolve only if it's still on top.
	PriorityRound(gs)
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		ResolveStackTop(gs)
		StateBasedActions(gs)
	}
	// Drain any triggers/responses left above it.
	DrainStack(gs)

	// §702.62g — if it resolved onto the battlefield as a creature, it gains
	// haste until its controller loses control of it. The flag lives on the
	// permanent, so it is automatically gone when the permanent leaves.
	grantSuspendHaste(gs, card)
	return true
}

// grantSuspendHaste stamps Flags["kw:haste"] on the battlefield permanent that
// is this suspended card, if it resolved into play as a creature (§702.62g).
func grantSuspendHaste(gs *GameState, card *Card) {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card != card {
				continue
			}
			if p.IsCreature() {
				if p.Flags == nil {
					p.Flags = map[string]int{}
				}
				p.Flags["kw:haste"] = 1
				gs.LogEvent(Event{
					Kind:   "suspend_haste_granted",
					Seat:   p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"rule": "702.62g",
					},
				})
			}
			return
		}
	}
}
