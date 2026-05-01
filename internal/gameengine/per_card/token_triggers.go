package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// ============================================================================
// Token-creation triggered abilities.
//
// These handlers register for the "token_created" event that fires after
// any token creation (resolve.go CreateToken, CreateTokenCopy, and the
// tokens.go helpers). The engine sets gs.Flags["in_token_trigger"]=1
// before firing, so tokens created by these handlers do NOT re-trigger
// (preventing infinite loops).
// ============================================================================

// --- Chatterfang, Squirrel General ---
//
// Oracle text:
//   Forestwalk
//   If one or more tokens would be created under your control, those
//   tokens plus that many 1/1 green Squirrel creature tokens are created
//   instead.
//
// Implementation: fires on "token_created" for the controller's seat.
// Creates N squirrel tokens where N = the count from the triggering event.
// The re-entrancy guard in resolve.go/tokens.go ensures the squirrels
// themselves do not trigger another round of Chatterfang.
func registerChatterfang(r *Registry) {
	r.OnTrigger("Chatterfang, Squirrel General", "token_created", chatterfangTrigger)
}

func chatterfangTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Only triggers on tokens YOU create.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}

	count, _ := ctx["count"].(int)
	if count <= 0 {
		return
	}

	// Avoid creating squirrels from our own squirrel creation (belt-and-
	// suspenders on top of the engine-level in_token_trigger guard).
	if src, ok := ctx["source"].(string); ok && src == "Squirrel" {
		return
	}

	// Create N 1/1 green Squirrel creature tokens.
	for i := 0; i < count; i++ {
		gameengine.CreateCreatureToken(gs, seat, "Squirrel", []string{"creature", "squirrel"}, 1, 1)
	}

	emit(gs, "chatterfang_trigger", "Chatterfang, Squirrel General", map[string]interface{}{
		"seat":     seat,
		"squirrels": count,
	})
}

// --- Pitiless Plunderer ---
//
// Oracle text:
//   Whenever another creature you control dies, create a Treasure token.
//
// Death trigger, not token-created. Registers on "creature_dies".
func registerPitilessPlunderer(r *Registry) {
	r.OnTrigger("Pitiless Plunderer", "creature_dies", pitilessPlundererTrigger)
}

func pitilessPlundererTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Only triggers on creatures YOU control dying.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}

	// "Another creature" -- the dying creature must not be Pitiless Plunderer itself.
	if dyingPerm, ok := ctx["perm"].(*gameengine.Permanent); ok && dyingPerm == perm {
		return
	}

	gameengine.CreateTreasureToken(gs, seat)

	emit(gs, "pitiless_plunderer", "Pitiless Plunderer", map[string]interface{}{
		"seat":  seat,
		"token": "Treasure",
	})
}

// Anointed Procession's token_created trigger is registered in
// batch17_sweep.go (upgrading the existing ETB-flag stub).
