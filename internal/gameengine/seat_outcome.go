package gameengine

import (
	"fmt"
	"github.com/hexdek/hexdek/internal/judge"
	"strings"
)

// seat_outcome.go — per-seat win/loss self-checker (phase 1; owner
// design from 7174n1c, r63).
//
// A flag-gated ride-along checker (default OFF — gs.SeatOutcome is nil
// and every hook is a nil-receiver no-op, the legality-validator
// pattern) that, for each seat INDEPENDENTLY, recomputes the seat's
// rules-defined outcome and cross-checks the engine's bookkeeping:
//
//  1. EvaluateSeatOutcome — the seat's rules-correct status from CR
//     first principles: life ≤ 0 (§104.3a/§704.5a), 10+ poison
//     (§704.5c), 21+ commander damage from a single commander
//     (§704.5v/§903.10a), gated by registered can't-lose replacements
//     (Platinum Angel / Gideon emblem class — probed via the registry's
//     Applies predicates, the read-only 614.1a pattern). §704.5b
//     empty-draw loss is consumed transiently at SBA time and cannot be
//     re-derived at check time; alternate WIN conditions (Lab Man /
//     Thassa's Oracle / Felidar / Approach…) are engine-marked events
//     this phase verifies for CONSISTENCY (exactly-one-winner,
//     can't-win gates) rather than re-deriving — see the phase-2 note
//     in /tmp/fable-review/seat-self-checker-r63.md.
//
//  2. Cross-seat consistency — a seat whose computed status is "lost"
//     must carry the engine Lost flag (divergence = the loss SBA
//     missed); a Lost seat must not control live stack objects; at game
//     end exactly one seat has Won and every other seat has Lost; a
//     seat must not be Won while an opponent's can't-win replacement
//     applies to it.
//
//  3. Leave-game cleanup verification (the centerpiece): around every
//     HandleSeatElimination, snapshot each seat's owned-card census and
//     verify afterwards that (a) no permanent owned OR controlled by
//     the leaver remains on any battlefield, (b) no stack item or
//     continuous effect of the leaver survives, and (c) NO OTHER SEAT
//     ended up cards-light — the exact shape of the stolen-permanent
//     vanish fixed in PR #1046/#1047, which this check would have
//     caught the moment it shipped.

// SeatOutcomeViolation is one structured divergence finding.
type SeatOutcomeViolation struct {
	Seat   int
	Kind   string // e.g. "loss_not_marked", "owned_census_dropped"
	Detail string
	Turn   int
	When   string // "sba" / "elimination" / "game_end"
}

func (v SeatOutcomeViolation) String() string {
	return fmt.Sprintf("[seat-outcome %s] turn=%d seat=%d %s: %s",
		v.When, v.Turn, v.Seat, v.Kind, v.Detail)
}

// Canonical maps the finding onto the canonical vocabulary
// (consolidation step 4) for the unified LogViolation router.
func (v SeatOutcomeViolation) Canonical() judge.ValidationViolation {
	return judge.ValidationViolation{
		Surface:   judge.SurfaceSeatOutcome,
		Dimension: judge.DimensionStateIntegrity,
		Name:      v.Kind,
		Severity:  judge.SeverityCritical,
		Message:   v.Detail,
		Seat:      v.Seat,
		Context: map[string]interface{}{
			"turn": v.Turn,
			"when": v.When,
		},
	}
}

// SeatOutcomeChecker rides along a game when attached to
// gs.SeatOutcome. All methods are nil-receiver-safe no-ops when off.
type SeatOutcomeChecker struct {
	Violations    []SeatOutcomeViolation
	MaxViolations int

	// preElim holds the per-seat owned-card census snapshotted by
	// BeginElimination, consumed by VerifyEliminationCleanup.
	preElim     []int
	preElimSeat int
	// preElimIDs maps InstanceID -> "Name(owner=N,zone=Z)" for every
	// identified card present at BeginElimination, so a census drop can
	// name the exact vanished cards.
	preElimIDs map[string]string
}

// NewSeatOutcomeChecker builds an enabled checker.
func NewSeatOutcomeChecker() *SeatOutcomeChecker {
	return &SeatOutcomeChecker{MaxViolations: 2000, preElimSeat: -1}
}

func (c *SeatOutcomeChecker) add(gs *GameState, v SeatOutcomeViolation) {
	if c == nil {
		return
	}
	v.Turn = gs.Turn
	if len(c.Violations) < c.MaxViolations {
		c.Violations = append(c.Violations, v)
	}
	// Consolidation step 4: route through the unified sink at origin.
	judge.LogViolation(v.Canonical())
}

// ---------------------------------------------------------------------------
// Part 1 — rules-correct outcome evaluation
// ---------------------------------------------------------------------------

// SeatHasCantLoseEffect reports whether a registered would_lose_game
// replacement currently applies to the seat (Platinum Angel, Gideon of
// the Trials emblem, Lich's Mastery…). Probes the registry's Applies
// predicates read-only — ApplyFn is never invoked (the 614.1a pattern).
func SeatHasCantLoseEffect(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	probe := NewReplEvent("would_lose_game")
	probe.TargetSeat = seatIdx
	for _, re := range gs.Replacements {
		if re == nil || re.Applies == nil || re.EventType != "would_lose_game" {
			continue
		}
		applies := false
		func() {
			defer func() { _ = recover() }()
			applies = re.Applies(gs, probe)
		}()
		if applies {
			return true
		}
	}
	return false
}

// SeatHasCantWinEffect reports whether a registered would_win_game
// replacement currently applies to the seat (opponent's Platinum Angel
// "your opponents can't win" clause).
func SeatHasCantWinEffect(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	probe := NewReplEvent("would_win_game")
	probe.TargetSeat = seatIdx
	for _, re := range gs.Replacements {
		if re == nil || re.Applies == nil || re.EventType != "would_win_game" {
			continue
		}
		applies := false
		func() {
			defer func() { _ = recover() }()
			applies = re.Applies(gs, probe)
		}()
		if applies {
			return true
		}
	}
	return false
}

// EvaluateSeatOutcome recomputes the seat's rules-defined status from
// first principles. Returns ("alive"|"lost"|"won", reason). The "won"
// arm trusts the engine flag — alternate-win re-derivation is phase 2;
// the win-side cross-checks live in CheckConsistency.
func EvaluateSeatOutcome(gs *GameState, seatIdx int) (string, string) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return "alive", "no_seat"
	}
	s := gs.Seats[seatIdx]
	if s.Won {
		return "won", "engine_flag"
	}

	lossReason := ""
	if s.Life <= 0 {
		lossReason = fmt.Sprintf("life %d <= 0 (CR 104.3a/704.5a)", s.Life)
	}
	if lossReason == "" && s.PoisonCounters >= 10 {
		lossReason = fmt.Sprintf("%d poison counters (CR 704.5c)", s.PoisonCounters)
	}
	if lossReason == "" && s.CommanderDamage != nil {
		for dealer, byName := range s.CommanderDamage {
			for name, dmg := range byName {
				if dmg >= 21 {
					lossReason = fmt.Sprintf("%d commander damage from %q (seat %d) (CR 704.5v/903.10a)", dmg, name, dealer)
					break
				}
			}
			if lossReason != "" {
				break
			}
		}
	}
	if lossReason == "" {
		return "alive", ""
	}
	if SeatHasCantLoseEffect(gs, seatIdx) {
		return "alive", "loss_prevented: " + lossReason
	}
	return "lost", lossReason
}

// ---------------------------------------------------------------------------
// Part 2 — cross-seat consistency
// ---------------------------------------------------------------------------

// CheckConsistency cross-checks every seat's computed outcome against
// the engine flags. `when` labels the call site ("sba" / "game_end").
func (c *SeatOutcomeChecker) CheckConsistency(gs *GameState, when string) {
	if c == nil || gs == nil {
		return
	}
	ended := gs.Flags != nil && gs.Flags["ended"] == 1

	winners := 0
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		status, reason := EvaluateSeatOutcome(gs, i)

		// A seat whose computed status is "lost" must be engine-Lost.
		// (markSeatLost may legitimately lag inside a single SBA pass;
		// callers invoke this at SBA END, after the loss sweeps ran.)
		if status == "lost" && !s.Lost {
			c.add(gs, SeatOutcomeViolation{
				Seat: i, Kind: "loss_not_marked", When: when,
				Detail: fmt.Sprintf("computed LOST (%s) but engine Lost=false", reason),
			})
		}
		// Vice versa, narrowly: an engine-Lost seat whose recorded loss
		// reason is a PERSISTENT condition must still satisfy it.
		// Transient reasons (empty-draw, concession, elimination by
		// effect) are not re-derivable and are skipped.
		if s.Lost && status == "alive" && reason == "" && persistentLossReason(s.LossReason) {
			c.add(gs, SeatOutcomeViolation{
				Seat: i, Kind: "loss_reason_stale", When: when,
				Detail: fmt.Sprintf("engine Lost (reason %q) but no loss condition holds and no record of a transient loss", s.LossReason),
			})
		}
		if s.Won {
			winners++
			if SeatHasCantWinEffect(gs, i) {
				c.add(gs, SeatOutcomeViolation{
					Seat: i, Kind: "won_despite_cant_win", When: when,
					Detail: "engine Won=true while a registered can't-win replacement applies to this seat",
				})
			}
		}
		// A Lost seat must not control live stack objects.
		if s.Lost {
			for _, item := range gs.Stack {
				if item != nil && item.Controller == i {
					name := "?"
					if item.Card != nil {
						name = item.Card.DisplayName()
					}
					c.add(gs, SeatOutcomeViolation{
						Seat: i, Kind: "lost_seat_active", When: when,
						Detail: fmt.Sprintf("Lost seat still controls stack item %q", name),
					})
					break
				}
			}
		}
	}

	if ended {
		// CR §104.4a — a game in which every remaining player loses
		// simultaneously is a legal DRAW (zero winners), not an
		// inconsistency. (Game 395 / seed 3950043: the last two living
		// seats both died to one Howling Banshee "each player loses 3
		// life" ETB.) A decided game has exactly one winner. Only the
		// genuinely inconsistent end shapes are violations:
		//   - 2+ seats Won without the engine collapsing to a §104.3b
		//     draw (the engine resolves multi-win to winner=-1, so a
		//     surviving 2+ here means the draw collapse was skipped);
		//   - 0 winners while a seat is still alive — a premature end or
		//     a seat that should have won was never marked (distinct
		//     from a clean all-players-lost draw, and also surfaced
		//     per-seat by unresolved_at_end below).
		aliveUndecided := 0
		for _, s := range gs.Seats {
			if s != nil && !s.Lost {
				aliveUndecided++
			}
		}
		switch {
		case winners >= 2:
			c.add(gs, SeatOutcomeViolation{
				Seat: -1, Kind: "winner_count", When: when,
				Detail: fmt.Sprintf("game ended with %d winners, want exactly 1 (or a recorded draw)", winners),
			})
		case winners == 0 && aliveUndecided > 0:
			c.add(gs, SeatOutcomeViolation{
				Seat: -1, Kind: "winner_count", When: when,
				Detail: fmt.Sprintf("game ended with 0 winners but %d seat(s) still alive — want a winner or an all-players-lost draw (CR 104.4a)", aliveUndecided),
			})
		}
		for i, s := range gs.Seats {
			if s != nil && !s.Won && !s.Lost {
				c.add(gs, SeatOutcomeViolation{
					Seat: i, Kind: "unresolved_at_end", When: when,
					Detail: "game ended but seat is neither Won nor Lost (zombie outcome)",
				})
			}
		}
	}
}

// persistentLossReason reports whether the recorded loss reason should
// still be re-derivable from state (life / poison / commander damage).
func persistentLossReason(reason string) bool {
	switch {
	case reason == "":
		return false
	}
	low := strings.ToLower(reason)
	for _, marker := range []string{"life total", "poison", "commander damage"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Part 3 — leave-game cleanup verification (the centerpiece)
// ---------------------------------------------------------------------------

// ownedCardCensus counts real (non-token) cards by OWNER across every
// seat's zones — the same owner-reconciled walk the feynman
// zone_accounting heuristic uses, computed on demand.
// presentCardIDs maps InstanceID -> descriptor for every identified
// card currently in any zone (companion to ownedCardCensus, used for
// the identity-level vanish diff).
func presentCardIDs(gs *GameState) map[string]string {
	out := map[string]string{}
	note := func(card *Card, zone string, seat int) {
		if card == nil || card.InstanceID == "" {
			return
		}
		out[card.InstanceID] = fmt.Sprintf("%s(owner=%d,%s@seat%d)", card.DisplayName(), card.Owner, zone, seat)
	}
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			note(c, "hand", si)
		}
		for _, c := range s.Library {
			note(c, "library", si)
		}
		for _, c := range s.Graveyard {
			note(c, "graveyard", si)
		}
		for _, c := range s.Exile {
			note(c, "exile", si)
		}
		for _, c := range s.CommandZone {
			note(c, "command", si)
		}
		for _, p := range s.Battlefield {
			if p == nil || p.IsToken() {
				continue
			}
			note(p.Card, "battlefield", si)
		}
	}
	for _, item := range gs.Stack {
		if item != nil && !item.IsCopy {
			note(item.Card, "stack", -1)
		}
	}
	for _, card := range gs.ResolvingCards {
		note(card, "resolving", -1)
	}
	return out
}

func ownedCardCensus(gs *GameState) []int {
	owned := make([]int, len(gs.Seats))
	bump := func(card *Card) {
		if card == nil {
			return
		}
		o := card.Owner
		if o < 0 || o >= len(owned) {
			o = 0
		}
		owned[o]++
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			bump(c)
		}
		for _, c := range s.Library {
			bump(c)
		}
		for _, c := range s.Graveyard {
			bump(c)
		}
		for _, c := range s.Exile {
			bump(c)
		}
		for _, c := range s.CommandZone {
			bump(c)
		}
		for _, p := range s.Battlefield {
			if p == nil || p.IsToken() {
				continue
			}
			bump(p.Card)
		}
	}
	for _, item := range gs.Stack {
		if item != nil && !item.IsCopy {
			bump(item.Card)
		}
	}
	for _, card := range gs.ResolvingCards {
		bump(card)
	}
	return owned
}

// BeginElimination snapshots the owned-card census before
// HandleSeatElimination runs its §800.4 cleanup.
func (c *SeatOutcomeChecker) BeginElimination(gs *GameState, seatIdx int) {
	if c == nil || gs == nil {
		return
	}
	c.preElim = ownedCardCensus(gs)
	c.preElimIDs = presentCardIDs(gs)
	c.preElimSeat = seatIdx
}

// VerifyEliminationCleanup runs after HandleSeatElimination and
// verifies the CR §800.4 / §104 cleanup actually happened:
//   - no permanent owned or controlled by the leaver on any battlefield;
//   - no stack item controlled by the leaver;
//   - no §613 continuous effect or §614 replacement of the leaver;
//   - NO OTHER seat's owned-card census dropped (the #1046/#1047 leak
//     shape: stolen permanents must revert/exile, never vanish).
func (c *SeatOutcomeChecker) VerifyEliminationCleanup(gs *GameState, seatIdx int) {
	if c == nil || gs == nil {
		return
	}
	for si, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			// Card.Owner is the ownership authority (CR §108.3) — several
			// theft handlers corrupt Permanent.Owner to the thief's seat,
			// so keying this assertion on p.Owner false-positives on
			// permanents that correctly survive the sweep.
			owner := p.Owner
			if p.Card != nil {
				owner = p.Card.Owner
			}
			if owner == seatIdx || p.Controller == seatIdx {
				name := "?"
				if p.Card != nil {
					name = p.Card.DisplayName()
				}
				c.add(gs, SeatOutcomeViolation{
					Seat: seatIdx, Kind: "leaver_permanent_survives", When: "elimination",
					Detail: fmt.Sprintf("permanent %q (cardOwner=%d permOwner=%d controller=%d) still on seat %d's battlefield after §800.4 sweep", name, owner, p.Owner, p.Controller, si),
				})
			}
		}
	}
	for _, item := range gs.Stack {
		if item != nil && item.Controller == seatIdx {
			c.add(gs, SeatOutcomeViolation{
				Seat: seatIdx, Kind: "leaver_stack_item_survives", When: "elimination",
				Detail: "stack item controlled by the leaver survived §800.4a purge",
			})
			break
		}
	}
	for _, ce := range gs.ContinuousEffects {
		if ce != nil && ce.ControllerSeat == seatIdx {
			c.add(gs, SeatOutcomeViolation{
				Seat: seatIdx, Kind: "leaver_continuous_effect_survives", When: "elimination",
				Detail: "§613 continuous effect of the leaver survived elimination",
			})
			break
		}
	}
	for _, re := range gs.Replacements {
		if re != nil && re.ControllerSeat == seatIdx {
			c.add(gs, SeatOutcomeViolation{
				Seat: seatIdx, Kind: "leaver_replacement_survives", When: "elimination",
				Detail: "§614 replacement of the leaver survived elimination",
			})
			break
		}
	}

	// Owned-card conservation for every OTHER seat. The leaver's own
	// count legitimately drops (their battlefield/stack cards leave the
	// game); nobody else's may.
	if c.preElim != nil && c.preElimSeat == seatIdx {
		after := ownedCardCensus(gs)
		for i := range after {
			if i == seatIdx || i >= len(c.preElim) {
				continue
			}
			if after[i] < c.preElim[i] {
				// Identity diff: name the exact vanished cards owned by
				// seat i (skip the leaver's own — those legally cease).
				missing := ""
				if c.preElimIDs != nil {
					afterIDs := presentCardIDs(gs)
					ownerTag := fmt.Sprintf("(owner=%d,", i)
					for id, desc := range c.preElimIDs {
						if _, still := afterIDs[id]; still {
							continue
						}
						// Only the VICTIM seat's cards — the leaver's own
						// ceasings are legal §800.4a departures.
						if !strings.Contains(desc, ownerTag) {
							continue
						}
						if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
							desc += "(CEASED)"
						}
						if missing != "" {
							missing += "; "
						}
						missing += desc
					}
				}
				c.add(gs, SeatOutcomeViolation{
					Seat: i, Kind: "owned_census_dropped", When: "elimination",
					Detail: fmt.Sprintf("seat %d owned %d cards before seat %d's elimination, %d after (lost %d) — §800.4 cleanup vanished another player's cards (the PR-#1046 stolen-permanent leak shape); vanished-unceased: [%s]", i, c.preElim[i], seatIdx, after[i], c.preElim[i]-after[i], missing),
				})
			}
		}
		c.preElim = nil
		c.preElimIDs = nil
		c.preElimSeat = -1
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------
