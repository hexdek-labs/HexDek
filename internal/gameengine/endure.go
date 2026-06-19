package gameengine

// endure.go — Endure N (CR §701.63; Tarkir: Dragonstorm keyword action).
//
// "[Source] endures N" gives the source's controller a choice, per the
// printed reminder text:
//
//	(Put N +1/+1 counters on it or create an N/N white Spirit creature token.)
//
// Note the token branch is ONE N/N Spirit — not N separate 1/1 Spirits.
//
// Both branches route through the canonical doubler chokepoints so global
// replacement effects apply uniformly and payoffs see the events:
//
//   - counters → PutCountersTriggered: the §616 would_put_counter chain
//     (Doubling Season / Hardened Scales / Branching Evolution / …) plus the
//     counter_placed payoff trigger.
//   - token    → CreateDoubledTokens: the would_create_token chain (Parallel
//     Lives / Anointed Procession / Mondrak / …); each mint goes through
//     CreateCreatureToken, which fires token_created and the token's ETB.
//
// Auto-choice (simulation): take the counters while the source is still a
// real on-battlefield permanent — endure is the creature growing itself, and
// the counters preserve the existing body's other counters/auras. If the
// source has already left the battlefield by the time endure resolves, the
// counter branch has no legal recipient (it would no-op), so fall back to the
// N/N Spirit token, which is always legal. This makes the "endure on a
// creature that already left" edge case resolve to a real object rather than
// fizzling silently.
//
// Returns the permanent that received the counters, or the created token, or
// nil on an invalid seat / count.
func Endure(gs *GameState, perm *Permanent, n int) *Permanent {
	if gs == nil || perm == nil || n <= 0 {
		return nil
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}

	// Counter branch — only when the source is still on the battlefield to
	// receive the counters (a creature that has left can't).
	if permanentOnBattlefield(gs, perm) {
		PutCountersTriggered(gs, perm, "+1/+1", n, perm)
		gs.LogEvent(Event{
			Kind:   "endure",
			Seat:   seat,
			Source: sourceName(perm),
			Amount: n,
			Details: map[string]interface{}{
				"choice": "counters",
				"rule":   "701.63",
			},
		})
		return perm
	}

	// Token branch — one N/N white Spirit creature token through the
	// would_create_token doubler chain.
	var token *Permanent
	CreateDoubledTokens(gs, seat, 1, perm, func() *Permanent {
		t := CreateCreatureToken(gs, seat, "Spirit",
			[]string{"creature", "spirit", "pip:W"}, n, n)
		if t != nil && t.Card != nil {
			t.Card.Colors = []string{"W"}
			if token == nil {
				token = t
			}
		}
		return t
	})
	gs.LogEvent(Event{
		Kind:   "endure",
		Seat:   seat,
		Source: sourceName(perm),
		Amount: n,
		Details: map[string]interface{}{
			"choice": "token",
			"rule":   "701.63",
		},
	})
	return token
}
