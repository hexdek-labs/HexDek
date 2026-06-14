package gameengine

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_kind_aura_loses.go — generic handler for the "aura_loses" scaffold KIND:
//
//	"Enchanted creature loses all abilities and is a <colors> <subtype>
//	 creature with base power and toughness X/Y."
//
// 18 corpus cards parse to this shape (Amphibian Downpour, Frogify, Kenrith's
// Transformation, Witness Protection, Ichthyomorphosis, Kasmina's Transmutation,
// Reprobation, Trickster's Elk, Unable to Scream, Utter Insignificance, …). The
// parser emits the static with Layer="" so registerASTStaticEffects skipped it
// and the aura attached but did NOTHING — the enchanted creature kept its real
// characteristics. This models the clause as the §613 layer stack scoped via
// perm.AttachedTo, exactly like RegisterLignify:
//
//	Layer 4 (type)    — becomes the named creature subtype (replacing others);
//	                    ensure it is a creature.
//	Layer 5 (color)   — becomes the named colors (replacing others).
//	Layer 6 (ability) — loses all abilities ("all abilities and …").
//	Layer 7b (P/T)    — base power/toughness set to X/Y.
//
// Because each ContinuousEffect is sourced from the aura permanent `p`, when the
// aura leaves the battlefield (e.g. a Storm-minted TOKEN Aura ceasing as an SBA)
// UnregisterContinuousEffectsForPermanent drops them and the creature REVERTS.
// This is the seam Amphibian Downpour's "Copies become tokens" rider needs.

var (
	reAuraLosesPT     = regexp.MustCompile(`base power and toughness (\d+)\s*/\s*(\d+)`)
	auraLosesColors   = map[string]string{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}
	reAuraLosesSubtyp = regexp.MustCompile(`is an? ([a-z ]+?) creature`)
)

// auraLosesSpec is the parsed transform.
type auraLosesSpec struct {
	loseAbilities bool
	hasPT         bool
	power         int
	toughness     int
	colors        []string // canonical W/U/B/R/G
	subtype       string   // capitalized, e.g. "Frog"
	becomesCreature bool
}

// parseAuraLoses extracts the transform from an aura_loses clause.
func parseAuraLoses(clause string) auraLosesSpec {
	low := strings.ToLower(clause)
	spec := auraLosesSpec{}
	// "loses all abilities" — the family universally starts "all abilities …".
	if strings.Contains(low, "all abilities") {
		spec.loseAbilities = true
	}
	if m := reAuraLosesPT.FindStringSubmatch(low); m != nil {
		spec.hasPT = true
		spec.power, _ = atoiSafe(m[1])
		spec.toughness, _ = atoiSafe(m[2])
	}
	// "is a <colors> <subtype> creature": pull the color words and the noun
	// immediately before "creature".
	if m := reAuraLosesSubtyp.FindStringSubmatch(low); m != nil {
		spec.becomesCreature = true
		words := strings.Fields(m[1])
		var nonColor []string
		for _, w := range words {
			w = strings.TrimSuffix(w, ",")
			if c, ok := auraLosesColors[w]; ok {
				spec.colors = append(spec.colors, c)
				continue
			}
			if w == "and" {
				continue
			}
			nonColor = append(nonColor, w)
		}
		if len(nonColor) > 0 {
			// The last non-color word is the subtype noun (e.g. "frog",
			// "fish", "elk", "citizen"). Capitalize for §205.3 subtype form.
			spec.subtype = strings.Title(nonColor[len(nonColor)-1])
		}
	}
	return spec
}

// registerAuraLoses scans p's AST for aura_loses statics and registers the
// layer stack on the enchanted creature. Called from
// RegisterContinuousEffectsForPermanent.
func registerAuraLoses(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.Card == nil || p.Card.AST == nil {
		return
	}
	source := p
	attached := func(_ *GameState, t *Permanent) bool {
		return source.AttachedTo != nil && t == source.AttachedTo
	}
	for i, ab := range p.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "aura_loses" {
			continue
		}
		clause := firstStringArg(st.Modification.Args)
		if clause == "" {
			continue
		}
		spec := parseAuraLoses(clause)
		ts := gs.NextTimestamp()
		disc := itoaLayers(i)
		name := p.Card.DisplayName()

		// Layer 4 — type/subtype: becomes the named creature subtype only.
		if spec.becomesCreature || spec.subtype != "" {
			sub := spec.subtype
			gs.RegisterContinuousEffect(&ContinuousEffect{
				Layer: LayerType, Timestamp: ts,
				SourcePerm: source, SourceCardName: name,
				ControllerSeat: source.Controller,
				HandlerID:      layerHandlerKey("aura_loses_type_"+disc, source),
				Predicate:      attached,
				ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
					if !charsHaveType(chars.Types, "creature") {
						chars.Types = append(chars.Types, "creature")
					}
					if sub != "" {
						chars.Subtypes = []string{sub} // "is a Frog" replaces subtypes
					}
				},
			})
		}
		// Layer 5 — color: becomes the named colors (replacing others).
		if len(spec.colors) > 0 {
			cols := append([]string(nil), spec.colors...)
			gs.RegisterContinuousEffect(&ContinuousEffect{
				Layer: LayerColor, Timestamp: ts,
				SourcePerm: source, SourceCardName: name,
				ControllerSeat: source.Controller,
				HandlerID:      layerHandlerKey("aura_loses_color_"+disc, source),
				Predicate:      attached,
				ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
					chars.Colors = append([]string(nil), cols...)
				},
			})
		}
		// Layer 6 — loses all abilities.
		if spec.loseAbilities {
			gs.RegisterContinuousEffect(&ContinuousEffect{
				Layer: LayerAbility, Timestamp: ts,
				SourcePerm: source, SourceCardName: name,
				ControllerSeat: source.Controller,
				HandlerID:      layerHandlerKey("aura_loses_strip_"+disc, source),
				Predicate:      attached,
				ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
					chars.Abilities = nil
					chars.Keywords = nil
					chars.LostAllAbilities = true
				},
			})
		}
		// Layer 7b — base power/toughness.
		if spec.hasPT {
			pw, th := spec.power, spec.toughness
			gs.RegisterContinuousEffect(&ContinuousEffect{
				Layer: LayerPT, Sublayer: "b", Timestamp: ts,
				SourcePerm: source, SourceCardName: name,
				ControllerSeat: source.Controller,
				HandlerID:      layerHandlerKey("aura_loses_pt_"+disc, source),
				Predicate:      attached,
				ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
					chars.Power = pw
					chars.Toughness = th
					chars.BasePower = pw
					chars.BaseToughness = th
				},
			})
		}
	}
}
