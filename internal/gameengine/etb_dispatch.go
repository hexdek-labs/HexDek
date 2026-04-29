package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// FirePermanentETBTriggers fires the complete ETB trigger cascade for a
// permanent that has already been added to the battlefield. Handles:
//
//  1. Self-AST ETB triggers (the entering permanent's own ETB abilities)
//  2. Per-card self-ETB hook (snowflake handlers)
//  3. Ascend check (city's blessing at 10+ permanents)
//  4. Per-card hook events (permanent_etb, nonland_permanent_etb)
//  5. AST observer ETB triggers (other permanents watching for ETBs)
//
// The permanent must already be on the battlefield and replacement effects
// registered before calling this function.
func FirePermanentETBTriggers(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}

	// CR §708.4: face-down permanents have no abilities — skip self-triggers.
	faceDown := perm.Flags != nil && perm.Flags["face_down"] != 0

	if !faceDown && perm.Card.AST != nil {
		for _, ab := range perm.Card.AST.Abilities {
			trig, ok := ab.(*gameast.Triggered)
			if !ok || trig.Effect == nil {
				continue
			}
			if !EventEquals(trig.Trigger.Event, "etb") {
				continue
			}
			PushTriggeredAbility(gs, perm, trig.Effect)
			if gs.CheckEnd() {
				return
			}
		}
	}

	if !faceDown {
		InvokeETBHook(gs, perm)
	}

	CheckAscend(gs, perm.Controller)

	if !perm.IsLand() {
		FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
			"perm":            perm,
			"controller_seat": perm.Controller,
			"card":            perm.Card,
		})
	}
	FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            perm,
		"controller_seat": perm.Controller,
		"card":            perm.Card,
	})

	fireObserverETBTriggers(gs, perm)
}
