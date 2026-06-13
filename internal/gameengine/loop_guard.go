package gameengine

// loop_guard.go — the uniform loop-guard event (r63, LIVENESS follow-up).
//
// Every runaway-loop guard in the engine historically emitted its own
// event vocabulary (game_draw{reason:trigger_loop_cap}, sba_cap_hit,
// trigger_evaluated{capped:...}, per-handler no-progress emitFails, or
// — worst — nothing at all, like DrainStack's iteration cap). Liveness
// auditing (judge/liveness.go cap_contract) and the grinder watchdog
// had to know every spelling. LogLoopGuardFired gives every guard one
// canonical, scannable emission: Kind "loop_guard_fired", Source = the
// guard name, plus guard-specific details. The legacy events stay (
// dashboards and existing invariants consume them); this is additive.
const EventLoopGuardFired = "loop_guard_fired"

// LogLoopGuardFired emits the uniform loop-guard event. Call it at
// every site where a runaway-loop guard trips — depth caps, per-turn
// fire caps, SBA pass caps, drain iteration caps, and per-handler
// no-progress breaks.
func LogLoopGuardFired(gs *GameState, guard string, details map[string]interface{}) {
	if gs == nil {
		return
	}
	d := map[string]interface{}{"guard": guard}
	for k, v := range details {
		d[k] = v
	}
	gs.LogEvent(Event{
		Kind:    EventLoopGuardFired,
		Seat:    -1,
		Target:  -1,
		Source:  guard,
		Details: d,
	})
}
