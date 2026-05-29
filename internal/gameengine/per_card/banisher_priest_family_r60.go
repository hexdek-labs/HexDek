package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Phase 3 LTB-return family (docs/instanceid-system-v2-r60.md §4.2 + §7).
//
// Banisher Priest, Oblivion Ring, Detention Sphere, Faceless Butcher all
// share the canonical CR §406.7 "exile until ~ leaves the battlefield"
// shape:
//
//   - ETB picks an opp permanent matching the card's filter.
//   - The target's Card moves to the OWNER's exile (§400.7c) via the
//     engine's ExileLinked primitive, which also stamps:
//       - perm.LinkedExile += card (pointer-side legacy tracking)
//       - card.ExiledByTimestamp = perm.Timestamp
//       - perm.ExiledByMe += card.InstanceID (Phase 3 source-held linkage)
//       - perm.LinkageKind = LTBReturn (when previously LinkageNone)
//   - On LTB (death, bounce, exile, commander-zone — any zone-change
//     out of the battlefield per design v2 §7), ReturnLinkedExile walks
//     LinkedExile and routes each card to the appropriate zone
//     (battlefield for these four; CR §406.7).
//
// Pattern reference: registerHostageTaker in per_card_batch_ai_r60.go
// (LTB-return + cast-grant) — this batch is the cast-grant-free subset.

// ---------------------------------------------------------------------------
// Banisher Priest
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Banisher%20Priest):
//
//	When this creature enters, exile target creature an opponent controls
//	until this creature leaves the battlefield.
//
// {1}{W}{W} 2/2 Creature — Human Cleric. Standard removal-on-a-body,
// archetype-defining for white midrange / blink shells. CR §406.7
// linked-exile semantics — return on LTB.
func registerBanisherPriest(r *Registry) {
	r.OnETB("Banisher Priest", banisherPriestETB)
	r.OnTrigger("Banisher Priest", "permanent_ltb", ltbReturnLinkedExile)
}

func banisherPriestETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "banisher_priest_exile"
	exileOppCreatureLinked(gs, perm, slug)
}

// ---------------------------------------------------------------------------
// Oblivion Ring
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Oblivion%20Ring):
//
//	When Oblivion Ring enters, exile another target nonland permanent an
//	opponent controls until Oblivion Ring leaves the battlefield.
//
// {2}{W} Enchantment. The original o-ring — broader than Banisher Priest
// (any nonland permanent, not just creatures). Same CR §406.7 LTB-return
// shape.
func registerOblivionRing(r *Registry) {
	r.OnETB("Oblivion Ring", oblivionRingETB)
	r.OnTrigger("Oblivion Ring", "permanent_ltb", ltbReturnLinkedExile)
}

func oblivionRingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "oblivion_ring_exile"
	exileOppNonlandLinked(gs, perm, slug)
}

// ---------------------------------------------------------------------------
// Detention Sphere
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Detention%20Sphere):
//
//	When this enchantment enters, exile target nonland permanent an
//	opponent controls and all other permanents with the same name as
//	that permanent until this enchantment leaves the battlefield.
//
// {1}{W}{U} Enchantment. The fanned-out o-ring variant. Picks one target,
// then sweeps every other permanent with the same name (token sweepers
// like the Saproling pile, Soldier piles, Goblin piles).
func registerDetentionSphere(r *Registry) {
	r.OnETB("Detention Sphere", detentionSphereETB)
	r.OnTrigger("Detention Sphere", "permanent_ltb", ltbReturnLinkedExile)
}

func detentionSphereETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "detention_sphere_exile"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	caster := perm.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	target := pickOppNonlandTarget(gs, caster, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_valid_nonland_target", map[string]interface{}{
			"caster": caster,
		})
		return
	}
	targetName := target.Card.DisplayName()

	// Exile the primary target.
	exileLinkedPermanent(gs, perm, target)
	exiledCount := 1
	swept := []string{targetName}

	// Sweep every OTHER permanent with the same name across the table
	// (including caster-side same-name permanents — oracle says "all
	// other permanents with the same name", not "opponent's").
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Snapshot first; the sweep mutates Battlefield slices.
		snapshot := make([]*gameengine.Permanent, len(s.Battlefield))
		copy(snapshot, s.Battlefield)
		for _, p := range snapshot {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if p.Card.DisplayName() != targetName {
				continue
			}
			exileLinkedPermanent(gs, perm, p)
			swept = append(swept, p.Card.DisplayName())
			exiledCount++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster":        caster,
		"primary":       targetName,
		"exiled_count":  exiledCount,
		"swept_names":   swept,
	})
}

// ---------------------------------------------------------------------------
// Faceless Butcher
// ---------------------------------------------------------------------------
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Faceless%20Butcher):
//
//	When Faceless Butcher enters, exile target creature other than
//	Faceless Butcher until Faceless Butcher leaves the battlefield.
//
// {2}{B}{B} 2/3 Creature — Nightmare Horror. The black equivalent of
// Banisher Priest — note "target creature OTHER THAN ~" includes any
// non-self creature on the battlefield (caster's own creatures legal,
// though the AI picks opp targets). LTB returns same as the rest.
func registerFacelessButcher(r *Registry) {
	r.OnETB("Faceless Butcher", facelessButcherETB)
	r.OnTrigger("Faceless Butcher", "permanent_ltb", ltbReturnLinkedExile)
}

func facelessButcherETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "faceless_butcher_exile"
	exileOppCreatureLinked(gs, perm, slug)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// ltbReturnLinkedExile is the canonical "permanent_ltb" trigger handler
// for LTBReturn-shape cards. Walks perm.LinkedExile via the engine's
// ReturnLinkedExile primitive, which also clears perm.ExiledByMe and
// resets each card's ExiledByTimestamp. ExpireSourceGrants is fired by
// the canonical LTB dispatch path elsewhere (PR #106 plumbing).
func ltbReturnLinkedExile(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ltb_return_linked_exile"
	if gs == nil || perm == nil {
		return
	}
	if len(perm.LinkedExile) == 0 {
		return
	}
	returned := len(perm.LinkedExile)
	gameengine.ReturnLinkedExile(gs, perm, "battlefield")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"returned_count": returned,
		"linkage_kind":   perm.LinkageKind.String(),
	})
}

// exileOppCreatureLinked picks the best opp creature and exiles it
// linked to perm. Used by Banisher Priest, Faceless Butcher.
func exileOppCreatureLinked(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	caster := perm.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	target := pickOppCreatureTarget(gs, caster, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_valid_creature_target", map[string]interface{}{
			"caster": caster,
		})
		return
	}
	exileLinkedPermanent(gs, perm, target)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster": caster,
		"target": target.Card.DisplayName(),
		"owner":  target.Owner,
	})
}

// exileOppNonlandLinked picks the best opp nonland permanent and exiles
// it linked to perm. Used by Oblivion Ring.
func exileOppNonlandLinked(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	caster := perm.Controller
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	target := pickOppNonlandTarget(gs, caster, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_valid_nonland_target", map[string]interface{}{
			"caster": caster,
		})
		return
	}
	exileLinkedPermanent(gs, perm, target)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster": caster,
		"target": target.Card.DisplayName(),
		"owner":  target.Owner,
	})
}

// exileLinkedPermanent detaches target from its controller's battlefield
// and threads the Card through ExileLinked. Mirrors the Hostage Taker
// sequence — removePermanent + UnregisterReplacementsForPermanent first,
// then ExileLinked (which routes the Card to OWNER's exile per §400.7c
// and stamps source-held InstanceID linkage via the Phase 3 hooks in
// state.go ExileLinked).
func exileLinkedPermanent(gs *gameengine.GameState, source, target *gameengine.Permanent) {
	if gs == nil || source == nil || target == nil || target.Card == nil {
		return
	}
	owner := target.Owner
	tgtCard := target.Card
	removePermanent(gs, target)
	gs.UnregisterReplacementsForPermanent(target)
	gameengine.ExileLinked(gs, source, tgtCard, owner, "battlefield")
}

// pickOppCreatureTarget — best opp creature (power desc, then CMC desc).
// Hexproof / shroud not modeled at picker level (targeting validation
// fires at cast time; this is post-cast resolution, so legality locked).
func pickOppCreatureTarget(gs *gameengine.GameState, caster int, source *gameengine.Permanent) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestScore := -1
	for _, opp := range gs.Opponents(caster) {
		if gs.Seats[opp] == nil {
			continue
		}
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.Card == nil || p == source {
				continue
			}
			if !cardHasType(p.Card, "creature") {
				continue
			}
			score := p.Power()*10 + p.Card.CMC
			if score > bestScore {
				bestScore = score
				best = p
			}
		}
	}
	return best
}

// pickOppNonlandTarget — best opp nonland permanent. Tiered:
//
//   - creatures (power desc)
//   - planeswalkers (loyalty desc)
//   - artifacts / enchantments (CMC desc)
//
// Lands explicitly excluded per oracle text on both Oblivion Ring and
// Detention Sphere.
func pickOppNonlandTarget(gs *gameengine.GameState, caster int, source *gameengine.Permanent) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestTier := 0
	bestScore := -1
	for _, opp := range gs.Opponents(caster) {
		if gs.Seats[opp] == nil {
			continue
		}
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.Card == nil || p == source {
				continue
			}
			if p.IsLand() {
				continue
			}
			tier := 0
			score := 0
			switch {
			case cardHasType(p.Card, "planeswalker"):
				tier = 4
				score = p.Card.CMC
			case cardHasType(p.Card, "creature"):
				tier = 3
				score = p.Power()*10 + p.Card.CMC
			case cardHasType(p.Card, "artifact"), cardHasType(p.Card, "enchantment"):
				tier = 2
				score = p.Card.CMC
			default:
				tier = 1
				score = p.Card.CMC
			}
			if tier > bestTier || (tier == bestTier && score > bestScore) {
				bestTier = tier
				bestScore = score
				best = p
			}
		}
	}
	return best
}
