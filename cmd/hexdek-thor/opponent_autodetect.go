package main

// opponent_autodetect.go — automatic adversarial seat setup for Thor.
//
// Cards with oracle text like "whenever an opponent casts a spell" need
// an active opposing seat to fire. Default test setup gives seat 1 a
// passive Bear that never acts, so opponent-conditioned triggers are
// never exercised. This file detects opponent references in a card's
// AST + oracle text and spawns a matching adversarial action (cast,
// attack, lose life, gain life, draw) so the listener fires.
//
// All detection is structural — no card-by-card curation. The result
// is an `opponentNeeds` mask plus an `applyAdversarialSetup` helper
// that materializes the required board changes and fires the events.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// opponentNeeds describes which kinds of adversarial action a card
// references. Each flag drives one block of adversarial setup.
type opponentNeeds struct {
	any       bool // any opponent reference at all (target, control, mention)
	cast      bool // listener cares about an opponent casting a spell
	attack    bool // listener cares about an opponent attacking
	loseLife  bool // listener cares about an opponent losing life
	gainLife  bool // listener cares about an opponent gaining life
	draw      bool // listener cares about an opponent drawing a card
	sacrifice bool // listener cares about an opponent sacrificing
}

// hasAny returns true if any opponent context is needed.
func (n opponentNeeds) hasAny() bool {
	return n.any || n.cast || n.attack || n.loseLife || n.gainLife || n.draw || n.sacrifice
}

// detectOpponentNeeds inspects the card's AST and oracle text and
// returns a mask describing what adversarial setup is required.
//
// The two signals are complementary: oracle text catches phrasings the
// AST didn't fully normalize (parser misses), while the AST walk
// catches Filter.OpponentControls / "each_opponent" actor references
// that don't always say "opponent" verbatim in lowered text.
func detectOpponentNeeds(ast *gameast.CardAST, oracleText string) opponentNeeds {
	var n opponentNeeds
	text := strings.ToLower(oracleText)

	if strings.Contains(text, "opponent") {
		n.any = true
	}

	// Oracle-text signals — phrase-level checks. We require "opponent"
	// nearby to avoid e.g. "you cast" matching the cast branch. Root
	// verbs are used so "casts" / "would cast" / "cast a spell" all hit.
	if strings.Contains(text, "opponent") {
		if oppPhraseNear(text, "cast") {
			n.cast = true
		}
		if oppPhraseNear(text, "attack") {
			n.attack = true
		}
		if oppPhraseNear(text, "lose life") || oppPhraseNear(text, "loses life") ||
			oppPhraseNear(text, "loses 1 life") || oppPhraseNear(text, "loses 2 life") ||
			oppPhraseNear(text, "loses 3 life") || oppPhraseNear(text, "would lose") {
			n.loseLife = true
		}
		if oppPhraseNear(text, "gain life") || oppPhraseNear(text, "gains life") ||
			oppPhraseNear(text, "gains 1 life") || oppPhraseNear(text, "gains 2 life") {
			n.gainLife = true
		}
		if oppPhraseNear(text, "draw") {
			n.draw = true
		}
		if oppPhraseNear(text, "sacrifice") {
			n.sacrifice = true
		}
	}

	if ast != nil {
		for _, ab := range ast.Abilities {
			switch v := ab.(type) {
			case *gameast.Triggered:
				visitTrigger(&n, &v.Trigger)
				visitEffect(&n, v.Effect)
			case *gameast.Activated:
				visitEffect(&n, v.Effect)
			}
		}
	}

	return n
}

// oppPhraseNear returns true when the lowered oracle text contains
// "opponent" within ~32 chars of the supplied phrase. Cheap proxy for
// "the phrase applies to an opponent, not the controller."
func oppPhraseNear(text, phrase string) bool {
	idx := 0
	for {
		hit := strings.Index(text[idx:], phrase)
		if hit < 0 {
			return false
		}
		hit += idx
		lo := hit - 32
		if lo < 0 {
			lo = 0
		}
		hi := hit + len(phrase) + 32
		if hi > len(text) {
			hi = len(text)
		}
		if strings.Contains(text[lo:hi], "opponent") {
			return true
		}
		idx = hit + len(phrase)
	}
}

func visitTrigger(n *opponentNeeds, tr *gameast.Trigger) {
	if tr == nil {
		return
	}
	if tr.Actor != nil {
		visitFilter(n, tr.Actor)
	}
	if tr.TargetFilter != nil {
		visitFilter(n, tr.TargetFilter)
	}
	ev := strings.ToLower(tr.Event)
	switch {
	case strings.Contains(ev, "cast"):
		// Only flag the cast branch when the trigger isn't scoped to
		// "you" / "this player" (i.e. controller's own cast).
		if !strings.Contains(strings.ToLower(tr.Controller), "you") {
			if hasOpponentScope(tr) {
				n.cast = true
				n.any = true
			}
		}
	case strings.Contains(ev, "attack"):
		if hasOpponentScope(tr) {
			n.attack = true
			n.any = true
		}
	case strings.Contains(ev, "life_lost") || strings.Contains(ev, "loses_life") || strings.Contains(ev, "opponent_loses"):
		n.loseLife = true
		n.any = true
	case strings.Contains(ev, "life_gained") || strings.Contains(ev, "gains_life"):
		if hasOpponentScope(tr) {
			n.gainLife = true
			n.any = true
		}
	case strings.Contains(ev, "draw"):
		if hasOpponentScope(tr) {
			n.draw = true
			n.any = true
		}
	}
}

// hasOpponentScope returns true if the trigger's actor or target
// filter references an opponent (Base="opponent" or OpponentControls).
// Triggers without an actor are conservatively treated as opponent-
// scoped when the event itself implies it (caller decides).
func hasOpponentScope(tr *gameast.Trigger) bool {
	if tr == nil {
		return false
	}
	if tr.Actor != nil && filterIsOpponent(tr.Actor) {
		return true
	}
	if tr.TargetFilter != nil && filterIsOpponent(tr.TargetFilter) {
		return true
	}
	// "each player" / no actor → assume opponent in scope (we'll set it up;
	// worst case it's a no-op for the listener's own cast).
	if tr.Actor == nil && tr.TargetFilter == nil {
		return true
	}
	return false
}

func filterIsOpponent(f *gameast.Filter) bool {
	if f == nil {
		return false
	}
	if strings.Contains(strings.ToLower(f.Base), "opponent") {
		return true
	}
	if f.OpponentControls {
		return true
	}
	return false
}

func visitFilter(n *opponentNeeds, f *gameast.Filter) {
	if f == nil {
		return
	}
	if filterIsOpponent(f) {
		n.any = true
	}
}

// visitEffect walks the effect tree and sets need flags for any
// opponent-targeted leaf. Recurses through Sequence / Choice /
// Optional_ / Conditional.
func visitEffect(n *opponentNeeds, e gameast.Effect) {
	if e == nil {
		return
	}
	switch v := e.(type) {
	case *gameast.Sequence:
		for _, it := range v.Items {
			visitEffect(n, it)
		}
	case *gameast.Choice:
		for _, opt := range v.Options {
			visitEffect(n, opt)
		}
	case *gameast.Optional_:
		visitEffect(n, v.Body)
	case *gameast.Conditional:
		visitEffect(n, v.Body)
		visitEffect(n, v.ElseBody)
	case *gameast.Damage:
		visitFilter(n, &v.Target)
	case *gameast.Draw:
		visitFilter(n, &v.Target)
	case *gameast.Discard:
		visitFilter(n, &v.Target)
	case *gameast.Mill:
		visitFilter(n, &v.Target)
	case *gameast.CounterSpell:
		visitFilter(n, &v.Target)
	case *gameast.Destroy:
		visitFilter(n, &v.Target)
	case *gameast.Exile:
		visitFilter(n, &v.Target)
	case *gameast.Bounce:
		visitFilter(n, &v.Target)
	case *gameast.GainLife:
		visitFilter(n, &v.Target)
	case *gameast.LoseLife:
		visitFilter(n, &v.Target)
		// Self-targeted "X loses life" is the controller; opponent-
		// targeted is what we want to mirror back as gainLife? No —
		// the source HERE is making the opp lose life; the listener
		// branch is detected from triggers. Nothing to do.
	case *gameast.SetLife:
		visitFilter(n, &v.Target)
	case *gameast.Sacrifice:
		actor := strings.ToLower(v.Actor)
		if strings.Contains(actor, "opponent") {
			n.any = true
			n.sacrifice = true
		}
		visitFilter(n, &v.Query)
	}
}

// applyAdversarialSetup mutates gs to satisfy the detected needs,
// fires the relevant trigger events from seat 1's perspective, and
// records OPPONENT_ACTION trace entries. Idempotent / no-op when
// needs.hasAny() is false.
//
// Strategy: rather than fully simulate a turn for seat 1 (which
// would require mana, the stack, priority), we fire the trigger
// events directly. That's sufficient to exercise listening per-card
// handlers on seat 0, which is exactly what Thor measures.
func applyAdversarialSetup(gs *gameengine.GameState, oc *oracleCard, needs opponentNeeds, tr *Tracer) {
	if gs == nil || !needs.hasAny() || len(gs.Seats) < 2 {
		return
	}

	tr.Record("OPPONENT_SETUP", "needs cast=%v attack=%v loseLife=%v gainLife=%v draw=%v sac=%v",
		needs.cast, needs.attack, needs.loseLife, needs.gainLife, needs.draw, needs.sacrifice)

	if needs.cast {
		spellCard := &gameengine.Card{
			Name:   "Thor Probe Bolt",
			Owner:  1,
			Types:  []string{"instant"},
			Colors: []string{"R"},
			CMC:    1,
		}
		// Mimic the live cast pipeline (CR §601.2a–i): the spell moves
		// hand → stack, THEN cast-trigger fanout fires. Per-card
		// spell_cast handlers (Possibility Storm, Knowledge Pool) exile
		// the cast spell via MoveCard("stack" → "exile"), which
		// intentionally no-ops on the stack-side removal
		// (removeCardFromZone leaves stack management to the caller).
		// In live play ResolveStackTop later pops the stale StackItem;
		// the scaffold mimics that cleanup explicitly below so
		// CardIdentity doesn't see the probe card in both stack and
		// the handler-chosen destination.
		item := &gameengine.StackItem{
			Kind:       "spell",
			Controller: 1,
			Card:       spellCard,
		}
		gameengine.PushStackItem(gs, item)
		tr.Record("OPPONENT_ACTION", "cast spell %q from seat 1", spellCard.Name)
		gameengine.FireCastTriggers(gs, 1, spellCard)
		// Pop the probe StackItem if still present — handlers that
		// exiled the spell leave it on gs.Stack as a stale reference
		// (the canonical post-resolution pop never runs in scaffold
		// land).
		for i := len(gs.Stack) - 1; i >= 0; i-- {
			if gs.Stack[i] == item {
				gs.Stack = append(gs.Stack[:i], gs.Stack[i+1:]...)
				break
			}
		}
	}

	if needs.attack {
		atkCard := &gameengine.Card{
			Name:          "Thor Probe Attacker",
			Owner:         1,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
		}
		atkPerm := &gameengine.Permanent{
			Card: atkCard, Controller: 1, Owner: 1,
			Flags: map[string]int{"attacking": 1},
		}
		gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, atkPerm)
		tr.Record("OPPONENT_ACTION", "attack with %q from seat 1", atkCard.Name)
		gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
			"attacker_perm": atkPerm,
			"attacker_seat": 1,
			"attacker_card": atkCard,
		})
	}

	if needs.loseLife {
		// Drop seat 1's life by 1 and fire the canonical event.
		gs.Seats[1].Life -= 1
		tr.Record("OPPONENT_ACTION", "seat 1 loses 1 life")
		gameengine.FireCardTrigger(gs, "life_lost", map[string]interface{}{
			"seat":   1,
			"amount": 1,
			"source": "thor_probe",
		})
	}

	if needs.gainLife {
		gameengine.GainLife(gs, 1, 1, "thor_probe")
		tr.Record("OPPONENT_ACTION", "seat 1 gains 1 life")
	}

	if needs.draw {
		// Ensure seat 1 has a card to draw, then move it.
		if len(gs.Seats[1].Library) == 0 {
			gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
				Name: "Thor Probe Draw", Owner: 1, Types: []string{"creature"},
			})
		}
		drawn := gs.Seats[1].Library[0]
		gs.Seats[1].Library = gs.Seats[1].Library[1:]
		gs.Seats[1].Hand = append(gs.Seats[1].Hand, drawn)
		tr.Record("OPPONENT_ACTION", "seat 1 draws %q", drawn.Name)
		gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
			"seat":   1,
			"source": "thor_probe",
		})
	}

	if needs.sacrifice {
		// Sacrifice the first non-source permanent on seat 1's battlefield,
		// or skip if seat 1 has no permanents to sac.
		for i, p := range gs.Seats[1].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			tr.Record("OPPONENT_ACTION", "seat 1 sacrifices %q", p.Card.Name)
			gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield[:i], gs.Seats[1].Battlefield[i+1:]...)
			gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, p.Card)
			gameengine.FireCardTrigger(gs, "permanent_sacrificed", map[string]interface{}{
				"seat":      1,
				"perm_card": p.Card,
			})
			break
		}
	}

	// Run state-based actions once so any cascading effects settle
	// before the test's own assertion phase.
	gameengine.StateBasedActions(gs)
}
