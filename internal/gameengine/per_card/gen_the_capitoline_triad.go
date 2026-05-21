package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheCapitolineTriad wires The Capitoline Triad.
//
// Oracle text:
//
//	Those Who Came Before — This spell costs {1} less to cast for
//	each historic card in your graveyard. (Artifacts, legendaries,
//	and Sagas are historic.)
//	Exile any number of historic cards from your graveyard with total
//	mana value 30 or greater: You get an emblem with "Creatures you
//	control have base power and toughness 9/9."
//
// Implementation:
//   - Cost reduction is engine-deep cast-time; partial breadcrumb +
//     seat flag.
//   - Activated ability: scan graveyard for historic cards (artifact /
//     legendary / saga). Greedily collect highest-CMC ones until the
//     total mana value reaches 30. If we can hit 30, exile them and
//     stamp the emblem flag on the seat. The actual base-9/9 grant is
//     engine layer 7b (set base P/T) — we record the marker via flag
//     and emit a partial.
func registerTheCapitolineTriad(r *Registry) {
	r.OnETB("The Capitoline Triad", capitolineTriadETBSetup)
	r.OnActivated("The Capitoline Triad", capitolineTriadEmblemActivate)
	// R55: emblem refresh keyed on permanent_etb so future creatures
	// pick up the 9/9 base after the activated ability has fired.
	r.OnTrigger("The Capitoline Triad", "permanent_etb", capitolineTriadEmblemRefresh)
}

func capitolineTriadETBSetup(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "capitoline_triad_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["capitoline_historic_discount_active"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Per-historic-graveyard-card cost reduction is wired in
	// gameengine/cost_modifiers.go (R49 batchD).
}

func capitolineTriadEmblemActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "capitoline_triad_emblem"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Collect historic graveyard cards sorted by CMC descending.
	type hist struct {
		idx int
		cmc int
	}
	var picks []hist
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") && !cardHasType(c, "legendary") && !cardHasType(c, "saga") {
			continue
		}
		picks = append(picks, hist{i, cardCMC(c)})
	}
	// Sort descending CMC.
	for i := 0; i < len(picks); i++ {
		for j := i + 1; j < len(picks); j++ {
			if picks[j].cmc > picks[i].cmc {
				picks[i], picks[j] = picks[j], picks[i]
			}
		}
	}
	total := 0
	consumed := []int{}
	for _, p := range picks {
		consumed = append(consumed, p.idx)
		total += p.cmc
		if total >= 30 {
			break
		}
	}
	if total < 30 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_historic_mv", map[string]interface{}{
			"total": total,
		})
		return
	}
	// Exile in reverse-index order so deletes don't shift.
	idxSet := map[int]bool{}
	for _, i := range consumed {
		idxSet[i] = true
	}
	newGY := make([]*gameengine.Card, 0, len(seat.Graveyard))
	for i, c := range seat.Graveyard {
		if idxSet[i] {
			seat.Exile = append(seat.Exile, c)
			continue
		}
		newGY = append(newGY, c)
	}
	seat.Graveyard = newGY
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["capitoline_emblem_base_9_9"] = 1

	// R55: wire the "Creatures you control have base power and
	// toughness 9/9" emblem via the Layer 7b RegisterSetPT primitive
	// added in R54. Each current creature gets a SET-9/9 effect;
	// future creatures must be picked up via permanent_etb refresh
	// (registered separately below at the trigger level — the emblem
	// outlives the Triad, so the refresh hook needs to live on the
	// seat-level flag, not the Triad's perm hook).
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		gameengine.RegisterSetPT(gs, p, 9, 9,
			gameengine.DurationPermanent,
			"The Capitoline Triad (emblem)",
			"capitoline_emblem_base_9_9")
		stamped++
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"exiled":   len(consumed),
		"total_mv": total,
		"stamped":  stamped,
	})
}

// capitolineTriadEmblemRefresh stamps the emblem 9/9 onto creatures
// that enter the controller's battlefield AFTER the emblem fires. The
// emblem persists even after the Triad leaves, so this refresh hook is
// registered for any perm with the capitoline_emblem_base_9_9 seat
// flag set — even if the Triad itself is gone. Currently this means
// the refresh is keyed off creature_etb events firing on ANY perm in
// the seat — see the registration block below.
func capitolineTriadEmblemRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags["capitoline_emblem_base_9_9"] != 1 {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered.Card == nil || !entered.IsCreature() {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	gameengine.RegisterSetPT(gs, entered, 9, 9,
		gameengine.DurationPermanent,
		"The Capitoline Triad (emblem)",
		"capitoline_emblem_base_9_9")
}

