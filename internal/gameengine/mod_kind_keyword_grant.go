package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_kind_keyword_grant.go — generic handler for the inert `keyword_grant`
// scaffold KIND (63 static occurrences):
//
//	"[Other] creatures you control have <keyword>." (Aggressive Mammoth →
//	trample, Rage Reflection → double strike, Vela the Night-Clad →
//	intimidate, Smellerbee → haste, …)
//
// A keyword anthem (a "lord" granting an evergreen keyword to your
// creatures). The parser emits keyword_grant with Layer=null, so
// registerASTStaticEffects skipped it at its Layer!="" gate and
// resolve_helpers.go merely logged it inert — the keyword was never
// conferred. Now that combat + targeting honor layer-6 keyword grants
// (the hex-dev-keywords-10 p.HasKeyword∪gs.HasKeywordOf fix), this
// confers a real, gameplay-affecting keyword.
//
// Args layout: [keyword, (scope?)]. scope == "other" excludes the source
// itself; otherwise all creatures the source's controller controls.
// Reuses the attachment-buff evergreen whitelist so only keywords the
// engine actually honors are granted (no garbage strings).
//
// Registered from RegisterContinuousEffectsForPermanent (layers.go),
// alongside registerAttachmentBuffs / registerAuraGrants, independently of
// the Layer!="" gate.

func registerKeywordGrantStatics(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.Card == nil || p.Card.AST == nil {
		return
	}
	src := p
	for i, ab := range p.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "keyword_grant" {
			continue
		}
		args := st.Modification.Args
		if len(args) == 0 {
			continue
		}
		kw, ok := args[0].(string)
		if !ok {
			continue
		}
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" || !attachmentGrantableKeywords[kw] {
			continue // skip non-evergreen / unrecognized keywords
		}
		other := false
		if len(args) >= 2 {
			if s, ok := args[1].(string); ok && s == "other" {
				other = true
			}
		}
		disc := itoaLayers(i)
		pred := func(_ *GameState, t *Permanent) bool {
			if t == nil || t.Controller != src.Controller {
				return false
			}
			if other && t == src {
				return false
			}
			return true
		}
		registerKeywordGrant(gs, p, kw, "ast-kw-anthem-"+disc+"-"+kw, pred)
	}
}
