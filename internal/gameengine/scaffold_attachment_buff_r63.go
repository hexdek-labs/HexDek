package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffold_attachment_buff_r63.go — generic handler for the inert
// attachment-buff scaffold KINDS (r63 scaffold-kind coverage):
//
//	aura_buff        (274 cards) — "Enchanted creature gets +X/+Y."
//	aura_buff_grant  (153 cards) — "Enchanted creature gets +X/+Y and has <kw>."
//	equip_buff_grant (130 cards) — "Equipped creature gets +X/+Y and has <kw>."
//
// These are emitted by the parser with Layer="" (null), so the existing
// registerASTStaticEffects loop — which skips any Modification whose
// Layer is empty — never reached them, and resolve_helpers.go's static
// no-op block only logged them. ~557 card-appearances were completely
// inert: auras/equipment attached but conferred no P/T or keywords.
// (The older enchanted_creature_pt case now has ZERO dataset occurrences;
// the parser emits aura_buff in its place.)
//
// We model each as §613 continuous effects on the attached creature,
// reusing the same layer helpers the anthem/lord kinds use:
//   - registerAnthemPT   (layer 7c P/T modify) for the +X/+Y portion.
//   - registerKeywordGrant (layer 6 ability add) for each granted keyword.
// The predicate is dynamic (re-reads AttachedTo each characteristic
// computation), so re-equipping / re-targeting follows automatically and
// UnregisterContinuousEffectsForPermanent cleans up on leave-play.

// attachmentBuffKinds is the set of AST Modification kinds this file owns.
var attachmentBuffKinds = map[string]bool{
	"aura_buff":        true,
	"aura_buff_grant":  true,
	"equip_buff_grant": true,
}

// attachmentGrantableKeywords whitelists the evergreen keywords we grant
// from a _grant clause. Restricted to keywords the engine actually honors
// (combat.go HasKeyword / layers HasKeywordOf) so we never inject inert or
// garbage keyword strings. Protection / ward / inline quoted abilities are
// deliberately skipped — they need their own modeling, not a bare keyword.
var attachmentGrantableKeywords = map[string]bool{
	"flying": true, "trample": true, "deathtouch": true, "lifelink": true,
	"first strike": true, "double strike": true, "vigilance": true,
	"haste": true, "menace": true, "reach": true, "defender": true,
	"hexproof": true, "shroud": true, "indestructible": true, "fear": true,
	"intimidate": true, "skulk": true, "horsemanship": true, "infect": true,
	"toxic": true, "flash": true,
}

// registerAttachmentBuffs scans a permanent's AST for attachment-buff
// static abilities and registers their continuous effects. Called from
// RegisterContinuousEffectsForPermanent at ETB, independently of the
// Layer!="" gate in registerASTStaticEffects (these kinds carry no layer).
func registerAttachmentBuffs(gs *GameState, p *Permanent) {
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
		mod := st.Modification
		if !attachmentBuffKinds[mod.ModKind] {
			continue
		}
		disc := itoaLayers(i)
		// Args layout: [powerDelta, toughnessDelta, (keywordClause?), Filter].
		pow, tough := extractPT(mod.Args, 0, 0)
		if pow != 0 || tough != 0 {
			registerAnthemPT(gs, p, pow, tough, "ast-attach-pt-"+disc, attachedPred)
		}
		// The _grant variants carry a keyword clause as a string arg
		// (Args[2]); plain aura_buff has only ints + the Filter, so no
		// string arg is found and no keyword is granted.
		clause := firstStringArg(mod.Args)
		if clause == "" {
			continue
		}
		for _, kw := range parseAttachmentKeywords(clause) {
			registerKeywordGrant(gs, p, kw, "ast-attach-kw-"+disc+"-"+kw, attachedPred)
		}
	}
}

// firstStringArg returns the first string element of args (the keyword
// clause in _grant kinds), or "" if none. Ints (P/T) and the Filter dict
// are not strings, so this isolates the clause.
func firstStringArg(args []interface{}) string {
	for _, a := range args {
		if s, ok := a.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// parseAttachmentKeywords harvests whitelisted evergreen keywords from a
// grant clause such as "trample and lifelink" or "protection from white
// and from black". Any portion at or after an inline quoted ability (a
// double quote) is dropped, and unrecognized words are ignored.
func parseAttachmentKeywords(clause string) []string {
	clause = strings.ToLower(clause)
	if idx := strings.Index(clause, "\""); idx >= 0 {
		clause = clause[:idx]
	}
	clause = strings.ReplaceAll(clause, " and ", ",")
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(clause, ",") {
		k := strings.TrimSpace(part)
		if attachmentGrantableKeywords[k] && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	return out
}
