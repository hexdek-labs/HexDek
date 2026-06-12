package gameengine

import (
	"strconv"
	"strings"
)

// attack_trigger_dispatch.go — raw-aware attack-trigger actor classes
// (r63; the PROGRESSION dimension's deferred dispatch-consistency audit,
// /tmp/fable-review/progression-triggers-r63.md "where does that
// knowledge come from, and is it consistent?").
//
// The parser drops trigger ACTOR phrases — every "whenever X attacks"
// parses to a bare `attack` event — so dispatch consistency lived in
// fireAttackTriggers' two hardcoded substrings ("a creature you control
// attacks", "another creature attacks"). A corpus sweep of all 1,047
// attack triggers found the fallout: ~69 attached-bearer triggers
// ("equipped/enchanted creature attacks") could NEVER fire (the bearer
// is an aura/equipment and never attacks), "a creature attacks"
// any-controller triggers (Ondu Rising class) never fired for
// opponents' attacks, and subtype/filtered ally triggers (Utvara
// Hellkite's Dragons, Mardu Ascendancy's nontoken, Rage Forger's
// counter-bearers, Raid Bombardment's power<=2) never fired on ally
// attacks at all.
//
// This classifier recovers the actor phrase from the trigger Raw (the
// same recover-from-raw pattern as triggerControllerMatchesRaw for
// phase triggers) and the dispatcher fires the missing classes
// ADDITIVELY:
//
//   - existing fires are untouched (pool 1 self-attack stays raw-blind;
//     the two legacy wordings keep their exact substring behavior);
//   - new-class fires require the actor phrase to be followed by
//     "attacks," — clauses like "attacks you", "attacks alone",
//     "attacks one of your opponents" stay out of scope (fail closed);
//   - bearers whose card has a registered per_card creature_attacks
//     handler are SKIPPED for new classes (the handler is authoritative;
//     prevents double-fire — Winota, Najeela, etc.);
//   - unrecognized actor phrases keep today's behavior exactly.
type attackTriggerClass int

const (
	atkUnknown attackTriggerClass = iota
	// atkAllyYouControl — "[a|another] <filter> you control attacks":
	// bearers on the ACTIVE seat fire per matching ally attacker.
	atkAllyYouControl
	// atkAttached — "equipped creature attacks" / "enchanted creature
	// attacks" (bare, no article): the bearer is the attachment; fires
	// when the permanent it is attached to attacks. Bearer may sit on
	// any seat (you can enchant an opponent's creature).
	atkAttached
	// atkAnyCreature — "a creature attacks" / "a <subtype> attacks":
	// any controller's attack; bearers on every seat fire.
	atkAnyCreature
)

// attackTriggerSpec is one classified actor phrase.
type attackTriggerSpec struct {
	class attackTriggerClass
	// legacy marks the two wordings fireAttackTriggers always handled —
	// they keep substring matching and bypass the per_card guard so
	// behavior is bit-for-bit unchanged for them.
	legacy bool

	// Filters on the ATTACKER (all zero-valued = any creature).
	subtype        string // single-token type/subtype ("dragon", "vehicle")
	nontoken       bool
	needsCounter   string // counter kind required ("+1/+1")
	flying         int    // 0 none; +1 with flying; -1 without flying
	powerAtMost    int    // >0: power <= N required
	powerAtLeast   int    // >0: power >= N required
	needsEquipment bool   // attacker must have an Equipment attached
}

// classifyAttackTrigger extracts and classifies the actor phrase of an
// attack-trigger Raw. Returns atkUnknown for anything not explicitly
// recognized — fail closed, current behavior preserved.
func classifyAttackTrigger(raw string) attackTriggerSpec {
	r := strings.ToLower(raw)

	// Legacy wordings first: exact substring, exactly as the old pool-2
	// whitelist matched them.
	if strings.Contains(r, "a creature you control attacks") {
		return attackTriggerSpec{class: atkAllyYouControl, legacy: true}
	}
	if strings.Contains(r, "another creature attacks") {
		return attackTriggerSpec{class: atkAllyYouControl, legacy: true}
	}

	// Actor phrase: between the trigger word and " attacks,". The comma
	// requirement keeps defender-scoped ("attacks you") and rider
	// ("attacks alone") clauses out of the new classes.
	var actor string
	for _, lead := range []string{"whenever ", "when "} {
		if i := strings.Index(r, lead); i >= 0 {
			rest := r[i+len(lead):]
			if j := strings.Index(rest, " attacks,"); j > 0 {
				actor = strings.TrimSpace(rest[:j])
			}
			break
		}
	}
	if actor == "" {
		return attackTriggerSpec{}
	}

	switch actor {
	case "equipped creature", "enchanted creature",
		"this creature or equipped creature", "~ or equipped creature",
		"this creature or enchanted creature", "~ or enchanted creature":
		// The or-self variants fire the self half via pool 1 already;
		// the attached half is what was missing.
		return attackTriggerSpec{class: atkAttached}
	case "a creature", "a creature token":
		return attackTriggerSpec{class: atkAnyCreature}
	case "another creature you control":
		return attackTriggerSpec{class: atkAllyYouControl}
	case "a nontoken creature you control":
		return attackTriggerSpec{class: atkAllyYouControl, nontoken: true}
	case "a creature you control with a +1/+1 counter on it":
		return attackTriggerSpec{class: atkAllyYouControl, needsCounter: "+1/+1"}
	case "a creature you control with flying":
		return attackTriggerSpec{class: atkAllyYouControl, flying: 1}
	case "a creature with flying":
		return attackTriggerSpec{class: atkAnyCreature, flying: 1}
	case "a creature without flying":
		return attackTriggerSpec{class: atkAnyCreature, flying: -1}
	case "an equipped creature you control":
		return attackTriggerSpec{class: atkAllyYouControl, needsEquipment: true}
	}

	// "a creature you control with power N or less/greater".
	if rest, ok := strings.CutPrefix(actor, "a creature you control with power "); ok {
		fields := strings.Fields(rest)
		if len(fields) == 3 && fields[1] == "or" {
			if n, err := strconv.Atoi(fields[0]); err == nil {
				switch fields[2] {
				case "less":
					return attackTriggerSpec{class: atkAllyYouControl, powerAtMost: n}
				case "greater":
					return attackTriggerSpec{class: atkAllyYouControl, powerAtLeast: n}
				}
			}
		}
		return attackTriggerSpec{}
	}

	// "a <subtype> you control" — single-token subtype/type only
	// ("a dragon you control", "a vehicle you control"). Multi-token
	// actors ("a zombie token you control with power 6 or greater")
	// stay unknown.
	if rest, ok := strings.CutPrefix(actor, "a "); ok {
		if sub, ok2 := strings.CutSuffix(rest, " you control"); ok2 {
			if sub != "" && !strings.Contains(sub, " ") && sub != "creature" {
				return attackTriggerSpec{class: atkAllyYouControl, subtype: sub}
			}
			return attackTriggerSpec{}
		}
		// "a <subtype>" — any controller ("a warrior").
		if rest != "" && !strings.Contains(rest, " ") &&
			rest != "creature" && rest != "player" && rest != "opponent" {
			return attackTriggerSpec{class: atkAnyCreature, subtype: rest}
		}
	}

	return attackTriggerSpec{}
}

// matchesAttacker reports whether the declared attacker satisfies the
// spec's filters.
func (s attackTriggerSpec) matchesAttacker(gs *GameState, atk *Permanent) bool {
	if atk == nil || atk.Card == nil {
		return false
	}
	if s.subtype != "" {
		if !cardHasSubtype(atk.Card, s.subtype) && !atk.hasType(s.subtype) {
			return false
		}
	} else if !atk.IsCreature() {
		// "a creature ..." forms require a creature attacker (attacking
		// non-creatures exist: vehicles pre-crew edge states fail closed).
		return false
	}
	if s.nontoken && atk.IsToken() {
		return false
	}
	if s.needsCounter != "" && (atk.Counters == nil || atk.Counters[s.needsCounter] <= 0) {
		return false
	}
	if s.flying > 0 && !atk.HasKeyword("flying") {
		return false
	}
	if s.flying < 0 && atk.HasKeyword("flying") {
		return false
	}
	if s.powerAtMost > 0 && atk.Power() > s.powerAtMost {
		return false
	}
	if s.powerAtLeast > 0 && atk.Power() < s.powerAtLeast {
		return false
	}
	if s.needsEquipment {
		// An attacker "is equipped" when some Equipment is attached to
		// it; there is no reverse pointer, so walk the battlefields.
		found := false
		for _, seat := range gs.Seats {
			if seat == nil || found {
				continue
			}
			for _, p := range seat.Battlefield {
				if p != nil && p.AttachedTo == atk && p.hasType("equipment") {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// fireClassifiedAttackTriggers is the classifier-driven replacement for
// fireAttackTriggers' old pool 2 (the two-substring ally walk), extended
// with the attached / any-creature / filtered-ally classes. Self-attack
// triggers (pool 1) are untouched; every (bearer, attacker) pair fired
// here has attacker != bearer.
func fireClassifiedAttackTriggers(gs *GameState, activeSeat int, declared []*Permanent, declaredSet map[*Permanent]struct{}) {
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		bearers := append([]*Permanent{}, seat.Battlefield...)
		for _, perm := range bearers {
			if perm == nil || perm.Card == nil {
				continue
			}
			if _, self := declaredSet[perm]; self {
				// The bearer is itself attacking; pool 1 owns its triggers.
				continue
			}
			for _, ab := range iterAttackTriggers(perm.Card) {
				spec := classifyAttackTrigger(ab.Raw)
				switch spec.class {
				case atkUnknown:
					continue
				case atkAllyYouControl:
					if seatIdx != activeSeat {
						continue
					}
				case atkAttached, atkAnyCreature:
					// bearers on every seat participate.
				}
				// per_card owns the nuanced cards (Winota, Najeela, …):
				// skip new-class AST dispatch when a creature_attacks
				// handler is registered. The legacy wordings bypass the
				// guard — they fired before this change and must keep
				// firing identically.
				if !spec.legacy && HasTriggerHook != nil &&
					HasTriggerHook(perm.Card.DisplayName(), "creature_attacks") {
					continue
				}
				for _, atk := range declared {
					if atk == perm {
						continue
					}
					if spec.class == atkAttached {
						if perm.AttachedTo == nil || atk != perm.AttachedTo {
							continue
						}
					} else if !spec.matchesAttacker(gs, atk) {
						continue
					}
					gs.LogEvent(Event{
						Kind: "trigger_fires", Seat: perm.Controller,
						Source: perm.Card.DisplayName(),
						Details: map[string]interface{}{
							"event":           "attack_ally",
							"trigger_by_card": atk.Card.DisplayName(),
							"rule":            "603.3a",
						},
					})
					if ab.Effect != nil {
						PushTriggeredAbilityWithIf(gs, perm, ab.Effect, ab.InterveningIf)
					}
				}
			}
		}
	}
}
