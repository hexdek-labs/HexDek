package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// the_meathook_massacre.go — per_card handler for The Meathook Massacre's
// entry board wipe.
//
// Oracle text (Legendary Enchantment, {X}{B}{B}):
//
//	When The Meathook Massacre enters, each creature gets -X/-X until end of
//	turn.
//	Whenever a creature you control dies, each opponent loses 1 life.
//	Whenever a creature an opponent controls dies, you gain 1 life.
//
// Why a per_card handler: the ETB clause parsed to an inert
// `parsed_effect_residual` (X-scaled -X/-X to all creatures), so Meathook —
// a premium scalable board wipe (58 decks) — wiped NOTHING. The two death
// drains DO work (they parse to structured LoseLife/GainLife triggers); only
// the entry sweep was inert, so this handler implements just that.
//
// Implementation (OnETB, mirrors Toxic Deluge): X = the X paid to cast,
// read from perm.Flags["x_paid"] (the standard X-on-permanent-cast plumb).
// Apply a -X/-X until-end-of-turn Modification to every creature on every
// battlefield; SBAs then kill creatures dropped to 0 toughness (this also
// bypasses indestructible, per the card). Under-wipes safely: if X is 0 /
// unavailable, nothing is modified.
func init() {
	registerTheMeathookMassacre(Global())
	AddResetHook(registerTheMeathookMassacre)
}

func registerTheMeathookMassacre(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("The Meathook Massacre", theMeathookMassacreETB)
}

func theMeathookMassacreETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_meathook_massacre"
	if gs == nil || perm == nil {
		return
	}
	x := 0
	if perm.Flags != nil {
		x = perm.Flags["x_paid"]
	}
	if x <= 0 {
		emit(gs, slug, "The Meathook Massacre", map[string]interface{}{
			"x": x, "modified": 0, "note": "x_zero_or_unavailable",
		})
		return
	}

	ts := gs.NextTimestamp()
	modified := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     -x,
				Toughness: -x,
				Duration:  "until_end_of_turn",
				Timestamp: ts,
			})
			modified++
		}
	}
	gs.InvalidateCharacteristicsCache()
	_ = gs.CheckEnd()

	emit(gs, slug, "The Meathook Massacre", map[string]interface{}{
		"seat": perm.Controller, "x": x, "modified": modified,
	})
}
