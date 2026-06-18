package gameengine

// keywords_plot.go — Plot (CR §702.172, Outlaws of Thunder Junction).
//
// CR §702.172a: Plot is an activated ability that functions in any zone
//               where the player could activate it. "Plot [cost]" means
//               "[cost], Exile this card from your hand: This card
//               becomes plotted. Activate only as a sorcery."
// CR §702.172b: A spell with plot can be cast from exile by its owner
//               on a later turn, as a sorcery, by paying {0} rather
//               than its mana cost.
//
// Engine model
// ------------
// Plot is a two-step alt-cost mechanic spread across turns:
//
//   Turn N (ActivatePlot, sorcery speed)
//     - Pay the plot cost from the card's owner's mana pool.
//     - Remove the card from the owner's hand.
//     - Place the card in the owner's exile zone.
//     - Stamp gs.PlotExile[card] = &PlotMeta{Seat, Turn=N, ExiledAt=N}.
//     - Register a ZoneCastPermission keyed on the card pointer with
//       Zone=exile, Keyword="plot", ManaCost=0, RequireController=owner,
//       GrantTurn=N, Duration="" (permanent until cast).
//
//   Turn M > N (CastPlot, sorcery speed)
//     - Verify the per-card PlotMeta exists, gs.Turn > meta.Turn, the
//       seat casting is the meta-owner, and a ZoneCastPermission with
//       Keyword="plot" is present on the card.
//     - Remove from exile, no mana cost (cost 0).
//     - Push a StackItem with CostMeta{"plot_cast": true,
//       "zone_cast_keyword": "plot"} and CastZone=ZoneExile.
//     - RemoveZoneCastGrant + clear gs.PlotExile[card]; the plot
//       eligibility is single-use.
//
// The "activate only as a sorcery" restriction is enforced via
// isSorceryTiming on BOTH legs (the activation in turn N and the cast
// from exile in turn M); CR §702.172a's sorcery-speed clause applies
// to the activation, and §702.172b's "as a sorcery" applies to the
// cast itself.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// PlotMeta — per-card plot bookkeeping
// ---------------------------------------------------------------------------

// PlotMeta is the per-card metadata recorded when Plot is activated.
// Lives in gs.PlotExile keyed by the plotted *Card pointer.
//
// Fields:
//   - Seat:     the seat that activated plot (also the card's owner —
//               per §702.172a plot is activated from hand, which only
//               that card's owner can do).
//   - Turn:     gs.Turn at activation time. The "on a later turn" gate
//               in CastPlot enforces gs.Turn > meta.Turn.
//   - ExiledAt: a redundant echo of Turn supplied for handler
//               ergonomics (matches the task's stamp shape
//               "PlotMeta{seat, turn, exiled_at}"). Kept in lockstep
//               with Turn at write time.
type PlotMeta struct {
	Seat     int
	Turn     int
	ExiledAt int
}

// ---------------------------------------------------------------------------
// HasPlot / PlotCost
// ---------------------------------------------------------------------------

// HasPlot reports whether the card has the plot keyword.
func HasPlot(card *Card) bool {
	return cardHasKeywordByName(card, "plot")
}

// PlotCost returns the converted mana cost of the plot keyword's
// activation cost. Accepts the keyword arg as either a mana string
// ("{2}") or a plain numeric value. Returns 0 if the keyword is
// absent or the args are malformed; callers should treat 0 as "free"
// only when they have positively confirmed HasPlot. Mirrors the
// reader pattern used by FlashbackCost / MayhemCost / OmenCost so
// printed mana costs in the keyword arg are parsed correctly rather
// than falling through to the card's printed CMC.
func PlotCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "plot") {
			continue
		}
		if len(kw.Args) == 0 {
			return card.CMC
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// ActivatePlot — sorcery-speed activation from hand
// ---------------------------------------------------------------------------

// ActivatePlot activates the plot ability of a card in `seatIdx`'s
// hand. CR §702.172a.
//
// Preconditions enforced here:
//   - card has the plot keyword
//   - card is in seat's hand
//   - sorcery timing (seat is active, in a main phase, stack empty)
//   - seat can afford `plotCost` mana (pass -1 to use the printed
//     PlotCost)
//
// On success the card is removed from hand, the plot cost is paid,
// the card is appended to seat's exile zone, gs.PlotExile is stamped
// with a fresh PlotMeta, and a ZoneCastPermission is registered so
// CastPlot (or any other zone-cast-aware caller) can pick it up on a
// later turn.
//
// Plot can only be activated on the controller's turn during a main
// phase with an empty stack (CR §702.172a "Activate only as a
// sorcery") — the same gate isSorceryTiming applies elsewhere in the
// engine (level-up activations).
func ActivatePlot(gs *GameState, seatIdx int, card *Card, plotCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasPlot(card) {
		return nil, &CastError{Reason: "no_plot_keyword"}
	}
	if !isSorceryTiming(gs, seatIdx) {
		return nil, &CastError{Reason: "sorcery_speed_only"}
	}
	if plotCost < 0 {
		plotCost = PlotCost(card)
	}
	if plotCost < 0 {
		return nil, &CastError{Reason: "invalid_plot_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	if seat.ManaPool < plotCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	// Drannith Magistrate's restriction is on CASTING from non-hand
	// zones; activating Plot from hand isn't a cast (it's an activated
	// ability of a card in hand — §602.1). No suppression check here.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the plot activation cost.
	seat.ManaPool -= plotCost
	SyncManaAfterSpend(seat)
	if plotCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: plotCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "plot_activate",
				"keyword": "plot",
				"rule":    "702.172a",
			},
		})
	}
	// Move into exile.
	seat.Exile = append(seat.Exile, card)

	// Stamp per-card metadata.
	if gs.PlotExile == nil {
		gs.PlotExile = map[*Card]*PlotMeta{}
	}
	meta := &PlotMeta{
		Seat:     seatIdx,
		Turn:     gs.Turn,
		ExiledAt: gs.Turn,
	}
	gs.PlotExile[card] = meta

	// Register the cast-from-exile permission. ManaCost = 0 because
	// the cast pays {0} per §702.172b (not the printed mana cost, and
	// not a separate "plot" cost — the plot activation pays the cost
	// up front, the cast itself is free).
	RegisterZoneCastGrant(gs, card, &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "plot",
		ManaCost:          0,
		RequireController: seatIdx,
		SourceName:        "plot_exile",
		Duration:          "", // permanent until cast or otherwise removed
		GrantTurn:         gs.Turn,
	})

	gs.LogEvent(Event{
		Kind:   "plot_activate",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: plotCost,
		Details: map[string]interface{}{
			"rule":      "702.172a",
			"exiled_at": gs.Turn,
		},
	})
	return &CostPaymentResult{}, nil
}

// GetPlotMeta returns the PlotMeta for `card` if it is currently
// plotted, or nil otherwise. Safe on nil game / nil card.
func GetPlotMeta(gs *GameState, card *Card) *PlotMeta {
	if gs == nil || card == nil || gs.PlotExile == nil {
		return nil
	}
	return gs.PlotExile[card]
}

// IsPlotCastEligible reports whether `card` has been plotted, the
// caller is its plot-time controller (per §702.172b "its owner"), and
// at least one turn boundary has passed since the plot was activated.
// Does NOT check sorcery timing — callers should pair this with
// isSorceryTiming when honoring CR §702.172b's "as a sorcery" clause.
func IsPlotCastEligible(gs *GameState, seatIdx int, card *Card) bool {
	meta := GetPlotMeta(gs, card)
	if meta == nil {
		return false
	}
	if meta.Seat != seatIdx {
		return false
	}
	if gs.Turn <= meta.Turn {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// CastPlot — sorcery-speed cast from exile for {0}
// ---------------------------------------------------------------------------

// CastPlot casts a previously-plotted card from `seatIdx`'s exile for
// {0} per CR §702.172b. Sorcery speed; cannot be done on the same turn
// the card was plotted.
//
// Preconditions enforced here:
//   - card is in seat's exile zone
//   - gs.PlotExile[card] exists and PlotMeta.Seat == seatIdx
//   - gs.Turn > PlotMeta.Turn (the "on a later turn" gate)
//   - sorcery timing
//   - a ZoneCastPermission with Keyword="plot" is registered for the
//     card (defense-in-depth; ActivatePlot always registers one)
//
// On success the card is removed from exile, pushed onto the stack
// with CostMeta{"plot_cast": true, "zone_cast_keyword": "plot"} and
// CastZone=ZoneExile; the cast-side bookkeeping + "whenever you cast a
// spell" trigger dispatch run through the same helpers as CastFromZone
// (so magecraft / prowess / storm / second-spell payoffs see the plot
// cast); the PlotMeta entry is cleared and the ZoneCastPermission is
// removed (plot is single-use). Resolution is left to the caller.
func CastPlot(gs *GameState, seatIdx int, card *Card) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !isSorceryTiming(gs, seatIdx) {
		return nil, &CastError{Reason: "sorcery_speed_only"}
	}
	meta := GetPlotMeta(gs, card)
	if meta == nil {
		return nil, &CastError{Reason: "not_plotted"}
	}
	if meta.Seat != seatIdx {
		return nil, &CastError{Reason: "wrong_controller"}
	}
	if gs.Turn <= meta.Turn {
		return nil, &CastError{Reason: "same_turn_as_plot"}
	}
	grant := GetZoneCastGrant(gs, card)
	if grant == nil || grant.Keyword != "plot" || grant.Zone != ZoneExile {
		return nil, &CastError{Reason: "no_plot_zone_cast_grant"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// Drannith Magistrate guard mirrors CastFlashback / CastMayhem —
	// opponents can't cast from non-hand zones.
	if drannithRestrictsZoneCast(gs, seatIdx) {
		gs.LogEvent(Event{
			Kind:   "cast_suppressed",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "drannith_magistrate",
				"zone":   ZoneExile,
				"rule":   "601.2a",
			},
		})
		return nil, &CastError{Reason: "drannith_magistrate"}
	}
	if !removeFromZone(seat, card, ZoneExile) {
		return nil, &CastError{Reason: "not_in_exile"}
	}
	// CR §702.172b — the cast pays {0}. No mana decrement.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneExile,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"plot_cast":         true,
			"zone_cast_keyword": "plot",
		},
	}
	PushStackItem(gs, item)

	// Consume the plot eligibility — single-use. Done BEFORE cast-trigger
	// dispatch so a "whenever you cast a spell" trigger that itself grants a
	// cast can't re-enter and reuse this same grant (mirrors CastFromZone's
	// "mark consumed before resolution" ordering).
	delete(gs.PlotExile, card)
	RemoveZoneCastGrant(gs, card)

	// CR §601.2i / §702.172b — the plotted card is now CAST. The prior path
	// hand-rolled the stack push and skipped every cast-side side effect that
	// the canonical CastFromZone pipeline performs, so a plot cast fired no
	// "whenever you cast a spell" triggers and didn't count toward magecraft /
	// prowess / storm / second-spell payoffs. Route the cast-side bookkeeping
	// + trigger dispatch through the same helpers CastFromZone uses. The plot
	// cast pays {0}, so there is no cost-modifier pass (CR §702.172b casts
	// "without paying its mana cost"). Resolution is intentionally left to the
	// caller — the spell sits on the stack — preserving the historical CastPlot
	// contract (CastFromZone resolves inline; CastPlot does not).
	IncrementCastCount(gs, seatIdx)
	RecordCast(gs, seatIdx, card, 0)
	seat.Turn.CastFromExile++
	fireCastTriggersFromZone(gs, seatIdx, card, ZoneExile)
	FireCastTriggerObservers(gs, card, seatIdx, false)
	InvokeCastHook(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_plot_cast_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "plot_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule":      "702.172b",
			"plot_turn": meta.Turn,
		},
	})
	return &CostPaymentResult{}, nil
}

// ---------------------------------------------------------------------------
// Stack / per-turn predicates
// ---------------------------------------------------------------------------

// IsPlotCast reports whether a StackItem was cast via plot.
func IsPlotCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["plot_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellPlotCastThisTurn returns true if any spell was cast via plot
// by `seatIdx` during the current turn.
func SpellPlotCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_plot_cast_this_turn:"+itoa(seatIdx)] > 0
}
