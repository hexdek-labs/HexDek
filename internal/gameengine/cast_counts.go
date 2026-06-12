package gameengine

// Cast-count increment hooks + cast-trigger observer dispatcher.
//
// Comp-rules citations:
//   - §700.4      — cast-count bookkeeping (implicit; "this turn" scoping)
//   - §702.40     — Storm keyword (see storm.go)
//   - §707.10     — copies of spells are not cast
//   - §601        — casting a spell (the event cast-triggers observe)
//   - §603.2/.3   — triggered abilities on-stack placement
//
// The IncrementCastCount function is called by CastSpell (stack.go) and by
// the commander-zone cast path (commander.go, owned by the Partner agent
// so we don't modify it here; commander.go callers should IncrementCastCount
// before pushing their stack item). Copies created by Storm / Twinflame /
// Dualcaster Mage MUST NOT call this.
//
// FireCastTriggerObservers is the bridge that runs the "whenever you cast…"
// style triggers for cards that don't yet have per-card handlers wired into
// the proper §603 Triggered-on-stack path. For the Tier 1 storm infra, we
// implement six observer cards inline here: Storm-Kiln Artist, Young
// Pyromancer, Third Path Iconoclast, Monastery Mentor, Runaway Steam-Kin,
// Birgi (God of Storytelling), Niv-Mizzet Parun. Each matches Python's
// _fire_cast_trigger_observers in scripts/playloop.py exactly so Go/Python
// parity holds.
//
// Long-term: these handlers should migrate to internal/gameengine/per_card/
// (owned by the per-card agent) and be dispatched via the normal
// RegisterCastTriggerObserver pipeline when that lands.

import (
	"strings"
)

// IncrementCastCount bumps the global + per-seat cast counters. Called by
// CastSpell AFTER cost has been paid and the card is en route to the stack,
// BEFORE the storm trigger is evaluated. Must NOT be called for copies
// (CR §707.10).
func IncrementCastCount(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.SpellsCastThisTurn++
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	seat.SpellsCastThisTurn++
	seat.Turn.SpellsCast++
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["spells_cast_this_turn"] = seat.Turn.SpellsCast
}

// RecordCast appends a CastRecord to the seat's TurnCounters. Called
// alongside IncrementCastCount when card metadata is available.
// Safe to call with nil card (no-ops).
func RecordCast(gs *GameState, seatIdx int, card *Card, xPaid int) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	seat.Turn.Casts = append(seat.Turn.Casts, CastRecord{
		CardName:  card.DisplayName(),
		Types:     card.Types,
		ManaValue: card.EffectiveCMC(),
		XCost:     ManaCostContainsX(card),
		XValue:    xPaid,
	})
}

// FireCastTriggerObservers fires every "whenever a spell is cast" style
// observer permanent for the cast of `cast`. `fromCopy` MUST be true when
// called from Storm copy propagation — copies are not cast (§707.10) and
// do not trigger observers.
//
// Mirror for scripts/playloop.py _fire_cast_trigger_observers.
func FireCastTriggerObservers(gs *GameState, cast *Card, controller int, fromCopy bool) {
	if gs == nil || cast == nil || fromCopy {
		return
	}
	// Derive the filters every observer reads off the cast spell. Both
	// Types and TypeLine on Card are authoritative (Types is the canonical
	// slice; TypeLine is a cache for tokens/copies that want a human-
	// readable string). We accept either so callers don't have to
	// double-populate.
	lowerTypes := map[string]bool{}
	for _, t := range cast.Types {
		lowerTypes[strings.ToLower(t)] = true
	}
	typeLine := strings.ToLower(cast.TypeLine)
	if strings.Contains(typeLine, "instant") {
		lowerTypes["instant"] = true
	}
	if strings.Contains(typeLine, "sorcery") {
		lowerTypes["sorcery"] = true
	}
	if strings.Contains(typeLine, "creature") {
		lowerTypes["creature"] = true
	}
	isInstant := lowerTypes["instant"]
	isSorcery := lowerTypes["sorcery"]
	isCreature := lowerTypes["creature"]
	isInstantOrSorcery := isInstant || isSorcery
	isNoncreature := !isCreature
	// Color — check Card.Colors (populated by corpus loader / token
	// creation). For Runaway Steam-Kin's "whenever you cast a red spell".
	isRed := false
	for _, col := range cast.Colors {
		if strings.ToUpper(col) == "R" {
			isRed = true
			break
		}
	}

	// Walk every battlefield permanent. Snapshot battlefield first so
	// observer-created tokens don't get iterated as observers themselves.
	type permNamed struct {
		perm *Permanent
		name string
	}
	var observers []permNamed
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil {
				continue
			}
			name := perm.Card.DisplayName()
			observers = append(observers, permNamed{perm: perm, name: name})
		}
	}
	castName := cast.DisplayName()

	for _, o := range observers {
		perm := o.perm
		name := o.name

		// --- Prowess keyword (CR §702.108) ---
		// "Whenever you cast a noncreature spell, this creature gets
		// +1/+1 until end of turn."
		// Check BEFORE the name switch so prowess fires on any permanent
		// that has the keyword, regardless of card name.
		if perm.Controller == controller && isNoncreature && perm.IsCreature() && perm.HasKeyword("prowess") {
			perm.Modifications = append(perm.Modifications, Modification{
				Power:     1,
				Toughness: 1,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			gs.LogEvent(Event{
				Kind:   "prowess",
				Seat:   perm.Controller,
				Source: name,
				Details: map[string]interface{}{
					"cast":   castName,
					"effect": "+1/+1 until end of turn",
					"rule":   "702.108",
				},
			})
		}

		// --- Extort keyword (CR §702.101) ---
		// "Whenever you cast a spell, you may pay {W/B}. If you do,
		// each opponent loses 1 life and you gain that much life."
		// Each instance of extort on each permanent triggers separately.
		// GreedyHat: pay if affordable (1 generic mana as MVP proxy
		// for {W/B}).
		if perm.Controller == controller && perm.HasKeyword("extort") {
			// Check if the caster can pay 1 mana for the extort trigger.
			casterSeat := gs.Seats[controller]
			if casterSeat != nil && casterSeat.ManaPool >= 1 {
				casterSeat.ManaPool -= 1
				SyncManaAfterSpend(casterSeat)
				gs.Legality.NoteManaSpend(controller, 1) // aux payment, not spell cost
				// Each opponent loses 1 life, controller gains that much.
				opps := gs.Opponents(controller)
				totalDrained := 0
				for _, oppIdx := range opps {
					opp := gs.Seats[oppIdx]
					if opp == nil {
						continue
					}
					LoseLife(gs, oppIdx, 1, name)
					totalDrained++
				}
				if totalDrained > 0 {
					GainLife(gs, controller, totalDrained, name)
					gs.LogEvent(Event{
						Kind:   "life_change",
						Seat:   controller,
						Amount: totalDrained,
						Source: name,
						Details: map[string]interface{}{
							"reason": "extort",
						},
					})
				}
				gs.LogEvent(Event{
					Kind:   "extort_trigger",
					Seat:   controller,
					Source: name,
					Amount: totalDrained,
					Details: map[string]interface{}{
						"cast":     castName,
						"drained":  totalDrained,
						"mana_paid": 1,
						"rule":     "702.101",
					},
				})
			}
		}

		// r62 Wave-1b double-dispatch audit: Storm-Kiln Artist's inline
		// arm DELETED — the per_card magecraft handler
		// (magecraft_consumers_r60.go, fired via FireMagecraftTriggers at
		// cast AND copy time) is authoritative; both firing minted two
		// Treasures per spell. Niv-Mizzet, Parun's inline draw arm
		// DELETED for the same reason (per_card instant_or_sorcery_cast
		// handler draws; both firing drew two cards per spell). See
		// /tmp/fable-review/wave1b-doubletax-audit.md for the full audit.
		switch name {
		case "Young Pyromancer":
			if perm.Controller == controller && isInstantOrSorcery {
				createSimpleCreatureToken(gs, perm.Controller,
					"Elemental Token", 1, 1, []string{"R"})
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "elemental_token",
					},
				})
			}
		case "Third Path Iconoclast":
			if perm.Controller == controller && isNoncreature {
				createSimpleCreatureToken(gs, perm.Controller,
					"Soldier Artifact Token", 1, 1, nil)
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "soldier_token",
					},
				})
			}
		case "Monastery Mentor":
			if perm.Controller == controller && isNoncreature {
				createSimpleCreatureToken(gs, perm.Controller,
					"Monk Token", 1, 1, []string{"W"})
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "monk_token",
					},
				})
			}
		case "Runaway Steam-Kin":
			if perm.Controller == controller && isRed {
				cur := 0
				if perm.Counters != nil {
					cur = perm.Counters["+1/+1"]
				}
				if cur < 3 {
					perm.AddCounter("+1/+1", 1)
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":   castName,
							"effect": "plus_one_counter",
						},
					})
				}
			}
		case "Birgi, God of Storytelling":
			if perm.Controller == controller {
				// Route through the canonical AddMana chokepoint (legality-
				// validator r62 finding #4): the pre-r62 direct ManaPool++
				// skipped the add_mana event stream and would skip any
				// future mana-replacement effects. AddMana also credits the
				// validator's in-window observation (this add fires INSIDE
				// the cast announcement window — FireCastTriggerObservers
				// runs mid-CastSpell), so the old direct NoteManaAdd
				// stopgap is gone with it. {R} per the printed ability —
				// the red bucket drains after Any in generic spends, so
				// spendability is unchanged.
				AddMana(gs, gs.Seats[perm.Controller], "R", 1, "Birgi, God of Storytelling")
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Amount: 1,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "add_mana_R",
					},
				})
			}
		}
	}
}

// createTreasureToken drops a Treasure artifact token onto the battlefield
// under `seatIdx`'s control. MVP: the token is just a marker; a full
// Treasure mana-ability wiring is a separate task (the existing mana pool
// MVP doesn't distinguish between tapped and untapped mana sources, and
// Treasure's {T}, sacrifice: add one mana of any color plays through the
// normal tap-mana scanner once that lands).
func createTreasureToken(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	token := &Card{
		Name:  "Treasure Token",
		Owner: seatIdx,
		Types: []string{"token", "artifact", "treasure"},
	}
	MintTokenInstanceID(gs, token, "", currentMintEnablerID(gs))
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		Tapped:        false,
		SummoningSick: false,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	seat.Turn.TokensCreated++
	seat.Turn.ArtifactsEntered++
	seat.Turn.TreasuresCreated++
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: "Treasure Token",
		Details: map[string]interface{}{
			"token": "Treasure Token",
		},
	})
}

// FireDrawTriggerObservers fires the "player_would_draw" fan-out for
// effect draws (resolve.go). Per-card "whenever a player draws" handlers
// (Smothering Tithe, Orcish Bowmasters, Consecrated Sphinx, Nekusar, …)
// do NOT live here — they listen on "card_drawn", fired from the drawOne
// chokepoint in state.go for every draw, where the CR §614.6
// first-draw-step marker (gs.Flags["_suppress_first_draw_trigger_seat"])
// is also consumed and surfaced as ctx["is_draw_step_draw"].
func FireDrawTriggerObservers(gs *GameState, drawerSeat int, count int, fromReplacement bool) {
	if gs == nil || drawerSeat < 0 || drawerSeat >= len(gs.Seats) {
		return
	}
	if count <= 0 {
		count = 1
	}
	// r62 Wave-1b double-dispatch audit: the per-draw inline switch
	// (Smothering Tithe tax/Treasure + Orcish Bowmasters archer/ping)
	// is DELETED. Both cards have per_card handlers on "card_drawn",
	// which fires from the drawOne chokepoint for EVERY draw — this
	// fan-out only ran for resolve.go effect draws, so those draws
	// dispatched BOTH implementations (two Treasures / two pings per
	// effect draw). The per_card handlers are authoritative. The
	// first-draw-step suppression marker is now consumed at the
	// drawOne chokepoint (state.go) where card_drawn fires, instead of
	// here — pre-r62 it was consumed on a path turn-step draws never
	// took, so it leaked onto the NEXT effect draw.
	//
	// What remains is the "player_would_draw" fan-out for per_card
	// draw-replacement handlers (Chains of Mephistopheles class) and
	// Niv-Mizzet's damage-on-draw listener.
	FireCardTrigger(gs, "player_would_draw", map[string]interface{}{
		"draw_seat":        drawerSeat,
		"count":            count,
		"from_replacement": fromReplacement,
		// Effect draws are never turn-draw-step draws.
		"is_draw_step_draw": false,
	})
}

// createSimpleCreatureToken drops a vanilla creature token onto the
// battlefield. Used by Young Pyromancer / Monastery Mentor / Third Path
// Iconoclast cast-trigger observers.
func createSimpleCreatureToken(gs *GameState, seatIdx int, name string,
	power, toughness int, colors []string) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	types := []string{"token", "creature"}
	token := &Card{
		Name:          name,
		Owner:         seatIdx,
		BasePower:     power,
		BaseToughness: toughness,
		Types:         types,
	}
	_ = colors // out-of-scope: preserving original behavior; Mint stamps "C"
	MintTokenInstanceID(gs, token, "", currentMintEnablerID(gs))
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		Tapped:        false,
		SummoningSick: true, // §302.1 — creatures enter with summoning sickness
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: name,
		Amount: power,
		Details: map[string]interface{}{
			"token":     name,
			"power":     power,
			"toughness": toughness,
			"colors":    colors,
		},
	})
}
