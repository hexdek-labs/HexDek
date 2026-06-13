package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// mod_kind_aura_grant.go — generic handler for the inert keyword-only
// attachment scaffold KINDS:
//
//	aura_grant  (126) — "Enchanted creature has <keyword>." (Flight,
//	                     Soaring Hope, Robe of Mirrors, Felidar Umbra, …)
//	equip_grant  (67) — "Equipped creature has <keyword>." (Zephyr Boots,
//	                     Quietus Spike, Gorgon's Head, Rover Blades, …)
//
// The parser emits aura_grant with Layer="" (null), so the
// registerASTStaticEffects loop skipped it at its Layer!="" gate and
// resolve_helpers.go's static no-op block merely logged it — the aura
// attached but conferred NO keyword. (Distinct from aura_buff_grant,
// which adds +X/+Y *and* a keyword and is owned by
// scaffold_attachment_buff_r63.go; aura_grant is keyword-only.)
//
// This models the grant as a §613 layer-6 continuous keyword grant on the
// enchanted creature, reusing the same attached-creature predicate and
// keyword whitelist/parser as the attachment-buff handler. Layer-6
// keyword grants are now honored by combat + targeting after the
// hex-dev-keywords-10 fix (p.HasKeyword ∪ gs.HasKeywordOf), so this is no
// longer half-cosmetic.
//
// Conservative parsing: parseAttachmentKeywords keeps only whitelisted
// evergreen keywords and drops "protection from …", inline quoted
// abilities, "loses <kw>" removals, and conditional riders ("vigilance as
// long as …") — those need their own modeling, so we under-grant rather
// than inject garbage.
//
// Registered from RegisterContinuousEffectsForPermanent (layers.go),
// alongside registerAttachmentBuffs, independently of the Layer!="" gate.

func registerAuraGrants(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.Card == nil || p.Card.AST == nil {
		return
	}
	src := p
	attachedPred := func(_ *GameState, t *Permanent) bool {
		return src.AttachedTo != nil && t == src.AttachedTo
	}
	for i, ab := range p.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		// aura_grant (Aura "enchanted creature has <kw>") and equip_grant
		// (Equipment "equipped creature has <kw>") are the same mechanism —
		// a layer-6 keyword grant on the AttachedTo permanent.
		if st.Modification.ModKind != "aura_grant" && st.Modification.ModKind != "equip_grant" {
			continue
		}
		clause := firstStringArg(st.Modification.Args)
		if clause == "" {
			continue
		}
		disc := itoaLayers(i)
		for _, kw := range parseAttachmentKeywords(clause) {
			registerKeywordGrant(gs, p, kw, "ast-attach-grant-"+disc+"-"+kw, attachedPred)
		}
	}
}
