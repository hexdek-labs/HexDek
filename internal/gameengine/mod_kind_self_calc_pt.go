package gameengine

import "strings"

// mod_kind_self_calc_pt.go — generic handler for the `self_calculated_pt`
// static scaffold KIND: a characteristic-defining ability that sets this
// creature's power and/or toughness equal to a dynamic game-state count
// (Tarmogoyf, Mortivore, Lhurgoyf, Masticore-of-X, *-equal-to-lands, …).
//
// Worker hex-dev-5, category: static + continuous effects.
//
// 185 corpus cards. The Modification's arg is a RAW phrase such as
// "~'s power and toughness are each equal to the number of lands you
// control". The parser left the `layer` tag null, so these fell through
// the registerASTStaticEffects gate and the creatures sat at their
// printed base (usually 0/0 or */* → 0/0) — inert.
//
// This registers a §613 layer-7 sublayer-"a" continuous effect whose
// ApplyFn recomputes the count on every gs.PowerOf / gs.ToughnessOf
// query (combat + SBA are layer-aware, so P/T-defining is honored —
// unlike keyword grants). We recognize the common count phrases and
// SKIP anything unrecognized (leaving it inert rather than guessing) —
// conservative + additive.

// selfCalcSpec describes a parsed self_calculated_pt phrase.
type selfCalcSpec struct {
	setPower     bool
	setToughness bool
	count        func(gs *GameState, src *Permanent) int
}

// parseSelfCalcSpec parses the raw phrase into which stats it defines and
// a count function. Returns ok=false for unrecognized count sources.
func parseSelfCalcSpec(phrase string) (selfCalcSpec, bool) {
	p := strings.ToLower(phrase)
	var spec selfCalcSpec
	switch {
	case strings.Contains(p, "power and toughness"):
		spec.setPower, spec.setToughness = true, true
	case strings.Contains(p, "power is equal"), strings.Contains(p, "power are equal"):
		spec.setPower = true
	case strings.Contains(p, "toughness is equal"), strings.Contains(p, "toughness are equal"):
		spec.setToughness = true
	default:
		return spec, false
	}

	cnt, ok := parseSelfCalcCount(p)
	if !ok {
		return spec, false
	}
	spec.count = cnt
	return spec, true
}

// parseSelfCalcCount maps a recognized count phrase to a counter func.
func parseSelfCalcCount(p string) (func(gs *GameState, src *Permanent) int, bool) {
	switch {
	case strings.Contains(p, "your life total"):
		return func(gs *GameState, src *Permanent) int {
			if src == nil || src.Controller < 0 || src.Controller >= len(gs.Seats) {
				return 0
			}
			return gs.Seats[src.Controller].Life
		}, true

	case strings.Contains(p, "cards in your hand"):
		return func(gs *GameState, src *Permanent) int { return countCardsInZone(gs, src, "hand", "you") }, true

	case strings.Contains(p, "card types among cards in all graveyards"),
		strings.Contains(p, "card types among cards in your graveyard"):
		allGY := strings.Contains(p, "all graveyards")
		return func(gs *GameState, src *Permanent) int { return countDistinctCardTypesInGraveyards(gs, src, allGY) }, true

	case strings.Contains(p, "creature cards in all graveyards"):
		return func(gs *GameState, src *Permanent) int { return countGraveyardCards(gs, src, true, true, "") }, true
	case strings.Contains(p, "creature cards in your graveyard"):
		return func(gs *GameState, src *Permanent) int { return countGraveyardCards(gs, src, false, true, "") }, true
	case strings.Contains(p, "land cards in your graveyard"):
		return func(gs *GameState, src *Permanent) int { return countGraveyardCards(gs, src, false, false, "land") }, true
	case strings.Contains(p, "cards in your graveyard"):
		return func(gs *GameState, src *Permanent) int { return countCardsInZone(gs, src, "graveyard", "you") }, true

	default:
		// "<noun> you control" battlefield count.
		if idx := strings.Index(p, "number of "); idx >= 0 {
			rest := p[idx+len("number of "):]
			if ci := strings.Index(rest, " you control"); ci >= 0 {
				noun := strings.TrimSpace(rest[:ci])
				if pred, ok := nounToPermPredicate(noun); ok {
					return func(gs *GameState, src *Permanent) int {
						if src == nil || src.Controller < 0 || src.Controller >= len(gs.Seats) {
							return 0
						}
						n := 0
						for _, q := range gs.Seats[src.Controller].Battlefield {
							if q != nil && pred(q) {
								n++
							}
						}
						return n
					}, true
				}
			}
		}
		return nil, false
	}
}

// nounToPermPredicate maps a (plural) battlefield noun to a predicate.
// Recognizes the card types, basic land types, "permanents", and a
// modest set of common creature subtypes; returns ok=false otherwise so
// the caller skips rather than guesses.
func nounToPermPredicate(noun string) (func(*Permanent) bool, bool) {
	n := strings.TrimSpace(noun)
	n = strings.TrimSuffix(n, "s") // depluralize
	switch n {
	case "permanent":
		return func(*Permanent) bool { return true }, true
	case "land":
		return func(p *Permanent) bool { return p.IsLand() }, true
	case "creature":
		return func(p *Permanent) bool { return p.IsCreature() }, true
	case "artifact":
		return func(p *Permanent) bool { return p.IsArtifact() }, true
	case "enchantment":
		return func(p *Permanent) bool { return p.IsEnchantment() }, true
	case "forest", "island", "swamp", "mountain", "plain":
		sub := n
		if sub == "plain" {
			sub = "plains"
		}
		return func(p *Permanent) bool { return p.IsLand() && permanentHasSubtype(p, sub) }, true
	case "elf", "goblin", "zombie", "vampire", "sliver", "dragon", "elemental", "soldier", "wizard", "snow permanent":
		sub := n
		if n == "snow permanent" {
			return func(p *Permanent) bool { return permanentHasSupertypeSnow(p) }, true
		}
		return func(p *Permanent) bool { return permanentHasSubtype(p, sub) }, true
	default:
		return nil, false
	}
}

func permanentHasSupertypeSnow(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if t == "snow" {
			return true
		}
	}
	return false
}

// countGraveyardCards counts cards in graveyard(s). allGraveyards=false
// restricts to the controller's. creatureOnly / typeFilter narrow by card
// type ("" = any when creatureOnly is false).
func countGraveyardCards(gs *GameState, src *Permanent, allGraveyards, creatureOnly bool, typeFilter string) int {
	if src == nil {
		return 0
	}
	seats := []int{src.Controller}
	if allGraveyards {
		seats = seats[:0]
		for i := range gs.Seats {
			seats = append(seats, i)
		}
	}
	n := 0
	for _, si := range seats {
		if si < 0 || si >= len(gs.Seats) {
			continue
		}
		for _, c := range gs.Seats[si].Graveyard {
			if c == nil {
				continue
			}
			if creatureOnly && !cardHasType(c, "creature") {
				continue
			}
			if typeFilter != "" && !cardHasType(c, typeFilter) {
				continue
			}
			n++
		}
	}
	return n
}

// countDistinctCardTypesInGraveyards counts distinct CR §300.1 card types
// (the Tarmogoyf rule). allGraveyards=false restricts to the controller's.
func countDistinctCardTypesInGraveyards(gs *GameState, src *Permanent, allGraveyards bool) int {
	if src == nil {
		return 0
	}
	canonical := map[string]struct{}{
		"artifact": {}, "battle": {}, "creature": {}, "enchantment": {},
		"instant": {}, "land": {}, "planeswalker": {}, "sorcery": {},
		"kindred": {}, "tribal": {},
	}
	seats := []int{src.Controller}
	if allGraveyards {
		seats = seats[:0]
		for i := range gs.Seats {
			seats = append(seats, i)
		}
	}
	seen := map[string]struct{}{}
	for _, si := range seats {
		if si < 0 || si >= len(gs.Seats) {
			continue
		}
		for _, c := range gs.Seats[si].Graveyard {
			if c == nil {
				continue
			}
			for _, t := range c.Types {
				lt := strings.ToLower(t)
				if _, ok := canonical[lt]; ok {
					seen[lt] = struct{}{}
				}
			}
		}
	}
	return len(seen)
}

// registerSelfCalcPTStatic registers the layer-7a characteristic-defining
// continuous effect for a recognized self_calculated_pt phrase. Unknown
// phrases are skipped (left inert).
func registerSelfCalcPTStatic(gs *GameState, p *Permanent, args []interface{}) {
	if len(args) == 0 {
		return
	}
	phrase, ok := args[0].(string)
	if !ok || phrase == "" {
		return
	}
	spec, ok := parseSelfCalcSpec(phrase)
	if !ok {
		return
	}
	src := p
	applyFn := func(gs *GameState, _ *Permanent, chars *Characteristics) {
		if !charsHaveType(chars.Types, "creature") {
			return
		}
		n := spec.count(gs, src)
		if n < 0 {
			n = 0
		}
		if spec.setPower {
			chars.Power = n
			chars.BasePower = n
		}
		if spec.setToughness {
			chars.Toughness = n
			chars.BaseToughness = n
		}
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "a",
		Timestamp:      src.Timestamp,
		SourcePerm:     src,
		SourceCardName: src.Card.DisplayName(),
		ControllerSeat: src.Controller,
		HandlerID:      layerHandlerKey("ast-self-calc-pt", src),
		Predicate:      func(_ *GameState, t *Permanent) bool { return t == src },
		ApplyFn:        applyFn,
	})
}
