package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// ---------------------------------------------------------------------
// r90 — "enters as a copy" is a replacement effect and applies ONCE.
//
// THE PROD CRASH THIS FIXES. On 2026-10-14 01:03 the .206 server died
// with `fatal error: stack overflow` (goroutine stack exceeds 1GB) and
// the wrapper relaunched it, resetting the grinder from 40018 games to
// zero. The crashing goroutine was a clean three-frame cycle repeated
// until the stack ran out, every frame on the SAME *Permanent:
//
//	batch_al_clones_r60.go:80   cloneVariantETB   -> fireETBOnCopy
//	phantasmal_image.go:203     fireETBOnCopy     -> copyEnchantmentETB
//	batch_al_clones_r60.go:160  copyEnchantmentETB-> cloneVariantETB
//
// The shape: a Copy Enchantment that is still NAMED "Copy Enchantment"
// (it entered with no legal target, so it stayed itself) is a legal
// enchantment target for the next Copy Enchantment. The second one
// enters as a copy of it, which leaves the copy ALSO named "Copy
// Enchantment"; fireETBOnCopy then dispatches the ETB handler
// registered under that name — which is the clone handler again, on a
// permanent that has already entered. scanCloneTargets excludes only
// `self`, so the first Copy Enchantment is still sitting there as a
// legal target, and the cycle has no exit: copying it never changes
// the name that selects the handler.
//
// WHY NEITHER EXISTING GUARD CAUGHT IT. Three different ad-hoc
// defences against this one hazard already existed, and the two cards
// that actually recursed had none of them:
//
//   - fireETBOnCopy guarded a single literal, `name == "Phantasmal
//     Image"` — the one card the file is named after.
//   - sakashimaCopyETB skips same-NAMED copy targets.
//   - Clone, Phyrexian Metamorph and Copy Enchantment: nothing.
//
// The hazard was even written down. The comment above the
// fireETBOnCopy call in phantasmal_image.go reasons explicitly about
// "if the copied card is another Phantasmal Image" and dismisses it as
// "rare but possible — Image of Image is a 0/0 anyway". That is true
// of Phantasmal Image and false of Copy Enchantment, and the guard was
// written for the card in front of the author rather than for the
// shared chokepoint all five clone paths route through.
//
// THE FIX IS THE RULE, NOT A DEPTH CAP. CR 614.1c: effects reading
// "[This permanent] enters as ..." are replacement effects. CR 614.5:
// "A replacement effect doesn't invoke itself repeatedly; it gets only
// one opportunity to affect an event or any modified events that may
// replace that event." A permanent that has already entered as a copy
// has spent that one opportunity. Re-running the clone handler on it
// is not a loop to be bounded — it is a replacement effect applying
// twice, which the rules forbid outright. Terminating is a consequence
// of getting that right, exactly as in r88's hand-discard fix.
//
// So the guard is a once-per-permanent claim, checked by every
// as-enters-copy handler. It is deliberately NOT a check on names or
// on recursion depth: a cross-name cycle (Phyrexian Metamorph enters
// as a copy of Clone, whose handler then re-clones it) is the same
// violation and a name guard would miss it.
//
// IT ALSO FIXES A FINITE CASE. A Clone entering as a copy of The
// Mimeoplasm previously ran Mimeoplasm's own "enters as a copy of a
// creature card in a graveyard" replacement afterwards — a second
// as-enters copy on a permanent that had already entered. That never
// hung, because the Mimeoplasm handler does not call fireETBOnCopy, so
// it was invisible. Same rule, same marker, no crash to find it by.
//
// The marker lives on the Permanent, not the Card: a permanent that
// leaves and re-enters the battlefield is a new object with fresh
// Flags, and correctly gets a fresh opportunity.
// ---------------------------------------------------------------------

// cloneAsEntersMarker records that an "enters as a copy" replacement
// effect has already been applied to this permanent (CR 614.1c).
const cloneAsEntersMarker = "clone_as_enters_applied"

// claimCloneAsEnters returns true the first time it is called for a
// given permanent and false on every later call, implementing the
// CR 614.5 "only one opportunity" bound for as-enters copy effects.
//
// Callers return early on false. The claim is taken even when the
// handler goes on to find no legal copy target: the replacement effect
// got its opportunity and the permanent entered as itself, which is a
// result, not a deferral.
func claimCloneAsEnters(perm *gameengine.Permanent) bool {
	if perm == nil {
		return false
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags[cloneAsEntersMarker] == 1 {
		return false
	}
	perm.Flags[cloneAsEntersMarker] = 1
	return true
}
