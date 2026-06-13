package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Chaos Warp — {2}{R} Instant.
//
//	The owner of target permanent shuffles it into their library, then
//	reveals the top card of their library. If it's a permanent card,
//	they put it onto the battlefield. Otherwise, they leave it on top.
//
// A premier red catch-all answer (top-tier EDHREC red staple). The whole
// effect parsed to a `custom` slug (chaos_warp) with no handler, so the
// spell resolved to a no-op — the targeted permanent stayed put. This
// handler implements both halves.
//
// Notes / scope:
//   - Token targets cease when they leave the battlefield (CR §111.7) and
//     are NOT added to the library — they just vanish, which is correct.
//   - Commander redirect (§903.9b — owner may put a commander into the
//     command zone instead of the library) is NOT modeled here; a
//     commander is shuffled into its owner's library like any other
//     permanent. Logged as a known limitation in the coverage report.
func init() {
	registerChaosWarp(Global())
	AddResetHook(registerChaosWarp)
}

func registerChaosWarp(r *Registry) {
	r.OnResolve("Chaos Warp", chaosWarpResolve)
}

func chaosWarpResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "chaos_warp"
	if gs == nil || item == nil {
		return
	}
	var target *gameengine.Permanent
	for _, t := range item.Targets {
		if t.Kind == gameengine.TargetKindPermanent && t.Permanent != nil {
			target = t.Permanent
			break
		}
	}
	if target == nil || target.Card == nil {
		emitFail(gs, slug, "Chaos Warp", "no_target", nil)
		return
	}
	card := target.Card
	owner := target.Owner
	if card.Owner >= 0 && card.Owner < len(gs.Seats) {
		owner = card.Owner
	}
	if owner < 0 || owner >= len(gs.Seats) || gs.Seats[owner] == nil {
		emitFail(gs, slug, "Chaos Warp", "bad_owner", nil)
		return
	}
	isToken := cardHasType(card, "token")
	targetName := card.DisplayName()

	if !removePermanent(gs, target) {
		emitFail(gs, slug, "Chaos Warp", "not_on_battlefield", nil)
		return
	}
	gs.UnregisterReplacementsForPermanent(target)
	gs.UnregisterContinuousEffectsForPermanent(target)
	gameengine.DetachAll(gs, target)
	gameengine.FireZoneChangeTriggers(gs, target, card, "battlefield", "library")

	os := gs.Seats[owner]
	shuffledIn := false
	if !isToken {
		os.Library = append(os.Library, card)
		rngShuffle(gs, len(os.Library), func(i, j int) {
			os.Library[i], os.Library[j] = os.Library[j], os.Library[i]
		})
		shuffledIn = true
	}

	// Reveal the top card; if it's a permanent card, put it onto the
	// battlefield under its owner's control.
	revealedName := ""
	revealedPut := false
	if len(os.Library) > 0 {
		top := os.Library[0]
		if top != nil {
			revealedName = top.DisplayName()
			if cardIsPermanent(top) {
				os.Library = os.Library[1:]
				enterBattlefieldWithETB(gs, owner, top, false)
				revealedPut = true
			}
		}
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "chaos_warp",
		Seat:   item.Controller,
		Target: owner,
		Source: "Chaos Warp",
		Details: map[string]interface{}{
			"shuffled_card": targetName,
			"was_token":     isToken,
			"shuffled_in":   shuffledIn,
			"revealed":      revealedName,
			"revealed_put":  revealedPut,
		},
	})
	emit(gs, slug, "Chaos Warp", map[string]interface{}{
		"shuffled":     targetName,
		"owner":        owner,
		"revealed":     revealedName,
		"revealed_put": revealedPut,
	})
}
