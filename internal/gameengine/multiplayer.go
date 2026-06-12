package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase 9 — multiplayer N-seat generalization (CR §800, §101.4).
//
// This file extends the 2-player MVP decision sites (attack targeting,
// end-of-game detection, APNAP iteration) to handle arbitrary seat counts
// (3, 4, 5, +). It mirrors the Python reference at scripts/playloop.py:
//
//   - Game.opp / Game.opponents / Game.living_opponents / Game.apnap_order
//   - Game.check_end          → CheckEnd here
//   - _handle_seat_elimination → HandleSeatElimination here
//
// §800.4 spans multiple sub-rules (§800.4a–§800.4p). We implement the
// hot-path subset relevant to the 4-player EDH gauntlet:
//
//   §800.4a  — leave-the-game cleanup (objects owned leave; controlled
//              objects on stack cease/exile; continuous/replacement
//              effects this seat sourced are dropped).
//   §800.4b  — control-change to a left player doesn't happen.
//   §800.4e  — combat damage to a left player isn't assigned (handled
//              via CheckEnd short-circuit + living-only attacker pick).
//
// §101.4    — APNAP: simultaneous choices resolve in turn order starting
//              from the active player.
//
// §104.2a / §104.3b — game ends when 1 or 0 living seats remain.
//
// state.go is the type-definition file; this module is pure behavior.

// -----------------------------------------------------------------------------
// Opponent / APNAP helpers (additive on GameState)
// -----------------------------------------------------------------------------

// OpponentsOf returns the seat indices of every non-source seat,
// INCLUDING dead ones, in APNAP order anchored at `seatIdx` (the seat
// AFTER seatIdx in turn order comes first). Mirrors Python
// Game.opponents — dead-inclusive so the caller can filter or not based
// on context. For living-only use LivingOpponents.
//
// Used by: each-opponent effect fan-out, threat-score iteration, §800.4
// cleanup ("next player in turn order").
func (gs *GameState) OpponentsOf(seatIdx int) []int {
	if gs == nil {
		return nil
	}
	n := len(gs.Seats)
	if n == 0 {
		return nil
	}
	out := make([]int, 0, n-1)
	for k := 1; k < n; k++ {
		cand := (seatIdx + k) % n
		if cand == seatIdx {
			continue
		}
		out = append(out, cand)
	}
	return out
}

// LivingOpponents returns non-source seats that aren't Lost, in APNAP
// order from seatIdx. CR §104.2a — a player "in the game" is one who
// hasn't lost. Mirrors Python Game.living_opponents.
//
// Combat target selection, "each opponent" fan-out, and policy/threat
// scoring should all use this rather than OpponentsOf, so effects don't
// leak onto eliminated seats (§800.4b / §800.4e).
func (gs *GameState) LivingOpponents(seatIdx int) []int {
	if gs == nil {
		return nil
	}
	all := gs.OpponentsOf(seatIdx)
	out := make([]int, 0, len(all))
	for _, i := range all {
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		out = append(out, i)
	}
	return out
}

// APNAPOrder returns every seat (living + dead) in APNAP order starting
// from `fromSeat`. CR §101.4a — "starting with the active player and
// proceeding in turn order." If fromSeat is out of range, anchors at
// gs.Active. Dead seats are included because some CR corners (§800.4h
// last-known-info fallback) still reference them; callers that want
// respondent polling should filter Lost themselves.
func (gs *GameState) APNAPOrder(fromSeat int) []int {
	if gs == nil {
		return nil
	}
	n := len(gs.Seats)
	if n == 0 {
		return nil
	}
	anchor := fromSeat
	if anchor < 0 || anchor >= n {
		anchor = gs.Active
	}
	out := make([]int, 0, n)
	for k := 0; k < n; k++ {
		out = append(out, (anchor+k)%n)
	}
	return out
}

// APNAPOrder returns seat indices in APNAP order starting from the
// active player, then clockwise through non-active players, skipping
// eliminated (Lost) seats.
// Per CR §101.4: "If multiple players would make choices and/or take
// actions at the same time, the active player makes any choices first,
// then each other player in turn order makes choices."
//
// This is a package-level convenience function for trigger ordering;
// the method GameState.APNAPOrder(fromSeat) includes dead seats and
// anchors at an arbitrary seat — use that for full-seat enumeration.
func APNAPOrder(gs *GameState) []int {
	if gs == nil {
		return nil
	}
	nSeats := len(gs.Seats)
	if nSeats == 0 {
		return nil
	}
	order := make([]int, 0, nSeats)
	for i := 0; i < nSeats; i++ {
		idx := (gs.Active + i) % nSeats
		if gs.Seats[idx] != nil && !gs.Seats[idx].Lost {
			order = append(order, idx)
		}
	}
	return order
}

// Opp returns the first living non-source seat in APNAP order from
// seatIdx. Legacy 2-player compatibility shim (mirrors Python Game.opp).
// Callers needing ALL opponents should use LivingOpponents / OpponentsOf.
//
// If all opponents are dead, falls back to any non-source seat so
// `.Life` reads don't crash; the game should already be ended by then
// via CheckEnd.
func (gs *GameState) Opp(seatIdx int) int {
	if gs == nil || len(gs.Seats) == 0 {
		return seatIdx
	}
	living := gs.LivingOpponents(seatIdx)
	if len(living) > 0 {
		return living[0]
	}
	all := gs.OpponentsOf(seatIdx)
	if len(all) > 0 {
		return all[0]
	}
	return seatIdx
}

// LivingSeats returns the indices of every seat whose Lost flag is
// false. Used by CheckEnd + "each player" fan-outs that should exclude
// eliminated seats (per §800.4).
func (gs *GameState) LivingSeats() []int {
	if gs == nil {
		return nil
	}
	out := make([]int, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		out = append(out, i)
	}
	return out
}

// NextLivingSeat returns the index of the next non-lost seat after
// gs.Active, wrapping around the table. Falls back to gs.Active when
// no other living seats remain (caller's responsibility to check the
// game-end condition before iterating). Used by analysis tools that
// step a finished game state through the turn order without invoking
// the engine's turn-loop.
func NextLivingSeat(gs *GameState) int {
	n := len(gs.Seats)
	for offset := 1; offset <= n; offset++ {
		idx := (gs.Active + offset) % n
		if gs.Seats[idx] != nil && !gs.Seats[idx].Lost {
			return idx
		}
	}
	return gs.Active
}

// -----------------------------------------------------------------------------
// CheckEnd — CR §104.2a last-seat-standing + §800.4 elimination sweep
// -----------------------------------------------------------------------------

// CheckEnd is the engine-level master aggregator for CR §104.2 / §104.3.
// Flips gs.Flags["ended"] = 1 when the game has resolved. Also runs
// §800.4a cleanup on any seat whose Lost flag flipped true since the
// previous call (idempotent via Seat.LeftGame).
//
// Resolution order:
//
//   - §800.4a cleanup pass on newly-Lost seats (eliminate first, then
//     evaluate end conditions).
//   - §104.2c — "you win the game" effects. Any seat with Won=true AND
//     Lost=false (the §104.3a "loss beats win when simultaneous on the
//     same player" gate) wins immediately. Multiple such seats in the
//     same window → simultaneous-win draw per §104.3b. The canonical
//     path for win-effects is resolveWinGame in resolve.go (sets Won
//     on the controller of a gameast.WinGame node) plus the alt-win
//     replacement handlers (Lab Maniac, Jace Wielder); Thassa's Oracle
//     also goes through here via emitWin.
//   - §104.2a — last seat standing wins. gs.Flags["winner"] = seat_idx.
//   - §104.3b — zero living seats → draw. gs.Flags["winner"] unset.
//
// Contract:
//
//   - ≥2 living seats AND no §104.2c winner → returns false (game
//     continues).
//   - exactly 1 §104.2c winner OR exactly 1 living seat → returns true
//     with gs.Flags["winner"] set + Won flag on the winner.
//   - ≥2 §104.2c winners OR 0 living seats → returns true with no
//     winner flag (draw).
//
// Always safe to call multiple times per SBA pass. Callers receive the
// returned bool as "is the game over?" and should stop turn/phase
// progression when true.
func (gs *GameState) CheckEnd() bool {
	if gs == nil {
		return false
	}
	// §704.3 / §104.4a — apply pending player-loss SBAs BEFORE evaluating
	// game-over, so simultaneous losses resolve as a draw rather than
	// crowning whoever happened to be SBA'd last. Judge sweep round 2
	// (seed 99 game 225): an SBA pass marked the last opponent Lost via
	// §704.5a, the same in-flight trigger cascade then drained the
	// surviving seat to life=0, and the next CheckEnd caller ran no fresh
	// SBA pass — declaring a §104.2a "winner" at 0 life where the CR
	// outcome is a §104.3b/§104.4a draw. Only the leaf player-loss SBAs
	// run here (5a life, 5b empty-draw, 5c poison) — a full
	// StateBasedActions call would recurse back into CheckEnd. Skipped
	// once ended=1 so repeated post-end calls can't retroactively change
	// a recorded outcome.
	if gs.Flags == nil || gs.Flags["ended"] != 1 {
		sba704_5a(gs)
		sba704_5b(gs)
		sba704_5c(gs)
	}
	// §800.4a — run leave-the-game cleanup for newly-Lost seats. Order
	// matches Python: eliminate first, THEN evaluate end conditions.
	for _, s := range gs.Seats {
		if s != nil && s.Lost && !s.LeftGame {
			HandleSeatElimination(gs, s.Idx)
		}
	}
	// §104.2c — "you win the game" effects, gated by §104.3a (a seat
	// also marked Lost in the same SBA window loses; the win doesn't
	// apply). Scan first so alt-win effects (Lab Maniac, Approach,
	// Maze's End) that didn't mark every opponent Lost still end the
	// game promptly. Multiple simultaneous Wons → draw per §104.3b.
	// endRule, winnerIdx, and drawReason capture the result for the
	// shared cleanup-and-event block at the bottom of the function;
	// winnerIdx = -1 signals a draw.
	endRule, winnerIdx, drawReason := "", -1, ""
	winners := make([]int, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if s != nil && s.Won && !s.Lost {
			winners = append(winners, i)
		}
	}
	if len(winners) == 1 {
		endRule, winnerIdx = "104.2c", winners[0]
	} else if len(winners) >= 2 {
		endRule, drawReason = "104.3b", "simultaneous_win_effects_draw"
	}
	alive := gs.LivingSeats()
	if endRule == "" && len(alive) > 1 {
		return false
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		return true
	}
	gs.Flags["ended"] = 1
	// Reconcile mana pools — combat-phase game ends skip the normal
	// end-of-turn drain, leaving typed/legacy pools potentially out of sync.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Mana != nil {
			s.ManaPool = s.Mana.Total()
		}
	}
	// r60 fuzz residual: ZoneCastGrants registered with
	// `until_end_of_turn` (heist / Narset-exile / Cruelclaw) survive past
	// their declared expiry when the game ends in mid-combat — the turn
	// loop returns before EOT cleanup runs, leaving stale grants that the
	// ZoneCastGrantExpiry invariant subsequently fires on. Flush the map
	// here at game-end; the grants can no longer be exercised legally.
	if len(gs.ZoneCastGrants) > 0 {
		for card, p := range gs.ZoneCastGrants {
			emitGrantExpired(gs, card, p, "game_end")
			delete(gs.ZoneCastGrants, card)
		}
	}
	// Determine the §104.2a / §104.3b outcome only if §104.2c didn't
	// already claim the end-rule above.
	if endRule == "" {
		if len(alive) == 1 {
			endRule, winnerIdx = "104.2a", alive[0]
		} else {
			endRule, drawReason = "104.3b", "simultaneous_elimination_draw"
		}
	}
	if winnerIdx >= 0 {
		gs.Flags["winner"] = winnerIdx
		gs.Seats[winnerIdx].Won = true
		reason := "last_seat_standing"
		if endRule == "104.2c" {
			reason = "you_win_the_game_effect"
		}
		gs.LogEvent(Event{
			Kind: "game_end", Seat: winnerIdx, Target: -1,
			Details: map[string]interface{}{
				"rule":   endRule,
				"winner": winnerIdx,
				"reason": reason,
			},
		})
	} else {
		details := map[string]interface{}{
			"rule":   endRule,
			"winner": -1,
			"reason": drawReason,
		}
		if endRule == "104.3b" && len(winners) >= 2 {
			details["winners"] = winners
		}
		gs.LogEvent(Event{
			Kind: "game_end", Seat: -1, Target: -1,
			Details: details,
		})
	}
	// Phase E — game-end orphan sweep mirrors the per-turn cleanup-step
	// sweep (see phases.go ScanExpiredDurations / ending+cleanup branch).
	// When the game ends MID-TURN (combat-phase lethal damage, alt-win
	// effect, mandatory-loop draw), TakeTurn returns before reaching the
	// §514.2 cleanup step — so the per-turn sweep doesn't fire. The
	// post-game invariant check (loki cmd line 937) would then see all
	// the turn's accumulated orphans. One final sweep here closes that
	// window. Idempotent: if cleanup already ran this turn, this call
	// is a no-op.
	SweepOrphanedInstanceIDs(gs)
	// r63 seat-outcome self-checker: final consistency snapshot at game
	// end (exactly-one-winner, no zombie outcomes, can't-win gates).
	gs.SeatOutcome.CheckConsistency(gs, "game_end")
	return true
}

// -----------------------------------------------------------------------------
// HandleSeatElimination — CR §800.4a cleanup
// -----------------------------------------------------------------------------

// HandleSeatElimination applies the §800.4a "when a player leaves the
// game" procedure for the seat at seatIdx. Idempotent via Seat.LeftGame.
//
// Steps (matching Python _handle_seat_elimination):
//
//  1. Remove from every battlefield each permanent OWNED by this seat
//     (CR §108.3 + §800.4a: "all objects owned by that player leave the
//     game"). Also remove permanents CONTROLLED by this seat that an
//     opponent happens to own — the control-effect ends (§800.4a second
//     clause) and without an active control effect the object returns
//     to its owner; our MVP simply exiles it by removing from play.
//     Unregister §614 replacement + §613 continuous effects keyed to
//     each removed permanent.
//  2. Purge stack items whose Controller == seat (§800.4a: "objects on
//     the stack not represented by cards ... cease to exist"; for
//     card-represented spells we drop them outright as a conservative
//     MVP — the rule says they're exiled, but we don't need the card
//     back in a zone for the simulator to proceed).
//  3. Drop §613 ContinuousEffects whose ControllerSeat == seat. Control-
//     change effects that gave this seat control of OTHER permanents
//     end now; the "restore to owner" clause is handled by step 1's
//     owner-based sweep.
//  4. Drop §614 Replacements whose ControllerSeat == seat.
//  5. Emit seat_eliminated event.
//
// §800.4b ("objects would enter under a left player's control don't")
// and §800.4e ("combat damage to a left player isn't assigned") are
// enforced at the decision sites (combat target pick + ResolveEffect
// controller checks), not here.
func HandleSeatElimination(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.LeftGame {
		return
	}
	seat.LeftGame = true

	// r63 seat-outcome self-checker: snapshot every seat's owned-card
	// census so VerifyEliminationCleanup below can prove no OTHER seat
	// went cards-light through this §800.4 sweep (nil-safe when off).
	gs.SeatOutcome.BeginElimination(gs, seatIdx)

	// Stamp the 1-based elimination sequence (r62). HandleSeatElimination
	// runs exactly once per seat (LeftGame guard above) and CheckEnd
	// invokes it in elimination order, so "count of seats already
	// stamped + 1" is the seat's position in the order. Same-CheckEnd
	// simultaneous eliminations tie-break by seat index (the §800.4a
	// loop order). Unlike the seat_eliminated event below, this survives
	// EventPolicy=EventLogNone — heimdall.ClassifyKill keys the winner's kill
	// method off the max-LostOrder opponent.
	if seat.LostOrder == 0 {
		order := 1
		for _, other := range gs.Seats {
			if other != nil && other != seat && other.LostOrder > 0 {
				order++
			}
		}
		seat.LostOrder = order
	}

	// CR §106.4 — eliminated players hold no mana.
	if seat.Mana != nil {
		seat.Mana.Clear()
	}
	seat.ManaPool = 0

	// Track real (non-token) cards that leave the game so the
	// ZoneConservation invariant can adjust its baseline. Per CR §800.4a,
	// objects owned by the leaving player cease to exist — they are NOT
	// placed into any zone.
	realCardsLeaving := 0

	// Step 0 (r63 §800.4a, shared return-to-owner operation): control
	// effects of/over the leaving seat END before anything is removed —
	// every permanent the leaver CONTROLS but does not own (Card.Owner
	// authority) reverts to its owner's battlefield. Only permanents
	// that cannot revert (owner also gone) remain for the exile arm of
	// the sweep below, per §800.4a's "still controlled" clause. This
	// supersedes the PR-#1046 exile-always MVP with the rules-faithful
	// revert; #1046's guarantee (no card vanishes) is preserved either
	// way and pinned by the SeatOutcome checker's census.
	RevertControlForLeavingSeat(gs, seatIdx)

	// Step 1: Walk EVERY seat's battlefield (the leaving player may
	// still own cards an opponent now controls — Gilded Drake trade).
	// Collect removed permanents so we can run detachAll after all
	// battlefields are rewritten — auras/equipment on a SURVIVING
	// seat's battlefield must not retain AttachedTo pointers to a
	// token/permanent that just ceased to exist (Loki r41/r59
	// AttachmentConsistency cluster: tokens owned by a leaving seat
	// disappear, Gorgon's Head / Hero's Blade / Dowsing Dagger left
	// pointing at the dead token).
	removed := 0
	var detached []*Permanent
	for _, other := range gs.Seats {
		if other == nil || len(other.Battlefield) == 0 {
			continue
		}
		kept := other.Battlefield[:0]
		for _, p := range other.Battlefield {
			if p == nil {
				continue
			}
			// r63 seat-outcome checker finding (games 85/89 seed 42):
			// several theft-style per_card handlers stamp
			// Permanent.Owner = controller, corrupting ownership on
			// OTHER players' cards. Card.Owner is set at deck build and
			// never changes (CR §108.3) — it is the authority for every
			// ownership decision in this sweep. With the corrupt
			// perm.Owner, victim-owned cards were swept AND ceased by
			// another seat's elimination (1-6 cards per game vanishing).
			cardOwner := p.Owner
			if p.Card != nil {
				cardOwner = p.Card.Owner
			}
			if p.Controller == seatIdx || cardOwner == seatIdx {
				// Phase 4 census: §800.4a says objects owned by the
				// leaving player cease to exist. Mark this permanent's
				// InstanceID ceased so checkZoneConservation drops it
				// from the expected (Minted - Ceased) set.
				if p.Card != nil && cardOwner == seatIdx {
					MarkInstanceIDCeased(gs, p.Card.InstanceID)
				}
				// Unregister any §614 / §613 hooks tied to this permanent.
				gs.UnregisterReplacementsForPermanent(p)
				gs.UnregisterContinuousEffectsForPermanent(p)
				// r60 extreme-stress / seed 42 game 5480: Kess, Dissident
				// Mage's `while_source_on_bf` graveyard-cast grant on
				// Compound Fracture survived seat 0's elimination because
				// the seat-loss cleanup path (this loop) never called
				// ExpireSourceGrants, even though every other LTB path
				// (DestroyPermanent / ExilePermanent / sacrificePermanentImpl
				// / BouncePermanent / destroyPermSBA / sacrificePermSBA)
				// does. Without this, the grant outlived its source by
				// many turns and tripped ZoneCastGrantExpiry. Same shape
				// as PR #106's LTB plumbing — extending the same hook to
				// the §800.4a seat-elimination path.
				ExpireSourceGrants(gs, p.Timestamp)
				// r60 Loki fresh-main 2026-05-30 ExileLinkageIntegrity
				// cluster — two complementary cleanup paths, both wired
				// here because the §800.4a seat-elimination sweep
				// bypasses the canonical zone_change.go LTB dispatch
				// that the dispatch fix f374f26b uses.
				//
				// ReleaseSourceLinkedExiles handles the Banisher Priest
				// / Hostage Taker / Oblivion Ring family — sources with
				// populated LinkedExile + ExiledByMe slices that need
				// the source-held bookkeeping cleared and each linked
				// card's ExiledByTimestamp reset. Routes nothing back
				// to the battlefield (mid-sweep MoveCard would race);
				// engine-correctness §406.7 return is a deferred-queue
				// TODO in the helper docstring.
				ReleaseSourceLinkedExiles(gs, p)
				// ClearLinkedExileTagsForSource handles the Knowledge
				// Pool / Myr Prototype / River Song's Diary family
				// (stamp ExiledByTimestamp as an exile-discovery key
				// without populating the source-held LinkedExile —
				// missed by ReleaseSourceLinkedExiles by construction).
				// Sweeps every seat's exile zone for stale tag matches.
				ClearLinkedExileTagsForSource(gs, p.Timestamp)
				// r63 production zone-disappearance fix (grinder
				// feynman zone_accounting: seats 5-20 cards LIGHT):
				// a permanent the leaver merely CONTROLLED but another
				// player OWNS does not leave the game — the control
				// effect ends and the object is EXILED (CR §800.4a
				// second clause / §800.4c). The pre-r63 sweep dropped
				// it from the battlefield slice without routing it
				// anywhere: the card object vanished from every zone
				// (the owner's count read light) AND it was counted
				// into the LEAVER's realCardsLeaving. Route the card
				// to its OWNER's exile zone (the doc comment's "MVP
				// simply exiles it" intent, now actually performed).
				// Token permanents cease either way (§704.5d — no card
				// to exile). Zone-change trigger dispatch is skipped
				// deliberately: mid-sweep MoveCard races are the same
				// hazard the LinkedExile comment above documents.
				if p.Card != nil && cardOwner != seatIdx && !p.IsToken() {
					ownerSeat := cardOwner
					if ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
						gs.Seats[ownerSeat].Exile = append(gs.Seats[ownerSeat].Exile, p.Card)
						gs.LogEvent(Event{
							Kind: "zone_change", Seat: ownerSeat, Target: -1,
							Source: p.Card.DisplayName(),
							Details: map[string]interface{}{
								"from_zone": "battlefield",
								"to_zone":   "exile",
								"rule":      "800.4c",
								"reason":    "controller_left_game",
							},
						})
					}
				}
				// Count real cards for zone conservation tracking —
				// ONLY the leaver's own cards actually leave the game.
				if p.Card != nil && !p.IsToken() && cardOwner == seatIdx {
					realCardsLeaving++
				}
				// §800.4a merged-limbo drain (r63 CONSERVATION residual
				// class): a swept MERGED permanent (Mutate §702.140 /
				// Meld §712) carries constituent *Cards in merge limbo
				// — in no zone, censused only through this permanent's
				// MergedCardPtrs. Drain the stack or every constituent
				// is stranded: leaver-owned cards never cease and
				// survivor-owned cards vanish from all zone accounting
				// (the orphan sweep then lossily retires a LIVING
				// player's card).
				realCardsLeaving += drainMergedLimboOnElimination(gs, p, seatIdx)
				detached = append(detached, p)
				removed++
				continue
			}
			kept = append(kept, p)
		}
		other.Battlefield = kept
	}
	for _, p := range detached {
		detachAll(gs, p)
	}

	// Step 1b (r63 CONSERVATION residual class): leaver-owned
	// constituents inside SURVIVING merged permanents. CR §702.140e
	// makes mixed-owner mutate stacks rare, but theft + owner-corruption
	// shapes reach them. The card leaves the game like every other
	// leaver-owned object (§800.4a): cease it and strip it from the
	// merge bookkeeping so a later UnmergeOnLeavePlay can't materialize
	// it into the departed seat's zones — the census skips those, and
	// the resulting minted-not-ceased-absent window is the strict-census
	// "card disappeared" residual.
	for _, other := range gs.Seats {
		if other == nil || other.LeftGame {
			continue
		}
		for _, p := range other.Battlefield {
			if p == nil || p.MergeKind == MergeNone || len(p.MergedCards) == 0 {
				continue
			}
			keptIDs := p.MergedCards[:0]
			for _, mergedID := range p.MergedCards {
				isBase := p.Card != nil && p.Card.InstanceID == mergedID
				c := p.MergedCardPtrs[mergedID]
				if !isBase && c != nil && c.Owner == seatIdx {
					MarkInstanceIDCeased(gs, mergedID)
					delete(p.MergedCardPtrs, mergedID)
					if p.TopCard == c {
						p.TopCard = p.Card
					}
					if !cardIsTokenForElim(c) {
						realCardsLeaving++
					}
					continue
				}
				keptIDs = append(keptIDs, mergedID)
			}
			p.MergedCards = keptIDs
		}
	}

	// Step 2: purge stack items sourced from this seat. §800.4a:
	// abilities cease to exist; spells are exiled. MVP: drop.
	if len(gs.Stack) > 0 {
		purged := 0
		kept := gs.Stack[:0]
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			if item.Controller == seatIdx {
				// Count real cards on the stack that are leaving.
				if item.Card != nil && !cardIsTokenForElim(item.Card) {
					realCardsLeaving++
				}
				// Phase 4 census: purged stack items belong to the
				// leaving seat and cease.
				if item.Card != nil && item.Card.Owner == seatIdx {
					MarkInstanceIDCeased(gs, item.Card.InstanceID)
				}
				purged++
				continue
			}
			kept = append(kept, item)
		}
		gs.Stack = kept
		if purged > 0 {
			gs.LogEvent(Event{
				Kind: "stack_purged_on_leave", Seat: seatIdx, Target: -1,
				Amount: purged,
				Details: map[string]interface{}{
					"rule":   "800.4a",
					"reason": "seat_left_game",
				},
			})
		}
	}

	// Step 2b: purge PENDING triggered abilities sourced from this seat.
	// CR §800.4a: abilities of a leaving player cease to exist. The CR
	// §603.3b trigger batch (`gs.pendingTriggers`) holds abilities that
	// triggered but haven't been pushed onto the stack yet — they need
	// the same cessation treatment. Without this, a Myr-Moonvessel-style
	// "when this dies, add {1} to your mana pool" trigger fired by SBA
	// 704.5g on the same seat the player just lost from triggers, gets
	// batched, the seat eliminates (mana cleared to 0), the batch
	// flushes, the trigger resolves, and add_mana puts the seat back to
	// ManaPool=1 — ResourceConservation invariant fires
	// ("seat 0 is Lost but has ManaPool=1"). Loki r60 extreme-stress /
	// seed 99 game 9804 turn 42.
	if len(gs.pendingTriggers) > 0 {
		purged := 0
		kept := gs.pendingTriggers[:0]
		for _, item := range gs.pendingTriggers {
			if item == nil {
				continue
			}
			if item.Controller == seatIdx {
				purged++
				continue
			}
			kept = append(kept, item)
		}
		gs.pendingTriggers = kept
		if purged > 0 {
			gs.LogEvent(Event{
				Kind: "pending_triggers_purged_on_leave", Seat: seatIdx, Target: -1,
				Amount: purged,
				Details: map[string]interface{}{
					"rule":   "800.4a",
					"reason": "abilities_cease_to_exist",
				},
			})
		}
	}

	// Adjust zone conservation baseline for cards leaving the game.
	// Also count cards remaining in the eliminated seat's private zones
	// (hand, library, graveyard, exile, command zone) that are now "out
	// of the game" per §800.4a. These cards remain in the zone data
	// structures (we don't nil them out) but they belong to a player who
	// has left — they should be excluded from the invariant's expected
	// count OR we leave them as-is (they're still counted). Since we
	// keep them in the data structures, only battlefield + stack removals
	// need adjustment.
	if realCardsLeaving > 0 {
		if gs.Flags != nil {
		}
		// Per-seat tracking so Feynman zone accounting can adjust expected.
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["cards_left_game"] = realCardsLeaving
	}

	// Phase 4 census: §800.4a — every card owned by the leaving seat
	// ceases to exist. Walk all private zones (hand, library, graveyard,
	// exile, command zone) and mark each owned card's InstanceID ceased.
	// The card pointers stay in the slices for forensic clarity (we
	// don't nil them out); the cessation marking is what drops them
	// from the (Minted - Ceased) census expectation.
	for _, c := range seat.Library {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}
	for _, c := range seat.Hand {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}
	for _, c := range seat.Graveyard {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}
	for _, c := range seat.Exile {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}
	for _, c := range seat.CommandZone {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}
	// §800.4a in-flight resolution (r63, seed-7777 game 1937): a spell
	// owned by the leaving seat can be MID-RESOLUTION when the
	// elimination runs — Biorhythm kills its own caster, and the card is
	// popped off the stack but not yet routed (gs.ResolvingCards limbo
	// window), so none of the zone walks above see it. Its
	// post-resolution routing will be refused by the MoveCard LeftGame
	// guard (PR #1041), leaving the card in no zone forever — cease the
	// ID here or the census reads it as "disappeared".
	for _, c := range gs.ResolvingCards {
		if c != nil && c.Owner == seatIdx {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
	}

	// Phase E — §400.7c duplicate-pointer reconciliation. When a card
	// owned by the leaving seat has somehow been duplicated into a
	// surviving seat's zone (an upstream zone-leak bug — Phase F class),
	// the cease above retires the ID while the surviving seat's *Card
	// reference stays put. The post-elimination invariant pass then
	// sees the ID present in a non-LeftGame zone but not in expected
	// (Minted - Ceased) → fabrication false-positive.
	//
	// Fix: walk every NON-LeftGame seat's zones; for any *Card whose
	// InstanceID is now in CeasedInstanceIDs AND was owned by the
	// leaving seat, purge the duplicate reference. This is a structural
	// reconciliation, not a §400.7c repair (the underlying duplication
	// bug remains in whatever code path produced it); the audit event
	// captures the purged shape so Phase F can hunt the source.
	if len(gs.CeasedInstanceIDs) > 0 {
		purgeCount := 0
		purgeFromSlice := func(zone []*Card) []*Card {
			w := 0
			for r := 0; r < len(zone); r++ {
				c := zone[r]
				if c != nil && c.Owner == seatIdx {
					if _, ceased := gs.CeasedInstanceIDs[c.InstanceID]; ceased {
						purgeCount++
						continue
					}
				}
				zone[w] = zone[r]
				w++
			}
			return zone[:w]
		}
		for _, other := range gs.Seats {
			if other == nil || other == seat || other.LeftGame {
				continue
			}
			other.Library = purgeFromSlice(other.Library)
			other.Hand = purgeFromSlice(other.Hand)
			other.Graveyard = purgeFromSlice(other.Graveyard)
			other.Exile = purgeFromSlice(other.Exile)
			other.CommandZone = purgeFromSlice(other.CommandZone)
			other.ForetellExile = purgeFromSlice(other.ForetellExile)
			if other.Companion != nil && other.Companion.Owner == seatIdx {
				if _, ceased := gs.CeasedInstanceIDs[other.Companion.InstanceID]; ceased {
					other.Companion = nil
					purgeCount++
				}
			}
		}
		if purgeCount > 0 {
			gs.LogEvent(Event{
				Kind:   "iid_seat_elim_duplicate_purge",
				Seat:   seatIdx,
				Target: -1,
				Amount: purgeCount,
				Details: map[string]interface{}{
					"rule":   "800.4a_phase_e",
					"reason": "duplicate_owned_card_pointer_in_surviving_seat_zone",
				},
			})
		}
	}

	// Phase E — sideband-zone reconciliation. Cross-game maps
	// (gs.ZoneCastGrants, gs.MadnessExile, gs.PlotExile, gs.MayhemDiscards,
	// gs.ParadigmExile) hold *Card pointers outside the standard six
	// zones. checkZoneConservationByInstanceID walks these maps in its
	// `present` set (invariants.go:238-254); HandleSeatElimination
	// previously skipped them, so a card owned by the leaving seat that
	// lived in one of these maps would have its ID ceased (private-zone
	// loop above caught the *Card-in-private-zone reference) but the
	// sideband map entry survived — fabrication false-positive.
	//
	// Drop any sideband entry whose *Card was owned by the leaving seat.
	// The sideband state is per-card-pointer (not zone semantics); the
	// leaving seat's "may-cast" grants are voided per §800.4a.
	// Sideband cleanup: cease the InstanceID before dropping the map
	// entry, mirroring ParadigmExile's pattern below. Without the cease,
	// any card whose ONLY zone reference is the sideband map (no longer
	// in seat.Exile after a transient drop) leaks as "minted but absent
	// from every zone" once the map entry is deleted. (Loki r60
	// disappearance cluster, 2026-05-30.)
	if len(gs.ZoneCastGrants) > 0 {
		for card := range gs.ZoneCastGrants {
			if card != nil && card.Owner == seatIdx {
				MarkInstanceIDCeased(gs, card.InstanceID)
				delete(gs.ZoneCastGrants, card)
			}
		}
	}
	if len(gs.MadnessExile) > 0 {
		for card := range gs.MadnessExile {
			if card != nil && card.Owner == seatIdx {
				MarkInstanceIDCeased(gs, card.InstanceID)
				delete(gs.MadnessExile, card)
			}
		}
	}
	if len(gs.PlotExile) > 0 {
		for card := range gs.PlotExile {
			if card != nil && card.Owner == seatIdx {
				MarkInstanceIDCeased(gs, card.InstanceID)
				delete(gs.PlotExile, card)
			}
		}
	}
	if len(gs.MayhemDiscards) > 0 {
		for card := range gs.MayhemDiscards {
			if card != nil && card.Owner == seatIdx {
				MarkInstanceIDCeased(gs, card.InstanceID)
				delete(gs.MayhemDiscards, card)
			}
		}
	}
	if cards, ok := gs.ParadigmExile[seatIdx]; ok && len(cards) > 0 {
		// Cease per-card before dropping the seat's bucket.
		for _, c := range cards {
			if c != nil {
				MarkInstanceIDCeased(gs, c.InstanceID)
			}
		}
		delete(gs.ParadigmExile, seatIdx)
	}

	// Step 3: drop §613 continuous effects controlled by this seat
	// (source already cleaned up in step 1; this catches effects whose
	// SourcePerm went nil or whose source was a resolved spell).
	if len(gs.ContinuousEffects) > 0 {
		before := len(gs.ContinuousEffects)
		kept := gs.ContinuousEffects[:0]
		for _, ce := range gs.ContinuousEffects {
			if ce == nil {
				continue
			}
			if ce.ControllerSeat == seatIdx {
				continue
			}
			kept = append(kept, ce)
		}
		gs.ContinuousEffects = kept
		if len(gs.ContinuousEffects) != before {
			gs.InvalidateCharacteristicsCache()
		}
	}

	// Step 4: drop §614 replacements controlled by this seat.
	if len(gs.Replacements) > 0 {
		kept := gs.Replacements[:0]
		for _, re := range gs.Replacements {
			if re == nil {
				continue
			}
			if re.ControllerSeat == seatIdx {
				continue
			}
			kept = append(kept, re)
		}
		gs.Replacements = kept
	}

	// Phase F backstop — orphan sweep AFTER all explicit cleanup steps.
	// Catches any remaining minted-but-absent InstanceID created during
	// the mid-turn window between the last EOT cleanup's
	// SweepOrphanedInstanceIDs and this elimination. Common shapes:
	//   - Tokens that left the battlefield via a non-canonical path
	//     during the seat's pre-elim turn (combat damage cleanup race,
	//     §704.5d window mid-SBA).
	//   - Cards whose only zone reference was a sideband map already
	//     dropped above without ceasing (defense in depth — the
	//     ZoneCastGrants/MadnessExile/PlotExile/MayhemDiscards cleanups
	//     now cease explicitly, but other future sideband zones default
	//     to the right behavior via this sweep).
	//   - Mid-turn zone movements that didn't route through
	//     gs.removePermanent (per_card splice paths in handler edge cases).
	//
	// LeftGame=true is set above (line 391), so SweepOrphanedInstanceIDs
	// skips the eliminated seat's zones — that's the correct semantic per
	// CR §800.4a (their objects cease). Any owned-card *Card still in a
	// surviving seat's zone is found via the surviving-seat walk and
	// stays present (no over-cease). (Loki r60 ZoneConservation
	// disappearance cluster, closed 2026-05-30.)
	SweepOrphanedInstanceIDs(gs)

	// Step 5: emit observation event.
	elimDetails := map[string]interface{}{
		"rule":               "800.4a",
		"permanents_removed": removed,
		"reason":             seat.LossReason,
	}
	// Consolidation step 2: carry the structured loss cause on the
	// elimination event so post-game consumers (analytics inferKiller)
	// can classify without substring-parsing the freeform reason.
	if seat.LossDetail != nil {
		elimDetails["loss_category"] = seat.LossDetail.Category
		if seat.LossDetail.SourceCard != "" {
			elimDetails["loss_source_card"] = seat.LossDetail.SourceCard
		}
	}
	gs.LogEvent(Event{
		Kind: "seat_eliminated", Seat: seatIdx, Target: -1,
		Amount:  removed,
		Details: elimDetails,
	})
	// Fire per-card triggers for seat elimination (e.g. Davros, Dalek Creator).
	FireCardTrigger(gs, "seat_eliminated", map[string]interface{}{
		"eliminated_seat": seatIdx,
		"reason":          seat.LossReason,
	})

	// §800.4h: If the active player leaves the game, advance to the next
	// living player. "If the active player leaves the game during their
	// own turn, the turn continues without an active player until cleanup."
	// MVP: advance to next living seat to avoid TurnStructure invariant
	// violations in downstream checks.
	if gs.Active == seatIdx {
		for i := 1; i < len(gs.Seats); i++ {
			next := (seatIdx + i) % len(gs.Seats)
			if gs.Seats[next] != nil && !gs.Seats[next].Lost {
				gs.Active = next
				break
			}
		}
	}

	// r63 seat-outcome self-checker: prove the §800.4/§104 cleanup
	// actually happened — no leaver objects survive, and no OTHER seat
	// went cards-light (the PR-#1046 stolen-permanent leak shape).
	gs.SeatOutcome.VerifyEliminationCleanup(gs, seatIdx)
}

// cardIsTokenForElim checks if a card is a token for elimination purposes.
// Mirrors cardIsTokenForInv in invariants.go.
func cardIsTokenForElim(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "token" {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Partner legality — CR §702.124 / §903.3c
// -----------------------------------------------------------------------------

// PartnerInfo summarises the partner-relevant AST keywords on a card.
// Populated by ReadPartnerInfo from a CardAST. Empty/zero-valued when
// the card has no partner ability.
type PartnerInfo struct {
	Partner          bool   // bare "Partner" keyword (CR §702.124a)
	FriendsForever   bool   // "Friends forever" (functionally partner)
	ChooseBackground bool   // "Choose a Background" commander
	IsBackground     bool   // type-line includes "Background"
	PartnerWith      string // "Partner with X" — names the required pair (CR §702.124g)
	DoctorsCompanion bool   // "Doctor's companion" (pairs with a Doctor)
	IsDoctor         bool   // type-line includes "Time Lord ... Doctor"
}

// ReadPartnerInfo walks a card's AST + Types slice and extracts which
// partner-family keyword(s) it carries. Safe on nil card or missing AST.
//
// The parser (scripts/parser.py) emits bare "partner" keywords and
// "partner with X" as Keyword nodes; we match on Name+Raw case-
// insensitively. Doctor's Companion / Friends Forever / Choose a
// Background land as Keyword nodes with raw text matching the tail of
// the keyword pattern in parser.py line ~1437.
//
// Type-line membership (Background subtype, Doctor subtype) is read
// from Card.Types; the deckparser populates that from Scryfall's
// type_line via parseTypes, lowercased.
func ReadPartnerInfo(card *Card) PartnerInfo {
	info := PartnerInfo{}
	if card == nil {
		return info
	}
	for _, t := range card.Types {
		low := strings.ToLower(t)
		switch low {
		case "background":
			info.IsBackground = true
		case "doctor":
			info.IsDoctor = true
		}
	}
	if card.AST == nil {
		return info
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok || kw == nil {
			continue
		}
		rawLow := strings.TrimSpace(kw.Raw)
		nameLow := strings.ToLower(strings.TrimSpace(kw.Name))
		switch {
		case rawLow == "partner" || (nameLow == "partner" && rawLow == ""):
			info.Partner = true
		case strings.HasPrefix(rawLow, "partner with "):
			// Keep everything after "partner with " up to comma / paren /
			// full stop. Names like "Partner with Kydele, Chosen of Kruphix"
			// are kept wholesale minus any reminder-text tail.
			name := strings.TrimSpace(kw.Raw[len("partner with "):])
			if idx := strings.IndexAny(name, ".("); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			// Comma-terminated only if the comma isn't part of the card name
			// itself. Real cards like "Partner with Kydele, Chosen of
			// Kruphix" rely on full string comparison against the paired
			// commander's name so we keep the comma in. We only strip
			// reminder text after a sentence break above.
			info.PartnerWith = name
		case rawLow == "friends forever":
			info.FriendsForever = true
		case rawLow == "choose a background":
			info.ChooseBackground = true
		case rawLow == "doctor's companion" ||
			(strings.HasPrefix(rawLow, "doctor") && strings.Contains(rawLow, "companion")):
			info.DoctorsCompanion = true
		}
	}
	return info
}

// ValidatePartnerPair checks CR §702.124 / §903.3c partner legality
// for a commander slice. Returns nil if legal, a *CastError describing
// the violation otherwise.
//
// Valid configurations:
//  1. Both cards have bare Partner keyword (§702.124a).
//  2. "Partner with X" + the named X on each side (§702.124g — specific pair).
//  3. Both Friends Forever (functionally identical to Partner).
//  4. "Choose a Background" commander + Background-typed card.
//  5. A Doctor + Doctor's Companion.
//
// Mixing keywords across categories (Partner + Friends Forever, etc.)
// is ILLEGAL per CR — each keyword pairs only with its own kind.
//
// Single-commander decks pass (len(cards) == 1). Empty decks fail.
// More than 2 commanders always fails — no format allows triple
// commanders.
func ValidatePartnerPair(cards []*Card) error {
	if len(cards) == 0 {
		return &CastError{Reason: "no_commander"}
	}
	if len(cards) == 1 {
		return nil // single commander — trivially legal
	}
	if len(cards) > 2 {
		return &CastError{Reason: "too_many_commanders"}
	}
	a, b := cards[0], cards[1]
	if a == nil || b == nil {
		return &CastError{Reason: "nil_commander"}
	}
	ia := ReadPartnerInfo(a)
	ib := ReadPartnerInfo(b)
	aName := a.DisplayName()
	bName := b.DisplayName()

	// Case 1: both bare Partner.
	if ia.Partner && ib.Partner {
		return nil
	}
	// Case 2: "Partner with X" — names must cross-match.
	if ia.PartnerWith != "" && partnerNameMatch(ia.PartnerWith, bName) {
		if ib.PartnerWith == "" || partnerNameMatch(ib.PartnerWith, aName) {
			return nil
		}
	}
	if ib.PartnerWith != "" && partnerNameMatch(ib.PartnerWith, aName) {
		if ia.PartnerWith == "" || partnerNameMatch(ia.PartnerWith, bName) {
			return nil
		}
	}
	// Case 3: Friends Forever pair.
	if ia.FriendsForever && ib.FriendsForever {
		return nil
	}
	// Case 4: Choose-a-Background commander + Background card.
	if (ia.ChooseBackground && ib.IsBackground) ||
		(ib.ChooseBackground && ia.IsBackground) {
		return nil
	}
	// Case 5: Doctor + Doctor's Companion.
	if (ia.IsDoctor && ib.DoctorsCompanion) ||
		(ib.IsDoctor && ia.DoctorsCompanion) {
		return nil
	}
	return &CastError{Reason: "invalid_partner_pair"}
}

// partnerNameMatch normalises a "Partner with X" reference and a
// candidate commander display name for comparison. Case-fold + trim.
func partnerNameMatch(partnerWith, candidate string) bool {
	return strings.EqualFold(strings.TrimSpace(partnerWith),
		strings.TrimSpace(candidate))
}
