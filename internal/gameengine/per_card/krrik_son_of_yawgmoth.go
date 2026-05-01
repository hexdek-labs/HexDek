package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKrrikSonOfYawgmoth wires K'rrik, Son of Yawgmoth.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	({B/P} can be paid with either {B} or 2 life.)
//	Lifelink
//	For each {B} in a cost, you may pay 2 life rather than pay that mana.
//	Whenever you cast a black spell, put a +1/+1 counter on K'rrik.
//
// Mana cost: {4}{B/P}{B/P}{B/P}.
//
// Implementation:
//   - OnETB: stamp the lifelink keyword flag (kw:lifelink) on K'rrik so
//     combat damage is mirrored as lifegain even when the card AST does
//     not carry an explicit lifelink keyword (test fixtures, synthetic
//     creatures). The keyword stack already covers AST-loaded copies.
//   - OnTrigger("spell_cast"): when K'rrik's controller casts any black
//     spell other than K'rrik himself, add a +1/+1 counter to K'rrik.
//   - Life-for-mana on {B} pips: handled coarsely via a cost reduction in
//     cost_modifiers.go (one generic pip per black pip on the card, with
//     a matching life payment recorded). We emit a per-card "partial"
//     event noting that the substitution is approximated rather than
//     fully integrated into the typed mana pool — the engine pays the
//     reduced mana cost and we deduct life as a side effect.
func registerKrrikSonOfYawgmoth(r *Registry) {
	r.OnETB("K'rrik, Son of Yawgmoth", krrikSonOfYawgmothETB)
	r.OnTrigger("K'rrik, Son of Yawgmoth", "spell_cast", krrikBlackSpellCast)
}

func krrikSonOfYawgmothETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "krrik_son_of_yawgmoth_static"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:lifelink"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"life-for-{B} substitution is approximated via cost_modifiers; typed mana pool integration pending")
}

func krrikBlackSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "krrik_son_of_yawgmoth_growth"
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
	// "Whenever you cast a black spell" — K'rrik himself is a black
	// spell, but his trigger reads the battlefield and he isn't there
	// until he resolves; fireTrigger filters that case for us.
	if !gameengine.CardHasColor(card, "B") {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
		"counters":   perm.Counters["+1/+1"],
	})
}
