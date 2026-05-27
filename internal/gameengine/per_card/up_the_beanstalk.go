package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUpTheBeanstalk wires Up the Beanstalk.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Up%20the%20Beanstalk):
//
//	When this enchantment enters and whenever you cast a spell with
//	mana value 5 or greater, draw a card.
//
// {1}{G} Enchantment. Standard-format showpiece, now a popular Commander
// value engine (Beanstalk + Boomweaver, Beanstalk + Eldrazi titan loops).
//
// Implementation:
//   - OnETB: draw one card for the controller.
//   - OnTrigger("spell_cast"): if caster_seat == controller AND the cast
//     spell's EffectiveCMC >= 5, draw one card. Uses the "spell_cast"
//     event (fires for every cast, including the controller's own) and
//     gates on caster_seat == controller — same shape as other
//     "whenever you cast" payoffs.
//
// CMC source: the cast spell's *Card via ctx["card"]. Beanstalk reads
// mana value of the SPELL, not whatever permanent it eventually becomes,
// so EffectiveCMC of the cast Card is correct here.
func registerUpTheBeanstalk(r *Registry) {
	r.OnETB("Up the Beanstalk", upTheBeanstalkETB)
	r.OnTrigger("Up the Beanstalk", "spell_cast", upTheBeanstalkOnCast)
}

func upTheBeanstalkETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "up_the_beanstalk_etb"
	if gs == nil || perm == nil {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"source": "etb",
	})
}

func upTheBeanstalkOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "up_the_beanstalk_cast"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return // "whenever YOU cast"
	}
	castCard, _ := ctx["card"].(*gameengine.Card)
	if castCard == nil {
		return
	}
	cmc := castCard.EffectiveCMC()
	if cmc < 5 {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_card":  castCard.DisplayName(),
		"cast_cmc":   cmc,
		"source":     "cast_trigger",
	})
}
