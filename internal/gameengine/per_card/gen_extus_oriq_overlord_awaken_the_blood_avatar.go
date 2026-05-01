package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerExtusOriqOverlordAwakenTheBloodAvatar wires Extus, Oriq Overlord // Awaken the Blood Avatar.
//
// Oracle text:
//
//   Double strike
//   Magecraft — Whenever you cast or copy an instant or sorcery spell, return target nonlegendary creature card from your graveyard to your hand.
//   As an additional cost to cast this spell, you may sacrifice any number of creatures. This spell costs {2} less to cast for each creature sacrificed this way.
//   Each opponent sacrifices a creature of their choice. Create a 3/6 black and red Avatar creature token with haste and "Whenever this token attacks, it deals 3 damage to each opponent."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerExtusOriqOverlordAwakenTheBloodAvatar(r *Registry) {
	r.OnETB("Extus, Oriq Overlord // Awaken the Blood Avatar", extusOriqOverlordAwakenTheBloodAvatarStaticETB)
}

func extusOriqOverlordAwakenTheBloodAvatarStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "extus_oriq_overlord_awaken_the_blood_avatar_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
