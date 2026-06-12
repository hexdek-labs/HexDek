package gameengine

// InstanceID Phase 8 — Mutate + Meld + Delayed-trigger pool per
// docs/instanceid-system-v2-r60.md §8 + §11. Companion to ApplyMutate
// (keywords_batch6.go) and Meld (keywords_batch5.go) which still drive
// the mechanic; this file owns the InstanceID-lineage layer (MergedCards
// / MergeKind / TopCard accounting, leave-play unmerge, and the
// AbilityInstance-based delayed-trigger pool).

// ---------------------------------------------------------------------------
// MergedCards / MergeKind primitives — §702.139 (Mutate) + §712 (Meld)
// ---------------------------------------------------------------------------

// RecordMutateMerge records that the dying Permanent's card has merged
// onto the surviving Permanent per CR §702.139. asTop=true means the
// surviving Permanent's own card stays on top of the stack and the
// dying card slides under; asTop=false means the dying card was the
// previous on-top and the surviving Permanent's card is now on top.
//
// (Both branches end with a single surviving Permanent — the asTop
// flag controls whether the existing stack's prior top remains on top
// or is displaced. Mirrors ApplyMutate's onTop semantics in
// keywords_batch6.go: when ApplyMutate keeps mutatingPerm and discards
// targetPerm, RecordMutateMerge is called with surviving=mutatingPerm
// and dying=targetPerm and the onTop flag forwarded unchanged.)
//
// If dying had its own MergedCards slice (multi-stack mutate of
// Brokkos onto a Vadrok+Snapdax stack), its InstanceIDs are inherited
// into surviving so the unmerge walker can route every constituent
// card individually on leave-play.
//
// Idempotent: callers are free to invoke this AFTER ApplyMutate has
// already moved characteristics around — the InstanceID accounting
// runs purely on the surviving Permanent and the dying Permanent's
// merge state.
func RecordMutateMerge(gs *GameState, surviving, dying *Permanent, asTop bool) {
	if surviving == nil || dying == nil {
		return
	}
	if surviving.MergeKind == MergeNone {
		surviving.MergeKind = MergeMutate
		// Seed the stack with the surviving permanent's own card so the
		// MergedCards slice always reflects every InstanceID currently
		// in the merge. Without this seeding, the bottom card of the
		// stack would be invisible to the unmerge walker.
		if surviving.Card != nil && surviving.Card.InstanceID != "" {
			surviving.MergedCards = append(surviving.MergedCards, surviving.Card.InstanceID)
		}
		if surviving.TopCard == nil {
			surviving.TopCard = surviving.Card
		}
	}
	if surviving.MergedCardPtrs == nil {
		surviving.MergedCardPtrs = map[string]*Card{}
	}
	if surviving.Card != nil && surviving.Card.InstanceID != "" {
		// Seed the surviving Permanent's own card pointer so the LTB
		// walker can route it if the canonical zone-write path misses
		// it (Token-cessation paths etc.).
		surviving.MergedCardPtrs[surviving.Card.InstanceID] = surviving.Card
	}
	// Inherit the dying Permanent's merge stack if it had one. Without
	// this, mutating onto a previously-merged Permanent loses the
	// stack's intermediate cards.
	var dyingStack []string
	if dying.MergeKind != MergeNone && len(dying.MergedCards) > 0 {
		dyingStack = append(dyingStack, dying.MergedCards...)
		for id, c := range dying.MergedCardPtrs {
			surviving.MergedCardPtrs[id] = c
		}
	} else if dying.Card != nil && dying.Card.InstanceID != "" {
		dyingStack = append(dyingStack, dying.Card.InstanceID)
	}
	if dying.Card != nil && dying.Card.InstanceID != "" {
		surviving.MergedCardPtrs[dying.Card.InstanceID] = dying.Card
	}
	// Filter dyingStack against surviving's existing MergedCards to
	// avoid duplicates when the same Card pointer is somehow re-merged.
	existing := make(map[string]bool, len(surviving.MergedCards))
	for _, id := range surviving.MergedCards {
		existing[id] = true
	}
	for _, id := range dyingStack {
		if existing[id] {
			continue
		}
		if asTop {
			// surviving keeps the top — dyingStack slides under.
			surviving.MergedCards = append(surviving.MergedCards, id)
		} else {
			// dying contained the prior top — its stack goes above
			// surviving's stack, with surviving's card now on top of
			// what remains (per ApplyMutate's onTop=false semantics
			// the SURVIVING Permanent is the original target and the
			// NEW card slid under — so dying's prior top now sits
			// above surviving's own card in the unmerge order). We
			// prepend to keep top-first order.
			surviving.MergedCards = append([]string{id}, surviving.MergedCards...)
		}
		existing[id] = true
	}
	// TopCard is the surviving Permanent's own Card. ApplyMutate already
	// arranged for mutatingPerm.Card to be the visible top when
	// onTop=true, and for targetPerm.Card to stay the visible top when
	// onTop=false.
	if surviving.Card != nil {
		surviving.TopCard = surviving.Card
	}
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "mutate_merge_record",
			Seat:   surviving.Controller,
			Source: surviving.Card.DisplayName(),
			Details: map[string]interface{}{
				"dying_card": dyingCardName(dying),
				"as_top":     asTop,
				"stack_size": len(surviving.MergedCards),
				"top_card":   topCardName(surviving),
				"rule":       "702.139",
			},
		})
	}
}

// dyingCardName is a nil-safe accessor for log payloads.
func dyingCardName(p *Permanent) string {
	if p == nil || p.Card == nil {
		return ""
	}
	return p.Card.DisplayName()
}

// RecordMeldMergeWithCards is the *Card-aware variant of RecordMeldMerge
// used by Meld() in keywords_batch5.go. The two component *Card
// pointers are stashed in MergedCardPtrs so UnmergeOnLeavePlay can
// route them individually to the merged permanent's destination zone.
func RecordMeldMergeWithCards(gs *GameState, result *Permanent, id1, id2 string, c1, c2 *Card) {
	if result == nil {
		return
	}
	result.MergeKind = MergeMeld
	result.MergedCards = nil
	if id1 != "" {
		result.MergedCards = append(result.MergedCards, id1)
	}
	if id2 != "" {
		result.MergedCards = append(result.MergedCards, id2)
	}
	if result.MergedCardPtrs == nil {
		result.MergedCardPtrs = map[string]*Card{}
	}
	if c1 != nil && id1 != "" {
		result.MergedCardPtrs[id1] = c1
	}
	if c2 != nil && id2 != "" {
		result.MergedCardPtrs[id2] = c2
	}
	result.TopCard = nil
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "meld_merge_record",
			Seat:   result.Controller,
			Source: result.Card.DisplayName(),
			Details: map[string]interface{}{
				"component_a": id1,
				"component_b": id2,
				"rule":        "712",
			},
		})
	}
}

// UnmergeOnLeavePlay walks a merged permanent's MergedCards slice and
// routes each constituent Card to destZone as a separate object with
// its InstanceID preserved. Per §702.139d (Mutate) and §712.3 (Meld)
// the merge dissolves the moment the permanent leaves the battlefield;
// each pre-merge card materializes individually in the destination
// zone.
//
// Phase 8 implementation note: the per-card Card pointers for the
// non-top mutate components are resolved by walking
// gs.MintedInstanceIDs and the existing exile/graveyard/library/hand
// zones to find the *Card with the matching InstanceID. When a
// constituent's Card pointer can't be located (e.g. an early
// implementation that didn't track the dying card pointers), the
// surviving Card is still routed normally — no leak, just no extra
// graveyard entries.
//
// destZone is the zone the merged permanent was heading for ("graveyard",
// "exile", "hand", "library"). For Meld the two components separate to
// the same zone the result was destined for, per §712.3.
//
// Returns the number of constituent cards actually routed individually.
// 0 when the permanent had no merge state (the normal LTB path handles
// the lone card).
func UnmergeOnLeavePlay(gs *GameState, perm *Permanent, destZone string) int {
	if gs == nil || perm == nil {
		return 0
	}
	if perm.MergeKind == MergeNone || len(perm.MergedCards) == 0 {
		return 0
	}
	moved := 0
	for _, mergedID := range perm.MergedCards {
		// The surviving permanent's own Card is routed by the normal
		// LTB path (DestroyPermanent's FireZoneChange / etc.). Skip it
		// here to avoid double-routing.
		if perm.Card != nil && perm.Card.InstanceID == mergedID {
			continue
		}
		// Prefer the impl-side card-pointer cache (cards are in "merge
		// limbo" while merged, not in any zone). Fall back to a zone
		// scan for forensic safety.
		c := perm.MergedCardPtrs[mergedID]
		if c == nil {
			c = findCardByInstanceID(gs, mergedID)
		}
		if c == nil {
			continue
		}
		ownerSeat := c.Owner
		if ownerSeat < 0 || ownerSeat >= len(gs.Seats) || gs.Seats[ownerSeat] == nil {
			continue
		}
		seat := gs.Seats[ownerSeat]
		// Route per destZone. The cards live in a "merged limbo" — they
		// don't have a current zone since they were absorbed into the
		// surviving Permanent. Append directly to the destination zone.
		switch destZone {
		case "graveyard":
			seat.Graveyard = append(seat.Graveyard, c)
		case "exile":
			seat.Exile = append(seat.Exile, c)
		case "hand":
			seat.Hand = append(seat.Hand, c)
		case "library":
			seat.Library = append(seat.Library, c)
		default:
			seat.Graveyard = append(seat.Graveyard, c)
		}
		gs.LogEvent(Event{
			Kind:   "merge_unmerge_route",
			Seat:   ownerSeat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"unmerged_card": c.DisplayName(),
				"unmerged_id":   mergedID,
				"to_zone":       destZone,
				"merge_kind":    perm.MergeKind.String(),
				"rule":          unmergeRuleFor(perm.MergeKind),
			},
		})
		moved++
	}
	// Clear the merge state once unmerged — the surviving Permanent
	// is leaving and shouldn't be considered "still merged" mid-LTB
	// trigger processing by observers.
	perm.MergedCards = nil
	perm.MergeKind = MergeNone
	perm.TopCard = nil
	return moved
}

// unmergeRuleFor returns the citation for the unmerge step depending on
// merge kind.
func unmergeRuleFor(k MergeKind) string {
	switch k {
	case MergeMutate:
		return "702.139d"
	case MergeMeld:
		return "712.3"
	}
	return ""
}

// topCardName is a nil-safe accessor for log payloads.
func topCardName(p *Permanent) string {
	if p == nil || p.TopCard == nil {
		return ""
	}
	return p.TopCard.DisplayName()
}

// findCardByInstanceID scans every seat's zones for a *Card with the
// given InstanceID. Used by UnmergeOnLeavePlay to recover constituent
// pointers when the merge state only retains InstanceIDs (the canonical
// form per design v2 §4). Returns nil when the ID is not currently
// present in any zone.
func findCardByInstanceID(gs *GameState, id string) *Card {
	if gs == nil || id == "" {
		return nil
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		if c := scanZoneForInstanceID(seat.Hand, id); c != nil {
			return c
		}
		if c := scanZoneForInstanceID(seat.Library, id); c != nil {
			return c
		}
		if c := scanZoneForInstanceID(seat.Graveyard, id); c != nil {
			return c
		}
		if c := scanZoneForInstanceID(seat.Exile, id); c != nil {
			return c
		}
	}
	return nil
}

func scanZoneForInstanceID(zone []*Card, id string) *Card {
	for _, c := range zone {
		if c != nil && c.InstanceID == id {
			return c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Delayed-trigger pool — §603.7c / design v2 §11
// ---------------------------------------------------------------------------

// RegisterDelayedAbility appends an AbilityInstance to the InstanceID-aware
// delayed-trigger pool. The caller must populate ab.DelayedUntil before
// calling — entries with a nil DelayedUntil are rejected (invariant per
// design v2 §11). The pool walker (FireDelayedAbilityPool) dispatches on
// event type and runs the condition's filter; matching entries move from
// pool to the legacy DelayedTriggers callback runner (when an EffectFn is
// attached via the AbilityInstance's TriggerMetadata "effect_fn" entry)
// or simply emit a delayed_ability_fires event for observers.
//
// Source-independence per §112.7a: the pool entry keeps firing even
// after its source permanent leaves play.
func RegisterDelayedAbility(gs *GameState, ab *AbilityInstance) {
	if gs == nil || ab == nil || ab.DelayedUntil == nil {
		return
	}
	ab.DelayedCreatedAt = gs.EffectTimestamp
	gs.DelayedAbilityInstances = append(gs.DelayedAbilityInstances, ab)
	gs.LogEvent(Event{
		Kind:   "delayed_ability_registered",
		Seat:   ab.Controller,
		Source: ab.SourceInstanceID,
		Details: map[string]interface{}{
			"instance_id": ab.InstanceID,
			"event_type":  ab.DelayedUntil.EventType,
			"one_shot":    ab.DelayedUntil.OneShot,
			"recurring":   ab.DelayedUntil.Recurring != nil,
			"created_at":  ab.DelayedCreatedAt,
			"expires_at":  ab.DelayedExpiresAt,
			"rule":        "603.7c",
		},
	})
}

// FireDelayedAbilityPool walks gs.DelayedAbilityInstances and fires any
// entry whose DelayedCondition matches the given event. The fire path
// runs the entry's TriggerMetadata "effect_fn" (a func(*GameState, *AbilityInstance))
// when present, emits a delayed_ability_fires event, then removes the
// entry from the pool unless it's recurring AND the recurrence's Until
// predicate has not yet returned true.
//
// Returns the number of pool entries that fired.
func FireDelayedAbilityPool(gs *GameState, ev *Event) int {
	if gs == nil || ev == nil || len(gs.DelayedAbilityInstances) == 0 {
		return 0
	}
	var toFire []*AbilityInstance
	for _, ab := range gs.DelayedAbilityInstances {
		if ab == nil || ab.DelayedUntil == nil {
			continue
		}
		if ab.DelayedUntil.EventType != "" && ab.DelayedUntil.EventType != ev.Kind {
			continue
		}
		if ab.DelayedUntil.EventFilter != nil && !ab.DelayedUntil.EventFilter(ev) {
			continue
		}
		toFire = append(toFire, ab)
	}
	if len(toFire) == 0 {
		return 0
	}
	keep := make(map[*AbilityInstance]bool, len(toFire))
	for _, ab := range toFire {
		fireDelayedAbilityInstance(gs, ab, ev)
		// Decide whether to keep this entry in the pool.
		if ab.DelayedUntil.Recurring != nil {
			if ab.DelayedUntil.Recurring.Until != nil && ab.DelayedUntil.Recurring.Until(gs, ab) {
				continue // expire after this firing
			}
			keep[ab] = true
		} else if !ab.DelayedUntil.OneShot {
			keep[ab] = true
		}
	}
	if len(toFire) > 0 {
		next := gs.DelayedAbilityInstances[:0]
		fireSet := make(map[*AbilityInstance]bool, len(toFire))
		for _, ab := range toFire {
			fireSet[ab] = true
		}
		for _, ab := range gs.DelayedAbilityInstances {
			if !fireSet[ab] || keep[ab] {
				next = append(next, ab)
			}
		}
		gs.DelayedAbilityInstances = next
	}
	return len(toFire)
}

// fireDelayedAbilityInstance runs the effect callback (if any) for one
// pool entry and emits an audit event.
func fireDelayedAbilityInstance(gs *GameState, ab *AbilityInstance, ev *Event) {
	gs.LogEvent(Event{
		Kind:   "delayed_ability_fires",
		Seat:   ab.Controller,
		Source: ab.SourceInstanceID,
		Details: map[string]interface{}{
			"instance_id": ab.InstanceID,
			"event_type":  ev.Kind,
			"rule":        "603.7c",
		},
	})
	if ab.TriggerMetadata == nil {
		return
	}
	if fn, ok := ab.TriggerMetadata["effect_fn"].(func(*GameState, *AbilityInstance, *Event)); ok && fn != nil {
		defer func() {
			if r := recover(); r != nil {
				gs.LogEvent(Event{
					Kind:   "delayed_ability_crashed",
					Source: ab.SourceInstanceID,
					Details: map[string]interface{}{
						"instance_id": ab.InstanceID,
						"panic":       r,
					},
				})
			}
		}()
		fn(gs, ab, ev)
	}
}

// CleanupExpiredDelayedAbilities removes pool entries whose
// DelayedExpiresAt has been reached without firing. Called at phase
// boundaries (CR §514 cleanup is the canonical EOT-scoped clearing
// point); the comparison is against gs.EffectTimestamp.
func CleanupExpiredDelayedAbilities(gs *GameState) int {
	if gs == nil || len(gs.DelayedAbilityInstances) == 0 {
		return 0
	}
	now := gs.EffectTimestamp
	kept := gs.DelayedAbilityInstances[:0]
	dropped := 0
	for _, ab := range gs.DelayedAbilityInstances {
		if ab == nil {
			dropped++
			continue
		}
		if ab.DelayedExpiresAt > 0 && ab.DelayedExpiresAt <= now {
			gs.LogEvent(Event{
				Kind:   "delayed_ability_expired",
				Seat:   ab.Controller,
				Source: ab.SourceInstanceID,
				Details: map[string]interface{}{
					"instance_id": ab.InstanceID,
					"expires_at":  ab.DelayedExpiresAt,
					"now":         now,
					"rule":        "514",
				},
			})
			dropped++
			continue
		}
		kept = append(kept, ab)
	}
	gs.DelayedAbilityInstances = kept
	return dropped
}

// ---------------------------------------------------------------------------
// Suspend — §702.61 as a delayed-trigger composition
// ---------------------------------------------------------------------------

// RegisterSuspendComposition wires CR §702.61 as a pair of delayed
// abilities on the pool, per design v2 §11 (Suspend reimplemented as a
// delayed-trigger composition). The card is assumed to already be in
// exile (the cast-time exile is handled at cast resolution; this helper
// only sets up the time-counter ticker and the cast-when-zero trigger).
//
//   - A RECURRING delayed ability fires at every controller upkeep,
//     removes one time counter, and stays in the pool until counters=0.
//   - A ONE-SHOT delayed ability fires when counters reach zero, casts
//     the suspended card for free per §702.61e.
//
// The two AbilityInstances share a SourceInstanceID (the suspended
// Card's own InstanceID) so observers can correlate the pair.
func RegisterSuspendComposition(gs *GameState, card *Card, controller int, timeCounters int) {
	if gs == nil || card == nil || timeCounters <= 0 {
		return
	}
	if card.Meta == nil {
		card.Meta = map[string]any{}
	}
	card.Meta["suspend_counters"] = timeCounters
	card.Meta["suspend_controller"] = controller
	card.Meta["suspended"] = true

	// Recurring ticker — at controller's upkeep, decrement.
	tick := NewAbilityInstance(gs, nil, controller, "suspend:tick", card.InstanceID, map[string]any{
		"suspend_card": card,
		"effect_fn":    suspendTickEffect,
	})
	tick.DelayedUntil = &DelayedCondition{
		EventType: "upkeep_begin",
		EventFilter: func(ev *Event) bool {
			return ev != nil && ev.Seat == controller
		},
		Recurring: &Recurrence{
			Until: func(gs *GameState, ab *AbilityInstance) bool {
				c, _ := ab.TriggerMetadata["suspend_card"].(*Card)
				if c == nil {
					return true
				}
				n, _ := c.Meta["suspend_counters"].(int)
				return n <= 0
			},
		},
	}
	RegisterDelayedAbility(gs, tick)

	// One-shot cast-when-zero — fires when the tick event sees a zero
	// counter, on the same upkeep.
	cast := NewAbilityInstance(gs, nil, controller, "suspend:cast", card.InstanceID, map[string]any{
		"suspend_card": card,
		"effect_fn":    suspendCastEffect,
	})
	cast.DelayedUntil = &DelayedCondition{
		EventType: "suspend_counters_zero",
		EventFilter: func(ev *Event) bool {
			if ev == nil {
				return false
			}
			if id, ok := ev.Details["suspend_card_id"].(string); ok {
				return id == card.InstanceID
			}
			return false
		},
		OneShot: true,
	}
	RegisterDelayedAbility(gs, cast)
}

func suspendTickEffect(gs *GameState, ab *AbilityInstance, ev *Event) {
	c, _ := ab.TriggerMetadata["suspend_card"].(*Card)
	if c == nil {
		return
	}
	n, _ := c.Meta["suspend_counters"].(int)
	if n <= 0 {
		return
	}
	n--
	c.Meta["suspend_counters"] = n
	gs.LogEvent(Event{
		Kind:   "suspend_tick",
		Seat:   ab.Controller,
		Source: c.DisplayName(),
		Details: map[string]interface{}{
			"suspend_card_id":    c.InstanceID,
			"counters_remaining": n,
			"rule":               "702.61b",
		},
	})
	if n == 0 {
		gs.LogEvent(Event{
			Kind:   "suspend_counters_zero",
			Seat:   ab.Controller,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"suspend_card_id": c.InstanceID,
				"rule":            "702.61c",
			},
		})
		// Drive the cast-when-zero one-shot from the same pool walk.
		FireDelayedAbilityPool(gs, &Event{
			Kind:   "suspend_counters_zero",
			Seat:   ab.Controller,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"suspend_card_id": c.InstanceID,
			},
		})
	}
}

func suspendCastEffect(gs *GameState, ab *AbilityInstance, ev *Event) {
	c, _ := ab.TriggerMetadata["suspend_card"].(*Card)
	if c == nil {
		return
	}
	c.Meta["suspended"] = false
	gs.LogEvent(Event{
		Kind:   "suspend_cast_for_free",
		Seat:   ab.Controller,
		Source: c.DisplayName(),
		Details: map[string]interface{}{
			"suspend_card_id": c.InstanceID,
			"rule":            "702.61e",
		},
	})
}
