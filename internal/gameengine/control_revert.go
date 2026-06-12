package gameengine

// control_revert.go — r63 owner-immutability work (owner design from
// 7174n1c). CR §108.3: a card's OWNER is write-once — whoever started
// the game with it in their deck (a token's owner is its creator,
// §110.5a). Theft effects change CONTROL only, and "return to owner" is
// ONE shared operation used by both until-end-of-turn expiry (§514.2 /
// Act of Treason class) and controller-leaves-game (§800.4a).

// TempControlGrant records an until-end-of-turn control change so the
// cleanup step can revert it. Registered by resolveGainControl when the
// AST GainControl carries Duration "until_end_of_turn" (76 corpus
// nodes: Act of Treason, Captivating Crew, Sauron the Lidless Eye,
// Angrath…). Pre-r63 the duration was silently ignored — every temp
// steal was PERMANENT.
type TempControlGrant struct {
	Perm *Permanent
	// PrevController is recorded for forensics; the revert target is
	// always Card.Owner (the §108.3 authority). With chained control
	// effects the CR-precise target is "whatever other effects say" —
	// MVP approximation documented in the r63 report.
	PrevController int
}

// RevertControlToOwner is THE shared return-to-owner operation: move
// the permanent back under its owner's control (Card.Owner authority —
// several legacy paths corrupted Permanent.Owner, see PR #1047).
// Returns false when the permanent is no longer on any battlefield
// (died/bounced since the grant) or the owner has left the game — the
// §800.4a caller handles the owner-left case by exile instead.
func RevertControlToOwner(gs *GameState, p *Permanent, reason string) bool {
	if gs == nil || p == nil || p.Card == nil {
		return false
	}
	owner := p.Card.Owner
	if owner < 0 || owner >= len(gs.Seats) || gs.Seats[owner] == nil {
		return false
	}
	if gs.Seats[owner].LeftGame || gs.Seats[owner].Lost {
		return false
	}
	if p.Controller == owner {
		return true // already home — idempotent
	}
	if !gs.removePermanent(p) {
		return false // left the battlefield since the grant
	}
	from := p.Controller
	p.Controller = owner
	p.Owner = owner // re-align any corrupted Permanent.Owner while here
	p.Timestamp = gs.NextTimestamp()
	gs.Seats[owner].Battlefield = append(gs.Seats[owner].Battlefield, p)
	gs.LogEvent(Event{
		Kind:   "control_reverted",
		Seat:   owner,
		Target: from,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": reason,
			"rule":   "108.3",
		},
	})
	return true
}

// RegisterTempControlGrant records an until-EOT steal for cleanup-step
// reversal.
func RegisterTempControlGrant(gs *GameState, p *Permanent, prevController int) {
	if gs == nil || p == nil {
		return
	}
	gs.TempControlGrants = append(gs.TempControlGrants, TempControlGrant{
		Perm: p, PrevController: prevController,
	})
}

// ExpireTempControlGrants reverts every registered until-EOT steal.
// Called from the cleanup step (§514.2) via ScanExpiredDurations and
// from game-end cleanup. Grants whose permanent left the battlefield
// are dropped silently (nothing to revert).
func ExpireTempControlGrants(gs *GameState) {
	if gs == nil || len(gs.TempControlGrants) == 0 {
		return
	}
	grants := gs.TempControlGrants
	gs.TempControlGrants = nil
	for _, g := range grants {
		RevertControlToOwner(gs, g.Perm, "until_end_of_turn_expired")
	}
}

// RevertControlForLeavingSeat is the §800.4a arm of the shared
// operation: every permanent the leaving seat CONTROLS but does not own
// (Card.Owner authority) reverts to its owner. Returns the permanents
// that could NOT be reverted (owner also gone) — the caller exiles
// those per §800.4a's "still controlled by the departed player" clause.
func RevertControlForLeavingSeat(gs *GameState, seatIdx int) []*Permanent {
	if gs == nil {
		return nil
	}
	var unrevertable []*Permanent
	// Snapshot: reverts mutate battlefield slices mid-walk.
	var stolen []*Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Controller == seatIdx && p.Card.Owner != seatIdx && !p.IsToken() {
				stolen = append(stolen, p)
			}
		}
	}
	for _, p := range stolen {
		if !RevertControlToOwner(gs, p, "controller_left_game") {
			unrevertable = append(unrevertable, p)
		}
	}
	return unrevertable
}
