package gameengine

// token_doubler_controller.go — controller-scoped pure TOKEN doubler
// (CR §614.1c / §707-style replacement) for the cards that double only
// tokens (no counter arm): Parallel Lives, Anointed Procession.
//
// "If an effect would create one or more tokens under your control, it
// creates twice that many of those tokens instead."
//
// This mirrors the token arm of RegisterDoublingSeason (replacement.go) but
// without the +1/+1 counter arm. Before this, Parallel Lives / Anointed
// Procession had NO registration at all (no named Register*, no per_card
// handler, and their would_create_tokens AST kind is dispatched nowhere) —
// they were fully inert. The per_card ETB wiring in
// per_card/unwired_doublers_etb.go calls this at ETB.
func RegisterTokenDoublerControllerOnly(gs *GameState, p *Permanent) {
	if p == nil || p.Card == nil {
		return
	}
	name := p.Card.DisplayName()
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_create_token",
		HandlerID:      handlerKey(name, "token_dbl", p),
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind:   "replacement_applied",
				Seat:   p.Controller,
				Source: name,
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"rule":   "614",
					"effect": "token_doubler",
				},
			})
		},
	})
}
