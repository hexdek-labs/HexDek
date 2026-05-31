package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Yeva, Nature's Herald — {2}{G}{G} 4/4 Legendary Creature — Elf Shaman.
//
//   Flash (You may cast this spell any time you could cast an instant.)
//   You may cast green creature spells as though they had flash.
//
// Static permission effect. Stamps gs.Flags["yeva_green_flash_seat_N"]
// + perm.Flags for downstream consumption by the timing-permission layer
// (latent-flag pattern mirrors necropotence.go / damia_sage_of_stone.go).
// The cast-priority pipeline can consult the flag to grant flash on
// green creature spells while Yeva is on the battlefield. ETB sets,
// LTB clears.
//
// Self-Flash is keyword-pipeline; not touched here.

func init() {
	registerYevaNaturesHerald(Global())
	AddResetHook(registerYevaNaturesHerald)
}

func registerYevaNaturesHerald(r *Registry) {
	r.OnETB("Yeva, Nature's Herald", yevaETB)
	r.OnTrigger("Yeva, Nature's Herald", "permanent_ltb", yevaLTB)
}

func yevaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yeva_green_creatures_flash"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["yeva_green_flash_seat_"+intToStr(perm.Controller)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"flash_permission":  "green_creatures",
	})
}

func yevaLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yeva_ltb_clear_flash_flag"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags != nil {
		delete(gs.Flags, "yeva_green_flash_seat_"+intToStr(perm.Controller))
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
