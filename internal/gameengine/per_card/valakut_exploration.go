package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerValakutExploration wires Valakut Exploration.
//
// Oracle text (Scryfall, verified 2026-05-16):
//
//	Landfall — Whenever a land you control enters, exile the top card
//	of your library. You may play that card for as long as it remains
//	exiled.
//	At the beginning of your end step, if there are cards exiled with
//	this enchantment, put them into their owner's graveyard, then this
//	enchantment deals that much damage to each opponent.
//
// Implementation:
//   - permanent_etb gated on entering land controlled by us: exile the
//     top of our library; tag with "valakut_exiled" so the end-step
//     sweeper can find it. R60 batch 7: wired the "may play that
//     card for as long as it remains exiled" half via
//     NewFreeCastFromExilePermission with Duration="while_source_on_bf"
//     + SourceTimestamp pointing at Valakut. ManaCost=-1 (normal cost;
//     oracle says "may play" — not "without paying its mana cost").
//     When Valakut LTBs, ExpireSourceGrants reaps the grants. When the
//     end-step sweeper moves the cards from exile to graveyard, the
//     grants implicitly invalidate (the *Card pointer no longer
//     resides in exile, and the AI cast-from-exile path checks zone).
//   - end_step (controller's): scan controller's exile for tagged
//     cards, move them to graveyard, then deal that many damage to
//     each opponent via LoseLife (engine treats non-combat damage to
//     players as life-loss for resolution purposes).
const valakutExplorationTag = "valakut_exiled"

func registerValakutExploration(r *Registry) {
	r.OnTrigger("Valakut Exploration", "permanent_etb", valakutExplorationLandfall)
	r.OnTrigger("Valakut Exploration", "end_step", valakutExplorationEndStep)
}

func valakutExplorationLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "valakut_exploration_landfall_exile"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm {
		return
	}
	if !entering.IsLand() {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	top := seat.Library[0]
	gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "valakut_exploration_exile")
	exiledName := ""
	grantKind := "none"
	if top != nil {
		top.Types = append(top.Types, valakutExplorationTag)
		exiledName = top.DisplayName()
		// Register the play-from-exile grant tied to Valakut's
		// timestamp so ExpireSourceGrants reaps it on Valakut's LTB.
		grant := gameengine.NewFreeCastFromExilePermission(perm.Controller, perm.Card.DisplayName())
		grant.ManaCost = -1 // "may play" — pay normal cost
		grant.Duration = "while_source_on_bf"
		grant.GrantTurn = gs.Turn
		grant.SourceTimestamp = perm.Timestamp
		gameengine.RegisterZoneCastGrant(gs, top, grant)
		grantKind = "play_for_normal_cost_while_valakut_in_play"
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"land":       entering.Card.DisplayName(),
		"exiled":     exiledName,
		"grant_kind": grantKind,
	})
}

func valakutExplorationEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "valakut_exploration_end_step_burn"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	tagged := []*gameengine.Card{}
	for _, c := range seat.Exile {
		if c == nil {
			continue
		}
		if cardHasType(c, valakutExplorationTag) {
			tagged = append(tagged, c)
		}
	}
	if len(tagged) == 0 {
		return
	}
	for _, c := range tagged {
		gameengine.MoveCard(gs, c, perm.Controller, "exile", "graveyard", "valakut_exploration_eos")
		valakutStripTag(c)
	}
	dmg := len(tagged)
	hit := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		gameengine.LoseLife(gs, i, dmg, perm.Card.DisplayName())
		hit++
	}
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"yarded":   dmg,
		"opps_hit": hit,
		"damage":   dmg,
	})
}

func valakutStripTag(c *gameengine.Card) {
	if c == nil {
		return
	}
	out := c.Types[:0]
	for _, t := range c.Types {
		if t == valakutExplorationTag {
			continue
		}
		out = append(out, t)
	}
	c.Types = out
}
