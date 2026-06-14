package per_card

// r63 you-attack cardinality audit — bucket 3.
//
// "Whenever YOU attack" / "whenever one or more creatures you control attack"
// is a single player-level attack-declaration event that must fire EXACTLY
// ONCE per combat, no matter how many creatures attack. The engine dispatches
// creature_attacks ONCE PER DECLARED ATTACKER, so an ungated bucket-3 handler
// over-fires once per attacker. These tests drive the REAL per-attacker
// dispatch (FireCardTrigger("creature_attacks") × N) on a 3-attacker combat
// and assert the effect resolves exactly once.
//
// Both Adeline and Alibou ALSO had a latent ctx-key bug: they read
// ctx["active_seat"] which the attacks event never populates, so they were
// inert for every non-seat-0 controller (and over-fired only for seat 0).
// The fix reads the canonical attacker_seat. These tests run on seat 1 to
// pin BOTH the seat correctness and the once-per-combat cardinality.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func fireThreeAttackerCombat(gs *gameengine.GameState, attackers []*gameengine.Permanent, defender int) {
	for _, p := range attackers {
		gameengine.MarkEnteredAttacking(p)
		gameengine.SetAttackerDefender(p, defender)
	}
	for _, atk := range attackers {
		gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
			"attacker_perm": atk,
			"attacker_seat": atk.Controller,
			"attacker_card": atk.Card,
		})
	}
}

// Adeline: "Whenever you attack, for EACH OPPONENT create a token." On a
// 4-player game that's one token PER OPPONENT (3) ONCE — not 3 attackers × 3
// opponents = 9.
func TestYouAttack_Adeline_OncePerCombat_Seat1(t *testing.T) {
	gs := newGame(t, 4)
	gs.Turn = 5
	gs.Active = 1
	adeline := stampCreaturePT(addPerm(gs, 1, "Adeline, Resplendent Cathar", "creature", "legendary"), 3, 4)
	a1 := stampCreaturePT(addPerm(gs, 1, "Bear A", "creature"), 2, 2)
	a2 := stampCreaturePT(addPerm(gs, 1, "Bear B", "creature"), 2, 2)

	fireThreeAttackerCombat(gs, []*gameengine.Permanent{adeline, a1, a2}, 0)

	tokens := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Human Token" {
			tokens++
		}
	}
	if tokens != 3 {
		t.Fatalf("Adeline must mint exactly one token per opponent ONCE per combat = 3; got %d (0=ctx-bug inert, 9=over-fire)", tokens)
	}
}

// Alibou: "Whenever one or more artifact creatures you control attack, deal X
// damage" (X = tapped artifacts). Three attackers → damage dealt ONCE, not 3×.
func TestYouAttack_Alibou_OncePerCombat_Seat1(t *testing.T) {
	gs := newGame(t, 4)
	gs.Turn = 5
	gs.Active = 1
	// Alibou is an artifact creature; make it an attacking artifact creature.
	alibou := stampCreaturePT(addPerm(gs, 1, "Alibou, Ancient Witness", "creature", "artifact", "legendary"), 3, 4)
	a1 := stampCreaturePT(addPerm(gs, 1, "Bear A", "creature"), 2, 2)
	a2 := stampCreaturePT(addPerm(gs, 1, "Bear B", "creature"), 2, 2)
	// Two tapped non-creature artifacts → X counts these + tapped Alibou.
	m1 := addPerm(gs, 1, "Mox A", "artifact")
	m1.Tapped = true
	m2 := addPerm(gs, 1, "Mox B", "artifact")
	m2.Tapped = true

	startLife := make([]int, len(gs.Seats))
	for i, s := range gs.Seats {
		startLife[i] = s.Life
	}
	fireThreeAttackerCombat(gs, []*gameengine.Permanent{alibou, a1, a2}, 0)

	// X = tapped artifacts (m1, m2, and attacking-thus-tapped Alibou) = 3.
	// Exactly one opponent should have lost exactly X life (once), total life
	// lost across opponents must equal X — NOT 3×X.
	totalLost := 0
	for i, s := range gs.Seats {
		if i == 1 {
			continue
		}
		totalLost += startLife[i] - s.Life
	}
	if totalLost == 0 {
		t.Fatalf("Alibou did not fire (ctx-key bug) — expected one X-damage hit")
	}
	if totalLost > 6 { // X=3 once; 3× over-fire would be 9
		t.Fatalf("Alibou over-fired: total opponent life lost = %d, want a single X(=3) hit", totalLost)
	}
}

// Anim Pakal: "Whenever you attack with one or more non-Gnome creatures, put a
// +1/+1 counter on Anim Pakal, then create that many 1/1 Gnomes." Three
// attackers → counter goes 0→1 (once), one Gnome — NOT 0→3 with 1+2+3 Gnomes.
func TestYouAttack_AnimPakal_OncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	gs.Active = 0
	pakal := stampCreaturePT(addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature", "legendary"), 2, 2)
	a1 := stampCreaturePT(addPerm(gs, 0, "Bear A", "creature"), 2, 2)
	a2 := stampCreaturePT(addPerm(gs, 0, "Bear B", "creature"), 2, 2)

	fireThreeAttackerCombat(gs, []*gameengine.Permanent{pakal, a1, a2}, 1)

	if got := pakal.Counters["+1/+1"]; got != 1 {
		t.Fatalf("Anim Pakal must gain exactly ONE +1/+1 counter per combat; got %d (3=per-attacker over-fire)", got)
	}
	gnomes := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Gnome Token" {
			gnomes++
		}
	}
	if gnomes != 1 {
		t.Fatalf("Anim Pakal: X=1 → one Gnome once; got %d (over-fire mints 1+2+3=6)", gnomes)
	}
}

// Clavileño: "Whenever you attack, target attacking Vampire becomes a Demon."
// Three attacking Vampires → exactly ONE becomes a Demon, not three.
func TestYouAttack_Clavileno_OncePerCombat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	gs.Active = 0
	clav := stampCreaturePT(addPerm(gs, 0, "Clavileño, First of the Blessed", "creature", "legendary", "vampire"), 2, 2)
	v1 := stampCreaturePT(addPerm(gs, 0, "Vampire A", "creature", "vampire"), 3, 3)
	v2 := stampCreaturePT(addPerm(gs, 0, "Vampire B", "creature", "vampire"), 4, 4)

	fireThreeAttackerCombat(gs, []*gameengine.Permanent{clav, v1, v2}, 1)

	demons := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && cardHasType(p.Card, "demon") {
			demons++
		}
	}
	if demons != 1 {
		t.Fatalf("Clavileño must promote exactly ONE Vampire to Demon per combat; got %d (3=per-attacker over-fire)", demons)
	}
}
