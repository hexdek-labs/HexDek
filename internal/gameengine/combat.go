package gameengine

// Phase 4 — combat phase.
//
// This file implements the §506–§511 combat phase on top of the Phase 3
// GameState. It mirrors the Python reference at scripts/playloop.py —
// specifically combat_phase / declare_attackers / declare_blockers /
// _deal_combat_damage_step / _fire_combat_damage_triggers.
//
// Scope:
//
//   - CombatPhase(gs)                  — full 5-step combat phase, CR §506
//   - DeclareAttackers(gs, seat)       — picks + taps attackers, fires attack triggers
//   - DeclareBlockers(gs, atks, seat)  — greedy chump / lethal-trade block plan
//   - DealCombatDamageStep(...)        — CR §510 damage (FS step + regular step)
//   - EndOfCombatStep(gs)              — clears combat flags + damage wear-off
//
// Per the Phase 4 contract this file also adds a small keyword helper
// (Permanent.HasKeyword) used by combat and, later, by Phase 6 SBAs.
// state.go is read-only for this phase.
//
// Implementation notes:
//   - Combat-state flags ("attacking", "declared_attacker_this_combat",
//     "blocking") live in Permanent.Flags so they survive across the
//     (otherwise stateless) combat function boundary. EndOfCombatStep
//     clears them (CR §506.4).
//   - Keywords come from two places: the AST (a permanent's card carries
//     Keyword abilities parsed from oracle text) and the Flags map for
//     runtime-only grants. HasKeyword scans both.
//   - "Attacks" triggers and "deals combat damage" triggers fire inline
//     via ResolveEffect so Phase 5 (stack) can layer priority later
//     without re-plumbing.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Permanent keyword + combat-flag helpers
// -----------------------------------------------------------------------------

// HasKeyword returns true if this permanent has the named keyword — either
// statically via a Keyword ability on its CardAST, via a granted ability
// recorded in GrantedAbilities, or via a runtime flag on Permanent.Flags
// ("kw:<name>" = 1). Name comparison is lowercase, whitespace-normalized.
//
// This helper is shared with Phase 6 (SBAs read "indestructible" /
// "hexproof"). If both phases added it concurrently, the merge is a no-op
// because the shape matches.
func (p *Permanent) HasKeyword(name string) bool {
	if p == nil {
		return false
	}
	want := strings.ToLower(strings.TrimSpace(name))
	// 1) AST-declared keywords.
	if p.Card != nil && p.Card.AST != nil {
		for _, ab := range p.Card.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				if strings.ToLower(strings.TrimSpace(kw.Name)) == want {
					return true
				}
			}
		}
	}
	// 2) Granted abilities (until-EOT grants, equipment, etc.).
	for _, g := range p.GrantedAbilities {
		if strings.ToLower(strings.TrimSpace(g)) == want {
			return true
		}
	}
	// 3) Runtime flag — how tests and tokens declare keywords without an AST.
	if p.Flags != nil {
		if _, ok := p.Flags["kw:"+want]; ok {
			return true
		}
	}
	// 4) Keyword counters — CR §122.1c: a keyword counter grants that
	// keyword to the permanent it's on. Flying counter = has flying, etc.
	if p.Counters != nil {
		if p.Counters[want] > 0 {
			return true
		}
	}
	return false
}

// permFlag gets/sets a single-bit combat flag on a Permanent. We route
// through Flags rather than extending the struct (state.go is read-only
// for Phase 4).
func permFlag(p *Permanent, key string) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	return p.Flags[key] != 0
}

func setPermFlag(p *Permanent, key string, v bool) {
	if p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	if v {
		p.Flags[key] = 1
	} else {
		delete(p.Flags, key)
	}
}

// Canonical combat-state flag keys.
const (
	flagAttacking          = "attacking"
	flagDeclaredAttacker   = "declared_attacker_this_combat"
	flagBlocking           = "blocking"
	flagAttackedThisCombat = "attacked_this_combat"
	// flagDefenderSeat stores (seat + 1) of the defending player that
	// this attacker is attacking. +1 offset so flag absence (zero) is
	// distinguishable from "seat 0". CR §506.1 — each attacker chooses
	// a defending player or planeswalker it attacks. Multiplayer
	// extension: each attacker may choose a different defender.
	flagDefenderSeat = "defender_seat_p1"
)

// AttackerDefender returns the seat the attacker is currently attacking.
// Returns (-1, false) if the attacker hasn't been assigned a defender.
// CR §506.1 — attacker chooses defender at declare-attackers.
func AttackerDefender(p *Permanent) (int, bool) {
	if p == nil || p.Flags == nil {
		return -1, false
	}
	v, ok := p.Flags[flagDefenderSeat]
	if !ok || v <= 0 {
		return -1, false
	}
	return v - 1, true
}

// SetAttackerDefender is the exported wrapper around setAttackerDefender,
// for per_card handlers creating tokens that enter "tapped and attacking"
// (CR §506.3 token-creation effects).
func SetAttackerDefender(p *Permanent, seatIdx int) {
	setAttackerDefender(p, seatIdx)
}

// setAttackerDefender records the defender for an attacker (§506.1).
func setAttackerDefender(p *Permanent, seatIdx int) {
	if p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	if seatIdx < 0 {
		delete(p.Flags, flagDefenderSeat)
		return
	}
	p.Flags[flagDefenderSeat] = seatIdx + 1
}

// -----------------------------------------------------------------------------
// Public accessors — convenient for tests, mirrored from Python Permanent.
// -----------------------------------------------------------------------------

// IsAttacking reports whether the permanent is currently an attacking
// creature (CR §506.4). Cleared by EndOfCombatStep.
func (p *Permanent) IsAttacking() bool { return permFlag(p, flagAttacking) }

// IsBlocking reports whether the permanent is currently a blocking
// creature (CR §509.1e). Cleared by EndOfCombatStep.
func (p *Permanent) IsBlocking() bool { return permFlag(p, flagBlocking) }

// WasDeclaredAttacker reports whether the permanent was declared as an
// attacker in the current combat (as opposed to entering tapped+attacking
// via a token creation effect — CR §508.1).
func (p *Permanent) WasDeclaredAttacker() bool { return permFlag(p, flagDeclaredAttacker) }

// -----------------------------------------------------------------------------
// CombatPhase — main entry (CR §506).
// -----------------------------------------------------------------------------

// CombatPhase runs a single combat phase end-to-end, mirroring the
// Python combat_phase(game). Callers wanting Aggravated Assault-style
// extra-combat cascades should set gs.Flags["extra_combats_pending"] and
// loop: each call here drains one pending combat if set, else runs the
// standard one. Multiple combat phases within a single turn fire
// "at beginning of combat" triggers each time.
//
// The active player is gs.Active (standard turn structure). The defending
// player is the next seat clockwise (2-player: the opponent; commander
// pods: seat (Active+1) % len(Seats) — MVP choice; multi-opponent combat
// targeting is a Phase 5+ concern).
func CombatPhase(gs *GameState) {
	if gs == nil || len(gs.Seats) == 0 {
		return
	}
	attacker := gs.Active

	// §507 Beginning of combat step.
	gs.Phase, gs.Step = "combat", "begin_of_combat"
	gs.LogEvent(Event{Kind: "phase_step", Seat: attacker, Details: map[string]interface{}{
		"phase": "combat", "step": "begin_of_combat",
	}})
	fireBeginningOfCombatTriggers(gs, attacker)
	FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": attacker,
	})
	StateBasedActions(gs)
	PriorityRound(gs)
	if gs.CheckEnd() {
		return
	}

	// §508 Declare attackers. DeclareAttackers now tags each attacker
	// with its chosen defending seat (§506.1) via flagDefenderSeat.
	gs.Step = "declare_attackers"
	attackers := DeclareAttackers(gs, attacker)
	if len(attackers) == 0 {
		// §506.1 — skip to end_of_combat.
		EndOfCombatStep(gs)
		return
	}
	StateBasedActions(gs)
	PriorityRound(gs)
	if gs.CheckEnd() {
		return
	}

	// §509 Declare blockers. Multiplayer: each defending seat gets its
	// own blocker assignment, merged into a single attacker → blockers
	// map. DeclareBlockers reads per-attacker defender from the flag.
	gs.Step = "declare_blockers"
	blockerMap := DeclareBlockersMulti(gs, attackers)

	// AST-driven block triggers: fire for each blocker that has a
	// "block" event trigger, and for each attacker that "becomes blocked".
	fireBlockTriggers(gs, attackers, blockerMap)

	// P1P2 combat triggers: bushido (§702.45) and flanking (§702.25)
	// fire when blockers are declared.
	CheckCombatKeywordsP1P2(gs, attackers, blockerMap)

	// Combat-file blocker triggers: rampage (§702.23) and afflict (§702.130)
	// fire when blockers are declared.
	CheckCombatKeywordsCombat(gs, attackers, blockerMap)

	// CR §702.49 — Ninjutsu activation window. After blockers are
	// declared but before combat damage, the attacking player may
	// activate ninjutsu: return an unblocked attacker to hand, put
	// a ninja from hand onto the battlefield tapped and attacking.
	// Per CR §702.49a, the ninja enters "tapped and attacking" but
	// was NOT declared as an attacker, so "whenever ~ attacks"
	// triggers do NOT fire.
	attackers = CheckNinjutsuRefactored(gs, attacker, attackers, blockerMap)

	// Sneak activation window — same timing as ninjutsu (declare
	// blockers step, after blockers declared). Sneak IS a cast
	// (increments commander tax, storm, fires cast triggers) but
	// the creature enters tapped and attacking like ninjutsu.
	attackers = CheckSneak(gs, attacker, attackers, blockerMap)

	StateBasedActions(gs)
	PriorityRound(gs)
	if gs.CheckEnd() {
		return
	}

	// §510 Combat damage step(s). If any attacker or blocker has
	// first/double strike there are two steps.
	hasFS := false
	for _, a := range attackers {
		if a.HasKeyword("first strike") || a.HasKeyword("double strike") {
			hasFS = true
			break
		}
	}
	if !hasFS {
		for _, bs := range blockerMap {
			for _, b := range bs {
				if b.HasKeyword("first strike") || b.HasKeyword("double strike") {
					hasFS = true
					break
				}
			}
			if hasFS {
				break
			}
		}
	}
	if hasFS {
		gs.Step = "first_strike_damage"
		DealCombatDamageStep(gs, attackers, blockerMap, true)
		// CR §510.1c / §704.3: SBAs fire between first-strike and regular
		// combat damage steps so creatures dropped to 0 toughness leave
		// before the second round of damage.
		StateBasedActions(gs)
		// CR §117.3: priority after first-strike damage.
		PriorityRound(gs)
		if gs.CheckEnd() {
			return
		}
	}
	gs.Step = "combat_damage"
	DealCombatDamageStep(gs, attackers, blockerMap, false)
	// CR §704.3: SBAs fire after the combat damage step resolves.
	StateBasedActions(gs)
	// CR §117.3: priority after combat damage.
	PriorityRound(gs)
	if gs.CheckEnd() {
		return
	}

	// §511 End of combat.
	EndOfCombatStep(gs)
	// CR §704.3: run SBAs once more after end-of-combat triggers resolved.
	StateBasedActions(gs)
	// CR §117.3: priority after end-of-combat.
	PriorityRound(gs)
}

// fireBeginningOfCombatTriggers fires "at the beginning of combat"
// Triggered abilities on the active player's permanents. CR §603.6a.
func fireBeginningOfCombatTriggers(gs *GameState, activeSeat int) {
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	// Snapshot — effects may spawn tokens while we iterate.
	perms := append([]*Permanent{}, gs.Seats[activeSeat].Battlefield...)
	for _, p := range perms {
		if p == nil || p.Card == nil || p.Card.AST == nil {
			continue
		}
		for _, ab := range p.Card.AST.Abilities {
			t, ok := ab.(*gameast.Triggered)
			if !ok {
				continue
			}
			if !isCombatBeginTrigger(&t.Trigger) {
				continue
			}
			gs.LogEvent(Event{
				Kind: "trigger_fires", Seat: p.Controller,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"event": "begin_of_combat",
					"rule":  "603.6a",
				},
			})
			if t.Effect != nil {
				// Phase 5 routing: triggered abilities go ON the stack
				// (CR §603.3a) instead of resolving inline. Priority opens
				// after the push — see PushTriggeredAbility.
				PushTriggeredAbility(gs, p, t.Effect)
			}
		}
	}
}

func isCombatBeginTrigger(tr *gameast.Trigger) bool {
	if tr == nil {
		return false
	}
	if tr.Event == "phase" && (tr.Phase == "combat_start" || tr.Phase == "begin_of_combat") {
		return true
	}
	if tr.Event == "combat_start" || tr.Event == "begin_of_combat" {
		return true
	}
	return false
}

// -----------------------------------------------------------------------------
// DeclareAttackers — CR §508.
// -----------------------------------------------------------------------------

// DeclareAttackers implements the declare-attackers step for the given
// seat. Greedy MVP policy: every legal attacker attacks. Returns the
// final list of attacking creatures (including any that entered
// "tapped and attacking" via a token-creation effect — CR §506.3). Only
// DECLARED attackers fire their own "attacks" triggers (§508.1).
func DeclareAttackers(gs *GameState, attackerSeat int) []*Permanent {
	if gs == nil || attackerSeat < 0 || attackerSeat >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[attackerSeat]

	// Clear per-combat declaration flags so multi-combat phases (Aggravated
	// Assault) can re-fire attack triggers for the same creature.
	for _, p := range seat.Battlefield {
		setPermFlag(p, flagDeclaredAttacker, false)
	}

	// Step 1: build legal attacker pool, then delegate to Hat.
	livingOpps := gs.LivingOpponents(attackerSeat)
	var legal []*Permanent
	for _, p := range seat.Battlefield {
		if canAttack(p) {
			legal = append(legal, p)
		}
	}

	// Hat picks which creatures to attack with (§506.1).
	chosen := legal // default: all legal attackers (greedy)
	if seat.Hat != nil {
		chosen = seat.Hat.ChooseAttackers(gs, attackerSeat, legal)
	}

	// Silent Arbiter: max one attacker per combat.
	if len(chosen) > 1 && silentArbiterOnBattlefield(gs) {
		chosen = chosen[:1]
		gs.LogEvent(Event{
			Kind: "attack_restricted",
			Seat: attackerSeat,
			Details: map[string]interface{}{
				"reason": "silent_arbiter",
				"max":    1,
			},
		})
	}

	declared := []*Permanent{}
	for _, p := range chosen {
		setPermFlag(p, flagDeclaredAttacker, true)
		setPermFlag(p, flagAttacking, true)
		setPermFlag(p, flagAttackedThisCombat, true)
		if len(livingOpps) == 0 {
			setPermFlag(p, flagDeclaredAttacker, false)
			setPermFlag(p, flagAttacking, false)
			setPermFlag(p, flagAttackedThisCombat, false)
			continue
		}
		// Hat picks which opponent each attacker targets (§506.1).
		def := pickAttackDefender(gs, p, livingOpps)
		if seat.Hat != nil {
			def = seat.Hat.ChooseAttackTarget(gs, attackerSeat, p, livingOpps)
		}
		setAttackerDefender(p, def)

		// Propaganda / Ghostly Prison: pay {2} per attacker or skip.
		tax := propagandaTaxFor(gs, def)
		if tax > 0 {
			if seat.ManaPool >= tax {
				seat.ManaPool -= tax
				SyncManaAfterSpend(seat)
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   attackerSeat,
					Source: "Propaganda",
					Amount: tax,
					Details: map[string]interface{}{
						"reason":   "attack_tax",
						"defender": def,
					},
				})
			} else {
				setPermFlag(p, flagDeclaredAttacker, false)
				setPermFlag(p, flagAttacking, false)
				setPermFlag(p, flagAttackedThisCombat, false)
				gs.LogEvent(Event{
					Kind: "attack_prevented",
					Seat: attackerSeat,
					Details: map[string]interface{}{
						"reason":   "propaganda_cant_pay",
						"defender": def,
						"tax":      tax,
					},
				})
				continue
			}
		}

		if !p.HasKeyword("vigilance") {
			p.Tapped = true
		}
		declared = append(declared, p)
	}
	if len(declared) > 0 {
		// §702.136 Raid — set seat-level flag so CheckRaid returns true
		// for the rest of this turn. The flag is cleared at turn start
		// (tournament/turn.go untap step bookkeeping).
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["attacked_this_turn"] = 1

		pairs := make([]map[string]interface{}, 0, len(declared))
		for _, a := range declared {
			def, _ := AttackerDefender(a)
			pairs = append(pairs, map[string]interface{}{
				"attacker":      a.Card.DisplayName(),
				"defender_seat": def,
			})
		}
		gs.LogEvent(Event{
			Kind: "declare_attackers", Seat: attackerSeat,
			Details: map[string]interface{}{"attackers": pairs},
		})
	}

	// Step 2: scoop in permanents that entered "tapped and attacking"
	// (CreateToken with e.Tapped + setPermFlag(attacking)). These are
	// attacking creatures per §506.3 but don't fire own-attack triggers.
	attackers := append([]*Permanent{}, declared...)
	for _, p := range seat.Battlefield {
		if permFlag(p, flagAttacking) && !permFlag(p, flagDeclaredAttacker) {
			attackers = append(attackers, p)
			gs.LogEvent(Event{
				Kind: "entered_attacking", Seat: attackerSeat,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{"rule": "506.3"},
			})
		}
	}

	// Step 3: fire attack triggers — only for creatures actually declared.
	// §508.1 / §603.3a. Handles both "this attacks" (self actor) and
	// "whenever a creature you control attacks" (ally actor).
	fireAttackTriggers(gs, attackerSeat, declared)

	// §702.83 — Exalted: "Whenever a creature you control attacks alone,
	// that creature gets +1/+1 until end of turn." Each instance of exalted
	// triggers separately (each permanent with exalted grants +1/+1).
	if len(declared) == 1 {
		ApplyExalted(gs, attackerSeat, declared[0])
	}

	// §702.105 — Dethrone: +1/+1 counter when attacking the player with
	// the most life. Fires after attack declaration but before blockers.
	FireDethroneTriggers(gs, attackerSeat, declared)

	// Combat-file attack keywords: battle cry, myriad, melee, annihilator,
	// provoke. Fires after exalted so that buffs layer correctly.
	CheckAttackKeywordsCombat(gs, attackerSeat, attackers)

	return attackers
}

// canAttack mirrors Python can_attack() — CR §508.1a.
func canAttack(p *Permanent) bool {
	if p == nil || !p.IsCreature() {
		return false
	}
	if p.Tapped {
		return false
	}
	// §702.26: phased-out permanents don't exist.
	if p.PhasedOut {
		return false
	}
	if p.SummoningSick && !p.HasKeyword("haste") {
		return false
	}
	if p.HasKeyword("defender") {
		return false
	}
	if p.Flags != nil && p.Flags["detained"] == 1 {
		return false
	}
	if p.Power() <= 0 {
		return false
	}
	return true
}

// fireAttackTriggers fires "attacks" triggers. Two pools:
//
//  1. Each declared attacker's own "when/whenever this attacks" triggers.
//  2. "Whenever a creature you control attacks" triggers on OTHER
//     permanents the active player controls — fired once per attacker.
//
// Matches Python _fire_attack_triggers().
func fireAttackTriggers(gs *GameState, activeSeat int, declared []*Permanent) {
	if len(declared) == 0 {
		return
	}
	// (1) Self-attack triggers.
	for _, atk := range declared {
		for _, ab := range iterAttackTriggers(atk.Card) {
			gs.LogEvent(Event{
				Kind: "trigger_fires", Seat: atk.Controller,
				Source: atk.Card.DisplayName(),
				Details: map[string]interface{}{
					"event": "attack", "rule": "603.3a",
				},
			})
			if ab.Effect != nil {
				// Phase 5: trigger goes on the stack (CR §603.3a).
				PushTriggeredAbility(gs, atk, ab.Effect)
			}
		}
		FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
			"attacker_perm": atk,
			"attacker_seat": atk.Controller,
			"attacker_card": atk.Card,
		})
	}
	// (2) Ally-attack triggers ("whenever a creature you control attacks").
	declaredSet := make(map[*Permanent]struct{}, len(declared))
	for _, a := range declared {
		declaredSet[a] = struct{}{}
	}
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	controllerPerms := append([]*Permanent{}, gs.Seats[activeSeat].Battlefield...)
	for _, perm := range controllerPerms {
		if _, self := declaredSet[perm]; self {
			continue
		}
		for _, ab := range iterAttackTriggers(perm.Card) {
			if !strings.Contains(ab.Raw, "a creature you control attacks") &&
				!strings.Contains(ab.Raw, "another creature attacks") {
				continue
			}
			for _, atk := range declared {
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: perm.Controller,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"event":           "attack_ally",
						"trigger_by_card": atk.Card.DisplayName(),
					},
				})
				if ab.Effect != nil {
					// Phase 5: trigger goes on the stack (CR §603.3a).
					PushTriggeredAbility(gs, perm, ab.Effect)
				}
			}
		}
	}
}

// pickAttackDefender returns the seat index this attacker should attack.
// Policy (mirrors Python threat-score heuristic): lowest-life living
// opponent wins; ties broken by fewest creatures (softer board), then
// APNAP order from the attacker (which livingOpps already encodes).
//
// Callers pass the pre-filtered living-opponents list; this function
// never returns a dead seat. Returns livingOpps[0] as a safe default.
func pickAttackDefender(gs *GameState, atk *Permanent, livingOpps []int) int {
	if len(livingOpps) == 0 {
		return -1
	}
	best := livingOpps[0]
	bestLife := gs.Seats[best].Life
	bestCreatures := countCreatures(gs, best)
	for _, cand := range livingOpps[1:] {
		s := gs.Seats[cand]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			best = cand
			bestLife = s.Life
			bestCreatures = countCreatures(gs, cand)
			continue
		}
		if s.Life == bestLife {
			cc := countCreatures(gs, cand)
			if cc < bestCreatures {
				best = cand
				bestCreatures = cc
			}
		}
	}
	return best
}

// countCreatures returns the number of creatures on seat's battlefield.
// Used by pickAttackDefender's tiebreaker.
func countCreatures(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}

// iterAttackTriggers returns Triggered abilities whose trigger.event == "attack".
func iterAttackTriggers(c *Card) []*gameast.Triggered {
	if c == nil || c.AST == nil {
		return nil
	}
	out := []*gameast.Triggered{}
	for _, ab := range c.AST.Abilities {
		if t, ok := ab.(*gameast.Triggered); ok {
			if EventEquals(t.Trigger.Event, "attack") {
				out = append(out, t)
			}
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// DeclareBlockers — CR §509.
// -----------------------------------------------------------------------------

// DeclareBlockers assigns blockers on the defending side. Greedy policy:
// for each attacker, pick the smallest-power untapped legal blocker that
// survives AND kills (or trades with) the attacker. If nothing qualifies,
// chump-block with the smallest creature when the attacker is lethal to
// the defending player's remaining life.
//
// Returns a map keyed by attacker pointer. Keys for unblocked attackers
// are present with empty slices.
func DeclareBlockers(gs *GameState, attackers []*Permanent, defenderSeat int) map[*Permanent][]*Permanent {
	out := map[*Permanent][]*Permanent{}
	for _, a := range attackers {
		out[a] = nil
	}
	if gs == nil || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return out
	}

	// Delegate to Hat if available (§509.1).
	if seat := gs.Seats[defenderSeat]; seat != nil && seat.Hat != nil {
		hatMap := seat.Hat.AssignBlockers(gs, defenderSeat, attackers)
		if hatMap != nil {
			for _, a := range attackers {
				blockers := hatMap[a]
				for _, b := range blockers {
					setPermFlag(b, flagBlocking, true)
				}
				out[a] = blockers
			}
			pairs := make([]map[string]interface{}, 0, len(attackers))
			for _, a := range attackers {
				names := make([]string, 0, len(out[a]))
				for _, b := range out[a] {
					names = append(names, b.Card.DisplayName())
				}
				pairs = append(pairs, map[string]interface{}{
					"attacker": a.Card.DisplayName(),
					"blockers": names,
				})
			}
			gs.LogEvent(Event{
				Kind: "blockers", Seat: defenderSeat,
				Details: map[string]interface{}{"pairs": pairs},
			})
			return out
		}
	}

	// Fallback: hardcoded greedy blocking policy.
	used := map[*Permanent]bool{} // blockers already committed
	pool := []*Permanent{}
	for _, p := range gs.Seats[defenderSeat].Battlefield {
		if p.IsCreature() && !p.Tapped {
			pool = append(pool, p)
		}
	}
	// Keep deterministic: iterate attackers in input order. For each,
	// try to find a single legal blocker that trades favorably. Menace
	// requires 2 blockers; chump unless we commit 2+.
	for _, atk := range attackers {
		if atk.HasKeyword("unblockable") {
			continue
		}
		// Gather candidates.
		cands := []*Permanent{}
		for _, b := range pool {
			if used[b] {
				continue
			}
			if !canBlockGS(gs, atk, b) {
				continue
			}
			cands = append(cands, b)
		}
		if len(cands) == 0 {
			continue
		}

		// Menace: need 2 blockers; else skip.
		menace := atk.HasKeyword("menace")
		if menace && len(cands) < 2 {
			continue
		}

		// Policy: pick the smallest-toughness creature that kills the
		// attacker without dying, else the smallest that trades, else
		// chump with the weakest. Mirrors Python GreedyPolicy.
		var best *Permanent
		bestScore := -1 << 30
		for _, b := range cands {
			s := scoreBlock(atk, b)
			if s > bestScore {
				bestScore = s
				best = b
			}
		}
		if best == nil {
			continue
		}
		assigned := []*Permanent{best}
		used[best] = true
		if menace {
			// Pick a second legal blocker — smallest remaining.
			var second *Permanent
			secondTough := 1 << 30
			for _, b := range cands {
				if used[b] {
					continue
				}
				if b.Toughness() < secondTough {
					secondTough = b.Toughness()
					second = b
				}
			}
			if second == nil {
				// Can't satisfy menace — unassign and skip.
				used[best] = false
				continue
			}
			assigned = append(assigned, second)
			used[second] = true
		}
		for _, b := range assigned {
			setPermFlag(b, flagBlocking, true)
		}
		out[atk] = assigned
	}

	// Log the assignment as a single event.
	pairs := make([]map[string]interface{}, 0, len(attackers))
	for _, a := range attackers {
		names := make([]string, 0, len(out[a]))
		for _, b := range out[a] {
			names = append(names, b.Card.DisplayName())
		}
		pairs = append(pairs, map[string]interface{}{
			"attacker": a.Card.DisplayName(),
			"blockers": names,
		})
	}
	gs.LogEvent(Event{
		Kind: "blockers", Seat: defenderSeat,
		Details: map[string]interface{}{"pairs": pairs},
	})
	return out
}

// DeclareBlockersMulti is the multiplayer generalization of DeclareBlockers.
// It partitions attackers by their recorded defender seat (flagDefenderSeat
// / AttackerDefender) and calls DeclareBlockers once per defender, merging
// the resulting maps into a single return value.
//
// CR §509.1a — "The defending player declares blockers." In multiplayer
// each defending player declares their own blockers; since attackers
// target specific players (§506.1), each defending seat only chooses
// blockers from the attackers targeting THEM.
//
// Attackers without a recorded defender (e.g. legacy callers that didn't
// set flagDefenderSeat) are bucketed under (gs.Active+1) % N for 2p
// compatibility — same as the old DeclareBlockers default.
func DeclareBlockersMulti(gs *GameState, attackers []*Permanent) map[*Permanent][]*Permanent {
	out := map[*Permanent][]*Permanent{}
	if gs == nil || len(attackers) == 0 {
		return out
	}
	n := len(gs.Seats)
	buckets := map[int][]*Permanent{}
	for _, atk := range attackers {
		def, ok := AttackerDefender(atk)
		if !ok || def < 0 || def >= n {
			// Legacy fallback: attack the next seat clockwise.
			def = (gs.Active + 1) % n
			setAttackerDefender(atk, def)
		}
		buckets[def] = append(buckets[def], atk)
	}
	for defSeat, atks := range buckets {
		partial := DeclareBlockers(gs, atks, defSeat)
		for k, v := range partial {
			out[k] = v
		}
	}
	return out
}

// CanBlock is the exported wrapper for canBlock — used by Hat
// implementations in internal/hat when enumerating legal blockers
// during AssignBlockers.
func CanBlock(attacker, blocker *Permanent) bool { return canBlockGS(nil, attacker, blocker) }

// CanBlockGS is like CanBlock but with game state for landwalk checks.
func CanBlockGS(gs *GameState, attacker, blocker *Permanent) bool {
	return canBlockGS(gs, attacker, blocker)
}

// canBlock mirrors Python can_block() — CR §509.1b. Legacy wrapper
// without game state (landwalk checks skipped).
func canBlock(attacker, blocker *Permanent) bool { return canBlockGS(nil, attacker, blocker) }

// canBlockGS mirrors Python can_block() with optional game state for
// §702.14 landwalk checks — CR §509.1b.
func canBlockGS(gs *GameState, attacker, blocker *Permanent) bool {
	if blocker == nil || !blocker.IsCreature() {
		return false
	}
	if blocker.Tapped {
		return false
	}
	// §702.26: phased-out permanents don't exist.
	if blocker.PhasedOut || attacker.PhasedOut {
		return false
	}
	// Flying: blocked only by flying or reach.
	if attacker.HasKeyword("flying") {
		if !(blocker.HasKeyword("flying") || blocker.HasKeyword("reach")) {
			return false
		}
	}
	// §702.30 — Horsemanship: blocked only by creatures with horsemanship.
	if !CanBlockP1P2(attacker, blocker) {
		return false
	}
	// Combat-file evasion keywords: intimidate, fear, shadow, skulk, daunt.
	if !CanBlockCombatKeywords(gs, attacker, blocker) {
		return false
	}
	// Sidar Kondo of Jamuraa — global static: creatures with power 2 or
	// less can't be blocked by creatures with power 3 or greater. Flag is
	// set by the per_card ETB handler; we re-verify Sidar Kondo is still
	// on a battlefield before applying.
	if gs != nil && gs.Flags != nil && gs.Flags["sidar_kondo_active"] == 1 {
		if attacker.Power() <= 2 && blocker.Power() >= 3 && sidarKondoOnBattlefield(gs) {
			return false
		}
	}
	// Unblockable-style effects.
	if attacker.HasKeyword("unblockable") {
		return false
	}
	// §702.14 — Landwalk: if attacker has landwalk and the blocker's
	// controller controls the matching land type, the attacker can't be
	// blocked by that player's creatures. Requires game state for land
	// checks — skipped when gs is nil (legacy CanBlock path).
	if gs != nil && blocker.Controller >= 0 {
		lt := LandwalkType(attacker)
		if lt != "" && DefenderControlsLandType(gs, blocker.Controller, lt) {
			return false
		}
	}
	// Protection on attacker from blocker's color — CR §702.16b:
	// "can't be blocked by" creatures of a color/quality the
	// attacker has protection from.
	if attackerHasProtectionFrom(attacker, blocker) {
		return false
	}
	// Protection on blocker from attacker — the blocker also can't
	// block if IT has protection from the attacker (§702.16e:
	// "can't block" is only relevant for the attacker side;
	// a blocker's protection doesn't prevent blocking).
	// NOTE: only attacker-side protection prevents blocking per CR.
	return true
}

// scoreBlock assigns a policy score to a candidate blocker-vs-attacker.
// Higher is better. Outcomes roughly ordered:
//
//	+100 : blocker survives and kills attacker (clean kill)
//	+ 50 : trade (both die)
//	+ 10 : chump, but attacker would otherwise kill defender
//	  0 : chump with nothing to gain
//
// Ties broken toward lower toughness (save resources for next combat).
func scoreBlock(atk, b *Permanent) int {
	atkKills := incomingIsLethal(atk, b)
	blkKills := incomingIsLethal(b, atk)
	score := 0
	switch {
	case !atkKills && blkKills:
		score = 100
	case atkKills && blkKills:
		score = 50
	case atkKills && !blkKills:
		score = 10
	default:
		score = 0
	}
	// Prefer lower toughness on ties.
	score -= b.Toughness()
	return score
}

// incomingIsLethal reports whether 'src' would deal lethal damage to 'dst'
// this combat (accounting for deathtouch and current marked damage).
func incomingIsLethal(src, dst *Permanent) bool {
	if src == nil || dst == nil {
		return false
	}
	if attackerHasProtectionFrom(dst, src) {
		// dst has protection → damage prevented.
		return false
	}
	dmg := src.Power()
	if dmg <= 0 {
		return false
	}
	if src.HasKeyword("deathtouch") {
		return true
	}
	return dmg+dst.MarkedDamage >= dst.Toughness()
}

// attackerHasProtectionFrom returns true if defender (or attacker in the
// blocker role) has protection from source's color. Simplified MVP —
// color-based protection only.
func attackerHasProtectionFrom(protected, source *Permanent) bool {
	if protected == nil || source == nil {
		return false
	}
	prot := protectionColors(protected)
	if len(prot) == 0 {
		return false
	}
	if _, any := prot["*"]; any {
		return true
	}
	for c := range cardColors(source.Card) {
		if _, hit := prot[c]; hit {
			return true
		}
	}
	return false
}

// protectionColors extracts the set of colors a permanent has protection
// from. Reads both AST keyword raw text ("protection from red") and
// runtime flags ("prot:R" on Permanent.Flags).
//
// Returns a set map whose keys are single-letter color codes:
// W, U, B, R, G — plus the sentinel "*" meaning "protection from
// everything".
func protectionColors(p *Permanent) map[string]struct{} {
	out := map[string]struct{}{}
	if p == nil {
		return out
	}
	if p.Flags != nil {
		for k := range p.Flags {
			if strings.HasPrefix(k, "prot:") {
				out[strings.TrimPrefix(k, "prot:")] = struct{}{}
			}
		}
	}
	if p.Card != nil && p.Card.AST != nil {
		for _, ab := range p.Card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok || strings.ToLower(kw.Name) != "protection" {
				continue
			}
			if strings.Contains(kw.Raw, "from everything") {
				out["*"] = struct{}{}
				continue
			}
			for word, letter := range colorWords {
				if strings.Contains(kw.Raw, word) {
					out[letter] = struct{}{}
				}
			}
		}
	}
	return out
}

var colorWords = map[string]string{
	"white": "W",
	"blue":  "U",
	"black": "B",
	"red":   "R",
	"green": "G",
}

// cardColors returns the colors of a card as a set of single-letter
// codes. MVP sources: Card.Types (tests often stuff "red"/"blue" in as
// a "type" since Phase 3 hasn't built a proper color axis yet) and
// Flags via the permanent. Returns empty for colorless.
func cardColors(c *Card) map[string]struct{} {
	out := map[string]struct{}{}
	if c == nil {
		return out
	}
	// Primary source: Card.Colors (populated by corpus loader).
	for _, clr := range c.Colors {
		u := strings.ToUpper(clr)
		if len(u) == 1 {
			out[u] = struct{}{}
		} else if letter, ok := colorWords[strings.ToLower(clr)]; ok {
			out[letter] = struct{}{}
		}
	}
	// Fallback: infer from type line (e.g., "red" in Types).
	if len(out) == 0 {
		for _, t := range c.Types {
			if letter, ok := colorWords[strings.ToLower(t)]; ok {
				out[letter] = struct{}{}
			}
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// DealCombatDamageStep — CR §510.
// -----------------------------------------------------------------------------

// DealCombatDamageStep applies combat damage. When isFirstStrike is true,
// only creatures with first_strike or double_strike deal damage; when
// false, creatures WITHOUT first_strike deal damage and double_strikers
// deal damage a second time. Mirrors Python _deal_combat_damage_step.
func DealCombatDamageStep(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent, isFirstStrike bool) {
	if gs == nil {
		return
	}
	// Fog check: prevent_all_combat_damage (Fog, Clinging Mists, etc.)
	if gs.Flags != nil && gs.Flags["prevent_all_combat_damage"] > 0 {
		gs.LogEvent(Event{
			Kind:   "combat_damage_prevented",
			Source: "fog_effect",
			Details: map[string]interface{}{
				"rule": "615.1",
			},
		})
		return
	}
	if len(gs.Seats) == 0 {
		return
	}
	fallbackDefender := (gs.Active + 1) % len(gs.Seats)

	// Phase A: attackers -> blockers or defending player.
	for _, atk := range attackers {
		if !alive(gs, atk) {
			continue
		}
		if !dealsInStep(atk, isFirstStrike) {
			continue
		}
		dmg := atk.Power()
		if dmg <= 0 {
			continue
		}
		// Per-attacker defender (CR §506.1). Multiplayer generalization.
		defenderSeat, ok := AttackerDefender(atk)
		if !ok || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
			defenderSeat = fallbackDefender
		}
		// §800.4e — combat damage to a seat that has left the game is
		// not assigned. Redirect to fallback if defender is dead.
		if gs.Seats[defenderSeat] != nil && gs.Seats[defenderSeat].Lost {
			continue
		}
		declaredBlockers := blockerMap[atk]
		liveBlockers := make([]*Permanent, 0, len(declaredBlockers))
		for _, b := range declaredBlockers {
			if alive(gs, b) {
				liveBlockers = append(liveBlockers, b)
			}
		}

		if len(declaredBlockers) == 0 {
			// Unblocked — all damage to defender.
			applyCombatDamageToPlayer(gs, atk, dmg, defenderSeat)
			continue
		}
		if len(liveBlockers) == 0 {
			// All declared blockers removed before damage. Trample goes
			// to defender; otherwise damage fizzles per §510.1c.
			if atk.HasKeyword("trample") {
				applyCombatDamageToPlayer(gs, atk, dmg, defenderSeat)
			}
			continue
		}

		// §510.1c: the attacking player assigns a damage assignment
		// order to the blockers. The attacker must assign at least
		// lethal damage to the first creature in order before moving
		// to the next. Policy: order by ascending toughness (kill the
		// weakest first, maximizing kills and trample spillover).
		ordered := append([]*Permanent{}, liveBlockers...)
		for i := 0; i < len(ordered)-1; i++ {
			for j := i + 1; j < len(ordered); j++ {
				if ordered[j].Toughness()-ordered[j].MarkedDamage <
					ordered[i].Toughness()-ordered[i].MarkedDamage {
					ordered[i], ordered[j] = ordered[j], ordered[i]
				}
			}
		}

		remaining := dmg
		for _, b := range ordered {
			if remaining <= 0 {
				break
			}
			need := lethalAmount(atk, b)
			give := remaining
			if give > need {
				give = need
			}
			applyCombatDamageToCreature(gs, atk, give, b)
			remaining -= give
		}
		if remaining > 0 && atk.HasKeyword("trample") {
			applyCombatDamageToPlayer(gs, atk, remaining, defenderSeat)
		}
	}

	// Phase B: blockers -> attackers.
	for _, atk := range attackers {
		for _, b := range blockerMap[atk] {
			if !alive(gs, b) || !alive(gs, atk) {
				continue
			}
			if !dealsInStep(b, isFirstStrike) {
				continue
			}
			dmg := b.Power()
			if dmg <= 0 {
				continue
			}
			applyCombatDamageToCreature(gs, b, dmg, atk)
		}
	}
}

// dealsInStep decides if p deals damage in this step. First-strike step
// is for FS+DS; the regular step is for everyone WITHOUT plain FS
// (double-strikers also deal in the regular step).
func dealsInStep(p *Permanent, firstStrike bool) bool {
	fs := p.HasKeyword("first strike")
	ds := p.HasKeyword("double strike")
	if firstStrike {
		return fs || ds
	}
	return !fs || ds
}

// alive reports whether p is still on its controller's battlefield.
func alive(gs *GameState, p *Permanent) bool {
	if gs == nil || p == nil {
		return false
	}
	if p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	for _, q := range gs.Seats[p.Controller].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

// lethalAmount mirrors Python _lethal_amount — respects deathtouch +
// pre-existing marked damage.
func lethalAmount(attacker, blocker *Permanent) int {
	if attacker.HasKeyword("deathtouch") {
		return 1
	}
	need := blocker.Toughness() - blocker.MarkedDamage
	if need < 1 {
		return 1
	}
	return need
}

// applyCombatDamageToPlayer applies combat damage from src to a player
// seat, including lifelink gain and per-instance "deals combat damage
// to a player" triggers. §702.16 protection from the source's color
// prevents the damage.
func applyCombatDamageToPlayer(gs *GameState, src *Permanent, amount, seatIdx int) {
	if amount <= 0 || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	// §702.16d: protection prevents damage from sources of the
	// protected quality. Check if the defending player has protection
	// flags (e.g. from Teferi's Protection "protection from everything").
	seat := gs.Seats[seatIdx]
	if seat != nil && seat.Flags != nil {
		if seat.Flags["protection_from_everything"] > 0 {
			gs.LogEvent(Event{
				Kind: "damage_prevented", Seat: src.Controller,
				Target: seatIdx, Source: src.Card.DisplayName(),
				Amount: amount,
				Details: map[string]interface{}{
					"reason": "protection_from_everything",
				},
			})
			return
		}
	}
	// §615: apply prevention shields before dealing combat damage.
	amount = PreventDamageToPlayer(gs, seatIdx, amount, src)
	if amount <= 0 {
		return
	}
	// §702.90 — Infect: damage dealt to players is dealt in the form
	// of poison counters instead of life loss.
	if HasInfect(src) {
		gs.Seats[seatIdx].PoisonCounters += amount
		gs.LogEvent(Event{
			Kind: "poison", Seat: src.Controller, Target: seatIdx,
			Source: src.Card.DisplayName(), Amount: amount,
			Details: map[string]interface{}{
				"target_kind": "player",
				"combat":      true,
				"rule":        "702.90",
			},
		})
	} else {
		gs.Seats[seatIdx].Life -= amount
		gs.LogEvent(Event{
			Kind: "damage", Seat: src.Controller, Target: seatIdx,
			Source: src.Card.DisplayName(), Amount: amount,
			Details: map[string]interface{}{"target_kind": "player", "combat": true},
		})
	}
	// Set damage_taken_this_turn flag so Bloodthirst (§702.54) and similar
	// mechanics can detect that this player was dealt damage.
	if gs.Seats[seatIdx].Flags == nil {
		gs.Seats[seatIdx].Flags = map[string]int{}
	}
	gs.Seats[seatIdx].Flags["damage_taken_this_turn"] = 1
	if src.HasKeyword("lifelink") {
		GainLife(gs, src.Controller, amount, src.Card.DisplayName())
	}
	// §702.165 — Toxic: in ADDITION to normal damage, the damaged player
	// gets N poison counters. Unlike infect (which replaces damage with
	// poison), toxic adds poison on top of regular damage.
	if hasToxic, n := HasToxic(src); hasToxic && n > 0 {
		gs.Seats[seatIdx].PoisonCounters += n
		gs.LogEvent(Event{
			Kind: "poison", Seat: src.Controller, Target: seatIdx,
			Source: src.Card.DisplayName(), Amount: n,
			Details: map[string]interface{}{
				"target_kind": "player",
				"combat":      true,
				"rule":        "702.165",
				"reason":      "toxic",
			},
		})
	}
	// §721.4 — If a creature deals combat damage to the monarch, its
	// controller becomes the monarch.
	CheckMonarchCombatSteal(gs, seatIdx, src.Controller)

	fireCombatDamageTriggers(gs, src, amount, "player", seatIdx, nil)
}

// applyCombatDamageToCreature applies combat damage from src to another
// permanent, respecting protection, deathtouch (lethal = any), lifelink,
// and per-instance combat-damage triggers.
func applyCombatDamageToCreature(gs *GameState, src *Permanent, amount int, target *Permanent) {
	if amount <= 0 || target == nil {
		return
	}
	if attackerHasProtectionFrom(target, src) {
		gs.LogEvent(Event{
			Kind: "damage_prevented", Seat: src.Controller,
			Target: target.Controller, Source: src.Card.DisplayName(),
			Amount: amount,
			Details: map[string]interface{}{
				"target_card": target.Card.DisplayName(),
				"reason":      "protection",
			},
		})
		return
	}
	// §615: apply prevention shields before dealing combat damage.
	amount = PreventDamageToPermanent(gs, target, amount, src)
	if amount <= 0 {
		return
	}
	// §702.90 — Infect: damage dealt to creatures is dealt in the form
	// of -1/-1 counters instead of marked damage.
	if HasInfect(src) {
		target.AddCounter("-1/-1", amount)
		gs.InvalidateCharacteristicsCache() // -1/-1 counters change P/T
		gs.LogEvent(Event{
			Kind: "infect_counters", Seat: src.Controller, Target: target.Controller,
			Source: src.Card.DisplayName(), Amount: amount,
			Details: map[string]interface{}{
				"target_kind": "creature",
				"target_card": target.Card.DisplayName(),
				"combat":      true,
				"rule":        "702.90",
			},
		})
		if src.HasKeyword("lifelink") {
			GainLife(gs, src.Controller, amount, src.Card.DisplayName())
		}
		fireCombatDamageTriggers(gs, src, amount, "creature", target.Controller, target)
		return
	}
	// §702.80 — Wither: damage dealt to creatures is dealt in the form
	// of -1/-1 counters instead of marked damage. Unlike infect, damage
	// to players is normal (handled in applyCombatDamageToPlayer).
	if HasWither(src) {
		ApplyWitherDamageToCreature(gs, src, target, amount)
		gs.InvalidateCharacteristicsCache() // -1/-1 counters change P/T
		if src.HasKeyword("lifelink") {
			GainLife(gs, src.Controller, amount, src.Card.DisplayName())
		}
		fireCombatDamageTriggers(gs, src, amount, "creature", target.Controller, target)
		return
	}
	target.MarkedDamage += amount
	if src.HasKeyword("deathtouch") && amount > 0 {
		// Any nonzero damage from a deathtouch source is lethal (§702.2b).
		if target.MarkedDamage < target.Toughness() {
			target.MarkedDamage = target.Toughness()
		}
		// Flag for §704.5h SBA — deathtouch sub-lethal damage kill.
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		target.Flags["deathtouch_damaged"] = 1
	}
	gs.LogEvent(Event{
		Kind: "damage", Seat: src.Controller, Target: target.Controller,
		Source: src.Card.DisplayName(), Amount: amount,
		Details: map[string]interface{}{
			"target_kind": "creature",
			"target_card": target.Card.DisplayName(),
			"combat":      true,
		},
	})
	if src.HasKeyword("lifelink") {
		GainLife(gs, src.Controller, amount, src.Card.DisplayName())
	}
	fireCombatDamageTriggers(gs, src, amount, "creature", target.Controller, target)
}

// fireCombatDamageTriggers fires "whenever ~ deals combat damage"
// triggers ONCE per damage instance (CR §510.2). Double-strikers fire
// twice automatically because DealCombatDamageStep runs twice for them.
func fireCombatDamageTriggers(gs *GameState, src *Permanent, amount int, targetKind string, targetSeat int, targetPerm *Permanent) {
	if amount <= 0 || src == nil {
		return
	}
	// Simic Basilisk / basilisk-granted ability: "Whenever this creature
	// deals combat damage to a creature, destroy that creature at end of
	// combat." Mark the target for delayed destruction.
	if targetKind == "creature" && targetPerm != nil && src.Flags != nil &&
		src.Flags["basilisk_granted"] > 0 {
		src.Flags["basilisk_combat_hit"] = 1
		if targetPerm.Flags == nil {
			targetPerm.Flags = map[string]int{}
		}
		targetPerm.Flags["basilisk_marked_destroy"] = 1
	}
	if src.Card == nil || src.Card.AST == nil {
		return
	}
	for _, ab := range src.Card.AST.Abilities {
		t, ok := ab.(*gameast.Triggered)
		if !ok {
			continue
		}
		if !EventEquals(t.Trigger.Event, "deals_combat_damage") &&
			!EventEquals(t.Trigger.Event, "deals_damage") {
			continue
		}
		ctxTargetName := ""
		if targetPerm != nil {
			ctxTargetName = targetPerm.Card.DisplayName()
		}
		gs.LogEvent(Event{
			Kind: "trigger_fires", Seat: src.Controller,
			Source: src.Card.DisplayName(), Amount: amount,
			Target: targetSeat,
			Details: map[string]interface{}{
				"event":       "deals_combat_damage",
				"target_kind": targetKind,
				"target_card": ctxTargetName,
				"rule":        "510.2",
			},
		})
		if t.Effect != nil {
			// Phase 5: damage triggers go on the stack (CR §603.3a). SBAs
			// will fire between damage dealing and the trigger's resolution
			// per CR §510.2 → §704.3.
			PushTriggeredAbility(gs, src, t.Effect)
		}
	}
	// Fire per-card trigger hooks for combat damage events so that
	// per_card handlers (Fynn, Yuriko, etc.) receive the notification.
	if targetKind == "player" {
		FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
			"source_seat":  src.Controller,
			"source_card":  src.Card.DisplayName(),
			"defender_seat": targetSeat,
			"amount":       amount,
		})
	}
}

// -----------------------------------------------------------------------------
// EndOfCombatStep — CR §511 (+ §506.4 attack/block status clearing and
// §514.2 damage wear-off is cleanup step; we clear marked damage here so
// mid-turn post-combat interactions see a clean slate only for tests
// that run combat in isolation. The full cleanup step will re-clear in
// Phase 6.)
// -----------------------------------------------------------------------------

// EndOfCombatStep fires "at end of combat" triggers, clears per-combat
// flags, and expires until-end-of-combat modifications. Marked damage
// persists until the cleanup step (§514.2) — we do NOT clear it here,
// so post-combat state-based actions in Phase 6 can still see it.
//
// In some Python tests the cleanup step also runs; until Phase 6 lands,
// DealCombatDamageStep's SBA side-effect is left to the caller.
func EndOfCombatStep(gs *GameState) {
	if gs == nil {
		return
	}
	gs.Phase, gs.Step = "combat", "end_of_combat"
	gs.LogEvent(Event{Kind: "phase_step", Seat: gs.Active, Details: map[string]interface{}{
		"phase": "combat", "step": "end_of_combat",
	}})

	// Fire "at end of combat" triggers on any seat's battlefield.
	for seatIdx, seat := range gs.Seats {
		_ = seatIdx
		perms := append([]*Permanent{}, seat.Battlefield...)
		for _, p := range perms {
			if p == nil || p.Card == nil || p.Card.AST == nil {
				continue
			}
			for _, ab := range p.Card.AST.Abilities {
				t, ok := ab.(*gameast.Triggered)
				if !ok {
					continue
				}
				if !isEndOfCombatTrigger(&t.Trigger) {
					continue
				}
				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"event": "end_of_combat", "rule": "603.6a",
					},
				})
				if t.Effect != nil {
					// Phase 5: end-of-combat triggers go on the stack.
					PushTriggeredAbility(gs, p, t.Effect)
				}
			}
		}
	}

	// Clear combat flags on every creature (§506.4).
	for _, seat := range gs.Seats {
		if seat.Flags != nil {
			delete(seat.Flags, "varina_triggered_this_combat")
		}
		for _, p := range seat.Battlefield {
			setPermFlag(p, flagAttacking, false)
			setPermFlag(p, flagDeclaredAttacker, false)
			setPermFlag(p, flagBlocking, false)
			setPermFlag(p, flagAttackedThisCombat, false)
		}
	}

	// Expire "until_end_of_combat" continuous effects (§500.5a).
	// MEDIUM #5 fix: clean ContinuousEffects in addition to Modifications.
	modsRemoved := false
	if len(gs.ContinuousEffects) > 0 {
		kept := gs.ContinuousEffects[:0]
		for _, ce := range gs.ContinuousEffects {
			if ce == nil {
				continue
			}
			if ce.Duration == "until_end_of_combat" {
				modsRemoved = true
				continue
			}
			kept = append(kept, ce)
		}
		gs.ContinuousEffects = kept
	}

	// Expire "until_end_of_combat" Modification entries.
	for _, seat := range gs.Seats {
		for _, p := range seat.Battlefield {
			if len(p.Modifications) == 0 {
				continue
			}
			kept := p.Modifications[:0]
			for _, m := range p.Modifications {
				if m.Duration == "until_end_of_combat" {
					modsRemoved = true
					continue
				}
				kept = append(kept, m)
			}
			p.Modifications = kept
		}
	}

	// Invalidate the characteristics cache so the subsequent SBA pass
	// sees updated P/T values. Without this, a creature whose toughness
	// was buffed "until end of combat" (e.g. Stitcher's Supplier at 1/1
	// + a +2/+2 buff) could remain on the battlefield at 0 toughness
	// because the SBA reads stale cached characteristics.
	if modsRemoved {
		gs.InvalidateCharacteristicsCache()
	}
}

func isEndOfCombatTrigger(tr *gameast.Trigger) bool {
	if tr == nil {
		return false
	}
	if tr.Event == "phase" && (tr.Phase == "end_of_combat" || tr.Phase == "combat_end") {
		return true
	}
	if tr.Event == "end_of_combat" || tr.Event == "combat_end" {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Ninjutsu — CR §702.49 (legacy wrapper)
// ---------------------------------------------------------------------------

// CheckNinjutsu is the backward-compatible entry point. It delegates to
// CheckNinjutsuRefactored in ninja_sneak.go which uses the shared
// FindUnblockedAttacker / BounceUnblockedAttacker helpers.
//
// Deprecated: use CheckNinjutsuRefactored directly for new code.
func CheckNinjutsu(gs *GameState, attackerSeat int, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) []*Permanent {
	return CheckNinjutsuRefactored(gs, attackerSeat, attackers, blockerMap)
}

// cardHasNinjutsu checks if a card has the ninjutsu keyword.
// Retained for backward compatibility; new code uses
// cardHasNinjutsuOrCommanderNinjutsu in ninja_sneak.go.
func cardHasNinjutsu(c *Card) bool {
	return cardHasNinjutsuOrCommanderNinjutsu(c, false)
}

// removePermanentFromSlice removes a permanent from a slice by pointer.
func removePermanentFromSlice(slice []*Permanent, p *Permanent) []*Permanent {
	for i, x := range slice {
		if x == p {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// sidarKondoOnBattlefield returns true if any Sidar Kondo of Jamuraa is
// on a battlefield. Reads the flag set by the per_card ETB handler.
func sidarKondoOnBattlefield(gs *GameState) bool {
	if gs == nil || gs.Flags == nil || gs.Flags["sidar_kondo_active"] == 0 {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == "Sidar Kondo of Jamuraa" {
				return true
			}
		}
	}
	return false
}

// silentArbiterOnBattlefield returns true if any Silent Arbiter is on
// a battlefield. Reads the flag set by per_card ETB handler.
func silentArbiterOnBattlefield(gs *GameState) bool {
	if gs == nil || gs.Flags == nil || gs.Flags["silent_arbiter_active"] == 0 {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == "Silent Arbiter" {
				return true
			}
		}
	}
	return false
}

// propagandaTaxFor returns the mana tax to attack a specific defending
// seat (Propaganda, Ghostly Prison, etc.). Scans the defender's
// battlefield for propaganda-type permanents.
func propagandaTaxFor(gs *GameState, defendingSeat int) int {
	if gs == nil || defendingSeat < 0 || defendingSeat >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[defendingSeat]
	if seat == nil {
		return 0
	}
	tax := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		switch p.Card.DisplayName() {
		case "Propaganda", "Ghostly Prison", "Windborn Muse",
			"Baird, Steward of Argive", "Norn's Annex":
			tax += 2
		case "Sphere of Safety":
			// Sphere taxes per enchantment the controller has.
			enchCount := 0
			for _, q := range seat.Battlefield {
				if q != nil && q.Card != nil {
					for _, t := range q.Card.Types {
						if t == "enchantment" {
							enchCount++
							break
						}
					}
				}
			}
			tax += enchCount
		}
	}
	return tax
}
