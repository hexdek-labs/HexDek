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
	// Two surfaces are checked: the legacy Permanent.Counters map (still
	// the dominant path until the Counter DB migration completes) and the
	// InstanceID-aware CounterStacks introduced by the Counter DB Phase 2
	// keyword-grant wiring. HasKeywordCounter additionally enforces that
	// the named counter type is registered as KeywordGrant — so a "+1/+1"
	// stack here will never satisfy a HasKeyword("+1/+1") lookup.
	if p.Counters != nil {
		if p.Counters[want] > 0 {
			return true
		}
	}
	if p.HasKeywordCounter(want) {
		return true
	}
	return false
}

// keywordActive is the layer-aware keyword query for combat: a permanent
// has the keyword if its raw sources declare it (p.HasKeyword — AST,
// GrantedAbilities, Flags["kw:"], keyword counters) OR a §613 layer-6
// continuous effect grants it (gs.HasKeywordOf — anthem-style "creatures
// you control have flying", per-card grants, etc.). This UNION is the
// r63 keyword-grant consumer fix: combat read only p.HasKeyword before,
// so every layer-6 keyword grant was inert in combat (the hex-dev-5
// blocker). The union is strictly additive — it can only ADD a keyword a
// grant confers, never remove one — so existing evasion behavior is
// preserved. gs == nil (legacy/test paths) falls back to p.HasKeyword.
func keywordActive(gs *GameState, p *Permanent, kw string) bool {
	if p == nil {
		return false
	}
	if p.HasKeyword(kw) {
		return true
	}
	return gs != nil && gs.HasKeywordOf(p, kw)
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
	// flagEnteredAttacking tags a permanent that was placed onto the
	// battlefield IN AN ATTACKING STATE by an effect (CR §508.1g — an
	// effect that "puts a creature onto the battlefield attacking"
	// bypasses the §508.1a-§508.1f declare-attackers restrictions,
	// including defender, summoning sickness, and tapped status). The
	// creature is attacking but was not declared via §508. The
	// CombatLegality invariant honors this tag and skips its
	// defender / summoning-sickness checks for the duration of the
	// combat phase. Set by per_card handlers that mint or pull a
	// creature onto the battlefield attacking: Raph & Mikey
	// (library-dig), Brimaz, Sauron, Najeela, Kaalia, Caesar, Altair,
	// Pakal, Satya, Strefan, Winota, Geist of Saint Traft, etc.
	// Cleared by EndOfCombatStep alongside flagAttacking.
	flagEnteredAttacking = "entered_battlefield_attacking"
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

// MarkEnteredAttacking is the canonical chokepoint for per_card
// handlers that place a creature onto the battlefield in an attacking
// state via a CR §508.1g effect. Sets both flagAttacking and
// flagEnteredAttacking so the CombatLegality invariant correctly
// skips its §508.1a-§508.1f restriction checks (defender,
// summoning sickness, tapped) for the duration of the combat phase.
//
// The §508.1g carve-out: per the comprehensive rules, an effect that
// puts a creature onto the battlefield attacking does NOT route
// through §508 declare-attackers, so the creature is exempt from
// defender / summoning-sickness checks. The invariant honors this
// via flagEnteredAttacking; the per_card handler MUST set the flag
// (preferably via this helper) for the carve-out to apply.
//
// Use sites: Raph & Mikey (library-dig), Brimaz / Sauron / Najeela /
// Kaalia / Caesar / Strefan / Winota / Altair / Pakal / Satya / Geist
// of Saint Traft (minted attacking tokens). Live bug fix: Raph &
// Mikey (PR #950 docs/loki-pathological-r60.md / game 1317 Wall of
// Tanglecord); the other sites mint defender-free tokens but should
// migrate to this helper for forward-correctness (defense-in-depth
// against future per_card enhancements that copy stats from arbitrary
// permanents).
func MarkEnteredAttacking(p *Permanent) {
	if p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags[flagAttacking] = 1
	p.Flags[flagEnteredAttacking] = 1
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

// FireBeginCombatTriggersForTest is the exported seam the PROGRESSION
// checker drives as its begin-combat stimulus (the same dispatch
// CombatPhase runs at §603.6a).
func FireBeginCombatTriggersForTest(gs *GameState, activeSeat int) {
	fireBeginningOfCombatTriggers(gs, activeSeat)
}

// fireBeginningOfCombatTriggers fires "at the beginning of combat"
// Triggered abilities. CR §603.6a.
//
// Scope semantics (r63 PROGRESSION widening): the parser emits the
// corpus's begin-combat triggers as Phase "combat_start_yours" (206
// cards, "at the beginning of combat on your turn") and
// "combat_start_each" (22, "at the beginning of each combat") — neither
// matched the old isCombatBeginTrigger (it knew only the bare
// "combat_start"/"begin_of_combat" spellings), so ALL of them were
// silent. "your"-scoped triggers fire only when the bearer's controller
// is the active player; "each"-scoped triggers fire on every combat, so
// the walk now covers every seat's battlefield.
func fireBeginningOfCombatTriggers(gs *GameState, activeSeat int) {
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	// Snapshot — effects may spawn tokens while we iterate.
	var perms []*Permanent
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		perms = append(perms, seat.Battlefield...)
	}
	for _, p := range perms {
		if p == nil || p.Card == nil || p.Card.AST == nil {
			continue
		}
		for _, ab := range p.Card.AST.Abilities {
			t, ok := ab.(*gameast.Triggered)
			if !ok {
				continue
			}
			scope := combatBeginTriggerScope(&t.Trigger)
			if scope == "" {
				continue
			}
			if scope == "your" && p.Controller != activeSeat {
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
				PushTriggeredAbilityWithIf(gs, p, t.Effect, t.InterveningIf)
			}
		}
	}
}

// combatBeginTriggerScope classifies a begin-combat trigger: "" (not
// one), "your" (controller-gated — fires only on the bearer
// controller's combat), or "each" (every combat). The bare
// "combat_start"/"begin_of_combat" spellings keep their historical
// active-player-only behavior ("your").
func combatBeginTriggerScope(tr *gameast.Trigger) string {
	if tr == nil {
		return ""
	}
	if tr.Event == "phase" {
		switch tr.Phase {
		case "combat_start", "begin_of_combat", "combat_start_yours":
			return "your"
		case "combat_start_each":
			return "each"
		}
		return ""
	}
	if tr.Event == "combat_start" || tr.Event == "begin_of_combat" {
		return "your"
	}
	return ""
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
	// Note: gs.CurrentCombatRestriction may be non-empty if we're inside
	// an extra combat with an attacker restriction (e.g. Bumi Unleashed's
	// "only land creatures can attack"). passesCombatRestriction filters
	// the pool to permanents that satisfy the restriction. Empty string
	// = no restriction = filter is a no-op.
	livingOpps := gs.LivingOpponents(attackerSeat)
	var legal []*Permanent
	for _, p := range seat.Battlefield {
		// canAttack covers printed/Flags defender; the layer-granted case
		// (scaffold_keyword_grant "defender") is invisible to HasKeyword
		// but real to the engine's layer system and to the §508.1a
		// legality validator. Without this gate a creature with
		// layer-granted defender entered the legal pool, the §508.1
		// sanitizer kept it (it IS in-pool), and it ATTACKED illegally —
		// a real state bug (deep loki r63c: 52 §508.1a hits, every one a
		// granted-defender attacker). Mirror the validator's gs.HasKeywordOf.
		if canAttackGS(gs, p) && passesCombatRestriction(gs, p) && !gs.HasKeywordOf(p, "defender") {
			legal = append(legal, p)
		}
	}

	// Hat picks which creatures to attack with (§506.1).
	chosen := legal // default: all legal attackers (greedy)
	if seat.Hat != nil {
		chosen = seat.Hat.ChooseAttackers(gs, attackerSeat, legal)
	}

	// CR §508.1 engine backstop (r62): the Hat's return was previously
	// applied UNVERIFIED — a policy bug (or any future code path handing
	// in a map) could declare a tapped / summoning-sick / defender /
	// off-pool creature and the engine would execute it; the phase-2
	// legality validator could SEE it but nothing STOPPED it. Drop any
	// entry that is nil, a duplicate, or not a member of the legal pool
	// computed above (membership is exact: every current Hat subsets its
	// input, so legal declarations are untouched by construction).
	// Dropped entries are still routed through the ride-along validator
	// so policy bugs remain visible even though they no longer execute.
	chosen = sanitizeDeclaredAttackers(gs, attackerSeat, chosen, legal)

	// CR §701.39 — Goad enforcement (must attack if able). Any legal
	// goaded creature the Hat omitted is force-added. "If able" is
	// satisfied by being in `legal` (canAttack passed). The Silent
	// Arbiter cap below may still trim, but the engine no longer
	// silently ignores the must-attack rider when a Hat returns an
	// incomplete attacker set.
	chosen = enforceGoadMustAttack(gs, chosen, legal)

	// Silent Arbiter: max one attacker per combat.
	//
	// CR §509.1d: when restrictions and requirements both apply, the
	// player must choose a legal attacker set that satisfies the maximum
	// number of restrictions AND the maximum number of requirements that
	// don't conflict with those restrictions. Naively keeping chosen[0]
	// drops any goaded creature that enforceGoadMustAttack appended at
	// the tail — leaving a non-goaded creature as the sole attacker and
	// silently violating the goad "must attack if able" requirement.
	// Prefer a goaded entry from `chosen` so the requirement is honored
	// inside the restriction's 1-attacker budget.
	if len(chosen) > 1 && silentArbiterOnBattlefield(gs) {
		pick := chosen[0]
		for _, p := range chosen {
			if MustAttackIfAble(gs, p) {
				pick = p
				break
			}
		}
		chosen = []*Permanent{pick}
		gs.LogEvent(Event{
			Kind: "attack_restricted",
			Seat: attackerSeat,
			Details: map[string]interface{}{
				"reason":    "silent_arbiter",
				"max":       1,
				"goad_pref": MustAttackIfAble(gs, pick),
				"rule":      "509.1d",
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
		// CR §701.39 — Goad's "attacks a player other than you if able"
		// rider: filter the goader out of the defender pool unless
		// doing so empties it. Both the default heuristic and the Hat
		// callback see the filtered list so the rule holds regardless
		// of Hat compliance.
		effectiveOpps := filterGoaderFromDefenders(gs, p, livingOpps)
		def := pickAttackDefender(gs, p, effectiveOpps)
		if seat.Hat != nil {
			def = seat.Hat.ChooseAttackTarget(gs, attackerSeat, p, effectiveOpps)
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

		// Ride-along legality validator (legality.go): the declaration
		// is final here and the attack-tap hasn't happened yet, so
		// Tapped still reflects pre-declaration state. The Hat's
		// ChooseAttackers return is applied UNVERIFIED above — this is
		// the only re-derivation of CR 508.1 for what was declared.
		// nil-receiver no-op when the validator is off.
		gs.Legality.ObserveAttackDeclaration(gs, attackerSeat, p)

		if !p.HasKeyword("vigilance") {
			p.Tapped = true
			// Dispatch tap_event for cards like Magda, Brazen Outlaw
			// and Emmara, Soul of the Accord.
			FireCardTrigger(gs, "tap_event", map[string]interface{}{
				"seat": p.Controller,
				"perm": p,
			})
			FireTapEventASTTriggers(gs, p)
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
		seat.Turn.Attacked = true

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
	//
	// R60: clear SummoningSick on the scooped-in creatures. Per CR §506.3,
	// the "enters the battlefield attacking" effect grants the creature
	// license to attack this combat despite normally being §302.1
	// summoning-sick — the seven per_card handlers that produce these
	// (Raph & Mikey, Kaalia, Sauron, Strefan, Winota, Satya, ninjutsu via
	// ninja_sneak.go) all stamp `Flags["attacking"]=1` on a freshly-created
	// permanent whose SummoningSick is still true. The scoop-in is the
	// canonical "you are legally attacking" moment — without clearing SS
	// here, a mid-combat game end (TakeTurn returns early on
	// gs.CheckEnd() before EndOfCombatStep clears flagAttacking) leaves
	// the bad SS=true + attacking=true combo for checkCombatLegality to
	// flag. Bit-stable signature across r41 -> r44 -> r60 round 1/2/3 as
	// "Behemoth of Vault 0 is attacking with summoning sickness and no
	// haste" — Behemoth is the cheated-creature target in chaos games
	// where one of the seven enabler cards lives in seat 0's deck.
	attackers := append([]*Permanent{}, declared...)
	for _, p := range seat.Battlefield {
		if permFlag(p, flagAttacking) && !permFlag(p, flagDeclaredAttacker) {
			if p.SummoningSick {
				p.SummoningSick = false
			}
			attackers = append(attackers, p)
			gs.LogEvent(Event{
				Kind: "entered_attacking", Seat: attackerSeat,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule":              "506.3",
					"cleared_summoning": true,
				},
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

	// §702.101 — Battalion: "Whenever this creature and at least two other
	// creatures attack, ..." Iterates each battalion-bearing attacker and
	// fires a battalion_triggered card trigger when the controller has
	// 3+ attacking creatures total. Scans the full `attackers` slice
	// (declared + §506.3 scoop-in) so creatures that entered attacking
	// count toward the threshold.
	FireBattalionTriggers(gs, attackerSeat, attackers)

	// §702.149 — Pack Tactics: total-power sibling of Battalion. Fires
	// for each attacking pack-tactics source when the controller's
	// total attacking power is >= PackTacticsPowerThreshold (6). Power
	// reflects current Permanent.Power() so already-resolved buffs
	// (Glorious Anthem, etc.) count.
	FirePackTacticsForAttackers(gs, attackers)

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

// canAttackGS is canAttack with §613 layer awareness: a summoning-sick
// creature whose haste is LAYER-GRANTED (an anthem, or an incarnation
// graveyard static like Anger) — invisible to the raw p.HasKeyword path —
// may still attack. Mirrors CanBlockGS (landwalk) and the §302.6 validator
// gate in legality.go, which already union p.HasKeyword with gs.HasKeywordOf.
// canAttack (raw) is kept for the nil-gs / legacy callers.
func canAttackGS(gs *GameState, p *Permanent) bool {
	if p == nil || !p.IsCreature() {
		return false
	}
	if p.Tapped || p.PhasedOut {
		return false
	}
	if p.SummoningSick && !keywordActive(gs, p, "haste") {
		return false
	}
	if p.HasKeyword("defender") {
		return false
	}
	if p.Flags != nil && p.Flags["detained"] == 1 {
		return false
	}
	// Kulrath Knight (§508 static): an opponent's counter-bearing creature
	// can't be declared as an attacker.
	if lockedByOpponentCounterStatic(gs, p) {
		return false
	}
	if p.Power() <= 0 {
		return false
	}
	return true
}

// isCounterCombatLockSource reports whether a permanent's printed static is
// "Creatures your opponents control with counters on them can't attack or
// block" (Kulrath Knight, CR §509/§508 continuous combat restriction). Kept as
// a small named set so siblings with the identical wording can be added here.
func isCounterCombatLockSource(name string) bool {
	switch name {
	case "Kulrath Knight":
		return true
	}
	return false
}

// hasAnyCounter reports whether the permanent currently carries at least one
// counter of ANY kind — +1/+1, -1/-1 (Wither/Infect), stun, shield, finality,
// a keyword counter, charge, etc. Checks both the legacy Counters map and the
// Counter-DB CounterStacks store.
func hasAnyCounter(p *Permanent) bool {
	if p == nil {
		return false
	}
	for _, n := range p.Counters {
		if n > 0 {
			return true
		}
	}
	for _, st := range p.CounterStacks {
		if st.Count > 0 {
			return true
		}
	}
	return false
}

// lockedByOpponentCounterStatic reports whether creature p may neither attack
// nor block because it carries a counter and an OPPONENT of its controller has
// a Kulrath-Knight-style "creatures your opponents control with counters on
// them can't attack or block" static on the battlefield. Re-evaluated on every
// legality check, so it turns off the instant the last counter is removed or
// the source leaves play (CR §613 continuous effect).
func lockedByOpponentCounterStatic(gs *GameState, p *Permanent) bool {
	if gs == nil || p == nil || !p.IsCreature() {
		return false
	}
	if !hasAnyCounter(p) {
		return false
	}
	for _, opp := range gs.Opponents(p.Controller) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, src := range s.Battlefield {
			if src == nil || src.Card == nil || src.PhasedOut {
				continue
			}
			if isCounterCombatLockSource(src.Card.DisplayName()) {
				return true
			}
		}
	}
	return false
}

// passesCombatRestriction checks gs.CurrentCombatRestriction (set by the
// turn loop when entering a restricted extra combat) against the
// candidate attacker. Returns true when there's no restriction in
// effect, or when the candidate satisfies it.
//
// Tag values must match the strings produced by per_card handlers /
// resolveExtraCombat in `PendingExtraCombat.Restriction`. Adding a new
// tag = add a case here. Currently supported:
//
//	""                       — no restriction (vanilla extra combat)
//	"land_creatures_only"    — only attackers with the Land type
//	                           (Bumi, Unleashed's "Only land creatures
//	                           can attack during that combat phase.")
func passesCombatRestriction(gs *GameState, p *Permanent) bool {
	if gs == nil || gs.CurrentCombatRestriction == "" {
		return true
	}
	if p == nil || p.Card == nil {
		return false
	}
	switch gs.CurrentCombatRestriction {
	case "land_creatures_only":
		// "Land creature" means a permanent that is both a Land and a
		// Creature (e.g. an earthbent land via Toph's static, a
		// Dryad Arbor, or a manlands like Mutavault that's been
		// activated). Check both types.
		return p.IsCreature() && p.IsLand()
	default:
		// Unknown tag — fail closed (don't allow the attacker).
		// Forces us to add the case explicitly when introducing new
		// restriction tags, rather than silently passing through.
		return false
	}
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
		// Judge r63 double-fire gate: per_card owns this card's attack
		// trigger (Brimaz, Krenko, Geist of Saint Traft…) — the
		// FireCardTrigger("creature_attacks") below dispatches the
		// handler; pushing the AST effect too resolves the ability
		// twice. Mirrors the #1059 gate on the observer classes.
		perCardOwnsAttack := PerCardOwnsTrigger(atk.Card.DisplayName(), "creature_attacks")
		for _, ab := range iterAttackTriggers(atk.Card) {
			if perCardOwnsAttack {
				break
			}
			gs.LogEvent(Event{
				Kind: "trigger_fires", Seat: atk.Controller,
				Source: atk.Card.DisplayName(),
				Details: map[string]interface{}{
					"event": "attack", "rule": "603.3a",
				},
			})
			if ab.Effect != nil {
				// Phase 5: trigger goes on the stack (CR §603.3a).
				PushTriggeredAbilityWithIf(gs, atk, ab.Effect, ab.InterveningIf)
			}
		}
		FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
			"attacker_perm": atk,
			"attacker_seat": atk.Controller,
			"attacker_card": atk.Card,
		})
	}
	// (2) Non-self attack triggers — ally ("a creature you control
	// attacks", with or without filters), attached ("equipped/enchanted
	// creature attacks"), and any-controller ("a creature attacks")
	// classes, recovered from the trigger Raw because the parser drops
	// actor phrases. See attack_trigger_dispatch.go (r63 PROGRESSION
	// dispatch-consistency audit) — replaces the old two-substring
	// whitelist with classifier-driven dispatch; the two legacy
	// wordings keep their exact historical behavior.
	declaredSet := make(map[*Permanent]struct{}, len(declared))
	for _, a := range declared {
		declaredSet[a] = struct{}{}
	}
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	fireClassifiedAttackTriggers(gs, activeSeat, declared, declaredSet)
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
				// Ride-along legality validator: observe the RAW policy
				// output BEFORE sanitizing and BEFORE flagBlocking is
				// stamped — policy bugs stay visible in the violation
				// stream even though the engine no longer executes them.
				gs.Legality.ObserveBlockDeclaration(gs, defenderSeat, a, blockers)
				// CR §509.1 engine backstop (r62): the Hat's block map was
				// previously applied with no engine validation at all —
				// the #1028 landwalk fix stopped hats from CHOOSING
				// illegal blocks, but the engine still executed whatever
				// map it was handed. Drop entries that fail §509.1a
				// (tapped / non-creature / phased out / not the defender's
				// battlefield creature), §509.1 one-attacker-per-blocker
				// (unless its text allows more), or §509.1b evasion via
				// canBlockGS (flying / menace / landwalk / protection /
				// fear / shadow / skulk / horsemanship / Sidar Kondo);
				// then enforce §702.110b menace pairing on the survivors.
				blockers = sanitizeDeclaredBlockers(gs, defenderSeat, a, blockers)
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
		if keywordActive(gs, atk, "unblockable") {
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

		// Menace: need 2 blockers; else skip. Layer-aware (r63).
		menace := keywordActive(gs, atk, "menace")
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
		gs.Legality.ObserveBlockDeclaration(gs, defenderSeat, atk, assigned)
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
	// CR §701.62a: a suspected creature can't block. The designation
	// persists across turns until UnsuspectCreature (investigate) clears
	// it, so this gate fires every block-declaration check independent
	// of the until-EOT cleanup pass.
	if IsSuspected(blocker) {
		return false
	}
	// §702.26: phased-out permanents don't exist.
	if blocker.PhasedOut || attacker.PhasedOut {
		return false
	}
	// Kulrath Knight (§509 static): an opponent's counter-bearing creature
	// can't be declared as a blocker.
	if lockedByOpponentCounterStatic(gs, blocker) {
		return false
	}
	// Flying: blocked only by flying or reach. Layer-aware (keywordActive)
	// so a granted-flying attacker ("creatures you control have flying")
	// is blockable only by granted/printed flying or reach (r63).
	if keywordActive(gs, attacker, "flying") {
		if !(keywordActive(gs, blocker, "flying") || keywordActive(gs, blocker, "reach")) {
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
	// "Cowards can't block Warriors" (Kargan Intimidator / Gornog the Red
	// Reaper) — a static block restriction. Seat-flag fast-path gate + an
	// on-battlefield re-verify of the granting permanent, mirroring the Sidar
	// Kondo wiring above. Uses layer-effective types so an until-EOT
	// "becomes a Coward" (RegisterAddTypes) is honored too.
	if gs != nil && gs.Flags != nil && gs.Flags["cowards_cant_block_warriors"] == 1 {
		if gs.HasTypeOf(attacker, "warrior") && gs.HasTypeOf(blocker, "coward") &&
			cowardBlockRestrictionActive(gs) {
			return false
		}
	}
	// Unblockable-style effects.
	if keywordActive(gs, attacker, "unblockable") {
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
	// attacker has protection from. Layer-aware (ColorsOf) so a blocker
	// whose color was changed reads correctly.
	if attackerHasProtectionFromGS(gs, attacker, blocker) {
		return false
	}
	// Protection on blocker from attacker — the blocker also can't
	// block if IT has protection from the attacker (§702.16e:
	// "can't block" is only relevant for the attacker side;
	// a blocker's protection doesn't prevent blocking).
	// NOTE: only attacker-side protection prevents blocking per CR.
	return true
}

// sanitizeDeclaredAttackers is the CR §508.1 engine backstop (r62): it
// filters a policy-chosen attacker list down to non-nil, non-duplicate
// members of the legal pool DeclareAttackers computed (canAttack +
// passesCombatRestriction over the controller's battlefield). Pointer
// membership is exact and cannot false-drop: every Hat builds its return
// from the legal slice it is handed, so a drop here means a genuinely
// out-of-pool declaration (policy bug or a future code path handing in a
// raw list). Dropped entries are routed through the ride-along legality
// validator so the attempt stays visible, and logged as
// attack_declaration_dropped.
func sanitizeDeclaredAttackers(gs *GameState, attackerSeat int, chosen, legal []*Permanent) []*Permanent {
	if len(chosen) == 0 {
		return chosen
	}
	inPool := make(map[*Permanent]bool, len(legal))
	for _, p := range legal {
		inPool[p] = true
	}
	kept := make([]*Permanent, 0, len(chosen))
	seen := make(map[*Permanent]bool, len(chosen))
	for _, p := range chosen {
		if p == nil {
			continue
		}
		if seen[p] {
			dropDeclaredAttacker(gs, attackerSeat, p, "duplicate attacker entry", "508.1a")
			continue
		}
		seen[p] = true
		if !inPool[p] {
			// Validator sees the attempt (re-derives 508.1/302.6/702.26
			// per checkLegalityAttackDecl) even though it won't execute.
			gs.Legality.ObserveAttackDeclaration(gs, attackerSeat, p)
			dropDeclaredAttacker(gs, attackerSeat, p, "not in the legal attacker pool (tapped / summoning-sick / defender / restriction / off-battlefield)", "508.1")
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func dropDeclaredAttacker(gs *GameState, seatIdx int, p *Permanent, reason, rule string) {
	name := "<unknown>"
	if p != nil && p.Card != nil {
		name = p.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "attack_declaration_dropped",
		Seat:   seatIdx,
		Source: name,
		Details: map[string]interface{}{
			"reason": reason,
			"rule":   rule,
		},
	})
}

// sanitizeDeclaredBlockers is the CR §509.1 engine backstop (r62): it
// filters a policy-assigned blocker set for one attacker down to entries
// that are legally able to block it RIGHT NOW, mirroring the phase-2
// validator's checkLegalityBlockDecl predicates exactly:
//
//   - §509.1a: creature, not phased out, not tapped, and on the
//     DEFENDER's battlefield (the fallback policy gets this implicitly
//     from its pool; the Hat branch previously got nothing);
//   - §509.1: not already blocking and not duplicated within this
//     assignment, unless its text allows blocking additional creatures
//     (legalityCanMultiBlock);
//   - §509.1b: canBlockGS evasion/restriction gate (flying, menace
//     count handled below, landwalk, protection, fear/intimidate,
//     shadow, skulk, horsemanship, Sidar Kondo, unblockable);
//   - §702.110b: if the attacker has menace (printed or layer-granted)
//     and exactly one blocker survives the filters, the single block is
//     illegal as a whole and is dropped too.
//
// The caller observes the RAW set through the validator before calling
// this, so dropped entries remain visible as violations.
func sanitizeDeclaredBlockers(gs *GameState, defenderSeat int, attacker *Permanent, blockers []*Permanent) []*Permanent {
	if attacker == nil || len(blockers) == 0 {
		return nil
	}
	onBF := map[*Permanent]bool{}
	if gs != nil && defenderSeat >= 0 && defenderSeat < len(gs.Seats) && gs.Seats[defenderSeat] != nil {
		for _, p := range gs.Seats[defenderSeat].Battlefield {
			onBF[p] = true
		}
	}
	kept := make([]*Permanent, 0, len(blockers))
	seen := make(map[*Permanent]bool, len(blockers))
	for _, b := range blockers {
		if b == nil {
			continue
		}
		switch {
		case seen[b] && !legalityCanMultiBlock(b):
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker committed twice in one assignment", "509.1")
		case !onBF[b]:
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker is not a creature on the defender's battlefield", "509.1a")
		case !b.IsCreature():
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker is not a creature", "509.1a")
		case b.PhasedOut:
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker is phased out", "702.26b")
		case b.Tapped:
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker is tapped", "509.1a")
		case b.IsBlocking() && !legalityCanMultiBlock(b):
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "blocker is already blocking another attacker", "509.1")
		case !canBlockGS(gs, attacker, b):
			dropDeclaredBlocker(gs, defenderSeat, attacker, b, "evasion/blocking restriction unsatisfied", "509.1b")
		default:
			seen[b] = true
			kept = append(kept, b)
			continue
		}
		seen[b] = true
	}
	// §702.110b — menace: can't be blocked except by two or more.
	if len(kept) == 1 && keywordActive(gs, attacker, "menace") {
		dropDeclaredBlocker(gs, defenderSeat, attacker, kept[0], "menace attacker blocked by exactly one creature", "702.110b")
		kept = kept[:0]
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func dropDeclaredBlocker(gs *GameState, defenderSeat int, attacker, b *Permanent, reason, rule string) {
	bName, aName := "<unknown>", "<unknown>"
	if b != nil && b.Card != nil {
		bName = b.Card.DisplayName()
	}
	if attacker != nil && attacker.Card != nil {
		aName = attacker.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "block_declaration_dropped",
		Seat:   defenderSeat,
		Source: bName,
		Details: map[string]interface{}{
			"attacker": aName,
			"reason":   reason,
			"rule":     rule,
		},
	})
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

// attackerHasProtectionFrom returns true if the protected permanent has
// protection from `source` per §702.16. Checks two axes:
//
//  1. Color-based protection (§702.16b) — "protection from red", etc.
//  2. Type-based protection (§702.16) — "protection from creatures",
//     "from artifacts", "from enchantments", "from planeswalkers",
//     "from lands". Reads ProtectionTypes (keywords_combat.go) and
//     checks whether the source has any of those types.
//
// Sentinel "*" in protectionColors means "protection from everything"
// (Teferi's Protection, Progenitus) and short-circuits true.
func attackerHasProtectionFrom(protected, source *Permanent) bool {
	return attackerHasProtectionFromGS(nil, protected, source)
}

// attackerHasProtectionFromGS is the layer-aware variant: when gs is non-nil
// it reads the SOURCE's effective (CR §613 layer-5) colors via ColorsOf, so a
// creature whose color was changed by a color-changing effect (Painter's
// Servant naming the color, "becomes black", etc.) is correctly matched
// against "protection from <color>". The gs-less wrapper falls back to the
// source's printed colors for the legacy CanBlock(nil, …) / hat-heuristic
// paths that have no game state.
func attackerHasProtectionFromGS(gs *GameState, protected, source *Permanent) bool {
	if protected == nil || source == nil {
		return false
	}
	prot := protectionColors(protected)
	if _, any := prot["*"]; any {
		return true
	}
	if len(prot) > 0 {
		var srcColors map[string]struct{}
		if gs != nil {
			srcColors = map[string]struct{}{}
			for _, c := range gs.ColorsOf(source) {
				srcColors[strings.ToUpper(c)] = struct{}{}
			}
		} else {
			srcColors = cardColors(source.Card)
		}
		for c := range srcColors {
			if _, hit := prot[c]; hit {
				return true
			}
		}
	}
	// Type-based protection (§702.16): protection from creatures /
	// artifacts / enchantments / planeswalkers / lands. Combat-relevant
	// types only — instant/sorcery never appear as combat sources.
	if types := ProtectionTypes(protected); len(types) > 0 {
		if _, ok := types["creature"]; ok && source.IsCreature() {
			return true
		}
		if _, ok := types["artifact"]; ok && source.IsArtifact() {
			return true
		}
		if _, ok := types["enchantment"]; ok && source.IsEnchantment() {
			return true
		}
		if _, ok := types["planeswalker"]; ok && source.IsPlaneswalker() {
			return true
		}
		if _, ok := types["land"]; ok && source.IsLand() {
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
		dmg := gs.PowerOf(atk)
		if dmg <= 0 {
			continue
		}
		// §310.5b — Battle defender path: if this attacker was
		// declared against a battle, route damage to the battle's
		// defense counters via ApplyCombatDamageToBattle. We honor
		// blockers (if any) by falling back to the standard
		// player-target path; in real games blockers can still
		// intercept attackers pointed at a battle. The blocker
		// branch below handles that case.
		if battleTS, ok := AttackerDefenderBattle(atk); ok {
			declaredBlockers := blockerMap[atk]
			liveBlockers := make([]*Permanent, 0, len(declaredBlockers))
			for _, b := range declaredBlockers {
				if alive(gs, b) {
					liveBlockers = append(liveBlockers, b)
				}
			}
			if len(declaredBlockers) == 0 {
				// Unblocked battle-attacker — all damage to the battle.
				if battle, ok := LookupBattleByTimestamp(gs, battleTS); ok {
					ApplyCombatDamageToBattle(gs, atk, dmg, battle)
				}
				continue
			}
			if len(liveBlockers) == 0 {
				// All declared blockers removed before damage. Trample
				// spills to the battle.
				if atk.HasKeyword("trample") {
					if battle, ok := LookupBattleByTimestamp(gs, battleTS); ok {
						ApplyCombatDamageToBattle(gs, atk, dmg, battle)
					}
				}
				continue
			}
			// Blocked battle-attacker: damage flows through blockers
			// like a normal attack. Trample remainder goes to the
			// battle. Fall through to the blocker-handling code below,
			// using a synthetic "battle defender" sentinel that the
			// post-blocker trample branch checks.
			ordered := append([]*Permanent{}, liveBlockers...)
			for i := 0; i < len(ordered)-1; i++ {
				for j := i + 1; j < len(ordered); j++ {
					if gs.ToughnessOf(ordered[j])-ordered[j].MarkedDamage <
						gs.ToughnessOf(ordered[i])-ordered[i].MarkedDamage {
						ordered[i], ordered[j] = ordered[j], ordered[i]
					}
				}
			}
			remaining := dmg
			for _, b := range ordered {
				if remaining <= 0 {
					break
				}
				need := lethalAmountGS(gs, atk, b)
				give := remaining
				if give > need {
					give = need
				}
				applyCombatDamageToCreature(gs, atk, give, b)
				remaining -= give
			}
			if remaining > 0 && atk.HasKeyword("trample") {
				if battle, ok := LookupBattleByTimestamp(gs, battleTS); ok {
					ApplyCombatDamageToBattle(gs, atk, remaining, battle)
				}
			}
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
				if gs.ToughnessOf(ordered[j])-ordered[j].MarkedDamage <
					gs.ToughnessOf(ordered[i])-ordered[i].MarkedDamage {
					ordered[i], ordered[j] = ordered[j], ordered[i]
				}
			}
		}

		remaining := dmg
		for _, b := range ordered {
			if remaining <= 0 {
				break
			}
			need := lethalAmountGS(gs, atk, b)
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
			dmg := gs.PowerOf(b)
			if dmg <= 0 {
				continue
			}
			applyCombatDamageToCreature(gs, b, dmg, atk)
		}
	}

	// §702.21k/l — Banding damage redistribution. With banded blockers,
	// the *defending* player chooses how each attacker's damage divides
	// among the band; with banded attackers, the *attacking* player
	// chooses how blocker damage divides among the band. MVP grouping:
	// same controller + same combat role (same defender for attackers,
	// same blocked attacker for blockers) + 2+ with the banding keyword.
	// Wither / infect paths route damage to -1/-1 counters instead of
	// MarkedDamage, so they skip this pass cleanly.
	applyBandingRedistribution(gs, attackers, blockerMap)
}

// applyBandingRedistribution finds bands of creatures that took damage
// this step and reallocates their MarkedDamage to minimize kills.
// Runs at the end of DealCombatDamageStep so the post-step SBA pass
// (back in CombatPhase) sees the redistributed totals.
//
// Band detection (MVP, controller-favored):
//
//   - Banded attackers: group by (controller, defender_seat).
//   - Banded blockers: per attacker, group by controller.
//
// A group qualifies as a band if it has 2+ members and at least one
// has banding. Real Magic requires explicit band declaration at
// attack-step, but for damage-redistribution purposes "all eligible
// creatures form one band" is the controller-favored outcome — exactly
// what the rules permit if the controller declared the band.
func applyBandingRedistribution(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}
	type bandKey struct {
		controller int
		defender   int
	}
	atkBands := map[bandKey][]*Permanent{}
	for _, a := range attackers {
		if !alive(gs, a) {
			continue
		}
		def, ok := AttackerDefender(a)
		if !ok {
			continue
		}
		k := bandKey{controller: a.Controller, defender: def}
		atkBands[k] = append(atkBands[k], a)
	}
	for _, band := range atkBands {
		if !bandHasBanding(band) {
			continue
		}
		ApplyBandingDamageRedistribution(gs, band)
	}

	for _, atk := range attackers {
		blkGroups := map[int][]*Permanent{}
		for _, b := range blockerMap[atk] {
			if !alive(gs, b) {
				continue
			}
			blkGroups[b.Controller] = append(blkGroups[b.Controller], b)
		}
		for _, band := range blkGroups {
			if !bandHasBanding(band) {
				continue
			}
			ApplyBandingDamageRedistribution(gs, band)
		}
	}
}

// bandHasBanding returns true if any creature in the group has banding.
// Requires 2+ members to qualify as a band per §702.21j.
func bandHasBanding(group []*Permanent) bool {
	if len(group) < 2 {
		return false
	}
	for _, p := range group {
		if HasBanding(p) {
			return true
		}
	}
	return false
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

// lethalAmountGS is the layer-aware variant used by DealCombatDamageStep.
func lethalAmountGS(gs *GameState, attacker, blocker *Permanent) int {
	if attacker.HasKeyword("deathtouch") {
		return 1
	}
	need := gs.ToughnessOf(blocker) - blocker.MarkedDamage
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
	// §614 damage replacement (R54) — Torbran +2 to opponents,
	// Lightning stagger doubling, Kuja Flare-Star doubling, Neriv
	// ETB-this-turn doubling, Sokrates dialogue prevent-and-draw
	// all funnel through here BEFORE the §615 prevention shield
	// reduction. Per §616 the application order is replacement
	// effects → prevention; the controller of the affected object
	// picks an ordering among the replacements (approximated by
	// registration order here).
	if len(gs.DamageReplacements) > 0 && src != nil && src.Card != nil {
		ctx := &DamageContext{
			Source:     src,
			SourceName: src.Card.DisplayName(),
			TargetSeat: seatIdx,
			Kind:       DamageCombatPlayer,
			Amount:     amount,
		}
		ApplyDamageReplacement(gs, ctx)
		if ctx.Prevented || ctx.Amount <= 0 {
			return
		}
		amount = ctx.Amount
	}
	// §615: apply prevention shields before dealing combat damage.
	amount = PreventDamageToPlayer(gs, seatIdx, amount, src)
	if amount <= 0 {
		return
	}
	// CR §704.6c / §903.10a — track commander damage. Damage is "dealt"
	// at this point (post-prevention, post-protection); both the infect
	// branch and the standard branch below count toward the 21-threshold
	// per §702.90c (the damage event still occurs, only its replacement
	// effect changes — poison counters vs life loss).
	if src.Card != nil && IsCommanderCard(gs, src.Controller, src.Card) {
		AccumulateCommanderDamage(gs, seatIdx, src.Controller, src.Card.DisplayName(), amount)
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
		// Life-loss tracking: only in the non-infect path because infect
		// replaces damage with poison counters (§702.90 — no life is lost).
		gs.Seats[seatIdx].Turn.LifeLost += amount
		if gs.Seats[seatIdx].Flags == nil {
			gs.Seats[seatIdx].Flags = map[string]int{}
		}
		gs.Seats[seatIdx].Flags["lost_life_this_turn"] += amount
		gs.Seats[seatIdx].Flags["life_lost_this_turn"] += amount
		// Fire life_lost trigger so Valgavoth, Lich's Mastery, etc. react
		// to combat damage as life loss.
		FireCardTrigger(gs, "life_lost", map[string]interface{}{
			"seat":   seatIdx,
			"amount": amount,
			"source": src.Card.DisplayName(),
		})
		// Fire life_change trigger so Exquisite Blood can react to combat damage.
		FireCardTrigger(gs, "life_change", map[string]interface{}{
			"seat":   seatIdx,
			"amount": -amount,
			"source": src.Card.DisplayName(),
		})
	}
	// Set damage_taken_this_turn flag so Bloodthirst (§702.54) and similar
	// mechanics can detect that this player was dealt damage (even via infect).
	if gs.Seats[seatIdx].Flags == nil {
		gs.Seats[seatIdx].Flags = map[string]int{}
	}
	gs.Seats[seatIdx].Flags["damage_taken_this_turn"] = 1
	gs.Seats[seatIdx].Turn.DamageReceived += amount
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
	// Track that this controller had a creature deal combat damage to a
	// player this turn — enables Freerunning (§702.169) and similar.
	if src.Controller >= 0 && src.Controller < len(gs.Seats) && gs.Seats[src.Controller] != nil {
		if gs.Seats[src.Controller].Flags == nil {
			gs.Seats[src.Controller].Flags = map[string]int{}
		}
		gs.Seats[src.Controller].Flags["creature_dealt_combat_damage_to_player"]++

		// §702.74 — Prowl needs to know which specific cards dealt
		// combat damage to a player this turn so it can compare
		// creature subtypes at cast time. Append once per card per
		// turn (dedupe on src.Card) so double-strike / extra-combat
		// hits don't bloat the slice and so a small linear scan in
		// CanPayProwl stays cheap.
		if src.Card != nil {
			ctrlTurn := &gs.Seats[src.Controller].Turn
			already := false
			for _, c := range ctrlTurn.CombatDamageBy {
				if c == src.Card {
					already = true
					break
				}
			}
			if !already {
				ctrlTurn.CombatDamageBy = append(ctrlTurn.CombatDamageBy, src.Card)
			}
		}
	}
	// §721.4 — If a creature deals combat damage to the monarch, its
	// controller becomes the monarch.
	CheckMonarchCombatSteal(gs, seatIdx, src.Controller)

	// §702.179 — Speed advances by 1 (once per turn per dealer) when a
	// player's source deals combat damage to a player. Routed through
	// SpeedDamageReporter so every damage call site uses the same
	// canonical entry point + once-per-turn gate.
	SpeedDamageReporter(gs, src.Controller)

	// §702.111 — Renown: if this creature has renown N, isn't already
	// renowned, and just dealt combat damage to a player, put N
	// +1/+1 counters on it and mark it renowned. No-op for sources
	// without the keyword (single keyword lookup) so the per-damage
	// dispatch cost stays flat.
	ApplyRenownOnCombatDamage(gs, src, seatIdx)

	fireCombatDamageTriggers(gs, src, amount, "player", seatIdx, nil)
}

// applyCombatDamageToCreature applies combat damage from src to another
// permanent, respecting protection, deathtouch (lethal = any), lifelink,
// and per-instance combat-damage triggers.
func applyCombatDamageToCreature(gs *GameState, src *Permanent, amount int, target *Permanent) {
	if amount <= 0 || target == nil {
		return
	}
	if attackerHasProtectionFromGS(gs, target, src) {
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
	// §614 damage replacement (R54) — Torbran +2 to opponent
	// permanents, Kuja Flare-Star doubling on Wizards, Lightning
	// stagger doubling against the marked defender. Runs before the
	// §615 prevention shields per §616.
	if len(gs.DamageReplacements) > 0 && src != nil && src.Card != nil && target.Card != nil {
		ctx := &DamageContext{
			Source:     src,
			SourceName: src.Card.DisplayName(),
			TargetSeat: target.Controller,
			TargetPerm: target,
			Kind:       DamageCombatCreature,
			Amount:     amount,
		}
		ApplyDamageReplacement(gs, ctx)
		if ctx.Prevented || ctx.Amount <= 0 {
			return
		}
		amount = ctx.Amount
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
		if target.MarkedDamage < gs.ToughnessOf(target) {
			target.MarkedDamage = gs.ToughnessOf(target)
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
// astAbilities returns a card's AST abilities, or nil when the card has no
// AST (tokens, hand-built test cards). Ranging over the nil result is a
// safe no-op, which lets combat-damage dispatch skip the AST loop without
// short-circuiting the per_card hooks that must fire for AST-less sources.
func astAbilities(c *Card) []gameast.Ability {
	if c == nil || c.AST == nil {
		return nil
	}
	return c.AST.Abilities
}

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
	if src.Card == nil {
		return
	}
	// AST-bearing sources dispatch their own "deals combat damage"
	// Triggered abilities here. NOTE: this loop is gated on AST != nil,
	// but the per_card combat_damage_player hook below must fire
	// regardless — a token or any AST-less attacker (e.g. a Ninja token)
	// still has to trigger Yuriko / Nashi / Gonti-style watchers. Before
	// r63 the whole function early-returned on a nil AST, silently
	// swallowing every per_card combat-damage trigger for token sources.
	for _, ab := range astAbilities(src.Card) {
		t, ok := ab.(*gameast.Triggered)
		if !ok {
			continue
		}
		if !EventEquals(t.Trigger.Event, "deals_combat_damage") &&
			!EventEquals(t.Trigger.Event, "deals_damage") {
			continue
		}
		// Judge r63 double-fire gate: when per_card owns this card's
		// combat-damage-to-player trigger, the FireCardTrigger
		// ("combat_damage_player") call below dispatches the handler —
		// pushing the AST effect here too would resolve the ability
		// twice (e.g. Rev, Tithe Extractor / Gonti, Canny Acquisitor
		// minting two Treasures / two impulse-exiles for a single batch).
		// Scoped to targetKind=="player" because the per_card event only
		// fires for player damage; combat-damage-to-creature AST triggers
		// on the same card still dispatch here. Mirrors the
		// creature_attacks gate above and the #1059 observer-class gates.
		if targetKind == "player" && PerCardOwnsTrigger(src.Card.DisplayName(), "combat_damage_player") {
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
			PushTriggeredAbilityWithIf(gs, src, t.Effect, t.InterveningIf)
		}
	}
	// Fire per-card trigger hooks for combat damage events so that
	// per_card handlers (Fynn, Yuriko, etc.) receive the notification.
	if targetKind == "player" {
		FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
			"source_seat":   src.Controller,
			"source_card":   src.Card.DisplayName(),
			"defender_seat": targetSeat,
			"amount":        amount,
			// source_perm / damage_seat are the richer contract a family of
			// self-gated per_card handlers (Nashi, Nine-Fingers Keene,
			// Atemsis, Calix, Captain Howler, Rex, Bello, Zeriam, Goro-Goro)
			// read to recognise "whenever ~ deals combat damage to a player".
			// Without them every one of those handlers early-returned on a
			// nil source_perm and was inert in real games (the per_card unit
			// tests passed only because they synthesised the ctx by hand).
			// damage_seat == the seat dealing the damage == src.Controller,
			// matching how those handlers compare it to perm.Controller.
			"source_perm": src,
			"damage_seat": src.Controller,
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

	// Per-card hook fan-out so handlers registered via
	// r.OnTrigger("...", "end_of_combat", ...) receive the
	// notification alongside the AST trigger pass below. Carries the
	// active seat so Raid-style "on your turn" gates can filter.
	// Wired in round 37 to support Lara Croft, Tomb Raider's Raid
	// Treasure trigger; any future per_card end-of-combat hook reads
	// the same event.
	FireCardTrigger(gs, "end_of_combat", map[string]interface{}{
		"active_seat": gs.Active,
	})

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
					PushTriggeredAbilityWithIf(gs, p, t.Effect, t.InterveningIf)
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
			setPermFlag(p, flagEnteredAttacking, false)
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

// MarkCowardsCantBlockWarriors stamps the shared "Cowards can't block
// Warriors" static markers for `perm` (Kargan Intimidator / Gornog the Red
// Reaper): a seat-flag fast-path gate plus the per-permanent grant flag that
// cowardBlockRestrictionActive scans for. Mirrors the Sidar Kondo flag wiring.
func MarkCowardsCantBlockWarriors(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["cowards_cant_block_warriors"] = 1
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["grants_cowards_cant_block_warriors"] = 1
}

// cowardBlockRestrictionActive re-verifies that a permanent granting the
// "Cowards can't block Warriors" static is still on a battlefield (the
// gs.Flags gate can outlive its source). Mirrors sidarKondoOnBattlefield.
func cowardBlockRestrictionActive(gs *GameState) bool {
	if gs == nil {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Flags != nil && p.Flags["grants_cowards_cant_block_warriors"] == 1 {
				return true
			}
		}
	}
	return false
}

// enforceGoadMustAttack adds any goaded creatures from `legal` that the
// Hat omitted from `chosen` back into the attacker set. CR §701.39
// requires the goaded creature to attack each combat if able; "able"
// here is satisfied by membership in `legal` (canAttack passed and the
// combat restriction lets it through). Returns the augmented list.
//
// Preserves order: chosen attackers first, then force-added goaded
// creatures in the order they appear in `legal`.
func enforceGoadMustAttack(gs *GameState, chosen, legal []*Permanent) []*Permanent {
	if gs == nil || len(legal) == 0 {
		return chosen
	}
	inChosen := make(map[*Permanent]struct{}, len(chosen))
	for _, p := range chosen {
		inChosen[p] = struct{}{}
	}
	out := chosen
	for _, p := range legal {
		if _, ok := inChosen[p]; ok {
			continue
		}
		if !MustAttackIfAble(gs, p) {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "goad_force_attack",
			Seat:   p.Controller,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule":   "701.39",
				"reason": "must_attack_if_able",
			},
		})
		out = append(out, p)
	}
	return out
}

// filterGoaderFromDefenders applies the §701.39 "attacks a player other
// than you if able" rider to a defender pool. Returns livingOpps minus
// the goader, unless filtering would empty the slice (the "if able"
// escape — if the goader is the only living opponent, the goaded
// creature is allowed to attack them).
//
// Non-goaded creatures pass through unchanged.
func filterGoaderFromDefenders(gs *GameState, perm *Permanent, livingOpps []int) []int {
	if gs == nil || perm == nil || len(livingOpps) == 0 {
		return livingOpps
	}
	if !IsGoaded(perm, gs.Turn) {
		return livingOpps
	}
	goader, ok := GoadedBySeat(perm, gs.Turn)
	if !ok {
		return livingOpps
	}
	filtered := make([]int, 0, len(livingOpps))
	for _, opp := range livingOpps {
		if opp == goader {
			continue
		}
		filtered = append(filtered, opp)
	}
	if len(filtered) == 0 {
		return livingOpps
	}
	return filtered
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
