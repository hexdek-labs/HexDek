package gameengine

// keywords_battalion.go — Battalion (CR §702.101, Gatecrash 2013) as a
// generic combat-declared trigger.
//
// CR §702.101a: Battalion is a triggered ability that functions while
//               the card with battalion is on the battlefield.
//               "Battalion — [effect]" means "Whenever this creature
//               and at least two other creatures attack, [effect]."
// CR §702.101b: A creature attacks if it's declared as an attacker or
//               put onto the battlefield attacking (§506.3). The
//               battalion trigger fires from the §603.2c "when state
//               becomes true" check — specifically, once at the moment
//               attackers are declared / scoop-in is complete.
//
// Implementation: a single hook, FireBattalionTriggers, called from
// declareAttackers in combat.go after the §506.3 scoop-in but before
// per-card combat keywords run. For each attacking permanent that has
// battalion AND whose controller has 3+ attacking creatures total (the
// battalion source plus 2 others), this fan-out fires
// FireCardTrigger("battalion_triggered", ctx) so per_card handlers can
// implement the specific battalion payoff.
//
// Per-card handlers register against the "battalion_triggered" event
// (rather than re-counting attackers themselves, the pattern the early
// Tajic handler used). The ctx carries:
//
//	source      *Permanent  — the battalion-bearing attacker
//	controller  int         — source.Controller
//	attackers   []*Permanent — all attacking creatures controlled by source.Controller
//	count       int         — len(attackers); always >= 3 when fired
//
// This file is a STUB in the sense that no engine-default payoff is
// applied — payoffs are entirely the per_card layer's responsibility.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasBattalion
// ---------------------------------------------------------------------------

// HasBattalion returns true if the card has the battalion keyword in
// its AST — either a bare Keyword node or (the shape real cards parse to)
// a Static{ability_word "battalion" → Triggered} wrapper.
func HasBattalion(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "battalion") {
			return true
		}
	}
	return battalionTriggerFromCard(card) != nil
}

// battalionTriggerFromCard extracts the inner *Triggered from a
// Static{ability_word "battalion" → Triggered} wrapper (the shape Thor emits
// for "Battalion — Whenever this creature and at least two other creatures
// attack, …"). Returns nil when the card has no such wrapper. The inner
// Triggered carries the payoff effect; the "at least two other creatures"
// cardinality is NOT in the AST — FireBattalionTriggers enforces it (the
// source attacking + 3+ attackers total) before resolving.
func battalionTriggerFromCard(card *Card) *gameast.Triggered {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "ability_word" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 3 {
			continue
		}
		if w, _ := args[0].(string); w != "battalion" {
			continue
		}
		if t, ok := args[2].(*gameast.Triggered); ok {
			return t
		}
	}
	return nil
}

// PermanentHasBattalion is the Permanent-level convenience for the
// declare-attackers hook. Reads from the card AST and from runtime
// grants (Permanent.GrantedAbilities, kw:battalion flag) — battalion
// is rarely granted but the same machinery already powers Tajic-style
// handlers in per_card so we honor it for parity.
func PermanentHasBattalion(p *Permanent) bool {
	if p == nil {
		return false
	}
	if HasBattalion(p.Card) {
		return true
	}
	for _, g := range p.GrantedAbilities {
		if equalFoldTrimmed(g, "battalion") {
			return true
		}
	}
	if p.Flags != nil && p.Flags["kw:battalion"] > 0 {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// FireBattalionTriggers — declare-attackers hook
// ---------------------------------------------------------------------------

// FireBattalionTriggers scans `attackers` (the full set of declared +
// scooped-in attacking creatures for the active player) and fires a
// "battalion_triggered" trigger for each attacking permanent with the
// battalion keyword whose controller has 3+ attacking creatures total.
//
// Called from declareAttackers after FireDethroneTriggers and before
// CheckAttackKeywordsCombat, so battalion payoffs (typically +X/+X)
// can layer cleanly with other attack-time buffs.
//
// Per CR §702.101a the trigger requires "this creature AND at least
// two other creatures attack" — so the source must itself be
// attacking, and the total must be >= 3.
func FireBattalionTriggers(gs *GameState, attackerSeat int, attackers []*Permanent) {
	if gs == nil || len(attackers) == 0 {
		return
	}
	// Group attackers by controller (in multiplayer, attackerSeat is the
	// active player but a future extra-combat or control-swap variant
	// might place enemy attackers in the list — group defensively).
	byController := make(map[int][]*Permanent, 4)
	for _, p := range attackers {
		if p == nil || !p.IsCreature() {
			continue
		}
		byController[p.Controller] = append(byController[p.Controller], p)
	}

	for _, p := range attackers {
		if p == nil || !p.IsCreature() {
			continue
		}
		if !PermanentHasBattalion(p) {
			continue
		}
		// CR §702.101a — source must itself be attacking. By construction
		// every element of `attackers` is an attacker (either declared via
		// fireAttackTriggers' `declared` list or scooped in via the
		// "entered tapped and attacking" branch). Guard explicitly so a
		// future caller that passes non-attackers can't slip through.
		if !p.IsAttacking() {
			continue
		}
		group := byController[p.Controller]
		if len(group) < 3 {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "battalion_trigger",
			Seat:   p.Controller,
			Source: p.Card.DisplayName(),
			Amount: len(group),
			Details: map[string]interface{}{
				"attacker_count": len(group),
				"rule":           "702.101a",
			},
		})
		FireCardTrigger(gs, "battalion_triggered", map[string]interface{}{
			"source":     p,
			"controller": p.Controller,
			"attackers":  group,
			"count":      len(group),
		})
		// r63 dead-dispatch sweep (attack class): resolve the wrapped
		// battalion effect. Pre-r63 this hook only fired the per_card event
		// above and never resolved the inner Triggered, so every battalion
		// card WITHOUT a bespoke per_card handler (the bulk — Boros Elite,
		// Wojek Halberdiers, Tajic, …) did nothing when 3+ attacked. Skip
		// when a per_card handler owns "battalion_triggered" (no double-fire).
		if trig := battalionTriggerFromCard(p.Card); trig != nil && trig.Effect != nil {
			if HasTriggerHook == nil || !HasTriggerHook(p.Card.DisplayName(), "battalion_triggered") {
				PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}
}
