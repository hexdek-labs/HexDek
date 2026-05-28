package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarmicGuide wires Karmic Guide.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Karmic%20Guide):
//
//	Flying
//	Protection from black
//	Echo {3}{W}{W}  (At the beginning of your upkeep, if this came under
//	your control since the beginning of your last upkeep, sacrifice it
//	unless you pay its echo cost.)
//	When this creature enters, you may return target creature card from
//	your graveyard to the battlefield.
//
// {3}{W}{W} Creature — Angel Spirit 2/2. Half of the iconic
// Karmic Guide + Reveillark loop (Guide returns Reveillark; Reveillark
// returns Guide + a 1-power creature on LTB). The ETB is the load-
// bearing part — Guide's body lets it BE returned by Reveillark (≤2
// power on LTB exile, but Reveillark's "from anywhere" Guide-return
// is the canonical engine restart).
//
// Implementation:
//   - Flying + Protection from black are AST-engine-side (parser
//     emits Static abilities for keywords). Not re-implemented here.
//   - Echo upkeep cost is the AST EchoKeyword path — also engine-side.
//   - ETB picker: return the highest-EV creature card from your own
//     graveyard to the battlefield. Priority:
//       1. Reveillark (the canonical loop partner) — flat top tier
//       2. ETB-payoff creatures (HasValueETB profile would tell us,
//          but per_card doesn't see Freya — fall back to CMC)
//       3. highest-CMC creature (best raw mana value retrieval)
//     Picker reuses the same shape as Eternal Witness's grave-to-
//     hand picker but constrained to creature cards and routing to
//     battlefield.
//   - "may" is non-negotiable yes; no legal target no-ops cleanly.
//   - Routes through MoveCard(graveyard→battlefield) — note this
//     differs from the canonical reanimate path (which would build
//     a Permanent and fire ETB triggers). Use MoveCard +
//     wrapCardAsPermanent so the entering creature exists as a
//     proper Permanent and fires its own ETB observers (Karmic
//     Guide → Reveillark → Karmic Guide chains depend on the
//     returned Reveillark's ETB firing).
func registerKarmicGuide(r *Registry) {
	r.OnETB("Karmic Guide", karmicGuideETB)
}

func karmicGuideETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "karmic_guide_etb_return"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	best := pickKarmicGuideTarget(seat.Graveyard)
	if best == nil {
		emitFail(gs, slug, "Karmic Guide", "no_legal_target", map[string]interface{}{
			"graveyard_size": len(seat.Graveyard),
		})
		return
	}
	bestName := best.DisplayName()
	// Move graveyard → battlefield. enterBattlefieldFromGraveyard builds
	// the Permanent wrapper and fires ETB triggers; falls back to a
	// MoveCard nudge when the helper isn't wired in the current build.
	reanimated := enterFromGraveyardOrFallback(gs, best, perm.Controller, slug)
	emit(gs, slug, "Karmic Guide", map[string]interface{}{
		"seat":        perm.Controller,
		"reanimated":  bestName,
		"materialized": reanimated,
	})
}

// pickKarmicGuideTarget scans a graveyard for the highest-EV creature
// card. Reveillark is the canonical pick (the loop partner); failing
// that, highest CMC wins. Returns nil for an empty / non-creature
// graveyard.
func pickKarmicGuideTarget(graveyard []*gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestScore := -1
	for _, c := range graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		score := cardCMC(c)
		if c.Name == "Reveillark" {
			score += 1000
		}
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

// enterFromGraveyardOrFallback materializes a graveyard card as a fresh
// permanent on the named seat's battlefield. Preferred path is the
// canonical reanimation helper if present; otherwise drops a synthetic
// Permanent and fires ETB triggers manually so observers
// (Reveillark, Athreos, dies/etb chains) still see the event.
func enterFromGraveyardOrFallback(gs *gameengine.GameState, c *gameengine.Card, seatIdx int, slug string) bool {
	if gs == nil || c == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	// Remove from graveyard.
	for i, g := range seat.Graveyard {
		if g == c {
			seat.Graveyard = append(seat.Graveyard[:i], seat.Graveyard[i+1:]...)
			break
		}
	}
	perm := &gameengine.Permanent{
		Card:          c,
		Controller:    seatIdx,
		Owner:         c.Owner,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	gameengine.RegisterReplacementsForPermanent(gs, perm)
	gameengine.FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(gameengine.Event{
		Kind:   "reanimate",
		Seat:   seatIdx,
		Source: slug,
		Details: map[string]interface{}{
			"card": c.DisplayName(),
			"from": "graveyard",
		},
	})
	return true
}
