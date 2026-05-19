package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCynetteJellyDrover wires Cynette, Jelly Drover.
//
// Oracle text (Scryfall, verified):
//
//	When Cynette enters or dies, create a 1/1 blue Jellyfish creature
//	token with flying.
//	Creatures you control with flying get +1/+1.
//
// Implementation (R46 stub port):
//   - ETB and dies: mint a 1/1 blue Jellyfish with kw:flying via
//     CreateCreatureToken (the auto-gen stub spawned a generic "Fish"
//     1/1; the port produces a correctly-typed Jellyfish + blue
//     color + flying flag). After each spawn we recompute the
//     flying-creatures buff so the new token's own +1/+1 lands
//     immediately.
//   - Static +1/+1 buff on flying creatures: recomputed at ETB,
//     creature_dies, and nonland_permanent_etb / permanent_ltb (any
//     event that could change which controller's creatures are
//     flying). A "cynette_flying_buff" Duration-tagged Modification
//     is stripped and re-appended per perm on every recompute, sized
//     to a flat +1/+1 if the perm has kw:flying (or printed flying
//     via AST keyword) and is controlled by Cynette's controller.
func registerCynetteJellyDrover(r *Registry) {
	r.OnETB("Cynette, Jelly Drover", cynetteJellyDroverETB)
	r.OnTrigger("Cynette, Jelly Drover", "creature_dies", cynetteOnDies)
	r.OnTrigger("Cynette, Jelly Drover", "nonland_permanent_etb", cynetteRecompute)
	r.OnTrigger("Cynette, Jelly Drover", "permanent_ltb", cynetteRecompute)
}

const cynetteFlyingBuffTag = "cynette_flying_buff"

func cynetteJellyDroverETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cynette_jelly_drover_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	cynetteMintJellyfish(gs, perm)
	cynetteSyncFlyingBuff(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func cynetteOnDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cynette_dies_jellyfish"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying != perm {
		return
	}
	cynetteMintJellyfish(gs, perm)
	cynetteSyncFlyingBuff(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func cynetteRecompute(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	cynetteSyncFlyingBuff(gs, perm)
}

func cynetteMintJellyfish(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Jellyfish",
		[]string{"creature", "jellyfish"},
		1, 1,
	)
	if tok == nil {
		return
	}
	if tok.Flags == nil {
		tok.Flags = map[string]int{}
	}
	tok.Flags["kw:flying"] = 1
	if tok.Card != nil {
		tok.Card.Colors = []string{"U"}
	}
}

// cynetteSyncFlyingBuff replaces every "cynette_flying_buff"-tagged
// Modification on the controller's creatures: if the creature flies
// (kw:flying flag or AST keyword), give +1/+1; otherwise none.
func cynetteSyncFlyingBuff(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		// Strip prior buff.
		mods := p.Modifications[:0]
		for _, m := range p.Modifications {
			if m.Duration == cynetteFlyingBuffTag {
				continue
			}
			mods = append(mods, m)
		}
		p.Modifications = mods
		// Append fresh buff if this perm flies.
		flies := false
		if p.Flags != nil && p.Flags["kw:flying"] == 1 {
			flies = true
		} else if cardHasKeyword(p.Card, "flying") {
			flies = true
		}
		if flies {
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     1,
				Toughness: 1,
				Duration:  cynetteFlyingBuffTag,
				Timestamp: gs.NextTimestamp(),
			})
		}
	}
	gs.InvalidateCharacteristicsCache()
}
