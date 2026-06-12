package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// goldilocks_am_dead_effects_r63.go — A-M slice of the goldilocks
// dead-effect backlog (r63 re-baseline, 101 residuals). These three are
// the genuinely per_card-fixable A-M cards: their AST parses cleanly
// but the engine's generic resolver produces no observable change, and
// the goldilocks battery exercises the per_card hook path for their
// ability kind (InvokeETBHook for ETB triggers, InvokeActivatedHook for
// activated). The rest of the A-M residue is scaffold/extractor/engine-
// resolver territory — see /tmp/fable-review/goldilocks-fixes-am-r63.md.
//
// Oracle texts (Scryfall, verified against ast_dataset 2026-06-12):
//
//	Cloud of Faeries {1}{U} — Creature, Faerie
//	  Flying / When this creature enters, untap up to two lands. /
//	  Cycling {2}
//
//	Great Whale {5}{U}{U} — Creature, Whale
//	  When this creature enters, untap up to seven lands.
//
//	Goblin Test Pilot {U}{R} — Creature, Goblin Pilot
//	  Flying / {T}: This creature deals 2 damage to any target chosen
//	  at random.
func init() {
	registerGoldilocksAMDeadEffectsR63(Global())
	AddResetHook(registerGoldilocksAMDeadEffectsR63)
}

func registerGoldilocksAMDeadEffectsR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Cloud of Faeries", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		etbUntapLands(gs, perm, 2, "cloud_of_faeries_untap")
	})
	r.OnETB("Great Whale", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		etbUntapLands(gs, perm, 7, "great_whale_untap")
	})
	r.OnActivated("Goblin Test Pilot", goblinTestPilotShoot)
}

// etbUntapLands implements the "when this creature enters, untap up to
// N lands" family. The oracle text permits ANY lands; the greedy policy
// untaps the controller's own tapped lands first (the only pick that is
// ever correct play — Palinchron-style mana untap), then falls through
// to other seats' tapped lands so the choice is still exercised on
// boards where the controller has no tapped land. "Up to" means zero
// untaps is legal; an all-untapped board is a clean no-op.
func etbUntapLands(gs *gameengine.GameState, perm *gameengine.Permanent, n int, slug string) {
	if gs == nil || perm == nil || perm.Card == nil || n <= 0 {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	untapped := 0
	untapOn := func(seatIdx int) {
		seat := gs.Seats[seatIdx]
		if seat == nil {
			return
		}
		for _, p := range seat.Battlefield {
			if untapped >= n {
				return
			}
			if p == nil || p.Card == nil || !p.Tapped || !p.IsLand() {
				continue
			}
			p.Tapped = false
			untapped++
			gs.LogEvent(gameengine.Event{
				Kind:   "untap",
				Seat:   seatIdx,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"untapped": p.Card.DisplayName(),
					"reason":   slug,
				},
			})
		}
	}
	untapOn(controller)
	for seatIdx := range gs.Seats {
		if untapped >= n {
			break
		}
		if seatIdx != controller {
			untapOn(seatIdx)
		}
	}
	if untapped > 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     controller,
			"untapped": untapped,
			"max":      n,
		})
	}
}

// goblinTestPilotShoot — "{T}: This creature deals 2 damage to any
// target chosen at random." The {T} cost is settled by the activation
// dispatcher; this handler resolves the effect. Candidate pool per
// "any target": every living player plus every creature on any
// battlefield (planeswalkers excluded until the engine grows a
// planeswalker-damage redirect path). Random pick via the per-game
// deterministic RNG; index 0 when no RNG is wired (fixture games).
func goblinTestPilotShoot(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "goblin_test_pilot_random_shot"
	if gs == nil || src == nil || src.Card == nil {
		return
	}

	type candidate struct {
		seat int                   // player target when perm == nil
		perm *gameengine.Permanent // creature target otherwise
	}
	var pool []candidate
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		pool = append(pool, candidate{seat: i})
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.IsCreature() {
				pool = append(pool, candidate{seat: i, perm: p})
			}
		}
	}
	if len(pool) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_target", nil)
		return
	}
	pick := pool[0]
	if gs.Rng != nil {
		pick = pool[gs.Rng.Intn(len(pool))]
	}

	if pick.perm == nil {
		gameengine.DealDamage(gs, pick.seat, 2, src.Card.DisplayName())
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"target_kind": "player",
			"target_seat": pick.seat,
			"damage":      2,
		})
		return
	}
	pick.perm.MarkedDamage += 2
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   pick.seat,
		Source: src.Card.DisplayName(),
		Amount: 2,
		Details: map[string]interface{}{
			"target_card": pick.perm.Card.DisplayName(),
			"target_kind": "creature",
			"random":      true,
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"target_kind": "creature",
		"target":      pick.perm.Card.DisplayName(),
		"damage":      2,
	})
}
