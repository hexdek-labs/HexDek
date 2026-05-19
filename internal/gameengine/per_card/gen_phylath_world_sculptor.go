package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhylathWorldSculptor wires Phylath, World Sculptor.
//
// Oracle text (Scryfall, verified):
//
//	When Phylath enters, create a 0/1 green Plant creature token for
//	each basic land you control.
//	Landfall — Whenever a land you control enters, put four +1/+1
//	counters on target Plant you control.
//
// Implementation (R45 stub port):
//   - ETB: count basic lands the controller controls and create that
//     many 0/1 green Plant tokens via CreateCreatureToken. The
//     auto-generated stub created a single unnamed token; the port
//     replaces it with a proper per-basic spawn loop and the
//     correct "Plant" creature type + green color.
//   - Landfall: OnTrigger("permanent_etb") gated on a land entering
//     under the controller's control. AI policy: stamp 4 +1/+1
//     counters on the highest-toughness Plant the controller
//     controls (best body to grow). Skip if no Plant available.
func registerPhylathWorldSculptor(r *Registry) {
	r.OnETB("Phylath, World Sculptor", phylathWorldSculptorETB)
	r.OnTrigger("Phylath, World Sculptor", "permanent_etb", phylathLandfall)
}

func phylathWorldSculptorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "phylath_world_sculptor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	basicCount := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "basic") {
			basicCount++
		}
	}
	for i := 0; i < basicCount; i++ {
		tok := gameengine.CreateCreatureToken(
			gs,
			seat,
			"Plant",
			[]string{"creature", "plant"},
			0, 1,
		)
		if tok != nil && tok.Card != nil {
			tok.Card.Colors = []string{"G"}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"basics": basicCount,
		"plants": basicCount,
	})
}

func phylathLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phylath_landfall_plant_counters"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered.Card == nil || !entered.IsLand() {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	// Pick highest-toughness Plant under controller's control.
	var pick *gameengine.Permanent
	bestT := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "plant") {
			continue
		}
		t := p.Toughness()
		if t > bestT {
			bestT = t
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_plant_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	pick.AddCounter("+1/+1", 4)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"land_entered": entered.Card.DisplayName(),
		"plant":        pick.Card.DisplayName(),
		"counters":     4,
	})
}
