package gameengine

// keywords_battle.go — Battle permanent type (CR §310, §704.5v,
// March of the Machine 2023).
//
// CR §310.1:  Battle is a permanent type. A battle card enters the
//             battlefield with N defense counters where N is the
//             number printed in its lower-right corner (modelled as
//             card.BaseToughness in this engine — the ETB-time
//             initializer in stack.go already stamps this).
// CR §310.5:  Players may attack battles like planeswalkers. The
//             attacking player chooses a battle (or a player, or a
//             planeswalker) as each attacker's defender.
// CR §306.7 / §510.1 (battle-flavored):
//             Combat damage dealt to a battle removes that many
//             defense counters from it.
// CR §704.5v: SBA — a battle with 0 defense counters triggers its
//             "when defeated" ability. The canonical Siege subtype
//             then transforms the battle into the creature face on
//             the opposite side; other subtypes (e.g. Battle —
//             Wager) have their own defeated-payoffs. The per-card
//             "becomes_defeated" trigger covers the diverse
//             payoffs; this file fires the event and clears the
//             flag so SBAs don't loop.
//
// Engine model
// ------------
// The existing stack.go ETB code already initializes the defense
// counter on perm.Counters["defense"] from card.BaseToughness, so a
// battle ETBs with the printed N out of the box. This file adds:
//
//   - Type / counter readers: IsBattle, BattleDefenseCounters.
//   - Counter mutators: AddDefenseCounters, RemoveDefenseCounters
//     (negative-clamp safe). RemoveDefenseCounters automatically
//     fires FireBattleZeroDefense once when the count reaches zero.
//   - "When defeated" hook: FireBattleZeroDefense — fires once per
//     defeat event, idempotent on a perm.Flags["battle_defeated"]
//     latch so the engine never double-resolves a "defeated" trigger.
//   - Attack-target plumbing for the declare-attackers step:
//     SetAttackerDefenderBattle / AttackerDefenderBattle /
//     LookupBattleByTimestamp. The existing AttackerDefender
//     (seat-indexed) is untouched; battle-attacks layer on top via
//     a parallel timestamp-keyed flag.
//   - Combat-damage routing: ApplyCombatDamageToBattle, called by
//     the DealCombatDamageStep when an attacker is pointed at a
//     battle instead of a player seat.

// ---------------------------------------------------------------------------
// Defense-counter key + attack-target flag keys
// ---------------------------------------------------------------------------

const (
	// defenseCounterKey is the perm.Counters key for §310.3 / §704.5v
	// defense counters. Matches the value stamped at ETB in stack.go.
	defenseCounterKey = "defense"

	// flagDefenderBattleTS is the attacker-side flag that records the
	// Permanent.Timestamp of the battle being attacked. Sibling to
	// flagDefenderSeat in combat.go — set by SetAttackerDefenderBattle.
	// Zero / absent means "this attacker is not pointed at a battle";
	// when nonzero, AttackerDefender(seat-style) is ignored and the
	// damage step routes to ApplyCombatDamageToBattle instead of
	// applyCombatDamageToPlayer.
	flagDefenderBattleTS = "defender_battle_ts"

	// battleDefeatedFlag latches once FireBattleZeroDefense has fired
	// for this permanent so a re-trigger (e.g. counter bounced back
	// up then back down again before SBAs sweep) doesn't double-fire
	// the "when defeated" payoff.
	battleDefeatedFlag = "battle_defeated"
)

// ---------------------------------------------------------------------------
// IsBattle / BattleDefenseCounters
// ---------------------------------------------------------------------------

// IsBattle reports whether the permanent is a battle. Type lookup
// is case-insensitive and covers both the printed card type and any
// granted type via the existing cardHasType helper. Safe on nil
// perm or nil card.
func IsBattle(perm *Permanent) bool {
	if perm == nil || perm.Card == nil {
		return false
	}
	return cardHasType(perm.Card, "battle")
}

// BattleDefenseCounters returns the current defense-counter total
// on `perm`. Returns 0 for non-battles, perms with no Counters
// map, or any perm whose defense counter has been zeroed out.
// Negative book values (shouldn't happen with the negative-clamp
// in RemoveDefenseCounters) are normalised to 0.
func BattleDefenseCounters(perm *Permanent) int {
	if perm == nil || perm.Counters == nil {
		return 0
	}
	n := perm.Counters[defenseCounterKey]
	if n < 0 {
		return 0
	}
	return n
}

// ---------------------------------------------------------------------------
// AddDefenseCounters / RemoveDefenseCounters
// ---------------------------------------------------------------------------

// AddDefenseCounters adds `n` defense counters to `perm` (no-op for
// n <= 0). Battle permanents that were previously defeated (latched
// via battle_defeated) DO have the latch cleared by this helper so
// "+N defense counters" + "return from graveyard" combos can re-arm
// the battle correctly.
func AddDefenseCounters(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters[defenseCounterKey] += n
	if perm.Counters[defenseCounterKey] > 0 && perm.Flags != nil {
		delete(perm.Flags, battleDefeatedFlag)
	}
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "defense_counter_added",
		Seat:   perm.Controller,
		Source: cardName,
		Amount: n,
		Details: map[string]interface{}{
			"rule":             "310.3",
			"defense_counters": perm.Counters[defenseCounterKey],
		},
	})
}

// RemoveDefenseCounters removes `n` defense counters from `perm`,
// clamping at zero. If the post-removal count is zero AND the
// battle hasn't been defeated this game yet (battle_defeated latch
// is unset), FireBattleZeroDefense is invoked exactly once.
//
// Returns the number of counters ACTUALLY removed (which may be
// less than `n` if `perm` had fewer counters).
func RemoveDefenseCounters(gs *GameState, perm *Permanent, n int) int {
	if gs == nil || perm == nil || n <= 0 {
		return 0
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	have := perm.Counters[defenseCounterKey]
	if have <= 0 {
		return 0
	}
	remove := n
	if remove > have {
		remove = have
	}
	perm.Counters[defenseCounterKey] = have - remove
	if perm.Counters[defenseCounterKey] <= 0 {
		delete(perm.Counters, defenseCounterKey)
	}
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "defense_counter_removed",
		Seat:   perm.Controller,
		Source: cardName,
		Amount: remove,
		Details: map[string]interface{}{
			"rule":             "310.3",
			"defense_counters": perm.Counters[defenseCounterKey],
		},
	})
	if perm.Counters[defenseCounterKey] <= 0 {
		FireBattleZeroDefense(gs, perm)
	}
	return remove
}

// ---------------------------------------------------------------------------
// FireBattleZeroDefense
// ---------------------------------------------------------------------------

// FireBattleZeroDefense is the "when defeated" hook for §704.5v.
// Fires the per-card "becomes_defeated" trigger so card handlers
// (Sieges with a transform payoff, Wagers with the printed
// payoff text) can react. Idempotent via the battle_defeated latch
// — a perm whose latch is already set is a silent no-op so multi-
// invocation from RemoveDefenseCounters + an external SBA sweep
// doesn't double-fire the payoff.
//
// Safe on non-battle perms (no-op) and nil inputs.
func FireBattleZeroDefense(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !IsBattle(perm) {
		return
	}
	if perm.Flags != nil && perm.Flags[battleDefeatedFlag] > 0 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags[battleDefeatedFlag] = 1

	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "battle_defeated",
		Seat:   perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "704.5v",
		},
	})
	FireCardTrigger(gs, "becomes_defeated", map[string]interface{}{
		"source":          perm,
		"controller_seat": perm.Controller,
	})
}

// IsBattleDefeated reports whether `perm` has been latched as
// defeated. Useful for SBA loops and AI/Hat policy to skip
// permanents whose "becomes_defeated" payoff has already resolved.
func IsBattleDefeated(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags[battleDefeatedFlag] > 0
}

// ---------------------------------------------------------------------------
// Attack-target plumbing — attacker may target a battle
// ---------------------------------------------------------------------------

// SetAttackerDefenderBattle records that `attacker` is attacking
// `battle` (CR §310.5). Stamps attacker.Flags[flagDefenderBattleTS]
// = battle.Timestamp so the damage step can resolve the target via
// LookupBattleByTimestamp. Clears the seat-style defender_seat_p1
// flag so callers don't accidentally route damage to a player too.
//
// The +1 offset convention mirrors flagDefenderSeat — flag absence
// (zero) is distinguishable from a real timestamp of 0. A
// permanent created at NextTimestamp() time = 0 is theoretically
// possible at game start, so the offset is necessary.
//
// No-op for nil attacker, nil battle, non-battle target.
func SetAttackerDefenderBattle(attacker *Permanent, battle *Permanent) {
	if attacker == nil || battle == nil {
		return
	}
	if !IsBattle(battle) {
		return
	}
	if attacker.Flags == nil {
		attacker.Flags = map[string]int{}
	}
	attacker.Flags[flagDefenderBattleTS] = battle.Timestamp + 1
	// Clear seat-style defender so the damage step doesn't see two
	// conflicting destinations.
	delete(attacker.Flags, flagDefenderSeat)
}

// AttackerDefenderBattle returns the Permanent.Timestamp of the
// battle this attacker is attacking, or (-1, false) if it isn't
// pointed at a battle. Callers chain this with
// LookupBattleByTimestamp to resolve the actual *Permanent.
func AttackerDefenderBattle(p *Permanent) (int, bool) {
	if p == nil || p.Flags == nil {
		return -1, false
	}
	v, ok := p.Flags[flagDefenderBattleTS]
	if !ok || v <= 0 {
		return -1, false
	}
	return v - 1, true
}

// LookupBattleByTimestamp walks every seat's battlefield looking
// for a battle permanent whose Timestamp matches `ts`. Returns
// (perm, true) on match, (nil, false) otherwise (battle was bounced
// / exiled / destroyed before the damage step ran). Linear scan;
// battle counts in any real game are O(1) so this is fine.
func LookupBattleByTimestamp(gs *GameState, ts int) (*Permanent, bool) {
	if gs == nil || ts < 0 {
		return nil, false
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil {
				continue
			}
			if p.Timestamp == ts && IsBattle(p) {
				return p, true
			}
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// ApplyCombatDamageToBattle — the damage-step entry point
// ---------------------------------------------------------------------------

// ApplyCombatDamageToBattle applies `amount` combat damage to a
// battle by removing that many defense counters (CR §310.5). If
// the post-damage count reaches zero, FireBattleZeroDefense is
// invoked via RemoveDefenseCounters.
//
// Symmetric with applyCombatDamageToPlayer's interface so the
// damage-step routing in combat.go can swap call sites cleanly.
// No-op for amount <= 0 or non-battle targets.
func ApplyCombatDamageToBattle(gs *GameState, src *Permanent, amount int, battle *Permanent) {
	if gs == nil || src == nil || battle == nil || amount <= 0 {
		return
	}
	if !IsBattle(battle) {
		return
	}
	srcName := ""
	if src.Card != nil {
		srcName = src.Card.DisplayName()
	}
	battleName := ""
	if battle.Card != nil {
		battleName = battle.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "combat_damage_to_battle",
		Seat:   src.Controller,
		Source: srcName,
		Target: battle.Controller,
		Amount: amount,
		Details: map[string]interface{}{
			"battle": battleName,
			"rule":   "310.5",
		},
	})
	RemoveDefenseCounters(gs, battle, amount)
}
