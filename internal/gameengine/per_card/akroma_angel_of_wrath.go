package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAkromaAngelOfWrath wires Akroma, Angel of Wrath.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{5}{W}{W}{W}
//	Legendary Creature — Angel
//	6/6
//	Flying, first strike, vigilance, trample, haste, protection from
//	black and from red
//
// Implementation (R53 batch N port):
//   - ETB stamps the canonical keyword + protection perm flags the
//     combat / target dispatcher reads (same fast-path used by Wraith,
//     Wilson, Thrun, etc.). The AST keyword pipeline applies the same
//     grants in parallel; the runtime stamps are belt-and-suspenders
//     for test seats and analytics scanners that build Cards without
//     fully populated AST trees.
func registerAkromaAngelOfWrath(r *Registry) {
	r.OnETB("Akroma, Angel of Wrath", akromaAngelOfWrathETB)
}

func akromaAngelOfWrathETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "akroma_angel_of_wrath_keyword_stamps"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, perm, "flying")
	gameengine.SetKeywordFlags(gs, perm, "first_strike")
	gameengine.SetKeywordFlags(gs, perm, "vigilance")
	gameengine.SetKeywordFlags(gs, perm, "trample")
	gameengine.SetKeywordFlags(gs, perm, "haste")
	perm.Flags["prot:B"] = 1
	perm.Flags["prot:R"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"keywords": []string{"flying", "first_strike", "vigilance", "trample", "haste"},
		"prot":     []string{"black", "red"},
	})
}
