package gameengine

// Granted miracle (CR §702.94) — a continuous effect that grants the
// miracle ability to cards in a player's hand, e.g. a permanent reading
// "Cards in your hand have miracle {0}."
//
// THE CORRECTNESS GATE (this whole file exists to enforce it):
// granting miracle to the whole hand must NOT let a player cast their
// entire hand for free. §702.94a still applies — a card may be cast for
// its miracle cost only if it's the FIRST card you've drawn this turn and
// you reveal it as you draw it. So the grant registry is consulted ONLY
// from MaybeOpenMiracleWindow, which the draw chokepoint calls on the
// first draw of the turn alone. Cards already in hand were never "drawn
// this turn" and never open a miracle window — no free hand-dump. The
// grant changes only the COST of the one card that legitimately opens a
// window each turn (e.g. to {0}), never the gate itself.

// MiracleGrant is a registered "cards in your hand have miracle {Cost}"
// effect. SourcePerm/HandlerID/SourceTimestamp mirror ZoneCastPolicy for
// lifecycle parity; the per_card handler that registers a grant is
// responsible for unregistering it at LTB (UnregisterMiracleGrantsForPermanent).
type MiracleGrant struct {
	SourcePerm      *Permanent
	HandlerID       string
	ControllerSeat  int              // the player whose hand cards gain miracle
	Cost            int              // granted miracle cost (generic-mana MVP; 0 = free)
	Predicate       func(*Card) bool // nil = applies to every card in hand
	SourceTimestamp int
}

// RegisterMiracleGrant appends a grant to the registry.
func (gs *GameState) RegisterMiracleGrant(g *MiracleGrant) {
	if gs == nil || g == nil {
		return
	}
	gs.MiracleGrants = append(gs.MiracleGrants, g)
	gs.LogEvent(Event{
		Kind:   "miracle_grant_registered",
		Seat:   g.ControllerSeat,
		Source: g.HandlerID,
		Amount: g.Cost,
		Details: map[string]interface{}{
			"rule": "702.94",
			"cost": g.Cost,
		},
	})
}

// UnregisterMiracleGrantsForPermanent drops every grant whose SourcePerm
// == p. Called from per_card LTB hooks so a bounced / exiled / destroyed
// source stops granting miracle (mirrors UnregisterZoneCastPoliciesForPermanent).
func (gs *GameState) UnregisterMiracleGrantsForPermanent(p *Permanent) {
	if gs == nil || p == nil || len(gs.MiracleGrants) == 0 {
		return
	}
	out := gs.MiracleGrants[:0]
	for _, g := range gs.MiracleGrants {
		if g != nil && g.SourcePerm == p {
			gs.LogEvent(Event{
				Kind:    "miracle_grant_expired",
				Seat:    g.ControllerSeat,
				Source:  g.HandlerID,
				Details: map[string]interface{}{"reason": "source_ltb"},
			})
			continue
		}
		out = append(out, g)
	}
	for i := len(out); i < len(gs.MiracleGrants); i++ {
		gs.MiracleGrants[i] = nil
	}
	gs.MiracleGrants = out
}

// grantedMiracleCost returns the lowest granted miracle cost that applies
// to `card` for `seat`, and whether any grant applies. A grant whose
// SourcePerm is non-nil and no longer on the battlefield is ignored
// (defensive — the canonical lifecycle is the per_card LTB unregister, but
// this also covers handlers that forget to unregister).
func grantedMiracleCost(gs *GameState, seat int, card *Card) (int, bool) {
	if gs == nil || card == nil {
		return 0, false
	}
	best := -1
	for _, g := range gs.MiracleGrants {
		if g == nil || g.ControllerSeat != seat {
			continue
		}
		if g.SourcePerm != nil && !permOnBattlefield(gs, g.SourcePerm) {
			continue
		}
		if g.Predicate != nil && !g.Predicate(card) {
			continue
		}
		if best < 0 || g.Cost < best {
			best = g.Cost
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}

// effectiveMiracleCost returns the miracle cost for `card` in `seat`'s
// hand and whether miracle applies at all (native printed miracle OR a
// registered grant). Native miracle uses its printed cost; otherwise a
// grant supplies the cost. Used at draw time by MaybeOpenMiracleWindow.
func effectiveMiracleCost(gs *GameState, seat int, card *Card) (int, bool) {
	if HasMiracle(card) {
		return MiracleCost(card), true
	}
	return grantedMiracleCost(gs, seat, card)
}

// permOnBattlefield reports whether p is currently on any seat's battlefield.
func permOnBattlefield(gs *GameState, p *Permanent) bool {
	if gs == nil || p == nil {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, bp := range s.Battlefield {
			if bp == p {
				return true
			}
		}
	}
	return false
}
