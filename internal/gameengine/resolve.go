package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ResolveEffect is the main entry point. It dispatches on effect.Kind()
// and mutates gs in place, emitting structured events to gs.EventLog.
//
// Contract:
//   - nil effects are a no-op (the parser sometimes emits nil for
//     unstructured text; the resolver shouldn't crash on them)
//   - handlers may recurse (Sequence -> Damage + Draw)
//   - handlers never return an error; log-and-continue is the philosophy
//
// The switch uses effect.Kind() — a string — to dispatch. Go could type-
// switch on *Damage/*Draw/... but the string switch compiles to the same
// machine-code lookup table and is easier to extend when a new kind is
// added to gameast.
func ResolveEffect(gs *GameState, src *Permanent, effect gameast.Effect) {
	if gs == nil || effect == nil {
		return
	}
	switch e := effect.(type) {
	// ---- Control flow ----
	case *gameast.Sequence:
		resolveSequence(gs, src, e)
	case *gameast.Choice:
		resolveChoice(gs, src, e)
	case *gameast.Optional_:
		if e.Body != nil {
			ResolveEffect(gs, src, e.Body)
		}
	case *gameast.Conditional:
		resolveConditional(gs, src, e)

	// ---- High-volume leaves first (hot path) ----
	case *gameast.Damage:
		resolveDamage(gs, src, e)
	case *gameast.Draw:
		resolveDraw(gs, src, e)
	case *gameast.Discard:
		resolveDiscard(gs, src, e)
	case *gameast.Mill:
		resolveMill(gs, src, e)
	case *gameast.GainLife:
		resolveGainLife(gs, src, e)
	case *gameast.LoseLife:
		resolveLoseLife(gs, src, e)
	case *gameast.SetLife:
		resolveSetLife(gs, src, e)

	// ---- Battlefield ops ----
	case *gameast.Destroy:
		resolveDestroy(gs, src, e)
	case *gameast.Exile:
		resolveExile(gs, src, e)
	case *gameast.Bounce:
		resolveBounce(gs, src, e)
	case *gameast.TapEffect:
		resolveTap(gs, src, e)
	case *gameast.UntapEffect:
		resolveUntap(gs, src, e)
	case *gameast.GainControl:
		resolveGainControl(gs, src, e)
	case *gameast.Sacrifice:
		resolveSacrifice(gs, src, e)
	case *gameast.Fight:
		resolveFight(gs, src, e)

	// ---- Tokens / buffs / counters ----
	case *gameast.CreateToken:
		resolveCreateToken(gs, src, e)
	case *gameast.Buff:
		resolveBuff(gs, src, e)
	case *gameast.GrantAbility:
		resolveGrantAbility(gs, src, e)
	case *gameast.CounterMod:
		resolveCounterMod(gs, src, e)

	// ---- Mana / library ops ----
	case *gameast.AddMana:
		resolveAddMana(gs, src, e)
	case *gameast.Tutor:
		resolveTutor(gs, src, e)
	case *gameast.Reanimate:
		resolveReanimate(gs, src, e)
	case *gameast.Recurse:
		resolveRecurse(gs, src, e)
	case *gameast.Shuffle:
		resolveShuffle(gs, src, e)

	// ---- Stack / timing ----
	case *gameast.CounterSpell:
		resolveCounterSpell(gs, src, e)
	case *gameast.CopySpell:
		resolveCopySpell(gs, src, e)
	case *gameast.CopyPermanent:
		resolveCopyPermanent(gs, src, e)
	case *gameast.ExtraTurn:
		resolveExtraTurn(gs, src, e)
	case *gameast.ExtraCombat:
		resolveExtraCombat(gs, src, e)

	// ---- Info / reveals (log-only in MVP) ----
	case *gameast.Scry:
		resolveScry(gs, src, e)
	case *gameast.Surveil:
		resolveSurveil(gs, src, e)
	case *gameast.LookAt:
		resolveLookAt(gs, src, e)
	case *gameast.Reveal:
		resolveReveal(gs, src, e)

	// ---- Win/lose ----
	case *gameast.WinGame:
		resolveWinGame(gs, src, e)
	case *gameast.LoseGame:
		resolveLoseGame(gs, src, e)

	// ---- Modification-as-effect (Wave 1a promoted labels) ----
	case *gameast.ModificationEffect:
		resolveModificationEffect(gs, src, e)

	// ---- Stubs for later phases ----
	case *gameast.Replacement:
		// §614 framework is Phase 7. Log that we saw one so later-phase
		// agents have a paper trail.
		gs.LogEvent(Event{
			Kind:   "replacement_seen",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"trigger_event": e.TriggerEvent,
			},
		})
	case *gameast.Prevent:
		// "Prevent all combat damage" (fog effects) or "prevent the next N
		// damage" to a specific source/target.
		amt, _ := evalNumber(gs, src, &e.Amount)
		isAll := (e.Amount.IsStr && e.Amount.Str == "all") || e.Duration == "until_end_of_turn"
		if amt == 0 && isAll {
			// Global fog: prevent ALL combat damage this turn. ~40 cards
			// (Fog, Moment's Peace, Clinging Mists, etc.)
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags["prevent_all_combat_damage"] = 1
		} else if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			if amt <= 0 {
				amt = 1
			}
			src.Flags["prevent_damage"] += amt
		}
		gs.LogEvent(Event{Kind: "prevent", Source: sourceName(src),
			Details: map[string]interface{}{"amount": amt, "global": isAll}})
	case *gameast.UnknownEffect:
		gs.LogEvent(Event{
			Kind:   "unknown_effect",
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": e.RawText,
			},
		})
	default:
		// A kind we haven't wired. Log it so the dev who added a new
		// gameast type gets an obvious signal in tests.
		gs.LogEvent(Event{
			Kind:   "unhandled_effect",
			Source: sourceName(src),
			Details: map[string]interface{}{
				"kind": effect.Kind(),
			},
		})
	}
}

// -----------------------------------------------------------------------------
// Control flow
// -----------------------------------------------------------------------------

func resolveSequence(gs *GameState, src *Permanent, e *gameast.Sequence) {
	for _, item := range e.Items {
		ResolveEffect(gs, src, item)
	}
}

func resolveChoice(gs *GameState, src *Permanent, e *gameast.Choice) {
	if len(e.Options) == 0 {
		return
	}
	pick, _ := evalNumber(gs, src, &e.Pick)
	if pick < 1 {
		pick = 1
	}
	if pick > len(e.Options) {
		pick = len(e.Options)
	}

	// Delegate mode selection to Hat if available (§601.2c).
	seatIdx := 0
	if src != nil {
		seatIdx = src.Controller
	}
	if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil && gs.Seats[seatIdx].Hat != nil {
		chosen := gs.Seats[seatIdx].Hat.ChooseMode(gs, seatIdx, e.Options)
		if chosen >= 0 && chosen < len(e.Options) {
			ResolveEffect(gs, src, e.Options[chosen])
			return
		}
	}

	// Fallback: resolve the first N options in order.
	for i := 0; i < pick; i++ {
		ResolveEffect(gs, src, e.Options[i])
	}
}

func resolveConditional(gs *GameState, src *Permanent, e *gameast.Conditional) {
	if evalCondition(gs, src, e.Condition) {
		if e.Body != nil {
			ResolveEffect(gs, src, e.Body)
		}
	} else if e.ElseBody != nil {
		ResolveEffect(gs, src, e.ElseBody)
	}
}

// evalCondition evaluates a boolean condition. MVP supports the common
// kinds the parser emits today; unknown kinds default to true (so the
// conditional fires — conservative for "if X, do something good" cases).
func evalCondition(gs *GameState, src *Permanent, c *gameast.Condition) bool {
	if c == nil {
		return true
	}
	switch c.Kind {
	case "life_threshold":
		// args = (op, n). MVP: check source controller's life.
		if src == nil || len(c.Args) < 2 {
			return true
		}
		if src.Controller < 0 || src.Controller >= len(gs.Seats) {
			return true
		}
		op, _ := c.Args[0].(string)
		n, _ := asInt(c.Args[1])
		life := gs.Seats[src.Controller].Life
		return compareInt(life, op, n)
	case "you_control":
		if src == nil {
			return false
		}
		filter := ""
		if len(c.Args) > 0 {
			filter, _ = c.Args[0].(string)
		}
		if filter == "" {
			return len(gs.Seats[src.Controller].Battlefield) > 0
		}
		return countControlledByType(gs, src.Controller, filter) > 0

	case "you_dont_control":
		if src == nil {
			return true
		}
		filter := ""
		if len(c.Args) > 0 {
			filter, _ = c.Args[0].(string)
		}
		if filter == "" {
			return len(gs.Seats[src.Controller].Battlefield) == 0
		}
		return countControlledByType(gs, src.Controller, filter) == 0

	case "card_count_zone":
		if len(c.Args) < 3 {
			return true
		}
		zone, _ := c.Args[0].(string)
		op, _ := c.Args[1].(string)
		n, _ := asInt(c.Args[2])
		count := countCardsInZone(gs, src, zone, "you")
		return compareInt(count, op, n)

	case "you_attacked_this_turn":
		if src == nil {
			return false
		}
		return CheckRaid(gs, src.Controller)

	case "kicked":
		// Kicker is tracked as a flag on the permanent.
		if src == nil {
			return false
		}
		kickCount, _ := src.Flags["kicked"]
		needed := 1
		if len(c.Args) > 0 {
			needed, _ = asInt(c.Args[0])
		}
		return kickCount >= needed

	case "descended_this_turn":
		if src == nil {
			return false
		}
		desc, _ := gs.Flags["descended_"+strconv.Itoa(src.Controller)]
		return desc > 0

	case "opponent_discarded_this_turn":
		if src == nil {
			return false
		}
		for i := range gs.Seats {
			if i == src.Controller {
				continue
			}
			key := "discarded_" + strconv.Itoa(i)
			if d, ok := gs.Flags[key]; ok && d > 0 {
				return true
			}
		}
		return false

	case "first_combat_phase":
		if gs.Flags == nil {
			return true
		}
		combats, _ := gs.Flags["combat_phases_this_turn"]
		return combats <= 1

	case "not_your_turn":
		if src == nil {
			return true
		}
		return gs.Active != src.Controller

	case "your_turn":
		if src == nil {
			return false
		}
		return gs.Active == src.Controller

	case "counter_on_self":
		if src == nil || len(c.Args) < 3 {
			return true
		}
		kind, _ := c.Args[0].(string)
		op, _ := c.Args[1].(string)
		n, _ := asInt(c.Args[2])
		count := 0
		if src.Counters != nil {
			count = src.Counters[kind]
		}
		return compareInt(count, op, n)

	case "self_is_type":
		if src == nil || len(c.Args) < 1 {
			return true
		}
		typeName, _ := c.Args[0].(string)
		return src.hasType(typeName) || (src.Card != nil && cardHasType(src.Card, typeName))

	case "permanent_left_this_turn":
		if gs.Flags == nil {
			return false
		}
		key := "permanent_left_bf"
		if src != nil {
			key = "permanent_left_bf_" + strconv.Itoa(src.Controller)
		}
		left, _ := gs.Flags[key]
		return left > 0

	case "mana_spent":
		// MVP: default true (mana tracking not fully implemented)
		return true

	case "raw":
		// Unknown condition — default true (conservative)
		return true
	}
	// Default-true: unknown conditions are assumed satisfied.
	return true
}

func compareInt(a int, op string, b int) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "==", "=":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

// countControlledByType counts permanents a player controls matching a type filter.
var colorNames = map[string]string{
	"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}

func countControlledByType(gs *GameState, seat int, filter string) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	count := 0
	// Extract color prefix if present (e.g. "green permanent" → color="G", remainder="permanent").
	var requireColor string
	for cName, cCode := range colorNames {
		if strings.HasPrefix(filter, cName+" ") {
			requireColor = cCode
			filter = strings.TrimSpace(filter[len(cName)+1:])
			break
		}
	}
	// Strip trailing junk after comma (parser artifact).
	if idx := strings.Index(filter, ","); idx > 0 {
		filter = strings.TrimSpace(filter[:idx])
	}
	// Extract "with power N or greater" suffix.
	minPower := -1
	if idx := strings.Index(filter, " with power "); idx > 0 {
		rest := filter[idx+12:]
		filter = strings.TrimSpace(filter[:idx])
		n, _ := strconv.Atoi(strings.Fields(rest)[0])
		if n > 0 {
			minPower = n
		}
	}
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if requireColor != "" {
			hasColor := false
			for _, c := range p.Card.Colors {
				if c == requireColor {
					hasColor = true
					break
				}
			}
			if !hasColor {
				continue
			}
		}
		if filter == "" || filter == "permanent" || p.hasType(filter) || cardHasType(p.Card, filter) || cardHasSubtype(p.Card, filter) {
			if minPower >= 0 && p.Card.BasePower < minPower {
				continue
			}
			count++
		}
	}
	return count
}

func CountControlledByTypeExported(gs *GameState, seat int, filter string) int {
	return countControlledByType(gs, seat, filter)
}

// -----------------------------------------------------------------------------
// Leaf handlers
// -----------------------------------------------------------------------------

func resolveDamage(gs *GameState, src *Permanent, e *gameast.Damage) {
	amount, _ := evalNumber(gs, src, &e.Amount)
	if amount <= 0 {
		return
	}
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, deal damage to an opponent (the most
	// common case for unresolved damage filters like "target creature or player").
	if len(targets) == 0 && src != nil {
		opps := gs.Opponents(src.Controller)
		if len(opps) > 0 {
			targets = []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
		}
	}
	if e.Divided && len(targets) > 1 {
		// CR §601.2d: "distribute N damage among any number of targets".
		// The caster divides the total damage among the targets at cast
		// time. MVP heuristic: distribute as evenly as possible, with
		// remainder going to the first target(s). A Hat-driven chooser
		// can override this via ChooseDamageDistribution in the future.
		distributeDamage(gs, src, targets, amount)
	} else {
		for _, t := range targets {
			applyDamage(gs, src, t, amount)
		}
	}
}

// distributeDamage divides `total` damage among `targets` as evenly as
// possible. CR §601.2d requires each target to receive at least 1 damage
// when damage is being distributed. Remainder after equal division goes
// to the first targets (heuristic: prioritize player targets over
// permanent targets since reducing life totals is typically more
// impactful for game outcome).
func distributeDamage(gs *GameState, src *Permanent, targets []Target, total int) {
	n := len(targets)
	if n == 0 || total <= 0 {
		return
	}
	// CR §601.2d: each chosen target must receive at least 1 damage.
	// If total < number of targets, only assign to the first `total` targets.
	if total < n {
		for i := 0; i < total; i++ {
			applyDamage(gs, src, targets[i], 1)
		}
		gs.LogEvent(Event{
			Kind:   "damage_distributed",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: total,
			Details: map[string]interface{}{
				"targets_hit": total,
				"targets_total": n,
				"rule":          "601.2d",
			},
		})
		return
	}
	base := total / n
	remainder := total % n
	// Hat-driven distribution: ask the controller's Hat if it implements
	// the optional DamageDistributor interface.
	controller := controllerSeat(src)
	if controller >= 0 && controller < len(gs.Seats) {
		seat := gs.Seats[controller]
		if dd, ok := seat.Hat.(DamageDistributor); seat != nil && seat.Hat != nil && ok {
			amounts := dd.ChooseDamageDistribution(gs, controller, targets, total)
			if len(amounts) == n {
				// Validate: sum must equal total, each >= 1.
				sum := 0
				valid := true
				for _, a := range amounts {
					if a < 1 {
						valid = false
						break
					}
					sum += a
				}
				if valid && sum == total {
					for i, t := range targets {
						applyDamage(gs, src, t, amounts[i])
					}
					gs.LogEvent(Event{
						Kind:   "damage_distributed",
						Seat:   controller,
						Source: sourceName(src),
						Amount: total,
						Details: map[string]interface{}{
							"distribution": amounts,
							"rule":         "601.2d",
						},
					})
					return
				}
			}
		}
	}
	// Default heuristic: even split with remainder to first targets.
	for i, t := range targets {
		dmg := base
		if i < remainder {
			dmg++
		}
		applyDamage(gs, src, t, dmg)
	}
	gs.LogEvent(Event{
		Kind:   "damage_distributed",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: total,
		Details: map[string]interface{}{
			"targets": n,
			"base":    base,
			"rule":    "601.2d",
		},
	})
}

func applyDamage(gs *GameState, src *Permanent, t Target, amount int) {
	switch t.Kind {
	case TargetKindSeat:
		if t.Seat < 0 || t.Seat >= len(gs.Seats) {
			return
		}
		// §614: fire would_be_dealt_damage replacement chain.
		modified, cancelled := FireDamageEvent(gs, src, t.Seat, nil, amount)
		if cancelled || modified <= 0 {
			return
		}
		// §615: apply prevention shields before dealing damage.
		modified = PreventDamageToPlayer(gs, t.Seat, modified, src)
		if modified <= 0 {
			return
		}
		gs.Seats[t.Seat].Life -= modified
		// Set damage_taken_this_turn flag so Bloodthirst (§702.54) and
		// similar mechanics can detect non-combat damage to this player.
		if gs.Seats[t.Seat].Flags == nil {
			gs.Seats[t.Seat].Flags = map[string]int{}
		}
		gs.Seats[t.Seat].Flags["damage_taken_this_turn"] = 1
		gs.LogEvent(Event{
			Kind:   "damage",
			Seat:   controllerSeat(src),
			Target: t.Seat,
			Source: sourceName(src),
			Amount: modified,
		})
		gs.LogEvent(Event{
			Kind:   "life_change",
			Seat:   t.Seat,
			Source: sourceName(src),
			Amount: -modified,
			Details: map[string]interface{}{
				"from": gs.Seats[t.Seat].Life + modified,
				"to":   gs.Seats[t.Seat].Life,
			},
		})
	case TargetKindPermanent:
		if t.Permanent == nil {
			return
		}
		modified, cancelled := FireDamageEvent(gs, src, t.Permanent.Controller, t.Permanent, amount)
		if cancelled || modified <= 0 {
			return
		}
		// §615: apply prevention shields before dealing damage.
		modified = PreventDamageToPermanent(gs, t.Permanent, modified, src)
		if modified <= 0 {
			return
		}
		t.Permanent.MarkedDamage += modified
		gs.LogEvent(Event{
			Kind:   "damage",
			Seat:   controllerSeat(src),
			Target: t.Permanent.Controller,
			Source: sourceName(src),
			Amount: modified,
			Details: map[string]interface{}{
				"target_card": t.Permanent.Card.DisplayName(),
			},
		})
	}
}

func resolveDraw(gs *GameState, src *Permanent, e *gameast.Draw) {
	count, _ := evalNumber(gs, src, &e.Count)
	if count <= 0 {
		count = 1
	}
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 {
		// Default to source's controller.
		if src != nil {
			targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
		} else {
			targets = []Target{{Kind: TargetKindSeat, Seat: 0}}
		}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		// §614.11: replacement effects fire PER draw-card event. A doubler
		// (Alhammarret's Archive) turns each would_draw into N draws; an
		// alt-win (Laboratory Maniac) cancels the draw and sets Won.
		drawn := 0
		for i := 0; i < count; i++ {
			if NarsetBlocksDraw(gs, seat) {
				break
			}
			modifiedCount, cancelled := FireDrawEvent(gs, seat, src)
			if cancelled {
				continue
			}
			for k := 0; k < modifiedCount; k++ {
				if _, ok := gs.drawOne(seat); ok {
					drawn++
					IncrementDrawCount(gs, seat)
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "draw",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: drawn,
		})
		// Fire draw-trigger observers (Smothering Tithe, Orcish Bowmasters)
		// for each card drawn. Mirrors Python _fire_draw_trigger_observers.
		if drawn > 0 {
			FireDrawTriggerObservers(gs, seat, drawn, false)
		}
	}
}

func resolveDiscard(gs *GameState, src *Permanent, e *gameast.Discard) {
	count, _ := evalNumber(gs, src, &e.Count)
	if count <= 0 {
		count = 1
	}
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, discard targets an opponent.
	if len(targets) == 0 && src != nil {
		opps := gs.Opponents(src.Controller)
		if len(opps) > 0 {
			targets = []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
		}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		discarded := discardN(gs, seat, count, e.ChosenBy)
		gs.LogEvent(Event{
			Kind:   "discard",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: discarded,
		})
	}
}

// DiscardCard moves a specific card from hand to graveyard and fires the
// card_discarded trigger. All discard paths should route through this to
// ensure Liliana's Caress, Waste Not, Tergrid, etc. see every discard.
func DiscardCard(gs *GameState, card *Card, seat int) {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	dest := "graveyard"
	if NecropotenceSkipsDraw(gs, seat) {
		dest = "exile"
	}
	MoveCard(gs, card, seat, "hand", dest, "discard")
	FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"card":           card,
		"card_name":      card.DisplayName(),
		"discarder_seat": seat,
		"is_permanent":   cardIsPermanentType(card),
		"exiled":         dest == "exile",
	})
}

func DiscardN(gs *GameState, seat, n int, policy string) int {
	return discardN(gs, seat, n, policy)
}

func discardN(gs *GameState, seat, n int, policy string) int {
	if seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	moved := 0
	for i := 0; i < n && len(s.Hand) > 0; i++ {
		idx := 0
		if policy == "random" && gs.Rng != nil && len(s.Hand) > 1 {
			idx = gs.Rng.Intn(len(s.Hand))
		}
		c := s.Hand[idx]
		DiscardCard(gs, c, seat)
		moved++
	}
	return moved
}

func resolveMill(gs *GameState, src *Permanent, e *gameast.Mill) {
	count, _ := evalNumber(gs, src, &e.Count)
	if count <= 0 {
		count = 1
	}
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, mill an opponent.
	if len(targets) == 0 && src != nil {
		opps := gs.Opponents(src.Controller)
		if len(opps) > 0 {
			targets = []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
		}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		milled := 0
		for i := 0; i < count; i++ {
			if _, ok := gs.millOne(seat); ok {
				milled++
			}
		}
		gs.LogEvent(Event{
			Kind:   "mill",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: milled,
		})
	}
}

func resolveGainLife(gs *GameState, src *Permanent, e *gameast.GainLife) {
	amount, _ := evalNumber(gs, src, &e.Amount)
	if amount <= 0 {
		return
	}
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 {
		// Default to source's controller.
		if src != nil {
			targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
		}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		// §614: would_gain_life replacement chain — doublers (Alhammarret's
		// Archive, Boon Reflection, Rhox Faithmender) multiply here.
		modified, cancelled := FireGainLifeEvent(gs, seat, amount, src)
		if cancelled || modified <= 0 {
			continue
		}
		GainLife(gs, seat, modified, sourceName(src))
		gs.LogEvent(Event{
			Kind:   "gain_life",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: modified,
		})
	}
}

func resolveLoseLife(gs *GameState, src *Permanent, e *gameast.LoseLife) {
	amount, _ := evalNumber(gs, src, &e.Amount)
	if amount <= 0 {
		return
	}
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 && src != nil {
		// Default to opponent — "lose life" effects typically target opponents.
		opps := gs.Opponents(src.Controller)
		if len(opps) > 0 {
			targets = []Target{{Kind: TargetKindSeat, Seat: opps[0]}}
		} else {
			targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
		}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		// §614: would_lose_life replacement chain.
		modified, cancelled := FireLoseLifeEvent(gs, seat, amount, src)
		if cancelled || modified <= 0 {
			continue
		}
		beforeLife := gs.Seats[seat].Life
		gs.Seats[seat].Life -= modified
		gs.LogEvent(Event{
			Kind:   "lose_life",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: modified,
		})
		gs.LogEvent(Event{
			Kind:   "life_change",
			Seat:   seat,
			Source: sourceName(src),
			Amount: -modified,
			Details: map[string]interface{}{
				"from": beforeLife,
				"to":   gs.Seats[seat].Life,
			},
		})
	}
}

func resolveSetLife(gs *GameState, src *Permanent, e *gameast.SetLife) {
	amount, _ := evalNumber(gs, src, &e.Amount)
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		prevLife := gs.Seats[seat].Life
		gs.Seats[seat].Life = amount
		gs.LogEvent(Event{
			Kind:   "set_life",
			Seat:   controllerSeat(src),
			Target: seat,
			Source: sourceName(src),
			Amount: amount,
		})
		gs.LogEvent(Event{
			Kind:   "life_change",
			Seat:   seat,
			Source: sourceName(src),
			Amount: amount - prevLife,
			Details: map[string]interface{}{
				"from": prevLife,
				"to":   amount,
			},
		})
	}
}

// -----------------------------------------------------------------------------
// Battlefield operations
// -----------------------------------------------------------------------------

func resolveDestroy(gs *GameState, src *Permanent, e *gameast.Destroy) {
	targets := PickTarget(gs, src, e.Target)
	for _, t := range targets {
		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		// Route through DestroyPermanent which checks indestructible,
		// runs §614 replacement effects, and fires dies/LTB triggers.
		DestroyPermanent(gs, t.Permanent, src)
	}
}

func resolveExile(gs *GameState, src *Permanent, e *gameast.Exile) {
	// Check if this is targeting a spell on the stack (e.g. Ashiok's Erasure).
	base := normalizeBase(e.Target.Base)
	if base == "spell" || base == "instant_or_sorcery_spell" {
		if exileStackSpell(gs, src, e.Target) {
			return
		}
	}

	// Check if this is targeting a card in a graveyard (instants/sorceries
	// can't be permanents on the battlefield).
	if isGraveyardBounceFilter(base) {
		if exileFromGraveyard(gs, src, e) {
			return
		}
	}

	targets := PickTarget(gs, src, e.Target)
	for _, t := range targets {
		switch t.Kind {
		case TargetKindPermanent:
			if t.Permanent == nil {
				continue
			}
			// Route through ExilePermanent which runs §614 replacement
			// effects, commander redirect, and fires LTB triggers.
			ExilePermanent(gs, t.Permanent, src)
		case TargetKindSeat:
			// Player-targeted exile (e.g. Bojuka Bog): exile all cards in
			// target player's graveyard.
			seat := t.Seat
			if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
				continue
			}
			if len(gs.Seats[seat].Graveyard) == 0 {
				continue
			}
			cards := make([]*Card, len(gs.Seats[seat].Graveyard))
			copy(cards, gs.Seats[seat].Graveyard)
			for _, c := range cards {
				MoveCard(gs, c, seat, "graveyard", "exile", "exile-graveyard")
			}
			gs.LogEvent(Event{
				Kind:   "exile",
				Seat:   controllerSeat(src),
				Target: seat,
				Source: sourceName(src),
				Amount: len(cards),
				Details: map[string]interface{}{
					"zone":   "graveyard",
					"reason": "exile_graveyard",
				},
			})
		}
	}
}

func resolveBounce(gs *GameState, src *Permanent, e *gameast.Bounce) {
	// Check if this is targeting a spell on the stack (e.g. Unsubstantiate).
	base := normalizeBase(e.Target.Base)
	if base == "spell" || base == "instant_or_sorcery_spell" {
		// Try to bounce a spell from the stack back to owner's hand.
		if bounceStackSpell(gs, src, e.Target) {
			return
		}
	}

	// Check if this is targeting a card in a graveyard (e.g. Archaeomancer:
	// "return instant or sorcery card from your graveyard to your hand").
	// Instants and sorceries can't be on the battlefield as permanents, so
	// when the filter base is instant/sorcery, search the graveyard instead.
	if isGraveyardBounceFilter(base) {
		if bounceFromGraveyard(gs, src, e) {
			return
		}
	}

	targets := PickTarget(gs, src, e.Target)
	for _, t := range targets {
		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		dest := "hand"
		switch e.To {
		case "top_of_library":
			dest = "library_top"
		case "bottom_of_library":
			dest = "library_bottom"
		}
		// Route through BouncePermanent which runs commander redirect
		// (§903.9b) and fires LTB triggers.
		BouncePermanent(gs, t.Permanent, src, dest)
	}
}

// isGraveyardBounceFilter returns true if the filter base refers to a card
// type that can only exist in a graveyard (not on the battlefield).
func isGraveyardBounceFilter(base string) bool {
	switch base {
	case "instant", "sorcery", "instant or sorcery", "instant_or_sorcery":
		return true
	}
	return false
}

// bounceFromGraveyard finds a matching card in a graveyard and moves it to
// the controller's hand (or other destination). Used for "return instant/sorcery
// card from your graveyard to your hand" effects.
func bounceFromGraveyard(gs *GameState, src *Permanent, e *gameast.Bounce) bool {
	if src == nil {
		return false
	}
	seat := src.Controller

	// Determine which graveyard to search.
	searchSeat := seat // Default: own graveyard.
	if e.Target.OpponentControls {
		opps := gs.Opponents(seat)
		if len(opps) == 0 {
			return false
		}
		searchSeat = opps[0]
	}

	// Find a matching card.
	gy := gs.Seats[searchSeat].Graveyard
	idx := -1
	for i, c := range gy {
		if cardMatchesFilter(c, e.Target) {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}

	c := gy[idx]

	dest := "hand"
	switch e.To {
	case "top_of_library":
		dest = "library_top"
	case "bottom_of_library":
		dest = "library_bottom"
	}
	MoveCard(gs, c, searchSeat, "graveyard", dest, "bounce-from-gy")
	return true
}

// bounceStackSpell removes a matching spell from the stack and puts its card
// into the owner's hand. Returns true if a spell was bounced.
func bounceStackSpell(gs *GameState, src *Permanent, filter gameast.Filter) bool {
	srcSeat := controllerSeat(src)
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Countered || si.Card == nil {
			continue
		}
		// Don't bounce your own spells (MVP heuristic).
		if si.Controller == srcSeat {
			continue
		}
		if !isStackItemSpell(si) {
			continue
		}
		// Bounce: remove from stack, put card in owner's hand.
		owner := si.Card.Owner
		gs.Stack = append(gs.Stack[:i], gs.Stack[i+1:]...)
		MoveCard(gs, si.Card, owner, "stack", "hand", "bounce-from-stack")
		gs.LogEvent(Event{
			Kind:   "bounce",
			Seat:   srcSeat,
			Target: owner,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"target_card": si.Card.DisplayName(),
				"from":        "stack",
			},
		})
		return true
	}
	return false
}

// exileFromGraveyard finds a matching card in a graveyard and exiles it.
// Used for effects that exile instant/sorcery cards from graveyards.
func exileFromGraveyard(gs *GameState, src *Permanent, e *gameast.Exile) bool {
	if src == nil {
		return false
	}
	seat := src.Controller

	// Search own graveyard first, then opponents'.
	searchSeats := []int{seat}
	if e.Target.OpponentControls {
		searchSeats = gs.Opponents(seat)
	} else {
		// Also check opponent graveyards as a fallback.
		searchSeats = append(searchSeats, gs.Opponents(seat)...)
	}

	for _, searchSeat := range searchSeats {
		gy := gs.Seats[searchSeat].Graveyard
		for _, c := range gy {
			if cardMatchesFilter(c, e.Target) {
				MoveCard(gs, c, searchSeat, "graveyard", "exile", "exile-from-gy")
				return true
			}
		}
	}
	return false
}

// exileStackSpell removes a matching spell from the stack and exiles its card.
// Returns true if a spell was exiled.
func exileStackSpell(gs *GameState, src *Permanent, filter gameast.Filter) bool {
	srcSeat := controllerSeat(src)
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Countered || si.Card == nil {
			continue
		}
		if si.Controller == srcSeat {
			continue
		}
		if !isStackItemSpell(si) {
			continue
		}
		// Exile: remove from stack, put card in exile.
		owner := si.Card.Owner
		gs.Stack = append(gs.Stack[:i], gs.Stack[i+1:]...)
		MoveCard(gs, si.Card, owner, "stack", "exile", "exile-from-stack")
		gs.LogEvent(Event{
			Kind:   "exile",
			Seat:   srcSeat,
			Target: owner,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"target_card": si.Card.DisplayName(),
				"from":        "stack",
			},
		})
		return true
	}
	return false
}

func resolveTap(gs *GameState, src *Permanent, e *gameast.TapEffect) {
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, tap the source permanent.
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	for _, t := range targets {
		if t.Kind == TargetKindPermanent && t.Permanent != nil {
			t.Permanent.Tapped = true
			gs.LogEvent(Event{Kind: "tap", Source: sourceName(src)})
		}
	}
}

func resolveUntap(gs *GameState, src *Permanent, e *gameast.UntapEffect) {
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, untap the source permanent.
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	for _, t := range targets {
		if t.Kind == TargetKindPermanent && t.Permanent != nil {
			t.Permanent.Tapped = false
			gs.LogEvent(Event{Kind: "untap", Source: sourceName(src)})
		}
	}
}

func resolveGainControl(gs *GameState, src *Permanent, e *gameast.GainControl) {
	targets := PickTarget(gs, src, e.Target)
	newController := 0
	if src != nil {
		newController = src.Controller
	}
	// Gain control only makes sense targeting opponents' permanents. If
	// PickTarget returned nothing useful (parser set you_control:true),
	// fall back to picking an opponent's permanent that matches the filter.
	hasUsable := false
	for _, t := range targets {
		if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent.Controller != newController {
			hasUsable = true
			break
		}
	}
	if !hasUsable && src != nil {
		for _, opp := range gs.Opponents(newController) {
			for _, p := range gs.Seats[opp].Battlefield {
				if p != nil {
					targets = []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: opp}}
					hasUsable = true
					break
				}
			}
			if hasUsable {
				break
			}
		}
	}
	for _, t := range targets {
		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		p := t.Permanent
		oldController := p.Controller
		if oldController == newController {
			continue
		}
		// Remove from old controller's battlefield, add to new.
		gs.removePermanent(p)
		p.Controller = newController
		p.Timestamp = gs.NextTimestamp()
		gs.Seats[newController].Battlefield = append(gs.Seats[newController].Battlefield, p)
		gs.LogEvent(Event{
			Kind:   "gain_control",
			Seat:   newController,
			Target: oldController,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"target_card": p.Card.DisplayName(),
			},
		})
	}
}

func resolveSacrifice(gs *GameState, src *Permanent, e *gameast.Sacrifice) {
	// Pick actor seat(s).
	actorSeats := []int{}
	switch e.Actor {
	case "controller", "":
		if src != nil {
			actorSeats = append(actorSeats, src.Controller)
		}
	case "target_player":
		if src != nil {
			actorSeats = append(actorSeats, gs.Opponents(src.Controller)...)
		}
	case "each_opponent":
		if src != nil {
			actorSeats = append(actorSeats, gs.Opponents(src.Controller)...)
		}
	case "each_player":
		// Every non-lost player sacrifices.
		for i, s := range gs.Seats {
			if s != nil && !s.Lost {
				actorSeats = append(actorSeats, i)
			}
		}
	case "that_player_choice", "that_player":
		// "That player" — typically refers to the opponent in single-target
		// contexts. MVP: pick opponents.
		if src != nil {
			actorSeats = append(actorSeats, gs.Opponents(src.Controller)...)
		}
	}
	for _, seat := range actorSeats {
		// Pick the weakest matching creature (lowest power).
		var victim *Permanent
		for _, p := range gs.Seats[seat].Battlefield {
			if !matchesPermanent(e.Query, p) {
				continue
			}
			if victim == nil || p.Power() < victim.Power() {
				victim = p
			}
		}
		if victim == nil {
			continue
		}
		// Route through SacrificePermanent which runs §614 replacement
		// effects, fires dies/LTB triggers, and handles commander redirect.
		// Sacrifice bypasses indestructible per CR §701.17b.
		SacrificePermanent(gs, victim, sourceName(src))
	}
}

func resolveFight(gs *GameState, src *Permanent, e *gameast.Fight) {
	aTargets := PickTarget(gs, src, e.A)
	bTargets := PickTarget(gs, src, e.B)
	if len(aTargets) == 0 || len(bTargets) == 0 {
		return
	}
	a := aTargets[0].Permanent
	b := bTargets[0].Permanent
	if a == nil || b == nil {
		return
	}
	aPow := a.Power()
	bPow := b.Power()

	// A always deals damage to B.
	if aPow > 0 {
		b.MarkedDamage += aPow
		// §702.2 — deathtouch: any damage from a deathtouch source is lethal.
		if a.HasKeyword("deathtouch") {
			if b.Flags == nil {
				b.Flags = map[string]int{}
			}
			b.Flags["deathtouch_damaged"] = 1
		}
	}

	// B deals damage to A only for mutual fights (CR §701.12), not bites.
	if !e.OneSided && bPow > 0 {
		a.MarkedDamage += bPow
		if b.HasKeyword("deathtouch") {
			if a.Flags == nil {
				a.Flags = map[string]int{}
			}
			a.Flags["deathtouch_damaged"] = 1
		}
	}

	kind := "fight"
	if e.OneSided {
		kind = "bite"
	}
	gs.LogEvent(Event{
		Kind:   kind,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"a":      a.Card.DisplayName(),
			"b":      b.Card.DisplayName(),
			"a_pow":  aPow,
			"b_pow":  bPow,
			"mutual": !e.OneSided,
		},
	})
}

// -----------------------------------------------------------------------------
// Tokens / buffs / counters
// -----------------------------------------------------------------------------

func resolveCreateToken(gs *GameState, src *Permanent, e *gameast.CreateToken) {
	count, _ := evalNumber(gs, src, &e.Count)
	if count <= 0 {
		count = 1
	}
	controller := 0
	if src != nil {
		controller = src.Controller
	}

	// §706.10a: "create a token that's a copy of [target]"
	if e.IsCopyOf != nil {
		resolveCreateTokenCopy(gs, src, e, controller, count)
		return
	}

	pow, tough := 0, 0
	if e.PT != nil {
		pow = e.PT[0]
		tough = e.PT[1]
	}
	types := append([]string{}, e.Types...)
	hasToken := false
	hasCreature := false
	for _, t := range types {
		if t == "token" {
			hasToken = true
		}
		if t == "creature" {
			hasCreature = true
		}
	}
	if !hasToken {
		types = append([]string{"token"}, types...)
	}
	if !hasCreature && (pow > 0 || tough > 0 || len(e.Color) > 0) {
		types = append([]string{"creature"}, types...)
	}
	// §614: would_create_token replacement chain.
	if modified, cancelled := FireCreateTokenEvent(gs, controller, count, src); cancelled {
		count = 0
	} else if modified > 0 {
		count = modified
	}
	for i := 0; i < count; i++ {
		card := &Card{
			Name:          tokenName(types, pow, tough),
			Owner:         controller,
			BasePower:     pow,
			BaseToughness: tough,
			Types:         types,
		}
		isCreature := false
		for _, t := range types {
			if t == "creature" {
				isCreature = true
				break
			}
		}
		p := &Permanent{
			Card:          card,
			Controller:    controller,
			Tapped:        e.Tapped,
			SummoningSick: isCreature,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
		RegisterReplacementsForPermanent(gs, p)
		FirePermanentETBTriggers(gs, p)
	}
	// Fire token_created trigger for cards like Chatterfang that care about
	// token creation events. Re-entrancy guard prevents infinite loops when
	// a token_created handler itself creates tokens.
	if count > 0 && (gs.Flags == nil || gs.Flags["in_token_trigger"] == 0) {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": controller,
			"count":           count,
			"types":           types,
			"source":          sourceName(src),
		})
		gs.Flags["in_token_trigger"] = 0
	}
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   controller,
		Target: controller,
		Source: sourceName(src),
		Amount: count,
		Details: map[string]interface{}{
			"types": types,
			"pt":    [2]int{pow, tough},
		},
	})
}

// resolveCreateTokenCopy handles CreateToken with IsCopyOf set (§706.10a).
// Finds the copy source via PickTarget, deep-copies its card, adds token
// type, and fires the full ETB cascade.
func resolveCreateTokenCopy(gs *GameState, src *Permanent, e *gameast.CreateToken, controller, count int) {
	targets := PickTarget(gs, src, *e.IsCopyOf)
	var copySource *Permanent
	for _, t := range targets {
		if t.Kind == TargetKindPermanent && t.Permanent != nil {
			copySource = t.Permanent
			break
		}
	}
	if copySource == nil {
		// Fallback: scan for any creature/permanent on the battlefield.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || (src != nil && p == src) {
					continue
				}
				copySource = p
				break
			}
			if copySource != nil {
				break
			}
		}
	}
	if copySource == nil {
		gs.LogEvent(Event{
			Kind:   "create_token_copy",
			Seat:   controller,
			Source: sourceName(src),
			Details: map[string]interface{}{"copy": "no_target"},
		})
		return
	}
	if modified, cancelled := FireCreateTokenEvent(gs, controller, count, src); cancelled {
		count = 0
	} else if modified > 0 {
		count = modified
	}
	for i := 0; i < count; i++ {
		card := copySource.Card.DeepCopy()
		hasToken := false
		for _, t := range card.Types {
			if t == "token" {
				hasToken = true
				break
			}
		}
		if !hasToken {
			card.Types = append([]string{"token"}, card.Types...)
		}
		card.Owner = controller
		p := &Permanent{
			Card:          card,
			Controller:    controller,
			Tapped:        e.Tapped,
			SummoningSick: true,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
		RegisterReplacementsForPermanent(gs, p)
		FirePermanentETBTriggers(gs, p)
	}
	// Fire token_created trigger for token copies too (same re-entrancy guard).
	if count > 0 && (gs.Flags == nil || gs.Flags["in_token_trigger"] == 0) {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		copyTypes := []string{"token"}
		if copySource.Card != nil {
			copyTypes = copySource.Card.Types
		}
		FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": controller,
			"count":           count,
			"types":           copyTypes,
			"source":          sourceName(src),
		})
		gs.Flags["in_token_trigger"] = 0
	}
	gs.LogEvent(Event{
		Kind:   "create_token_copy",
		Seat:   controller,
		Source: sourceName(src),
		Amount: count,
		Details: map[string]interface{}{
			"copy_of": copySource.Card.DisplayName(),
			"rule":    "706.10a",
		},
	})
}

func tokenName(types []string, pow, tough int) string {
	if len(types) == 0 {
		return "Token"
	}
	// Simple display: "1/1 Soldier Token"
	name := ""
	for i, t := range types {
		if i > 0 {
			name += " "
		}
		name += t
	}
	return name + " Token"
}

func resolveBuff(gs *GameState, src *Permanent, e *gameast.Buff) {
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, apply to source permanent (self-buff).
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	// If targets are all players (not permanents), fall back to an
	// opponent creature or source permanent. Cards like Dark Salvation
	// have player-targeted buff due to parser limitations — the actual
	// oracle targets a creature.
	allPlayers := true
	for _, t := range targets {
		if t.Kind == TargetKindPermanent {
			allPlayers = false
			break
		}
	}
	if allPlayers && src != nil {
		// Try to find an opponent creature to debuff.
		for _, opp := range gs.Opponents(src.Controller) {
			for _, p := range gs.Seats[opp].Battlefield {
				if p != nil && p.IsCreature() {
					targets = []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller}}
					allPlayers = false
					break
				}
			}
			if !allPlayers {
				break
			}
		}
		if allPlayers {
			targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
		}
	}
	duration := e.Duration
	if duration == "" {
		duration = "until_end_of_turn"
	}
	ts := gs.NextTimestamp()
	applied := false
	for _, t := range targets {
		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		t.Permanent.Modifications = append(t.Permanent.Modifications, Modification{
			Power:     e.Power,
			Toughness: e.Toughness,
			Duration:  duration,
			Timestamp: ts,
		})
		applied = true
		gs.LogEvent(Event{
			Kind:   "buff",
			Source: sourceName(src),
			Amount: e.Power,
			Details: map[string]interface{}{
				"target_card": t.Permanent.Card.DisplayName(),
				"power":       e.Power,
				"toughness":   e.Toughness,
				"duration":    duration,
			},
		})
	}
	// Modifications change P/T — invalidate the characteristics cache
	// so subsequent SBA checks see the updated toughness (§704.5f).
	if applied {
		gs.InvalidateCharacteristicsCache()
	}
}

func resolveGrantAbility(gs *GameState, src *Permanent, e *gameast.GrantAbility) {
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, grant to source permanent (self-grant).
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	// If targets are players (not permanents), fall back to source
	// permanent. Cards like Domineering Will have player-targeted
	// grant_ability due to parser limitations.
	allPlayers := true
	for _, t := range targets {
		if t.Kind == TargetKindPermanent {
			allPlayers = false
			break
		}
	}
	if allPlayers && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	for _, t := range targets {
		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		t.Permanent.GrantedAbilities = append(t.Permanent.GrantedAbilities, e.AbilityName)
		gs.LogEvent(Event{
			Kind:   "grant_ability",
			Source: sourceName(src),
			Details: map[string]interface{}{
				"target_card": t.Permanent.Card.DisplayName(),
				"ability":     e.AbilityName,
				"duration":    e.Duration,
			},
		})
	}
}

func resolveCounterMod(gs *GameState, src *Permanent, e *gameast.CounterMod) {
	count, _ := evalNumber(gs, src, &e.Count)
	if count <= 0 {
		count = 1
	}
	targets := PickTarget(gs, src, e.Target)
	// Fallback: if no target found, apply to source permanent (self-targeting
	// counter effects like "put a +1/+1 counter on this creature").
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindPermanent, Permanent: src, Seat: src.Controller}}
	}
	op := e.Op
	if op == "" {
		op = "put"
	}
	// Check if this is a player-targeted counter type (poison, rad, energy, experience).
	// These go on players, not permanents — §122 (poison), Fallout (rad).
	isPlayerCounter := e.CounterKind == "poison" || e.CounterKind == "rad" ||
		e.CounterKind == "energy" || e.CounterKind == "experience"

	for _, t := range targets {
		// Handle player-targeted counters (poison, rad, energy, experience).
		if t.Kind == TargetKindSeat && isPlayerCounter {
			seat := t.Seat
			if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
				continue
			}
			switch e.CounterKind {
			case "poison":
				gs.Seats[seat].PoisonCounters += count
			case "rad":
				if gs.Seats[seat].Flags == nil {
					gs.Seats[seat].Flags = map[string]int{}
				}
				gs.Seats[seat].Flags["rad_counters"] += count
			case "energy":
				if gs.Seats[seat].Flags == nil {
					gs.Seats[seat].Flags = map[string]int{}
				}
				gs.Seats[seat].Flags["energy_counters"] += count
			case "experience":
				if gs.Seats[seat].Flags == nil {
					gs.Seats[seat].Flags = map[string]int{}
				}
				gs.Seats[seat].Flags["experience_counters"] += count
			}
			gs.LogEvent(Event{
				Kind:   "counter_mod",
				Seat:   controllerSeat(src),
				Target: seat,
				Source: sourceName(src),
				Amount: count,
				Details: map[string]interface{}{
					"op":           op,
					"counter_kind": e.CounterKind,
					"on_player":    true,
				},
			})
			continue
		}

		if t.Kind != TargetKindPermanent || t.Permanent == nil {
			continue
		}
		effectiveCount := count
		switch op {
		case "put":
			// §614: would_put_counter replacement chain — Hardened Scales
			// adds +1, Doubling Season doubles (order matters — see
			// replacement_test.go APNAP cases).
			modified, cancelled := FirePutCounterEvent(gs, t.Permanent, e.CounterKind, count, src)
			if cancelled || modified <= 0 {
				continue
			}
			effectiveCount = modified
			t.Permanent.AddCounter(e.CounterKind, modified)
		case "remove":
			t.Permanent.AddCounter(e.CounterKind, -count)
		case "double":
			if t.Permanent.Counters != nil {
				t.Permanent.AddCounter(e.CounterKind, t.Permanent.Counters[e.CounterKind])
			}
		case "move":
			// MVP no-op — move-counters needs a second target slot.
		}
		// §613.4c / Phase 8: counter mutations change the post-pass
		// P/T, so any cached Characteristics for this permanent is
		// now stale. The cheap global-epoch invalidation is correct
		// regardless of which counter kind we touched.
		gs.InvalidateCharacteristicsCache()
		gs.LogEvent(Event{
			Kind:   "counter_mod",
			Source: sourceName(src),
			Amount: effectiveCount,
			Details: map[string]interface{}{
				"target_card":  t.Permanent.Card.DisplayName(),
				"op":           op,
				"counter_kind": e.CounterKind,
			},
		})
		if op == "put" && effectiveCount > 0 {
			FireCardTrigger(gs, "counter_placed", map[string]interface{}{
				"target_perm":   t.Permanent,
				"target_seat":   t.Permanent.Controller,
				"counter_kind":  e.CounterKind,
				"amount":        effectiveCount,
				"source_card":   sourceName(src),
				"source_seat":   controllerSeat(src),
			})
		}
	}
}

// -----------------------------------------------------------------------------
// Mana + library operations
// -----------------------------------------------------------------------------

func resolveAddMana(gs *GameState, src *Permanent, e *gameast.AddMana) {
	if src == nil {
		return
	}
	count := 0
	// MVP: generic bucket. Phase 6 will split into colored pools.
	for _, sym := range e.Pool {
		count += 1 // one mana per symbol
		if sym.Generic > 0 {
			count += sym.Generic - 1
		}
	}
	count += e.AnyColorCount
	if count <= 0 {
		// Scaling mana sources (e.g. "add X mana where X is the number
		// of Elves") have empty Pool and 0 AnyColorCount — the parser
		// couldn't structure the amount. Fall back to gs.Flags["x"]
		// (pre-set by callers or Goldilocks to a reasonable default)
		// so the effect still produces observable mana.
		if gs.Flags != nil {
			if x, ok := gs.Flags["x"]; ok && x > 0 {
				count = x
			}
		}
		if count <= 0 {
			return
		}
	}
	gs.Seats[src.Controller].ManaPool += count
	SyncManaAfterAdd(gs.Seats[src.Controller], count)
	gs.LogEvent(Event{
		Kind:   "add_mana",
		Seat:   src.Controller,
		Target: src.Controller,
		Source: sourceName(src),
		Amount: count,
	})
}

func resolveTutor(gs *GameState, src *Permanent, e *gameast.Tutor) {
	if src == nil {
		return
	}
	seat := src.Controller
	// Delegate to the generic AST-driven tutor resolver which handles:
	// - Full filter matching (type, subtype, color, CMC, extras)
	// - Opposition Agent interception
	// - Correct shuffle ordering (shuffle-first for top-of-library)
	// - "up to N" and "all" quantifiers
	// - Hat-driven card selection (highest CMC for greedy baseline)
	ResolveTutorGeneric(gs, seat, e)
}

// oppositionAgentControlsSeat returns the seat index of the Opposition
// Agent controller that would intercept library searches by `searchingSeat`,
// or -1 if no Agent is active. Engine-side consumer of the per_card flag
// (avoids importing per_card to prevent cycles).
func oppositionAgentControlsSeat(gs *GameState, searchingSeat int) int {
	if gs == nil || gs.Flags == nil {
		return -1
	}
	for i := range gs.Seats {
		if i == searchingSeat {
			continue
		}
		if gs.Flags["opposition_agent_seat_"+itoa(i)] > 0 {
			return i
		}
	}
	return -1
}

// NarsetBlocksDraw returns true if an opponent of drawingSeat controls
// a Narset, Parter of Veils AND the seat has already drawn 1+ cards
// this turn. Consults gs.Flags["narset_parter_seat_N"] (set by per_card
// ETB handler) and gs.Flags["draws_this_turn_seat_N"].
func NarsetBlocksDraw(gs *GameState, drawingSeat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	if gs.Flags["draws_this_turn_seat_"+strconv.Itoa(drawingSeat)] < 1 {
		return false
	}
	for i := range gs.Seats {
		if i == drawingSeat {
			continue
		}
		if gs.Flags["narset_parter_seat_"+strconv.Itoa(i)] > 0 {
			return true
		}
	}
	return false
}

// IncrementDrawCount bumps the per-seat-per-turn draw counter.
func IncrementDrawCount(gs *GameState, seat int) {
	if gs == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["draws_this_turn_seat_"+strconv.Itoa(seat)]++
}

// NecropotenceSkipsDraw returns true if the active seat has a
// Necropotence flag set (skip draw step per oracle text).
func NecropotenceSkipsDraw(gs *GameState, seat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["necropotence_seat_"+strconv.Itoa(seat)] > 0
}

// placeTutoredCard puts a tutored card into the chosen destination.
// Handles "hand" (default), "battlefield" (+ "battlefield_tapped"),
// "graveyard", "top_of_library".
func placeTutoredCard(gs *GameState, seat int, c *Card, dest string) {
	switch dest {
	case "battlefield", "battlefield_tapped":
		tapped := dest == "battlefield_tapped"
		p := &Permanent{
			Card:          c,
			Controller:    seat,
			Tapped:        tapped,
			SummoningSick: true,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
		RegisterReplacementsForPermanent(gs, p)
		FirePermanentETBTriggers(gs, p)
	case "graveyard":
		gs.moveToZone(seat, c, "graveyard")
	case "top_of_library":
		gs.moveToZone(seat, c, "library_top")
	case "hand", "":
		gs.moveToZone(seat, c, "hand")
	default:
		gs.moveToZone(seat, c, "hand")
	}
}

func resolveReanimate(gs *GameState, src *Permanent, e *gameast.Reanimate) {
	if src == nil {
		return
	}
	seat := src.Controller
	// Pick the first matching card in the graveyard.
	gy := gs.Seats[seat].Graveyard
	idx := -1
	for i, c := range gy {
		if cardMatchesFilter(c, e.Query) {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	c := gy[idx]
	removeCardFromZone(gs, seat, c, "graveyard")
	tapped := e.Destination == "battlefield_tapped"
	p := &Permanent{
		Card:          c,
		Controller:    seat,
		Tapped:        tapped,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	RegisterReplacementsForPermanent(gs, p)
	FirePermanentETBTriggers(gs, p)
	gs.LogEvent(Event{
		Kind:   "reanimate",
		Seat:   seat,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"target_card": c.DisplayName(),
		},
	})
}

func resolveRecurse(gs *GameState, src *Permanent, e *gameast.Recurse) {
	if src == nil {
		return
	}
	seat := src.Controller
	gy := gs.Seats[seat].Graveyard
	idx := -1
	for i, c := range gy {
		if cardMatchesFilter(c, e.Query) {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	c := gy[idx]
	dest := e.Destination
	if dest == "" {
		dest = "hand"
	}
	MoveCard(gs, c, seat, "graveyard", dest, "recurse")
}

func resolveShuffle(gs *GameState, src *Permanent, e *gameast.Shuffle) {
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		shuffleLibrary(gs, seat)
		gs.LogEvent(Event{
			Kind:   "shuffle",
			Seat:   seat,
			Target: seat,
			Source: sourceName(src),
		})
	}
}

func shuffleLibrary(gs *GameState, seat int) {
	if seat < 0 || seat >= len(gs.Seats) || gs.Rng == nil {
		return
	}
	lib := gs.Seats[seat].Library
	gs.Rng.Shuffle(len(lib), func(i, j int) {
		lib[i], lib[j] = lib[j], lib[i]
	})
}

// cardMatchesFilter is the card-side filter matcher used by Reanimate,
// Recurse, and other graveyard/exile zone searches. Handles base type,
// creature subtypes, Extra adjectives, color filters, and CMC constraints.
func cardMatchesFilter(c *Card, f gameast.Filter) bool {
	if c == nil {
		return false
	}
	// Use the comprehensive tutor filter matcher — it handles all the
	// same base types, extras, colors, and CMC constraints that apply
	// to any card-side filter (not just tutor searches).
	return cardMatchesTutorFilter(c, f)
}

// cardHasType is defined in cost_modifiers.go — single definition shared
// across the package. Removed duplicate here.

// -----------------------------------------------------------------------------
// Stack / timing
// -----------------------------------------------------------------------------

func resolveCounterSpell(gs *GameState, src *Permanent, e *gameast.CounterSpell) {
	// Generic counterspell resolver — reads the AST's CounterSpell node
	// fields (target filter, unless-pay cost) and resolves them against
	// the game state. Replaces the old Phase 3 MVP "mark top as countered"
	// stub with proper filter-aware targeting.
	//
	// Per-card snowflake handlers (Swan Song, Arcane Denial, Mana Drain,
	// etc.) are dispatched BEFORE this function is called — they handle
	// side effects that can't be expressed in the AST. This function only
	// runs for counterspells that DON'T have a per-card handler.
	casterSeat := controllerSeat(src)
	ResolveCounterSpellGeneric(gs, casterSeat, e)
}

func resolveCopySpell(gs *GameState, src *Permanent, e *gameast.CopySpell) {
	controller := controllerSeat(src)
	// §707.2: find the spell to copy on the stack.
	var target *StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		item := gs.Stack[i]
		if item == nil || item.Countered {
			continue
		}
		if item.Card == nil {
			continue
		}
		// Prefer instant/sorcery if the filter says so; otherwise any spell.
		target = item
		break
	}
	if target == nil {
		gs.LogEvent(Event{
			Kind:   "copy_spell",
			Source: sourceName(src),
			Details: map[string]interface{}{"copy": "no_target"},
		})
		return
	}
	// §707.2: create a copy of the spell on the stack. The copy is not
	// "cast" — it was "created" directly on the stack (CR §706.10).
	copyCard := target.Card.DeepCopy()
	copyCard.IsCopy = true // CR §704.5e — ceases to exist outside stack/battlefield
	copyItem := &StackItem{
		Controller: controller,
		Card:       copyCard,
		Effect:     target.Effect,
		IsCopy:     true,
		Kind:       target.Kind,
	}
	// §707.10c: the copy's controller may choose new targets.
	if e.MayChooseNewTargets && len(target.Targets) > 0 {
		copyItem.Targets = append([]Target(nil), target.Targets...)
	}
	PushStackItem(gs, copyItem)
	gs.LogEvent(Event{
		Kind:   "copy_spell",
		Seat:   controller,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"copied":  target.Card.DisplayName(),
			"is_copy": true,
			"rule":    "707.2",
		},
	})
}

func resolveCopyPermanent(gs *GameState, src *Permanent, e *gameast.CopyPermanent) {
	if src == nil {
		gs.LogEvent(Event{Kind: "copy_permanent", Source: sourceName(src)})
		return
	}
	var copySource *Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == src || !p.IsCreature() {
				continue
			}
			copySource = p
		}
	}
	if copySource == nil {
		gs.LogEvent(Event{
			Kind:   "copy_permanent",
			Source: sourceName(src),
			Details: map[string]interface{}{"copy": "no_target"},
		})
		return
	}
	if e.AsToken {
		// §706.10a — create a token that's a copy of the target.
		// The token enters as a new permanent; src is NOT modified.
		controller := src.Controller
		card := copySource.Card.DeepCopy()
		// Ensure "token" type so IsToken() works and zone conservation
		// doesn't count it as a real card.
		hasToken := false
		for _, t := range card.Types {
			if t == "token" {
				hasToken = true
				break
			}
		}
		if !hasToken {
			card.Types = append([]string{"token"}, card.Types...)
		}
		card.Owner = controller
		p := &Permanent{
			Card:          card,
			Controller:    controller,
			SummoningSick: true,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
		RegisterReplacementsForPermanent(gs, p)
		FirePermanentETBTriggers(gs, p)
		gs.LogEvent(Event{
			Kind:   "create_token_copy",
			Seat:   controller,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"copy_of": copySource.Card.DisplayName(),
				"rule":    "706.10a",
			},
		})
		return
	}
	// §706.2 / §613.1a — src becomes a copy of the target.
	CopyPermanentLayered(gs, src, copySource, DurationPermanent)
}

func resolveExtraTurn(gs *GameState, src *Permanent, e *gameast.ExtraTurn) {
	gs.Flags["extra_turns_pending"]++
	gs.LogEvent(Event{
		Kind:   "extra_turn",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
	})
}

func resolveExtraCombat(gs *GameState, src *Permanent, e *gameast.ExtraCombat) {
	gs.Flags["extra_combats_pending"]++
	// Mirror into GameState.PendingExtraCombats which the turn loop's
	// combat-phase loop reads. Flags stays populated for older callers.
	gs.PendingExtraCombats++
	gs.LogEvent(Event{
		Kind:   "extra_combat",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
	})
}

// -----------------------------------------------------------------------------
// Info / reveals (log-only MVP)
// -----------------------------------------------------------------------------

func resolveScry(gs *GameState, src *Permanent, e *gameast.Scry) {
	count, _ := evalNumber(gs, src, &e.Count)
	gs.LogEvent(Event{
		Kind:   "scry",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: count,
	})
}

func resolveSurveil(gs *GameState, src *Permanent, e *gameast.Surveil) {
	count, _ := evalNumber(gs, src, &e.Count)
	gs.LogEvent(Event{
		Kind:   "surveil",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: count,
	})
}

func resolveLookAt(gs *GameState, src *Permanent, e *gameast.LookAt) {
	count, _ := evalNumber(gs, src, &e.Count)
	gs.LogEvent(Event{
		Kind:   "look_at",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: count,
	})
}

func resolveReveal(gs *GameState, src *Permanent, e *gameast.Reveal) {
	count, _ := evalNumber(gs, src, &e.Count)
	gs.LogEvent(Event{
		Kind:   "reveal",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: count,
		Details: map[string]interface{}{
			"source": e.Source,
		},
	})
}

// -----------------------------------------------------------------------------
// Win / lose
// -----------------------------------------------------------------------------

func resolveWinGame(gs *GameState, src *Permanent, e *gameast.WinGame) {
	targets := PickTarget(gs, src, e.Target)
	if len(targets) == 0 && src != nil {
		targets = []Target{{Kind: TargetKindSeat, Seat: src.Controller}}
	}
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		gs.Seats[seat].Won = true
		gs.LogEvent(Event{
			Kind:   "win_game",
			Seat:   seat,
			Target: seat,
			Source: sourceName(src),
		})
	}
}

func resolveLoseGame(gs *GameState, src *Permanent, e *gameast.LoseGame) {
	targets := PickTarget(gs, src, e.Target)
	for _, t := range targets {
		seat, ok := seatFromTarget(t)
		if !ok {
			continue
		}
		gs.Seats[seat].Lost = true
		gs.LogEvent(Event{
			Kind:   "lose_game",
			Seat:   seat,
			Target: seat,
			Source: sourceName(src),
		})
	}
}

// -----------------------------------------------------------------------------
// Small helpers used across handlers
// -----------------------------------------------------------------------------

func controllerSeat(src *Permanent) int {
	if src == nil {
		return -1
	}
	return src.Controller
}

func sourceName(src *Permanent) string {
	if src == nil || src.Card == nil {
		return ""
	}
	return src.Card.DisplayName()
}

// seatFromTarget extracts a seat index from a Target. For seat targets, returns
// the seat directly. For permanent targets (e.g. "self" base that resolved to
// a permanent instead of a player), returns the permanent's controller.
// Returns -1 if no seat can be determined.
func seatFromTarget(t Target) (int, bool) {
	seat, ok := t.SeatTarget()
	if ok {
		return seat, true
	}
	if t.Kind == TargetKindPermanent && t.Permanent != nil {
		return t.Permanent.Controller, true
	}
	return -1, false
}
