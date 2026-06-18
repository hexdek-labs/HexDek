package gameengine

// keywords_batch6.go — Remaining missing keyword abilities + keyword actions.
//
// This batch brings every FAIL keyword in KEYWORD_COVERAGE_REPORT.md to PASS.
//
// KEYWORD ABILITIES (§702):
//   - Madness              — CR §702.35
//   - Backup N             — CR §702.165
//   - Enlist               — CR §702.154
//   - Mutate (stub)        — CR §702.140
//   - For Mirrodin!        — CR §702.150
//   - Read Ahead           — CR §702.155
//   - Ravenous             — CR §702.156
//   - Compleated           — CR §702.163
//   - Changeling           — CR §702.73  (all creature types)
//   - Equip activation     — CR §702.6
//   - Epic                 — CR §702.50
//   - Recover              — CR §702.60
//   - Aura Swap            — CR §702.65
//   - Frenzy               — CR §702.68
//   - Gravestorm           — CR §702.69
//   - Transfigure          — CR §702.71
//   - Hidden Agenda        — CR §702.106
//   - Umbra Armor          — CR §702.89
//   - Ingest               — CR §702.113b
//   - Warp                 — CR §702.185
//   - Station              — CR §702.184
//   - Start Your Engines!  — CR §702.179
//   - Harmonize            — CR §702.180
//   - Mobilize             — CR §702.181
//   - Freerunning          — CR §702.169
//   - Gift                 — CR §702.192
//   - Space Sculptor       — §702.173
//   - Visit                — §702.177
//   - Max Speed            — §702.178
//   - Tiered               — §702.182
//   - Job Select           — §702.183
//   - Solved               — §702.186
//   - Mayhem               — §702.187
//   - Infinity             — §702.190
//   - Exhaust (already in keywords_misc.go; this adds HasExhaust)
//
// KEYWORD ACTIONS (§701):
//   - Behold               — CR §701.4
//   - Triple               — CR §701.11
//   - Exchange             — CR §701.12
//   - Convert              — CR §701.28
//   - Vote                 — CR §701.38
//   - Harness              — CR §701.64
//   - Airbend/Earthbend/Waterbend/Firebend — CR §701.65-68

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ===========================================================================
// §702.35 — Madness
// ===========================================================================

// HasMadness returns true if the card has the madness keyword.
func HasMadness(card *Card) bool {
	return cardHasKeywordByName(card, "madness")
}

// ===========================================================================
// §702.165 — Backup
// ===========================================================================

// ===========================================================================
// §702.154 — Enlist
// ===========================================================================

// ===========================================================================
// §702.140 — Mutate
// ===========================================================================

// HasMutate returns true if the card has the mutate keyword.
func HasMutate(card *Card) bool {
	return cardHasKeywordByName(card, "mutate")
}

// ApplyMutate merges a mutating creature with a target creature per CR
// §702.140. If onTop is true, the mutating creature goes on top and its
// characteristics (name, power/toughness, types) replace the target's,
// but it gains all abilities from every card in the merged pile. If onTop
// is false, the target keeps its characteristics and gains all keyword
// abilities from the mutating creature. In both cases a "creature_mutated"
// trigger fires for "whenever this creature mutates" effects.
func ApplyMutate(gs *GameState, mutatingPerm *Permanent, targetPerm *Permanent, onTop bool) {
	if gs == nil || mutatingPerm == nil || targetPerm == nil {
		return
	}
	if mutatingPerm == targetPerm {
		return
	}

	seat := targetPerm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	if onTop {
		// Mutating card goes on top — takes over characteristics.
		// Absorb target's granted abilities.
		mutatingPerm.GrantedAbilities = append(mutatingPerm.GrantedAbilities, targetPerm.GrantedAbilities...)
		// Absorb keyword abilities from the target card's AST.
		mutatingPerm.GrantedAbilities = append(mutatingPerm.GrantedAbilities, getKeywordNames(targetPerm)...)
		// Copy target's counters.
		if targetPerm.Counters != nil {
			if mutatingPerm.Counters == nil {
				mutatingPerm.Counters = map[string]int{}
			}
			for k, v := range targetPerm.Counters {
				mutatingPerm.Counters[k] += v
			}
		}
		// Remove target from battlefield; keep mutating perm.
		gs.removePermanent(targetPerm)
		detachAll(gs, targetPerm)
		if mutatingPerm.Flags == nil {
			mutatingPerm.Flags = map[string]int{}
		}
		mutatingPerm.Flags["mutated"] = 1
	} else {
		// Mutating card goes under — target keeps characteristics.
		// Target gains mutating card's keyword abilities.
		targetPerm.GrantedAbilities = append(targetPerm.GrantedAbilities, getKeywordNames(mutatingPerm)...)
		// Also absorb mutating perm's already-granted abilities.
		targetPerm.GrantedAbilities = append(targetPerm.GrantedAbilities, mutatingPerm.GrantedAbilities...)
		// Inherit the dying component's counters. Per CR §702.140 the
		// merged creature is a single object — counters live on the
		// permanent, so any +1/+1, loyalty, charge, etc. counters on
		// the mutating perm must transfer to the survivor or the merge
		// silently strips them. Symmetric to the onTop=true branch
		// above; the prior implementation only copied counters in one
		// direction, so a mutating creature sliding under (Brokkos
		// reanimating onto a +1/+1-stacked Pollywog Symbiote, etc.)
		// lost its accumulated counters.
		if mutatingPerm.Counters != nil {
			if targetPerm.Counters == nil {
				targetPerm.Counters = map[string]int{}
			}
			for k, v := range mutatingPerm.Counters {
				targetPerm.Counters[k] += v
			}
		}
		// Remove mutating perm from battlefield.
		gs.removePermanent(mutatingPerm)
		detachAll(gs, mutatingPerm)
		if targetPerm.Flags == nil {
			targetPerm.Flags = map[string]int{}
		}
		targetPerm.Flags["mutated"] = 1
	}

	// Fire "whenever this creature mutates" triggers. The "merged" perm
	// (the one that survives the merge) is passed in ctx so per_card
	// handlers for "Whenever this creature mutates, ..." can gate on
	// "did I just mutate" rather than firing on every controller-level
	// mutation. mutated_perm == mutatingPerm when onTop=true (the
	// mutating card takes characteristics on top of the target) and
	// targetPerm when onTop=false (the mutating card slides under the
	// target). The single surviving permanent carries Flags["mutated"]
	// = 1 in both branches.
	mergedPerm := mutatingPerm
	dyingPerm := targetPerm
	if !onTop {
		mergedPerm = targetPerm
		dyingPerm = mutatingPerm
	}

	// Phase 8: stamp MergedCards / MergeKind / TopCard for InstanceID
	// lineage tracking per design v2 §8. The surviving Permanent's own
	// card is seeded into MergedCards on first invocation; subsequent
	// mutates onto the same Permanent inherit the dying Permanent's
	// prior merge stack so the unmerge walker can route every
	// constituent card individually on leave-play.
	RecordMutateMerge(gs, mergedPerm, dyingPerm, onTop)
	FireCardTrigger(gs, "creature_mutated", map[string]interface{}{
		"controller_seat": seat,
		"mutated_perm":    mergedPerm,
	})

	mutName := "<nil>"
	if mutatingPerm.Card != nil {
		mutName = mutatingPerm.Card.DisplayName()
	}
	targName := "<nil>"
	if targetPerm.Card != nil {
		targName = targetPerm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind: "mutate", Seat: seat,
		Source: mutName,
		Details: map[string]interface{}{
			"target": targName,
			"on_top": onTop,
			"rule":   "702.140",
		},
	})
}

// §702.140 — ApplyMutatePlaceholder removed. Real impl: ApplyMutate above.
// No production callers existed (verified by grep), so the orphan stub
// went away in round 23 cleanup.

// ===========================================================================
// §702.150 — For Mirrodin!
// ===========================================================================

// ===========================================================================
// §702.155 — Read Ahead
// ===========================================================================

// ===========================================================================
// §702.156 — Ravenous
// ===========================================================================

// ===========================================================================
// §702.163 — Compleated
// ===========================================================================

// ===========================================================================
// §702.73 — Changeling (all creature types)
// ===========================================================================

// ===========================================================================
// §702.6 — Equip activation
// ===========================================================================

// EquipCost extracts the equip cost from an equipment card.
func EquipCost(card *Card) int {
	return keywordArgCost(card, "equip")
}

// ActivateEquip pays the equip cost and attaches an equipment to a target
// creature the controller controls. Sorcery speed only (CR §702.6a).
// Returns true on success.
func ActivateEquip(gs *GameState, seatIdx int, equipment *Permanent, target *Permanent) bool {
	if gs == nil || equipment == nil || target == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !equipment.IsEquipment() {
		return false
	}
	if !target.IsCreature() {
		return false
	}
	// CR §702.6b — an Equipment can't equip itself. Creature-Equipments
	// (Reconfigure, Living weapon) satisfy both IsEquipment and IsCreature.
	if equipment == target {
		return false
	}
	if equipment.Controller != seatIdx || target.Controller != seatIdx {
		return false
	}

	// Sorcery speed check.
	if gs.Active != seatIdx {
		return false
	}

	seat := gs.Seats[seatIdx]
	cost := EquipCost(equipment.Card)

	if seat.ManaPool < cost {
		return false
	}

	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Detach from previous creature if any.
	equipment.AttachedTo = target

	gs.LogEvent(Event{
		Kind:   "equip",
		Seat:   seatIdx,
		Source: equipment.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.6",
		},
	})
	return true
}

// ===========================================================================
// §702.50 — Epic
// ===========================================================================

// ApplyEpic copies the spell at the beginning of each of your upkeeps for
// the rest of the game. You can't cast spells for the rest of the game.
func ApplyEpic(gs *GameState, seatIdx int, item *StackItem) {
	if gs == nil || item == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Set the "can't cast spells" flag.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["epic_no_cast"] = 1

	// Register a delayed trigger to copy the spell each upkeep.
	epicCard := item.Card
	epicEffect := item.Effect
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "upkeep",
		ControllerSeat: seatIdx,
		SourceCardName: epicCard.DisplayName() + " (epic)",
		OneShot:        false, // repeating
		EffectFn: func(gs *GameState) {
			if gs.Active != seatIdx {
				return
			}
			// Route the epic copy through MintSpellCopy so it carries a
			// fresh CP-provenance InstanceID rather than aliasing the
			// captured epicCard pointer. Without this, stack.go's §707.10
			// cease branch retires the SOURCE's InstanceID each upkeep —
			// the source card living in the graveyard (epic spells go
			// there once cast) would then be flagged as fabrication
			// (Phase G sibling-site closure, Aziza-shape bypass).
			copyCard := MintSpellCopy(gs, epicCard)
			copyItem := &StackItem{
				Card:       copyCard,
				Controller: seatIdx,
				Effect:     epicEffect,
				IsCopy:     true,
				CostMeta:   map[string]interface{}{"epic_copy": true},
			}
			PushStackItem(gs, copyItem)
			gs.LogEvent(Event{
				Kind:   "epic_copy",
				Seat:   seatIdx,
				Source: epicCard.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.50",
				},
			})
			// CR §702.137a / "whenever you copy a spell" — each upkeep epic
			// copy fires the canonical copy-trigger fan-out (magecraft +
			// spell_copied). Epic spells are sorceries, so magecraft applies.
			FireSpellCopyTriggers(gs, seatIdx, copyCard, epicCard)
		},
	})

	gs.LogEvent(Event{
		Kind:   "epic",
		Seat:   seatIdx,
		Source: epicCard.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.50",
		},
	})
}

// ===========================================================================
// §702.60 — Recover
// ===========================================================================

// ===========================================================================
// §702.65 — Aura Swap
// ===========================================================================

// ===========================================================================
// §702.68 — Frenzy
// ===========================================================================

// ===========================================================================
// §702.69 — Gravestorm
// ===========================================================================

// ===========================================================================
// §702.71 — Transfigure
// ===========================================================================

// ===========================================================================
// §702.106 — Hidden Agenda
// ===========================================================================

// ===========================================================================
// §702.89 — Umbra Armor
// ===========================================================================

// ===========================================================================
// §702.113b — Ingest
// ===========================================================================

// ===========================================================================
// Newer / Set-Specific Keywords (§702.169+)
// ===========================================================================

// ---------------------------------------------------------------------------
// §702.169 — Freerunning
// ---------------------------------------------------------------------------

// CanCastForFreerunning returns true if a creature you control dealt combat
// damage to a player this turn, enabling the freerunning alt cost.
func CanCastForFreerunning(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Flags == nil {
		return false
	}
	return seat.Flags["creature_dealt_combat_damage_to_player"] > 0
}

// ---------------------------------------------------------------------------
// §702.192 — Gift
// ---------------------------------------------------------------------------
// HasGift / GiftType / CastWithGift / ResolveGift live in
// keywords_gift.go where the cast-time promise + resolution-time
// token-creation pipeline is implemented in full. The previous
// always-declines ApplyGift stub here was removed (it had no
// callers).

// §702.173 — Space Sculptor: real impl (sector partition + zone-control)
// lives in keywords_stubs_tail.go.

// ---------------------------------------------------------------------------
// §702.177 — Visit
// ---------------------------------------------------------------------------
// HasVisit / ApplyVisit / VisitedThisTurn / ClearVisitFlags live in
// keywords_visit.go where the mechanic is implemented as a per-permanent
// "visited_this_turn" counter with trigger fan-out and EOT cleanup.

// ---------------------------------------------------------------------------
// §702.178 — Max Speed
// ---------------------------------------------------------------------------
//
// "Max speed" is a rider keyword: the effect after "max speed —" only
// kicks in while the permanent's controller is at MaxSpeedCap (4). The
// player-side speed counter system lives in keywords_speed_counter.go;
// this file just exposes the rider predicates resolvers call.

// HasMaxSpeedKeyword reports whether the printed text of the permanent
// includes the max-speed rider keyword. Pure keyword check, no game
// state needed.
func HasMaxSpeedKeyword(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	return perm.HasKeyword("max speed")
}

// HasMaxSpeed reports whether the max-speed rider on this permanent is
// currently active — i.e. the permanent has the keyword AND its
// controller is at max speed (Seat.Speed == MaxSpeedCap). Resolvers
// gating riders should call this rather than the bare keyword check.
//
// Returns false for nil game / nil perm / unknown controller.
func HasMaxSpeed(gs *GameState, perm *Permanent) bool {
	if !HasMaxSpeedKeyword(perm) {
		return false
	}
	return MaxSpeedActive(gs, perm.Controller)
}

// ---------------------------------------------------------------------------
// §702.179 — Start Your Engines!
// ---------------------------------------------------------------------------

// ApplyStartYourEngines animates all Vehicles the controller controls
// until end of turn, making them creature artifacts. Records on each
// animated permanent whether we *added* the creature type so the
// cleanup-step hook can restore the original type list without stripping
// "creature" from vehicles that were already creatures (Living Metal,
// crewed-pre-engine, etc.).
//
// State written per animated permanent:
//
//	p.Flags["start_your_engines"]                 = 1
//	p.Flags["start_your_engines_added_creature"]  = 1 (only if we appended "creature")
//
// EndStepClearStartYourEngines reads both flags and is wired into the
// cleanup-step pass in phases.go (next to ClearMayhemDiscards /
// ClearVisitFlags).
func ApplyStartYourEngines(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isVehicle := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "vehicle") {
				isVehicle = true
				break
			}
		}
		if !isVehicle {
			continue
		}
		hasCreature := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "creature") {
				hasCreature = true
				break
			}
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if !hasCreature {
			p.Card.Types = append(p.Card.Types, "creature")
			p.Flags["start_your_engines_added_creature"] = 1
		}
		p.Flags["start_your_engines"] = 1
		count++
	}

	gs.LogEvent(Event{
		Kind:   "start_your_engines",
		Seat:   seatIdx,
		Amount: count,
		Details: map[string]interface{}{
			"vehicles_animated": count,
			"rule":              "702.179",
		},
	})
}

// EndStepClearStartYourEngines is the cleanup-step counterpart to
// ApplyStartYourEngines: for each permanent flagged as animated this
// turn it strips the "creature" type we added (if any), clears both
// flags, and invalidates the characteristics cache so SBAs see the
// restored type set.
//
// Idempotent: running it twice in succession is a no-op on the second
// pass. Safe to call on a GameState with no animated permanents.
// Returns the number of permanents whose flags were cleared (mostly
// useful for tests / telemetry).
func EndStepClearStartYourEngines(gs *GameState) int {
	if gs == nil {
		return 0
	}
	cleared := 0
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Flags == nil {
				continue
			}
			if p.Flags["start_your_engines"] == 0 {
				continue
			}
			if p.Flags["start_your_engines_added_creature"] == 1 && p.Card != nil {
				kept := p.Card.Types[:0]
				for _, t := range p.Card.Types {
					if strings.EqualFold(t, "creature") {
						continue
					}
					kept = append(kept, t)
				}
				p.Card.Types = kept
			}
			delete(p.Flags, "start_your_engines")
			delete(p.Flags, "start_your_engines_added_creature")
			cleared++
		}
	}
	if cleared > 0 {
		gs.InvalidateCharacteristicsCache()
		gs.LogEvent(Event{
			Kind:   "start_your_engines_clear",
			Amount: cleared,
			Details: map[string]interface{}{
				"rule": "702.179",
			},
		})
	}
	return cleared
}

// ---------------------------------------------------------------------------
// §702.180 — Harmonize
//
// Harmonize is an enchantment subtype introduced in Aetherdrift. A Harmonize
// enchantment provides its controller with one or more activated abilities
// that read "{cost}: Search your library for a card, put it into your hand,
// then shuffle." Each activation pays the cost; the activation must be at
// sorcery speed unless the printed ability says otherwise.
//
// Modeling:
//   - HasHarmonize(perm)         reports the subtype via Subtypes / Types /
//                                TypeLine, since the card-types and subtypes
//                                are mixed in the Card.Types slice.
//   - HasHarmonizeCard(card)     same check for a Card.
//   - HarmonizeActivate          pays the activation cost, searches the
//                                controller's library for a card, moves it
//                                to hand, shuffles, and logs the activation.
// ---------------------------------------------------------------------------

// HasHarmonize reports whether the permanent is a Harmonize enchantment.
// Per the Aetherdrift introduction, Harmonize is a noncreature enchantment
// subtype, so we accept either an explicit Subtypes entry (post-layers) or
// a substring match in the printed Types / TypeLine.
func HasHarmonize(perm *Permanent) bool {
	if perm == nil || perm.Card == nil {
		return false
	}
	if !cardHasType(perm.Card, "enchantment") {
		return false
	}
	return cardHasSubtype(perm.Card, "harmonize")
}

// HasHarmonizeCard reports whether the card has the Harmonize subtype while
// off the battlefield. Used during deck analysis / cast-pipeline checks.
func HasHarmonizeCard(card *Card) bool {
	if card == nil {
		return false
	}
	if !cardHasType(card, "enchantment") {
		return false
	}
	return cardHasSubtype(card, "harmonize")
}

// HarmonizeActivate activates a Harmonize enchantment's search-library
// ability. The controller pays `cost` mana, searches their library for the
// first card matching `cardFilter` (or the first card at all if filter is
// nil), puts it into their hand, and shuffles. Returns true on a successful
// activation (cost paid and search resolved, even if no card was found).
//
// Sorcery-speed gating: the activation is rejected unless `seatIdx` is the
// active player. Per CR §307 / §602, search-library activated abilities on
// Harmonize enchantments are restricted to sorcery speed.
//
// `cardFilter` lets the caller restrict the search (e.g. "creature card",
// basic land, specific subtype). nil means "any card" — useful for tutor-
// style Harmonize abilities. The first matching card encountered in library
// order is selected; this keeps tests deterministic and matches the
// existing tutor helpers in keywords_batch5.go (Transfigure) and
// keywords_misc.go.
func HarmonizeActivate(gs *GameState, seatIdx int, perm *Permanent, cost int, cardFilter func(*Card) bool) bool {
	if gs == nil || perm == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !HasHarmonize(perm) {
		return false
	}
	if perm.Controller != seatIdx {
		return false
	}
	// Sorcery-speed gate (CR §307.4 — activated abilities on enchantments
	// default to instant speed, but Harmonize search abilities are printed
	// at sorcery speed; enforce it here).
	if gs.Active != seatIdx {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.ManaPool < cost {
		return false
	}

	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	foundIdx := -1
	for i, c := range seat.Library {
		if c == nil {
			continue
		}
		if cardFilter != nil && !cardFilter(c) {
			continue
		}
		foundIdx = i
		break
	}

	foundName := "<none>"
	if foundIdx >= 0 {
		found := seat.Library[foundIdx]
		MoveCard(gs, found, seatIdx, "library", "hand", "tutor-to-hand")
		if found != nil {
			foundName = found.DisplayName()
		}
	}

	// Shuffle library per Harmonize wording ("...then shuffle.").
	if gs.Rng != nil && len(seat.Library) > 1 {
		gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
			seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
		})
	}

	gs.LogEvent(Event{
		Kind:   "harmonize_activate",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"found": foundName,
			"rule":  "702.180",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// §702.181 — Mobilize (Tarkir: Dragonstorm)
//
// CR §702.181a: "Mobilize N" means "Whenever this creature attacks, create
// N 1/1 red Warrior creature tokens. Those tokens enter tapped and
// attacking. Sacrifice them at the beginning of the next end step."
//
// Modeling:
//   - HasMobilize(card)          reports the keyword.
//   - MobilizeCount(card)        extracts N from the keyword args; falls
//                                back to 1 (the §702.181a default) if the
//                                argument is missing or not numeric.
//   - ApplyMobilize              runs the attack trigger: mints N tapped-
//                                attacking 1/1 red Warrior tokens that
//                                attack the same defender as the source via
//                                the canonical CreateDoubledTokens chokepoint
//                                (so token-count doublers apply and
//                                token_created fires), and registers a
//                                one-shot end-of-turn delayed trigger to
//                                SACRIFICE them (CR §702.181a — not exile, so
//                                "whenever a token leaves / dies" payoffs and
//                                graveyard recursion see them). Mirrors the
//                                Myriad pattern in keywords_combat.go.
//   - FireMobilizeTriggers       checks every declared attacker and fires
//                                ApplyMobilize for each one with the keyword,
//                                EXCEPT cards whose creature_attacks trigger
//                                is owned by a per_card handler (Zurgo
//                                Stormrender, Zurgo, Thunder's Decree) — those
//                                implement mobilize themselves, so the generic
//                                path would double the tokens. Mirrors the
//                                PerCardOwnsTrigger gate in combat.go's
//                                fireAttackTriggers. Wired into
//                                CheckAttackKeywordsCombat.
// ---------------------------------------------------------------------------

// HasMobilize reports whether the card carries the mobilize keyword.
func HasMobilize(card *Card) bool {
	return cardHasKeywordByName(card, "mobilize")
}

// MobilizeCount returns N for "Mobilize N". Defaults to 1 if no numeric
// argument is parsed — matches the §702.181a fallback for "Mobilize" with
// no explicit count printed.
func MobilizeCount(card *Card) int {
	n := keywordArgCost(card, "mobilize")
	if n <= 0 {
		return 1
	}
	return n
}

// ApplyMobilize fires the Mobilize attack trigger for `attacker`. Creates N
// 1/1 red Warrior creature tokens that are tapped and attacking the same
// defender as `attacker` (CR §702.181a), and registers a one-shot end-of-turn
// delayed trigger to SACRIFICE them.
//
// Tokens are minted through the canonical CreateDoubledTokens chokepoint so
// token-count doublers (Doubling Season / Parallel Lives / Anointed
// Procession) double the Warrior tokens and the token_created trigger fires
// for token-matters payoffs — the prior hand-rolled append bypassed both.
// MarkEnteredAttacking stamps the §508.1g "entered attacking" carve-out so
// checkCombatLegality honors them and they are NOT re-run through
// declare-attackers triggers.
//
// If the attacker has no recorded defender (e.g. tests calling this in
// isolation) the tokens are created but no AttackerDefender is set; they
// are still flagged "attacking" so combat-damage assignment can pick them
// up if a defender is later assigned by the caller.
func ApplyMobilize(gs *GameState, attacker *Permanent, attackerSeat int) {
	if gs == nil || attacker == nil {
		return
	}
	if attacker.Card == nil || !HasMobilize(attacker.Card) {
		return
	}
	if attackerSeat < 0 || attackerSeat >= len(gs.Seats) {
		return
	}
	n := MobilizeCount(attacker.Card)
	if n <= 0 {
		return
	}

	defSeat, hasDef := AttackerDefender(attacker)
	sourceName := attacker.Card.DisplayName()

	seat := gs.Seats[attackerSeat]
	if seat == nil {
		return
	}

	// CR §702.181a — "create N 1/1 red Warrior creature tokens" is a single
	// token-creation event minting N tokens, so route the whole batch through
	// the §614 would_create_token replacement chain (doublers see N→2N) and
	// apply the tapped/attacking/Warrior setup to every resulting token,
	// including doubled copies. Mirrors ApplyMyriad.
	tokens := CreateDoubledTokens(gs, attackerSeat, n, attacker, func() *Permanent {
		token := CreateCreatureToken(gs, attackerSeat, "Warrior Token",
			[]string{"creature", "warrior"}, 1, 1)
		if token == nil {
			return nil
		}
		if token.Card != nil {
			token.Card.Colors = []string{"R"}
			token.Card.TypeLine = "Token Creature — Warrior"
		}
		token.Tapped = true
		token.SummoningSick = false
		MarkEnteredAttacking(token)
		if hasDef {
			setAttackerDefender(token, defSeat)
		}
		if token.Flags == nil {
			token.Flags = map[string]int{}
		}
		token.Flags["mobilize_token"] = 1
		return token
	})

	gs.LogEvent(Event{
		Kind:   "mobilize",
		Seat:   attackerSeat,
		Source: sourceName,
		Amount: len(tokens),
		Details: map[string]interface{}{
			"defender": defSeat,
			"tokens":   len(tokens),
			"rule":     "702.181",
		},
	})

	// Sacrifice at the beginning of the next end step (CR §702.181a). Uses
	// end_of_turn rather than end_of_combat so the tokens persist through the
	// damage step. SacrificePermanent (not exile) fires permanent_ltb /
	// dies triggers and routes the tokens to the graveyard, so "whenever a
	// token you control leaves the battlefield" payoffs (Zurgo Stormrender's
	// sibling ability, aristocrats engines) and death triggers see them.
	capturedTokens := tokens
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: attackerSeat,
		SourceCardName: sourceName + " (mobilize)",
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			for _, tok := range capturedTokens {
				if alive(gs, tok) {
					SacrificePermanent(gs, tok, "mobilize_end_step")
				}
			}
		},
	})
}

// FireMobilizeTriggers iterates declared attackers and fires ApplyMobilize
// for every one that has the keyword. Called from
// CheckAttackKeywordsCombat in keywords_combat.go.
func FireMobilizeTriggers(gs *GameState, attackerSeat int, attackers []*Permanent) {
	if gs == nil {
		return
	}
	for _, atk := range attackers {
		if atk == nil || atk.Card == nil {
			continue
		}
		if !HasMobilize(atk.Card) {
			continue
		}
		// Double-fire gate: a per_card handler that owns this card's
		// creature_attacks trigger (Zurgo Stormrender, Zurgo, Thunder's
		// Decree) mints the mobilize Warrior tokens itself. Firing the
		// generic path too would double the count. Mirrors the
		// PerCardOwnsTrigger gate in combat.go's fireAttackTriggers.
		if PerCardOwnsTrigger(atk.Card.DisplayName(), "creature_attacks") {
			continue
		}
		ApplyMobilize(gs, atk, attackerSeat)
	}
}

// ---------------------------------------------------------------------------
// §702.182 — Tiered (stub)
// ---------------------------------------------------------------------------

// §702.183 Job Select implementation moved to keywords_job_select.go
// §702.184 Station implementation moved to keywords_station.go

// §702.184 — ApplyStation orphan stub removed. Real impl: ActivateStation
// in keywords_station.go (counter + threshold + becomes_stationed event).
// No production callers existed (verified by grep), so the orphan went
// away in round 23 cleanup.

// ---------------------------------------------------------------------------
// §702.185 — Warp
//
// "Warp [cost]" represents two static abilities on the card while in the
// stack-or-hand-or-exile zones:
//
//  1. "You may cast this card from your hand by paying [cost] rather than
//     its mana cost." — alternative cost (CR §601.2b, §601.2f–h).
//  2. "If this spell's warp cost was paid, exile the permanent this spell
//     becomes at the beginning of the next end step. Its owner may cast
//     this card after the current turn has ended for as long as it
//     remains exiled." — delayed triggered exile + zone-cast permission.
//
// Implementation:
//
//   - HasWarp(card) reports the keyword.
//   - CastWarp pays the warp cost, removes the card from its owner's
//     hand, and pushes a StackItem flagged with CostMeta["warped"]=true.
//   - In stack.go's spell-resolves-as-permanent branch, when the resolved
//     permanent's source StackItem has CostMeta["warped"], we call
//     RegisterWarpExileTrigger to schedule the end-step exile.
//   - The delayed-trigger EffectFn exiles the permanent and grants a
//     ZoneCastPermission so the owner may cast it from exile on a later
//     turn at its normal mana cost.
// ---------------------------------------------------------------------------

// HasWarp returns true if the card has the warp keyword.
func HasWarp(card *Card) bool {
	return cardHasKeywordByName(card, "warp")
}

// CastWarp casts a card from `seatIdx`'s hand for its warp cost.
// CR §702.185a.
//
// Preconditions:
//   - card is in seat's hand
//   - card has the warp keyword (caller is responsible for matching
//     warpCost to the printed keyword cost; this function does not parse
//     the keyword arguments)
//   - seat can afford warpCost mana
//   - normal timing/legality applies (sorcery-speed for permanents, etc.)
//     — checked by upstream cast pipeline; CastWarp itself does not
//     enforce sorcery-speed because that's a generic spell-casting rule
//     enforced before any alt-cost decision.
//
// On success: removes card from hand, pays warpCost, pushes a StackItem
// with CostMeta["warped"]=true and CostMeta["warp_cost"]=warpCost, sets
// the seat-level flag "spell_warped_this_turn:<seat>" for cards that ask
// "was a spell warped this turn?" (e.g. Axavar, Fate Thief).
func CastWarp(gs *GameState, seatIdx int, card *Card, warpCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasWarp(card) {
		return nil, &CastError{Reason: "no_warp_keyword"}
	}
	if warpCost < 0 {
		return nil, &CastError{Reason: "invalid_warp_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the warp alternative cost
	// too (warp casts from hand). Zero-value context.
	warpCost = EffectiveAlternativeCost(gs, card, seatIdx, warpCost, CastContext{})
	if seat.ManaPool < warpCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	// Remove from hand.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the warp cost.
	seat.ManaPool -= warpCost
	SyncManaAfterSpend(seat)
	if warpCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: warpCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "warp_cast",
				"keyword": "warp",
				"rule":    "601.2f",
			},
		})
	}
	// Push onto the stack flagged as warped.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		CostMeta: map[string]interface{}{
			"warped":    true,
			"warp_cost": warpCost,
		},
	}
	PushStackItem(gs, item)

	// Mark the seat as having warped a spell this turn — used by cards
	// like Axavar, Fate Thief that key off "if a spell was warped this
	// turn." Cleared in the cleanup step alongside other "this turn"
	// flags (caller responsibility; we just set the marker).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_warped_this_turn:"+itoaBatch(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "warp_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: warpCost,
		Details: map[string]interface{}{
			"rule": "702.185a",
		},
	})

	return &CostPaymentResult{}, nil
}

// NewWarpCastFromExilePermission returns a ZoneCastPermission that lets
// the owner cast a previously-warp-exiled card from exile at its normal
// mana cost on a later turn. Created by the warp delayed trigger when it
// exiles the permanent. CR §702.185a, second clause.
//
// ManaCost -1 instructs CastFromZone to use the card's printed mana cost
// (the "cast normally from exile" semantic — the alt warp cost is only
// available on the FIRST cast, from hand).
//
// RequireController is set to the warped card's owner so opponents
// cannot steal the cast permission.
func NewWarpCastFromExilePermission(owner int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "warp",
		ManaCost:          -1, // use card's printed mana cost
		RequireController: owner,
		SourceName:        "warp_exile",
		Duration:          "", // permanent until cast
	}
}

// RegisterWarpExileTrigger schedules a one-shot delayed trigger that
// fires at the next end step and exiles the warped permanent, then
// grants its owner a ZoneCastPermission to cast it from exile on a later
// turn. CR §702.185a, second static.
//
// The trigger captures the permanent's timestamp so subsequent
// look-up resolves the same object even if it has been bounced and
// re-entered. If the permanent has already left the battlefield by the
// time the trigger fires, the trigger is a no-op (the §603.10 "intervening
// 'if'" doesn't apply, but the effect itself does nothing useful when the
// permanent is gone — exiling from elsewhere is not what the keyword
// instructs).
func RegisterWarpExileTrigger(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	cardName := perm.Card.DisplayName()
	owner := perm.Owner
	timestamp := perm.Timestamp
	controller := perm.Controller

	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: controller,
		SourceCardName: cardName,
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			// Locate the permanent on whichever battlefield by timestamp.
			var found *Permanent
			var seatIdx int
			for i, s := range gs.Seats {
				if s == nil {
					continue
				}
				for _, p := range s.Battlefield {
					if p != nil && p.Timestamp == timestamp {
						found = p
						seatIdx = i
						break
					}
				}
				if found != nil {
					break
				}
			}
			if found == nil {
				// Permanent left the battlefield before the trigger fired.
				// The §702.185a clause only applies to "the permanent this
				// spell becomes"; if it's no longer that permanent, nothing
				// to exile. Still grant the cast-from-exile permission if
				// the card is currently in exile (someone exiled it during
				// the turn — the card-from-exile-cast clause still
				// nominally applies for the owner).
				gs.LogEvent(Event{
					Kind:   "warp_exile_skipped",
					Seat:   controller,
					Source: cardName,
					Details: map[string]interface{}{
						"reason": "permanent_not_on_battlefield",
						"rule":   "702.185a",
					},
				})
				return
			}
			// Exile the permanent. Use SacrificePermanent? No — sacrifice
			// is a different game action with its own triggers. Use the
			// standard permanent-to-exile zone move.
			ExileWarpedPermanent(gs, found, seatIdx)
			// Grant cast-from-exile permission to the owner. The owner
			// (not necessarily the current controller — control can have
			// changed) can cast the card from exile on a later turn at
			// its normal mana cost.
			if gs.ZoneCastGrants == nil {
				gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
			}
			gs.ZoneCastGrants[found.Card] = NewWarpCastFromExilePermission(owner)
			gs.LogEvent(Event{
				Kind:   "warp_exile",
				Seat:   controller,
				Source: cardName,
				Details: map[string]interface{}{
					"owner": owner,
					"rule":  "702.185a",
				},
			})
		},
	})

	gs.LogEvent(Event{
		Kind:   "warp_trigger_registered",
		Seat:   controller,
		Source: cardName,
		Details: map[string]interface{}{
			"trigger_at": "end_of_turn",
			"rule":       "702.185a",
		},
	})
}

// ExileWarpedPermanent removes a permanent from its controller's
// battlefield and adds the underlying Card to its owner's exile. This is
// the move performed by warp's delayed trigger and is distinct from a
// sacrifice (no dies/LTB triggers should fire for a warp exile per
// §702.185a — exile-from-battlefield is a §614 zone change, but it's not
// a sacrifice, so abilities keyed on "dies" or "is sacrificed" don't
// trigger; abilities keyed on "leaves the battlefield" DO trigger).
//
// We rely on MoveCard for the zone change so LTB triggers (zone_change
// events) fire normally. The permanent struct itself is removed from
// Battlefield; the Card is appended to the owner's Exile.
func ExileWarpedPermanent(gs *GameState, perm *Permanent, controllerSeat int) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[controllerSeat]
	if seat == nil {
		return
	}
	// Remove from battlefield.
	idx := -1
	for i, p := range seat.Battlefield {
		if p == perm {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	seat.Battlefield = append(seat.Battlefield[:idx], seat.Battlefield[idx+1:]...)
	// Card to owner's exile.
	owner := perm.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = controllerSeat
	}
	ownerSeat := gs.Seats[owner]
	if ownerSeat == nil {
		return
	}
	ownerSeat.Exile = append(ownerSeat.Exile, perm.Card)
	gs.LogEvent(Event{
		Kind:   "zone_change",
		Seat:   controllerSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"from":   "battlefield",
			"to":     "exile",
			"reason": "warp",
			"rule":   "702.185a",
		},
	})
}

// SpellWarpedThisTurn returns true if any spell was cast for its warp
// cost by `seatIdx` during the current turn. Backs queries like
// Axavar, Fate Thief's "if ... a spell was warped this turn".
func SpellWarpedThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_warped_this_turn:"+itoaBatch(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// §702.186 — Solved
// ---------------------------------------------------------------------------
// IsSolved / MarkSolved / ClearSolved / HasSolveAbility live in
// keywords_solved.go where the designation is implemented as a real
// per-permanent flag with "became_solved" trigger fan-out.

// ---------------------------------------------------------------------------
// §702.187 — Mayhem
// ---------------------------------------------------------------------------
// HasMayhem / MayhemCost / CastMayhem live in keywords_mayhem.go where the
// alt-cost mechanic is implemented in full (cast-from-graveyard gated on
// "if you discarded it this turn," exile-on-resolve per §702.187c).

// ---------------------------------------------------------------------------
// §702.190 — Infinity (stub)
// ---------------------------------------------------------------------------

// ===========================================================================
// KEYWORD ACTIONS (§701)
// ===========================================================================

// ---------------------------------------------------------------------------
// §701.4 — Behold
// ---------------------------------------------------------------------------
// Behold / BeholdRevealFromHand / BeholdChoosePermanent / HasBeheld /
// BeheldCount / ClearBeholdRegistry live in keywords_behold.go where
// the keyword action is implemented as a real per-seat per-turn
// registry with quality matching and "when you behold" trigger fan-out.

// ---------------------------------------------------------------------------
// §701.11 — Triple
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.12 — Exchange
// ---------------------------------------------------------------------------

// ExchangeControl exchanges control of two permanents.
//
// Control exchange (CR §701.10) keeps both permanents on the battlefield,
// so this is NOT a leaves-the-battlefield event: auras/equipment attached
// to either permanent stay attached (CR §702.6 — control change alone does
// not detach), and any replacement / continuous effects sourced from the
// exchanged permanents must keep firing. We therefore avoid
// removePermanentFromBattlefield (which after the r60 AttachmentConsistency
// fix runs detachAll + UnregisterReplacementsForPermanent +
// UnregisterContinuousEffectsForPermanent — correct for LTB-equivalent
// exits like Craft/Meld/exile-self, wrong for in-place control swap).
// Removal must also happen BEFORE swapping Controller, so gs.removePermanent
// finds each perm in its current owner's slice rather than scanning the
// post-swap slice and silently no-op'ing — the prior order left both
// permanents duplicated across both battlefields.
func ExchangeControl(gs *GameState, perm1, perm2 *Permanent) {
	if gs == nil || perm1 == nil || perm2 == nil || perm1 == perm2 {
		return
	}
	gs.removePermanent(perm1)
	gs.removePermanent(perm2)

	perm1.Controller, perm2.Controller = perm2.Controller, perm1.Controller
	perm1.Timestamp = gs.NextTimestamp()
	perm2.Timestamp = gs.NextTimestamp()

	if perm1.Controller >= 0 && perm1.Controller < len(gs.Seats) {
		gs.Seats[perm1.Controller].Battlefield = append(
			gs.Seats[perm1.Controller].Battlefield, perm1)
	}
	if perm2.Controller >= 0 && perm2.Controller < len(gs.Seats) {
		gs.Seats[perm2.Controller].Battlefield = append(
			gs.Seats[perm2.Controller].Battlefield, perm2)
	}

	name1, name2 := "<nil>", "<nil>"
	if perm1.Card != nil {
		name1 = perm1.Card.DisplayName()
	}
	if perm2.Card != nil {
		name2 = perm2.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind: "exchange_control",
		Details: map[string]interface{}{
			"perm1":          name1,
			"perm1_new_ctrl": perm1.Controller,
			"perm2":          name2,
			"perm2_new_ctrl": perm2.Controller,
			"rule":           "701.12",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.28 — Convert
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.38 — Vote
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.64 — Harness
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.65-68 — Elemental Bending (Airbend, Earthbend, Waterbend, Firebend)
// These are very new set-specific keyword actions. Stub implementations.
// ---------------------------------------------------------------------------

// Earthbend logs an earthbend action.
func Earthbend(gs *GameState, seatIdx int) {
	if gs == nil {
		return
	}
	gs.LogEvent(Event{
		Kind: "earthbend", Seat: seatIdx,
		Details: map[string]interface{}{"rule": "701.66"},
	})
}

// ===========================================================================
// Internal helpers (batch6-local)
// ===========================================================================

// getKeywordNames extracts all keyword names from a permanent's AST.
func getKeywordNames(perm *Permanent) []string {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return nil
	}
	var names []string
	for _, ab := range perm.Card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok {
			names = append(names, strings.ToLower(strings.TrimSpace(kw.Name)))
		}
	}
	return names
}
