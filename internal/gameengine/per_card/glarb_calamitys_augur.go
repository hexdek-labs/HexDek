package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGlarbCalamitysAugur wires Glarb, Calamity's Augur.
//
// Oracle text:
//
//	Deathtouch
//	You may look at the top card of your library any time.
//	You may play lands and cast spells with mana value 4 or greater
//	from the top of your library.
//	{T}: Surveil 2.
//
// Implementation:
//   - Deathtouch is an AST keyword; the engine handles it.
//   - "Look at the top of your library" is a no-op in the current
//     hidden-info model.
//   - Top-of-library cast permission for MV >= 4 cards: register a
//     ZoneCastPermission on the current top card whenever its CMC
//     qualifies. Re-register on draw / library change is best-effort —
//     we re-arm at upkeep alongside the surveil activation.
//   - {T}: Surveil 2 — auto-activated on the controller's upkeep when
//     Glarb is untapped and not summoning sick. Mirrors the Hashaton /
//     The One Ring auto-activation pattern: the AI doesn't have a
//     general "spend untapped activated abilities at upkeep" planner
//     yet, so per-card handlers self-fire.
func registerGlarbCalamitysAugur(r *Registry) {
	r.OnETB("Glarb, Calamity's Augur", glarbETB)
	r.OnTrigger("Glarb, Calamity's Augur", "upkeep_controller", glarbUpkeep)
}

func glarbETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "glarb_etb"
	if gs == nil || perm == nil {
		return
	}
	glarbArmTopOfLibrary(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"top_cast":  "library_cast_mv_4_or_greater",
	})
}

func glarbUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glarb_upkeep_surveil"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Re-arm the top-of-library cast permission each upkeep so the AI
	// always sees the current top card as eligible when MV >= 4.
	glarbArmTopOfLibrary(gs, perm)

	// {T}: Surveil 2. Tap Glarb herself; skip if already tapped, summoning
	// sick (CR §302.1 — though §605.1 mana abilities aren't restricted by
	// SS, surveil isn't a mana ability and Glarb the creature is the
	// activator), or no library to surveil.
	if perm.Tapped || perm.SummoningSick {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost || len(seat.Library) == 0 {
		return
	}
	perm.Tapped = true
	gameengine.Surveil(gs, perm.Controller, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"surveil":  2,
		"tapped":   true,
	})
	// Top card may have changed via surveil; re-arm cast permission.
	glarbArmTopOfLibrary(gs, perm)
}

// glarbArmTopOfLibrary registers a zone-cast permission on the current
// top of Glarb's controller's library if the card has MV >= 4. The card
// is cast paying its normal mana cost (not life — Glarb's permission is
// not Bolas's Citadel).
func glarbArmTopOfLibrary(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Library) == 0 {
		return
	}
	top := s.Library[0]
	if top == nil {
		return
	}
	if cardCMC(top) < 4 {
		return
	}
	cp := gameengine.NewLibraryCastPermission(0) // pay normal mana cost
	cp.RequireController = seat
	cp.SourceName = "Glarb, Calamity's Augur"
	gameengine.RegisterZoneCastGrant(gs, top, cp)
}
