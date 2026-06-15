package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func setAttacking(p *gameengine.Permanent) {
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["attacking"] = 1
	// "Attacks alone" abilities (Rafiq/Squall/Grunn) count DECLARED
	// attackers (CR §508.4a), so a test attacker must carry the
	// declared-this-combat flag too — this is what DeclareAttackers sets
	// for genuinely declared attackers (combat.go).
	p.Flags["declared_attacker_this_combat"] = 1
}

// setEnteredAttacking marks a permanent as attacking WITHOUT declaring it
// (the §506.3 "enters the battlefield attacking" shape: tokens, ninjutsu,
// Kaalia). Such creatures must NOT count toward "attacks alone".
func setEnteredAttacking(p *gameengine.Permanent) {
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["attacking"] = 1
}

func TestRafiq_LoneAttackerGainsDoubleStrike(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	attacker := addPerm(gs, 0, "Combat Celebrant", "creature")
	setAttacking(attacker)

	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})

	if attacker.Flags["kw:double_strike"] != 1 {
		t.Errorf("expected attacker to have kw:double_strike, got %d", attacker.Flags["kw:double_strike"])
	}
	if attacker.Flags["double_strike_eot"] != 1 {
		t.Errorf("expected double_strike_eot cleanup marker")
	}
}

func TestRafiq_NoTriggerWithMultipleAttackers(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	a := addPerm(gs, 0, "Bear A", "creature")
	b := addPerm(gs, 0, "Bear B", "creature")
	setAttacking(a)
	setAttacking(b)

	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})

	if a.Flags["kw:double_strike"] == 1 || b.Flags["kw:double_strike"] == 1 {
		t.Errorf("Rafiq must not fire when 2+ creatures attack")
	}
}

func TestRafiq_NoTriggerWithZeroAttackers(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})
	// No crash; no event of kind per_card_handler.
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" {
			if d, ok := ev.Details["slug"].(string); ok && d == "rafiq_attacks_alone_double_strike" {
				t.Errorf("Rafiq must not emit when no creature is attacking")
			}
		}
	}
}

func TestRafiq_LoneAttackerCanBeRafiqHimself(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	setAttacking(rafiq)

	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})

	if rafiq.Flags["kw:double_strike"] != 1 {
		t.Errorf("Rafiq attacking alone should grant himself double strike too (any creature you control)")
	}
}
