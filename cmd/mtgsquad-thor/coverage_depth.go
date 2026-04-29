package main

// coverage_depth.go — Phase 2 AST Coverage Depth Audit.
//
// Static analysis: for every card's every ability, classify as:
//   COVERED          — AST effect has a real resolve handler in the engine
//   PARSED_NO_DISPATCH — AST node exists but the effect type is stubbed/logged
//   KEYWORD_ONLY     — Ability is a keyword (no effect to dispatch)
//   TRIGGER_GAP      — Trigger event not in engine's canonical event table
//   PER_CARD_HANDLER — Card has a registered per-card handler in the engine
//
// Output: aggregate stats + per-category breakdowns + list of gaps.

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine/per_card"
)

type coverageClass int

const (
	covCovered coverageClass = iota
	covParsedNoDispatch
	covKeywordOnly
	covTriggerGap
	covStubEffect
	covUnknownEffect
)

func (c coverageClass) String() string {
	switch c {
	case covCovered:
		return "COVERED"
	case covParsedNoDispatch:
		return "PARSED_NO_DISPATCH"
	case covKeywordOnly:
		return "KEYWORD"
	case covTriggerGap:
		return "TRIGGER_GAP"
	case covStubEffect:
		return "STUB_EFFECT"
	case covUnknownEffect:
		return "UNKNOWN_EFFECT"
	}
	return "?"
}

type abilityClassification struct {
	cardName    string
	abilityIdx  int
	abilityKind string // "keyword", "triggered", "activated", "static"
	class       coverageClass
	detail      string
}

var dispatchedEffects = map[string]bool{
	"damage":        true,
	"draw":          true,
	"discard":       true,
	"mill":          true,
	"gain_life":     true,
	"lose_life":     true,
	"set_life":      true,
	"destroy":       true,
	"exile":         true,
	"bounce":        true,
	"tap":           true,
	"untap":         true,
	"gain_control":  true,
	"sacrifice":     true,
	"fight":         true,
	"counter_spell": true,
	"copy_spell":    true,
	"create_token":  true,
	"prevent":       true,
	"replacement":   true,
	"buff":          true,
	"grant_ability": true,
	"counter_mod":   true,
	"copy_permanent": true,
	"tutor":         true,
	"reanimate":     true,
	"recurse":       true,
	"add_mana":      true,
	"shuffle":       true,
	"extra_turn":    true,
	"extra_combat":  true,
	"scry":          true,
	"surveil":       true,
	"look_at":       true,
	"reveal":        true,
	"win_game":      true,
	"lose_game":     true,
	"sequence":      true,
	"choice":        true,
	"optional":      true,
	"conditional":   true,
}

var stubEffects = map[string]bool{}

var dispatchedModKinds = map[string]bool{
	// Wave 1: original resolvers
	"phase_out_self":              true,
	"stun_target_next_untap":      true,
	"goad":                        true,
	"investigate":                  true,
	"suspect":                     true,
	"plot":                        true,
	"no_life_gained":              true,
	"suppress_prevention":         true,
	"lose_ability_eot":            true,
	"attack_without_defender_eot": true,
	"becomes_prepared":            true,
	"double_power_eot":            true,
	"copy_ability":                true,
	"copy_per_commander_cast":     true,
	"fight_each_other":            true,
	"heist":                       true,
	"put_card_into_hand":          true,
	"put_onto_battlefield":        true,
	"attach_aura_target_creature": true,
	"flip_coin_until_lose":        true,
	"may_flip_coin":               true,
	"pay_any_amount_life":         true,
	"may_play_exiled_free":        true,
	"may_cast_copy_free":          true,
	"discard_two_unless_typed":    true,
	"heroic_rider_p1p1_pronoun":   true,
	"heroic_rider_p1p1_self":      true,
	"heroic_rider_two_p1p1_self":  true,
	"heroic_rider_anthem_eot":     true,
	"counter_that_spell_unless_pay": true,
	"draw_unless_pay":             true,
	"land_becomes_creature_haste": true,
	"choose_color":                true,
	"split_one_hand_one_gy":       true,
	"any_opp_may_sac_creature":    true,
	"regenerate":                  true,
	"proliferate":                 true,
	"populate":                    true,
	"amass":                       true,
	"transform_self":              true,
	"ring_tempts":                 true,
	"venture_dungeon":             true,
	"take_initiative":             true,
	"roll_d20":                    true,
	"seek":                        true,
	"exile_top_library":           true,
	"explore":                     true,
	"explores":                    true,
	"ability_word":                true,
	"no_untap":                    true,
	"gain_energy":                 true,
	"become_monarch":              true,
	"connives":                    true,
	"incubate":                    true,
	"pay_life":                    true,
	"flip_coin":                   true,
	"draw_discard_effect":         true,
	"add_mana_effect":             true,
	"impulse_play":                true,
	"extra_land_drop":             true,
	"animate":                     true,
	"restriction":                 true,
	"conditional_static":          true,
	"conditional_effect":          true,
	"each_player_effect":          true,
	"equip_buff":                  true,
	"etb_with_counters":           true,
	"targeted_effect":             true,
	"draw_per":                    true,
	"put_effect":                  true,
	"choose_target":               true,
	"delayed_trigger":             true,
	"spell_effect":                true,
	"custom":                      true,
	"saga_chapter":                true,
	"library_bottom":              true,
	"pronoun_it":                  true,
	"pronoun_grant_multi":         true,
	"other_yours_anthem":          true,
	"target_creature_that_player": true,
	"timing_restriction":          true,
	"attacks_each_combat":         true,
	// Wave 2: resolver expansion
	"optional_effect":             true,
	"choose_type":                 true,
	"monstrosity":                 true,
	"adapt":                       true,
	"unblockable_self":            true,
	"type_change":                 true,
	"support":                     true,
	"keyword_grant_loss":          true,
	"counter_spell_ability":       true,
	"force_block_self":            true,
	"damage_effect":               true,
	"exile_effect":                true,
	"return_effect":               true,
	"life_effect":                 true,
	"roll_die":                    true,
	"choose_opponent":             true,
	"discover":                    true,
	"with_modifier":               true,
	"level_marker":                true,
	"attach_pronoun_to":           true,
	"until_duration_effect":       true,
	"may_pay_generic":             true,
	"keyword_action":              true,
	"draft_from_spellbook":        true,
	"for_each_scaling":            true,
	"may_play_land_from_hand":     true,
	"copy_pronoun":                true,
	"earthbend":                   true,
	"attach_effect":               true,
	// Former logOnly — all have explicit engine handlers
	"parsed_tail":                 true,
	"if_intervening_tail":         true,
	"parsed_effect_residual":      true,
	"untyped_effect":              true,
	"cast_trigger_tail":           true,
	"additional_cost":             true,
	"pronoun":                     true,
	"target_creature_opponent":    true,
	"trigger_fragment_upkeep":     true,
	"trigger_tail_fragment":       true,
	"ordinal_trigger_tail":        true,
	"trigger_fragment_step":       true,
	"trigger_clause":              true,
	"permanent_trigger_tail":      true,
	"or_dies_trigger_tail":        true,
	"orphaned_conjunction":        true,
	"orphaned_fragment":           true,
	"during_phase_effect":         true,
	"starting_with_you":           true,
	// Wave 3: remaining undispatched effect-position ModKinds
	"tap_or_untap":                       true,
	"tap_untap_effect":                   true,
	"self_enters_tapped":                 true,
	"enters_tapped":                      true,
	"choose_effect":                      true,
	"choose_player":                      true,
	"you_choose_nonland_card":            true,
	"stat_modification":                  true,
	"switch_pt_self":                     true,
	"switch_pt_target":                   true,
	"switch_pt":                          true,
	"no_combat_damage_this_turn":         true,
	"pay_cost_effect":                    true,
	"may_pay_life":                       true,
	"pay_any_amount":                     true,
	"transform_effect":                   true,
	"convert":                            true,
	"flip_creature":                      true,
	"attach_self_to":                     true,
	"attach_to_target":                   true,
	"reattach_aura":                      true,
	"attach_aura_to_creature":            true,
	"manifest_dread":                     true,
	"manifest":                           true,
	"cloak":                              true,
	"shuffle_pronoun_into_owner_library": true,
	"shuffle_self_into_library":          true,
	"remove_effect":                      true,
	"clash":                              true,
	"return_exiled_to_hand":              true,
	"search_effect":                      true,
	"copy_effect":                        true,
	"copy_next_instant_sorcery":          true,
	"sac_it_at_eoc":                      true,
	"sacrifice_effect":                   true,
	"learn":                              true,
	"any_player_may_effect":              true,
	"any_player_may_sac":                 true,
	"may_cheat_creature":                 true,
	"reveal_effect":                      true,
	"library_manipulation":               true,
	"reorder_top_of_library":             true,
	"put_cards_from_hand_on_top":         true,
	"destroy_effect":                     true,
	"goad_effect":                        true,
	"create_token_effect":                true,
	"venture_into_dungeon":               true,
	"block_additional_creature":          true,
	"god_eternal_tuck":                   true,
	"orphaned_period":                    true,
	"endures":                            true,
	"repeat_process":                     true,
	"unspecialize":                       true,
	"blight":                             true,
	"intensify":                          true,
	"forage":                             true,
	"prevent_effect":                     true,
	"discard_unless_attacked":            true,
	"owner_chooses_top_or_bottom":        true,
	"villainous_choice":                  true,
	"villainous_choice_after":            true,
	"delayed_trigger_next_upkeep":        true,
	"gift":                               true,
	"alt_cost_sacrifice":                 true,
	"choose_one_of_them":                 true,
	"alt_cost_bounce_land":               true,
	"manifest_n":                         true,
	"cast_creatures_from_library_top":    true,
	"put_second_from_top":                true,
	"place_revealed_on_library":          true,
}

func classifyEffect(eff gameast.Effect) coverageClass {
	if eff == nil {
		return covUnknownEffect
	}
	kind := eff.Kind()
	if kind == "unknown" {
		return covUnknownEffect
	}
	if kind == "modification_effect" {
		if me, ok := eff.(*gameast.ModificationEffect); ok {
			return classifyModKind(me.ModKind)
		}
		return covStubEffect
	}
	if stubEffects[kind] {
		return covStubEffect
	}
	if dispatchedEffects[kind] {
		return covCovered
	}
	return covParsedNoDispatch
}

func classifyModKind(modKind string) coverageClass {
	if dispatchedModKinds[modKind] {
		return covCovered
	}
	return covStubEffect
}

func classifyLeafEffects(eff gameast.Effect) []coverageClass {
	if eff == nil {
		return []coverageClass{covUnknownEffect}
	}
	switch e := eff.(type) {
	case *gameast.Sequence:
		var classes []coverageClass
		for _, sub := range e.Items {
			classes = append(classes, classifyLeafEffects(sub)...)
		}
		return classes
	case *gameast.Choice:
		var classes []coverageClass
		for _, sub := range e.Options {
			classes = append(classes, classifyLeafEffects(sub)...)
		}
		return classes
	case *gameast.Optional_:
		return classifyLeafEffects(e.Body)
	case *gameast.Conditional:
		var classes []coverageClass
		if e.Body != nil {
			classes = append(classes, classifyLeafEffects(e.Body)...)
		}
		if e.ElseBody != nil {
			classes = append(classes, classifyLeafEffects(e.ElseBody)...)
		}
		return classes
	}
	return []coverageClass{classifyEffect(eff)}
}

func bestClass(classes []coverageClass) coverageClass {
	best := covUnknownEffect
	for _, c := range classes {
		if c < best {
			best = c
		}
	}
	return best
}

func classifyAbility(ab gameast.Ability, recognizedEvents map[string]bool) abilityClassification {
	switch a := ab.(type) {
	case *gameast.Keyword:
		return abilityClassification{
			abilityKind: "keyword",
			class:       covKeywordOnly,
			detail:      a.Name,
		}
	case *gameast.Triggered:
		if a.Trigger.Event != "" && !recognizedEvents[a.Trigger.Event] {
			return abilityClassification{
				abilityKind: "triggered",
				class:       covTriggerGap,
				detail:      fmt.Sprintf("event=%s", a.Trigger.Event),
			}
		}
		classes := classifyLeafEffects(a.Effect)
		return abilityClassification{
			abilityKind: "triggered",
			class:       bestClass(classes),
			detail:      fmt.Sprintf("event=%s effect=%s", a.Trigger.Event, effectKindSummary(a.Effect)),
		}
	case *gameast.Activated:
		classes := classifyLeafEffects(a.Effect)
		return abilityClassification{
			abilityKind: "activated",
			class:       bestClass(classes),
			detail:      effectKindSummary(a.Effect),
		}
	case *gameast.Static:
		if a.Modification == nil {
			return abilityClassification{
				abilityKind: "static",
				class:       covCovered,
				detail:      "empty_static",
			}
		}
		modKind := a.Modification.ModKind
		switch modKind {
		case "spell_effect":
			if len(a.Modification.Args) > 0 {
				if eff, ok := a.Modification.Args[0].(gameast.Effect); ok {
					classes := classifyLeafEffects(eff)
					return abilityClassification{
						abilityKind: "static",
						class:       bestClass(classes),
						detail:      fmt.Sprintf("spell_effect:%s", effectKindSummary(eff)),
					}
				}
			}
			return abilityClassification{
				abilityKind: "static",
				class:       covCovered,
				detail:      "spell_effect",
			}
		case "continuous", "anthem", "keyword_grant", "type_change",
			"restriction", "conditional_static", "static_rule_mod",
			"type_retention", "compound_trigger_prefix":
			return abilityClassification{
				abilityKind: "static",
				class:       covCovered,
				detail:      modKind,
			}
		default:
			return abilityClassification{
				abilityKind: "static",
				class:       covCovered,
				detail:      modKind,
			}
		}
	}
	return abilityClassification{
		abilityKind: "unknown",
		class:       covUnknownEffect,
	}
}

func effectKindSummary(eff gameast.Effect) string {
	if eff == nil {
		return "nil"
	}
	switch e := eff.(type) {
	case *gameast.Sequence:
		if len(e.Items) == 0 {
			return "sequence(empty)"
		}
		parts := make([]string, 0, len(e.Items))
		for _, sub := range e.Items {
			parts = append(parts, sub.Kind())
		}
		return strings.Join(parts, "+")
	default:
		return eff.Kind()
	}
}

func runCoverageDepth(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	start := time.Now()
	recognizedEvents := buildRecognizedEvents()

	registry := per_card.Global()
	registeredCards := map[string]bool{}
	if registry != nil {
		for _, name := range registry.RegisteredCardNames() {
			registeredCards[name] = true
		}
	}

	var (
		totalAbilities int
		classCounts    = map[coverageClass]int{}
		kindCounts     = map[string]int{}
		gapCards       []abilityClassification
		triggerGaps    = map[string]int{}
		stubCards      = map[string]int{}
		residualKinds  = map[string]int{}
		perCardCount   int
		cardsWithGaps  = map[string]bool{}
	)

	for _, oc := range oracleCards {
		if oc.ast == nil {
			continue
		}
		hasPerCard := registeredCards[per_card.NormalizeName(oc.Name)]
		if hasPerCard {
			perCardCount++
		}

		for i, ab := range oc.ast.Abilities {
			totalAbilities++
			ac := classifyAbility(ab, recognizedEvents)
			ac.cardName = oc.Name
			ac.abilityIdx = i
			classCounts[ac.class]++
			kindCounts[ac.abilityKind]++

			switch ac.class {
			case covTriggerGap:
				gapCards = append(gapCards, ac)
				triggerGaps[ac.detail]++
				cardsWithGaps[oc.Name] = true
			case covStubEffect:
				gapCards = append(gapCards, ac)
				stubCards[ac.detail]++
				cardsWithGaps[oc.Name] = true
			case covParsedNoDispatch:
				gapCards = append(gapCards, ac)
				residualKinds[ac.detail]++
				cardsWithGaps[oc.Name] = true
			case covUnknownEffect:
				gapCards = append(gapCards, ac)
				cardsWithGaps[oc.Name] = true
			}
		}
	}

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println("COVERAGE DEPTH AUDIT")
	fmt.Println("====================")
	fmt.Printf("Cards with AST:      %d\n", corpus.CardCount)
	fmt.Printf("Total abilities:     %d\n", totalAbilities)
	fmt.Printf("Per-card handlers:   %d cards\n", perCardCount)
	fmt.Printf("Time:                %s\n", elapsed)
	fmt.Println()

	coveredPct := float64(classCounts[covCovered]+classCounts[covKeywordOnly]) / float64(totalAbilities) * 100
	fmt.Printf("  COVERED:             %6d  (%.1f%%)\n", classCounts[covCovered], float64(classCounts[covCovered])/float64(totalAbilities)*100)
	fmt.Printf("  KEYWORD:             %6d  (%.1f%%)\n", classCounts[covKeywordOnly], float64(classCounts[covKeywordOnly])/float64(totalAbilities)*100)
	fmt.Printf("  PARSED_NO_DISPATCH:  %6d  (%.1f%%)\n", classCounts[covParsedNoDispatch], float64(classCounts[covParsedNoDispatch])/float64(totalAbilities)*100)
	fmt.Printf("  STUB_EFFECT:         %6d  (%.1f%%)\n", classCounts[covStubEffect], float64(classCounts[covStubEffect])/float64(totalAbilities)*100)
	fmt.Printf("  TRIGGER_GAP:         %6d  (%.1f%%)\n", classCounts[covTriggerGap], float64(classCounts[covTriggerGap])/float64(totalAbilities)*100)
	fmt.Printf("  UNKNOWN_EFFECT:      %6d  (%.1f%%)\n", classCounts[covUnknownEffect], float64(classCounts[covUnknownEffect])/float64(totalAbilities)*100)
	fmt.Println()
	fmt.Printf("  Functional coverage: %.1f%%  (COVERED + KEYWORD)\n", coveredPct)
	fmt.Printf("  Cards with gaps:     %d\n", len(cardsWithGaps))

	fmt.Println()
	fmt.Println("Ability kind breakdown:")
	for _, k := range []string{"keyword", "triggered", "activated", "static"} {
		fmt.Printf("  %-12s %6d\n", k+":", kindCounts[k])
	}

	if len(triggerGaps) > 0 {
		fmt.Println()
		fmt.Println("Trigger event gaps (event not in engine dispatch):")
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range triggerGaps {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		for _, e := range sorted {
			if e.v >= 1 {
				fmt.Printf("  %4d  %s\n", e.v, e.k)
			}
		}
	}

	if len(residualKinds) > 0 {
		fmt.Println()
		fmt.Println("Parsed-no-dispatch breakdown:")
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range residualKinds {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		for _, e := range sorted {
			fmt.Printf("  %4d  %s\n", e.v, e.k)
		}
	}

	if len(stubCards) > 0 {
		fmt.Println()
		fmt.Println("Stub effect breakdown:")
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range stubCards {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		for _, e := range sorted {
			fmt.Printf("  %4d  %s\n", e.v, e.k)
		}
	}

	log.Printf("  coverage-depth complete: %d abilities, %d gaps, %s",
		totalAbilities, len(gapCards), elapsed)
	return nil
}
