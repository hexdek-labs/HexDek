package main

// conditional_setup.go — registry-driven priming for triggered-ability
// preconditions. The #1 source of Goldilocks gaps (~4.7K draw/life/damage
// failures) was Thor placing the card on the battlefield but never
// satisfying the trigger's CONDITION clause: a "whenever a creature you
// control dies, draw a card" card has nothing to die, so the trigger
// never fires and the test reports a dead effect.
//
// This file maps each canonical trigger event to a small priming action
// that builds the world so the trigger CAN fire when fireTriggerEvent
// runs after the snapshot. Priming runs BEFORE the snapshot so the
// before/after diff captures only the trigger's effect, not the priming
// entities.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// conditionAction primes the game state so a trigger's condition can be
// satisfied when fireTriggerEvent runs. Each action is idempotent: if
// the world already has what's needed (e.g. an opponent creature), the
// action is a no-op.
type conditionAction struct {
	// kind is the canonical condition slug used in trace entries.
	kind string
	// describe returns a one-line summary suitable for trace output.
	describe func(t *gameast.Trigger) string
	// apply mutates gs to prime the precondition. srcPerm is the card
	// under test; it may be nil if Goldilocks couldn't find the source
	// permanent (we still try a best-effort prime).
	apply func(gs *gameengine.GameState, srcPerm *gameengine.Permanent)
}

// classifyTrigger maps a Trigger AST node to a registry slug.
//
// We can't dispatch on Event alone — the AST also encodes the actor
// filter ("a creature you control" vs "a creature an opponent controls")
// and the phase ("upkeep" vs "end_step"), and the firing logic differs
// per slot.
func classifyTrigger(t *gameast.Trigger) string {
	if t == nil {
		return ""
	}
	event := strings.ToLower(strings.TrimSpace(t.Event))
	phase := strings.ToLower(strings.TrimSpace(t.Phase))

	actorBase := ""
	if t.Actor != nil {
		actorBase = strings.ToLower(t.Actor.Base)
	}
	opponentActor := strings.Contains(actorBase, "opponent") ||
		strings.Contains(actorBase, "an opponent")

	switch {
	case event == "dies" || strings.Contains(event, "dies") ||
		strings.Contains(event, "is put into a graveyard"):
		return "creature_dies"
	case event == "etb" || strings.Contains(event, "enters"):
		return "creature_etb"
	case event == "attacks" || strings.Contains(event, "attack"):
		return "attacks"
	case event == "deal_combat_damage" || event == "deals_combat_damage" ||
		strings.Contains(event, "combat damage"):
		return "combat_damage"
	case strings.Contains(event, "cast") || strings.Contains(event, "spell"):
		if opponentActor {
			return "opponent_cast"
		}
		return "cast_spell"
	case strings.Contains(event, "gain") && strings.Contains(event, "life"):
		return "gain_life"
	case strings.Contains(event, "lose") && strings.Contains(event, "life"):
		return "lose_life"
	case strings.Contains(event, "draw"):
		return "draw_card"
	case strings.Contains(event, "discard"):
		if opponentActor {
			return "opponent_discards"
		}
		return "discard"
	case strings.Contains(event, "leaves") || strings.Contains(event, "ltb"):
		return "ltb"
	case strings.Contains(event, "sacrific"):
		return "sacrifice"
	case event == "phase" || phase != "":
		switch phase {
		case "upkeep":
			return "upkeep"
		case "end_step", "end of turn", "endstep":
			return "end_step"
		case "combat_start", "begin_combat", "beginning of combat":
			return "begin_combat"
		case "untap":
			return "untap_step"
		case "draw_step":
			return "draw_step"
		}
		return "phase_" + phase
	}
	return ""
}

// triggerConditionActions is the dispatch table. The keys come from
// classifyTrigger.
var triggerConditionActions = map[string]conditionAction{
	"creature_dies": {
		kind: "creature_dies",
		describe: func(t *gameast.Trigger) string {
			return "place 'Setup Victim' 1/1 token on seat 0 (will die in fireTriggerEvent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			// Always add a fresh victim, distinct from srcPerm and any
			// existing prime, so 'whenever ANOTHER creature dies' triggers
			// also fire (a friendly creature already on the board may be
			// the source itself).
			placeNamedFriendlyCreature(gs, "Setup Victim")
		},
	},

	"creature_etb": {
		kind: "creature_etb",
		describe: func(t *gameast.Trigger) string {
			actor := ""
			if t.Actor != nil {
				actor = t.Actor.Base
			}
			if strings.Contains(strings.ToLower(actor), "another") {
				return "place 'ETB Buddy' 1/1 token (a second creature for 'another creature ETBs')"
			}
			return "no-op (source's own ETB will fire trigger)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			// Source's own ETB is fired by fireTriggerEvent. For
			// "another" variants we need a second creature already
			// present so the source's ETB can trigger off it (or vice
			// versa — fireTriggerEvent invokes the source's ETB hook).
			placeNamedFriendlyCreature(gs, "ETB Buddy")
		},
	},

	"attacks": {
		kind: "attacks",
		describe: func(t *gameast.Trigger) string {
			return "untap source so it can attack; ensure opponent has no blockers"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if srcPerm != nil {
				srcPerm.Tapped = false
				if srcPerm.Flags == nil {
					srcPerm.Flags = map[string]int{}
				}
				// Clear summoning sickness so the source is attack-eligible.
				delete(srcPerm.Flags, "summoning_sick")
			}
		},
	},

	"combat_damage": {
		kind: "combat_damage",
		describe: func(t *gameast.Trigger) string {
			return "ensure opponent creature exists as combat-damage target"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if !hasOpponentCreature(gs) {
				placeTargetCreatureOnOpponent(gs)
			}
		},
	},

	"cast_spell": {
		kind: "cast_spell",
		describe: func(t *gameast.Trigger) string {
			return "ensure 5 castable cards in seat 0 library"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats[0].Library) < 5 {
				fillLibrary(gs, 0, 5)
			}
		},
	},

	"opponent_cast": {
		kind: "opponent_cast",
		describe: func(t *gameast.Trigger) string {
			return "ensure 5 castable cards in seat 1 library (opponent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats) > 1 && len(gs.Seats[1].Library) < 5 {
				fillLibrary(gs, 1, 5)
			}
		},
	},

	"gain_life": {
		kind: "gain_life",
		describe: func(t *gameast.Trigger) string {
			return "no priming (life gain is fired in fireTriggerEvent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"lose_life": {
		kind: "lose_life",
		describe: func(t *gameast.Trigger) string {
			return "raise seat 0 life to 30 so a fired loss is observable"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if gs.Seats[0].Life < 30 {
				gs.Seats[0].Life = 30
			}
		},
	},

	"draw_card": {
		kind: "draw_card",
		describe: func(t *gameast.Trigger) string {
			return "ensure 5 cards in seat 0 library"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats[0].Library) < 5 {
				fillLibrary(gs, 0, 5)
			}
		},
	},

	"discard": {
		kind: "discard",
		describe: func(t *gameast.Trigger) string {
			return "ensure 3 cards in seat 0 hand"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats[0].Hand) < 3 {
				fillHand(gs, 0, 3-len(gs.Seats[0].Hand))
			}
		},
	},

	"opponent_discards": {
		kind: "opponent_discards",
		describe: func(t *gameast.Trigger) string {
			return "ensure 3 cards in seat 1 hand (opponent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats) > 1 && len(gs.Seats[1].Hand) < 3 {
				fillHand(gs, 1, 3-len(gs.Seats[1].Hand))
			}
		},
	},

	"ltb": {
		kind: "ltb",
		describe: func(t *gameast.Trigger) string {
			return "no priming (source bounced in fireTriggerEvent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"sacrifice": {
		kind: "sacrifice",
		describe: func(t *gameast.Trigger) string {
			return "place 'Sac Fodder' 1/1 token as a sacrifice candidate"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			placeNamedFriendlyCreature(gs, "Sac Fodder")
		},
	},

	"upkeep": {
		kind: "upkeep",
		describe: func(t *gameast.Trigger) string {
			return "no priming (phase advance happens in fireTriggerEvent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"end_step": {
		kind: "end_step",
		describe: func(t *gameast.Trigger) string {
			return "no priming (phase advance happens in fireTriggerEvent)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"begin_combat": {
		kind: "begin_combat",
		describe: func(t *gameast.Trigger) string {
			return "no priming (phase advance handled at fire time)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"untap_step": {
		kind: "untap_step",
		describe: func(t *gameast.Trigger) string {
			return "no priming (phase advance handled at fire time)"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {},
	},

	"draw_step": {
		kind: "draw_step",
		describe: func(t *gameast.Trigger) string {
			return "ensure 5 cards in seat 0 library so the draw step succeeds"
		},
		apply: func(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
			if len(gs.Seats[0].Library) < 5 {
				fillLibrary(gs, 0, 5)
			}
		},
	},
}

// primeTriggerCondition runs the registry action that satisfies the
// card's trigger precondition, recording CONDITION_SETUP and TRIGGER_FIRE
// trace entries. Returns true if a registry entry matched, false on
// fallback (no action taken — Goldilocks falls back to existing
// behaviour, i.e. fireTriggerEvent does its own best-effort firing).
func primeTriggerCondition(gs *gameengine.GameState, info *effectInfo, srcPerm *gameengine.Permanent, tr *Tracer) bool {
	if info == nil || info.trigger == nil || gs == nil {
		return false
	}
	slug := classifyTrigger(info.trigger)
	if slug == "" {
		tr.Record("CONDITION_SETUP", "event=%q (unrecognised) → fallback to fireTriggerEvent default",
			info.trigger.Event)
		return false
	}
	action, ok := triggerConditionActions[slug]
	if !ok {
		// Phase events that didn't match a specific phase fall through to
		// here. We still classify so the trace is informative.
		tr.Record("CONDITION_SETUP", "slug=%s (no registry action) → fallback", slug)
		return false
	}

	tr.Record("CONDITION_SETUP", "%q → %s", info.trigger.Event, action.describe(info.trigger))
	action.apply(gs, srcPerm)

	cardName := "<unknown>"
	if srcPerm != nil && srcPerm.Card != nil {
		cardName = srcPerm.Card.Name
	}
	tr.Record("TRIGGER_FIRE", "%s primed on %q", slug, cardName)

	// Layer the intervening-if priming on top of the event-based action.
	// Most "if X this turn / if you have the city's blessing / if you
	// control another <subtype>" conditions live in the conditional_effect
	// args text, not in trigger.Condition (which the parser leaves null
	// for these cards). The two priming layers are orthogonal.
	primeInterveningIf(gs, info, srcPerm, tr)
	return true
}

// ---------------------------------------------------------------------------
// Intervening-if priming.
//
// For triggered abilities of shape "AT/WHENEVER trigger, IF condition,
// EFFECT" the parser emits the trigger as Trigger{Event=phase/etb/...} and
// the rest as a single Modification(kind="conditional_effect",
// args=["if X, Y"]) at the effect slot. trigger.Condition and
// intervening_if are both null. Without priming, evalCondition's "raw"
// fallback defaults to TRUE so the effect fires anyway, but the resulting
// trace is misleading and any per-card listener that reads concrete state
// (Lathiel's lathiel_life_gained_this_turn flag, Sorin's seat
// life_gained_this_turn counter, Twilight Prophet's citys_blessing) will
// silently fail.
//
// primeInterveningIf walks info.condition and the conditional_effect text
// and applies the appropriate setup so both the engine's evalCondition
// path AND any per-card flag-reading handler observe the condition as
// satisfied.
// ---------------------------------------------------------------------------

var (
	reLifeMoreStarting = regexp.MustCompile(`at least (\d+) life more than your starting life total`)
	reControlAnother   = regexp.MustCompile(`you control another ([a-z][a-z\- ]*?)(?:[,.]| and| or|$)`)
	reControlSubtype   = regexp.MustCompile(`you control a ([a-z][a-z\- ]*?)(?:[,.]| and| or|$)`)
)

// extractInterveningText returns the lower-cased "if ..." clause text from
// either info.condition (typed Condition AST) or from a conditional_effect
// ModificationEffect's args[0].
func extractInterveningText(info *effectInfo) string {
	if info == nil {
		return ""
	}
	if info.condition != nil {
		// A typed Condition. Stringify Kind+Args so downstream
		// substring matchers pick up "raw" args[0] verbatim AND
		// well-known kinds via their Kind tag.
		parts := []string{strings.ToLower(info.condition.Kind)}
		for _, a := range info.condition.Args {
			parts = append(parts, strings.ToLower(fmt.Sprintf("%v", a)))
		}
		return strings.Join(parts, " ")
	}
	if t := conditionalEffectText(info.effect); t != "" {
		return t
	}
	return conditionalEffectText(info.fullEffect)
}

// conditionalEffectText extracts args[0] from a *gameast.ModificationEffect
// whose ModKind is "conditional_effect", recursively descending Sequence
// and Optional wrappers.
func conditionalEffectText(eff gameast.Effect) string {
	if eff == nil {
		return ""
	}
	switch e := eff.(type) {
	case *gameast.ModificationEffect:
		if e.ModKind == "conditional_effect" && len(e.Args) > 0 {
			if s, ok := e.Args[0].(string); ok {
				return strings.ToLower(s)
			}
		}
	case *gameast.Sequence:
		for _, item := range e.Items {
			if t := conditionalEffectText(item); t != "" {
				return t
			}
		}
	case *gameast.Optional_:
		return conditionalEffectText(e.Body)
	}
	return ""
}

// primeInterveningIf applies state mutations that satisfy embedded "if X"
// clauses found in info. Returns true when at least one prime was applied.
func primeInterveningIf(gs *gameengine.GameState, info *effectInfo, srcPerm *gameengine.Permanent, tr *Tracer) bool {
	text := extractInterveningText(info)
	if text == "" {
		return false
	}

	primed := false

	// Life gain/loss this turn. Order matters: the combined "gained or
	// lost" form must be checked before the individual matchers so we
	// don't double-apply.
	switch {
	case strings.Contains(text, "gained or lost life this turn"):
		primeGainedLife(gs, 3)
		primeLostLife(gs, 1)
		tr.Record("CONDITION_SETUP",
			"%q → GainLife(seat0, 3) + LoseLife(seat0, 1)",
			"gained or lost life this turn")
		primed = true
	case strings.Contains(text, "gained life this turn"):
		primeGainedLife(gs, 3)
		tr.Record("CONDITION_SETUP",
			"%q → GainLife(seat0, 3)",
			"gained life this turn")
		primed = true
	case strings.Contains(text, "lost life this turn"):
		primeLostLife(gs, 2)
		tr.Record("CONDITION_SETUP",
			"%q → LoseLife(seat0, 2)",
			"lost life this turn")
		primed = true
	}

	// Counter placement this turn. Lasting Tarfire, Lord Jyscal Guado.
	if strings.Contains(text, "put a counter") && strings.Contains(text, "this turn") {
		primeCounterPlaced(gs)
		tr.Record("CONDITION_SETUP",
			"%q → +1/+1 counter on friendly creature",
			"put a counter on a creature this turn")
		primed = true
	}

	// Life-vs-starting threshold. Angel of Destiny: "at least 15 life
	// more than your starting life total".
	if m := reLifeMoreStarting.FindStringSubmatch(text); m != nil {
		n, _ := strconv.Atoi(m[1])
		primeLifeMoreThanStarting(gs, n)
		tr.Record("CONDITION_SETUP",
			"%q → set seat0 Life=%d",
			fmt.Sprintf("at least %d life more than starting", n),
			gs.Seats[0].Life)
		primed = true
	}

	// Ascend / city's blessing. Twilight Prophet.
	if strings.Contains(text, "city's blessing") || strings.Contains(text, "citys blessing") {
		primeAscend(gs)
		tr.Record("CONDITION_SETUP",
			"%q → 10 permanents on seat0, citys_blessing flag set",
			"city's blessing (ascend)")
		primed = true
	}

	// Subtype control. Acclaimed Contender ("if you control another
	// Knight"). The "another <subtype>" matcher takes precedence over
	// the bare "a <subtype>" matcher because a card can satisfy both
	// clauses ("control another Knight" implies "control a Knight").
	if m := reControlAnother.FindStringSubmatch(text); m != nil {
		subtype := strings.TrimSpace(m[1])
		if subtype != "" && !isGenericWord(subtype) {
			placeNamedFriendlyCreatureWithSubtype(gs,
				strings.Title(subtype)+" Buddy", subtype)
			tr.Record("CONDITION_SETUP",
				"%q → placed %s creature on seat0",
				"control another "+subtype, subtype)
			primed = true
		}
	} else if m := reControlSubtype.FindStringSubmatch(text); m != nil {
		subtype := strings.TrimSpace(m[1])
		if subtype != "" && !isGenericWord(subtype) {
			placeNamedFriendlyCreatureWithSubtype(gs,
				strings.Title(subtype)+" Buddy", subtype)
			tr.Record("CONDITION_SETUP",
				"%q → placed %s creature on seat0",
				"you control a "+subtype, subtype)
			primed = true
		} else if subtype == "creature" {
			// Birthing Ritual ("if you control a creature"). The
			// source itself usually counts (creatures bring their
			// own permanent), but enchantment sources need a
			// separate witness.
			if primeFriendlyCreatureIfMissing(gs) {
				tr.Record("CONDITION_SETUP",
					"%q → placed witness creature on seat0",
					"you control a creature")
				primed = true
			}
		}
	}

	// Exile linkage. Smirking Spelljacker ("if a card is exiled with
	// it"). Place a card in seat0's exile and tag the source with a
	// has-exiled-card flag so the per-card handler can find it.
	if (strings.Contains(text, "exiled with it") ||
		strings.Contains(text, "exiled with this") ||
		strings.Contains(text, "card is exiled with")) && srcPerm != nil {
		primeExiledWith(gs, srcPerm)
		tr.Record("CONDITION_SETUP",
			"%q → exile-linked card placed",
			"a card is exiled with it")
		primed = true
	}

	return primed
}

// ---------------------------------------------------------------------------
// Intervening-if priming helpers.
// ---------------------------------------------------------------------------

// primeGainedLife fires GainLife so per-card listeners (Lathiel, Heliod,
// Vito) increment their own counters, AND tops up seat.Flags so handlers
// (Sorin, Shanna) that read the seat-level flag directly see the gain.
// Both paths matter because the engine has not centralised turn-scoped
// life tracking.
func primeGainedLife(gs *gameengine.GameState, amount int) {
	if gs == nil || amount <= 0 || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	gameengine.GainLife(gs, 0, amount, "thor_priming")
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["life_gained_this_turn"] += amount
}

// primeLostLife decrements seat 0 life and sets the seat-level flag. The
// engine has no public LoseLife helper, so we mutate Life and synthesise
// the trigger event for any per-card listeners.
func primeLostLife(gs *gameengine.GameState, amount int) {
	if gs == nil || amount <= 0 || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	gs.Seats[0].Life -= amount
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["life_lost_this_turn"] += amount
	gs.Seats[0].Flags["lost_life_this_turn"] += amount
	gameengine.FireCardTrigger(gs, "life_lost", map[string]interface{}{
		"seat":   0,
		"amount": amount,
		"source": "thor_priming",
	})
}

// primeCounterPlaced ensures a friendly creature has a +1/+1 counter
// placed on it during the current turn and fires the engine's
// counter_placed trigger so listeners increment their this-turn flags.
func primeCounterPlaced(gs *gameengine.GameState) {
	if gs == nil || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	var target *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			target = p
			break
		}
	}
	if target == nil {
		target = placeNamedFriendlyCreature(gs, "Counter Recipient")
	}
	target.AddCounter("+1/+1", 1)
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["counter_placed_this_turn"] = 1
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["counter_placed_this_turn"] = 1
	gameengine.FireCardTrigger(gs, "counter_placed", map[string]interface{}{
		"target_perm":  target,
		"target_seat":  0,
		"counter_kind": "+1/+1",
		"amount":       1,
		"source_card":  "thor_priming",
		"source_seat":  0,
	})
}

// primeLifeMoreThanStarting raises seat 0 life so it sits exactly n above
// StartingLife. Falls back to a Commander default of 40 when StartingLife
// is unset (the goldilocks factory builds seats at Life=20 without
// populating StartingLife).
func primeLifeMoreThanStarting(gs *gameengine.GameState, n int) {
	if gs == nil || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	starting := gs.Seats[0].StartingLife
	if starting <= 0 {
		starting = 40
	}
	target := starting + n
	if gs.Seats[0].Life < target {
		gs.Seats[0].Life = target
	}
}

// primeAscend places enough vanilla permanents on seat 0 to satisfy the
// 10-permanent threshold and eagerly sets the citys_blessing flag so
// readers that bypass CheckAscend (Twilight Prophet's intervening-if
// path) still observe the blessing.
func primeAscend(gs *gameengine.GameState) {
	if gs == nil || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	seat := gs.Seats[0]
	for len(seat.Battlefield) < 10 {
		seat.Battlefield = append(seat.Battlefield, &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:  fmt.Sprintf("Ascend Filler %d", len(seat.Battlefield)),
				Owner: 0,
				Types: []string{"land"},
			},
			Controller: 0,
			Owner:      0,
			Flags:      map[string]int{},
			Counters:   map[string]int{},
		})
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["citys_blessing"] = 1
	gameengine.CheckAscend(gs, 0)
}

// primeFriendlyCreatureIfMissing places a witness creature on seat 0 only
// when none currently exists. Returns true if a creature was placed.
// Intended for "if you control a creature" preconditions on
// non-creature sources (Birthing Ritual is an enchantment).
func primeFriendlyCreatureIfMissing(gs *gameengine.GameState) bool {
	if gs == nil || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return false
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			return false
		}
	}
	placeNamedFriendlyCreature(gs, "Creature Witness")
	return true
}

// primeExiledWith places a card in seat 0's exile and flags srcPerm as
// having an exiled-with companion. Smirking Spelljacker and similar
// imprint-style cards check for this on attack.
func primeExiledWith(gs *gameengine.GameState, srcPerm *gameengine.Permanent) {
	if gs == nil || srcPerm == nil || len(gs.Seats) == 0 || gs.Seats[0] == nil {
		return
	}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, &gameengine.Card{
		Name:  "Exiled Companion",
		Owner: 0,
		Types: []string{"instant"},
	})
	if srcPerm.Flags == nil {
		srcPerm.Flags = map[string]int{}
	}
	srcPerm.Flags["card_exiled_with"] = 1
}

// placeNamedFriendlyCreature is a tiny variant of placeFriendlyCreature
// that uses a caller-supplied name. The default helper hard-codes
// "Friendly Creature", but we want distinct trace-friendly names per
// priming role ("Setup Victim", "ETB Buddy", "Sac Fodder").
func placeNamedFriendlyCreature(gs *gameengine.GameState, name string) *gameengine.Permanent {
	perm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          name,
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 1,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	return perm
}

// ---------------------------------------------------------------------------
// Raw-condition scaffolding (Category B / D fix).
//
// The parser surfaces "if ..." and "as long as ..." clauses as Conditions
// with kind = intervening_if / as_long_as / conditional and the verbatim
// English in args[0]. The existing setupCondition switch only handles
// canonicalised kinds (fateful_hour, threshold, morbid, etc.), so cards
// like Land Tax ("if an opponent controls more lands than you...") and
// Oversold Cemetery ("if there are four or more creature cards in your
// graveyard...") never had their preconditions satisfied.
//
// detectConditionScaffold inspects the raw text and returns a shape; apply
// mutates gs accordingly; describe returns a one-line summary used for
// CONDITION_SETUP traces.
// ---------------------------------------------------------------------------

type conditionScaffoldKind int

const (
	condScaffoldNone conditionScaffoldKind = iota
	condScaffoldOpponentMoreLands
	condScaffoldYouControlSubtype
	condScaffoldCreatureDiedThisTurn
	condScaffoldCreatureCardsInGraveyard
	condScaffoldCardInGraveyard
	condScaffoldEnergyThreshold

	// Trigger-condition scaffold kinds — handle raw condition text that
	// describes a trigger event precondition. These bridge the gap when
	// the AST parser emits a raw/intervening_if condition instead of a
	// structured Trigger node, or when a static ability's condition text
	// references a trigger-like predicate.
	condScaffoldGainedLifeThisTurn
	condScaffoldCastSpellThisTurn
	condScaffoldCreatureETBThisTurn
	condScaffoldDrawnCardThisTurn
	condScaffoldAttackedThisTurn
	condScaffoldSacrificedThisTurn
	condScaffoldCombatDamageDealt
	condScaffoldLandfallThisTurn
	condScaffoldDiscardedThisTurn
	condScaffoldEnchantedCreature
	condScaffoldOpponentLostLife
	condScaffoldLifeAboveThreshold
	condScaffoldLifeBelowThreshold
	condScaffoldUpkeepPhase

	// Ability-word / status scaffold kinds — each mirrors a Check<Word>
	// helper in the engine. Detection accepts both the ability-word slug
	// ("hellbent", "delirium") and the verbatim English description that
	// the AST sometimes surfaces ("if you have no cards in hand", "four or
	// more card types in your graveyard").
	condScaffoldHellbent
	condScaffoldMonarch
	condScaffoldInitiative
	condScaffoldDelirium
	condScaffoldSpellMastery
	condScaffoldRevolt
	condScaffoldMetalcraft
	condScaffoldFerocious
	condScaffoldFormidable

	// New engine structure scaffold kinds — bridge the gap between
	// the TurnCounters migration and the scaffold priming system.
	condScaffoldPermanentLeftBF     // disappear / void — Turn.PermanentsLeft
	condScaffoldSecondSpellThisTurn // CastRecord nth-spell — Turn.Casts
	condScaffoldDescendedThisTurn   // descended — Turn.Descended
	condScaffoldLifeLostThisTurn    // opponent/self lost life — Turn.LifeLost
	condScaffoldTokensCreatedCount  // X = tokens created — Turn.TokensCreated
	condScaffoldCastFromExile       // cast from exile — Turn.CastFromExile
	condScaffoldExileLinkedReturn   // exile-until-leaves — ExileLinked/ReturnLinkedExile

	// Tier 1 audit additions — bridge structured AST condition Kinds that
	// the scaffold detector currently ignores. Each maps to existing engine
	// state (no new fields required); see Permanent.Flags["kicked"] for
	// kicker, Seat.Turn.* for did_prior_action, Permanent.Counters for ETB.
	condScaffoldPaidOptionalCost // Cond.Kind="paid_optional_cost" / kicker
	condScaffoldForEach          // Cond.Kind="for_each" / "for each X"
	condScaffoldETBAs            // Cond.Kind="etb_as"/"enters_with" / "enters with N counters"
	condScaffoldDidPriorAction   // Cond.Kind="did_prior_action" — generic prior-turn action

	// Tier 2B audit additions — five medium-priority scaffolds bridging
	// existing engine subsystems. Cycling fires via fireCyclingTriggers,
	// mutate uses Permanent.Flags["mutated"], doors/rooms use
	// Permanent.Flags["unlocked"], werewolf transform reads
	// Seat.SpellsCastLastTurn, and soulbond pairs via PairSoulbond.
	condScaffoldCycled              // "when you cycle" / cycle event
	condScaffoldMutates             // "whenever this creature mutates"
	condScaffoldUnlockDoor          // "unlock the door" / Duskmourn rooms
	condScaffoldPriorTurnSpellCount // "no spells were cast last turn" (Werewolf)
	condScaffoldPairedSoulbond      // "as long as ~ is paired" / soulbond

	// Tier 2A audit additions — four medium-priority scaffolds derived from
	// the Era 2 corpus audit. None require new engine fields:
	//   - TurnedFaceUp uses gameengine.TurnFaceUp + Card.FaceDown.
	//   - BeginningOfOrdinalStep uses gs.Phase / gs.Step.
	//   - TribeYouControlETB uses placeNamedFriendlyCreatureWithSubtype.
	//   - ManaSpentThreshold uses Permanent.Flags["mana_spent"] + Card.CMC
	//     (the engine's resolve.go currently default-trues mana_spent; the
	//     flag stamp is the durable signal once the predicate is tightened).
	condScaffoldTurnedFaceUp           // morph/megamorph/manifest/disguise turn-face-up trigger
	condScaffoldBeginningOfOrdinalStep // beginning of combat/draw/end/main step
	condScaffoldTribeYouControlETB     // "another <type> enters under your control"
	condScaffoldManaSpentThreshold     // "if N or more mana was spent to cast"

	// Era 4 Tier 1 — three highest-hit pre-Modern scaffold gaps.
	condScaffoldAnyPlayerPhase          // "each player's upkeep/end step" — fires for all seats
	condScaffoldDelayedDrawNextUpkeep   // "draw a card at the beginning of the next turn's upkeep"
	condScaffoldETBModalChoice          // "as ~ enters, choose a color/creature type/player"

	// Era 4 Tier 2 — four medium-hit pre-Modern trigger clusters that
	// the existing whitelist misses:
	//   - BecomesTapped: Insolence/Lifetap/Seizures (16 cards) — prime by
	//     placing a target permanent on battlefield and tapping it.
	//   - BecomesTarget: Tar Pit Warrior/Cephalid Illusionist (14 cards) —
	//     prime by simulating a spell-on-stack targeting srcPerm.
	//   - UntilEOTDelayed: Spiritualize/"next cleanup" auras (13 cards) —
	//     prime by advancing to the end / cleanup step.
	//   - LandPlayOrTap: Pangosaur/Storm Cauldron/Mana Web (11 cards) —
	//     prime by seeding lands on both seats and logging a land-play
	//     event. NOT landfall (that is controller-only via existing slug).
	condScaffoldBecomesTapped   // "becomes tapped" / "is tapped"
	condScaffoldBecomesTarget   // "becomes the target of a spell or ability"
	condScaffoldUntilEOTDelayed // "until end of turn" / "next cleanup step"
	condScaffoldLandPlayOrTap   // "whenever a player plays a land" / "tapped for mana"

	// Era 1 audit additions — 11 new structured-Kind scaffolds bridging
	// pre-Modern (1993-2014) gaps surfaced by scripts/era1_scaffold_audit.py.
	// Most reuse existing engine state and just need the Kind dispatched.
	condScaffoldETBTappedUnless           // "X enters tapped unless Y" — satisfy unless clause
	condScaffoldDomain                    // "domain" — control 4 different basic land types
	condScaffoldETBIf                     // "X enters under condition Y" — raw-text routed
	condScaffoldRepeatN                   // "do this N times" — set X / repeat counter
	condScaffoldLieutenant                // "as long as you control your commander"
	condScaffoldKiCountersGE2             // ki-counter threshold on source (Kamigawa flip)
	condScaffoldSelfIsTapped              // source is tapped
	condScaffoldAttackedOrBlockedCombat   // source attacked or blocked this combat (Clockwork)
	condScaffoldCoven                     // 3 creatures with different powers
	condScaffoldSelfHasCounter            // source has named counter on it
	condScaffoldDidntAttackThisTurn       // source / controller didn't attack this turn
	condScaffoldDealtDamageOpponentTurn   // dealt damage to an opponent this turn

	// Era 4 audit additions — bridge structured Kinds and raw-text fragments
	// surfaced by scripts/era4_scaffold_audit.py for the 2023-2026 corpus
	// (discover / descend / battles / prototype / craft / role tokens / the
	// ring). Each maps to existing engine state; the highest-hit raw clusters
	// are had-counters past-state, you-cast-from-hand, planeswalker/artifact
	// ETB-this-turn, and the explore/scry-style land-or-hand reveal pattern.
	condScaffoldItWasCreature             // structured: "it was a creature" post-death typecheck
	condScaffoldNoCreaturesOnBattlefield  // structured: "no creatures are on the battlefield"
	condScaffoldHadCountersOnIt           // raw: "it had (a/one or more/N) counter(s) on it"
	condScaffoldYouCastFromHand           // raw: "you cast it [from your hand]"
	condScaffoldPlaneswalkerETBThisTurn   // raw: "a planeswalker entered the battlefield under your control this turn"
	condScaffoldArtifactETBThisTurn       // raw: "an artifact entered the battlefield under your control this turn"
	condScaffoldStillOnBattlefield        // raw: "it's on the battlefield" / "if ~ is still on the battlefield"
	condScaffoldRevealLandOtherwiseHand   // raw: "if it's a land card, put it onto the battlefield. otherwise put it into your hand"

	// Era 2 audit additions — bridge raw-text fragments surfaced by
	// scripts/era2_scaffold_audit.py for the 2015-2019 corpus (crew /
	// vehicles / energy / amass / ascend / eminence / mutate). Each pattern
	// has low per-card frequency but the underlying mechanic is pervasive
	// across Kaladesh-era vehicles, Rivals of Ixalan ascend, Aetherdrift
	// velocity counters, and Ikoria mutate subtype lookups.
	condScaffoldVelocityCounters    // raw: "N or more velocity counters on it"
	condScaffoldNotDeclaredAttacker // raw: "isn't being declared as an attacker"
	condScaffoldManaValueLE         // raw: "its mana value is N or less"
	condScaffoldCrewedBySubtype     // raw: "a(n) <subtype> crewed it this turn"
	condScaffoldIsSubtype           // raw: "that creature is a(n) <subtype>"
	condScaffoldAscendBlessing      // raw: "city's blessing" / "if you have the city's blessing"
	condScaffoldEminenceCommandZone // raw: "in the command zone" eminence-style trigger

	// Era 3 audit additions — 13 new scaffolds bridging 2020-2022 gaps and
	// the parallel "recent printings" (2025+) cluster surfaced by
	// data/thor-audit-era3-recent.md. Each maps to existing engine state
	// (no new fields required); see ManaSpentThreshold + Permanent.Flags
	// for the per-source stamping pattern.
	condScaffoldControlNCreatures       // "you control N or more creatures"
	condScaffoldControlNLands           // "you control N or more lands"
	condScaffoldCounterReplacementBoost // "+1/+1 counter doubler" replacement (Doubling Season etc)
	condScaffoldTokenReplacementBoost   // "create that many plus one tokens instead"
	condScaffoldPersistUndyingCheck     // "if it had no +1/+1 [or -1/-1] counters"
	condScaffoldCardTypeReveal          // "if it's a land/creature/permanent card" reveal
	condScaffoldEquipmentAttached       // "as long as ~ is equipped" / "equipped creature"
	condScaffoldHandSizeThreshold       // "fewer than N cards in hand" / "N or more cards in hand"
	condScaffoldPutCounterThisTurn      // "you put a counter on ~ this turn"
	condScaffoldCastNSpellsThisTurn     // count variant: "cast N or more spells this turn"
	condScaffoldFullParty               // Zendikar Rising full party (4 distinct classes)
	condScaffoldTotalToughness          // "creatures you control have total toughness N+"
	condScaffoldMainPhaseOrFirstCombat  // "it's your main phase" / "first combat phase"

	// Era 1 R60 sweep — 19 new scaffolds bridging the dominant pre-Modern
	// raw-text gaps surfaced by scripts/era1_scaffold_audit.py. Each maps to
	// existing engine state (no new fields required); the top clusters are
	// tribute (KTK)/bargain (WOE) alt-cost gates, "you cast it from hand",
	// colored-mana-spent ({R}{R}/{G}{G}/{U}{U} cast-cost predicates),
	// life-comparison forms (you-more / opp-more / above-starting), suspend /
	// time-counter checks, and per-source state predicates (untapped /
	// no-counter / wasnt-cast / wasnt-blocking / damage-taken).
	condScaffoldTributeNotPaid           // "tribute wasn't paid" — KTK tribute alt-cost
	condScaffoldBargained                // "it was bargained" — WOE bargain alt-cost
	condScaffoldSharesCreatureType       // "if it shares a creature type with ~" tribal-reveal
	condScaffoldNotTheirTurn             // "it's not their turn" — off-turn predicate
	condScaffoldMoreLifeThanOpponent     // "you have more life than an opponent" / inverse
	condScaffoldTimeCounterOnSelf        // "had no time counters on it" — suspend post-state
	condScaffoldWasntCast                // "it wasn't cast" — token / put-onto-battlefield path
	condScaffoldLostLifeLastTurn         // "you lost life last turn" — last-turn life delta
	condScaffoldDamageDealtToSelfTurn    // "N or more damage was dealt to it this turn"
	condScaffoldMoreCardsThanOpponents   // "you have more cards in hand than each opponent"
	condScaffoldSelfIsUntapped           // "this creature is untapped" — mirror of self_is_tapped
	condScaffoldSelfHasNoCounter         // "has no <kind> counters on it" — negative counter check
	condScaffoldSurgeCostPaid            // "its surge cost was paid" — EMN surge alt-cost
	condScaffoldLibraryEmpty             // "your library has no cards in it" — Jace win-con
	condScaffoldColoredManaSpent         // "{R}{R}/{G}{G}/{U}{U}/{R} was spent to cast it"
	condScaffoldSelfIsSuspended          // "this card is suspended" — exile suspend state
	condScaffoldLifeAboveStarting        // "life total is greater than your starting life total"
	condScaffoldOpponentMoreLife         // "an opponent has more life than you"
	condScaffoldWasntBlocking            // "it wasn't blocking" — combat exit predicate

	// Era 3 sweep r60 — second pass at the 2025+ recent-printings cluster
	// surfaced by data/thor-audit-era3-recent.md. Each scaffold maps to
	// existing engine state (Turn counters, Permanent.Counters, Seat
	// flags) — no new gameengine fields required.
	condScaffoldDiscardedNonland         // "you discarded a nonland card" (Spider-Verse / Doom et al)
	condScaffoldControlNTappedCreatures  // "you control N or more tapped creatures" (EoE Pilots)
	condScaffoldGainedNLifeThisTurn      // count variant: "you gained N or more life this turn"
	condScaffoldDrawnNCardsThisTurn      // count variant: "you've drawn N or more cards this turn"
	condScaffoldStartingPlayer           // "you were the starting player" / "weren't"
	condScaffoldQuestCounters            // "it has N or more quest counters on it" (TLA Ascension)
	condScaffoldDragonBeheld             // "a Dragon was beheld" (Tarkir Dragonstorm)
)

type conditionScaffold struct {
	kind        conditionScaffoldKind
	description string // populated lazily by apply
	rawText     string

	// Shape-specific payload.
	subtype   string // for condScaffoldYouControlSubtype: e.g. "wizard"
	count     int    // for graveyard count / energy threshold
	threshold int    // for life threshold conditions
}

var (
	rawTribalRe   = regexp.MustCompile(`(?:another|a|an)\s+([a-z]+)`)
	energyAmtRe   = regexp.MustCompile(`(\d+)\s+or\s+more\s+energy`)
	graveCntRe    = regexp.MustCompile(`(?:(\d+)|one|two|three|four|five|six|seven)\s*(?:or\s+more\s+)?creature\s+cards?`)
	lifeAboveRe   = regexp.MustCompile(`(?:you have|your life total is)\s+(\d+)\s+or\s+more\s+life`)
	lifeBelowRe   = regexp.MustCompile(`(?:you have|your life total is)\s+(\d+)\s+or\s+(?:less|fewer)\s+life`)
	lifeBelowAltRe = regexp.MustCompile(`life total is\s+(\d+)\s+or\s+less`)
	// CastRecord nth-spell pattern — "second spell", "third creature", etc.
	secondSpellRe = regexp.MustCompile(`(?:second|third)\s+(?:spell|creature|noncreature|instant|sorcery|artifact|enchantment)`)
	// Ability-word fingerprints. Each accepts either the ability word
	// itself or the canonical English description so we catch both AST
	// forms.
	deliriumRe       = regexp.MustCompile(`four\s+or\s+more\s+card\s+types`)
	spellMasteryRe   = regexp.MustCompile(`(?:two|2)\s+or\s+more\s+(?:instant|sorcery)`)
	metalcraftRe     = regexp.MustCompile(`(?:three|3)\s+or\s+more\s+artifacts?`)
	ferociousRe      = regexp.MustCompile(`creature\s+with\s+power\s+(?:four|4)\s+or\s+(?:more|greater)`)
	formidableRe     = regexp.MustCompile(`total\s+power\s+(?:eight|8)\s+or\s+(?:more|greater)`)
	// Tier 1 audit patterns.
	kickerWasPaidRe  = regexp.MustCompile(`(?:was kicked|kicker (?:cost )?was paid|if (?:it|this) was kicked)`)
	multikickerRe    = regexp.MustCompile(`for each time .* (?:was )?kicked|multikicker`)
	forEachRe        = regexp.MustCompile(`for (?:each|every)\s+([a-z]+)`)
	entersWithCntrRe = regexp.MustCompile(`enters?(?:\s+the\s+battlefield)?\s+with\s+(\d+|a|an|one|two|three|four)\s+([+\-]\d+/[+\-]\d+|[a-z]+)\s+counters?`)
	entersAsRe       = regexp.MustCompile(`(?:as\s+~|as\s+\w[\w\s]*?)\s+enters(?:\s+the\s+battlefield)?,\s*(?:choose|you may choose)`)
	// did_prior_action verb scanner — order matters: most specific first.
	priorActionVerbRe = regexp.MustCompile(`(attacked|cast (?:a |an )?(?:noncreature |creature )?spell|cast a spell|sacrificed|(?:a )?creature died|gained life|drew (?:a )?card|discarded|played (?:a )?land|dealt damage)`)

	// Tier 2A patterns.
	// "another <type> enters the battlefield under your control"
	// "another <type> you control enters"
	// "a <type> you control enters the battlefield"
	tribeETBRe = regexp.MustCompile(
		`(?:another\s+([a-z]+)\s+(?:creature\s+)?(?:enters?|is\s+put\s+onto)(?:[^\.]*?under your control|[^\.]*?you control)|` +
			`(?:a|an)\s+([a-z]+)\s+(?:creature\s+)?you\s+control\s+(?:enters?))`)
	// "if N or more mana was spent to cast" / "N mana was spent" / "X or more mana"
	// Tolerates the curly-brace mana-symbol form ("{5} or more mana").
	manaSpentNumRe = regexp.MustCompile(`\{?(\d+)\}?\s+or\s+more\s+(?:[a-z]+\s+)?mana\s+(?:was\s+)?(?:spent|paid)`)
	// "mana value of ~ is N or greater"
	manaValueGtRe = regexp.MustCompile(`mana\s+value\s+of\s+\S+\s+is\s+(\d+)\s+or\s+(?:greater|more)`)

	// Era 2 raw-text patterns. manaValueLERe captures N from "its mana
	// value is N or less"; crewedBySubtypeRe captures the pilot subtype;
	// isSubtypeRe captures the target subtype from "that creature is a
	// <subtype>".
	manaValueLERe     = regexp.MustCompile(`(?:its\s+)?mana\s+value\s+(?:of\s+\S+\s+)?is\s+(\d+)\s+or\s+(?:less|fewer)`)
	crewedBySubtypeRe = regexp.MustCompile(`(?:an?\s+)([a-z]+)\s+crewed\s+(?:it|this\s+vehicle)\s+this\s+turn`)
	isSubtypeRe       = regexp.MustCompile(`that\s+creature\s+is\s+(?:an?\s+)?([a-z]+)\b`)
)

func conditionRawText(cond *gameast.Condition) string {
	if cond == nil || len(cond.Args) == 0 {
		return ""
	}
	s, _ := cond.Args[0].(string)
	return strings.ToLower(strings.TrimSpace(s))
}

// detectConditionScaffold examines a raw-text condition and returns the
// shape that scaffolding should match. The shape is then handed to apply
// or describe — both share this detection so the trace text and the
// mutation always agree.
func detectConditionScaffold(cond *gameast.Condition) conditionScaffold {
	if cond == nil {
		return conditionScaffold{}
	}
	kind := strings.ToLower(cond.Kind)

	// Tier 1 audit additions — structured AST kinds the existing whitelist
	// dropped on the floor. Detect these BEFORE the whitelist filter
	// because they don't use intervening_if/as_long_as packaging.
	switch kind {
	case "paid_optional_cost", "was_kicked":
		// was_kicked is the empty-args canonical form of paid_optional_cost.
		return detectPaidOptionalCost(cond)
	case "for_each":
		return detectForEach(cond)
	case "etb_as", "enters_as", "enters_with":
		return detectETBAs(cond)
	case "did_prior_action":
		return detectDidPriorAction(cond)
	case "mana_spent", "no_mana_spent_to_cast":
		out := conditionScaffold{kind: condScaffoldManaSpentThreshold, count: 4}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out

	// Era 1 audit additions — direct Kind dispatch. Each routes to either
	// an existing scaffold (canonical short-circuit) or a new shape below.
	case "hellbent":
		return conditionScaffold{kind: condScaffoldHellbent}
	case "raid", "attacked_this_turn", "you_attacked_this_turn":
		// "you_attacked_this_turn" is the AST kind emitted by the parser
		// for "if you attacked this turn" / "if ~ attacked you this turn"
		// gates (Trynn, Nightsquad Commando, raid-style EOT triggers).
		// evalCondition reads it directly via CheckRaid; the scaffold
		// just needs to flip the same flag CheckRaid consults.
		return conditionScaffold{kind: condScaffoldAttackedThisTurn}
	case "spell_mastery":
		return conditionScaffold{kind: condScaffoldSpellMastery}
	case "gained_life_this_turn":
		return conditionScaffold{kind: condScaffoldGainedLifeThisTurn}
	case "creature_died_this_turn":
		return conditionScaffold{kind: condScaffoldCreatureDiedThisTurn}
	case "no_spells_cast_last_turn":
		return conditionScaffold{kind: condScaffoldPriorTurnSpellCount, count: 0}
	case "two_plus_spells_cast_last_turn":
		return conditionScaffold{kind: condScaffoldPriorTurnSpellCount, count: 2}
	case "you_control_creature_power_ge":
		out := conditionScaffold{kind: condScaffoldFerocious, count: 4}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "etb_tapped_unless":
		out := conditionScaffold{kind: condScaffoldETBTappedUnless}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.rawText = strings.ToLower(strings.TrimSpace(s))
			}
		}
		return out
	case "domain":
		return conditionScaffold{kind: condScaffoldDomain, count: 4}
	case "etb_if":
		// Reuse the raw-text matcher on args[0] so the existing patterns
		// (you-cast-from-hand, no-mana-spent, etc.) can satisfy the
		// precondition. Falls through to a generic ETB flag if nothing
		// matches.
		out := conditionScaffold{kind: condScaffoldETBIf}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.rawText = strings.ToLower(strings.TrimSpace(s))
			}
		}
		return out
	case "repeat_n":
		out := conditionScaffold{kind: condScaffoldRepeatN, count: 3}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "lieutenant":
		return conditionScaffold{kind: condScaffoldLieutenant}
	case "ki_counters_ge_2":
		return conditionScaffold{kind: condScaffoldKiCountersGE2, count: 2}
	case "self_is_tapped":
		return conditionScaffold{kind: condScaffoldSelfIsTapped}
	case "attacked_or_blocked_this_combat":
		return conditionScaffold{kind: condScaffoldAttackedOrBlockedCombat}
	case "coven":
		return conditionScaffold{kind: condScaffoldCoven}
	case "self_has_counter":
		out := conditionScaffold{kind: condScaffoldSelfHasCounter, subtype: "+1/+1", count: 1}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.subtype = strings.ToLower(strings.TrimSpace(s))
			}
		}
		return out
	case "didnt_attack_this_turn":
		return conditionScaffold{kind: condScaffoldDidntAttackThisTurn}
	case "dealt_damage_to_opponent_this_turn":
		return conditionScaffold{kind: condScaffoldDealtDamageOpponentTurn}

	// Era 4 audit — structured Kinds the existing whitelist drops on the
	// floor. landfall + you_descended_this_turn route to the existing
	// flag-based scaffolds; it_was_a_creature / no_creatures_on_battlefield
	// need new shapes (post-death typecheck / board-empty check).
	case "landfall":
		return conditionScaffold{kind: condScaffoldLandfallThisTurn}
	case "you_descended_this_turn":
		return conditionScaffold{kind: condScaffoldDescendedThisTurn}
	case "it_was_a_creature":
		return conditionScaffold{kind: condScaffoldItWasCreature}
	case "no_creatures_on_battlefield":
		return conditionScaffold{kind: condScaffoldNoCreaturesOnBattlefield}

	// Era 1 R60 sweep — structured-Kind dispatch for the 19 new scaffolds.
	// Most arrive as raw/intervening_if (handled by the text patterns below),
	// but the parser occasionally surfaces them as named Kinds; route here so
	// either shape resolves the same scaffold.
	case "tribute_not_paid", "tribute_wasnt_paid":
		return conditionScaffold{kind: condScaffoldTributeNotPaid}
	case "bargained", "was_bargained":
		return conditionScaffold{kind: condScaffoldBargained}
	case "shares_creature_type":
		out := conditionScaffold{kind: condScaffoldSharesCreatureType, subtype: "elf"}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.subtype = strings.ToLower(strings.TrimSpace(s))
			}
		}
		return out
	case "not_their_turn", "not_your_turn":
		return conditionScaffold{kind: condScaffoldNotTheirTurn}
	case "more_life_than_opponent", "you_more_life":
		return conditionScaffold{kind: condScaffoldMoreLifeThanOpponent}
	case "no_time_counters", "time_counter_on_self":
		return conditionScaffold{kind: condScaffoldTimeCounterOnSelf}
	case "wasnt_cast":
		return conditionScaffold{kind: condScaffoldWasntCast}
	case "lost_life_last_turn":
		return conditionScaffold{kind: condScaffoldLostLifeLastTurn}
	case "damage_dealt_to_self_this_turn", "damage_taken_this_turn":
		out := conditionScaffold{kind: condScaffoldDamageDealtToSelfTurn, count: 4}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "more_cards_in_hand_than_opponents", "more_cards_than_each_opponent":
		return conditionScaffold{kind: condScaffoldMoreCardsThanOpponents}
	case "self_is_untapped":
		return conditionScaffold{kind: condScaffoldSelfIsUntapped}
	case "self_has_no_counter", "no_named_counter":
		out := conditionScaffold{kind: condScaffoldSelfHasNoCounter, subtype: "oil"}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.subtype = strings.ToLower(strings.TrimSpace(s))
			}
		}
		return out
	case "surge_cost_paid":
		return conditionScaffold{kind: condScaffoldSurgeCostPaid}
	case "library_empty":
		return conditionScaffold{kind: condScaffoldLibraryEmpty}
	case "colored_mana_spent":
		out := conditionScaffold{kind: condScaffoldColoredManaSpent, subtype: "R", count: 1}
		if len(cond.Args) > 0 {
			if s, ok := cond.Args[0].(string); ok {
				out.subtype = strings.ToUpper(strings.TrimSpace(s))
			}
		}
		return out
	case "self_is_suspended":
		return conditionScaffold{kind: condScaffoldSelfIsSuspended}
	case "life_above_starting":
		return conditionScaffold{kind: condScaffoldLifeAboveStarting}
	case "opponent_more_life":
		return conditionScaffold{kind: condScaffoldOpponentMoreLife}
	case "wasnt_blocking":
		return conditionScaffold{kind: condScaffoldWasntBlocking}

	// Era 3 r48 scaffold wiring — the prior sweep added enum constants
	// and apply cases but never wired structured-Kind detection. Route
	// the AST Kinds the audit script's BUCKETED_KINDS set already lists
	// (control_n_creatures, hand_size_*, full_party, …) to their apply
	// paths so the dead code goes live.
	case "control_n_creatures", "you_control_n_creatures":
		out := conditionScaffold{kind: condScaffoldControlNCreatures, count: 3}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "control_n_lands", "you_control_n_lands":
		out := conditionScaffold{kind: condScaffoldControlNLands, count: 5}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "counter_doubler", "counter_replacement_boost":
		return conditionScaffold{kind: condScaffoldCounterReplacementBoost}
	case "token_doubler", "token_replacement_boost":
		return conditionScaffold{kind: condScaffoldTokenReplacementBoost}
	case "persist_check", "undying_check", "no_counters":
		out := conditionScaffold{kind: condScaffoldPersistUndyingCheck, subtype: "+1/+1"}
		for _, a := range cond.Args {
			if s, ok := a.(string); ok {
				ls := strings.ToLower(strings.TrimSpace(s))
				if strings.Contains(ls, "-1/-1") {
					out.subtype = "-1/-1"
				} else if strings.Contains(ls, "+1/+1") {
					out.subtype = "+1/+1"
				}
			}
		}
		return out
	case "card_type_reveal", "revealed_card_type":
		out := conditionScaffold{kind: condScaffoldCardTypeReveal, subtype: "creature"}
		for _, a := range cond.Args {
			if s, ok := a.(string); ok {
				ls := strings.ToLower(strings.TrimSpace(s))
				if ls != "" && !isGenericWord(ls) {
					out.subtype = ls
				} else if ls != "" {
					out.subtype = ls
				}
			}
		}
		return out
	case "equipped", "equipment_attached":
		return conditionScaffold{kind: condScaffoldEquipmentAttached}
	case "hand_size_le", "hand_size_lt", "hand_size_ge", "hand_size_threshold":
		out := conditionScaffold{kind: condScaffoldHandSizeThreshold, subtype: kind, count: 7}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n >= 0 {
				out.count = n
			}
		}
		return out
	case "put_counter_this_turn":
		return conditionScaffold{kind: condScaffoldPutCounterThisTurn}
	case "cast_n_spells_this_turn":
		out := conditionScaffold{kind: condScaffoldCastNSpellsThisTurn, count: 2}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "full_party":
		return conditionScaffold{kind: condScaffoldFullParty}
	case "total_toughness":
		out := conditionScaffold{kind: condScaffoldTotalToughness, count: 8}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "main_phase":
		return conditionScaffold{kind: condScaffoldMainPhaseOrFirstCombat, subtype: "main_phase"}
	case "first_combat_phase":
		return conditionScaffold{kind: condScaffoldMainPhaseOrFirstCombat, subtype: "first_combat_phase"}

	// Era 3 sweep r60 — new structured-Kind dispatch for clusters that
	// fell out of the r48 batch.
	case "discarded_nonland":
		return conditionScaffold{kind: condScaffoldDiscardedNonland}
	case "control_n_tapped_creatures":
		out := conditionScaffold{kind: condScaffoldControlNTappedCreatures, count: 2}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "gained_n_life_this_turn":
		out := conditionScaffold{kind: condScaffoldGainedNLifeThisTurn, count: 3}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "drawn_n_cards_this_turn":
		out := conditionScaffold{kind: condScaffoldDrawnNCardsThisTurn, count: 2}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "starting_player", "you_were_starting_player":
		out := conditionScaffold{kind: condScaffoldStartingPlayer, subtype: "starting"}
		for _, a := range cond.Args {
			if s, ok := a.(string); ok {
				ls := strings.ToLower(strings.TrimSpace(s))
				if strings.Contains(ls, "weren't") || strings.Contains(ls, "not") {
					out.subtype = "not_starting"
				}
			}
		}
		return out
	case "quest_counters":
		out := conditionScaffold{kind: condScaffoldQuestCounters, count: 4}
		for _, a := range cond.Args {
			if n, ok := a.(int); ok && n > 0 {
				out.count = n
			}
		}
		return out
	case "dragon_beheld":
		return conditionScaffold{kind: condScaffoldDragonBeheld}
	}

	switch kind {
	case "intervening_if", "as_long_as", "conditional", "raw", "if":
		// proceed — "if" is the structurally-bare wrapper the parser emits
		// when a clause starts with "if ..." and the predicate isn't one
		// of the canonical Kinds. Treat it like "raw": route through the
		// text-pattern detector below.
	default:
		return conditionScaffold{}
	}
	txt := conditionRawText(cond)
	if txt == "" {
		return conditionScaffold{}
	}
	cs := conditionScaffold{rawText: txt}

	// Tier 1: text-form fallbacks for the same patterns when AST emitted a
	// raw/intervening_if instead of the structured Kind.
	if kickerWasPaidRe.MatchString(txt) || multikickerRe.MatchString(txt) {
		out := conditionScaffold{kind: condScaffoldPaidOptionalCost, rawText: txt, count: 1}
		if multikickerRe.MatchString(txt) {
			out.count = 2
		}
		return out
	}
	if forEachRe.MatchString(txt) {
		return parseForEachText(txt)
	}
	// Era 4 — ETBModalChoice: "as ~ enters, choose a color/creature
	// type/player/card name". Distinguished from generic ETBAs (counter
	// placement) by requiring an explicit "choose" + category noun. Must
	// come BEFORE the generic ETBAs catch so "enters...choose" doesn't
	// collapse into the counter-placement path.
	if strings.Contains(txt, "enters") &&
		(strings.Contains(txt, "choose") || strings.Contains(txt, "choosing")) &&
		(strings.Contains(txt, "color") || strings.Contains(txt, "creature type") ||
			strings.Contains(txt, "player") || strings.Contains(txt, "card name") ||
			strings.Contains(txt, "basic land type") || strings.Contains(txt, "a type")) {
		cs.kind = condScaffoldETBModalChoice
		cs.subtype = parseETBChoiceCategory(txt)
		return cs
	}
	if strings.Contains(txt, "enters") &&
		(strings.Contains(txt, " with ") || strings.Contains(txt, " as ") ||
			strings.Contains(txt, "choose") || strings.Contains(txt, "choosing")) {
		return parseETBAsText(txt)
	}

	// Era 4 Tier 2 detection — placed HIGH in the chain because all four
	// patterns are highly specific (no false-positive overlap with later
	// matchers) and several of them ("becomes tapped", "until end of turn,
	// whenever", "plays a land") sit inside texts that downstream matchers
	// would otherwise grab first (e.g. "enchanted creature becomes tapped"
	// would be eaten by EnchantedCreature; "you played a land this turn,
	// draw a card" by DrawnCardThisTurn). Order within this block: most
	// specific first.

	// Era 4 Tier 2 — BecomesTapped: Insolence, Lifetap, Seizures, Relic
	// Bind. Anchor on present-tense "becomes/is tapped" so we don't
	// collide with "tap target permanent" effect text.
	if strings.Contains(txt, "becomes tapped") ||
		strings.Contains(txt, "becomes the tapped") ||
		(strings.Contains(txt, " is tapped") && !strings.Contains(txt, "is tapped for mana") &&
			!strings.Contains(txt, "as long as")) {
		cs.kind = condScaffoldBecomesTapped
		return cs
	}

	// Era 4 Tier 2 — BecomesTarget: Tar Pit Warrior, Cephalid Illusionist,
	// Cursed Monstrosity, Skulking Fugitive. "becomes the target" is
	// unique to this trigger shape; BecomesTargetByAlly is a strict subset.
	if strings.Contains(txt, "becomes the target") ||
		strings.Contains(txt, "becomes a target of") {
		cs.kind = condScaffoldBecomesTarget
		return cs
	}

	// Era 4 Tier 2 — UntilEOTDelayed: Spiritualize, Bubbling Muck,
	// "next cleanup step" auras. "until end of turn" alone is too broad
	// (every P/T pump uses it), so we anchor on "whenever"/"delayed"
	// pairing or the explicit cleanup phrase.
	if (strings.Contains(txt, "until end of turn") &&
		(strings.Contains(txt, "whenever") || strings.Contains(txt, "delayed"))) ||
		strings.Contains(txt, "next cleanup step") ||
		strings.Contains(txt, "beginning of the next cleanup") {
		cs.kind = condScaffoldUntilEOTDelayed
		return cs
	}

	// Era 4 Tier 2 — LandPlayOrTap: Pangosaur, Storm Cauldron, Mana Web.
	// Anchored on present-tense "plays a land" (no "this turn" qualifier
	// — that's controller-only landfall) or "tapped for mana".
	if (strings.Contains(txt, "plays a land") &&
		!strings.Contains(txt, "this turn")) ||
		strings.Contains(txt, "tapped for mana") {
		cs.kind = condScaffoldLandPlayOrTap
		cs.subtype = "any_player"
		if strings.Contains(txt, "opponent") {
			cs.subtype = "opponent"
		}
		return cs
	}

	// "an opponent controls more lands than you" / "more lands than you do"
	if (strings.Contains(txt, "more land") || strings.Contains(txt, "controls more")) &&
		strings.Contains(txt, "than you") {
		cs.kind = condScaffoldOpponentMoreLands
		return cs
	}

	// "if/as long as a creature died this turn"
	if strings.Contains(txt, "died this turn") {
		cs.kind = condScaffoldCreatureDiedThisTurn
		return cs
	}

	// Delirium — 4+ distinct card types in your graveyard. Must come
	// before the generic graveyard matchers below so the more permissive
	// CardInGraveyard case doesn't swallow it.
	if strings.Contains(txt, "delirium") || deliriumRe.MatchString(txt) {
		cs.kind = condScaffoldDelirium
		return cs
	}

	// Spell Mastery — 2+ instant/sorcery in your graveyard. Same
	// ordering rationale as Delirium.
	if strings.Contains(txt, "spell mastery") ||
		(spellMasteryRe.MatchString(txt) && strings.Contains(txt, "graveyard")) {
		cs.kind = condScaffoldSpellMastery
		return cs
	}

	// Graveyard count: "four or more creature cards in your graveyard"
	if strings.Contains(txt, "graveyard") &&
		(strings.Contains(txt, "creature card") || strings.Contains(txt, "creatures in")) {
		cs.kind = condScaffoldCreatureCardsInGraveyard
		cs.count = parseGraveyardCount(txt)
		if cs.count < 4 {
			cs.count = 4
		}
		return cs
	}

	// Generic "card in your graveyard" — Necromancy, return-from-graveyard.
	if strings.Contains(txt, "graveyard") {
		cs.kind = condScaffoldCardInGraveyard
		return cs
	}

	// Energy counter threshold ("if you have N or more energy counters").
	if strings.Contains(txt, "energy") {
		n := 30
		if m := energyAmtRe.FindStringSubmatch(txt); len(m) > 1 {
			if v, err := strconv.Atoi(m[1]); err == nil && v > 0 {
				n = v
			}
		}
		cs.kind = condScaffoldEnergyThreshold
		cs.count = n
		return cs
	}

	// Life gained this turn. Sorin, Lathiel, Heliod.
	if (strings.Contains(txt, "gained life") || strings.Contains(txt, "gain life")) &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldGainedLifeThisTurn
		return cs
	}

	// Cast a spell this turn. Monastery Mentor, Prowess-adjacent.
	if (strings.Contains(txt, "cast a spell") || strings.Contains(txt, "cast a noncreature") ||
		strings.Contains(txt, "cast an instant") || strings.Contains(txt, "you cast")) &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldCastSpellThisTurn
		return cs
	}

	// Creature ETB this turn. "if a creature entered the battlefield"
	if (strings.Contains(txt, "creature entered") || strings.Contains(txt, "creature enters")) &&
		(strings.Contains(txt, "this turn") || strings.Contains(txt, "battlefield")) {
		cs.kind = condScaffoldCreatureETBThisTurn
		return cs
	}

	// Drew a card this turn. "if you've drawn a card this turn"
	if (strings.Contains(txt, "drew a card") || strings.Contains(txt, "drawn a card") ||
		strings.Contains(txt, "draw a card")) &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldDrawnCardThisTurn
		return cs
	}

	// Attacked this turn. "if you attacked this turn" / "if you attacked with a creature"
	if (strings.Contains(txt, "attacked this turn") || strings.Contains(txt, "you attacked") ||
		strings.Contains(txt, "creature attacked")) {
		cs.kind = condScaffoldAttackedThisTurn
		return cs
	}

	// Sacrificed this turn. "if you sacrificed a creature this turn"
	if strings.Contains(txt, "sacrific") && strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldSacrificedThisTurn
		return cs
	}

	// Combat damage dealt this turn. "if a creature dealt combat damage"
	if strings.Contains(txt, "combat damage") &&
		(strings.Contains(txt, "this turn") || strings.Contains(txt, "to a player") ||
			strings.Contains(txt, "dealt")) {
		cs.kind = condScaffoldCombatDamageDealt
		return cs
	}

	// Landfall. "if a land entered the battlefield" / "landfall" / "if you played a land"
	if strings.Contains(txt, "landfall") ||
		(strings.Contains(txt, "land") && strings.Contains(txt, "entered")) ||
		(strings.Contains(txt, "played a land") && strings.Contains(txt, "this turn")) {
		cs.kind = condScaffoldLandfallThisTurn
		return cs
	}

	// Discarded this turn. "if you discarded a card this turn"
	if strings.Contains(txt, "discard") && strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldDiscardedThisTurn
		return cs
	}

	// Enchanted creature. "enchanted creature" condition for auras.
	if strings.Contains(txt, "enchanted creature") {
		cs.kind = condScaffoldEnchantedCreature
		return cs
	}

	// Opponent lost life this turn. "if an opponent lost life this turn"
	if strings.Contains(txt, "opponent") &&
		(strings.Contains(txt, "lost life") || strings.Contains(txt, "lose life") ||
			strings.Contains(txt, "dealt damage")) &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldOpponentLostLife
		return cs
	}

	// Life above threshold. "if you have 25 or more life"
	if m := lifeAboveRe.FindStringSubmatch(txt); m != nil {
		n, _ := strconv.Atoi(m[1])
		cs.kind = condScaffoldLifeAboveThreshold
		cs.threshold = n
		return cs
	}

	// Life below threshold. "if you have 5 or less life" / "your life total is 5 or less"
	if m := lifeBelowRe.FindStringSubmatch(txt); m != nil {
		n, _ := strconv.Atoi(m[1])
		cs.kind = condScaffoldLifeBelowThreshold
		cs.threshold = n
		return cs
	}
	if m := lifeBelowAltRe.FindStringSubmatch(txt); m != nil {
		n, _ := strconv.Atoi(m[1])
		cs.kind = condScaffoldLifeBelowThreshold
		cs.threshold = n
		return cs
	}

	// Era 4 — AnyPlayerPhase: "each player's upkeep/end step" fires for
	// all seats, not just the active player. Must come BEFORE the generic
	// upkeep catch-all so "each player's upkeep" doesn't collapse to
	// condScaffoldUpkeepPhase.
	if (strings.Contains(txt, "each player") || strings.Contains(txt, "each opponent")) &&
		(strings.Contains(txt, "upkeep") || strings.Contains(txt, "end step")) {
		cs.kind = condScaffoldAnyPlayerPhase
		if strings.Contains(txt, "end step") {
			cs.subtype = "end_step"
		} else {
			cs.subtype = "upkeep"
		}
		return cs
	}

	// Era 4 — DelayedDrawNextUpkeep: "draw a card at the beginning of
	// the next turn's upkeep" — Mirage/Visions delayed draw pattern.
	// Must come BEFORE generic upkeep catch-all.
	if strings.Contains(txt, "next turn") && strings.Contains(txt, "upkeep") &&
		(strings.Contains(txt, "draw") || strings.Contains(txt, "next turn's upkeep")) {
		cs.kind = condScaffoldDelayedDrawNextUpkeep
		return cs
	}

	// Upkeep phase condition. "during your upkeep" / "it's your upkeep"
	if strings.Contains(txt, "upkeep") {
		cs.kind = condScaffoldUpkeepPhase
		return cs
	}

	// Hellbent — "if you have no cards in hand" / ability word.
	if strings.Contains(txt, "hellbent") ||
		(strings.Contains(txt, "no cards in") && strings.Contains(txt, "hand")) {
		cs.kind = condScaffoldHellbent
		return cs
	}

	// Monarch — "if you're the monarch" / "you are the monarch".
	if strings.Contains(txt, "the monarch") ||
		strings.Contains(txt, "you're the monarch") ||
		strings.Contains(txt, "you are the monarch") {
		cs.kind = condScaffoldMonarch
		return cs
	}

	// Initiative — "if you have the initiative".
	if strings.Contains(txt, "the initiative") ||
		strings.Contains(txt, "have initiative") {
		cs.kind = condScaffoldInitiative
		return cs
	}

	// Revolt — "a permanent you controlled left the battlefield this turn".
	if strings.Contains(txt, "revolt") ||
		(strings.Contains(txt, "permanent") &&
			strings.Contains(txt, "left the battlefield") &&
			strings.Contains(txt, "this turn")) {
		cs.kind = condScaffoldRevolt
		return cs
	}

	// Metalcraft — "if you control three or more artifacts".
	if strings.Contains(txt, "metalcraft") ||
		(metalcraftRe.MatchString(txt) && strings.Contains(txt, "you control")) {
		cs.kind = condScaffoldMetalcraft
		return cs
	}

	// Ferocious — "if you control a creature with power 4 or greater".
	if strings.Contains(txt, "ferocious") || ferociousRe.MatchString(txt) {
		cs.kind = condScaffoldFerocious
		return cs
	}

	// Formidable — "creatures you control have total power 8 or greater".
	if strings.Contains(txt, "formidable") || formidableRe.MatchString(txt) {
		cs.kind = condScaffoldFormidable
		return cs
	}

	// Permanent left the battlefield. Disappear / Void ability words and
	// revolt-like conditions that don't use the "revolt" keyword.
	if (strings.Contains(txt, "permanent left") || strings.Contains(txt, "permanent left the battlefield")) &&
		!strings.Contains(txt, "revolt") {
		cs.kind = condScaffoldPermanentLeftBF
		return cs
	}

	// Second/third spell this turn. CastRecord-dependent conditions.
	if secondSpellRe.MatchString(txt) {
		cs.kind = condScaffoldSecondSpellThisTurn
		return cs
	}

	// Descended this turn. Ixalan keyword.
	if strings.Contains(txt, "descended") || strings.Contains(txt, "descend") {
		cs.kind = condScaffoldDescendedThisTurn
		return cs
	}

	// Life lost this turn (self or opponent).
	if (strings.Contains(txt, "lost life") || strings.Contains(txt, "lose life")) &&
		strings.Contains(txt, "this turn") &&
		!strings.Contains(txt, "opponent") {
		cs.kind = condScaffoldLifeLostThisTurn
		return cs
	}

	// Tokens created count. Thalisse / Vazi / Ellyn Harbreeze.
	if strings.Contains(txt, "tokens") &&
		(strings.Contains(txt, "created this turn") || strings.Contains(txt, "you created this turn")) {
		cs.kind = condScaffoldTokensCreatedCount
		return cs
	}

	// Cast from exile. Impulse draw and flashback conditions.
	if strings.Contains(txt, "cast") && strings.Contains(txt, "from exile") &&
		!strings.Contains(txt, "you may cast") {
		cs.kind = condScaffoldCastFromExile
		return cs
	}

	// Exile-linked return. O-Ring / Fiend Hunter patterns.
	if strings.Contains(txt, "exiled with") &&
		(strings.Contains(txt, "return") || strings.Contains(txt, "leaves")) {
		cs.kind = condScaffoldExileLinkedReturn
		return cs
	}

	// Tier 2B — Cycled. "when you cycle" / "whenever you cycle" / explicit
	// "cycling" trigger. Engine fires via fireCyclingTriggers; we just need
	// the controller to be visible as having cycled a card.
	if (strings.Contains(txt, "cycle") || strings.Contains(txt, "cycling")) &&
		!strings.Contains(txt, "cycled this way") {
		cs.kind = condScaffoldCycled
		return cs
	}

	// Tier 2B — Mutates. Mutate-trigger events on the source creature itself.
	if strings.Contains(txt, "mutate") {
		cs.kind = condScaffoldMutates
		return cs
	}

	// Tier 2B — Unlock door. Duskmourn enchantment-room "when you unlock".
	// Excludes generic "lock" usage that isn't door-related.
	if (strings.Contains(txt, "unlock") || strings.Contains(txt, "unlocked")) &&
		(strings.Contains(txt, "door") || strings.Contains(txt, "this room") ||
			strings.Contains(txt, "this enchantment")) {
		cs.kind = condScaffoldUnlockDoor
		return cs
	}

	// Tier 2B — Prior-turn spell count. Werewolf transform conditions and
	// other "if/as long as ... last turn" spell-count predicates.
	if strings.Contains(txt, "last turn") &&
		(strings.Contains(txt, "no spells were cast") ||
			strings.Contains(txt, "no spell was cast") ||
			strings.Contains(txt, "cast two or more spells") ||
			(strings.Contains(txt, "cast") && strings.Contains(txt, "spells") &&
				(strings.Contains(txt, "two or more") || strings.Contains(txt, "or more")))) {
		cs.kind = condScaffoldPriorTurnSpellCount
		// Encode the variant in count: 0 = no spells, 2 = 2+ spells.
		if strings.Contains(txt, "no spell") {
			cs.count = 0
		} else {
			cs.count = 2
		}
		return cs
	}

	// Tier 2B — Soulbond / paired check. "as long as ~ is paired" /
	// "when ~ is paired" / "soulbond" ability word in a conditional clause.
	if strings.Contains(txt, "soulbond") ||
		(strings.Contains(txt, "paired") &&
			(strings.Contains(txt, "is paired") || strings.Contains(txt, "are paired") ||
				strings.Contains(txt, "creature is paired"))) {
		cs.kind = condScaffoldPairedSoulbond
		return cs
	}

	// Tier 2A — Turned face up. Morph / megamorph / manifest / disguise
	// triggers all surface as "when[ever] ~ is turned face up". The phrase
	// "turned face up" is the canonical fingerprint; we also catch the bare
	// "manifest" and "disguise" cost-payment phrasing when paired with a
	// trigger word so we don't false-match noun-form occurrences.
	if strings.Contains(txt, "turned face up") ||
		(strings.Contains(txt, "manifest") &&
			(strings.Contains(txt, "is turned") || strings.Contains(txt, "becomes turned"))) ||
		(strings.Contains(txt, "disguise") &&
			(strings.Contains(txt, "is turned") || strings.Contains(txt, "becomes turned"))) ||
		(strings.Contains(txt, "megamorph") && strings.Contains(txt, "face up")) {
		cs.kind = condScaffoldTurnedFaceUp
		return cs
	}

	// Tier 2A — Beginning-of <step> trigger condition for steps the
	// existing UpkeepPhase scaffold doesn't cover: combat, draw, end,
	// pre/postcombat main. We park the resolved step name in cs.subtype.
	if strings.Contains(txt, "beginning of") && !strings.Contains(txt, "beginning of upkeep") {
		switch {
		case strings.Contains(txt, "beginning of combat") ||
			strings.Contains(txt, "beginning of each combat"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "combat"
			return cs
		case strings.Contains(txt, "beginning of") && strings.Contains(txt, "draw step"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "draw"
			return cs
		case strings.Contains(txt, "beginning of") && strings.Contains(txt, "end step"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "end_step"
			return cs
		case strings.Contains(txt, "beginning of your second main phase") ||
			strings.Contains(txt, "beginning of your postcombat main") ||
			strings.Contains(txt, "beginning of the postcombat main"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "postcombat_main"
			return cs
		case strings.Contains(txt, "beginning of your precombat main") ||
			strings.Contains(txt, "beginning of the precombat main") ||
			strings.Contains(txt, "beginning of your first main phase"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "precombat_main"
			return cs
		case strings.Contains(txt, "beginning of") && strings.Contains(txt, "untap step"):
			cs.kind = condScaffoldBeginningOfOrdinalStep
			cs.subtype = "untap"
			return cs
		}
	}

	// Tier 2A — Tribe-ETB ("whenever another <type> enters under your
	// control" / "whenever a <type> you control enters"). Must come BEFORE
	// the catch-all YouControlSubtype tribal matcher so that ETB-keyed
	// scaffolding is preferred over the static "you control" form.
	if (strings.Contains(txt, "enters") || strings.Contains(txt, "is put onto")) &&
		(strings.Contains(txt, "under your control") || strings.Contains(txt, "you control")) {
		if m := tribeETBRe.FindStringSubmatch(txt); m != nil {
			subtype := m[1]
			if subtype == "" {
				subtype = m[2]
			}
			if subtype != "" && !isGenericWord(subtype) {
				cs.kind = condScaffoldTribeYouControlETB
				cs.subtype = subtype
				return cs
			}
		}
	}

	// Tier 2A — Mana-spent / mana-value threshold. Maelstrom Archangel
	// ("if {5} or more mana was spent to cast"), Boros Reckoner ("amount
	// of mana spent"), Phyrexian Dreadnought-style "mana value greater".
	if (strings.Contains(txt, "mana was spent") ||
		strings.Contains(txt, "mana was paid") ||
		strings.Contains(txt, "amount of mana spent") ||
		strings.Contains(txt, "amount of mana paid")) ||
		manaValueGtRe.MatchString(txt) {
		cs.kind = condScaffoldManaSpentThreshold
		// Default count, overridden by parsed numerics below.
		cs.count = 4
		if m := manaSpentNumRe.FindStringSubmatch(txt); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				cs.count = n
			}
		} else if m := manaValueGtRe.FindStringSubmatch(txt); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				cs.count = n
			}
		}
		return cs
	}

	// Era 4 raw-text matchers — placed before the catch-all "you control"
	// tribal so the more specific clauses win. Order within this block:
	// most specific phrase first.

	// "a planeswalker entered the battlefield under your control this turn"
	// Oath of Liliana / Oath of Chandra-style planeswalker-ETB conditions.
	if (strings.Contains(txt, "planeswalker") &&
		(strings.Contains(txt, "entered the battlefield") ||
			strings.Contains(txt, "enters the battlefield")) &&
		strings.Contains(txt, "this turn")) ||
		(strings.Contains(txt, "planeswalker") && strings.Contains(txt, "you've cast") && strings.Contains(txt, "this turn")) {
		cs.kind = condScaffoldPlaneswalkerETBThisTurn
		return cs
	}

	// "as long as an artifact entered the battlefield under your control
	// this turn" — Mechan Shieldmate, Shipwreck Sentry.
	if strings.Contains(txt, "artifact") &&
		(strings.Contains(txt, "entered the battlefield") || strings.Contains(txt, "enters the battlefield")) &&
		strings.Contains(txt, "this turn") &&
		(strings.Contains(txt, "under your control") || strings.Contains(txt, "you control")) {
		cs.kind = condScaffoldArtifactETBThisTurn
		return cs
	}

	// "if it's a land card, put it onto the battlefield. otherwise, …"
	// Coiling Oracle / Skyward Eye Prophets / Nadu reveal-and-route. Anchor
	// on the dual phrasing so we don't match generic land tutors.
	if strings.Contains(txt, "if it's a land card") &&
		(strings.Contains(txt, "onto the battlefield") || strings.Contains(txt, "put it onto")) &&
		strings.Contains(txt, "otherwise") {
		cs.kind = condScaffoldRevealLandOtherwiseHand
		return cs
	}

	// "it had (a/one or more/N) counter(s) on it" — Ozolith, Angelic Sleuth,
	// Resourceful Defense, Nikara, Leader's Talent. Past-state counter check
	// fired on death/leaves-battlefield triggers.
	if (strings.Contains(txt, "had counters on it") ||
		strings.Contains(txt, "had a counter on it") ||
		strings.Contains(txt, "had a +1/+1 counter") ||
		strings.Contains(txt, "had one or more counters") ||
		strings.Contains(txt, "had any counters on it") ||
		strings.Contains(txt, "had a death counter")) &&
		!strings.Contains(txt, "ki counter") {
		cs.kind = condScaffoldHadCountersOnIt
		return cs
	}

	// "you cast it from your hand" / "you cast it" (cast-from-hand check on
	// the source itself — Wild Pair, Feasting Troll King, Yathan Roadwatcher,
	// Bringer of the Last Gift). Distinguish from generic "you cast a spell
	// this turn" which is already handled by CastSpellThisTurn.
	if (strings.Contains(txt, "you cast it from your hand") ||
		strings.Contains(txt, "you cast it from a graveyard") ||
		(strings.Contains(txt, "you cast it") && !strings.Contains(txt, "you cast it from"))) &&
		!strings.Contains(txt, "you cast a spell") {
		cs.kind = condScaffoldYouCastFromHand
		return cs
	}

	// "it's on the battlefield" / "is still on the battlefield" — Hex,
	// Stalking Yeti, Auratouched Mage. Static still-on-bf precondition.
	if (strings.Contains(txt, "it's on the battlefield") ||
		strings.Contains(txt, "is still on the battlefield")) &&
		!strings.Contains(txt, "isn't on the battlefield") &&
		!strings.Contains(txt, "is not on the battlefield") {
		cs.kind = condScaffoldStillOnBattlefield
		return cs
	}

	// Era 1 R60 sweep — raw-text matchers for the 19 pre-Modern scaffolds.
	// Ordered most-specific first; phrases that overlap downstream matchers
	// (e.g. "life total" / "lost life" / "cards in hand") are pinned here so
	// the broader life/hand/cast catches don't eat them.

	// Tribute (KTK) — "tribute wasn't paid" / "if tribute was paid". Anchor
	// on the explicit "tribute" noun so we don't false-match Tributary Sage
	// flavor text. Cards: Ornitharch, Nessian Demolok, Pharagax Giant.
	if strings.Contains(txt, "tribute wasn't paid") ||
		strings.Contains(txt, "tribute was paid") ||
		(strings.Contains(txt, "tribute") &&
			(strings.Contains(txt, "wasn't paid") || strings.Contains(txt, "was paid"))) {
		cs.kind = condScaffoldTributeNotPaid
		return cs
	}

	// Bargain (WOE) — "it was bargained". Trigger fires when the source was
	// cast with bargain. Cards: Troublemaker Ouphe, Realm-Scorcher Hellkite,
	// High Fae Negotiator.
	if strings.Contains(txt, "it was bargained") ||
		strings.Contains(txt, "if it was bargained") ||
		strings.Contains(txt, "this spell was bargained") {
		cs.kind = condScaffoldBargained
		return cs
	}

	// Shares creature type — "if it shares a creature type with this
	// creature". Tribal-reveal pattern: Sensation Gorger, Ink Dissolver,
	// Squeaking Pie Grubfellows, Leaf-Crowned Elder, Wandering Graybeard.
	// Anchor on the full noun phrase so generic "shares a type" doesn't
	// false-match.
	if strings.Contains(txt, "shares a creature type") {
		cs.kind = condScaffoldSharesCreatureType
		cs.subtype = "goblin"
		// Try to lift a subtype hint from the surrounding tribal clause.
		if m := rawTribalRe.FindStringSubmatch(txt); len(m) > 1 && !isGenericWord(m[1]) {
			cs.subtype = m[1]
		}
		return cs
	}

	// Suspend post-state — "had no time counters on it" (Deadly Grub,
	// Chronozoa). Pre-empts the generic counter / had-counters matcher
	// because the engine reads the "time" counter via the suspend zone, not
	// via the +1/+1 path.
	if strings.Contains(txt, "no time counters") ||
		strings.Contains(txt, "had no time counters") {
		cs.kind = condScaffoldTimeCounterOnSelf
		return cs
	}

	// Suspended state — "this card is suspended" (Deep-Sea Kraken, Nihilith).
	// Distinct from the above (post-state vs current-state).
	if strings.Contains(txt, "this card is suspended") ||
		strings.Contains(txt, "while it's suspended") ||
		strings.Contains(txt, "while suspended") {
		cs.kind = condScaffoldSelfIsSuspended
		return cs
	}

	// "you cast it from your hand" already matched above by YouCastFromHand.
	// This block catches "it wasn't cast" — the inverse predicate that fires
	// on token / put-onto-battlefield paths (Rapid Augmenter, Preston, the
	// Vanisher, Mirror of Life Trapping, Anti-Venom).
	if strings.Contains(txt, "it wasn't cast") ||
		strings.Contains(txt, "wasn't cast") ||
		strings.Contains(txt, "he wasn't cast") ||
		strings.Contains(txt, "she wasn't cast") ||
		strings.Contains(txt, "they weren't cast") {
		cs.kind = condScaffoldWasntCast
		return cs
	}

	// "it wasn't blocking" — combat exit predicate (Guildsworn Prowler).
	if strings.Contains(txt, "it wasn't blocking") ||
		strings.Contains(txt, "wasn't blocking") ||
		strings.Contains(txt, "wasn't being blocked by") {
		cs.kind = condScaffoldWasntBlocking
		return cs
	}

	// Surge (EMN) — "if its surge cost was paid" (Reckless Bushwhacker,
	// Tyrant of Valakut). Conceptual alt-cost cousin of kicker; we keep it
	// as a distinct scaffold so the trace shows the actual mechanic.
	if strings.Contains(txt, "surge cost was paid") ||
		strings.Contains(txt, "its surge cost was paid") ||
		strings.Contains(txt, "if surge cost was paid") {
		cs.kind = condScaffoldSurgeCostPaid
		return cs
	}

	// Library empty — "your library has no cards in it" (Jace, Wielder of
	// Mysteries; Stillness in Motion). Win-con / mill predicate. Anchor on
	// "no cards" + "library" so we don't match "your library is empty"
	// metaphor in flavor text.
	if (strings.Contains(txt, "no cards in") && strings.Contains(txt, "library")) ||
		strings.Contains(txt, "library has no cards") ||
		strings.Contains(txt, "library is empty") {
		cs.kind = condScaffoldLibraryEmpty
		return cs
	}

	// Colored mana spent to cast — "{R}{R}/{G}{G}/{U}{U}/{R} was spent to
	// cast it" (Vibrance, Catharsis, Wistfulness, Deceit, Gruul Scrapper,
	// Steamcore Weird). Distinct from generic mana_spent because we need
	// a specific colored payment, not just CMC≥N.
	if (strings.Contains(txt, "was spent to cast") ||
		strings.Contains(txt, "spent to cast it")) &&
		(strings.Contains(txt, "{r}") || strings.Contains(txt, "{g}") ||
			strings.Contains(txt, "{u}") || strings.Contains(txt, "{b}") ||
			strings.Contains(txt, "{w}") || strings.Contains(txt, "{c}") ||
			strings.Contains(txt, "mana from a treasure")) {
		cs.kind = condScaffoldColoredManaSpent
		switch {
		case strings.Contains(txt, "{r}{r}"):
			cs.subtype = "RR"
			cs.count = 2
		case strings.Contains(txt, "{g}{g}"):
			cs.subtype = "GG"
			cs.count = 2
		case strings.Contains(txt, "{u}{u}"):
			cs.subtype = "UU"
			cs.count = 2
		case strings.Contains(txt, "{b}{b}"):
			cs.subtype = "BB"
			cs.count = 2
		case strings.Contains(txt, "{w}{w}"):
			cs.subtype = "WW"
			cs.count = 2
		case strings.Contains(txt, "mana from a treasure"):
			cs.subtype = "treasure"
			cs.count = 1
		case strings.Contains(txt, "{r}"):
			cs.subtype = "R"
			cs.count = 1
		case strings.Contains(txt, "{g}"):
			cs.subtype = "G"
			cs.count = 1
		case strings.Contains(txt, "{u}"):
			cs.subtype = "U"
			cs.count = 1
		case strings.Contains(txt, "{b}"):
			cs.subtype = "B"
			cs.count = 1
		case strings.Contains(txt, "{w}"):
			cs.subtype = "W"
			cs.count = 1
		default:
			cs.subtype = "C"
			cs.count = 1
		}
		return cs
	}

	// Lost life last turn — "you lost life last turn" (First Response,
	// Paladin of Atonement). Distinct from LifeLostThisTurn: the prior-turn
	// snapshot lives on Seat.Flags, not Turn.LifeLost.
	if strings.Contains(txt, "lost life last turn") ||
		(strings.Contains(txt, "last turn") &&
			(strings.Contains(txt, "lost life") || strings.Contains(txt, "lose life"))) {
		cs.kind = condScaffoldLostLifeLastTurn
		return cs
	}

	// Damage dealt to source this turn — "N or more damage was dealt to it
	// this turn" (Burning-Eye Zubera, Rushing-Tide Zubera). Per-source
	// MarkedDamage / damage_taken counter.
	if (strings.Contains(txt, "damage was dealt to it") ||
		strings.Contains(txt, "damage was dealt to this") ||
		strings.Contains(txt, "damage dealt to it")) &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldDamageDealtToSelfTurn
		cs.count = 4
		if m := manaSpentNumRe.FindStringSubmatch(txt); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				cs.count = n
			}
		}
		return cs
	}

	// More cards in hand than each opponent — "you have more cards in hand
	// than each opponent" (Death of a Thousand Stings, Exile into Darkness,
	// Akuta, Born of Ash). Hand-size comparison predicate.
	if strings.Contains(txt, "more cards in hand than each opponent") ||
		(strings.Contains(txt, "more cards in") && strings.Contains(txt, "hand") &&
			(strings.Contains(txt, "than each") || strings.Contains(txt, "than any"))) {
		cs.kind = condScaffoldMoreCardsThanOpponents
		return cs
	}

	// Source untapped — "this creature is untapped" (Sphinx Sovereign, Nim
	// Abomination). Mirror of self_is_tapped; the source must be UNTAPPED
	// for the ability to qualify.
	if (strings.Contains(txt, "this creature is untapped") ||
		strings.Contains(txt, "~ is untapped") ||
		(strings.Contains(txt, "is untapped") &&
			(strings.Contains(txt, "this creature") ||
				strings.Contains(txt, "this artifact") ||
				strings.Contains(txt, "this permanent")))) &&
		!strings.Contains(txt, "as long as ~ is untapped, players can't") {
		cs.kind = condScaffoldSelfIsUntapped
		return cs
	}

	// Source has NO named counter — "it has no oil counters on it" /
	// "no loyalty counters" (Evolved Spinoderm, Archfiend of the Dross,
	// Ozolith-style negative checks). Inverse of self_has_counter.
	if (strings.Contains(txt, "has no") || strings.Contains(txt, "had no")) &&
		strings.Contains(txt, "counters on it") {
		cs.kind = condScaffoldSelfHasNoCounter
		cs.subtype = "oil"
		// Try to extract the named counter kind ("no oil", "no -1/-1", etc).
		for _, k := range []string{"oil", "loyalty", "+1/+1", "-1/-1", "stun",
			"charge", "quest", "fade", "vanishing", "shield", "ki", "blood",
			"divinity", "time", "verse", "wage", "wish"} {
			if strings.Contains(txt, "no "+k+" counter") {
				cs.subtype = k
				break
			}
		}
		return cs
	}

	// Life-above-starting — "your life total is greater than your starting
	// life total" (Cosmos Elixir, A-Cosmos Elixir). Distinct from
	// LifeAboveThreshold because the threshold itself is dynamic (40 for
	// Commander, 20 otherwise).
	if (strings.Contains(txt, "greater than your starting life total") ||
		strings.Contains(txt, "more than your starting life total") ||
		strings.Contains(txt, "above your starting life total")) {
		cs.kind = condScaffoldLifeAboveStarting
		return cs
	}

	// Opponent has more life than you — "an opponent has more life than you"
	// (Linvala, the Preserver; Pulse of the Fields). Pre-empts the broader
	// MoreLifeThanOpponent matcher (which is the inverse direction).
	if (strings.Contains(txt, "opponent has more life than you") ||
		strings.Contains(txt, "any opponent has more life") ||
		strings.Contains(txt, "an opponent has more life") ||
		(strings.Contains(txt, "opponent") && strings.Contains(txt, "more life than you"))) {
		cs.kind = condScaffoldOpponentMoreLife
		return cs
	}

	// "you have more life than an opponent" / "no opponent has more life
	// than that player" (Glorious Enforcer, Feudkiller's Verdict, Survival
	// Cache, Veteran Soldier). Mirror of the above. Must come AFTER the
	// opponent-more-life matcher so the inverse direction wins.
	if (strings.Contains(txt, "more life than an opponent") ||
		strings.Contains(txt, "more life than each opponent") ||
		strings.Contains(txt, "no opponent has more life than that player") ||
		strings.Contains(txt, "no opponent has more life than you")) {
		cs.kind = condScaffoldMoreLifeThanOpponent
		return cs
	}

	// Off-turn predicate — "it's not their turn" (Glademuse, Scytheclaw
	// Raptor, Adrenaline Jockey). Trigger fires only outside the listed
	// player's own turn. Anchor on the exact phrase so we don't false-match
	// "their turn" inside unrelated clauses.
	if strings.Contains(txt, "not their turn") ||
		strings.Contains(txt, "not your turn") ||
		strings.Contains(txt, "isn't their turn") ||
		strings.Contains(txt, "during another player's turn") ||
		strings.Contains(txt, "on a turn that isn't yours") {
		cs.kind = condScaffoldNotTheirTurn
		return cs
	}

	// Era 2 raw-text matchers — anchored on Era 2-specific keywords so they
	// don't false-match the broader corpus. Each clause is rare per card but
	// the mechanic cluster matters for vehicle/crew/mutate/eminence
	// regression coverage.

	// "N or more velocity counters on it" — Aetherdrift racing vehicles.
	if strings.Contains(txt, "velocity counter") {
		cs.kind = condScaffoldVelocityCounters
		cs.count = 2
		return cs
	}

	// "isn't being declared as an attacker" — Rhoda, Geist Avenger and
	// other anti-attack triggers. Distinct from didnt_attack_this_turn:
	// this fires DURING declare-attackers, not at end-of-turn.
	if strings.Contains(txt, "isn't being declared as an attacker") ||
		strings.Contains(txt, "is not being declared as an attacker") ||
		strings.Contains(txt, "not declared as an attacker") {
		cs.kind = condScaffoldNotDeclaredAttacker
		return cs
	}

	// "its mana value is N or less" — Amped Raptor, Thunderous Velocipede,
	// cascade-style cost filters.
	if m := manaValueLERe.FindStringSubmatch(txt); m != nil {
		cs.kind = condScaffoldManaValueLE
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cs.count = n
		} else {
			cs.count = 4
		}
		return cs
	}

	// "a(n) <subtype> crewed it this turn" — Adrestia and other Kaladesh
	// vehicles with subtype-gated crew triggers. Capture the subtype so
	// the priming can place a matching pilot.
	if m := crewedBySubtypeRe.FindStringSubmatch(txt); m != nil {
		subtype := strings.TrimSpace(m[1])
		if subtype != "" && !isGenericWord(subtype) {
			cs.kind = condScaffoldCrewedBySubtype
			cs.subtype = subtype
			return cs
		}
	}

	// "that creature is a(n) <subtype>" — Turtle Van / mutate creature-type
	// gates. Anchor on the explicit copula so we don't match generic
	// "creature with subtype" clauses.
	if m := isSubtypeRe.FindStringSubmatch(txt); m != nil {
		subtype := strings.TrimSpace(m[1])
		if subtype != "" && !isGenericWord(subtype) {
			cs.kind = condScaffoldIsSubtype
			cs.subtype = subtype
			return cs
		}
	}

	// "city's blessing" — Rivals of Ixalan ascend. The existing primeAscend
	// helper handles the engine side; the scaffold dispatch just routes the
	// clause so it gets bucketed rather than dropped.
	if strings.Contains(txt, "city's blessing") ||
		strings.Contains(txt, "have ascended") {
		cs.kind = condScaffoldAscendBlessing
		return cs
	}

	// "in the command zone" / "from the command zone" — eminence triggers
	// (Kynaios, Edgar Markov, Inalla, The Ur-Dragon). Static-zone check on
	// the commander itself.
	if strings.Contains(txt, "in the command zone") ||
		strings.Contains(txt, "from the command zone") {
		cs.kind = condScaffoldEminenceCommandZone
		return cs
	}

	// Era 3 r48 raw-text wiring + r60 new clusters. Each clause appears
	// in data/thor-audit-era3-recent.md ≥3×. Order: most specific phrase
	// first so the catch-all YouControlSubtype matcher doesn't eat
	// "you control N or more creatures" with an empty subtype.

	// Counter-doubler replacement — "one or more (+1/+1)? counters would
	// be put on …". Distinct from PutCounterThisTurn (after-the-fact
	// stamping) — this is a replacement-effect predicate.
	if strings.Contains(txt, "counters would be put on") ||
		strings.Contains(txt, "counter would be put on") ||
		strings.Contains(txt, "one or more counters would") ||
		strings.Contains(txt, "one or more +1/+1 counters would") {
		cs.kind = condScaffoldCounterReplacementBoost
		return cs
	}
	// Token-doubler replacement — "one or more tokens would be created".
	if strings.Contains(txt, "tokens would be created") ||
		strings.Contains(txt, "token would be created") ||
		strings.Contains(txt, "one or more tokens would") {
		cs.kind = condScaffoldTokenReplacementBoost
		return cs
	}
	// Persist/undying — "if it had no +1/+1 / -1/-1 counters on it".
	// Anchored on the "had no … counters" phrasing so we don't collide
	// with HadCountersOnIt (the inverse / counted form).
	if strings.Contains(txt, "had no -1/-1 counters") ||
		strings.Contains(txt, "had no +1/+1 counters") ||
		strings.Contains(txt, "had no counters on it") {
		cs.kind = condScaffoldPersistUndyingCheck
		if strings.Contains(txt, "-1/-1") {
			cs.subtype = "-1/-1"
		} else {
			cs.subtype = "+1/+1"
		}
		return cs
	}
	// CardTypeReveal — "if it's a (land|creature|permanent|instant|sorcery
	// |artifact|enchantment) card". Anchored on the "it's a … card" copula
	// to avoid eating generic typeline clauses.
	if (strings.Contains(txt, "it's a land card") ||
		strings.Contains(txt, "it's a creature card") ||
		strings.Contains(txt, "it's a permanent card") ||
		strings.Contains(txt, "it's an instant card") ||
		strings.Contains(txt, "it's a sorcery card") ||
		strings.Contains(txt, "it's an artifact card") ||
		strings.Contains(txt, "it's an enchantment card") ||
		strings.Contains(txt, "it's a planeswalker card")) ||
		(strings.Contains(txt, "is a") && strings.Contains(txt, "card") &&
			(strings.Contains(txt, "land card") || strings.Contains(txt, "creature card") ||
				strings.Contains(txt, "permanent card"))) {
		cs.kind = condScaffoldCardTypeReveal
		switch {
		case strings.Contains(txt, "land card"):
			cs.subtype = "land"
		case strings.Contains(txt, "permanent card"):
			cs.subtype = "permanent"
		case strings.Contains(txt, "creature card"):
			cs.subtype = "creature"
		case strings.Contains(txt, "instant card"):
			cs.subtype = "instant"
		case strings.Contains(txt, "sorcery card"):
			cs.subtype = "sorcery"
		case strings.Contains(txt, "artifact card"):
			cs.subtype = "artifact"
		case strings.Contains(txt, "enchantment card"):
			cs.subtype = "enchantment"
		case strings.Contains(txt, "planeswalker card"):
			cs.subtype = "planeswalker"
		default:
			cs.subtype = "creature"
		}
		return cs
	}
	// EquipmentAttached — "as long as ~ is equipped" / "equipped creature
	// is a …" / "this Equipment is attached to a creature" / "Cloud is
	// equipped". Anchor on the "is equipped" / "is attached to a creature"
	// phrasings so we don't grab generic "equip" cost text.
	if strings.Contains(txt, " is equipped") ||
		strings.Contains(txt, "equipped creature is") ||
		strings.Contains(txt, "while equipped") ||
		strings.Contains(txt, "is attached to a creature") {
		cs.kind = condScaffoldEquipmentAttached
		return cs
	}
	// HandSizeThreshold — "fewer than N cards in hand" / "N or more cards
	// in hand" / "N or fewer cards in hand".
	if (strings.Contains(txt, "cards in") && strings.Contains(txt, "hand")) &&
		(strings.Contains(txt, "fewer than") || strings.Contains(txt, "or more") ||
			strings.Contains(txt, "or fewer") || strings.Contains(txt, "no more than")) {
		cs.kind = condScaffoldHandSizeThreshold
		cs.count = 4
		switch {
		case strings.Contains(txt, "fewer than"):
			cs.subtype = "hand_size_lt"
		case strings.Contains(txt, "or fewer"), strings.Contains(txt, "no more than"):
			cs.subtype = "hand_size_le"
		case strings.Contains(txt, "or more"):
			cs.subtype = "hand_size_ge"
		default:
			cs.subtype = "hand_size_threshold"
		}
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// PutCounterThisTurn — "you put a counter on … this turn".
	if (strings.Contains(txt, "put a counter on") ||
		strings.Contains(txt, "put a +1/+1 counter on")) &&
		strings.Contains(txt, "this turn") &&
		strings.Contains(txt, "you") {
		cs.kind = condScaffoldPutCounterThisTurn
		return cs
	}
	// CastNSpellsThisTurn — "you've cast N or more spells this turn".
	// Must come BEFORE generic CastSpellThisTurn matchers in the chain
	// above; we rely on running last (this block is below those).
	if (strings.Contains(txt, "you've cast") || strings.Contains(txt, "you have cast")) &&
		strings.Contains(txt, "this turn") &&
		(strings.Contains(txt, "spells") || strings.Contains(txt, "noncreature spells")) &&
		(strings.Contains(txt, "or more") || strings.Contains(txt, "at least")) {
		cs.kind = condScaffoldCastNSpellsThisTurn
		cs.count = 2
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// FullParty — Zendikar Rising 4-class party.
	if strings.Contains(txt, "full party") {
		cs.kind = condScaffoldFullParty
		return cs
	}
	// TotalToughness — "creatures you control have total toughness N+".
	if strings.Contains(txt, "total toughness") {
		cs.kind = condScaffoldTotalToughness
		cs.count = 8
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// MainPhase / FirstCombatPhase. Order: first-combat-phase before main
	// so "it's the first combat phase" doesn't get eaten by the main test.
	if strings.Contains(txt, "first combat phase") {
		cs.kind = condScaffoldMainPhaseOrFirstCombat
		cs.subtype = "first_combat_phase"
		return cs
	}
	if strings.Contains(txt, "your main phase") || strings.Contains(txt, "it's your main") {
		cs.kind = condScaffoldMainPhaseOrFirstCombat
		cs.subtype = "main_phase"
		return cs
	}

	// Era 3 r60 — new clusters.

	// DiscardedNonland — Spider-Verse / Doctor Doom-style: "you discarded
	// a nonland card". The bare "nonland" qualifier distinguishes from the
	// generic DiscardedThisTurn matcher in the era 1 block above.
	if strings.Contains(txt, "discarded a nonland") ||
		(strings.Contains(txt, "discarded") && strings.Contains(txt, "nonland card")) {
		cs.kind = condScaffoldDiscardedNonland
		return cs
	}
	// ControlNTappedCreatures — EoE Pilots: "you control N or more tapped
	// creatures". Must run before the generic ControlNCreatures match so
	// the more-specific "tapped" predicate wins.
	if strings.Contains(txt, "tapped creature") &&
		(strings.Contains(txt, "you control") || strings.Contains(txt, "control")) &&
		strings.Contains(txt, "or more") {
		cs.kind = condScaffoldControlNTappedCreatures
		cs.count = 2
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// ControlNCreatures / ControlNLands — the r48 raw patterns. Generic
	// "you control" tail eats these if we don't claim first.
	if strings.Contains(txt, "you control") && strings.Contains(txt, "or more") &&
		(strings.Contains(txt, "land") && !strings.Contains(txt, "landfall")) {
		cs.kind = condScaffoldControlNLands
		cs.count = 5
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	if strings.Contains(txt, "you control") && strings.Contains(txt, "or more") &&
		strings.Contains(txt, "creature") {
		cs.kind = condScaffoldControlNCreatures
		cs.count = 3
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// DrawnNCardsThisTurn — "you've drawn N or more cards this turn"
	// (Avatar TLA). Count variant of DrawnCardThisTurn.
	if (strings.Contains(txt, "you've drawn") || strings.Contains(txt, "you have drawn")) &&
		strings.Contains(txt, "or more") && strings.Contains(txt, "cards") &&
		strings.Contains(txt, "this turn") {
		cs.kind = condScaffoldDrawnNCardsThisTurn
		cs.count = 2
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// GainedNLifeThisTurn — "you gained N or more life this turn".
	if strings.Contains(txt, "gained") && strings.Contains(txt, "life this turn") &&
		(strings.Contains(txt, "or more") || strings.Contains(txt, "at least")) {
		cs.kind = condScaffoldGainedNLifeThisTurn
		cs.count = 3
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// StartingPlayer — "you were/weren't the starting player".
	if strings.Contains(txt, "starting player") {
		cs.kind = condScaffoldStartingPlayer
		cs.subtype = "starting"
		if strings.Contains(txt, "weren't") || strings.Contains(txt, "were not") {
			cs.subtype = "not_starting"
		}
		return cs
	}
	// QuestCounters — Avatar TLA Ascension: "it has N or more quest
	// counters on it". Distinguish from generic SelfHasCounter by the
	// explicit "quest counter" subtype.
	if strings.Contains(txt, "quest counter") {
		cs.kind = condScaffoldQuestCounters
		cs.count = 4
		if n := parseFirstSpelledInt(txt); n > 0 {
			cs.count = n
		}
		return cs
	}
	// DragonBeheld — Tarkir Dragonstorm new mechanic: "a Dragon was beheld".
	if strings.Contains(txt, "dragon was beheld") ||
		strings.Contains(txt, "was beheld") {
		cs.kind = condScaffoldDragonBeheld
		return cs
	}

	// Tribal: "if you control another <subtype>" / "if you control a <subtype>".
	// Run last because it's the most permissive matcher.
	if strings.Contains(txt, "you control") {
		// Strip the leading "you control" segment so we match the subtype.
		idx := strings.Index(txt, "you control")
		tail := txt[idx+len("you control"):]
		if m := rawTribalRe.FindStringSubmatch(strings.TrimSpace(tail)); len(m) > 1 {
			subtype := m[1]
			// Filter out generic words that happen to follow "another"/"a".
			if !isGenericWord(subtype) {
				cs.kind = condScaffoldYouControlSubtype
				cs.subtype = subtype
				return cs
			}
		}
	}

	return conditionScaffold{}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func nonEmpty(a, fallback string) string {
	if a == "" {
		return fallback
	}
	return a
}

func isGenericWord(s string) bool {
	switch s {
	case "creature", "permanent", "card", "spell", "ability",
		"player", "opponent", "thing", "land", "artifact",
		"enchantment", "planeswalker":
		return true
	}
	return false
}

// detectPaidOptionalCost handles structured Cond.Kind="paid_optional_cost"
// emitted by the AST for kicker / additional-cost gates. Args[0] (when
// present) is the human-readable cost description; we sniff it for
// "kicked" / multikicker so applyConditionScaffolding can set the right
// count on Permanent.Flags["kicked"].
func detectPaidOptionalCost(cond *gameast.Condition) conditionScaffold {
	cs := conditionScaffold{kind: condScaffoldPaidOptionalCost, count: 1}
	if cond == nil {
		return cs
	}
	if len(cond.Args) > 0 {
		if s, ok := cond.Args[0].(string); ok {
			cs.rawText = strings.ToLower(strings.TrimSpace(s))
			if multikickerRe.MatchString(cs.rawText) {
				cs.count = 2
			}
			cs.subtype = cs.rawText
		}
	}
	return cs
}

// detectForEach handles structured Cond.Kind="for_each". Args[0] is the
// counted thing ("creature you control", "artifact", "opponent", etc).
// We extract the noun via parseForEachText and stash it in subtype so the
// apply step seeds the right type of permanent.
func detectForEach(cond *gameast.Condition) conditionScaffold {
	if cond == nil {
		return conditionScaffold{kind: condScaffoldForEach, subtype: "creature", count: 3}
	}
	txt := ""
	if len(cond.Args) > 0 {
		if s, ok := cond.Args[0].(string); ok {
			txt = strings.ToLower(strings.TrimSpace(s))
		}
	}
	if txt == "" {
		return conditionScaffold{kind: condScaffoldForEach, subtype: "creature", count: 3}
	}
	cs := parseForEachText("for each " + txt)
	cs.rawText = txt
	return cs
}

// parseForEachText extracts the counted noun from a "for each X" phrase.
// Default count is 3 (engine-side checks for `>= 1` are common, but 3
// covers `>= 2`/`>= 3` thresholds without overshooting).
func parseForEachText(txt string) conditionScaffold {
	cs := conditionScaffold{kind: condScaffoldForEach, count: 3, rawText: txt, subtype: "creature"}
	m := forEachRe.FindStringSubmatch(txt)
	if len(m) >= 2 {
		noun := m[1]
		// Trim trailing 's' so plurals normalize.
		if len(noun) > 3 && strings.HasSuffix(noun, "s") {
			noun = strings.TrimSuffix(noun, "s")
		}
		switch noun {
		case "creature", "land", "artifact", "enchantment", "planeswalker", "permanent":
			cs.subtype = noun
		case "opponent":
			cs.subtype = "opponent" // no-op apply: game already has opponents
		default:
			// Treat unknown nouns as creature subtype tokens
			// ("for each goblin you control") so we still satisfy the count.
			if !isGenericWord(noun) {
				cs.subtype = noun
			}
		}
	}
	return cs
}

// detectETBAs handles structured Cond.Kind="etb_as" / "enters_as" /
// "enters_with". Args[0] (when present) describes the modal payload:
// "with N +1/+1 counters", "as a copy of", "choose a creature type", etc.
func detectETBAs(cond *gameast.Condition) conditionScaffold {
	cs := conditionScaffold{kind: condScaffoldETBAs}
	if cond == nil {
		return cs
	}
	if len(cond.Args) > 0 {
		if s, ok := cond.Args[0].(string); ok {
			cs.rawText = strings.ToLower(strings.TrimSpace(s))
		}
	}
	if cs.rawText == "" {
		return cs
	}
	parsed := parseETBAsText("enters " + cs.rawText)
	parsed.kind = condScaffoldETBAs
	parsed.rawText = cs.rawText
	return parsed
}

// parseETBAsText pulls the "enters with N <kind> counters" payload out of
// a raw ETB phrase. subtype carries the counter kind ("+1/+1", "loyalty",
// "charge", etc.); count carries N. When the phrase is the modal "as ~
// enters, choose ..." form, subtype="choose_mode" and count=0.
func parseETBAsText(txt string) conditionScaffold {
	cs := conditionScaffold{kind: condScaffoldETBAs, rawText: txt}
	if m := entersWithCntrRe.FindStringSubmatch(txt); len(m) >= 3 {
		nWord := m[1]
		kind := strings.TrimSpace(m[2])
		n := 1
		switch nWord {
		case "a", "an", "one":
			n = 1
		case "two":
			n = 2
		case "three":
			n = 3
		case "four":
			n = 4
		default:
			if v, err := strconv.Atoi(nWord); err == nil && v > 0 {
				n = v
			}
		}
		cs.count = n
		cs.subtype = kind
		return cs
	}
	if entersAsRe.MatchString(txt) || strings.Contains(txt, "choose") || strings.Contains(txt, "choosing") {
		cs.subtype = "choose_mode"
		return cs
	}
	// Generic "enters as a copy" / "enters as <subtype>" — no counter to
	// place; flag-only apply.
	cs.subtype = "etb_modal"
	return cs
}

// detectDidPriorAction handles structured Cond.Kind="did_prior_action".
// Args[0] is the verb phrase ("attacked", "cast a spell", "sacrificed",
// "creature died", "gained life", etc.). We map it to a TurnCounters
// field via the subtype slug; apply does the actual mutation.
func detectDidPriorAction(cond *gameast.Condition) conditionScaffold {
	cs := conditionScaffold{kind: condScaffoldDidPriorAction}
	if cond == nil {
		return cs
	}
	if len(cond.Args) > 0 {
		if s, ok := cond.Args[0].(string); ok {
			cs.rawText = strings.ToLower(strings.TrimSpace(s))
		}
	}
	cs.subtype = classifyPriorActionVerb(cs.rawText)
	return cs
}

// classifyPriorActionVerb returns one of: "attacked", "cast", "sacrificed",
// "creature_died", "gained_life", "drew_card", "discarded", "played_land",
// "dealt_damage", or "" when nothing matches.
// parseETBChoiceCategory returns the category of the modal ETB choice:
// "color", "creature_type", "player", "card_name", "basic_land_type".
func parseETBChoiceCategory(txt string) string {
	switch {
	case strings.Contains(txt, "creature type"):
		return "creature_type"
	case strings.Contains(txt, "basic land type"):
		return "basic_land_type"
	case strings.Contains(txt, "card name"):
		return "card_name"
	case strings.Contains(txt, "color"):
		return "color"
	case strings.Contains(txt, "player"):
		return "player"
	case strings.Contains(txt, "a type"):
		return "creature_type"
	}
	return "color"
}

func classifyPriorActionVerb(txt string) string {
	switch {
	case strings.Contains(txt, "attacked"):
		return "attacked"
	case strings.Contains(txt, "cast a spell") || strings.Contains(txt, "cast an instant") ||
		strings.Contains(txt, "cast a noncreature") || strings.Contains(txt, "cast a creature"):
		return "cast"
	case strings.Contains(txt, "sacrific"):
		return "sacrificed"
	case strings.Contains(txt, "creature died") || strings.Contains(txt, "creature you controlled died"):
		return "creature_died"
	case strings.Contains(txt, "gained life") || strings.Contains(txt, "gain life"):
		return "gained_life"
	case strings.Contains(txt, "drew a card") || strings.Contains(txt, "drawn a card"):
		return "drew_card"
	case strings.Contains(txt, "discarded"):
		return "discarded"
	case strings.Contains(txt, "played a land"):
		return "played_land"
	case strings.Contains(txt, "dealt damage"):
		return "dealt_damage"
	}
	return ""
}

// parseFirstSpelledInt returns the first integer (digit or English word
// form, "one"-"twenty") found in txt, or 0 if none. Used by the era 3
// scaffold raw-text matchers ("N or more …") that need a count without
// committing to a specific regex shape. Word forms beat digits when both
// appear — "two or more" should win over a stray "1" elsewhere in the
// sentence.
func parseFirstSpelledInt(txt string) int {
	if txt == "" {
		return 0
	}
	for _, w := range []struct {
		word string
		n    int
	}{
		{"twenty", 20}, {"nineteen", 19}, {"eighteen", 18}, {"seventeen", 17},
		{"sixteen", 16}, {"fifteen", 15}, {"fourteen", 14}, {"thirteen", 13},
		{"twelve", 12}, {"eleven", 11}, {"ten", 10},
		{"nine", 9}, {"eight", 8}, {"seven", 7}, {"six", 6},
		{"five", 5}, {"four", 4}, {"three", 3}, {"two", 2}, {"one", 1},
	} {
		if strings.Contains(txt, w.word) {
			return w.n
		}
	}
	for i := 0; i < len(txt); i++ {
		c := txt[i]
		if c < '0' || c > '9' {
			continue
		}
		j := i
		for j < len(txt) && txt[j] >= '0' && txt[j] <= '9' {
			j++
		}
		if n, err := strconv.Atoi(txt[i:j]); err == nil {
			return n
		}
	}
	return 0
}

func parseGraveyardCount(txt string) int {
	if m := graveCntRe.FindStringSubmatch(txt); len(m) > 0 {
		if m[1] != "" {
			if v, err := strconv.Atoi(m[1]); err == nil {
				return v
			}
		}
		// Word-form numbers.
		for word, n := range map[string]int{
			"one": 1, "two": 2, "three": 3, "four": 4,
			"five": 5, "six": 6, "seven": 7,
		} {
			if strings.Contains(m[0], word) {
				return n
			}
		}
	}
	return 0
}

// applyConditionScaffolding mutates gs to satisfy the condition's shape.
// Returns the same scaffold descriptor with description populated, or a
// zero-value when nothing applied. Idempotent across types where it
// matters (it tops up rather than appending unconditionally).
func applyConditionScaffolding(gs *gameengine.GameState, cond *gameast.Condition, srcPerm *gameengine.Permanent) conditionScaffold {
	cs := detectConditionScaffold(cond)
	if cs.kind == condScaffoldNone || gs == nil {
		return conditionScaffold{}
	}

	switch cs.kind {
	case condScaffoldOpponentMoreLands:
		seedSeatLands(gs, 1, 6, "Plains", "plains")
		cs.description = "seeded 6 Plains on seat 1"

	case condScaffoldYouControlSubtype:
		placeNamedFriendlyCreatureWithSubtype(gs, "Tribal "+cs.subtype, cs.subtype)
		cs.description = fmt.Sprintf("placed %s creature on seat 0", cs.subtype)

	case condScaffoldCreatureDiedThisTurn:
		// Engine reads seat.Turn.CreaturesDied for morbid checks (since
		// TurnCounters migration). Also set legacy flag + place creature
		// card in graveyard so graveyard scans see a death event.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["creature_died_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.CreaturesDied++
			gs.Seats[0].Turn.PermanentsLeft++
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
				Name:          "Died Setup",
				Owner:         0,
				Types:         []string{"creature"},
				BasePower:     1,
				BaseToughness: 1,
			})
		}
		cs.description = "set Turn.CreaturesDied + legacy flag + added creature to graveyard"

	case condScaffoldCreatureCardsInGraveyard:
		topUpGraveyardCreatures(gs, 0, cs.count)
		cs.description = fmt.Sprintf("populated seat 0 graveyard with %d creature cards", cs.count)

	case condScaffoldCardInGraveyard:
		topUpGraveyardCreatures(gs, 0, 1)
		cs.description = "placed creature card in seat 0 graveyard"

	case condScaffoldEnergyThreshold:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["energy_counters"] = cs.count
		}
		cs.description = fmt.Sprintf("set seat 0 energy counters to %d", cs.count)

	case condScaffoldGainedLifeThisTurn:
		primeGainedLife(gs, 3)
		cs.description = "gained 3 life for seat 0 (life_gained_this_turn flag set)"

	case condScaffoldCastSpellThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].SpellsCastThisTurn++
			gs.Seats[0].Turn.SpellsCast++
			gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts, gameengine.CastRecord{
				CardName:  "Scaffold Spell",
				Types:     []string{"instant"},
				ManaValue: 2,
			})
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["cast_spell_this_turn"] = 1
			gs.Seats[0].Flags["spells_cast_this_turn"] = gs.Seats[0].Turn.SpellsCast
		}
		gs.SpellsCastThisTurn++
		cs.description = "incremented Turn.SpellsCast + legacy cast counters for seat 0"

	case condScaffoldCreatureETBThisTurn:
		placeNamedFriendlyCreature(gs, "ETB Witness")
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.CreaturesEntered++
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["creature_etb_this_turn"] = 1
		cs.description = "set Turn.CreaturesEntered + legacy flag + placed ETB Witness"

	case condScaffoldDrawnCardThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.CardsDrawn++
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["drawn_card_this_turn"] = 1
			if len(gs.Seats[0].Library) < 5 {
				fillLibrary(gs, 0, 5)
			}
		}
		cs.description = "set Turn.CardsDrawn + legacy flag + filled library"

	case condScaffoldAttackedThisTurn:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["attacked_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Attacked = true
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["attacked_this_turn"] = 1
		}
		cs.description = "set Turn.Attacked + legacy flag on seat 0 and game"

	case condScaffoldSacrificedThisTurn:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["sacrificed_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Sacrificed++
			gs.Seats[0].Turn.PermanentsLeft++
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["sacrificed_this_turn"] = 1
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
				Name:          "Sac Victim Setup",
				Owner:         0,
				Types:         []string{"creature"},
				BasePower:     1,
				BaseToughness: 1,
			})
		}
		cs.description = "set Turn.Sacrificed + legacy flag + placed creature in graveyard"

	case condScaffoldCombatDamageDealt:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["combat_damage_dealt_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["combat_damage_dealt_this_turn"] = 1
		}
		cs.description = "set combat_damage_dealt_this_turn flag"

	case condScaffoldLandfallThisTurn:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["landfall_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.LandsPlayed++
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["landfall_this_turn"] = 1
			// Place a land on the battlefield to represent the landfall trigger source.
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
				Card: &gameengine.Card{
					Name:  "Landfall Setup Land",
					Owner: 0,
					Types: []string{"land", "forest"},
				},
				Controller: 0,
				Owner:      0,
				Flags:      map[string]int{},
				Counters:   map[string]int{},
			})
		}
		cs.description = "set landfall_this_turn flag + placed land on seat 0"

	case condScaffoldDiscardedThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["discarded_this_turn"] = 1
			gs.Seats[0].Turn.Discarded++
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
				Name:  "Discarded Setup",
				Owner: 0,
				Types: []string{"instant"},
			})
			if len(gs.Seats[0].Hand) < 3 {
				fillHand(gs, 0, 3-len(gs.Seats[0].Hand))
			}
		}
		cs.description = "set Turn.Discarded + legacy flag + placed card in graveyard"

	case condScaffoldEnchantedCreature:
		// For aura conditions that reference "enchanted creature", we need
		// a creature on the battlefield with the source attached to it.
		target := placeNamedFriendlyCreature(gs, "Enchanted Target")
		if srcPerm != nil {
			srcPerm.AttachedTo = target
		}
		cs.description = "placed Enchanted Target creature for aura attachment"

	case condScaffoldOpponentLostLife:
		if len(gs.Seats) > 1 && gs.Seats[1] != nil {
			gs.Seats[1].Life -= 3
			if gs.Seats[1].Flags == nil {
				gs.Seats[1].Flags = map[string]int{}
			}
			gs.Seats[1].Flags["life_lost_this_turn"] = 3
			gs.Seats[1].Flags["lost_life_this_turn"] = 3
		}
		cs.description = "opponent (seat 1) lost 3 life this turn"

	case condScaffoldLifeAboveThreshold:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Life < cs.threshold {
				gs.Seats[0].Life = cs.threshold
			}
		}
		cs.description = fmt.Sprintf("set seat 0 life to %d (above threshold)", cs.threshold)

	case condScaffoldLifeBelowThreshold:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Life > cs.threshold {
				gs.Seats[0].Life = cs.threshold
			}
		}
		cs.description = fmt.Sprintf("set seat 0 life to %d (below threshold)", cs.threshold)

	case condScaffoldUpkeepPhase:
		gs.Phase = "beginning"
		gs.Step = "upkeep"
		cs.description = "set game phase to upkeep"

	case condScaffoldHellbent:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Hand = nil
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["hellbent"] = 1
		}
		cs.description = "emptied seat 0 hand (hellbent active)"

	case condScaffoldMonarch:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["has_monarch"] = 1
		gs.Flags["monarch_seat"] = 0
		cs.description = "made seat 0 the monarch"

	case condScaffoldInitiative:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["initiative_holder"] = 0
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["has_initiative"] = 1
		}
		cs.description = "gave seat 0 the initiative"

	case condScaffoldDelirium:
		seedDeliriumGraveyard(gs, 0)
		cs.description = "seeded seat 0 graveyard with 4 distinct card types"

	case condScaffoldSpellMastery:
		seedSpellMasteryGraveyard(gs, 0)
		cs.description = "seeded seat 0 graveyard with 2 instant/sorcery cards"

	case condScaffoldRevolt:
		// Revolt reads gs.EventLog for a destroy/sacrifice/exile/bounce
		// event by seat 0 this turn. Append a synthesised sacrifice event
		// rather than mutating actual zones — the engine's CheckRevolt only
		// consults the log.
		gs.EventLog = append(gs.EventLog, gameengine.Event{
			Kind:   "sacrifice",
			Seat:   0,
			Target: -1,
			Source: "thor_priming",
		})
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["revolt_active"] = 1
		cs.description = "logged sacrifice event for seat 0 (revolt active)"

	case condScaffoldMetalcraft:
		seedSeatArtifacts(gs, 0, 3)
		cs.description = "placed 3 artifacts on seat 0 (metalcraft active)"

	case condScaffoldFerocious:
		placePoweredCreature(gs, 0, "Ferocious Setup", 4, 4)
		cs.description = "placed 4/4 creature on seat 0 (ferocious active)"

	case condScaffoldFormidable:
		placePoweredCreature(gs, 0, "Formidable Setup A", 4, 4)
		placePoweredCreature(gs, 0, "Formidable Setup B", 4, 4)
		cs.description = "placed creatures totaling 8 power on seat 0 (formidable active)"

	case condScaffoldPermanentLeftBF:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["permanent_left_bf"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.PermanentsLeft++
			gs.Flags["permanent_left_bf_0"] = 1
		}
		gs.EventLog = append(gs.EventLog, gameengine.Event{
			Kind:   "sacrifice",
			Seat:   0,
			Target: -1,
			Source: "thor_priming",
		})
		cs.description = "set Turn.PermanentsLeft + permanent_left_bf flag (disappear/void)"

	case condScaffoldSecondSpellThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.SpellsCast = 2
			gs.Seats[0].SpellsCastThisTurn = 2
			gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts,
				gameengine.CastRecord{CardName: "Scaffold Spell 1", Types: []string{"instant"}, ManaValue: 1},
				gameengine.CastRecord{CardName: "Scaffold Spell 2", Types: []string{"sorcery"}, ManaValue: 2},
			)
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["spells_cast_this_turn"] = 2
			gs.Seats[0].Flags["cast_spell_this_turn"] = 1
		}
		gs.SpellsCastThisTurn = 2
		cs.description = "set Turn.SpellsCast=2 + 2 CastRecords (second spell scaffold)"

	case condScaffoldDescendedThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Descended = true
			gs.Seats[0].DescendedThisTurn = true
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
				Name:  "Descended Setup",
				Owner: 0,
				Types: []string{"creature"},
			})
		}
		cs.description = "set Turn.Descended + legacy DescendedThisTurn + creature in GY"

	case condScaffoldLifeLostThisTurn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.LifeLost += 3
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["life_lost_this_turn"] = 3
			gs.Seats[0].Flags["lost_life_this_turn"] = 3
		}
		cs.description = "set Turn.LifeLost=3 + legacy flags (life lost this turn)"

	case condScaffoldTokensCreatedCount:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.TokensCreated = 3
			gs.Seats[0].Turn.TreasuresCreated = 2
		}
		cs.description = "set Turn.TokensCreated=3, TreasuresCreated=2"

	case condScaffoldCastFromExile:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.CastFromExile++
		}
		cs.description = "incremented Turn.CastFromExile"

	case condScaffoldExileLinkedReturn:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil && srcPerm != nil {
			exiledCard := &gameengine.Card{
				Name:  "Exile-Linked Target",
				Owner: 1,
				Types: []string{"creature"},
			}
			srcPerm.LinkedExile = append(srcPerm.LinkedExile, exiledCard)
			gs.Seats[1].Exile = append(gs.Seats[1].Exile, exiledCard)
		}
		cs.description = "attached exile-linked card to source permanent (O-Ring scaffold)"

	case condScaffoldPaidOptionalCost:
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["paid_optional_cost"] = 1
		// Engine reads Permanent.Flags["kicked"] (count, with multikicker
		// stacking) — see resolve.go condition handling. Mark srcPerm when
		// available so a kicker-gated trigger condition resolves true.
		n := cs.count
		if n < 1 {
			n = 1
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["kicked"] = n
		}
		cs.description = fmt.Sprintf("set Flags[paid_optional_cost]=1 + srcPerm.Flags[kicked]=%d", n)

	case condScaffoldForEach:
		count := cs.count
		if count < 3 {
			count = 3
		}
		switch cs.subtype {
		case "land":
			seedSeatLands(gs, 0, count, "Forest", "forest")
			cs.description = fmt.Sprintf("seeded %d lands on seat 0 (for_each land)", count)
		case "artifact":
			seedSeatArtifacts(gs, 0, count)
			cs.description = fmt.Sprintf("seeded %d artifacts on seat 0 (for_each artifact)", count)
		case "opponent":
			cs.description = "for_each opponent — no priming required"
		case "creature", "permanent", "":
			seedSeatCreatures(gs, 0, count, "ForEach Creature", "")
			cs.description = fmt.Sprintf("seeded %d creatures on seat 0 (for_each %s)", count, cs.subtype)
		default:
			// Subtype token — wizard, goblin, etc. Place creatures with the
			// subtype tag.
			seedSeatCreatures(gs, 0, count, "ForEach "+cs.subtype, cs.subtype)
			cs.description = fmt.Sprintf("seeded %d %s creatures on seat 0", count, cs.subtype)
		}

	case condScaffoldETBAs:
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			switch cs.subtype {
			case "choose_mode":
				srcPerm.Flags["etb_choice_set"] = 1
				cs.description = "set srcPerm.Flags[etb_choice_set]=1 (modal ETB)"
			case "etb_modal", "":
				// Generic ETB — let the ETB handler do its job. We just mark
				// the flag so any downstream condition that asks "did this
				// permanent enter via the modal path" sees a positive answer.
				srcPerm.Flags["etb_modal"] = 1
				cs.description = "set srcPerm.Flags[etb_modal]=1 (generic ETB-as)"
			default:
				n := cs.count
				if n < 1 {
					n = 1
				}
				srcPerm.Counters[cs.subtype] += n
				cs.description = fmt.Sprintf("placed %d %q counters on srcPerm (ETB-with)", n, cs.subtype)
			}
		} else {
			cs.description = "etb_as detected but srcPerm nil — flag-only"
		}

	case condScaffoldDidPriorAction:
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			switch cs.subtype {
			case "attacked":
				seat.Turn.Attacked = true
				seat.Flags["attacked_this_turn"] = 1
				cs.description = "set Turn.Attacked=true (did_prior_action: attacked)"
			case "cast":
				seat.Turn.SpellsCast++
				seat.Turn.Casts = append(seat.Turn.Casts, gameengine.CastRecord{
					CardName:  "PriorAction Spell",
					Types:     []string{"instant"},
					ManaValue: 2,
				})
				seat.SpellsCastThisTurn++
				gs.SpellsCastThisTurn++
				seat.Flags["cast_spell_this_turn"] = 1
				cs.description = "incremented Turn.SpellsCast + appended CastRecord (did_prior_action: cast)"
			case "sacrificed":
				seat.Turn.Sacrificed++
				seat.Turn.PermanentsLeft++
				seat.Flags["sacrificed_this_turn"] = 1
				cs.description = "incremented Turn.Sacrificed (did_prior_action: sacrificed)"
			case "creature_died":
				seat.Turn.CreaturesDied++
				seat.Turn.PermanentsLeft++
				if gs.Flags == nil {
					gs.Flags = map[string]int{}
				}
				gs.Flags["creature_died_this_turn"] = 1
				seat.Graveyard = append(seat.Graveyard, &gameengine.Card{
					Name:          "Prior Death",
					Owner:         0,
					Types:         []string{"creature"},
					BasePower:     1,
					BaseToughness: 1,
				})
				cs.description = "incremented Turn.CreaturesDied + added creature to graveyard"
			case "gained_life":
				seat.Turn.LifeGained += 3
				seat.Life += 3
				seat.Flags["life_gained_this_turn"] = 3
				cs.description = "added 3 to Turn.LifeGained + Life (did_prior_action: gained_life)"
			case "drew_card":
				seat.Turn.CardsDrawn++
				seat.Flags["drawn_card_this_turn"] = 1
				cs.description = "incremented Turn.CardsDrawn (did_prior_action: drew_card)"
			case "discarded":
				seat.Turn.Discarded++
				seat.Flags["discarded_this_turn"] = 1
				cs.description = "incremented Turn.Discarded (did_prior_action: discarded)"
			case "played_land":
				seat.Turn.LandsPlayed++
				seat.Flags["landfall_this_turn"] = 1
				cs.description = "incremented Turn.LandsPlayed (did_prior_action: played_land)"
			case "dealt_damage":
				if gs.Flags == nil {
					gs.Flags = map[string]int{}
				}
				gs.Flags["combat_damage_dealt_this_turn"] = 1
				seat.Flags["combat_damage_dealt_this_turn"] = 1
				cs.description = "set combat_damage_dealt_this_turn flag (did_prior_action: dealt_damage)"
			default:
				// Unknown verb — nothing to prime, but still valid scaffold so
				// the caller logs it instead of dropping it.
				cs.description = "did_prior_action with unrecognized verb — no-op prime"
			}
		}

	case condScaffoldCycled:
		// Engine fires "when you cycle" via fireCyclingTriggers. The
		// trigger plumbing lives on permanents, but Goldilocks only
		// cares that the controller's cycle counter / event log show a
		// recent cycle. We log a cycle event for seat 0 and bump a
		// per-seat flag. The actual trigger fires in fireTriggerEvent.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			seat.Flags["cycled_this_turn"] = 1
			// Place a placeholder card in the graveyard to represent the
			// discarded-via-cycling card so any "the cycled card" lookups
			// have something to find.
			seat.Graveyard = append(seat.Graveyard, &gameengine.Card{
				Name:  "Cycled Card Setup",
				Owner: 0,
				Types: []string{"creature"},
			})
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "cycle",
			Seat:   0,
			Source: "Cycled Card Setup",
			Details: map[string]interface{}{
				"reason": "scaffold_prime",
				"rule":   "702.29",
			},
		})
		cs.description = "logged cycle event + set cycled_this_turn on seat 0"

	case condScaffoldMutates:
		// Engine marks mutated permanents with Flags["mutated"] = 1
		// (see ApplyMutate in keywords_batch6.go). For scaffolding,
		// stamping the source as already mutated lets "whenever this
		// creature mutates" / "as long as ~ has mutated" reads succeed.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["mutated"] = 1
			cs.description = "set srcPerm.Flags[mutated]=1"
		} else {
			cs.description = "mutate detected but srcPerm nil — flag-only"
		}
		gs.LogEvent(gameengine.Event{
			Kind: "mutate",
			Seat: 0,
			Details: map[string]interface{}{
				"stub": true,
				"rule": "702.140",
			},
		})

	case condScaffoldUnlockDoor:
		// Duskmourn rooms / "when you unlock this door" triggers. No
		// dedicated room-state struct in the engine; mark the source
		// permanent's Flags["unlocked"] and emit an unlock_door event
		// so log readers see the priming.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["unlocked"] = 1
			cs.description = "set srcPerm.Flags[unlocked]=1"
		} else {
			cs.description = "unlock_door detected but srcPerm nil — flag-only"
		}
		gs.LogEvent(gameengine.Event{
			Kind: "unlock_door",
			Seat: 0,
			Details: map[string]interface{}{
				"reason": "scaffold_prime",
			},
		})

	case condScaffoldPriorTurnSpellCount:
		// Werewolf transform conditions read prior-turn cast counts. The
		// engine snapshots SpellsCastThisTurn into SpellsCastLastTurn at
		// untap. Set the value directly on every seat: cs.count==0 means
		// "no spells last turn" (silver werewolf transform); 2 means
		// "two or more spells" (gold werewolf transform).
		want := cs.count
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			seat.SpellsCastLastTurn = want
		}
		cs.description = fmt.Sprintf("set SpellsCastLastTurn=%d on all seats", want)

	case condScaffoldPairedSoulbond:
		// Soulbond pairs two unpaired creatures under the same
		// controller. Place a partner creature on seat 0 and pair it
		// with srcPerm via PairSoulbond so IsPaired(srcPerm) → true.
		// Falls back to placing two creatures and pairing them when
		// srcPerm is nil (audit context where the source isn't on the
		// battlefield yet).
		if srcPerm == nil || srcPerm.Controller != 0 {
			// Audit/standalone path — place both halves of the pair.
			a := placeNamedFriendlyCreature(gs, "Soulbond A")
			b := placeNamedFriendlyCreature(gs, "Soulbond B")
			if a != nil && b != nil {
				// Stamp timestamps so PairSoulbond's GetPairedPartner
				// lookup succeeds (it walks battlefield by Timestamp).
				if a.Timestamp == 0 {
					a.Timestamp = 1001
				}
				if b.Timestamp == 0 {
					b.Timestamp = 1002
				}
				gameengine.PairSoulbond(gs, a, b)
				cs.description = "placed two creatures + paired via soulbond"
			} else {
				cs.description = "soulbond detected but seat 0 missing — no-op"
			}
		} else {
			partner := placeNamedFriendlyCreature(gs, "Soulbond Partner")
			if partner == nil {
				cs.description = "soulbond detected but partner placement failed"
			} else {
				if partner.Timestamp == 0 {
					partner.Timestamp = srcPerm.Timestamp + 1
					if partner.Timestamp == 0 {
						partner.Timestamp = 1001
					}
				}
				if srcPerm.Timestamp == 0 {
					srcPerm.Timestamp = partner.Timestamp - 1
				}
				gameengine.PairSoulbond(gs, srcPerm, partner)
				cs.description = "placed Soulbond Partner + paired with srcPerm"
			}
		}

	case condScaffoldTurnedFaceUp:
		// Place srcPerm (or a stand-in) on the battlefield face-down, then
		// flip it face-up via the canonical engine path so a `turn_face_up`
		// event lands in the log and the listener side of the trigger sees
		// the state transition.
		target := srcPerm
		if target == nil {
			target = placeNamedFriendlyCreature(gs, "Morph Subject")
		}
		if target != nil && target.Card != nil {
			target.Card.FaceDown = true
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["face_down"] = 1
			gameengine.TurnFaceUp(gs, target, "scaffold_turned_face_up")
			cs.description = "set source face-down then TurnFaceUp (turn_face_up event emitted)"
		} else {
			cs.description = "turned-face-up detected but no source available"
		}

	case condScaffoldBeginningOfOrdinalStep:
		// Move the game clock to the matched step on the active seat.
		// `Phase` is the wider phase; `Step` is the granular step name.
		switch cs.subtype {
		case "combat":
			gs.Phase, gs.Step = "combat", "begin_of_combat"
		case "draw":
			gs.Phase, gs.Step = "beginning", "draw"
		case "end_step":
			gs.Phase, gs.Step = "ending", "end_step"
		case "postcombat_main":
			gs.Phase, gs.Step = "postcombat_main", "postcombat_main"
		case "precombat_main":
			gs.Phase, gs.Step = "precombat_main", "precombat_main"
		case "untap":
			gs.Phase, gs.Step = "beginning", "untap"
		default:
			gs.Phase, gs.Step = "beginning", cs.subtype
		}
		cs.description = fmt.Sprintf("set Phase=%s Step=%s", gs.Phase, gs.Step)

	case condScaffoldTribeYouControlETB:
		subtype := cs.subtype
		if subtype == "" {
			subtype = "creature"
		}
		// Already-on-battlefield witness so the trigger sees a friendly
		// permanent of the matching subtype. The ETB itself is fired by
		// the trigger machinery once a fresh permanent enters; if the
		// caller needs a *new* ETB event after this priming runs, it can
		// place an additional creature itself.
		placeNamedFriendlyCreatureWithSubtype(gs, "Tribe ETB "+subtype, subtype)
		cs.description = fmt.Sprintf("placed %s creature on seat 0 (tribe-ETB witness)", subtype)

	case condScaffoldManaSpentThreshold:
		// Stamp srcPerm so the engine's mana_spent / mana-value condition
		// can resolve true once it is tightened past the current default.
		// Comfortable margin (+2) above the threshold so equality and
		// strict-greater predicates both pass.
		threshold := cs.count
		if threshold < 1 {
			threshold = 1
		}
		paid := threshold + 2
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["mana_spent"] = paid
			srcPerm.Flags["mana_value_spent"] = paid
			if srcPerm.Card != nil && srcPerm.Card.CMC < threshold {
				srcPerm.Card.CMC = threshold
			}
		}
		// Also leave a CastRecord so MaxManaValue queries see a spell at
		// the threshold (or above) for this turn.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts, gameengine.CastRecord{
				CardName:  "Mana Spent Scaffold",
				Types:     []string{"sorcery"},
				ManaValue: paid,
			})
		}
		cs.description = fmt.Sprintf("set srcPerm.Flags[mana_spent]=%d (threshold=%d) + CastRecord", paid, threshold)

	case condScaffoldAnyPlayerPhase:
		if cs.subtype == "end_step" {
			gs.Phase, gs.Step = "ending", "end_step"
		} else {
			gs.Phase, gs.Step = "beginning", "upkeep"
		}
		if len(gs.Seats) > 1 {
			gs.Active = 1
		}
		cs.description = fmt.Sprintf("set Phase=%s Step=%s Active=1 (any-player phase)", gs.Phase, gs.Step)

	case condScaffoldDelayedDrawNextUpkeep:
		gs.Phase, gs.Step = "beginning", "upkeep"
		gs.Turn++
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if len(gs.Seats[0].Library) < 5 {
				fillLibrary(gs, 0, 5)
			}
		}
		cs.description = fmt.Sprintf("set Phase=beginning Step=upkeep Turn=%d + filled library (delayed draw)", gs.Turn)

	case condScaffoldETBModalChoice:
		target := srcPerm
		if target == nil {
			target = placeNamedFriendlyCreature(gs, "ETB Choice Subject")
		}
		if target != nil {
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["etb_choice_set"] = 1
			switch cs.subtype {
			case "color":
				target.Flags["chosen_color"] = 1
			case "creature_type":
				target.Flags["chosen_creature_type"] = 1
			case "basic_land_type":
				target.Flags["chosen_land_type"] = 1
			case "player":
				target.Flags["chosen_player"] = 1
			case "card_name":
				target.Flags["chosen_card_name"] = 1
			}
		}
		cs.description = fmt.Sprintf("set ETB modal choice=%s on srcPerm", cs.subtype)

	case condScaffoldBecomesTapped:
		// Place (or reuse) a target permanent untapped, then tap it so the
		// "becomes tapped" trigger has a state transition to observe. We
		// also stamp the source's Flags so observers that read attachment
		// state (e.g. Insolence's "enchanted creature becomes tapped")
		// see a friendly target. NOT calling TapPermanent here because the
		// trigger registry uses Permanent.Tapped directly during fire.
		target := placeNamedFriendlyCreature(gs, "Becomes-Tapped Subject")
		if target != nil {
			target.Tapped = false
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["scaffold_becomes_tapped_target"] = 1
			// Toggle the tap state so the trigger sees an untapped → tapped
			// transition during the fire pass.
			target.Tapped = true
		}
		if srcPerm != nil && target != nil {
			srcPerm.AttachedTo = target
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "becomes_tapped",
			Seat:   0,
			Source: nonEmpty(func() string {
				if target != nil && target.Card != nil {
					return target.Card.Name
				}
				return ""
			}(), "Becomes-Tapped Subject"),
			Details: map[string]interface{}{
				"reason": "scaffold_prime",
			},
		})
		cs.description = "tapped subject permanent + logged becomes_tapped event"

	case condScaffoldBecomesTarget:
		// Simulate targeting srcPerm by pushing a placeholder spell onto
		// the stack with srcPerm as its sole target. The trigger registry's
		// "becomes_target" alias fires off the targeted-event observer in
		// engine resolve flow; here we just need state that a downstream
		// observer can read. We also flag the permanent so static reads
		// like "if ~ has been targeted this turn" succeed.
		target := srcPerm
		if target == nil {
			target = placeNamedFriendlyCreature(gs, "Target Subject")
		}
		if target != nil {
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["was_targeted_this_turn"] = 1
		}
		// Push a synthetic stack item targeting the permanent.
		if target != nil {
			gs.Stack = append(gs.Stack, &gameengine.StackItem{
				Controller: 1, // simulate an opponent's spell targeting us
				Card: &gameengine.Card{
					Name:  "Targeting Spell Setup",
					Owner: 1,
					Types: []string{"instant"},
				},
				Targets: []gameengine.Target{
					{Kind: gameengine.TargetKindPermanent, Permanent: target, Seat: 0},
				},
			})
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "becomes_target",
			Seat:   0,
			Target: 1,
			Details: map[string]interface{}{
				"reason": "scaffold_prime",
			},
		})
		cs.description = "pushed targeting spell on stack + flagged permanent + logged becomes_target"

	case condScaffoldUntilEOTDelayed:
		// Advance the clock to the end / cleanup step so a delayed trigger
		// scheduled "until end of turn" / "at the next cleanup step" has
		// a phase boundary to fire on. Cleanup is the strict "end-of-turn"
		// fire point per CR §514; end_step is where most "at the beginning
		// of the end step" triggers land.
		if strings.Contains(cs.rawText, "cleanup") {
			gs.Phase, gs.Step = "ending", "cleanup"
		} else {
			gs.Phase, gs.Step = "ending", "end_step"
		}
		// Mark the delayed trigger as registered so static reads can see
		// pending triggers in scope.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["delayed_eot_trigger_active"] = 1
		cs.description = fmt.Sprintf("set Phase=ending Step=%s + delayed_eot_trigger_active flag", gs.Step)

	case condScaffoldLandPlayOrTap:
		// Seed lands on both seats so any-player and opponent variants
		// have a target, then log a land_played / land_tapped_for_mana
		// event for the right seats. Default subtype "any_player" primes
		// both; "opponent" primes only seat 1.
		seats := []int{0, 1}
		if cs.subtype == "opponent" {
			seats = []int{1}
		}
		for _, s := range seats {
			seedSeatLands(gs, s, 3, "Forest", "forest")
		}
		// Move to a main phase so the trigger has a legal land-play window.
		gs.Phase, gs.Step = "precombat_main", "precombat_main"
		// Log both event flavors so detect-vs-apply handlers downstream
		// see at least one signal (the audit collapsed three slugs into
		// this single scaffold).
		landSeat := seats[len(seats)-1]
		gs.LogEvent(gameengine.Event{
			Kind:   "land_played",
			Seat:   landSeat,
			Source: "Forest",
			Details: map[string]interface{}{
				"reason":      "scaffold_prime",
				"any_player":  cs.subtype == "any_player",
				"opp_only":    cs.subtype == "opponent",
			},
		})
		gs.LogEvent(gameengine.Event{
			Kind:   "land_tapped_for_mana",
			Seat:   landSeat,
			Source: "Forest",
			Details: map[string]interface{}{
				"reason": "scaffold_prime",
				"color":  "G",
			},
		})
		cs.description = fmt.Sprintf("seeded lands on %v + logged land_played/tapped_for_mana (subtype=%s)", seats, cs.subtype)

	case condScaffoldETBTappedUnless:
		// "X enters tapped unless Y" — satisfy Y so the source enters
		// untapped. Most "unless" clauses are land-control checks; the
		// raw-text patterns below seed a covering board state. When
		// nothing matches, we still mark the source untapped + flag so
		// the engine's ETB path can fall through cleanly.
		txt := cs.rawText
		switch {
		case strings.Contains(txt, "basic land") || strings.Contains(txt, "two or more basic"):
			seedSeatLands(gs, 0, 2, "Forest", "forest")
			seedSeatLands(gs, 0, 2, "Plains", "plains")
		case strings.Contains(txt, "plains") || strings.Contains(txt, "island") ||
			strings.Contains(txt, "swamp") || strings.Contains(txt, "mountain") ||
			strings.Contains(txt, "forest"):
			// Place a relevant dual-land basic on seat 0.
			for _, name := range []string{"plains", "island", "swamp", "mountain", "forest"} {
				if strings.Contains(txt, name) {
					seedSeatLands(gs, 0, 1, strings.Title(name), name)
				}
			}
		case strings.Contains(txt, "13 or less life") || strings.Contains(txt, "13 or fewer life"):
			if len(gs.Seats) > 1 && gs.Seats[1] != nil {
				gs.Seats[1].Life = 13
			}
		case strings.Contains(txt, "two or more"):
			seedSeatLands(gs, 0, 2, "Forest", "forest")
		}
		if srcPerm != nil {
			srcPerm.Tapped = false
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["etb_tapped_unless_satisfied"] = 1
		}
		cs.description = "satisfied etb_tapped_unless clause + marked source untapped"

	case condScaffoldDomain:
		// Domain counts distinct basic land types you control. Seed one of
		// each basic so CheckDomain reports 5 (max).
		seedSeatLands(gs, 0, 1, "Plains", "plains")
		seedSeatLands(gs, 0, 2, "Island", "island")
		seedSeatLands(gs, 0, 3, "Swamp", "swamp")
		seedSeatLands(gs, 0, 4, "Mountain", "mountain")
		seedSeatLands(gs, 0, 5, "Forest", "forest")
		cs.description = "seeded 5 distinct basic land types on seat 0 (domain max)"

	case condScaffoldETBIf:
		// Reuse the raw-text matcher: if args[0] looks like one of our
		// canonical raw-text patterns, dispatch into the matching scaffold
		// by synthesising a raw-form Condition. Otherwise mark the source
		// with a generic ETB flag so trigger handlers can short-circuit.
		if cs.rawText != "" {
			synthetic := &gameast.Condition{
				Kind: "raw",
				Args: []interface{}{cs.rawText},
			}
			sub := applyConditionScaffolding(gs, synthetic, srcPerm)
			if sub.kind != condScaffoldNone {
				cs.description = "etb_if delegated → " + sub.description
				break
			}
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["etb_if_satisfied"] = 1
		}
		// Generic "cast from hand" priming is the most common variant —
		// mark the controller as having cast the source from hand this turn.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts, gameengine.CastRecord{
				CardName:  "ETB-If Cast",
				Types:     []string{"creature"},
				ManaValue: 2,
			})
		}
		cs.description = "set srcPerm.Flags[etb_if_satisfied]=1 + cast record (generic etb_if)"

	case condScaffoldRepeatN:
		// "do this N times" / "repeat X times". The engine reads the
		// repeat count from the surrounding effect; for priming purposes
		// we just stamp a per-source flag so observers that check
		// "repeated this turn" succeed.
		n := cs.count
		if n < 1 {
			n = 1
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["repeat_n"] = n
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["repeat_n"] = n
		cs.description = fmt.Sprintf("stamped repeat_n=%d on srcPerm + game flags", n)

	case condScaffoldLieutenant:
		// "As long as you control your commander, ..." Stamp a per-seat
		// flag the engine's commander tracker reads, and place a stand-in
		// commander permanent on seat 0 so any "is your commander" check
		// against the battlefield succeeds.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["controls_commander"] = 1
		}
		cmdr := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:          "Lieutenant Commander Setup",
				Owner:         0,
				Types:         []string{"creature", "legendary"},
				BasePower:     3,
				BaseToughness: 3,
			},
			Controller: 0,
			Owner:      0,
			Flags:      map[string]int{"is_commander": 1},
			Counters:   map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, cmdr)
		cs.description = "placed commander stand-in + set controls_commander flag"

	case condScaffoldKiCountersGE2:
		// Kamigawa flip cards (Faithful Squire, Cunning Bandit, Budoka Pupil)
		// track ki counters on the source; ≥2 enables the flip. The engine
		// counts via srcPerm.Counters["ki"].
		want := cs.count
		if want < 2 {
			want = 2
		}
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Counters["ki"] < want {
				srcPerm.Counters["ki"] = want
			}
		}
		cs.description = fmt.Sprintf("placed %d ki counters on srcPerm", want)

	case condScaffoldSelfIsTapped:
		// Hollow Trees / Sand Silos / Dwarven Hold: source must be tapped
		// for the ability to qualify. Tap srcPerm.
		if srcPerm != nil {
			srcPerm.Tapped = true
		}
		cs.description = "tapped srcPerm (self_is_tapped active)"

	case condScaffoldAttackedOrBlockedCombat:
		// Clockwork series — "if ~ attacked or blocked this combat".
		// Stamp both attacked and blocked flags on the source and set
		// the seat's attacked-this-turn flag so any wider observer sees
		// combat activity.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["attacked_this_combat"] = 1
			srcPerm.Flags["blocked_this_combat"] = 1
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Attacked = true
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["attacked_this_turn"] = 1
		}
		cs.description = "set source attacked_this_combat + blocked_this_combat + seat attacked_this_turn"

	case condScaffoldCoven:
		// Coven (Innistrad Midnight Hunt) — control 3+ creatures with
		// pairwise-different powers. Seed three creatures at 1/1, 2/2, 3/3.
		for i, pt := range []int{1, 2, 3} {
			placePoweredCreature(gs, 0, fmt.Sprintf("Coven Setup %d", i), pt, pt)
		}
		cs.description = "placed 3 creatures (1/1, 2/2, 3/3) on seat 0 (coven active)"

	case condScaffoldSelfHasCounter:
		// "as long as ~ has a counter on it" — Skyclave Sentinel,
		// Scuttlegator, Woolly Razorback. Stamp the named counter on
		// srcPerm; default to +1/+1 when args[0] is blank.
		kind := cs.subtype
		if kind == "" {
			kind = "+1/+1"
		}
		n := cs.count
		if n < 1 {
			n = 1
		}
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Counters[kind] < n {
				srcPerm.Counters[kind] = n
			}
		}
		cs.description = fmt.Sprintf("placed %d %q counters on srcPerm", n, kind)

	case condScaffoldDidntAttackThisTurn:
		// Inverse of attacked_this_turn — explicitly clear the flag in
		// case prior scaffolding stamped it.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Attacked = false
			if gs.Seats[0].Flags != nil {
				delete(gs.Seats[0].Flags, "attacked_this_turn")
			}
		}
		if gs.Flags != nil {
			delete(gs.Flags, "attacked_this_turn")
			delete(gs.Flags, "seat_0_attacked_this_turn")
		}
		cs.description = "cleared attacked_this_turn (didnt_attack_this_turn active)"

	case condScaffoldDealtDamageOpponentTurn:
		// "if you dealt damage to an opponent this turn" — Hatred,
		// some warrior tribal. Log a damage event and stamp seat flags.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["dealt_damage_to_opponent_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["dealt_damage_to_opponent_this_turn"] = 1
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   0,
			Target: 1,
			Amount: 3,
			Source: "thor_priming",
		})
		cs.description = "set dealt_damage_to_opponent_this_turn flag + logged damage event"

	// Era 4 audit additions — apply implementations.

	case condScaffoldItWasCreature:
		// Post-death typecheck — "if it was a creature when it died /
		// left the battlefield". Place a creature card in seat 0's
		// graveyard and stamp a per-game flag so observers reading
		// "was_creature_on_leave" succeed.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
				Name:          "Was-Creature Setup",
				Owner:         0,
				Types:         []string{"creature"},
				BasePower:     1,
				BaseToughness: 1,
			})
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["last_left_was_creature"] = 1
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["was_creature_on_leave"] = 1
		}
		cs.description = "placed creature in seat 0 graveyard + set last_left_was_creature flag"

	case condScaffoldNoCreaturesOnBattlefield:
		// "no creatures are on the battlefield" — Sothera, the Supervoid;
		// Portcullis. Clear creature permanents from every seat so the
		// condition resolves true.
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			filtered := seat.Battlefield[:0]
			for _, p := range seat.Battlefield {
				if p == nil || p.Card == nil {
					filtered = append(filtered, p)
					continue
				}
				isCreature := false
				for _, t := range p.Card.Types {
					if t == "creature" {
						isCreature = true
						break
					}
				}
				if !isCreature {
					filtered = append(filtered, p)
				}
			}
			seat.Battlefield = filtered
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["no_creatures_on_battlefield"] = 1
		cs.description = "removed all creature permanents from every seat"

	case condScaffoldHadCountersOnIt:
		// Past-state counter check on srcPerm (Ozolith, Nikara, Leader's
		// Talent). Stamp +1/+1 counters now so any "had counters on it"
		// post-leave snapshot the engine takes resolves true.
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Counters["+1/+1"] < 1 {
				srcPerm.Counters["+1/+1"] = 1
			}
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["had_counters"] = 1
		}
		cs.description = "stamped +1/+1 counter on srcPerm + had_counters flag"

	case condScaffoldYouCastFromHand:
		// "you cast it [from your hand]" condition on the source itself.
		// Append a cast record naming the source (or a generic stand-in)
		// so any "cast from hand" predicate finds it. Also stamp the
		// permanent's cast-from-hand flag for direct lookups.
		castName := "Cast-From-Hand Setup"
		if srcPerm != nil && srcPerm.Card != nil && srcPerm.Card.Name != "" {
			castName = srcPerm.Card.Name
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts, gameengine.CastRecord{
				CardName:  castName,
				Types:     []string{"creature"},
				ManaValue: 2,
			})
			gs.Seats[0].SpellsCastThisTurn++
			gs.Seats[0].Turn.SpellsCast++
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["cast_spell_this_turn"] = 1
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["cast_from_hand"] = 1
		}
		cs.description = "appended cast record + set cast_from_hand flag on srcPerm"

	case condScaffoldPlaneswalkerETBThisTurn:
		// "a planeswalker entered the battlefield under your control this
		// turn" — Oath of Liliana, Oath of Chandra. Place a planeswalker
		// on seat 0 + increment Turn.PlaneswalkersEntered if engine tracks
		// it; fall back to a stamped game flag otherwise.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
				Card: &gameengine.Card{
					Name:  "Planeswalker ETB Setup",
					Owner: 0,
					Types: []string{"planeswalker", "legendary"},
				},
				Controller: 0,
				Owner:      0,
				Flags:      map[string]int{"entered_this_turn": 1},
				Counters:   map[string]int{"loyalty": 3},
			})
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["planeswalker_etb_this_turn"] = 1
		cs.description = "placed planeswalker on seat 0 + set planeswalker_etb_this_turn flag"

	case condScaffoldArtifactETBThisTurn:
		// "as long as an artifact entered the battlefield under your
		// control this turn" — Mechan Shieldmate, Shipwreck Sentry. Seed
		// one artifact on seat 0 and stamp the ETB-this-turn flag.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
				Card: &gameengine.Card{
					Name:  "Artifact ETB Setup",
					Owner: 0,
					Types: []string{"artifact"},
				},
				Controller: 0,
				Owner:      0,
				Flags:      map[string]int{"entered_this_turn": 1},
				Counters:   map[string]int{},
			})
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["artifact_etb_this_turn"] = 1
		cs.description = "placed artifact on seat 0 + set artifact_etb_this_turn flag"

	case condScaffoldStillOnBattlefield:
		// "if it's on the battlefield" / "is still on the battlefield" —
		// static self-presence check. srcPerm being on the battlefield is
		// the natural state inside applyConditionScaffolding; we just need
		// to stamp a flag so downstream observers can read it.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["still_on_battlefield"] = 1
		}
		cs.description = "set still_on_battlefield flag on srcPerm"

	case condScaffoldRevealLandOtherwiseHand:
		// "if it's a land card, put it onto the battlefield. otherwise,
		// put it into your hand" — Coiling Oracle / Nadu / Skyward Eye
		// Prophets. Seed a land card on top of seat 0's library so the
		// reveal hits the land branch.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			landCard := &gameengine.Card{
				Name:  "Top-of-Library Land",
				Owner: 0,
				Types: []string{"land", "forest"},
			}
			gs.Seats[0].Library = append([]*gameengine.Card{landCard}, gs.Seats[0].Library...)
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["top_of_library_is_land"] = 1
		cs.description = "placed land on top of seat 0 library (reveal-and-route)"

	// Era 2 audit additions — apply implementations.

	case condScaffoldVelocityCounters:
		// Aetherdrift racing vehicles — accumulate velocity counters as
		// the race progresses. Stamp the threshold on srcPerm.
		n := cs.count
		if n < 1 {
			n = 2
		}
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Counters["velocity"] < n {
				srcPerm.Counters["velocity"] = n
			}
		}
		cs.description = fmt.Sprintf("placed %d velocity counters on srcPerm", n)

	case condScaffoldNotDeclaredAttacker:
		// "isn't being declared as an attacker" — fires during
		// declare-attackers when the source is held back. Clear the
		// attacker flag on srcPerm explicitly so observers reading
		// "attacking" state see false.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["not_attacker"] = 1
			delete(srcPerm.Flags, "attacking")
			delete(srcPerm.Flags, "is_attacking")
		}
		gs.Phase, gs.Step = "combat", "declare_attackers"
		cs.description = "marked srcPerm not_attacker + set Phase=combat Step=declare_attackers"

	case condScaffoldManaValueLE:
		// "its mana value is N or less" — cascade-style cost filters
		// (Amped Raptor, Thunderous Velocipede). Lower srcPerm.Card.CMC
		// below the threshold so the cost predicate resolves true.
		threshold := cs.count
		if threshold < 1 {
			threshold = 4
		}
		if srcPerm != nil && srcPerm.Card != nil {
			if srcPerm.Card.CMC > threshold {
				srcPerm.Card.CMC = threshold
			}
		}
		cs.description = fmt.Sprintf("set srcPerm.Card.CMC <= %d (mana_value_le)", threshold)

	case condScaffoldCrewedBySubtype:
		// Kaladesh vehicles with subtype-gated crew triggers (Adrestia).
		// Place a matching pilot creature on seat 0 + mark srcPerm as
		// crewed with the subtype recorded so observers see the pilot.
		subtype := cs.subtype
		if subtype == "" {
			subtype = "pilot"
		}
		placeNamedFriendlyCreatureWithSubtype(gs, "Crew Pilot "+subtype, subtype)
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["crewed_this_turn"] = 1
			srcPerm.Flags["crewed_by_"+subtype] = 1
		}
		cs.description = fmt.Sprintf("placed %s pilot + flagged srcPerm crewed_by_%s", subtype, subtype)

	case condScaffoldIsSubtype:
		// "that creature is a(n) <subtype>" — Turtle Van / mutate
		// subtype gates. Stamp the subtype on srcPerm.Card.Types so
		// subtype predicates resolve true (subtypes live alongside
		// supertypes in Types in this codebase — see
		// placeNamedFriendlyCreatureWithSubtype).
		subtype := cs.subtype
		if subtype == "" {
			subtype = "creature"
		}
		if srcPerm != nil && srcPerm.Card != nil {
			has := false
			for _, t := range srcPerm.Card.Types {
				if t == subtype {
					has = true
					break
				}
			}
			if !has {
				srcPerm.Card.Types = append(srcPerm.Card.Types, subtype)
			}
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["is_"+subtype] = 1
		}
		cs.description = fmt.Sprintf("appended %q subtype to srcPerm.Types + is_%s flag", subtype, subtype)

	case condScaffoldAscendBlessing:
		// "if you have the city's blessing" / "ascend". Reuses the
		// existing primeAscend helper which places 10 permanents and
		// stamps the citys_blessing flag.
		primeAscend(gs)
		cs.description = "primeAscend: 10 permanents on seat 0 + citys_blessing flag"

	case condScaffoldEminenceCommandZone:
		// Eminence triggers fire while the commander is in the command
		// zone or on the battlefield. Mark seat 0 as controlling its
		// commander (so the on-battlefield branch passes) and stamp a
		// commander-in-zone flag for the explicit-zone branch.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["commander_in_command_zone"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["eminence_active"] = 1
		cs.description = "set commander_in_command_zone + eminence_active flags"

	case condScaffoldControlNCreatures:
		// "you control N or more creatures" — seed enough generic creatures
		// on seat 0 to satisfy the threshold (count+1 to guard against
		// the count itself being the source).
		n := cs.count
		if n < 1 {
			n = 3
		}
		seedSeatCreatures(gs, 0, n+1, "Threshold Creature", "")
		cs.description = fmt.Sprintf("seeded %d creatures on seat 0 (control_n_creatures n=%d)", n+1, n)

	case condScaffoldControlNLands:
		// "you control N or more lands" — seed Forests on seat 0. Distinct
		// from Domain (which needs 5 land TYPES, not 5 lands of one type).
		n := cs.count
		if n < 1 {
			n = 5
		}
		seedSeatLands(gs, 0, n+1, "Forest", "forest")
		cs.description = fmt.Sprintf("seeded %d Forests on seat 0 (control_n_lands n=%d)", n+1, n)

	case condScaffoldCounterReplacementBoost:
		// +1/+1 counter doubler replacement (Doubling Season, Hardened
		// Scales, Branching Evolution, Pir, Conclave Mentor). Stamp seat
		// and game flags the engine's counter-placement hook reads. Also
		// place a witness creature so the doubler has something to act on.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["counter_doubler_active"] = 1
			gs.Seats[0].Flags["plus_one_counter_doubler"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["counter_doubler_active"] = 1
		placeNamedFriendlyCreature(gs, "Counter Doubler Witness")
		cs.description = "set counter_doubler_active flag + placed witness creature"

	case condScaffoldTokenReplacementBoost:
		// Token doubler replacement (Anointed Procession, Parallel Lives,
		// Doubling Season's token half, Mondrak, Adrix and Nev).
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["token_doubler_active"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["token_doubler_active"] = 1
		cs.description = "set token_doubler_active flag on seat 0 + game"

	case condScaffoldPersistUndyingCheck:
		// "if it had no -1/-1 counters on it" (persist) / "if it had no
		// +1/+1 counters on it" (undying). The check fires at LTB-replace
		// time against the LAST counter state. Clear the named counter
		// from srcPerm so the predicate evaluates true.
		kind := cs.subtype
		if kind == "" {
			kind = "+1/+1"
		}
		if srcPerm != nil && srcPerm.Counters != nil {
			delete(srcPerm.Counters, kind)
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["persist_undying_check"] = 1
		}
		cs.description = fmt.Sprintf("cleared %q counters on srcPerm (persist/undying)", kind)

	case condScaffoldCardTypeReveal:
		// "if it's a {creature,land,permanent,...} card". Place a card of
		// the named type on top of the controller's library and stamp a
		// flag the reveal hook reads.
		sub := cs.subtype
		if sub == "" {
			sub = "creature"
		}
		card := &gameengine.Card{
			Name:  "Revealed " + sub,
			Owner: 0,
			Types: []string{sub},
		}
		if sub == "permanent" {
			card.Types = []string{"creature"}
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Library = append([]*gameengine.Card{card}, gs.Seats[0].Library...)
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["revealed_card_type_"+sub] = 1
		}
		cs.description = fmt.Sprintf("placed %q card on top of seat 0 library (card_type_reveal)", sub)

	case condScaffoldEquipmentAttached:
		// "as long as ~ is equipped" / aura-style attachment check on an
		// equipment source. Place a creature on seat 0 and link srcPerm
		// as its attached equipment.
		target := placeNamedFriendlyCreature(gs, "Equipped Creature")
		if srcPerm != nil && target != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["is_equipped"] = 1
			srcPerm.AttachedTo = target
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["has_equipment"] = 1
		}
		cs.description = "placed Equipped Creature on seat 0 and attached srcPerm"

	case condScaffoldHandSizeThreshold:
		// "fewer than N / N or more / N or fewer cards in hand". Adjust
		// seat 0 hand size to satisfy the predicate.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			target := cs.count
			switch cs.subtype {
			case "hand_size_lt":
				// fewer than N — set to N-1 (min 0)
				want := target - 1
				if want < 0 {
					want = 0
				}
				trimOrFillHand(gs, 0, want)
			case "hand_size_le":
				trimOrFillHand(gs, 0, target)
			case "hand_size_ge", "hand_size_threshold":
				want := target + 1
				if want < 1 {
					want = 1
				}
				trimOrFillHand(gs, 0, want)
			default:
				trimOrFillHand(gs, 0, target+1)
			}
		}
		cs.description = fmt.Sprintf("adjusted seat 0 hand for %s count=%d", cs.subtype, cs.count)

	case condScaffoldPutCounterThisTurn:
		// "you put a counter on ~ this turn". Log a counter-placement
		// event and stamp the per-turn flag.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["put_counter_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["put_counter_this_turn"] = 1
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "counter_placed",
			Seat:   0,
			Source: "thor_priming",
		})
		cs.description = "set put_counter_this_turn flag + logged counter_placed event"

	case condScaffoldCastNSpellsThisTurn:
		// "you've cast N or more spells this turn" — increment Turn.Casts
		// to N+1 so the threshold is strictly cleared, and bump legacy
		// counters in lockstep.
		n := cs.count
		if n < 1 {
			n = 2
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			for i := 0; i < n+1; i++ {
				seat.SpellsCastThisTurn++
				seat.Turn.SpellsCast++
				seat.Turn.Casts = append(seat.Turn.Casts, gameengine.CastRecord{
					CardName:  fmt.Sprintf("Cast %d", i),
					Types:     []string{"instant"},
					ManaValue: 1,
				})
			}
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			seat.Flags["cast_spell_this_turn"] = 1
			seat.Flags["spells_cast_this_turn"] = seat.Turn.SpellsCast
		}
		gs.SpellsCastThisTurn += n + 1
		cs.description = fmt.Sprintf("incremented Turn.SpellsCast to %d (cast_n_spells threshold=%d)", n+1, n)

	case condScaffoldFullParty:
		// Zendikar Rising 'full party' — control 4 creatures with the
		// four distinct party classes: warrior, wizard, cleric, rogue.
		for _, cls := range []string{"warrior", "wizard", "cleric", "rogue"} {
			placeNamedFriendlyCreatureWithSubtype(gs, "Party "+cls, cls)
		}
		cs.description = "placed warrior + wizard + cleric + rogue on seat 0 (full party)"

	case condScaffoldTotalToughness:
		// "creatures you control have total toughness N or greater". Seed
		// creatures totaling at least N+1 toughness via two 4-toughness
		// creatures by default; scale up for very high thresholds.
		want := cs.count
		if want < 1 {
			want = 8
		}
		placed := 0
		for placed < want+1 {
			t := 4
			remaining := (want + 1) - placed
			if remaining < t {
				t = remaining
			}
			placePoweredCreature(gs, 0, fmt.Sprintf("Toughness Setup %d", placed), 1, t)
			placed += t
		}
		cs.description = fmt.Sprintf("seeded creatures totaling >=%d toughness on seat 0", want+1)

	case condScaffoldMainPhaseOrFirstCombat:
		// "it's your main phase" / "it's the first combat phase". Set the
		// phase/step directly so phase-gated abilities evaluate true.
		switch cs.subtype {
		case "first_combat_phase":
			gs.Phase = "combat"
			gs.Step = "begin_combat"
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags["first_combat_phase"] = 1
		default:
			gs.Phase = "precombat_main"
			gs.Step = "main"
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags["main_phase"] = 1
		}
		cs.description = fmt.Sprintf("set Phase=%q Step=%q (%s)", gs.Phase, gs.Step, cs.subtype)

	// Era 1 R60 sweep — apply implementations.

	case condScaffoldTributeNotPaid:
		// Tribute (KTK) — the trigger fires when the opponent who was
		// offered tribute DECLINED to pay it. Stamp the source so the
		// declined-tribute predicate resolves true.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["tribute_paid"] = 0
			srcPerm.Flags["tribute_declined"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["tribute_declined_this_turn"] = 1
		cs.description = "set srcPerm.Flags[tribute_declined]=1 (tribute not paid)"

	case condScaffoldBargained:
		// Bargain (WOE) — the source was cast with bargain (sacrificing an
		// artifact/enchantment/token for an alt benefit). Same shape as
		// kicker; stamp the per-source flag the engine reads.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["bargained"] = 1
			srcPerm.Flags["paid_optional_cost"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["bargained_this_turn"] = 1
		cs.description = "set srcPerm.Flags[bargained]=1 + paid_optional_cost flag"

	case condScaffoldSharesCreatureType:
		// "if it shares a creature type with this creature" — place a
		// creature of the matching subtype on top of seat 0's library so
		// the reveal succeeds. Default subtype is goblin/elf (high-density
		// tribal); subtype carries whatever the detector lifted.
		subtype := cs.subtype
		if subtype == "" {
			subtype = "goblin"
		}
		if srcPerm != nil && srcPerm.Card != nil {
			// Stamp the matching subtype on srcPerm so the "this creature"
			// side of the compare also carries it.
			has := false
			for _, t := range srcPerm.Card.Types {
				if t == subtype {
					has = true
					break
				}
			}
			if !has {
				srcPerm.Card.Types = append(srcPerm.Card.Types, subtype)
			}
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			match := &gameengine.Card{
				Name:          "Shared-Type " + subtype,
				Owner:         0,
				Types:         []string{"creature", subtype},
				BasePower:     1,
				BaseToughness: 1,
			}
			gs.Seats[0].Library = append([]*gameengine.Card{match}, gs.Seats[0].Library...)
		}
		cs.description = fmt.Sprintf("placed %q creature atop seat 0 library + stamped subtype on srcPerm", subtype)

	case condScaffoldNotTheirTurn:
		// "it's not their turn" — push the active player off seat 0 so any
		// "during another player's turn" predicate keyed off gs.Active sees
		// a non-controlling seat.
		if len(gs.Seats) > 1 {
			gs.Active = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["off_turn"] = 1
		cs.description = "set gs.Active=1 + off_turn flag (not their turn)"

	case condScaffoldMoreLifeThanOpponent:
		// "you have more life than an opponent" — bump seat 0 well above
		// seat 1's life so the per-opponent compare resolves true for at
		// least one opponent.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Life = 40
		}
		if len(gs.Seats) > 1 && gs.Seats[1] != nil {
			gs.Seats[1].Life = 20
		}
		cs.description = "set seat 0 life=40, seat 1 life=20 (more life than opponent)"

	case condScaffoldTimeCounterOnSelf:
		// "had no time counters on it" — suspend completion check. Clear
		// "time" counters on srcPerm so the predicate resolves true.
		if srcPerm != nil {
			if srcPerm.Counters != nil {
				delete(srcPerm.Counters, "time")
			}
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["no_time_counters"] = 1
			srcPerm.Flags["suspended_completed"] = 1
		}
		cs.description = "cleared time counters on srcPerm + suspended_completed flag"

	case condScaffoldWasntCast:
		// "it wasn't cast" — token / put-onto-battlefield path predicate.
		// Stamp the source so observers reading the cast-status flag see
		// the not-cast value.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["wasnt_cast"] = 1
			delete(srcPerm.Flags, "cast_from_hand")
			delete(srcPerm.Flags, "cast_spell_this_turn")
		}
		cs.description = "set srcPerm.Flags[wasnt_cast]=1 (token / put-onto-battlefield path)"

	case condScaffoldLostLifeLastTurn:
		// "you lost life last turn" — the prior-turn snapshot. Stamp the
		// per-seat flag the engine reads during start-of-turn checks.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["lost_life_last_turn"] = 3
			gs.Seats[0].Flags["life_lost_last_turn"] = 3
		}
		cs.description = "set seat 0 Flags[lost_life_last_turn]=3"

	case condScaffoldDamageDealtToSelfTurn:
		// "N or more damage was dealt to it this turn" — stamp the
		// MarkedDamage / damage_taken counter on srcPerm. Engine reads
		// MarkedDamage for the §704.5g destroy check; we stamp a flag
		// in parallel for direct lookups.
		amt := cs.count
		if amt < 1 {
			amt = 4
		}
		if srcPerm != nil {
			srcPerm.MarkedDamage = amt
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["damage_taken_this_turn"] = amt
		}
		cs.description = fmt.Sprintf("set srcPerm.MarkedDamage=%d + damage_taken_this_turn flag", amt)

	case condScaffoldMoreCardsThanOpponents:
		// "you have more cards in hand than each opponent" — top up seat 0
		// hand to 7, trim other seats' hands to 1 so the per-opponent
		// compare resolves true for every opponent.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			trimOrFillHand(gs, 0, 7)
		}
		for i := 1; i < len(gs.Seats); i++ {
			if gs.Seats[i] != nil {
				trimOrFillHand(gs, i, 1)
			}
		}
		cs.description = "set seat 0 hand=7, opponents hand=1 (more cards than each opponent)"

	case condScaffoldSelfIsUntapped:
		// Mirror of self_is_tapped — source must be UNTAPPED for the
		// ability to qualify (Sphinx Sovereign, Nim Abomination).
		if srcPerm != nil {
			srcPerm.Tapped = false
		}
		cs.description = "untapped srcPerm (self_is_untapped active)"

	case condScaffoldSelfHasNoCounter:
		// Inverse of self_has_counter — clear the named counter so the
		// "has no <kind> counters" predicate resolves true.
		kind := cs.subtype
		if kind == "" {
			kind = "oil"
		}
		if srcPerm != nil && srcPerm.Counters != nil {
			delete(srcPerm.Counters, kind)
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["no_"+kind+"_counters"] = 1
		}
		cs.description = fmt.Sprintf("cleared %q counters on srcPerm (no-counter predicate)", kind)

	case condScaffoldSurgeCostPaid:
		// Surge (EMN) — alt-cost cousin of kicker. Stamp the alt-cost flag
		// and route through the same paid_optional_cost flag the engine
		// reads for kicker-style predicates.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["surge_paid"] = 1
			srcPerm.Flags["paid_optional_cost"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["surge_paid"] = 1
		cs.description = "set srcPerm.Flags[surge_paid]=1 + paid_optional_cost flag"

	case condScaffoldLibraryEmpty:
		// "your library has no cards in it" — Jace win-con. Empty seat 0
		// library so the predicate resolves true. Keep other seats alone:
		// the trigger is keyed on the source's controller, not globally.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Library = nil
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["library_empty"] = 1
		}
		cs.description = "emptied seat 0 library + set library_empty flag"

	case condScaffoldColoredManaSpent:
		// "{R}{R}/{G}{G}/{U}{U}/{R}/treasure was spent to cast it" —
		// stamp a per-source mana-payment flag the cast-condition reader
		// consults. Generalizes the existing mana_spent scaffold which
		// only tracks total CMC.
		color := cs.subtype
		if color == "" {
			color = "C"
		}
		amt := cs.count
		if amt < 1 {
			amt = 1
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["mana_spent_"+color] = amt
			// Compatibility with the generic mana_spent flag for callers
			// that don't break down by color.
			if srcPerm.Flags["mana_spent"] < amt {
				srcPerm.Flags["mana_spent"] = amt
			}
		}
		cs.description = fmt.Sprintf("set srcPerm.Flags[mana_spent_%s]=%d", color, amt)

	case condScaffoldSelfIsSuspended:
		// "this card is suspended" — source lives in exile with the
		// "suspended" flag and a "time" counter on its Card. Move srcPerm's
		// Card (if any) into seat 0 exile with the suspend state stamped.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			suspendCard := &gameengine.Card{
				Name:  "Suspended Card Setup",
				Owner: 0,
				Types: []string{"instant"},
			}
			gs.Seats[0].Exile = append(gs.Seats[0].Exile, suspendCard)
		}
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["suspended"] = 1
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			if srcPerm.Counters["time"] < 1 {
				srcPerm.Counters["time"] = 2
			}
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["suspend_active"] = 1
		cs.description = "placed suspended card in seat 0 exile + stamped suspended flag on srcPerm"

	case condScaffoldLifeAboveStarting:
		// "life total is greater than your starting life total" — set seat
		// 0 life well above Commander starting (40). 50 covers both
		// 20-life formats and Commander.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Life = 50
		}
		cs.description = "set seat 0 life=50 (above starting life total)"

	case condScaffoldOpponentMoreLife:
		// Mirror of MoreLifeThanOpponent — opponent must have MORE life
		// than you for the trigger to fire.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			gs.Seats[0].Life = 5
		}
		if len(gs.Seats) > 1 && gs.Seats[1] != nil {
			gs.Seats[1].Life = 40
		}
		cs.description = "set seat 0 life=5, seat 1 life=40 (opponent has more life)"

	case condScaffoldWasntBlocking:
		// "it wasn't blocking" — combat exit predicate (Guildsworn
		// Prowler). Clear any blocking state on srcPerm and set the flag
		// the post-combat reader consults.
		if srcPerm != nil {
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			delete(srcPerm.Flags, "blocking")
			delete(srcPerm.Flags, "is_blocking")
			srcPerm.Flags["wasnt_blocking"] = 1
		}
		cs.description = "cleared blocking flags on srcPerm + set wasnt_blocking"

	case condScaffoldDiscardedNonland:
		// "you discarded a nonland card" — Spider-Verse / Doctor Doom.
		// Push a nonland card into seat 0's graveyard and bump the
		// Discarded counter so per-card handlers that read either path
		// observe a discard event. Also stamp the gs.Flags discard
		// counter the engine sets in DiscardCard.
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			discarded := &gameengine.Card{
				Name:  "Discarded Nonland",
				Owner: 0,
				Types: []string{"creature"},
			}
			seat.Graveyard = append(seat.Graveyard, discarded)
			seat.Turn.Discarded++
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			seat.Flags["discarded_this_turn"] = 1
			seat.Flags["discarded_nonland_this_turn"] = 1
		}
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["discarded_0"]++
		gs.LogEvent(gameengine.Event{
			Kind:   "card_discarded",
			Seat:   0,
			Source: "thor_priming",
		})
		cs.description = "discarded nonland card to seat 0 graveyard + flags + event"

	case condScaffoldControlNTappedCreatures:
		// "you control N or more tapped creatures" — EoE Pilots. Seed
		// creatures and flip each one's Tapped bit so the threshold is
		// satisfied (count+1 to clear strict-greater predicates too).
		n := cs.count
		if n < 1 {
			n = 2
		}
		seedSeatCreatures(gs, 0, n+1, "Tapped Pilot", "")
		tapped := 0
		want := n + 1
		for _, p := range gs.Seats[0].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			isCreature := false
			for _, t := range p.Card.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if !isCreature {
				continue
			}
			p.Tapped = true
			tapped++
			if tapped >= want {
				break
			}
		}
		cs.description = fmt.Sprintf("tapped %d creatures on seat 0 (control_n_tapped_creatures n=%d)", tapped, n)

	case condScaffoldGainedNLifeThisTurn:
		// "you gained N or more life this turn" — count variant of
		// GainedLifeThisTurn. Bump Turn.LifeGained directly so the
		// engine's threshold checks see at least N+1 gained.
		n := cs.count
		if n < 1 {
			n = 3
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			seat.Turn.LifeGained += n + 1
			seat.Life += n + 1
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			seat.Flags["gained_life_this_turn"] = 1
			seat.Flags["life_gained_this_turn"] = seat.Turn.LifeGained
		}
		cs.description = fmt.Sprintf("bumped seat 0 LifeGained by %d (threshold=%d)", n+1, n)

	case condScaffoldDrawnNCardsThisTurn:
		// "you've drawn N or more cards this turn" — Avatar TLA. Bump
		// Turn.CardsDrawn and the legacy drawn_card_this_turn flag.
		n := cs.count
		if n < 1 {
			n = 2
		}
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			seat := gs.Seats[0]
			seat.Turn.CardsDrawn += n + 1
			if seat.Flags == nil {
				seat.Flags = map[string]int{}
			}
			seat.Flags["drawn_card_this_turn"] = 1
			seat.Flags["cards_drawn_this_turn"] = seat.Turn.CardsDrawn
		}
		cs.description = fmt.Sprintf("bumped seat 0 CardsDrawn by %d (threshold=%d)", n+1, n)

	case condScaffoldStartingPlayer:
		// "you were the starting player" / "you weren't the starting
		// player". The engine doesn't expose a dedicated StartingPlayer
		// field, but the convention used by per_card handlers is to
		// stamp gs.Flags["starting_player_seat"] = N and the seat-level
		// "is_starting_player" flag. Set both consistently so the
		// predicate has data to read regardless of which convention the
		// AST handler picks up.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		startSeat := 0
		if cs.subtype == "not_starting" {
			startSeat = 1
		}
		gs.Flags["starting_player_seat"] = startSeat
		for i, s := range gs.Seats {
			if s == nil {
				continue
			}
			if s.Flags == nil {
				s.Flags = map[string]int{}
			}
			if i == startSeat {
				s.Flags["is_starting_player"] = 1
			} else {
				delete(s.Flags, "is_starting_player")
			}
		}
		cs.description = fmt.Sprintf("set starting_player_seat=%d (subtype=%s)", startSeat, cs.subtype)

	case condScaffoldQuestCounters:
		// "it has N or more quest counters on it" — Avatar TLA Ascension
		// triple. Stamp the named counter on srcPerm. Use count+1 so
		// strict-greater thresholds also pass.
		n := cs.count
		if n < 1 {
			n = 4
		}
		if srcPerm != nil {
			if srcPerm.Counters == nil {
				srcPerm.Counters = map[string]int{}
			}
			srcPerm.Counters["quest"] = n + 1
			if srcPerm.Flags == nil {
				srcPerm.Flags = map[string]int{}
			}
			srcPerm.Flags["quest_counters"] = n + 1
		}
		cs.description = fmt.Sprintf("placed %d quest counters on srcPerm (threshold=%d)", n+1, n)

	case condScaffoldDragonBeheld:
		// "a Dragon was beheld" — Tarkir Dragonstorm. New mechanic; no
		// dedicated engine field yet. Stamp both a gs-level and seat-level
		// flag the per_card Behold handlers (when they land) can read.
		// Also place a Dragon creature on seat 0 as a witness so
		// downstream "target/sacrifice/exile a Dragon" effects have
		// something to operate on.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["dragon_beheld_this_turn"] = 1
		if len(gs.Seats) > 0 && gs.Seats[0] != nil {
			if gs.Seats[0].Flags == nil {
				gs.Seats[0].Flags = map[string]int{}
			}
			gs.Seats[0].Flags["dragon_beheld_this_turn"] = 1
		}
		placeNamedFriendlyCreatureWithSubtype(gs, "Beheld Dragon", "dragon")
		gs.LogEvent(gameengine.Event{
			Kind:   "dragon_beheld",
			Seat:   0,
			Source: "thor_priming",
		})
		cs.description = "set dragon_beheld_this_turn flags + placed Dragon witness + event"
	}
	return cs
}

// traceConditionScaffolding emits a CONDITION_SETUP entry describing the
// scaffold that would be / was applied for cond. Pure observation — no
// mutation. Safe to call with a nil tracer (no-op).
func traceConditionScaffolding(cond *gameast.Condition, tr *Tracer) {
	if tr == nil {
		return
	}
	cs := detectConditionScaffold(cond)
	if cs.kind == condScaffoldNone {
		return
	}
	// Build the description without mutating; mirrors apply's switch.
	desc := ""
	switch cs.kind {
	case condScaffoldOpponentMoreLands:
		desc = "seeded 6 Plains on seat 1"
	case condScaffoldYouControlSubtype:
		desc = fmt.Sprintf("placed %s creature on seat 0", cs.subtype)
	case condScaffoldCreatureDiedThisTurn:
		desc = "set creature_died_this_turn flag + added creature to graveyard"
	case condScaffoldCreatureCardsInGraveyard:
		desc = fmt.Sprintf("populated seat 0 graveyard with %d creature cards", cs.count)
	case condScaffoldCardInGraveyard:
		desc = "placed creature card in seat 0 graveyard"
	case condScaffoldEnergyThreshold:
		desc = fmt.Sprintf("set seat 0 energy counters to %d", cs.count)
	case condScaffoldGainedLifeThisTurn:
		desc = "gained 3 life for seat 0 (life_gained_this_turn flag set)"
	case condScaffoldCastSpellThisTurn:
		desc = "incremented spell cast counters for seat 0"
	case condScaffoldCreatureETBThisTurn:
		desc = "placed ETB Witness creature and set creature_etb_this_turn flag"
	case condScaffoldDrawnCardThisTurn:
		desc = "set drawn_card_this_turn flag + filled library"
	case condScaffoldAttackedThisTurn:
		desc = "set attacked_this_turn flag on seat 0 and game"
	case condScaffoldSacrificedThisTurn:
		desc = "set sacrificed_this_turn flag + placed creature in graveyard"
	case condScaffoldCombatDamageDealt:
		desc = "set combat_damage_dealt_this_turn flag"
	case condScaffoldLandfallThisTurn:
		desc = "set landfall_this_turn flag + placed land on seat 0"
	case condScaffoldDiscardedThisTurn:
		desc = "set discarded_this_turn flag + placed card in graveyard"
	case condScaffoldEnchantedCreature:
		desc = "placed Enchanted Target creature for aura attachment"
	case condScaffoldOpponentLostLife:
		desc = "opponent (seat 1) lost 3 life this turn"
	case condScaffoldLifeAboveThreshold:
		desc = fmt.Sprintf("set seat 0 life to %d (above threshold)", cs.threshold)
	case condScaffoldLifeBelowThreshold:
		desc = fmt.Sprintf("set seat 0 life to %d (below threshold)", cs.threshold)
	case condScaffoldUpkeepPhase:
		desc = "set game phase to upkeep"
	case condScaffoldHellbent:
		desc = "emptied seat 0 hand (hellbent active)"
	case condScaffoldMonarch:
		desc = "made seat 0 the monarch"
	case condScaffoldInitiative:
		desc = "gave seat 0 the initiative"
	case condScaffoldDelirium:
		desc = "seeded seat 0 graveyard with 4 distinct card types"
	case condScaffoldSpellMastery:
		desc = "seeded seat 0 graveyard with 2 instant/sorcery cards"
	case condScaffoldRevolt:
		desc = "logged sacrifice event for seat 0 (revolt active)"
	case condScaffoldMetalcraft:
		desc = "placed 3 artifacts on seat 0 (metalcraft active)"
	case condScaffoldFerocious:
		desc = "placed 4/4 creature on seat 0 (ferocious active)"
	case condScaffoldFormidable:
		desc = "placed creatures totaling 8 power on seat 0 (formidable active)"
	case condScaffoldPaidOptionalCost:
		desc = fmt.Sprintf("set kicked=%d on srcPerm + paid_optional_cost flag", maxInt(cs.count, 1))
	case condScaffoldForEach:
		desc = fmt.Sprintf("seeded %d %s on seat 0 (for_each)", maxInt(cs.count, 3), nonEmpty(cs.subtype, "creature"))
	case condScaffoldETBAs:
		if cs.subtype == "choose_mode" || cs.subtype == "etb_modal" {
			desc = "set ETB modal flag on srcPerm"
		} else {
			desc = fmt.Sprintf("placed %d %q counters on srcPerm", maxInt(cs.count, 1), cs.subtype)
		}
	case condScaffoldDidPriorAction:
		desc = fmt.Sprintf("primed Turn counter for did_prior_action verb=%q", cs.subtype)
	case condScaffoldCycled:
		desc = "logged cycle event + set cycled_this_turn on seat 0"
	case condScaffoldMutates:
		desc = "set srcPerm.Flags[mutated]=1 + logged mutate event"
	case condScaffoldUnlockDoor:
		desc = "set srcPerm.Flags[unlocked]=1 + logged unlock_door event"
	case condScaffoldPriorTurnSpellCount:
		desc = fmt.Sprintf("set SpellsCastLastTurn=%d on all seats", cs.count)
	case condScaffoldPairedSoulbond:
		desc = "placed soulbond partner + paired"
	case condScaffoldTurnedFaceUp:
		desc = "set source face-down then TurnFaceUp"
	case condScaffoldBeginningOfOrdinalStep:
		desc = fmt.Sprintf("set Phase/Step to %q", cs.subtype)
	case condScaffoldTribeYouControlETB:
		desc = fmt.Sprintf("placed %s creature on seat 0 (tribe-ETB)", cs.subtype)
	case condScaffoldManaSpentThreshold:
		desc = fmt.Sprintf("stamped srcPerm mana_spent=%d (+CastRecord)", cs.count+2)
	case condScaffoldAnyPlayerPhase:
		desc = fmt.Sprintf("set Phase/Step for any-player %s, Active=1", cs.subtype)
	case condScaffoldDelayedDrawNextUpkeep:
		desc = "set Phase=upkeep Turn++ + filled library (delayed draw next upkeep)"
	case condScaffoldETBModalChoice:
		desc = fmt.Sprintf("set ETB modal choice=%s on srcPerm", cs.subtype)
	case condScaffoldBecomesTapped:
		desc = "tapped subject permanent + logged becomes_tapped event"
	case condScaffoldBecomesTarget:
		desc = "pushed targeting spell on stack + flagged permanent + logged becomes_target"
	case condScaffoldUntilEOTDelayed:
		desc = "set Phase=ending Step=end_step/cleanup + delayed_eot_trigger_active"
	case condScaffoldLandPlayOrTap:
		desc = fmt.Sprintf("seeded lands + logged land_played/tapped_for_mana (subtype=%s)", cs.subtype)
	case condScaffoldETBTappedUnless:
		desc = "satisfied etb_tapped_unless clause + marked source untapped"
	case condScaffoldDomain:
		desc = "seeded 5 distinct basic land types on seat 0 (domain)"
	case condScaffoldETBIf:
		desc = "etb_if raw-text routed (+ generic ETB flag)"
	case condScaffoldRepeatN:
		desc = fmt.Sprintf("stamped repeat_n=%d", cs.count)
	case condScaffoldLieutenant:
		desc = "placed commander stand-in + set controls_commander flag"
	case condScaffoldKiCountersGE2:
		desc = fmt.Sprintf("placed %d ki counters on srcPerm", cs.count)
	case condScaffoldSelfIsTapped:
		desc = "tapped srcPerm (self_is_tapped)"
	case condScaffoldAttackedOrBlockedCombat:
		desc = "set source attacked/blocked combat flags + seat attacked_this_turn"
	case condScaffoldCoven:
		desc = "placed 3 creatures with different powers (coven)"
	case condScaffoldSelfHasCounter:
		desc = fmt.Sprintf("placed %d %q counters on srcPerm", maxInt(cs.count, 1), cs.subtype)
	case condScaffoldDidntAttackThisTurn:
		desc = "cleared attacked_this_turn (didnt_attack_this_turn)"
	case condScaffoldDealtDamageOpponentTurn:
		desc = "set dealt_damage_to_opponent_this_turn + logged damage"
	case condScaffoldItWasCreature:
		desc = "placed creature in graveyard + set last_left_was_creature flag"
	case condScaffoldNoCreaturesOnBattlefield:
		desc = "removed all creature permanents (no_creatures_on_battlefield)"
	case condScaffoldHadCountersOnIt:
		desc = "stamped +1/+1 counter + had_counters flag on srcPerm"
	case condScaffoldYouCastFromHand:
		desc = "appended cast record + cast_from_hand flag on srcPerm"
	case condScaffoldPlaneswalkerETBThisTurn:
		desc = "placed planeswalker + planeswalker_etb_this_turn flag"
	case condScaffoldArtifactETBThisTurn:
		desc = "placed artifact + artifact_etb_this_turn flag"
	case condScaffoldStillOnBattlefield:
		desc = "stamped still_on_battlefield flag on srcPerm"
	case condScaffoldRevealLandOtherwiseHand:
		desc = "placed land on top of seat 0 library (reveal-and-route)"
	case condScaffoldVelocityCounters:
		desc = fmt.Sprintf("placed %d velocity counters on srcPerm", maxInt(cs.count, 2))
	case condScaffoldNotDeclaredAttacker:
		desc = "marked srcPerm not_attacker + Phase=combat declare_attackers"
	case condScaffoldManaValueLE:
		desc = fmt.Sprintf("set srcPerm.Card.CMC <= %d (mana_value_le)", maxInt(cs.count, 4))
	case condScaffoldCrewedBySubtype:
		desc = fmt.Sprintf("placed %s pilot + crewed_by_%s flag", nonEmpty(cs.subtype, "pilot"), nonEmpty(cs.subtype, "pilot"))
	case condScaffoldIsSubtype:
		desc = fmt.Sprintf("stamped %q subtype + is_%s flag on srcPerm", nonEmpty(cs.subtype, "creature"), nonEmpty(cs.subtype, "creature"))
	case condScaffoldAscendBlessing:
		desc = "primeAscend: 10 permanents + citys_blessing flag"
	case condScaffoldEminenceCommandZone:
		desc = "set commander_in_command_zone + eminence_active flags"
	case condScaffoldControlNCreatures:
		desc = fmt.Sprintf("seeded %d creatures on seat 0 (control_n_creatures)", cs.count+1)
	case condScaffoldControlNLands:
		desc = fmt.Sprintf("seeded %d Forests on seat 0 (control_n_lands)", cs.count+1)
	case condScaffoldCounterReplacementBoost:
		desc = "set counter_doubler_active flag + witness creature"
	case condScaffoldTokenReplacementBoost:
		desc = "set token_doubler_active flag on seat 0 + game"
	case condScaffoldPersistUndyingCheck:
		desc = fmt.Sprintf("cleared %q counters on srcPerm (persist/undying)", cs.subtype)
	case condScaffoldCardTypeReveal:
		desc = fmt.Sprintf("placed %q card atop seat 0 library (card_type_reveal)", cs.subtype)
	case condScaffoldEquipmentAttached:
		desc = "placed Equipped Creature + attached srcPerm"
	case condScaffoldHandSizeThreshold:
		desc = fmt.Sprintf("adjusted seat 0 hand (%s n=%d)", cs.subtype, cs.count)
	case condScaffoldPutCounterThisTurn:
		desc = "set put_counter_this_turn flag + logged counter_placed event"
	case condScaffoldCastNSpellsThisTurn:
		desc = fmt.Sprintf("incremented Turn.SpellsCast to %d (cast_n_spells)", cs.count+1)
	case condScaffoldFullParty:
		desc = "placed warrior+wizard+cleric+rogue (full party)"
	case condScaffoldTotalToughness:
		desc = fmt.Sprintf("seeded creatures totaling >=%d toughness", cs.count+1)
	case condScaffoldMainPhaseOrFirstCombat:
		desc = fmt.Sprintf("set Phase/Step for %s", cs.subtype)

	// Era 1 R60 sweep — trace descriptions.
	case condScaffoldTributeNotPaid:
		desc = "set srcPerm.Flags[tribute_declined]=1 (tribute not paid)"
	case condScaffoldBargained:
		desc = "set srcPerm.Flags[bargained]=1 + paid_optional_cost"
	case condScaffoldSharesCreatureType:
		desc = fmt.Sprintf("placed %q creature atop seat 0 library + stamped subtype on srcPerm", nonEmpty(cs.subtype, "goblin"))
	case condScaffoldNotTheirTurn:
		desc = "set gs.Active=1 + off_turn flag (not their turn)"
	case condScaffoldMoreLifeThanOpponent:
		desc = "set seat 0 life=40, seat 1 life=20 (more life than opponent)"
	case condScaffoldTimeCounterOnSelf:
		desc = "cleared time counters on srcPerm + suspended_completed flag"
	case condScaffoldWasntCast:
		desc = "set srcPerm.Flags[wasnt_cast]=1 (token / put-onto-bf)"
	case condScaffoldLostLifeLastTurn:
		desc = "set seat 0 Flags[lost_life_last_turn]=3"
	case condScaffoldDamageDealtToSelfTurn:
		desc = fmt.Sprintf("set srcPerm.MarkedDamage=%d + damage_taken_this_turn", maxInt(cs.count, 4))
	case condScaffoldMoreCardsThanOpponents:
		desc = "set seat 0 hand=7, opponents hand=1 (more cards than each opponent)"
	case condScaffoldSelfIsUntapped:
		desc = "untapped srcPerm (self_is_untapped)"
	case condScaffoldSelfHasNoCounter:
		desc = fmt.Sprintf("cleared %q counters on srcPerm (no-counter)", nonEmpty(cs.subtype, "oil"))
	case condScaffoldSurgeCostPaid:
		desc = "set srcPerm.Flags[surge_paid]=1 + paid_optional_cost"
	case condScaffoldLibraryEmpty:
		desc = "emptied seat 0 library + library_empty flag"
	case condScaffoldColoredManaSpent:
		desc = fmt.Sprintf("set srcPerm.Flags[mana_spent_%s]=%d", nonEmpty(cs.subtype, "C"), maxInt(cs.count, 1))
	case condScaffoldSelfIsSuspended:
		desc = "placed suspended card in seat 0 exile + suspended flag on srcPerm"
	case condScaffoldLifeAboveStarting:
		desc = "set seat 0 life=50 (above starting life total)"
	case condScaffoldOpponentMoreLife:
		desc = "set seat 0 life=5, seat 1 life=40 (opponent has more life)"
	case condScaffoldWasntBlocking:
		desc = "cleared blocking flags on srcPerm + wasnt_blocking"

	// Era 3 sweep r60 — trace descriptions.
	case condScaffoldDiscardedNonland:
		desc = "discarded nonland to seat 0 graveyard + flags + event"
	case condScaffoldControlNTappedCreatures:
		desc = fmt.Sprintf("tapped %d creatures on seat 0 (control_n_tapped n=%d)", cs.count+1, cs.count)
	case condScaffoldGainedNLifeThisTurn:
		desc = fmt.Sprintf("bumped seat 0 LifeGained by %d (threshold=%d)", cs.count+1, cs.count)
	case condScaffoldDrawnNCardsThisTurn:
		desc = fmt.Sprintf("bumped seat 0 CardsDrawn by %d (threshold=%d)", cs.count+1, cs.count)
	case condScaffoldStartingPlayer:
		desc = fmt.Sprintf("set starting_player_seat (subtype=%s)", cs.subtype)
	case condScaffoldQuestCounters:
		desc = fmt.Sprintf("placed %d quest counters on srcPerm (threshold=%d)", cs.count+1, cs.count)
	case condScaffoldDragonBeheld:
		desc = "set dragon_beheld_this_turn flags + Dragon witness + event"
	}
	tr.Record("CONDITION_SETUP", "%q → %s", cs.rawText, desc)
}

// ---------------------------------------------------------------------------
// Mutation helpers used by applyConditionScaffolding.
// ---------------------------------------------------------------------------

// seedSeatLands tops up `seat` so it controls at least `count` basic lands
// of the given subtype. Existing matching lands are kept; only the deficit
// is appended.
func seedSeatLands(gs *gameengine.GameState, seat, count int, name, subtype string) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isLand := false
		for _, t := range p.Card.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			have++
		}
	}
	for i := have; i < count; i++ {
		perm := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:  fmt.Sprintf("%s %d", name, i),
				Owner: seat,
				Types: []string{"land", subtype},
			},
			Controller: seat,
			Owner:      seat,
			Flags:      map[string]int{},
			Counters:   map[string]int{},
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	}
}

// placeNamedFriendlyCreatureWithSubtype is like placeNamedFriendlyCreature
// but appends a creature subtype (Wizard, Knight, etc.) so "you control
// another <subtype>" checks resolve true.
func placeNamedFriendlyCreatureWithSubtype(gs *gameengine.GameState, name, subtype string) *gameengine.Permanent {
	perm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          name,
			Owner:         0,
			Types:         []string{"creature", subtype},
			BasePower:     1,
			BaseToughness: 1,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	return perm
}

// topUpGraveyardCreatures ensures `seat`'s graveyard contains at least n
// creature cards, appending tokens-by-name when short.
func topUpGraveyardCreatures(gs *gameengine.GameState, seat, n int) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := 0
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if t == "creature" {
				have++
				break
			}
		}
	}
	for i := have; i < n; i++ {
		gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, &gameengine.Card{
			Name:          fmt.Sprintf("GraveCreatureSetup %d-%d", seat, i),
			Owner:         seat,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
		})
	}
}

// seedDeliriumGraveyard tops up `seat`'s graveyard so CheckDelirium passes:
// 4 cards covering 4 distinct delirium-counted types. Existing graveyard
// contents are kept; only missing types are appended.
func seedDeliriumGraveyard(gs *gameengine.GameState, seat int) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := map[string]bool{}
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			have[strings.ToLower(t)] = true
		}
	}
	// Cover the four most universal types — they don't overlap in a way
	// that would let CheckDelirium double-count.
	for _, ty := range []string{"creature", "instant", "sorcery", "artifact"} {
		if have[ty] {
			continue
		}
		gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, &gameengine.Card{
			Name:  fmt.Sprintf("Delirium Setup %s", ty),
			Owner: seat,
			Types: []string{ty},
		})
	}
}

// seedSpellMasteryGraveyard tops up `seat`'s graveyard with 2 instant or
// sorcery cards so CheckSpellMastery passes. Counts existing entries so
// repeated calls don't pile up duplicates.
func seedSpellMasteryGraveyard(gs *gameengine.GameState, seat int) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := 0
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			lower := strings.ToLower(t)
			if lower == "instant" || lower == "sorcery" {
				have++
				break
			}
		}
	}
	for i := have; i < 2; i++ {
		ty := "instant"
		if i%2 == 1 {
			ty = "sorcery"
		}
		gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, &gameengine.Card{
			Name:  fmt.Sprintf("SpellMastery Setup %d", i),
			Owner: seat,
			Types: []string{ty},
		})
	}
}

// seedSeatArtifacts tops up `seat`'s battlefield so it controls at least
// `count` artifacts. Existing artifacts are kept; only the deficit is
// appended. Used for metalcraft priming.
func seedSeatArtifacts(gs *gameengine.GameState, seat, count int) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == "artifact" {
				have++
				break
			}
		}
	}
	for i := have; i < count; i++ {
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:  fmt.Sprintf("Metalcraft Setup %d", i),
				Owner: seat,
				Types: []string{"artifact"},
			},
			Controller: seat,
			Owner:      seat,
			Flags:      map[string]int{},
			Counters:   map[string]int{},
		})
	}
}

// seedSeatCreatures tops up `seat`'s battlefield to at least `count`
// creatures. Existing creatures of the requested subtype (or any creature
// when subtype is empty) count toward the total. Used by for_each priming.
func seedSeatCreatures(gs *gameengine.GameState, seat, count int, name, subtype string) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	have := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isCreature := false
		hasSub := subtype == ""
		for _, t := range p.Card.Types {
			if t == "creature" {
				isCreature = true
			}
			if subtype != "" && t == subtype {
				hasSub = true
			}
		}
		if isCreature && hasSub {
			have++
		}
	}
	for i := have; i < count; i++ {
		types := []string{"creature"}
		if subtype != "" {
			types = append(types, subtype)
		}
		perm := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:          fmt.Sprintf("%s %d", name, i),
				Owner:         seat,
				Types:         types,
				BasePower:     1,
				BaseToughness: 1,
			},
			Controller: seat,
			Owner:      seat,
			Flags:      map[string]int{},
			Counters:   map[string]int{},
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	}
}

// trimOrFillHand adjusts seat's hand to exactly `n` cards. Existing cards
// are preserved up to n; excess cards are moved to the graveyard so we
// don't introduce zone violations. Used by HandSizeThreshold priming.
func trimOrFillHand(gs *gameengine.GameState, seat, n int) {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s := gs.Seats[seat]
	for len(s.Hand) > n {
		last := len(s.Hand) - 1
		s.Graveyard = append(s.Graveyard, s.Hand[last])
		s.Hand = s.Hand[:last]
	}
	for len(s.Hand) < n {
		s.Hand = append(s.Hand, &gameengine.Card{
			Name:  fmt.Sprintf("Hand Filler %d", len(s.Hand)),
			Owner: seat,
			Types: []string{"instant"},
		})
	}
}

// placePoweredCreature appends a creature with the given P/T to `seat`'s
// battlefield. Used by ferocious / formidable priming where the threshold
// check depends on a specific power level.
func placePoweredCreature(gs *gameengine.GameState, seat int, name string, power, toughness int) *gameengine.Permanent {
	if seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	perm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          name,
			Owner:         seat,
			Types:         []string{"creature"},
			BasePower:     power,
			BaseToughness: toughness,
		},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}
