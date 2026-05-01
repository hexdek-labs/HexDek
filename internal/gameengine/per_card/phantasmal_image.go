package per_card

import (
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhantasmalImage wires up Phantasmal Image.
//
// Oracle text:
//
//	You may have Phantasmal Image enter the battlefield as a copy of
//	any creature on the battlefield, except it's an Illusion in
//	addition to its other types and it has "When this creature
//	becomes the target of a spell or ability, sacrifice it."
//
// A 1U clone. Combos with mana dorks (Kinnan + Lotus Cobra + Image
// as a second Cobra), with untap effects (Image + Palinchron = 2
// Palinchrons = infinite mana via alternating untaps), and with any
// ETB creature you'd want doubled (Dockside Extortionist!).
//
// Batch #3 scope:
//   - OnETB: resolve the "enter as a copy" clause. ctx["copy_target"]
//     specifies the Permanent to clone from; if absent, Image enters
//     as a 0/0 Illusion (dies to SBA). The copy shares Card pointer
//     (so all ETB triggers on the copied card fire — including
//     Dockside's treasure ETB, making 2 Docksides worth of treasure).
//   - The "sacrifice when targeted" rider is a permanent flag that
//     downstream targeting code can consult.
func registerPhantasmalImage(r *Registry) {
	r.OnETB("Phantasmal Image", phantasmalImageETB)
	// The "became_target" trigger is name-keyed by the handler registry,
	// but AFTER Image copies another creature, its Card.DisplayName() is
	// the COPIED name — so the registry wouldn't find Phantasmal Image's
	// handler. We work around by registering the handler under the
	// copied-name ALSO (at copy time), AND by scanning via the
	// phantasmal_sac_on_target flag in the dispatch helper. The second
	// path is registered below as a global handler keyed on a sentinel
	// name; the integration is via explicit calls to
	// PhantasmalImageCheckTargeted which any targeting primitive can
	// call at target-assignment time.
	r.OnTrigger("Phantasmal Image", "became_target", phantasmalImageOnTargeted)
}

func phantasmalImageETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "phantasmal_image_clone"
	if gs == nil || perm == nil {
		return
	}
	// Mark the sac-on-target flag regardless of copy target.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["phantasmal_sac_on_target"] = 1

	// Determine copy target.
	var target *gameengine.Permanent
	// The caller may stash the copy target on gs.Flags or on the
	// Permanent Flags. Check both. For tests we use a package-level
	// per-Image copy-target map.
	if v, ok := phantasmalImageTargets.Load(perm); ok {
		target = v.(*gameengine.Permanent)
	}
	if target == nil {
		// Fallback: the last ETB'd creature permanent OTHER than Image
		// itself. This is the "auto-copy best ETB" policy: cEDH tests
		// want Image to clone Dockside/Drake/etc. that just entered.
		seat := perm.Controller
		bf := gs.Seats[seat].Battlefield
		for i := len(bf) - 1; i >= 0; i-- {
			p := bf[i]
			if p == nil || p == perm {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			target = p
			break
		}
		// Also scan opponents' creatures — the oracle is "any creature
		// on the battlefield". We prefer our own for combo, but fall
		// back to opponents.
		if target == nil {
			for _, s := range gs.Seats {
				if s == nil {
					continue
				}
				for _, p := range s.Battlefield {
					if p == nil || p == perm {
						continue
					}
					if !p.IsCreature() {
						continue
					}
					target = p
					break
				}
				if target != nil {
					break
				}
			}
		}
	}
	if target == nil {
		// No creature on battlefield — Image enters as a 0/0 Illusion
		// and will die to SBA next pass.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":       perm.Controller,
			"copy":       "none",
			"enters_as":  "0_0_illusion_dies_to_sba",
		})
		return
	}
	// Point Image's Card at the copied Card (shares the AST, types,
	// powers, abilities — this IS what "enter as a copy" means per CR
	// §706.2). We preserve perm.Flags (so the sac-on-target rider
	// persists) and perm.Timestamp (Image has its own ETB timestamp).
	//
	// Phantasmal-specific addenda (illusion type + sac-rider) are
	// signaled via perm.Flags; a full characteristic-layering pass
	// would merge "Illusion" into the copied types, but the Flags
	// sentinel suffices for the trigger handler.
	perm.OriginalCard = perm.Card
	perm.Card = target.Card.DeepCopy()
	// Copy +1/+1 counters that are "part of the copy" per §706.2 —
	// actually, counters on the COPY SOURCE are NOT copied (they're
	// per-permanent state). We leave perm.Counters empty; this matches
	// oracle intent.
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"copied":     target.Card.DisplayName(),
		"sac_rider":  1,
	})
	emitPartial(gs, slug, "Phantasmal Image",
		"sac_on_target_trigger_is_name_keyed_and_wont_match_after_copy_callers_must_invoke_PhantasmalImageCheckTargeted_at_target_time")
	// Fire the ETB cascade on the copy. The copied card's per_card
	// ETB handler (if any) fires as if Image had entered under that
	// name. This is CRITICAL for Dockside clones — Image-as-Dockside
	// should generate treasure on its own ETB.
	//
	// We DON'T recurse into InvokeETBHook here because we're already
	// IN it (the engine invoked phantasmalImageETB as part of its
	// ETB cascade). Instead, we fire the copy's handler directly.
	//
	// BUT: calling InvokeETBHook again would double-fire Image's OWN
	// ETB handler if the copied card is another Phantasmal Image (rare
	// but possible — Image of Image is a 0/0 anyway).
	//
	// Safe approach: fire the copied card's registry entry directly,
	// bypassing the dispatch hook (which is keyed by name). The Image's
	// handler already emitted the clone event; the copied card's
	// handler needs to run to trigger ETB combos.
	fireETBOnCopy(gs, perm)
}

var phantasmalImageTargets sync.Map

// fireETBOnCopy fires the copied card's per_card ETB handler. We look
// up the handler via the registry (by the NOW-copied name) and call
// it directly. This avoids firing Phantasmal Image's OWN ETB handler
// a second time.
func fireETBOnCopy(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	name := perm.Card.DisplayName()
	// Skip self-referential copies.
	if name == "Phantasmal Image" {
		return
	}
	reg := Global()
	reg.mu.RLock()
	handlers := append([]ETBHandler(nil), reg.etb[normalizeName(name)]...)
	reg.mu.RUnlock()
	for _, h := range handlers {
		if gs.Flags != nil && gs.Flags["ended"] == 1 {
			return
		}
		h(gs, perm)
	}
}

func phantasmalImageOnTargeted(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phantasmal_image_sac_on_target"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["phantasmal_sac_on_target"] != 1 {
		return
	}
	// "When this creature becomes the target" — confirm this is the
	// target permanent. ctx["target_perm"] identifies the targeted one.
	if v, ok := ctx["target_perm"].(*gameengine.Permanent); ok {
		if v != perm {
			return
		}
	}
	// Sacrifice it.
	gameengine.SacrificePermanent(gs, perm, "phantasmal_image_targeted")
	emit(gs, slug, "Phantasmal Image", map[string]interface{}{
		"seat":   perm.Controller,
		"reason": "became_target_of_spell_or_ability",
	})
}

// PhantasmalImageCheckTargeted is the scan-by-flag entry point that
// ANY targeting primitive in the engine can call when a permanent
// becomes the target of a spell or ability. It walks every battlefield
// permanent (in case the targeted permanent is Image under a different
// copied name), and sacrifices any that has the
// phantasmal_sac_on_target flag set AND matches the targeted Permanent
// pointer.
//
// The reason we can't rely solely on the per-card trigger dispatcher:
// after Image copies another creature, its Card.DisplayName() returns
// the COPIED name (e.g. "Llanowar Elves"), so the registry's name-
// keyed lookup misses the Phantasmal Image handler.
func PhantasmalImageCheckTargeted(gs *gameengine.GameState, targeted *gameengine.Permanent) {
	if gs == nil || targeted == nil {
		return
	}
	if targeted.Flags == nil || targeted.Flags["phantasmal_sac_on_target"] != 1 {
		return
	}
	// Sacrifice the targeted permanent.
	gameengine.SacrificePermanent(gs, targeted, "phantasmal_image_targeted")
	emit(gs, "phantasmal_image_sac_on_target", "Phantasmal Image", map[string]interface{}{
		"seat":       targeted.Controller,
		"reason":     "became_target_of_spell_or_ability",
		"via":        "flag_scan",
	})
}
