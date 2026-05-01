package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase 6 — State-based actions (CR §704).
//
// Per the 2026-02-27 comp-rules file (`data/rules/MagicCompRules-20260227.txt`)
// §704.3: "Whenever a player would get priority (see rule 117, 'Timing and
// Priority'), the game first performs all applicable state-based actions as
// a single event (see rule 704.3), then repeats this process until no state-
// based actions are performed."
//
// The 2026-02-27 file lists sub-letters §704.5a–§704.5z (skipping l/o per
// the note in the preamble at line 7). Commander-variant SBAs live at
// §704.6c (21-commander-damage loss) and §704.6d (commander return from
// graveyard/exile).
//
// Ordering: CR §704.3 specifies SBAs happen simultaneously within a pass;
// we call helpers sequentially and re-loop until a pass reports no change.
// Cap of 40 passes matches the Python reference.
//
// Each helper returns true iff it mutated state. StateBasedActions (the
// entry point) returns true iff ANY helper reported a mutation.

// StateBasedActions runs every applicable §704.5 / §704.6 state-based action
// in a loop until no further changes occur (CR §704.3), capped at 40 passes
// as a safety net against malformed card interactions.
//
// Returns true iff at least one SBA fired during the call. Caller (Phase 5
// priority pump) is expected to invoke this whenever a player would get
// priority.
func StateBasedActions(gs *GameState) bool {
	if gs == nil {
		return false
	}
	// Stack trace: log SBA check for CR audit.
	GlobalStackTrace.Log("sba_check", "", -1, len(gs.Stack), "checking_sbas")
	const maxPasses = 40 // CR §704.3 safety cap (matches Python).
	anyChange := false
	passes := 0
	for passes < maxPasses {
		passes++
		changed := false

		// Player-loss SBAs (§704.5a–§704.5c).
		if sba704_5a(gs) {
			changed = true
		}
		if sba704_5b(gs) {
			changed = true
		}
		if sba704_5c(gs) {
			changed = true
		}

		// Zone-existence / copy SBAs (§704.5d–§704.5e).
		if sba704_5d(gs) {
			changed = true
		}
		if sba704_5e(gs) {
			changed = true
		}

		// Creature / planeswalker death SBAs (§704.5f–§704.5i).
		if sba704_5f(gs) {
			changed = true
		}
		if sba704_5g(gs) {
			changed = true
		}
		sba704_5h(gs) // stub — deathtouch tracking not wired.
		if sba704_5i(gs) {
			changed = true
		}

		// Duplicate / supertype SBAs (§704.5j–§704.5k).
		if sba704_5j(gs) {
			changed = true
		}
		if sba704_5k(gs) {
			changed = true
		}

		// Attachment SBAs (§704.5m–§704.5p).
		if sba704_5m(gs) {
			changed = true
		}
		if sba704_5n(gs) {
			changed = true
		}
		if sba704_5p(gs) {
			changed = true
		}

		// Counter SBAs (§704.5q–§704.5r).
		if sba704_5q(gs) {
			changed = true
		}
		if sba704_5r(gs) {
			changed = true
		}

		// Saga / Dungeon / Battle / Role / Speed (§704.5s–§704.5z).
		if sba704_5s(gs) {
			changed = true
		}
		sba704_5t(gs) // stub — dungeons.
		sba704_5u(gs) // stub — space sculptor / sector designation.
		if sba704_5v(gs) {
			changed = true
		}
		sba704_5w(gs) // stub — battle protectors.
		sba704_5x(gs) // stub — siege protector reset.
		if sba704_5y(gs) {
			changed = true
		}
		sba704_5z(gs) // stub — Start Your Engines speed init.

		// Variant-specific SBAs (§704.6). Commander clauses are gated on
		// gs.CommanderFormat; non-commander games short-circuit at helper
		// entry.
		if sba704_6c(gs) {
			changed = true
		}
		if sba704_6d(gs) {
			changed = true
		}

		if !changed {
			break
		}
		anyChange = true
	}

	if passes >= maxPasses {
		gs.LogEvent(Event{
			Kind:   "sba_cap_hit",
			Seat:   -1,
			Target: -1,
			Details: map[string]interface{}{
				"rule":   "704.3",
				"reason": "exceeded_max_passes",
				"passes": passes,
			},
		})
		// CR §104.4b: if the game detects a loop that no player can
		// break, the game is a draw. When SBAs loop at the cap, it
		// indicates a mandatory loop (e.g. Worldgorger Dragon + Animate
		// Dead with no other targets). Flag the game as a draw.
		gs.LogEvent(Event{
			Kind:   "infinite_loop_draw",
			Seat:   -1,
			Target: -1,
			Details: map[string]interface{}{
				"rule":   "104.4b",
				"reason": "mandatory_loop_detected_via_sba_cap",
				"passes": passes,
			},
		})
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["game_draw"] = 1
		gs.Flags["ended"] = 1
	}
	if anyChange {
		gs.LogEvent(Event{
			Kind:   "sba_cycle_complete",
			Seat:   -1,
			Target: -1,
			Details: map[string]interface{}{
				"passes": passes,
			},
		})
	}
	// Sync legacy ManaPool with typed pool. Many code paths deduct from
	// ManaPool directly (seat.ManaPool -= cost) without updating the typed
	// pool. Reconcile: if ManaPool < Mana.Total(), drain the difference
	// from the typed pool's generic/any buckets.
	for _, s := range gs.Seats {
		if s == nil || s.Mana == nil {
			continue
		}
		typed := s.Mana.Total()
		if s.ManaPool < typed {
			deficit := typed - s.ManaPool
			// Drain from any → colorless → colors
			for deficit > 0 && s.Mana.Any > 0 {
				s.Mana.Any--
				deficit--
			}
			for deficit > 0 && s.Mana.C > 0 {
				s.Mana.C--
				deficit--
			}
			for deficit > 0 && s.Mana.R > 0 {
				s.Mana.R--
				deficit--
			}
			for deficit > 0 && s.Mana.G > 0 {
				s.Mana.G--
				deficit--
			}
			for deficit > 0 && s.Mana.B > 0 {
				s.Mana.B--
				deficit--
			}
			for deficit > 0 && s.Mana.U > 0 {
				s.Mana.U--
				deficit--
			}
			for deficit > 0 && s.Mana.W > 0 {
				s.Mana.W--
				deficit--
			}
		} else if s.ManaPool > typed {
			// ManaPool was added to directly — sync typed to match
			s.ManaPool = typed
		}
	}

	return anyChange
}

// -----------------------------------------------------------------------------
// §704.5a — life ≤ 0 → loss.
// -----------------------------------------------------------------------------

// sba704_5a: "If a player has 0 or less life, that player loses the game."
// (CR §704.5a, rules file line 5453.)
func sba704_5a(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Life <= 0 && !s.SBA704_5a_emitted {
			// §614: would_lose_game — Platinum Angel cancels.
			if FireLoseGameEvent(gs, s.Idx) {
				// Don't emit the SBA event or mark Lost; Platinum Angel
				// kept the player alive at negative life.
				continue
			}
			s.SBA704_5a_emitted = true
			gs.LogEvent(Event{
				Kind:   "sba_704_5a",
				Seat:   s.Idx,
				Target: -1,
				Amount: s.Life,
				Details: map[string]interface{}{
					"rule":   "704.5a",
					"reason": "life_total_zero_or_less",
				},
			})
			if !s.Lost {
				s.Lost = true
				s.LossReason = "life total 0 or less (CR 704.5a)"
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5b — drew from empty library → loss.
// -----------------------------------------------------------------------------

// sba704_5b: "If a player attempted to draw a card from a library with no
// cards in it since the last time state-based actions were checked, that
// player loses the game." (CR §704.5b, rules file line 5455.)
func sba704_5b(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil || !s.AttemptedEmptyDraw {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "sba_704_5b",
			Seat:   s.Idx,
			Target: -1,
			Details: map[string]interface{}{
				"rule":   "704.5b",
				"reason": "draw_from_empty_library",
			},
		})
		if !s.Lost {
			s.Lost = true
			s.LossReason = "drew from empty library (CR 704.5b)"
		}
		s.AttemptedEmptyDraw = false
		changed = true
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5c — 10+ poison counters → loss.
// -----------------------------------------------------------------------------

// sba704_5c: "If a player has ten or more poison counters, that player loses
// the game." (CR §704.5c, rules file line 5457.)
func sba704_5c(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.PoisonCounters >= 10 {
			s.Lost = true
			s.LossReason = "ten or more poison counters (CR 704.5c)"
			gs.LogEvent(Event{
				Kind:   "sba_704_5c",
				Seat:   s.Idx,
				Target: -1,
				Amount: s.PoisonCounters,
				Details: map[string]interface{}{
					"rule":   "704.5c",
					"reason": "poison_counters",
				},
			})
			changed = true
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5d — token in non-battlefield zone → ceases to exist.
// -----------------------------------------------------------------------------

// sba704_5d: "If a token is in a zone other than the battlefield, it ceases
// to exist." (CR §704.5d, rules file line 5459.)
//
// Tokens are identified via the "token" entry in Card.Types (the resolver's
// CreateToken handler tags them at spawn).
func sba704_5d(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, zoneName := range []string{"hand", "graveyard", "exile", "library"} {
			var zone *[]*Card
			switch zoneName {
			case "hand":
				zone = &s.Hand
			case "graveyard":
				zone = &s.Graveyard
			case "exile":
				zone = &s.Exile
			case "library":
				zone = &s.Library
			}
			removed := 0
			kept := (*zone)[:0]
			// Iterate explicitly so we can rebuild in place.
			src := *zone
			for _, c := range src {
				if cardIsToken(c) {
					removed++
					continue
				}
				kept = append(kept, c)
			}
			if removed > 0 {
				*zone = kept
				changed = true
				gs.LogEvent(Event{
					Kind:   "sba_704_5d",
					Seat:   s.Idx,
					Target: -1,
					Amount: removed,
					Details: map[string]interface{}{
						"rule":   "704.5d",
						"reason": "token_in_nonbattlefield_zone",
						"zone":   zoneName,
					},
				})
			}
		}
	}
	return changed
}

// cardIsToken mirrors the Permanent.IsToken check at the Card level (used
// for zone sweeps that see loose Card entries, not Permanents).
func cardIsToken(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "token" {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// §704.5e — spell/card copies outside legal zones → cease to exist.
// -----------------------------------------------------------------------------

// sba704_5e: "If a copy of a spell is in a zone other than the stack, it
// ceases to exist. If a copy of a card is in any zone other than the stack
// or the battlefield, it ceases to exist." (CR §704.5e, rules file line 5461.)
//
// Copies are flagged via Card.IsCopy (set by resolveCopySpell, storm copies,
// cascade copies, etc.). This SBA sweeps hand, graveyard, exile, and library
// for copy-flagged cards and removes them — they were never real cards in any
// deck, so zone conservation doesn't apply.
func sba704_5e(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Sweep hand.
		if n := removeCopiesFromZone(&s.Hand); n > 0 {
			gs.LogEvent(Event{
				Kind: "sba_704_5e", Seat: s.Idx,
				Amount: n,
				Details: map[string]interface{}{
					"zone": "hand", "rule": "704.5e",
				},
			})
			changed = true
		}
		// Sweep graveyard.
		if n := removeCopiesFromZone(&s.Graveyard); n > 0 {
			gs.LogEvent(Event{
				Kind: "sba_704_5e", Seat: s.Idx,
				Amount: n,
				Details: map[string]interface{}{
					"zone": "graveyard", "rule": "704.5e",
				},
			})
			changed = true
		}
		// Sweep exile.
		if n := removeCopiesFromZone(&s.Exile); n > 0 {
			gs.LogEvent(Event{
				Kind: "sba_704_5e", Seat: s.Idx,
				Amount: n,
				Details: map[string]interface{}{
					"zone": "exile", "rule": "704.5e",
				},
			})
			changed = true
		}
		// Sweep library.
		if n := removeCopiesFromZone(&s.Library); n > 0 {
			gs.LogEvent(Event{
				Kind: "sba_704_5e", Seat: s.Idx,
				Amount: n,
				Details: map[string]interface{}{
					"zone": "library", "rule": "704.5e",
				},
			})
			changed = true
		}
	}
	return changed
}

// removeCopiesFromZone removes all Card entries with IsCopy==true from the
// given zone slice in-place. Returns the count removed. Used by sba704_5e.
func removeCopiesFromZone(zone *[]*Card) int {
	if zone == nil || len(*zone) == 0 {
		return 0
	}
	removed := 0
	kept := (*zone)[:0]
	for _, c := range *zone {
		if c != nil && c.IsCopy {
			removed++
			continue
		}
		kept = append(kept, c)
	}
	*zone = kept
	return removed
}

// -----------------------------------------------------------------------------
// §704.5f — creature toughness ≤ 0 → graveyard.
// -----------------------------------------------------------------------------

// sba704_5f: "If a creature has toughness 0 or less, it's put into its
// owner's graveyard. Regeneration can't replace this event."
// (CR §704.5f, rules file line 5463.)
func sba704_5f(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Snapshot the battlefield: destroyPerm mutates it.
		for _, p := range snapshotBattlefield(s) {
			if p.PhasedOut {
				continue // §702.26 — phased-out permanents don't exist
			}
			// Phase 8: layer-aware creature query. Opalescence-granted
			// creatures with toughness 0 die here via §704.5f.
			if !gs.IsCreatureOf(p) {
				continue
			}
			if gs.ToughnessOf(p) <= 0 {
				destroyPermSBA(gs, p, "toughness_zero_or_less", "704.5f")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5g — creature lethal damage → destroy (regen ok, indestructible skips).
// -----------------------------------------------------------------------------

// sba704_5g: "If a creature has toughness greater than 0, it has damage
// marked on it, and the total damage marked on it is greater than or equal
// to its toughness, that creature has been dealt lethal damage and is
// destroyed." (CR §704.5g, rules file line 5465; §702.12b for indestructible.)
func sba704_5g(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if p.PhasedOut {
				continue // §702.26 — phased-out permanents don't exist
			}
			// Phase 8: layer-aware creature + toughness queries.
			if !gs.IsCreatureOf(p) {
				continue
			}
			t := gs.ToughnessOf(p)
			if t > 0 && p.MarkedDamage >= t {
				if p.IsIndestructible() {
					continue // §702.12b
				}
				destroyPermSBA(gs, p, "lethal_damage", "704.5g")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5h — deathtouch damage kill.
// -----------------------------------------------------------------------------

// sba704_5h: "If a creature has toughness greater than 0, and it's been
// dealt damage by a source with deathtouch since the last time state-based
// actions were checked, that creature is destroyed." (CR §704.5h, rules
// file line 5467.)
//
// Implemented: checks the Permanent.Flags["deathtouch_damaged"] flag, which
// is set by combat damage assignment (applyCombatDamageToCreature) when the
// source has deathtouch. The flag is consumed (cleared) by this SBA check
// so it fires once per damage event.
func sba704_5h(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if !gs.IsCreatureOf(p) {
				continue
			}
			if p.PhasedOut {
				continue
			}
			if p.Flags == nil {
				continue
			}
			if p.Flags["deathtouch_damaged"] <= 0 {
				continue
			}
			// Clear the flag so we don't double-fire.
			delete(p.Flags, "deathtouch_damaged")
			t := gs.ToughnessOf(p)
			if t <= 0 {
				continue // §704.5f already handles this.
			}
			if p.MarkedDamage <= 0 {
				continue // no actual damage on the creature
			}
			// §704.5h — any nonzero damage from a deathtouch source is lethal.
			if p.IsIndestructible() {
				continue // §702.12b
			}
			destroyPermSBA(gs, p, "deathtouch_damage", "704.5h")
			changed = true
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5i — planeswalker 0 loyalty → graveyard.
// -----------------------------------------------------------------------------

// sba704_5i: "If a planeswalker has loyalty 0, it's put into its owner's
// graveyard." (CR §704.5i, rules file line 5469.)
//
// Note: Python's implementation lazy-initializes a missing "loyalty" key
// (ETB init safety net). We replicate: a planeswalker with NO "loyalty"
// counter key is assumed to not-yet-initialized and is skipped this pass.
// A key with value ≤ 0 dies.
func sba704_5i(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if p.PhasedOut {
				continue
			}
			if !p.IsPlaneswalker() {
				continue
			}
			if p.Counters == nil {
				continue
			}
			v, has := p.Counters["loyalty"]
			if !has {
				continue // ETB init pending; don't destroy.
			}
			if v <= 0 {
				destroyPermSBA(gs, p, "zero_loyalty", "704.5i")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5j — Legend Rule. Keeper = earliest timestamp (Python policy).
// -----------------------------------------------------------------------------

// sba704_5j: "If two or more legendary permanents with the same name are
// controlled by the same player, that player chooses one of them, and the
// rest are put into their owners' graveyards." (CR §704.5j, rules file line
// 5471.) Keeper policy: earliest timestamp (first-cast stays), per Python
// reference _sba_704_5j.
func sba704_5j(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		groups := map[string][]*Permanent{}
		for _, p := range s.Battlefield {
			if p.PhasedOut {
				continue
			}
			if !p.IsLegendary() {
				continue
			}
			name := p.Card.DisplayName()
			groups[name] = append(groups[name], p)
		}
		for name, perms := range groups {
			if len(perms) < 2 {
				continue
			}
			// Keeper = earliest timestamp; destroy the rest.
			keeper := perms[0]
			for _, q := range perms[1:] {
				if q.Timestamp < keeper.Timestamp {
					keeper = q
				}
			}
			destroyedCount := 0
			for _, q := range perms {
				if q == keeper {
					continue
				}
				destroyPermSBA(gs, q, "legend_rule", "704.5j")
				destroyedCount++
			}
			if destroyedCount > 0 {
				changed = true
				gs.LogEvent(Event{
					Kind:   "sba_704_5j_keep",
					Seat:   s.Idx,
					Target: -1,
					Source: name,
					Amount: destroyedCount,
					Details: map[string]interface{}{
						"rule":             "704.5j",
						"keeper_timestamp": keeper.Timestamp,
						"destroyed_count":  destroyedCount,
						"card":             name,
					},
				})
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5k — World Rule. Keeper = newest timestamp (Python policy).
// -----------------------------------------------------------------------------

// sba704_5k: "If two or more permanents have the supertype world, all except
// the one that has had the world supertype for the shortest amount of time
// are put into their owners' graveyards. In the event of a tie for the
// shortest amount of time, all are put into their owners' graveyards."
// (CR §704.5k, rules file line 5473.)
//
// Keeper policy: highest timestamp = shortest time as world (the Phase-3
// engine does not track gain-type effects, so ETB timestamp stands in for
// "time as world"). Mirrors Python reference _sba_704_5k.
func sba704_5k(gs *GameState) bool {
	var worlds []*Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p.PhasedOut {
				continue
			}
			if p.IsWorld() {
				worlds = append(worlds, p)
			}
		}
	}
	if len(worlds) < 2 {
		return false
	}
	newest := worlds[0].Timestamp
	newestCount := 0
	for _, w := range worlds {
		if w.Timestamp > newest {
			newest = w.Timestamp
		}
	}
	for _, w := range worlds {
		if w.Timestamp == newest {
			newestCount++
		}
	}
	changed := false
	if newestCount > 1 {
		// Ambiguous newest — all worlds die.
		for _, p := range worlds {
			destroyPermSBA(gs, p, "world_rule_tie", "704.5k")
			changed = true
		}
	} else {
		for _, p := range worlds {
			if p.Timestamp != newest {
				destroyPermSBA(gs, p, "world_rule", "704.5k")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5m — Aura attached illegally → graveyard.
// -----------------------------------------------------------------------------

// sba704_5m: "If an Aura is attached to an illegal object or player, or is
// not attached to an object or player, that Aura is put into its owner's
// graveyard." (CR §704.5m, rules file line 5475.)
//
// Two paths:
//   1. Gate-based (legacy): Auras with Flags["aura_expects_attach"]==1 that
//      have nil AttachedTo or whose AttachedTo left the battlefield.
//   2. Orphan detection: any Aura whose AttachedTo was non-nil but the target
//      permanent is no longer on the battlefield (attachment broken by removal).
//
// Auras that were never attached (no flag, nil AttachedTo) are still skipped
// to avoid wiping Auras before the resolver's attach path runs.
func sba704_5m(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if !p.IsAura() {
				continue
			}
			if p.PhasedOut {
				continue
			}
			// An Aura on the battlefield with no legal attachment is illegal.
			// §704.5m applies whether the Aura was explicitly wired or not.
			if p.AttachedTo == nil || !permanentOnBattlefield(gs, p.AttachedTo) {
				destroyPermSBA(gs, p, "aura_illegal_attach", "704.5m")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5n — Equipment / Fortification illegally attached → unattach.
// -----------------------------------------------------------------------------

// sba704_5n: "If an Equipment or Fortification is attached to an illegal
// permanent or to a player, it becomes unattached from that permanent or
// player. It remains on the battlefield." (CR §704.5n, rules file line 5477.)
func sba704_5n(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if !(p.IsEquipment() || p.IsFortification()) {
				continue
			}
			if p.AttachedTo == nil {
				continue
			}
			tgt := p.AttachedTo
			reason := ""
			if !permanentOnBattlefield(gs, tgt) {
				reason = "equipment_target_gone"
			} else if p.IsEquipment() && !tgt.IsCreature() {
				// §301.5 — Equipment attaches to creatures.
				reason = "equipment_non_creature"
			} else if p.IsFortification() && !tgt.IsLand() {
				// §301.5a — Fortification attaches to lands.
				reason = "fortification_non_land"
			}
			if reason == "" {
				continue
			}
			p.AttachedTo = nil
			changed = true
			gs.LogEvent(Event{
				Kind:   "sba_704_5n",
				Seat:   p.Controller,
				Target: -1,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule":   "704.5n",
					"reason": reason,
					"card":   p.Card.DisplayName(),
				},
			})
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5p — creature/battle/other attached → unattach (stays on battlefield).
// -----------------------------------------------------------------------------

// sba704_5p: "If a battle or creature is attached to an object or player, it
// becomes unattached and remains on the battlefield. Similarly, if any
// nonbattle, noncreature permanent that's neither an Aura, an Equipment,
// nor a Fortification is attached to an object or player, it becomes
// unattached and remains on the battlefield." (CR §704.5p, rules file line
// 5479.)
func sba704_5p(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p.AttachedTo == nil {
				continue
			}
			// Auras, Equipment, and Fortifications legitimately attach.
			if p.IsAura() || p.IsEquipment() || p.IsFortification() {
				continue
			}
			p.AttachedTo = nil
			changed = true
			gs.LogEvent(Event{
				Kind:   "sba_704_5p",
				Seat:   p.Controller,
				Target: -1,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule":   "704.5p",
					"reason": "non_attachable_was_attached",
					"card":   p.Card.DisplayName(),
				},
			})
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5q — +1/+1 and -1/-1 counters annihilate 1-for-1.
// -----------------------------------------------------------------------------

// sba704_5q: "If a permanent has both a +1/+1 counter and a -1/-1 counter on
// it, N +1/+1 and N -1/-1 counters are removed from it, where N is the
// smaller of the number of +1/+1 and -1/-1 counters on it." (CR §704.5q,
// rules file line 5481.)
func sba704_5q(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p.Counters == nil {
				continue
			}
			plus := p.Counters["+1/+1"]
			minus := p.Counters["-1/-1"]
			if plus > 0 && minus > 0 {
				n := plus
				if minus < n {
					n = minus
				}
				p.Counters["+1/+1"] = plus - n
				p.Counters["-1/-1"] = minus - n
				if p.Counters["+1/+1"] == 0 {
					delete(p.Counters, "+1/+1")
				}
				if p.Counters["-1/-1"] == 0 {
					delete(p.Counters, "-1/-1")
				}
				changed = true
				gs.LogEvent(Event{
					Kind:   "sba_704_5q",
					Seat:   p.Controller,
					Target: -1,
					Source: p.Card.DisplayName(),
					Amount: n,
					Details: map[string]interface{}{
						"rule":          "704.5q",
						"card":          p.Card.DisplayName(),
						"removed_plus":  n,
						"removed_minus": n,
					},
				})
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5r — "can't have more than N counters of X" → trim excess.
// -----------------------------------------------------------------------------

// sba704_5r: "If a permanent with an ability that says it can't have more
// than N counters of a certain kind on it has more than N counters of that
// kind on it, all but N of those counters are removed from it."
// (CR §704.5r, rules file line 5483.)
func sba704_5r(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p.Counters == nil || p.Card == nil || p.Card.AST == nil {
				continue
			}
			for _, ab := range p.Card.AST.Abilities {
				st, ok := ab.(*gameast.Static)
				if !ok || st.Raw == "" {
					continue
				}
				maxN, counterKind := parseCounterLimit(st.Raw)
				if maxN < 0 {
					continue
				}
				current := p.Counters[counterKind]
				if current > maxN {
					excess := current - maxN
					p.Counters[counterKind] = maxN
					if maxN == 0 {
						delete(p.Counters, counterKind)
					}
					changed = true
					gs.LogEvent(Event{
						Kind:   "sba_704_5r",
						Seat:   p.Controller,
						Target: -1,
						Source: p.Card.DisplayName(),
						Amount: excess,
						Details: map[string]interface{}{
							"rule":         "704.5r",
							"card":         p.Card.DisplayName(),
							"counter_kind": counterKind,
							"max":          maxN,
							"removed":      excess,
						},
					})
				}
			}
		}
	}
	return changed
}

var counterLimitWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
}

func parseCounterLimit(raw string) (maxN int, counterKind string) {
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "can't have more than ")
	if idx < 0 {
		return -1, ""
	}
	rest := lower[idx+len("can't have more than "):]
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return -1, ""
	}
	n, ok := counterLimitWords[parts[0]]
	if !ok {
		n64, err := strconv.Atoi(parts[0])
		if err != nil {
			return -1, ""
		}
		n = n64
	}
	kind := parts[1]
	if strings.HasSuffix(kind, "counters") || strings.HasSuffix(kind, "counter") {
		return -1, ""
	}
	return n, kind
}

// -----------------------------------------------------------------------------
// §704.5s — Saga with lore counters ≥ final chapter → sacrifice.
// -----------------------------------------------------------------------------

// sba704_5s: "If the number of lore counters on a Saga permanent with one
// or more chapter abilities is greater than or equal to its final chapter
// number and it isn't the source of a chapter ability that has triggered
// but not yet left the stack, that Saga's controller sacrifices it."
// (CR §704.5s, rules file line 5485.)
//
// Scope: we use Permanent.Counters["saga_final_chapter"] as the per-card
// final-chapter value (Phase 7 Saga ETB hook will set this; Python mines
// the oracle text). Without a final-chapter record, we skip. This
// preserves correctness: Sagas without chapter tracking simply don't fire.
func sba704_5s(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if !p.IsSaga() {
				continue
			}
			if p.Counters == nil {
				continue
			}
			final, has := p.Counters["saga_final_chapter"]
			if !has || final <= 0 {
				continue // no tracked final-chapter; skip.
			}
			lore := p.Counters["lore"]
			if lore >= final {
				sacrificePermSBA(gs, p, "saga_final_chapter", "704.5s",
					map[string]interface{}{
						"lore":          lore,
						"final_chapter": final,
					})
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5t — dungeon venture bottom → remove from game.
// -----------------------------------------------------------------------------

// sba704_5t: "If a player's venture marker is on the bottommost room of a
// dungeon card, and that dungeon card isn't the source of a room ability
// that has triggered but not yet left the stack, the dungeon card's owner
// removes it from the game." (CR §704.5t, rules file line 5487.)
//
// STUB — dungeons aren't in the 4-deck tournament cardpool. Mirrors Python
// stub (_sba_704_5t).
func sba704_5t(gs *GameState) bool {
	// §704.5t: "If a player has completed a dungeon and that dungeon is
	// the source of a room ability that has triggered but not yet left
	// the stack, that dungeon can't leave the command zone. Otherwise,
	// if a player has completed a dungeon, that dungeon is removed from
	// the game."
	//
	// MVP implementation: check each seat for a "dungeon_completed" flag.
	// If set, emit a dungeon_completed event and remove the dungeon card
	// from the command zone.
	if gs == nil {
		return false
	}
	changed := false
	for _, seat := range gs.Seats {
		if seat == nil || seat.Flags == nil {
			continue
		}
		if seat.Flags["dungeon_completed"] > 0 {
			gs.LogEvent(Event{
				Kind: "dungeon_completed",
				Seat: seat.Idx,
				Details: map[string]interface{}{
					"rule": "704.5t",
				},
			})
			delete(seat.Flags, "dungeon_completed")
			changed = true
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5u — space sculptor sector designation.
// -----------------------------------------------------------------------------

// sba704_5u: "If a permanent with space sculptor and any creatures without
// a sector designation are on the battlefield, each player who controls one
// or more of those creatures and doesn't control a permanent with space
// sculptor chooses a sector designation for each of those creatures they
// control. Then, each other player who controls one or more of those
// creatures chooses a sector designation for each of those creatures they
// control." (CR §704.5u, rules file line 5489.)
//
// STUB — Space Sculptor (Unfinity keyword) is not in scope for the 4-deck
// tournament. Mirrors Python stub (_sba_704_5u).
func sba704_5u(gs *GameState) bool {
	// TODO: implement if Unfinity cards land in a future expansion.
	_ = gs
	return false
}

// -----------------------------------------------------------------------------
// §704.5v — battle with 0 defense → graveyard.
// -----------------------------------------------------------------------------

// sba704_5v: "If a battle has defense 0 and it isn't the source of an
// ability that has triggered but not yet left the stack, it's put into its
// owner's graveyard." (CR §704.5v, rules file line 5491.)
func sba704_5v(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range snapshotBattlefield(s) {
			if !p.IsBattle() {
				continue
			}
			def := 0
			if p.Counters != nil {
				def = p.Counters["defense"]
			}
			if def <= 0 {
				destroyPermSBA(gs, p, "battle_defense_zero", "704.5v")
				changed = true
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5w — battle without protector.
// -----------------------------------------------------------------------------

// sba704_5w: "If a battle has no player in the game designated as its
// protector and no attacking creatures are currently attacking that battle,
// that battle's controller chooses an appropriate player to be its
// protector based on its battle type. If no player can be chosen this way,
// the battle is put into its owner's graveyard." (CR §704.5w, rules file
// line 5493.)
//
// STUB — battle protectors aren't modeled yet. Mirrors Python stub
// (_sba_704_5w).
func sba704_5w(gs *GameState) bool {
	// TODO: model battle protectors.
	_ = gs
	return false
}

// -----------------------------------------------------------------------------
// §704.5x — siege controller = protector reset.
// -----------------------------------------------------------------------------

// sba704_5x: "If a Siege's controller is also its designated protector,
// that player chooses an opponent to become its protector. If no player
// can be chosen this way, the battle is put into its owner's graveyard."
// (CR §704.5x, rules file line 5495.)
//
// STUB — Siege subtype + protector state not modeled. Mirrors Python stub
// (_sba_704_5x).
func sba704_5x(gs *GameState) bool {
	// TODO: model Siege subtype + protector state.
	_ = gs
	return false
}

// -----------------------------------------------------------------------------
// §704.5y — Role uniqueness (keep newest timestamp per controller).
// -----------------------------------------------------------------------------

// sba704_5y: "If a permanent has more than one Role controlled by the same
// player attached to it, each of those Roles except the one with the most
// recent timestamp is put into its owner's graveyard."
// (CR §704.5y, rules file line 5497.)
func sba704_5y(gs *GameState) bool {
	changed := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, holder := range s.Battlefield {
			// Find Roles attached to holder, grouped by controller.
			rolesByCtrl := map[int][]*Permanent{}
			for _, s2 := range gs.Seats {
				if s2 == nil {
					continue
				}
				for _, r := range s2.Battlefield {
					if r.IsRole() && r.AttachedTo == holder {
						rolesByCtrl[r.Controller] = append(rolesByCtrl[r.Controller], r)
					}
				}
			}
			for _, roles := range rolesByCtrl {
				if len(roles) < 2 {
					continue
				}
				// Keep newest (highest timestamp); destroy rest.
				newest := roles[0]
				for _, r := range roles[1:] {
					if r.Timestamp > newest.Timestamp {
						newest = r
					}
				}
				for _, r := range roles {
					if r == newest {
						continue
					}
					destroyPermSBA(gs, r, "role_uniqueness", "704.5y")
					changed = true
				}
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.5z — Start Your Engines speed initialization.
// -----------------------------------------------------------------------------

// sba704_5z: "If a player controls a permanent with start your engines! and
// that player has no speed, that player's speed becomes 1."
// (CR §704.5z, rules file line 5499.)
//
// STUB — Speed mechanic not modeled. Mirrors Python stub (_sba_704_5z).
func sba704_5z(gs *GameState) bool {
	// TODO: implement when Speed / Max Speed cards land.
	_ = gs
	return false
}

// -----------------------------------------------------------------------------
// §704.6c — 21+ commander damage → loss.
// -----------------------------------------------------------------------------

// sba704_6c: "In a Commander game, a player who's been dealt 21 or more
// combat damage by the same commander over the course of the game loses
// the game." (CR §704.6c, rules file line 5507; §903.10a.)
//
// Relies on Phase 4's combat damage handler to maintain
// Seat.CommanderDamage[dealer_seat][commander_name]. This helper walks
// the nested map and checks the 21-threshold per (dealer, name) bucket.
//
// Partner support: Seat.CommanderDamage is keyed by
// (dealerSeat, commanderName) so Kraum+Tymna each accrue independently.
// A pilot dealt 15 by Kraum AND 15 by Tymna has NOT lost — no single
// bucket crossed 21.
func sba704_6c(gs *GameState) bool {
	if !gs.CommanderFormat {
		return false
	}
	changed := false
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for dealer, byName := range s.CommanderDamage {
			for name, dmg := range byName {
				if dmg >= 21 {
					s.Lost = true
					s.LossReason = "21+ commander damage from " + name + " (CR 704.6c)"
					gs.LogEvent(Event{
						Kind:   "sba_704_6c",
						Seat:   s.Idx,
						Target: -1,
						Source: name,
						Amount: dmg,
						Details: map[string]interface{}{
							"rule":         "704.6c+903.10a",
							"commander":    name,
							"dealer_seat":  dealer,
							"total_damage": dmg,
						},
					})
					changed = true
					break
				}
			}
			if changed {
				break
			}
		}
	}
	return changed
}

// -----------------------------------------------------------------------------
// §704.6d — commander in graveyard/exile → may go to command zone.
// -----------------------------------------------------------------------------

// sba704_6d: "In a Commander game, if a commander is in a graveyard or in
// exile and that object was put into that zone since the last time
// state-based actions were checked, its owner may put it into the command
// zone." (CR §704.6d, rules file line 5509; §903.9a.)
//
// Policy: greedy-allow (every matching commander returns). This mirrors
// Python's _sba_704_6d. A seat's commander identity is name-keyed via
// Seat.CommanderNames.
func sba704_6d(gs *GameState) bool {
	if !gs.CommanderFormat {
		return false
	}
	changed := false
	for _, s := range gs.Seats {
		if s == nil || len(s.CommanderNames) == 0 {
			continue
		}
		// Graveyard.
		kept := s.Graveyard[:0]
		for _, c := range s.Graveyard {
			if c != nil && isCommanderName(s.CommanderNames, c.DisplayName()) {
				s.CommandZone = append(s.CommandZone, c)
				gs.LogEvent(Event{
					Kind:   "sba_704_6d",
					Seat:   s.Idx,
					Target: -1,
					Source: c.DisplayName(),
					Details: map[string]interface{}{
						"rule":      "704.6d+903.9a",
						"card":      c.DisplayName(),
						"from_zone": "graveyard",
					},
				})
				changed = true
				continue
			}
			kept = append(kept, c)
		}
		s.Graveyard = kept
		// Exile.
		kept = s.Exile[:0]
		for _, c := range s.Exile {
			if c != nil && isCommanderName(s.CommanderNames, c.DisplayName()) {
				s.CommandZone = append(s.CommandZone, c)
				gs.LogEvent(Event{
					Kind:   "sba_704_6d",
					Seat:   s.Idx,
					Target: -1,
					Source: c.DisplayName(),
					Details: map[string]interface{}{
						"rule":      "704.6d+903.9a",
						"card":      c.DisplayName(),
						"from_zone": "exile",
					},
				})
				changed = true
				continue
			}
			kept = append(kept, c)
		}
		s.Exile = kept
	}
	return changed
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

// snapshotBattlefield returns a copy of s.Battlefield so iteration is safe
// across destroyPermSBA calls that mutate the slice. Reuses a per-seat
// buffer to avoid allocating on every SBA pass.
func snapshotBattlefield(s *Seat) []*Permanent {
	if s == nil || len(s.Battlefield) == 0 {
		return nil
	}
	n := len(s.Battlefield)
	if cap(s.sbaSnapBuf) < n {
		s.sbaSnapBuf = make([]*Permanent, n, n*2)
	} else {
		s.sbaSnapBuf = s.sbaSnapBuf[:n]
	}
	copy(s.sbaSnapBuf, s.Battlefield)
	return s.sbaSnapBuf
}

// permanentOnBattlefield returns true iff p is currently on its controller's
// battlefield. §704.5m / §704.5n use this to test attach legality.
func permanentOnBattlefield(gs *GameState, p *Permanent) bool {
	if p == nil || p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	for _, q := range gs.Seats[p.Controller].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

// destroyPermSBA moves p from its controller's battlefield to its
// destination zone (default graveyard, possibly exile per Rest in Peace /
// Leyline / Anafenza §614 replacement), detaches any attached permanent,
// and emits the matching sba_<rule> event. Idempotent if p isn't on the
// battlefield.
//
// §614 replacement hook: before the card moves zones, FireDieEvent runs
// the would_die chain. If a handler sets payload["to_zone"] = "exile"
// the card goes to exile instead. If the chain is Cancelled the creature
// stays on the battlefield.
func destroyPermSBA(gs *GameState, p *Permanent, reason, rule string) {
	if p == nil {
		return
	}
	// §614 "would die" replacement — Anafenza/Rest in Peace/Leyline
	// redirect to exile; hypothetical "survive" effects may Cancel.
	repl := FireDieEvent(gs, p)
	if repl.Cancelled {
		return
	}
	destZone := "graveyard"
	if z := repl.String("to_zone"); z != "" {
		destZone = z
	}
	if !gs.removePermanent(p) {
		return
	}
	// Drop replacement registrations so chained events can't target a
	// permanent that's no longer on the battlefield.
	gs.UnregisterReplacementsForPermanent(p)
	// Emit the high-level "destroy" event that the parity probe expects,
	// then the specific SBA event for auditors. Python emits game.ev("destroy")
	// for SBA creature deaths; Go was only emitting sba_704_5f/g.
	gs.LogEvent(Event{
		Kind:   "destroy",
		Seat:   p.Controller,
		Target: -1,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": p.Card.DisplayName(),
			"reason":      reason,
			"rule":        rule,
			"to_zone":     destZone,
		},
	})
	// Emit event FIRST so auditors always see the intent.
	gs.LogEvent(Event{
		Kind:   "sba_" + ruleToEventSuffix(rule),
		Seat:   p.Controller,
		Target: -1,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":    rule,
			"reason":  reason,
			"card":    p.Card.DisplayName(),
			"to_zone": destZone,
		},
	})
	// Unregister continuous effects too (layers).
	gs.UnregisterContinuousEffectsForPermanent(p)
	// Tokens cease to exist — don't add them to graveyard (§704.5d would
	// clean them up anyway, but skipping the zone write is tidier).
	if !p.IsToken() {
		finalZone := FireZoneChange(gs, p, p.Card, p.Card.Owner, "battlefield", destZone)
		// Fire dies/LTB triggers — §603.6 / §603.10.
		FireZoneChangeTriggers(gs, p, p.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, p, p.Card, "battlefield", destZone)
	}
	// Detach anything that was attached to p so §704.5n/§704.5p don't
	// mis-fire next pass.
	detachAll(gs, p)
}

// sacrificePermSBA mirrors destroyPermSBA but uses "sba_<rule>_sacrifice"
// semantics for Saga §704.5s, which says "sacrifice" rather than "destroy."
func sacrificePermSBA(gs *GameState, p *Permanent, reason, rule string, extra map[string]interface{}) {
	if p == nil {
		return
	}
	// Run §614 "would die" replacement chain for sacrifice SBAs too.
	repl := FireDieEvent(gs, p)
	if repl.Cancelled {
		return
	}
	destZone := "graveyard"
	if z := repl.String("to_zone"); z != "" {
		destZone = z
	}
	if !gs.removePermanent(p) {
		return
	}
	gs.UnregisterReplacementsForPermanent(p)
	gs.UnregisterContinuousEffectsForPermanent(p)
	details := map[string]interface{}{
		"rule":    rule,
		"reason":  reason,
		"card":    p.Card.DisplayName(),
		"to_zone": destZone,
	}
	for k, v := range extra {
		details[k] = v
	}
	gs.LogEvent(Event{
		Kind:    "sba_" + ruleToEventSuffix(rule) + "_saga",
		Seat:    p.Controller,
		Target:  -1,
		Source:  p.Card.DisplayName(),
		Details: details,
	})
	if !p.IsToken() {
		finalZone := FireZoneChange(gs, p, p.Card, p.Card.Owner, "battlefield", destZone)
		FireZoneChangeTriggers(gs, p, p.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, p, p.Card, "battlefield", destZone)
	}
	detachAll(gs, p)
}

// detachAll nils out AttachedTo on every permanent pointing at p, across
// every seat's battlefield. Called by destroy/sacrifice helpers so attach-
// oriented SBAs don't retain dangling references.
func detachAll(gs *GameState, p *Permanent) {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, other := range s.Battlefield {
			if other.AttachedTo == p {
				other.AttachedTo = nil
			}
		}
	}
}

// ruleToEventSuffix converts "704.5j" → "704_5j" for event Kind strings.
func ruleToEventSuffix(rule string) string {
	out := make([]byte, 0, len(rule))
	for i := 0; i < len(rule); i++ {
		c := rule[i]
		if c == '.' {
			out = append(out, '_')
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

// isCommanderName returns true if name matches any entry in the seat's
// CommanderNames list.
func isCommanderName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
