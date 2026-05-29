package counters

// init populates the registry with the Phase 1 set — the 10 most common
// counter types observed in Probe F's corpus walk (PR #746,
// docs/counter-types-catalog-r60.md). Phase 2+ extends with the keyword
// counter family and the long-tail 242 remaining types.
func init() {
	// CR §122 stat-modifier counters. Pair-removal per §704.5r runs in the
	// SBA pass (wired in Phase 3). DoublingApplies=true so Hardened Scales /
	// Doubling Season / Primal Vigor amplify the placement once the Phase 6
	// replacement-effect pipeline is live.
	registerDefinition(&CounterTypeDef{
		Name:             "+1/+1",
		Aliases:          []string{"plus-one", "+1+1"},
		Category:         StatModifier,
		ValidTargets:     []TargetType{TargetCreature, TargetPlaneswalker},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: PairsWith("-1/-1"),
		Notes:            "CR §122 base counter; §704.5r pair-removal with -1/-1",
	})
	registerDefinition(&CounterTypeDef{
		Name:             "-1/-1",
		Aliases:          []string{"minus-one"},
		Category:         StatModifier,
		ValidTargets:     []TargetType{TargetCreature},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: PairsWith("+1/+1"),
		Notes:            "CR §122 base counter; §704.5r pair-removal with +1/+1; infect/wither sources place these",
	})

	// CR §122.1c keyword counters. The presence of the counter grants the
	// keyword for as long as the counter remains. Persistence through type
	// changes per §122.6 is pinned by counters_property_test.go.
	registerDefinition(&CounterTypeDef{
		Name:             "lifelink",
		Category:         KeywordGrant,
		ValidTargets:     []TargetType{TargetCreature},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		GrantedAbility:   &AbilityRef{Keyword: "lifelink"},
		Notes:            "CR §122.1c keyword counter; grants while present, persists through type changes per §122.6",
	})

	// CR §122 charge counters — the broadest "other tracker" family. Many
	// artifacts (Aetherflux Reservoir, Coalition Relic, Coretapper) and the
	// charge-counter Sagas use this type. DoublingApplies=true per §122.1g.
	registerDefinition(&CounterTypeDef{
		Name:             "charge",
		Category:         OtherTracker,
		ValidTargets:     []TargetType{TargetArtifact, TargetCreature, TargetLand, TargetEnchantment},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §122; broad multi-source tracker (Aetherflux Reservoir, Coalition Relic, Coretapper)",
	})

	// CR §306 loyalty counters. Doubling Season's planeswalker-ETB doubling
	// is §306.5g — handled via the replacement-effect path (Phase 6), so the
	// generic DoublingApplies flag stays true to ride that pipeline rather
	// than requiring a parallel code path.
	registerDefinition(&CounterTypeDef{
		Name:             "loyalty",
		Category:         LoyaltyCounter,
		ValidTargets:     []TargetType{TargetPlaneswalker},
		Placement:        PlaceEnterCondition | PlaceAbilityCost,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §306; Doubling Season interaction routes through §306.5g + §122.1g replacement pipeline",
	})

	// CR §714 saga lore counters. Auto-placed during precombat main per
	// §714.2; chapter abilities trigger on placement (Phase 7 wiring).
	registerDefinition(&CounterTypeDef{
		Name:             "lore",
		Category:         LoreCounter,
		ValidTargets:     []TargetType{TargetSaga},
		Placement:        PlaceAutoUpkeep | PlaceAbilityCounter,
		DoublingApplies:  false,
		Proliferate:      true,
		StackingBehavior: NoPair,
		OnPlacedTrigger:  []TriggerSpec{{Pattern: "saga_chapter_trigger"}},
		Notes:            "CR §714 (Sagas); chapter abilities trigger on placement",
	})

	// CR §702.61 time counters. Suspended cards in exile tick down at the
	// upkeep step; vanishing/fading creatures self-tick.
	registerDefinition(&CounterTypeDef{
		Name:             "time",
		Category:         TimeCounter,
		ValidTargets:     []TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetLand, TargetPlaneswalker},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter,
		DoublingApplies:  false,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §702.61 (suspend) / §702.131 (vanishing); ticks down at controller's upkeep",
	})

	// Oil counters — Phyrexia: All Will Be One mechanic. §122 generic
	// tracker; DoublingApplies per the printed rules.
	registerDefinition(&CounterTypeDef{
		Name:             "oil",
		Category:         OtherTracker,
		ValidTargets:     []TargetType{TargetCreature, TargetArtifact, TargetPlaneswalker, TargetEnchantment},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §122; ONE-block tracker (Skrelv, Defector Mite; Mondrak, Glory Dominus interactions)",
	})

	// CR §701.55 stun counters. "If a permanent with a stun counter would
	// become untapped, remove one from it instead." Modeled as a tracker
	// here; the §701.55 untap-replacement plumb lands in Phase 6 alongside
	// the rest of the replacement-effect work.
	registerDefinition(&CounterTypeDef{
		Name:             "stun",
		Category:         ResourceMarker,
		ValidTargets:     []TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetPlaneswalker, TargetLand},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §701.55 (stun counters); intercepts untap as a replacement effect (Phase 6 wiring)",
	})

	// CR §701.58 shield counters. "If a permanent with a shield counter
	// would be destroyed or dealt damage, remove a shield counter from it
	// instead." Same Phase 6 replacement-effect wiring as stun.
	registerDefinition(&CounterTypeDef{
		Name:             "shield",
		Category:         ResourceMarker,
		ValidTargets:     []TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetPlaneswalker, TargetLand},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §701.58 (shield counters); intercepts destruction/damage as a replacement effect (Phase 6 wiring)",
	})
}
