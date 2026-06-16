package gameengine

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameast"
)

// class_level_triggers.go — "When this Class becomes level N" triggered-ability
// dispatch (CR §716.2c / §716.4) on the production level-up path.
//
// Background: a Class enchantment levels up via its "{cost}: Level N" activated
// ability, whose AST effect is a `level_marker` Modification. The level_marker
// resolve handler (resolve_helpers.go) records the new level but — before this
// file — fired NO "became level N" event. The corpus models those payoffs two
// ways, BOTH of which were silent in real games:
//
//   - per_card consumers (Wizard/Warlock/Artificer/Monk/Stormchaser's/Builder's
//     in per_card/class_level_up_consumers_r60.go) registered on the
//     `class_level_up` trigger — but the only emitters of that trigger are the
//     dead LevelUpClass / AdvanceClassLevel helpers (zero production callers),
//     so the consumers never fired.
//   - generic AST `Triggered{event: "class_becomes_level", phase: "<N>"}` nodes
//     (Wizard draw-2, Cleric reanimate, Druid, …) had no dispatcher at all —
//     the same alias-to-nowhere shape as the gain_life / becomes_tapped
//     families fixed in event_ast_dispatch.go.
//
// FireClassLevelTriggers, called from the level_marker handler when a Class's
// level increases, fires BOTH paths once per level gained: the per_card
// consumers (via FireCardTrigger) and the generic AST nodes (skipping
// per_card-owned cards via HasTriggerHook so nothing double-fires), mirroring
// the canonical dispatch discipline in event_ast_dispatch.go.

// FireClassLevelTriggers dispatches the "becomes level N" triggers for a Class
// permanent whose level just rose from prevLevel to newLevel. Per CR §716.2c a
// class fires the becomes-level trigger for EACH level it reaches, so a jump
// (rare — activations are sequential) fires every intervening level once.
func FireClassLevelTriggers(gs *GameState, perm *Permanent, prevLevel, newLevel int) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if newLevel <= prevLevel {
		return
	}
	for lvl := prevLevel + 1; lvl <= newLevel; lvl++ {
		// per_card consumers (registered on "class_level_up"). The dispatcher
		// walks the battlefield by name; each handler self-gates on
		// ctx["source"]==perm and ctx["new_level"].
		FireCardTrigger(gs, "class_level_up", map[string]interface{}{
			"source":          perm,
			"controller_seat": perm.Controller,
			"previous_level":  lvl - 1,
			"new_level":       lvl,
		})
		// Generic AST class_becomes_level nodes.
		fireClassBecomesLevelAST(gs, perm, lvl)
	}
}

// fireClassBecomesLevelAST pushes the leveling-up Class's own
// `Triggered{event: class_becomes_level}` nodes whose printed level matches
// `level`. per_card-owned Classes are skipped (their curated handler already
// fired via FireCardTrigger above) so the payoff never double-fires.
func fireClassBecomesLevelAST(gs *GameState, perm *Permanent, level int) {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	if HasTriggerHook != nil && HasTriggerHook(perm.Card.DisplayName(), "class_level_up") {
		return
	}
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	for _, ab := range perm.Card.AST.Abilities {
		trig, ok := ab.(*gameast.Triggered)
		if !ok || trig.Effect == nil {
			continue
		}
		if !EventEquals(trig.Trigger.Event, "class_becomes_level") {
			continue
		}
		// The parser stuffs the printed level number into Trigger.Phase
		// ("2" / "3"). A node with no level matches every level (fail-open
		// would over-fire, so an unparseable phase fails CLOSED instead).
		band, err := strconv.Atoi(trig.Trigger.Phase)
		if err != nil || band != level {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "trigger_fires",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"event": "class_becomes_level",
				"level": level,
				"rule":  "716.2c",
			},
		})
		PushTriggeredAbilityWithIf(gs, perm, trig.Effect, trig.InterveningIf)
	}
}
