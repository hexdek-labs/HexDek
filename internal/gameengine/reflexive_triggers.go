package gameengine

// reflexive_triggers.go — Reflexive triggered abilities (CR §603.12).
//
// A reflexive triggered ability is one that triggers based on actions
// taken during the resolution of the spell or ability that created it.
// These are identified by the "when you do" / "when it enters" / "when
// a player does" pattern in oracle text.
//
// Examples:
//   - Valakut Exploration: "Whenever a land enters the battlefield under
//     your control, exile the top card of your library. You may play
//     that card for as long as it remains exiled. If you don't, [...].
//     When you do, Valakut Exploration deals damage..."
//   - Searing Blaze: "... deals 1 damage to target player or
//     planeswalker and 1 damage to target creature that player or that
//     planeswalker's controller controls. Landfall — If you had a land
//     enter the battlefield under your control this turn, Searing Blaze
//     deals 3 damage instead." (Not reflexive, but often confused.)
//   - Pyre of Heroes: "When the creature enters the battlefield this
//     way, [effect]."
//
// CR §603.12: "A reflexive triggered ability is a triggered ability
// that has no trigger condition and is created by an effect. It
// triggers once as a result of performing a particular action during
// the resolution of the effect that created it."
//
// Implementation:
//
// QueueReflexiveTrigger registers an effect function to be executed
// after the current effect finishes resolving. The trigger is put on
// the stack as a triggered ability (so it can be responded to) rather
// than executing inline.
//
// Usage in per-card handlers:
//
//   // During resolution of an effect:
//   QueueReflexiveTrigger(gs, perm, "card_name", func(gs *GameState) {
//       // This fires after the parent effect resolves, on the stack.
//       DealDamageToPlayer(gs, targetSeat, 3, perm.Controller, "card_name")
//   })
