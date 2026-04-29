package main

// clock_pressure.go — Performance regression tests.
//
// Runs chaos game turns under a time budget. Each turn must complete
// within the budget or it's flagged. Finds O(n²) interactions, slow
// SBA passes, and resolution paths that don't scale.

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	clockGameCount  = 1000
	clockMaxTurns   = 30
	clockTurnBudget = 100 * time.Millisecond // 100ms per turn
	clockWorkers    = 4
)

func runClockPressure(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	var pool []*oracleCard
	for _, oc := range oracleCards {
		if oc.ast != nil {
			pool = append(pool, oc)
		}
	}
	if len(pool) < chaosDeckSize*2 {
		log.Printf("  clock: not enough cards (%d), skipping", len(pool))
		return nil
	}

	var (
		fails     []failure
		mu        sync.Mutex
		gamesDone int64
		slowTurns int64
	)

	start := time.Now()
	work := make(chan int64, 256)
	go func() {
		for i := int64(0); i < clockGameCount; i++ {
			work <- i
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < clockWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range work {
				gf, slow := playClockGame(seed, pool)
				atomic.AddInt64(&gamesDone, 1)
				atomic.AddInt64(&slowTurns, int64(slow))
				if len(gf) > 0 {
					mu.Lock()
					fails = append(fails, gf...)
					mu.Unlock()
				}
				done := atomic.LoadInt64(&gamesDone)
				if done%250 == 0 {
					log.Printf("  clock: %d games, %d slow turns, %d fails",
						done, atomic.LoadInt64(&slowTurns), len(fails))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("  clock complete: %d games, %d slow turns (>%s), %d fails, %s",
		atomic.LoadInt64(&gamesDone), atomic.LoadInt64(&slowTurns),
		clockTurnBudget, len(fails), time.Since(start))
	return fails
}

func playClockGame(seed int64, pool []*oracleCard) (failures []failure, slowCount int) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("clock_game_%d", seed),
				Interaction: "clock_panic",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, stack[:min(len(stack), 500)]),
			})
		}
	}()

	rng := rand.New(rand.NewSource(seed))
	gs := &gameengine.GameState{Rng: rng, Flags: map[string]int{}}
	for i := 0; i < 2; i++ {
		deck := buildRandomDeck(rng, pool)
		for _, c := range deck {
			c.Owner = i
		}
		rng.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
		seat := &gameengine.Seat{Life: chaosStartLife, Flags: map[string]int{}, Library: deck, Idx: i}
		for j := 0; j < chaosStartHand && len(seat.Library) > 0; j++ {
			c := seat.Library[0]
			seat.Library = seat.Library[1:]
			seat.Hand = append(seat.Hand, c)
		}
		gs.Seats = append(gs.Seats, seat)
	}

	for turn := 1; turn <= clockMaxTurns; turn++ {
		active := (turn - 1) % 2
		gs.Active = active
		seat := gs.Seats[active]
		if seat.Lost || seat.Life <= 0 {
			break
		}
		if gs.Seats[1-active].Lost || gs.Seats[1-active].Life <= 0 {
			break
		}

		turnStart := time.Now()

		gs.Phase, gs.Step = "beginning", "untap"
		gameengine.UntapAll(gs, active)
		gameengine.StateBasedActions(gs)

		gs.Step = "draw"
		if (turn > 1 || active == 1) && len(seat.Library) > 0 {
			drawn := seat.Library[0]
			seat.Library = seat.Library[1:]
			seat.Hand = append(seat.Hand, drawn)
		}

		gs.Phase, gs.Step = "precombat_main", "main"
		seat.ManaPool = turn + 2
		gameengine.SyncManaAfterSpend(seat)
		clockPlayCards(gs, active, rng)
		gameengine.StateBasedActions(gs)

		gs.Phase, gs.Step = "combat", "declare_attackers"
		attackers := chaosAttack(gs, active, rng)
		if len(attackers) > 0 {
			gs.Step = "combat_damage"
			blockerMap := make(map[*gameengine.Permanent][]*gameengine.Permanent)
			gameengine.DealCombatDamageStep(gs, attackers, blockerMap, false)
			gameengine.StateBasedActions(gs)
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Flags != nil {
				delete(p.Flags, "attacking")
				delete(p.Flags, "blocked")
			}
		}

		gs.Phase, gs.Step = "ending", "end"
		gameengine.FirePhaseTriggers(gs, "ending", "end")
		gameengine.ScanExpiredDurations(gs, "ending", "end")
		gameengine.StateBasedActions(gs)

		gs.Step = "cleanup"
		gameengine.CleanupHandSize(gs, active, 7)
		gameengine.StateBasedActions(gs)

		for _, s := range gs.Seats {
			if s != nil {
				s.ManaPool = 0
			}
		}

		turnDuration := time.Since(turnStart)
		if turnDuration > clockTurnBudget {
			slowCount++
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("clock_game_%d_t%d", seed, turn),
				Interaction: "clock_slow_turn",
				Message: fmt.Sprintf("turn took %s (budget %s), bf0=%d bf1=%d",
					turnDuration, clockTurnBudget,
					len(gs.Seats[0].Battlefield), len(gs.Seats[1].Battlefield)),
			})
		}
	}

	return failures, slowCount
}

func clockPlayCards(gs *gameengine.GameState, seatIdx int, rng *rand.Rand) {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	attempts := 0
	for attempts < 3 && len(seat.Hand) > 0 && seat.ManaPool > 0 {
		attempts++
		affordable := -1
		order := rng.Perm(len(seat.Hand))
		for _, idx := range order {
			c := seat.Hand[idx]
			if c != nil && c.CMC <= seat.ManaPool {
				affordable = idx
				break
			}
		}
		if affordable < 0 {
			break
		}
		card := seat.Hand[affordable]
		isPermanent := false
		for _, t := range card.Types {
			switch t {
			case "creature", "artifact", "enchantment", "planeswalker", "land", "battle":
				isPermanent = true
			}
		}
		seat.Hand = append(seat.Hand[:affordable], seat.Hand[affordable+1:]...)
		seat.ManaPool -= card.CMC
		gameengine.SyncManaAfterSpend(seat)

		if isPermanent {
			isCreature := false
			tough := card.BaseToughness
			for _, t := range card.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if isCreature && tough <= 0 {
				tough = 1
			}
			_ = tough
			perm := &gameengine.Permanent{
				Card: card, Controller: seatIdx, Owner: seatIdx,
				SummoningSick: isCreature, Timestamp: gs.NextTimestamp(),
				Counters: map[string]int{}, Flags: map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, perm)
			gameengine.InvokeETBHook(gs, perm)
			gameengine.RegisterReplacementsForPermanent(gs, perm)
			if card.AST != nil {
				for _, ab := range card.AST.Abilities {
					if trig, ok := ab.(*gameast.Triggered); ok && trig.Trigger.Event == "etb" && trig.Effect != nil {
						gameengine.ResolveEffect(gs, perm, trig.Effect)
					}
				}
			}
		} else {
			if card.AST != nil {
				for _, ab := range card.AST.Abilities {
					switch a := ab.(type) {
					case *gameast.Triggered:
						if a.Effect != nil {
							gameengine.ResolveEffect(gs, nil, a.Effect)
						}
					}
				}
			}
			if !cardInAnyZone(gs, card) {
				gameengine.MoveCard(gs, card, seatIdx, "stack", "graveyard", "spell_resolve")
			}
		}
		gameengine.StateBasedActions(gs)
		if seat.Lost {
			break
		}
	}
}
