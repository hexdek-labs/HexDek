package per_card

import (
	"fmt"
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVren wires Vren, the Relentless.
//
// Oracle text:
//
//	Ward {2}
//	If a creature an opponent controls would die, exile it instead.
//	At the beginning of each end step, create X 1/1 black Rat creature
//	tokens with "This token gets +1/+1 for each other Rat you control,"
//	where X is the number of creatures that were exiled under your
//	opponents' control this turn.
//
// Implementation:
//   - Ward {2}: handled by the AST keyword pipeline; not wired here.
//   - ETB: register a §614 would_die replacement that retargets the dying
//     opponent-controlled creature to exile, mirroring Anafenza, the
//     Foremost. We tally each successful exile in
//     seat.Flags["vren_exiled_t<turn>"] for the end-step trigger.
//   - "end_step": create X 1/1 black Rat tokens, where X is the per-turn
//     exile counter. The token's "This token gets +1/+1 for each other
//     Rat you control" anthem is evaluated at token creation time as a
//     baked-in static stat boost (engine has no per-token continuous
//     anthem hook); each token is sized to current other-Rat count at
//     creation. emitPartial flags the dynamic-anthem limitation.
//
// Scope note: the oracle's "creatures that were exiled under your
// opponents' control this turn" technically counts ANY creature an
// opponent controlled and exiled this turn, regardless of cause. We
// only count Vren's own replacement-driven exiles, which is the
// dominant case. emitPartial flags the broader semantics.
func registerVren(r *Registry) {
	r.OnETB("Vren, the Relentless", vrenETB)
	r.OnTrigger("Vren, the Relentless", "end_step", vrenEndStep)
}

func vrenETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "vren_exile_replacement"
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      "Vren, the Relentless:exile_opp_creature:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.TargetPerm == nil {
				return false
			}
			tp := ev.TargetPerm
			if tp.Controller == controller {
				return false
			}
			if !tp.IsCreature() {
				return false
			}
			// Already redirected somewhere other than graveyard? Skip.
			if ev.String("to_zone") != "" && ev.String("to_zone") != "graveyard" {
				return false
			}
			return true
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			seat := gs.Seats[controller]
			if seat != nil {
				if seat.Flags == nil {
					seat.Flags = map[string]int{}
				}
				seat.Flags[vrenExileKey(gs.Turn)]++
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Vren, the Relentless",
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "opp_creature_die_to_exile",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_die",
		"scope":    "opponent_controlled_creature",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"end_step_token_count_only_tracks_vren_replacement_exiles_and_token_anthem_baked_at_creation")
}

func vrenEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vren_end_step_rat_tokens"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	turnKey := vrenExileKey(gs.Turn)
	x := seat.Flags[turnKey]
	if x <= 0 {
		// Still clear stale prior-turn keys to keep Flags small.
		clearStaleVrenKeys(seat, gs.Turn)
		return
	}
	// Reset for the next turn cycle.
	delete(seat.Flags, turnKey)
	clearStaleVrenKeys(seat, gs.Turn)

	// Count "other Rats you control" — used as baked-in anthem at creation.
	otherRats := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasType(p.Card, "rat") {
			otherRats++
		}
	}

	// Each token is 1/1 + (otherRats) — note "other" excludes the token
	// itself but includes prior tokens once they've entered the battlefield.
	// We update the running count after each token enters.
	for i := 0; i < x; i++ {
		boost := otherRats
		token := &gameengine.Card{
			Name:          "Rat Token",
			Owner:         perm.Controller,
			Types:         []string{"creature", "token", "rat", "pip:B"},
			BasePower:     1 + boost,
			BaseToughness: 1 + boost,
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":    "Rat Token",
				"reason":   "vren_end_step",
				"power":    1 + boost,
				"tough":    1 + boost,
				"anthem_n": boost,
			},
		})
		otherRats++ // newly minted rat counts toward later siblings.
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"tokens_minted": x,
		"final_rats":    otherRats,
	})
}

func vrenExileKey(turn int) string {
	return fmt.Sprintf("vren_exiled_t%d", turn+1)
}

// clearStaleVrenKeys removes vren_exiled_t* keys older than the current
// turn so seat.Flags doesn't accumulate over a long game.
func clearStaleVrenKeys(seat *gameengine.Seat, currentTurn int) {
	if seat == nil || seat.Flags == nil {
		return
	}
	prefix := "vren_exiled_t"
	for k := range seat.Flags {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		// Parse the suffix integer. Stored as currentTurn+1 sentinel.
		nStr := k[len(prefix):]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			continue
		}
		if n <= currentTurn {
			delete(seat.Flags, k)
		}
	}
}
