package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
	_ "github.com/hexdek/hexdek/internal/gameengine/per_card"
)

func runComboDemo() {
	traceDeathtouch_Trample()
	traceThassasOracle_DemonicConsultation()
	traceBloodArtist_SacChain()
	traceHumility_Creatures()
	traceFogEffect()
	traceCounterWar()
}

func newDemoGS() *gameengine.GameState {
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(4, rng, nil)
	return gs
}

func addDemoPerm(gs *gameengine.GameState, seat int, name string, types []string, pow, tough int) *gameengine.Permanent {
	card := &gameengine.Card{
		Name: name, Owner: seat, Types: types,
		BasePower: pow, BaseToughness: tough,
	}
	perm := &gameengine.Permanent{
		Card: card, Controller: seat, Owner: seat,
		Flags: map[string]int{}, Counters: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

func printEvents(gs *gameengine.GameState, title string) {
	fmt.Printf("\n══════════════════════════════════════════\n")
	fmt.Printf("  COMBO: %s\n", title)
	fmt.Printf("══════════════════════════════════════════\n")
	for i, ev := range gs.EventLog {
		detail := ""
		if ev.Details != nil {
			parts := []string{}
			for k, v := range ev.Details {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			detail = strings.Join(parts, " ")
		}
		src := ev.Source
		if src == "" { src = "—" }
		amt := ""
		if ev.Amount != 0 { amt = fmt.Sprintf(" amt=%d", ev.Amount) }
		tgt := ""
		if ev.Target >= 0 { tgt = fmt.Sprintf(" →seat%d", ev.Target) }
		fmt.Printf("  %2d. [seat%d] %-24s src=%-30s%s%s %s\n",
			i+1, ev.Seat, ev.Kind, src, amt, tgt, detail)
	}
	// Print final state
	fmt.Printf("\n  FINAL STATE:\n")
	for i, s := range gs.Seats {
		if s == nil { continue }
		status := "ALIVE"
		if s.Lost { status = "LOST (" + s.LossReason + ")" }
		bf := len(s.Battlefield)
		fmt.Printf("    Seat %d: life=%-3d bf=%-2d gy=%-2d exile=%-2d %s\n",
			i, s.Life, bf, len(s.Graveyard), len(s.Exile), status)
	}
	viols := gameengine.RunAllInvariants(gs)
	if len(viols) == 0 {
		fmt.Printf("\n  INVARIANTS: ✅ ALL PASS\n")
	} else {
		for _, v := range viols {
			fmt.Printf("\n  INVARIANT FAIL: %s: %s\n", v.Name, v.Message)
		}
	}
}

func traceDeathtouch_Trample() {
	gs := newDemoGS()
	gs.EventLog = nil

	attacker := addDemoPerm(gs, 0, "Questing Beast", []string{"creature"}, 6, 6)
	attacker.Flags["kw:deathtouch"] = 1
	attacker.Flags["kw:trample"] = 1
	attacker.Flags["attacking"] = 1

	blocker := addDemoPerm(gs, 1, "Wall of Omens", []string{"creature"}, 0, 4)

	// Deal combat damage: deathtouch+trample = 1 to blocker (lethal), 5 tramples through
	gameengine.DealCombatDamageStep(gs,
		[]*gameengine.Permanent{attacker},
		map[*gameengine.Permanent][]*gameengine.Permanent{attacker: {blocker}},
		false)
	gameengine.StateBasedActions(gs)

	printEvents(gs, "Deathtouch + Trample (Questing Beast vs Wall of Omens)")
}

func traceThassasOracle_DemonicConsultation() {
	gs := newDemoGS()
	gs.EventLog = nil

	// Thassa's Oracle on battlefield
	oracle := addDemoPerm(gs, 0, "Thassa's Oracle", []string{"creature"}, 1, 3)
	_ = oracle

	// Simulate Demonic Consultation exiling entire library
	gs.Seats[0].Library = nil // library empty after Consultation

	// Fire Oracle's ETB: devotion to blue >= library size (0) = win
	gameengine.InvokeETBHook(gs, oracle)
	gameengine.StateBasedActions(gs)

	printEvents(gs, "Thassa's Oracle + Demonic Consultation (empty library win)")
}

func traceBloodArtist_SacChain() {
	gs := newDemoGS()
	gs.EventLog = nil

	// Blood Artist on battlefield
	artist := addDemoPerm(gs, 0, "Blood Artist", []string{"creature"}, 0, 1)
	_ = artist

	// 3 tokens to sacrifice
	for i := 0; i < 3; i++ {
		addDemoPerm(gs, 0, fmt.Sprintf("Zombie Token %d", i), []string{"token", "creature"}, 2, 2)
	}

	// Sacrifice each token
	for _, p := range gs.Seats[0].Battlefield {
		if strings.Contains(p.Card.Name, "Token") {
			gameengine.SacrificePermanent(gs, p, "Viscera Seer")
			break
		}
	}
	gameengine.StateBasedActions(gs)

	// Fire Blood Artist trigger
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"controller_seat": 0,
		"dying_creature": "Zombie Token 0",
	})

	printEvents(gs, "Blood Artist + Sacrifice Chain")
}

func traceHumility_Creatures() {
	gs := newDemoGS()
	gs.EventLog = nil

	// Humility on battlefield (all creatures are 1/1 with no abilities)
	humility := addDemoPerm(gs, 0, "Humility", []string{"enchantment"}, 0, 0)
	humility.Flags["humility_active"] = 1
	gs.Flags["humility_active"] = 1

	// Big creature that should become 1/1
	big := addDemoPerm(gs, 0, "Emrakul, the Aeons Torn", []string{"creature"}, 15, 15)
	_ = big

	// Small creature
	small := addDemoPerm(gs, 1, "Llanowar Elves", []string{"creature"}, 1, 1)
	_ = small

	gameengine.StateBasedActions(gs)

	fmt.Printf("\n══════════════════════════════════════════\n")
	fmt.Printf("  COMBO: Humility Layer Effect\n")
	fmt.Printf("══════════════════════════════════════════\n")
	fmt.Printf("  Humility on battlefield (all creatures lose abilities, become 1/1)\n")
	fmt.Printf("  Emrakul base 15/15 → under Humility: P=%d T=%d\n", big.Power(), big.Toughness())
	fmt.Printf("  Llanowar Elves base 1/1 → under Humility: P=%d T=%d\n", small.Power(), small.Toughness())
	viols := gameengine.RunAllInvariants(gs)
	if len(viols) == 0 {
		fmt.Printf("\n  INVARIANTS: ✅ ALL PASS\n")
	}
}

func traceFogEffect() {
	gs := newDemoGS()
	gs.EventLog = nil

	// Attacker
	attacker := addDemoPerm(gs, 0, "Craterhoof Behemoth", []string{"creature"}, 10, 10)
	attacker.Flags["attacking"] = 1

	// Set fog flag (Clinging Mists / Fog)
	gs.Flags["prevent_all_combat_damage"] = 1

	// Try to deal combat damage — should be prevented
	gameengine.DealCombatDamageStep(gs,
		[]*gameengine.Permanent{attacker},
		map[*gameengine.Permanent][]*gameengine.Permanent{},
		false)
	gameengine.StateBasedActions(gs)

	printEvents(gs, "Fog Effect (prevent all combat damage)")
}

func traceCounterWar() {
	gs := newDemoGS()
	gs.EventLog = nil

	// Player 0 casts a spell
	spell := &gameengine.Card{Name: "Demonic Tutor", Owner: 0, Types: []string{"sorcery"}, CMC: 2}
	spellItem := &gameengine.StackItem{Kind: "spell", Controller: 0, Card: spell}
	gameengine.PushStackItem(gs, spellItem)

	// Player 1 counters it
	counter1 := &gameengine.Card{Name: "Counterspell", Owner: 1, Types: []string{"instant"}, CMC: 2}
	counterItem := &gameengine.StackItem{
		Kind: "spell", Controller: 1, Card: counter1,
		Targets: []gameengine.Target{{Kind: gameengine.TargetKindStackItem, Stack: spellItem}},
	}
	gameengine.PushStackItem(gs, counterItem)

	// Player 0 counters the counter
	counter2 := &gameengine.Card{Name: "Swan Song", Owner: 0, Types: []string{"instant"}, CMC: 1}
	counterItem2 := &gameengine.StackItem{
		Kind: "spell", Controller: 0, Card: counter2,
		Targets: []gameengine.Target{{Kind: gameengine.TargetKindStackItem, Stack: counterItem}},
	}
	gameengine.PushStackItem(gs, counterItem2)

	fmt.Printf("\n══════════════════════════════════════════\n")
	fmt.Printf("  COMBO: Counter War (Demonic Tutor → Counterspell → Swan Song)\n")
	fmt.Printf("══════════════════════════════════════════\n")
	fmt.Printf("  Stack (bottom to top):\n")
	for i, item := range gs.Stack {
		fmt.Printf("    %d. [seat%d] %s\n", i+1, item.Controller, item.Card.Name)
	}
	fmt.Printf("\n  Resolving LIFO...\n")

	// Resolve stack LIFO
	for len(gs.Stack) > 0 {
		top := gs.Stack[len(gs.Stack)-1]
		fmt.Printf("    → Resolving: %s (seat%d)\n", top.Card.Name, top.Controller)
		gameengine.ResolveStackTop(gs)
		gameengine.StateBasedActions(gs)
	}

	fmt.Printf("\n  Stack empty. Demonic Tutor resolved: %v\n", !spellItem.Countered)
	viols := gameengine.RunAllInvariants(gs)
	if len(viols) == 0 {
		fmt.Printf("\n  INVARIANTS: ✅ ALL PASS\n")
	}
}
