package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAltairIbnLaAhad wires Altaïr Ibn-La'Ahad. Batch #31.
//
// Oracle text (Assassin's Creed, {R}{W}{B}, 3/3 Legendary Creature —
// Human Assassin):
//
//	First strike
//	Whenever Altaïr attacks, exile up to one target Assassin creature
//	card from your graveyard with a memory counter on it. Then for
//	each creature card you own in exile with a memory counter on it,
//	create a tapped and attacking token that's a copy of it. Exile
//	those tokens at end of combat.
//
// Implementation:
//   - "creature_attacks" gated on atk == perm.
//   - Memory counters are not modeled at the card-in-zone level (no
//     Card.Counters field; the engine only carries counters on
//     battlefield Permanents). We approximate by tagging cards with a
//     "memory_counter" entry in Card.Types when Altaïr exiles them.
//     Cards already exiled from earlier Freedom-style triggers (Memory:
//     mechanic from Assassin's Creed) won't carry the tag, so we treat
//     this as a per-Altair pool: each attack exiles a fresh Assassin
//     and re-creates copies of every Altair-tagged exile.
//   - Pick: highest-power Assassin creature card in our graveyard.
//     Tokens that died into the graveyard are skipped (state-based
//     actions already removed them, but defensive).
//   - For each tagged card in our exile, deep-copy and create a token
//     copy on our battlefield, tapped and attacking the same defender
//     as Altaïr.
//   - Register an "end_of_combat" delayed trigger to exile those tokens
//     (CR §706.10 and the printed clause).
//
// Caveats (emitPartial):
//   - "memory counter" is approximated via a Card.Types tag; the
//     mechanic's true semantics (zone-counters that survive shuffles
//     and zone changes) are not modeled.
//   - The "up to one target" mode is taken greedily — we always pick
//     the best Assassin if one exists, never zero.
func registerAltairIbnLaAhad(r *Registry) {
	r.OnTrigger("Altaïr Ibn-La'Ahad", "creature_attacks", altairAttackCopySwarm)
}

const altairMemoryTag = "memory_counter"

func altairAttackCopySwarm(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "altair_ibn_la_ahad_attack_copy_swarm"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	defenderSeat, _ := gameengine.AttackerDefender(perm)
	if defenderSeat < 0 {
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_defender", nil)
		return
	}

	// Step 1: pick best Assassin in our graveyard, exile it, tag it.
	pickedName := ""
	if pick := altairBestAssassinInGraveyard(seat); pick != nil {
		pickedName = pick.DisplayName()
		altairTagMemory(pick)
		gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "exile", "altair_attack_exile")
	}

	// Step 2: for every Altair-tagged creature card in our exile, create a
	// tapped+attacking token copy.
	var createdTokens []*gameengine.Permanent
	for _, c := range seat.Exile {
		if c == nil || !altairHasMemoryTag(c) {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if c.Owner != perm.Controller {
			continue
		}
		token := altairBuildCopyToken(c, perm.Controller)
		tok := &gameengine.Permanent{
			Card:          token,
			Controller:    perm.Controller,
			Owner:         perm.Controller,
			Tapped:        true,
			SummoningSick: false,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{"attacking": 1, "altair_copy_token": 1},
		}
		seat.Battlefield = append(seat.Battlefield, tok)
		gameengine.SetAttackerDefender(tok, defenderSeat)
		gameengine.RegisterReplacementsForPermanent(gs, tok)
		gameengine.FirePermanentETBTriggers(gs, tok)
		createdTokens = append(createdTokens, tok)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":          slug,
				"token":         token.DisplayName(),
				"copy_of":       c.DisplayName(),
				"tapped":        true,
				"attacking":     true,
				"defender_seat": defenderSeat,
				"reason":        "altair_attack_copy",
			},
		})
	}

	// Step 3: register an end-of-combat delayed trigger to exile the tokens.
	if len(createdTokens) > 0 {
		tokens := createdTokens
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_combat",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName() + " (altair attack copies)",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, tok := range tokens {
					if tok == nil {
						continue
					}
					gameengine.ExilePermanent(gs, tok, perm)
				}
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"exiled_pick":   pickedName,
		"tokens":        len(createdTokens),
	})
	if pickedName == "" {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"no_assassin_in_graveyard_at_attack_time")
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"memory_counter_modeled_via_card_types_tag_only_for_altair_pool")
}

// altairBestAssassinInGraveyard picks the highest-power Assassin creature
// card from the seat's graveyard. Returns nil if none exist.
func altairBestAssassinInGraveyard(seat *gameengine.Seat) *gameengine.Card {
	if seat == nil {
		return nil
	}
	var best *gameengine.Card
	bestPow := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if !cardHasType(c, "assassin") {
			continue
		}
		pow := c.BasePower
		if pow > bestPow {
			bestPow = pow
			best = c
		}
	}
	return best
}

func altairTagMemory(c *gameengine.Card) {
	if c == nil {
		return
	}
	for _, t := range c.Types {
		if t == altairMemoryTag {
			return
		}
	}
	c.Types = append(c.Types, altairMemoryTag)
}

func altairHasMemoryTag(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == altairMemoryTag {
			return true
		}
	}
	return false
}

// altairBuildCopyToken deep-copies a card and stamps it as a token. The
// copy keeps the original's types, P/T, colors, and name (with a
// (token copy) suffix for log clarity).
func altairBuildCopyToken(src *gameengine.Card, owner int) *gameengine.Card {
	cp := src.DeepCopy()
	cp.Owner = owner
	cp.IsCopy = true
	cp.Name = src.DisplayName() + " (altair copy)"
	hasToken := false
	for _, t := range cp.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		cp.Types = append(cp.Types, "token")
	}
	return cp
}
