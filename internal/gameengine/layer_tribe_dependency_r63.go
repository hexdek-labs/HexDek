package gameengine

import "strings"

// r63 — §613.8 dependency foundation: tribal lords must see layer-4 type
// changes.
//
// A tribal lord ("Elves you control get +1/+1") is a layer-7 P/T effect whose
// applicability depends on a creature's SUBTYPE. CR §613.8's canonical example:
// Conspiracy ("all creatures are Elves", a layer-4 type-changing effect) must
// apply BEFORE the Elvish lord, so the lord pumps creatures that are Elves only
// via Conspiracy. Layer ORDER (4 before 7) already sequences this — but the old
// tribe gate read permanentHasSubtype(t) = the PRINTED TypeLine, never the
// layer-computed type. So Conspiracy-made Elves were not pumped.
//
// Fix: evaluate the subtype against the IN-PROGRESS characteristics (printed OR
// any layer-4 grant already applied). BaseCharacteristics does NOT seed
// chars.Subtypes with printed subtypes, so the check is "printed (TypeLine) OR
// chars.Subtypes" — the union is the effective subtype set at layer 7.

// charsHaveSubtype reports whether the in-progress characteristics carry the
// given subtype (added by a layer-4 type-changer such as Conspiracy / Arcane
// Adaptation). Case-insensitive.
func charsHaveSubtype(chars *Characteristics, sub string) bool {
	if chars == nil || sub == "" {
		return false
	}
	want := strings.ToLower(sub)
	for _, s := range chars.Subtypes {
		if strings.ToLower(s) == want {
			return true
		}
	}
	return false
}

// effectiveHasSubtype is the §613.8-correct subtype test: a permanent has the
// subtype if its PRINTED type line carries it OR a layer-4 effect granted it
// (visible in the in-progress chars at layer 6/7).
func effectiveHasSubtype(perm *Permanent, chars *Characteristics, sub string) bool {
	return permanentHasSubtype(perm, sub) || charsHaveSubtype(chars, sub)
}

// registerTribeAnthemPT registers a layer-7c P/T anthem gated by a creature
// SUBTYPE evaluated against the in-progress characteristics (so layer-4
// type-changers feed it per §613.8). controllerPred carries the non-type gate
// (controller / "other" / global). When exclude is true the anthem applies to
// creatures that do NOT have the subtype ("non-Elf creatures get …").
func registerTribeAnthemPT(gs *GameState, src *Permanent, pow, tough int, disc, tribe string, exclude bool, controllerPred func(*GameState, *Permanent) bool) {
	applyFn := func(_ *GameState, perm *Permanent, chars *Characteristics) {
		if !charsHaveType(chars.Types, "creature") {
			return
		}
		// include: skip when the creature does NOT have the subtype.
		// exclude: skip when it DOES.
		if effectiveHasSubtype(perm, chars, tribe) == exclude {
			return
		}
		chars.Power += pow
		chars.Toughness += tough
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c",
		Timestamp:      src.Timestamp,
		SourcePerm:     src,
		SourceCardName: src.Card.DisplayName(),
		ControllerSeat: src.Controller,
		HandlerID:      layerHandlerKey(disc, src),
		Predicate:      controllerPred,
		ApplyFn:        applyFn,
	})
}

// registerTribeKeywordGrant is the layer-6 keyword-grant analogue of
// registerTribeAnthemPT ("Elves you control have flying"): the subtype gate is
// evaluated against the in-progress chars so a layer-4 type-changer feeds it.
func registerTribeKeywordGrant(gs *GameState, src *Permanent, kw, disc, tribe string, controllerPred func(*GameState, *Permanent) bool) {
	applyFn := func(_ *GameState, perm *Permanent, chars *Characteristics) {
		if !charsHaveType(chars.Types, "creature") {
			return
		}
		if !effectiveHasSubtype(perm, chars, tribe) {
			return
		}
		for _, existing := range chars.Keywords {
			if strings.EqualFold(existing, kw) {
				return
			}
		}
		chars.Keywords = append(chars.Keywords, kw)
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:          LayerAbility,
		Timestamp:      src.Timestamp,
		SourcePerm:     src,
		SourceCardName: src.Card.DisplayName(),
		ControllerSeat: src.Controller,
		HandlerID:      layerHandlerKey(disc, src),
		Predicate:      controllerPred,
		ApplyFn:        applyFn,
	})
}
