package gameengine

import "strings"

// -----------------------------------------------------------------------------
// Incarnation graveyard statics (Judgment cycle) — r63
// -----------------------------------------------------------------------------
//
// The "incarnation" cycle grants a keyword to the creatures you control while
// the incarnation card sits IN YOUR GRAVEYARD and you control a land of the
// matching basic type:
//
//	Anger  — "As long as Anger is in your graveyard and you control a
//	          Mountain, creatures you control have haste."
//	Brawn  — trample, as long as you control a Forest.
//	Filth  — swampwalk, as long as you control a Swamp.
//	Wonder — flying, as long as you control an Island.
//	Valor  — first strike, as long as you control a Plains.
//
// This is a from-GRAVEYARD static — a code path the battlefield-sourced §613
// continuous-effect registry (which keys every effect to a SourcePerm and
// unregisters it on LTB) cannot express, so the grant is applied as a direct
// layer-6 pass during characteristics computation instead. Because the pass
// runs on every (cache-missed) GetEffectiveCharacteristics, the grant is
// re-derived from live state each time:
//
//	(a) it FUNCTIONS from the graveyard zone (this pass, layer 6 / §613.1f);
//	(b) the land condition is evaluated DYNAMICALLY (live DefenderControlsLandType);
//	(c) the keyword reaches only YOUR creatures (gated on perm.Controller ==
//	    the graveyard's owner) and flows through chars.Keywords, which the
//	    combat keyword-query (keywordActive / HasKeywordOf) consumes;
//	(d) it stops the instant the card leaves the graveyard or the land
//	    condition fails (the next recompute simply won't find it);
//	(e) it never applies from any other zone — only gs.Seats[seat].Graveyard
//	    is scanned, so a copy in hand / library / exile / on the battlefield
//	    grants nothing.
//
// Glory and Genesis are deliberately EXCLUDED: their graveyard abilities are
// ACTIVATED ({cost}, exile/discard …), not static keyword grants.

type incarnationStatic struct {
	keyword  string
	landType string // lowercase basic land subtype
}

// incarnationGraveyardStatics maps incarnation card name → its graveyard grant.
var incarnationGraveyardStatics = map[string]incarnationStatic{
	"Anger":  {keyword: "haste", landType: "mountain"},
	"Brawn":  {keyword: "trample", landType: "forest"},
	"Filth":  {keyword: "swampwalk", landType: "swamp"},
	"Wonder": {keyword: "flying", landType: "island"},
	"Valor":  {keyword: "first strike", landType: "plains"},
}

// applyIncarnationGraveyardGrants is the layer-6 pass that confers incarnation
// keywords onto perm when an incarnation rests in perm's controller's
// graveyard and the land condition holds. Called from GetEffectiveCharacteristics
// immediately after the registered layer-6 (LayerAbility) effects, so a later
// layer (e.g. type-loss) has already settled chars.Types.
func applyIncarnationGraveyardGrants(gs *GameState, perm *Permanent, chars *Characteristics) {
	if gs == nil || perm == nil || chars == nil {
		return
	}
	// Only creatures receive these grants ("creatures you control have …").
	if !charsHaveType(chars.Types, "creature") {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		inc, ok := incarnationGraveyardStatics[c.DisplayName()]
		if !ok {
			continue
		}
		// §613 dynamic condition: the controller must control the matching
		// basic land type RIGHT NOW. DefenderControlsLandType is seat-agnostic
		// despite the name (it checks the given seat's battlefield).
		if !DefenderControlsLandType(gs, seat, inc.landType) {
			continue
		}
		addKeywordIfAbsent(chars, inc.keyword)
	}
}

// addKeywordIfAbsent appends kw to chars.Keywords unless already present
// (case-insensitive). Keeps the grant idempotent across multiple incarnations
// or repeated passes.
func addKeywordIfAbsent(chars *Characteristics, kw string) {
	if chars == nil || kw == "" {
		return
	}
	want := strings.ToLower(strings.TrimSpace(kw))
	for _, k := range chars.Keywords {
		if strings.ToLower(strings.TrimSpace(k)) == want {
			return
		}
	}
	chars.Keywords = append(chars.Keywords, kw)
}
