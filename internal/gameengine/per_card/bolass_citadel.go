package per_card

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBolassCitadel wires up Bolas's Citadel.
//
// Oracle text:
//
//	You may look at the top card of your library any time.
//	You may play lands and cast spells from the top of your library.
//	If you cast a spell this way, pay life equal to its mana value
//	rather than pay its mana cost.
//	{T}, Sacrifice ten nontoken permanents: Each opponent loses 10
//	life.
//
// THE Aetherflux Reservoir combo: Citadel lets you cast from the top
// paying life instead of mana, Aetherflux lifegains on every cast, so
// a sufficiently low-curve top-of-library produces a loop that gains
// 1-per-cast life and eventually activates Aetherflux for 50 damage.
//
// Implementation:
//   - OnETB: register a ZoneCastPermission for the controller's top-
//     of-library cards (library_cast keyword with life cost). This
//     integrates with the engine's zone_cast.go CastFromZone primitive
//     so the Hat/AI can actually cast spells from the top of library
//     paying life instead of mana. Also set gs.Flags for the cast-
//     from-top presence check.
//   - OnActivated(0, ...): the "play top of library paying life" mode.
//     Moves the top card to hand and pays life = CMC as an MVP fallback
//     for when the zone-cast path isn't exercised directly. In normal
//     engine operation the CastFromZone path is preferred.
//   - OnActivated(1, ...): the sac-10-nontoken mode. Each opponent
//     loses 10 life. Sacrifice cost is assumed paid by the caller.
//
// The "you may look at the top card" clause is a no-op in the current
// observation model (no hidden information yet).
func registerBolassCitadel(r *Registry) {
	r.OnETB("Bolas's Citadel", bolassCitadelETB)
	r.OnActivated("Bolas's Citadel", bolassCitadelActivate)
}

func bolassCitadelETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bolass_citadel_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["bolas_citadel_active_seat_"+intToStr(seat)] = perm.Timestamp

	// Register zone-cast permission for the top of library. The engine's
	// zone_cast.go CastFromZone path will consult this when the Hat/AI
	// decides to cast. The LifeCostInsteadOfMana field tells the engine
	// to pay life = CMC instead of mana.
	//
	// We grant the permission to the current top card of the library. As
	// the Hat exercises CastFromZone, subsequent top cards become eligible
	// via the citadel_active flag check in the zone-cast scanner.
	if seat >= 0 && seat < len(gs.Seats) {
		s := gs.Seats[seat]
		if len(s.Library) > 0 {
			topCard := s.Library[0]
			cmc := cardCMC(topCard)
			perm := gameengine.NewLibraryCastPermission(cmc)
			perm.RequireController = seat
			perm.SourceName = "Bolas's Citadel"
			gameengine.RegisterZoneCastGrant(gs, topCard, perm)
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"zone_cast":     "library_cast_with_life_cost",
		"integrated":    true,
	})
}

func bolassCitadelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil || src.Card.AST == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Bolas's Citadel has ONE activated ability:
	//   "{T}, Sacrifice ten nonland permanents: Each opponent loses 10 life."
	// The engine passes the RAW AST.Abilities index — the drain sits at
	// index 3 (after three static abilities). Identify it by the AST node
	// at abilityIdx rather than a hardcoded number: the earlier 0/1
	// hardcodes never matched the real index, so the drain silently never
	// fired (r64 bug). Keying off the node is also robust to re-ordering.
	if abilityIdx < 0 || abilityIdx >= len(src.Card.AST.Abilities) {
		return
	}
	act, ok := src.Card.AST.Abilities[abilityIdx].(*gameast.Activated)
	if !ok {
		return
	}
	if _, ok := act.Effect.(*gameast.LoseLife); !ok {
		// Not the drain ability — nothing else on this card is
		// handler-owned. (The "play top of library for life" static is
		// handled by the ETB zone-cast grant, not an activated ability.)
		return
	}

	// {T} and the ten-permanent sacrifice are this ability's COST, paid by
	// the engine's activation cost path (CR §602.1b) before the ability
	// resolves. The handler owns only the EFFECT — it must not re-tap or
	// re-sacrifice. (The old code did `if src.Tapped { return }`, which
	// actually aborted the drain: this is a non-mana ability, so the
	// source is already tapped by the paid cost by the time this resolves.)
	const slug = "bolass_citadel_sac_ten"
	opps := gs.Opponents(seat)
	for _, opp := range opps {
		gameengine.LoseLife(gs, opp, 10, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"opponents_hit": len(opps),
		"life_lost_per": 10,
	})
	_ = gs.CheckEnd()
}
