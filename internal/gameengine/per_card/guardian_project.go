package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGuardianProject wires Guardian Project.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Guardian%20Project):
//
//	Whenever a nontoken creature enters under your control, if it has a
//	different name than each other creature you control and each
//	creature card in your graveyard, draw a card.
//
// {2}{G} Enchantment. Singleton-creature draw engine. Yarok / token-light
// midrange staple — every distinct creature you cast cantrips, but
// re-casts / token spam don't. Stronger in 99 than in 60-card formats
// because Commander's singleton restriction guarantees most casts pass
// the uniqueness check.
//
// Implementation:
//   - OnTrigger("nonland_permanent_etb"). Filter chain matches the
//     oracle clause exactly: (1) entering perm must be a creature,
//     (2) controller_seat == perm.Controller, (3) the entering perm
//     must NOT be a token (CR §111 — tokens are "nontoken creature"
//     restriction exclusion), (4) no OTHER creature on perm.Controller's
//     battlefield shares the entering creature's name, AND (5) no
//     creature card in perm.Controller's graveyard shares it.
//   - Action: draw 1.
//
// The "different name than each other creature you control" includes
// the entering creature itself — so we exclude the entering perm from
// the battlefield-scan loop. The graveyard scan reads CARD names (no
// "other" qualifier — graveyard creatures with the same name as the
// entering creature, even if they're copies, block the trigger).
func registerGuardianProject(r *Registry) {
	r.OnTrigger("Guardian Project", "nonland_permanent_etb", guardianProjectETB)
}

func guardianProjectETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "guardian_project"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if !entering.IsCreature() {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if entering.IsToken() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	enteringName := entering.Card.DisplayName()

	// (4) Battlefield uniqueness — exclude the entering perm itself.
	for _, p := range seat.Battlefield {
		if p == nil || p == entering || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if p.Card.DisplayName() == enteringName {
			emit(gs, slug, "Guardian Project", map[string]interface{}{
				"seat":           perm.Controller,
				"entering":       enteringName,
				"blocked_reason": "battlefield_name_collision",
			})
			return
		}
	}

	// (5) Graveyard uniqueness — scan creature cards only.
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardIsCreature(c) {
			continue
		}
		if c.DisplayName() == enteringName {
			emit(gs, slug, "Guardian Project", map[string]interface{}{
				"seat":           perm.Controller,
				"entering":       enteringName,
				"blocked_reason": "graveyard_name_collision",
			})
			return
		}
	}

	drawOne(gs, perm.Controller, "Guardian Project")

	emit(gs, slug, "Guardian Project", map[string]interface{}{
		"seat":     perm.Controller,
		"entering": enteringName,
	})
}

// cardIsCreature mirrors Permanent.IsCreature() at the Card level —
// inspects Card.Types for a "creature" entry. Used by graveyard scans
// where we have raw *Card pointers, not Permanents.
func cardIsCreature(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "creature" {
			return true
		}
	}
	return false
}
