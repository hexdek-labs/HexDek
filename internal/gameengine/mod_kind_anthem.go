package gameengine

// Generic "anthem" modification kind — mass power/toughness modification
// (CR §613 P/T layer for statics; one-shot until-EOT pumps for spells).
//
//	"Creatures [you control | your opponents control | all creatures] get
//	 +X/+Y [until end of turn]."
//
// AST args = [pow, tough, ...scope/duration tokens]. Scope tokens:
//
//	"opponents" — creatures your opponents control
//	"all"       — all creatures
//	"other"     — creatures you control except the source (static only)
//	(default)   — creatures you control
//	"until_eot" — temporary (spell pump); absent on a static ability the
//	              effect is continuous.
//
// Before this handler the `anthem` kind fell through the inert default in
// both dispatch sites (193 cards corpus-wide silently did nothing):
//   - resolveModificationEffect (resolve_helpers.go) — SPELL / triggered
//     effects → applyAnthemSpellEffect (one-shot, until end of turn).
//   - registerASTStaticEffects (layers.go) — a permanent's STATIC ability →
//     registerGenericAnthemStatic (continuous §613 layer effect).
//
// The two paths never double-apply: a static ability is registered as a
// continuous effect; a spell/triggered effect is resolved once.

// anthemHasToken reports whether the anthem arg list contains the scope/
// duration token tok (e.g. "opponents", "all", "other", "until_eot").
func anthemHasToken(args []interface{}, tok string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && s == tok {
			return true
		}
	}
	return false
}

// anthemPredicate builds the creature filter for an anthem given the source's
// controller and (for the "other" scope) the source permanent. srcPerm may be
// nil on the spell path — "other" then degrades to "your creatures", which is
// harmless because spells don't use the "other" scope.
func anthemPredicate(srcController int, srcPerm *Permanent, args []interface{}) func(*GameState, *Permanent) bool {
	switch {
	case anthemHasToken(args, "all"):
		return func(_ *GameState, _ *Permanent) bool { return true }
	case anthemHasToken(args, "opponents"):
		return func(_ *GameState, t *Permanent) bool { return t.Controller != srcController }
	case anthemHasToken(args, "other"):
		return func(_ *GameState, t *Permanent) bool {
			return t.Controller == srcController && t != srcPerm
		}
	default:
		return func(_ *GameState, t *Permanent) bool { return t.Controller == srcController }
	}
}

// applyAnthemSpellEffect applies a one-shot (until end of turn) P/T change to
// every creature matching the anthem's scope. Used for spell- and trigger-
// resolved anthems. A spell pump always lasts until end of turn (CR §611.2c),
// whether or not the parser captured an explicit "until_eot" token.
func applyAnthemSpellEffect(gs *GameState, src *Permanent, args []interface{}) {
	if gs == nil {
		return
	}
	seat := controllerSeat(src)
	if seat < 0 {
		return
	}
	pow, tough := extractPT(args, 1, 1)
	pred := anthemPredicate(seat, src, args)
	affected := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if !pred(gs, p) {
				continue
			}
			p.Modifications = append(p.Modifications, Modification{
				Power:     pow,
				Toughness: tough,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			affected++
		}
	}
	if affected > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	gs.LogEvent(Event{
		Kind:   "anthem",
		Seat:   seat,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"power":     pow,
			"toughness": tough,
			"affected":  affected,
			"duration":  "until_end_of_turn",
		},
	})
}

// registerGenericAnthemStatic registers a continuous §613 P/T anthem for a
// permanent's static ability ("creatures you control get +X/+Y").
func registerGenericAnthemStatic(gs *GameState, p *Permanent, args []interface{}) {
	if gs == nil || p == nil || p.Card == nil {
		return
	}
	pow, tough := extractPT(args, 1, 1)
	pred := anthemPredicate(p.Controller, p, args)
	registerAnthemPT(gs, p, pow, tough, "ast-generic-anthem", pred)
}
