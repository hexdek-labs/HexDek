package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMica wires Mica, Reader of Ruins.
//
// Oracle text:
//
//	Ward—Pay 3 life. (Whenever this creature becomes the target of a
//	spell or ability an opponent controls, counter it unless that
//	player pays 3 life.)
//	Whenever you cast an instant or sorcery spell, you may sacrifice
//	an artifact. If you do, copy that spell and you may choose new
//	targets for the copy.
//
// Implementation:
//   - Ward is an AST keyword and fires through the stock targeting
//     pipeline. No handler work needed.
//   - Listens on "instant_or_sorcery_cast"; gates on caster_seat ==
//     perm.Controller.
//   - AI policy: opt YES on the may-sacrifice clause whenever a
//     low-value artifact is available (token, Treasure, Clue, Food, or
//     CMC <= 1 mana rock). Decline when the only available artifacts
//     are mana rocks of CMC >= 2 — copying one spell rarely outweighs
//     ramp-engine tempo.
//   - Copy mirrors krark.go / resolveCopySpell — deep-copy the
//     StackItem, mark IsCopy, push above the original.
func registerMica(r *Registry) {
	r.OnTrigger("Mica, Reader of Ruins", "instant_or_sorcery_cast", micaSpellCast)
}

func micaSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mica_reader_of_ruins_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	victim := chooseArtifactSacForMica(gs, perm.Controller, perm)
	if victim == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"spell":  card.DisplayName(),
			"copied": false,
			"reason": "no_low_value_artifact",
		})
		return
	}

	// Locate the spell's StackItem before we sacrifice (sacrifice may
	// trigger handlers that reorganize the stack).
	var stackItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card != card {
			continue
		}
		stackItem = si
		break
	}
	if stackItem == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "spell_not_on_stack", map[string]interface{}{
			"spell": card.DisplayName(),
		})
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "mica_reader_of_ruins_cost")

	copyCard := card.DeepCopy()
	copyCard.IsCopy = true
	copyItem := &gameengine.StackItem{
		Controller: perm.Controller,
		Card:       copyCard,
		Effect:     stackItem.Effect,
		Kind:       stackItem.Kind,
		IsCopy:     true,
	}
	if len(stackItem.Targets) > 0 {
		copyItem.Targets = append([]gameengine.Target(nil), stackItem.Targets...)
	}
	gameengine.PushStackItem(gs, copyItem)

	gs.LogEvent(gameengine.Event{
		Kind:   "copy_spell",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":       slug,
			"copied":     card.DisplayName(),
			"sacrificed": victimName,
			"is_copy":    true,
			"rule":       "707.2",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"spell":      card.DisplayName(),
		"sacrificed": victimName,
		"copied":     true,
	})
}

// chooseArtifactSacForMica picks the lowest-value artifact the controller
// can spare. Preference order:
//
//  1. Token artifacts (Treasure, Clue, Food, generic) — pure fodder.
//  2. Non-mana-rock artifacts of CMC 0-1 (e.g. Mox, expended baubles).
//  3. None — refuse the sacrifice rather than crack a real engine piece.
//
// We deliberately skip mana rocks of CMC >= 2 (Sol Ring, Mind Stone,
// Signets, Talismans) since their continuing tempo outweighs one extra
// spell copy in nearly all board states.
func chooseArtifactSacForMica(gs *gameengine.GameState, seat int, src *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}

	// Pass 1: token artifacts.
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsArtifact() || p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
	}

	// Pass 2: low-CMC non-token artifacts (Mox, Lotus Petal, etc.).
	var best *gameengine.Permanent
	bestCMC := 99
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsArtifact() || p.IsLand() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc > 1 {
			continue
		}
		if cmc < bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	return best
}
