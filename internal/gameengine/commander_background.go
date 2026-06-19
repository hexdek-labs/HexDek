package gameengine

import (
	"regexp"
	"strconv"
	"strings"
)

// commander_background.go — Background static ability grants (CR §702.124e).
//
// A Background is a "Legendary Enchantment — Background" that, while on the
// battlefield, grants its printed clause to "commander creatures you own".
// The parser emits the clause as a `commander_anthem` Static whose single arg
// is the RAW TEXT of the grant (e.g. "base power and toughness 10/10 and are
// giants in addition to their other types").
//
// The previous layers.go handler had two bugs:
//   1. Filter — it buffed EVERY creature the Background's controller controlled
//      (`t.Controller == src.Controller`), not just commander creatures the
//      controller OWNS. Raised by Giants made the whole board 10/10-ish.
//   2. Values — it read extractPT(args, 1, 1) against the raw-text arg, so it
//      always applied a flat +1/+1 and dropped the real P/T / base-P/T / type
//      grant entirely.
//
// This file parses the clause for the bounded, unambiguous shapes (additive
// +X/+Y, base-P/T set, "are <type> in addition to their other types") and
// scopes every grant to commander creatures the Background's controller owns.
// Quoted-ability grants ("have \"Whenever …\"") and parameterized keywords
// (Ward—Pay N) are NOT parsed here — see /tmp/fable-review/mechanic-backgrounds-r63.md
// for the broader group_quoted_ability_grant / commander-buff-grant plan.

var (
	reCmdrBasePT  = regexp.MustCompile(`base power and toughness (\d+)/(\d+)`)
	reCmdrBoost   = regexp.MustCompile(`(?:^|have |get )\+(\d+)/\+(\d+)`)
	reCmdrAreType = regexp.MustCompile(`(?:are|become) (\w+?)s? in addition to (?:its|their) other types`)
)

// commanderOwnedCreaturePred selects commander creatures the given seat OWNS
// (CR §702.124e "commander creatures you own" — ownership-based, independent
// of who currently controls the creature). The Background itself is excluded
// downstream because the anthem helpers self-gate on the "creature" type.
func commanderOwnedCreaturePred(ownerSeat int) func(*GameState, *Permanent) bool {
	return func(g *GameState, t *Permanent) bool {
		if t == nil || t.Card == nil || t.Card.Owner != ownerSeat {
			return false
		}
		return IsCommanderCard(g, ownerSeat, t.Card)
	}
}

// applyCommanderAnthemFromText wires the parseable parts of a Background's
// "commander creatures you own …" clause as continuous effects scoped to the
// owner's commander creatures. Unparsed shapes are left alone (no-op) rather
// than mis-applied.
func applyCommanderAnthemFromText(gs *GameState, p *Permanent, text string) {
	if gs == nil || p == nil || p.Card == nil {
		return
	}
	low := strings.ToLower(text)
	pred := commanderOwnedCreaturePred(p.Controller)

	// P/T: a base-P/T SET (layer 7b, "base power and toughness X/Y") and an
	// additive +X/+Y boost (layer 7c) are mutually exclusive on a given card.
	if m := reCmdrBasePT.FindStringSubmatch(low); m != nil {
		bp, _ := strconv.Atoi(m[1])
		bt, _ := strconv.Atoi(m[2])
		registerAnthemSetPT(gs, p, bp, bt, "ast-cmdr-base-pt", pred)
	} else if m := reCmdrBoost.FindStringSubmatch(low); m != nil {
		bp, _ := strconv.Atoi(m[1])
		bt, _ := strconv.Atoi(m[2])
		registerAnthemPT(gs, p, bp, bt, "ast-cmdr-boost", pred)
	}

	// Type addition (layer 4): "are Giants in addition to their other types".
	if m := reCmdrAreType.FindStringSubmatch(low); m != nil {
		registerCommanderGroupTypeAdd(gs, p, m[1], pred)
	}
}

// registerCommanderGroupTypeAdd registers a layer-4 type-add continuous effect
// over a predicate-selected group of creatures (the per-target RegisterAddTypes
// only handles a single fixed permanent).
func registerCommanderGroupTypeAdd(gs *GameState, src *Permanent, typ string,
	pred func(*GameState, *Permanent) bool) {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" {
		return
	}
	applyFn := func(_ *GameState, _ *Permanent, chars *Characteristics) {
		if !charsHaveType(chars.Types, "creature") {
			return
		}
		if !charsHaveType(chars.Types, typ) {
			chars.Types = append(chars.Types, typ)
		}
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:          LayerType,
		Timestamp:      src.Timestamp,
		SourcePerm:     src,
		SourceCardName: src.Card.DisplayName(),
		ControllerSeat: src.Controller,
		HandlerID:      layerHandlerKey("ast-cmdr-type", src),
		Predicate:      pred,
		ApplyFn:        applyFn,
	})
}
