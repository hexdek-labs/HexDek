package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #19 — artifact-commander synergy handlers.
//
// Drafna, Founder of Lat-Nam: activate to create a token copy of the
// best non-token artifact you own.
// Jolene, the Plunder Queen: whenever a Treasure token enters under
// your control, put a +1/+1 counter on Jolene.

// ---------------------------------------------------------------------------
// Drafna, Founder of Lat-Nam
//
// Oracle text (paraphrased for engine MVP):
//   {2}{U}, {T}: Create a token that's a copy of target nonland,
//   nontoken artifact you control.
//
// Implementation: pick the highest-CMC non-token artifact permanent
// the controller owns and create a token copy of it. The copy is a
// fresh Permanent whose Card is a DeepCopy of the source with "token"
// prepended to Types. ETB cascade fires for the copy so any artifact
// ETB triggers (Treasure-makers, mana rocks, etc.) resolve normally.
// ---------------------------------------------------------------------------

func registerDrafna(r *Registry) {
	r.OnActivated("Drafna, Founder of Lat-Nam", drafnaActivated)
}

func drafnaActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "drafna_token_copy"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	var best *gameengine.Permanent
	bestCMC := -1
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsArtifact() || p.IsToken() || p.IsLand() {
			continue
		}
		cmc := gameengine.ManaCostOf(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	if best == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_nontoken_artifact_to_copy", nil)
		return
	}

	// Phase 5 chokepoint: MintTokenAsCopyOf DeepCopys the source AND
	// clears the inherited InstanceID before stamping a fresh TK ID. The
	// hand-rolled DeepCopy path that previously lived here inherited
	// Spikeshell Harrier's OG ID onto the token, producing the
	// "h1OGVU500020 appears in both seat 1 battlefield and seat 1
	// battlefield" CardIdentity violation surfaced by Loki r60 game 4635.
	// MintTokenAsCopyOf also handles the "token" type prepend and the
	// Owner reassignment, so the manual splice below is no longer needed.
	enablerID := gameengine.CurrentMintEnablerID(gs)
	card := gameengine.MintTokenAsCopyOf(gs, best.Card, seat, enablerID)
	if card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "mint_token_returned_nil", nil)
		return
	}

	perm := &gameengine.Permanent{
		Card:                   card,
		Controller:             seat,
		Owner:                  seat,
		Tapped:                 false,
		SummoningSick:          true,
		Timestamp:              gs.NextTimestamp(),
		Counters:               map[string]int{},
		Flags:                  map[string]int{},
		CopiedTargetInstanceID: best.Card.InstanceID,
	}
	s.Battlefield = append(s.Battlefield, perm)
	gameengine.RegisterReplacementsForPermanent(gs, perm)
	gameengine.FirePermanentETBTriggers(gs, perm)

	// Fire token_created so downstream triggers (Anointed Procession,
	// Jolene, Chatterfang, etc.) see the token. Use the engine's
	// re-entrancy guard to avoid recursive doubling.
	if gs.Flags == nil || gs.Flags["in_token_trigger"] == 0 {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		gameengine.FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": seat,
			"count":           1,
			"types":           card.Types,
			"source":          src.Card.DisplayName(),
		})
		gs.Flags["in_token_trigger"] = 0
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"copy_of": best.Card.DisplayName(),
		"cmc":     bestCMC,
	})
}

// ---------------------------------------------------------------------------
// Jolene, the Plunder Queen
//
// Oracle text (relevant clause):
//   Whenever you create one or more Treasure tokens, put a +1/+1
//   counter on Jolene, the Plunder Queen.
//
// Implementation: OnTrigger("token_created") — increments perm.Counters
// by one (regardless of how many Treasure tokens were created in the
// same event, matching the "one or more" wording).
// ---------------------------------------------------------------------------

func registerJolene(r *Registry) {
	r.OnTrigger("Jolene, the Plunder Queen", "token_created", joleneTreasureTrigger)
}

func joleneTreasureTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jolene_treasure_counter"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}
	types, _ := ctx["types"].([]string)
	isTreasure := false
	for _, t := range types {
		if strings.EqualFold(t, "treasure") {
			isTreasure = true
			break
		}
	}
	if !isTreasure {
		return
	}

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"counters": perm.Counters["+1/+1"],
	})
}
