package gameengine

// Per-card dispatch hooks. Populated by the internal/gameengine/per_card
// subpackage's init. When nil, all dispatch is a no-op — this file is
// the clean seam that lets the engine reference the per_card subsystem
// without importing it (which would be cyclic, since per_card imports
// this package).
//
// The engine side-effects of registering a per-card handler:
//
//   - ETBHook fires at the tail of resolvePermanentSpellETB (stack.go),
//     after stock AST ETB triggers have been pushed+resolved. The per-
//     card ETB handler runs as the "snowflake bottom" of the ETB cascade.
//
//   - CastHook fires inside CastSpell (stack.go), AFTER the stack item
//     has been pushed but BEFORE priority opens. Currently unused in
//     batch #1 but kept in the contract for future expansion.
//
//   - ResolveHook fires inside ResolveStackTop (stack.go) just before
//     stock Effect dispatch. If it returns >0 (indicating handlers
//     fired), stock Effect dispatch is SKIPPED — the handler is the
//     authoritative spell effect. Used by Doomsday / Demonic
//     Consultation / Tainted Pact.
//
//   - ActivatedHook fires when an activated ability is resolved. The
//     engine's activated-ability path currently routes through stack.go;
//     callers can invoke the hook directly via InvokeActivatedHook for
//     card-specific activation. Used by Aetherflux / Walking Ballista.
//
//   - TriggerHook fires when the engine emits a named game event
//     ("spell_cast_by_opponent", "noncreature_spell_cast", etc.). Cards
//     whose oracle text is "whenever X happens, do Y" plug in via
//     OnTrigger + the engine calls FireCardTrigger(name, ctx) at the
//     event site. Used by Rhystic Study, Mystic Remora, Aetherflux,
//     Displacer Kitten, Hullbreaker Horror, Cloudstone Curio.
var (
	ETBHook          func(gs *GameState, perm *Permanent)
	CastHook         func(gs *GameState, item *StackItem)
	ResolveHook      func(gs *GameState, item *StackItem) int
	ActivatedHook    func(gs *GameState, src *Permanent, abilityIdx int, ctx map[string]interface{})
	TriggerHook      func(gs *GameState, event string, ctx map[string]interface{})
	HasTriggerHook   func(cardName, event string) bool
)

// FireCardTrigger is the engine-side helper for emitting a custom
// game-event trigger to whatever per-card handlers are registered for
// that event. Safe to call with TriggerHook == nil.
//
// Callers pass the canonical event name (conventions below) plus a ctx
// map of event details. Common events emitted by the engine:
//
//   - "spell_cast"                    — any spell (any caster)
//   - "spell_cast_by_opponent"        — opponent of the listener cast
//                                       (scoping is the HANDLER's job
//                                        using ctx["caster_seat"])
//   - "noncreature_spell_cast"        — spell type != creature
//   - "creature_spell_cast"           — creature spell
//   - "instant_or_sorcery_cast"       — instant or sorcery
//   - "permanent_etb"                 — permanent entered battlefield
//   - "nonland_permanent_etb"         — non-land permanent entered
//   - "opponent_drew_card"            — opponent drew
//   - "life_gained"                   — any seat gained life
//
// ctx keys by convention:
//
//   - "caster_seat": int                       — who cast the spell
//   - "spell_name":  string                    — display name of spell
//   - "card":        *Card                     — the spell's card
//   - "is_creature": bool                      — was the spell creature?
//   - "perm":        *Permanent                — for ETB events
//   - "amount":      int                       — for life/damage events
func FireCardTrigger(gs *GameState, event string, ctx map[string]interface{}) {
	if TriggerHook == nil || gs == nil {
		return
	}
	TriggerHook(gs, event, ctx)
}

// InvokeETBHook calls the ETB hook if installed. Safe to call always.
func InvokeETBHook(gs *GameState, perm *Permanent) {
	if ETBHook == nil {
		return
	}
	ETBHook(gs, perm)
}

// InvokeResolveHook invokes the resolve hook and returns the count fired
// (0 when no per-card handler exists, which tells stack.go to use stock
// Effect dispatch).
func InvokeResolveHook(gs *GameState, item *StackItem) int {
	if ResolveHook == nil {
		return 0
	}
	return ResolveHook(gs, item)
}

// InvokeCastHook calls the cast hook if installed.
func InvokeCastHook(gs *GameState, item *StackItem) {
	if CastHook == nil {
		return
	}
	CastHook(gs, item)
}

// InvokeActivatedHook calls the activated-ability hook if installed.
func InvokeActivatedHook(gs *GameState, src *Permanent, abilityIdx int, ctx map[string]interface{}) {
	if ActivatedHook == nil {
		return
	}
	ActivatedHook(gs, src, abilityIdx, ctx)
}
