package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrzaLordHighArtificer wires up Urza, Lord High Artificer.
//
// Oracle text:
//
//	When Urza, Lord High Artificer enters the battlefield, create a
//	0/0 colorless Construct artifact creature token with "This
//	creature gets +1/+1 for each artifact you control."
//	Tap an untapped artifact you control: Add {U}.
//	{5}: Exile the top card of your library. You may play that card
//	this turn.
//
// cEDH commander (Kinnan/Urza/Thrasios meta). The {5} ability is
// incidental compared to the tap-artifact-for-{U} ability, which
// turns Urza into a mana dork engine when paired with artifact-heavy
// decks (Mox Diamond + Grim Monolith + Basalt Monolith + Winter Orb
// etc.).
//
// Batch #2 scope:
//   - OnETB: create the Construct token (0/0 creature, artifact). We
//     don't implement the "gets +1/+1 per artifact you control" static
//     ability (layer system integration — Phase 8). Log partial.
//   - OnActivated(0, ...): tap an artifact (ctx["target_perm"]) to add
//     {U}. Adds 1 to the generic pool as an MVP.
//   - OnActivated(1, ...): exile top of library; register a
//     ZoneCastPermission (free cast from exile) with end-of-turn cleanup.
func registerUrzaLordHighArtificer(r *Registry) {
	r.OnETB("Urza, Lord High Artificer", urzaETB)
	r.OnActivated("Urza, Lord High Artificer", urzaActivate)
}

func urzaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urza_construct_token"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Create the 0/0 Construct artifact creature token. Base P/T 0/0 —
	// its static "+1/+1 per artifact" is deferred; caller reads
	// permanents it controls' artifact count + the engine's Phase 8
	// layers will fold it in when that lands.
	token := &gameengine.Card{
		Name:          "Construct Token",
		Owner:         seat,
		BasePower:     0,
		BaseToughness: 0,
		Types:         []string{"token", "artifact", "creature", "construct"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Construct Token",
			"power":  0,
			"tough":  0,
			"reason": "urza_etb",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"construct_plus_one_per_artifact_static_not_in_layers_until_phase_8")
}

func urzaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	switch abilityIdx {
	case 0:
		// Tap an artifact you control: add {U}. Caller provides the
		// artifact via ctx["target_perm"]. We enforce:
		//   - target belongs to the activator,
		//   - target is untapped,
		//   - target is an artifact.
		const slug = "urza_tap_artifact_for_u"
		v, _ := ctx["target_perm"].(*gameengine.Permanent)
		if v == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_target_perm_in_ctx", nil)
			return
		}
		if v.Controller != seat {
			emitFail(gs, slug, src.Card.DisplayName(), "target_not_controlled", nil)
			return
		}
		if !v.IsArtifact() {
			emitFail(gs, slug, src.Card.DisplayName(), "target_not_artifact", nil)
			return
		}
		if v.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "target_already_tapped", nil)
			return
		}
		v.Tapped = true
		s.ManaPool++
		gameengine.SyncManaAfterAdd(s, 1)
		gs.LogEvent(gameengine.Event{
			Kind:   "add_mana",
			Seat:   seat,
			Target: seat,
			Source: src.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"reason":      "urza_tap_artifact",
				"tapped_card": v.Card.DisplayName(),
				"pool":        "U",
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         seat,
			"tapped_card":  v.Card.DisplayName(),
			"new_mana_pool": s.ManaPool,
		})
	case 1:
		// {5}: exile top of library. "You may play that card this turn."
		// ManaCost 0 = free cast (the {5} activation cost covers it).
		const slug = "urza_exile_top_may_play"
		if len(s.Library) == 0 {
			emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
			return
		}
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "exile", "impulse-draw")

		// Register zone-cast permission so the exiled card can be played.
		gameengine.RegisterZoneCastGrant(gs, c, &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneExile,
			Keyword:           "urza_exile_play",
			ManaCost:          0, // free — Urza's {5} pays the cost
			RequireController: seat,
			SourceName:        "Urza, Lord High Artificer",
		})

		// Clean up permission at end of turn.
		cardRef := c
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_turn",
			ControllerSeat: seat,
			SourceCardName: "Urza, Lord High Artificer",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				gameengine.RemoveZoneCastGrant(gs, cardRef)
			},
		})

		gs.LogEvent(gameengine.Event{
			Kind:   "exile",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":   c.DisplayName(),
				"reason": "urza_exile_top_may_play",
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":              seat,
			"exiled_card":       c.DisplayName(),
			"zone_cast_granted": true,
		})
	}
}
