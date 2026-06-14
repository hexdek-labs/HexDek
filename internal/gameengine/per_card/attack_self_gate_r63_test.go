package per_card

// r63 — attack-trigger self-gate audit (follow-up to the Aang and Katara fix
// 5254f9cf). The engine fires "creature_attacks" ONCE PER DECLARED ATTACKER
// (combat.go), dispatching to every battlefield permanent that registered a
// handler for that event. A handler implementing "Whenever THIS creature
// attacks" must therefore gate on attacker_perm == perm, or it fires N times
// per combat (one per attacker the controller declares) — wrong multiplier,
// and for token-minting handlers a potential non-terminating game.
//
// This audit found 18 self-trigger handlers missing that gate. The core
// regression below is NO-FIRE-ON-OTHER: with the card on the battlefield and a
// DIFFERENT creature declared as the attacker, the handler must NOT mutate game
// state. A representative subset also asserts FIRE-ON-SELF (the harness setup
// genuinely enables an effect, so the no-fire assertion is meaningful).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// gameFingerprint sums the state a self-gate violation would perturb: zone
// sizes, life, and the total of all counters + flags across every permanent.
// Each dimension carries a DISTINCT PRIME WEIGHT so that effects which move a
// card between zones (draw+discard, reanimate = battlefield+1/graveyard-1) do
// NOT cancel to a net-zero delta — a naive equal-weight sum would let such a
// handler fire undetected on a foreign attacker.
func gameFingerprint(gs *gameengine.GameState) int {
	sum := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		sum += len(s.Battlefield)*7 + len(s.Hand)*11 + len(s.Graveyard)*13 +
			len(s.Exile)*17 + len(s.Library)*19 + s.Life*23
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			for _, v := range p.Counters {
				sum += v * 29
			}
			for _, v := range p.Flags {
				sum += v * 31
			}
		}
	}
	return sum
}

// buildSelfGateScene puts the target card (the self-trigger bearer) on seat 0
// alongside a foreign vanilla attacker, an opponent creature, and generous
// shared resources so every handler in the table has material to act on if it
// (wrongly) fires. Returns the target perm and the foreign attacker.
func buildSelfGateScene(t *testing.T, cardName string, types []string) (*gameengine.GameState, *gameengine.Permanent, *gameengine.Permanent) {
	t.Helper()
	gs := newGame(t, 3)

	self := stampCreaturePT(addPerm(gs, 0, cardName, types...), 3, 3)
	if self.Counters == nil {
		self.Counters = map[string]int{}
	}
	foreign := stampCreaturePT(addPerm(gs, 0, "Vanilla Attacker", "creature"), 2, 2)

	// Opponent creature (suspect/goad/target fodder) + a target land/creature
	// on our side (Eluge/Omo) + a token-shaped permanent (Finneas).
	stampCreaturePT(addPerm(gs, 1, "Opp Creature", "creature"), 2, 2)
	addPerm(gs, 0, "Our Land", "land")
	stampCreaturePT(addPerm(gs, 0, "Rabbit Token", "creature", "rabbit", "token"), 1, 1)

	// Libraries (basic land for Aerial Surveyor; artifacts for Ayesha/Kylox;
	// generic cards for loot/exile effects) on every seat.
	plains := &gameengine.Card{Name: "Plains", Owner: 0, Types: []string{"land", "basic", "plains"}}
	solRing := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact", "cost:1"}}
	graveCreature := &gameengine.Card{Name: "Dead Beast", Owner: 0, Types: []string{"creature", "cost:2"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, plains, solRing,
		&gameengine.Card{Name: "Lib A", Owner: 0, Types: []string{"creature", "cost:1"}},
		&gameengine.Card{Name: "Lib B", Owner: 0, Types: []string{"instant", "cost:1"}})
	gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{Name: "Opp Lib", Owner: 1})
	gs.Seats[2].Library = append(gs.Seats[2].Library, &gameengine.Card{Name: "Opp2 Lib", Owner: 2})
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, graveCreature)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{Name: "Hand A", Owner: 0})

	// Experience counters (Otharri / Katara read these) + Treasure fuel.
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["experience"] = 2

	// Foreign attacker declares against seat 1.
	gameengine.SetAttackerDefender(foreign, 1)
	gameengine.SetAttackerDefender(self, 1)
	return gs, self, foreign
}

func TestAttackSelfGate_NoFireOnForeignAttacker(t *testing.T) {
	// Every handler the r63 audit gated. types kept minimal (legendary creature
	// unless the card needs a subtype the handler keys on).
	cards := []struct {
		name  string
		types []string
	}{
		{"Aphelia, Viper Whisperer", []string{"creature", "legendary"}},
		{"Ayesha Tanaka, Armorer", []string{"creature", "legendary"}},
		{"Bruse Tarl, Boorish Herder", []string{"creature", "legendary"}},
		{"Burakos, Party Leader", []string{"creature", "legendary", "warrior"}},
		{"Cait, Cage Brawler", []string{"creature", "legendary", "warrior"}},
		{"Eluge, the Shoreless Sea", []string{"creature", "legendary"}},
		{"Excava, the Risen Past", []string{"creature", "legendary"}},
		{"Finneas, Ace Archer", []string{"creature", "legendary"}},
		{"Greven, Predator Captain", []string{"creature", "legendary"}},
		{"Katara, Waterbending Master", []string{"creature", "legendary"}},
		{"Kylox, Visionary Inventor", []string{"creature", "legendary"}},
		{"Mr. Foxglove", []string{"creature", "legendary"}},
		{"Nelly Borca, Impulsive Accuser", []string{"creature", "legendary"}},
		{"Omo, Queen of Vesuva", []string{"creature", "legendary"}},
		{"Otharri, Suns' Glory", []string{"creature", "legendary"}},
		{"Pako, Arcane Retriever", []string{"creature", "legendary"}},
		{"Radha, Heir to Keld", []string{"creature", "legendary"}},
		{"Aerial Surveyor", []string{"artifact", "creature", "vehicle"}},
	}

	for _, c := range cards {
		t.Run(c.name, func(t *testing.T) {
			gs, _, foreign := buildSelfGateScene(t, c.name, c.types)
			before := gameFingerprint(gs)
			// Fire creature_attacks with the FOREIGN creature as the attacker.
			gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
				"attacker_perm": foreign,
				"attacker_seat": foreign.Controller,
				"attacker_card": foreign.Card,
			})
			after := gameFingerprint(gs)
			if before != after {
				t.Errorf("%s mutated game state when a FOREIGN creature attacked "+
					"(self-gate missing): fingerprint %d → %d", c.name, before, after)
			}
		})
	}
}

func TestAttackSelfGate_FiresOnSelf(t *testing.T) {
	// Representative subset whose effect reliably perturbs the fingerprint when
	// the card ITSELF attacks — proving the no-fire harness above is sensitive
	// (the handlers do act; the gate, not an inert setup, is what suppresses
	// them on a foreign attacker).
	cards := []struct {
		name  string
		types []string
	}{
		{"Otharri, Suns' Glory", []string{"creature", "legendary"}},        // xp + Rebel tokens
		{"Pako, Arcane Retriever", []string{"creature", "legendary"}},      // exiles top of libraries + counters
		{"Burakos, Party Leader", []string{"creature", "legendary", "warrior"}}, // defender life loss + Treasure
		{"Nelly Borca, Impulsive Accuser", []string{"creature", "legendary"}}, // suspect/goad flags on opp creature
	}

	for _, c := range cards {
		t.Run(c.name, func(t *testing.T) {
			gs, self, _ := buildSelfGateScene(t, c.name, c.types)
			before := gameFingerprint(gs)
			gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
				"attacker_perm": self,
				"attacker_seat": self.Controller,
				"attacker_card": self.Card,
			})
			after := gameFingerprint(gs)
			if before == after {
				t.Errorf("%s did NOT act when it attacked itself (harness should enable "+
					"its effect): fingerprint unchanged at %d", c.name, before)
			}
		})
	}
}
