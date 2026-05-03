package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Light-Paws, Emperor's Voice
//
// Oracle text:
//   Whenever an Aura is put onto the battlefield under your control, if
//   you didn't put it onto the battlefield with this ability, you may
//   search your library for an Aura card with mana value less than or
//   equal to that Aura and with a different name, put it onto the
//   battlefield attached to Light-Paws, Emperor's Voice, then shuffle.
//
// Implementation:
//   - OnTrigger("permanent_etb"): fires whenever any permanent enters the
//     battlefield. Gate: entering permanent is an Aura (cardHasType "aura"),
//     controller matches Light-Paws' controller, and the
//     "light_paws_searching" flag is NOT set on Light-Paws (prevents the
//     Aura fetched by this ability from re-triggering it).
//   - Search library for the highest-CMC Aura with CMC <= entering Aura's
//     CMC and a different name than the entering Aura.
//   - Put found Aura onto battlefield attached to Light-Paws, shuffle.

func registerLightPawsEmperorsVoiceETB(r *Registry) {
	r.OnTrigger("Light-Paws, Emperor's Voice", "permanent_etb", lightPawsAuraETBTrigger)
}

func lightPawsAuraETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "light_paws_emperors_voice_aura_etb"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}

	// perm is the Light-Paws permanent observing the trigger.
	if perm.Card.DisplayName() != "Light-Paws, Emperor's Voice" {
		return
	}

	// Extract the entering permanent from the trigger context.
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}

	// Gate: entering permanent must be an Aura.
	if !cardHasType(entering.Card, "aura") {
		return
	}

	// Gate: entering Aura must be under Light-Paws' controller.
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}

	// Guard against re-trigger: if Light-Paws is currently searching,
	// the entering Aura was put onto the battlefield by this ability.
	if perm.Flags != nil && perm.Flags["light_paws_searching"] != 0 {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	enteringCMC := entering.Card.CMC
	enteringName := entering.Card.DisplayName()

	// Search library for the best Aura: highest CMC that is still
	// <= the entering Aura's CMC, with a different name.
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "aura") {
			continue
		}
		if c.CMC > enteringCMC {
			continue
		}
		if c.DisplayName() == enteringName {
			continue
		}
		if c.CMC > bestCMC {
			bestCMC = c.CMC
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_aura", map[string]interface{}{
			"entering_aura": enteringName,
			"max_cmc":       enteringCMC,
		})
		return
	}

	// Set the searching flag to prevent the fetched Aura's ETB from
	// re-triggering this ability.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["light_paws_searching"] = 1

	card := s.Library[bestIdx]
	gameengine.MoveCard(gs, card, seat, "library", "battlefield", "light_paws_emperors_voice")
	fetched := enterBattlefieldWithETB(gs, seat, card, false)

	// Attach the fetched Aura to Light-Paws.
	if fetched != nil {
		fetched.AttachedTo = perm
	}

	shuffleLibraryPerCard(gs, seat)

	// Clear the searching flag.
	delete(perm.Flags, "light_paws_searching")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"entering_aura": enteringName,
		"entering_cmc":  enteringCMC,
		"found":         card.DisplayName(),
		"found_cmc":     card.CMC,
	})
}
