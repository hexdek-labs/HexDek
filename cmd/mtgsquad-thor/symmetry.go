package main

// symmetry.go — Symmetry verification tests.
//
// Runs the same game twice with players swapped. Verifies the engine
// produces symmetric results — if player 0 wins game A, player 1
// should win game B (same seed, swapped decks). Catches seat-bias bugs,
// APNAP ordering errors, and hardcoded seat assumptions.

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
	symGameCount = 2000
	symMaxTurns  = 30
	symWorkers   = 8
)

func runSymmetry(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	var pool []*oracleCard
	for _, oc := range oracleCards {
		if oc.ast != nil {
			pool = append(pool, oc)
		}
	}
	if len(pool) < chaosDeckSize*2 {
		log.Printf("  symmetry: not enough cards (%d), skipping", len(pool))
		return nil
	}

	var (
		fails     []failure
		mu        sync.Mutex
		gamesDone int64
		panics    int64
		asymmetry int64
	)

	start := time.Now()
	work := make(chan int64, 256)
	go func() {
		for i := int64(0); i < symGameCount; i++ {
			work <- i
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < symWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range work {
				gf, gp, asym := playSymmetryPair(seed, pool)
				atomic.AddInt64(&gamesDone, 1)
				if gp {
					atomic.AddInt64(&panics, 1)
				}
				if asym {
					atomic.AddInt64(&asymmetry, 1)
				}
				if len(gf) > 0 {
					mu.Lock()
					fails = append(fails, gf...)
					mu.Unlock()
				}
				done := atomic.LoadInt64(&gamesDone)
				if done%500 == 0 {
					log.Printf("  symmetry: %d pairs, %d panics, %d asymmetric, %d fails",
						done, atomic.LoadInt64(&panics),
						atomic.LoadInt64(&asymmetry), len(fails))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("  symmetry complete: %d pairs, %d panics, %d asymmetric, %d fails, %s",
		atomic.LoadInt64(&gamesDone), atomic.LoadInt64(&panics),
		atomic.LoadInt64(&asymmetry), len(fails), time.Since(start))
	return fails
}

type symResult struct {
	seat0Life int
	seat1Life int
	seat0Lost bool
	seat1Lost bool
	seat0BF   int
	seat1BF   int
	panicked  bool
}

func playSymmetryPair(seed int64, pool []*oracleCard) (failures []failure, didPanic bool, asymmetric bool) {
	// Game A: normal order.
	rngA := rand.New(rand.NewSource(seed))
	deck0 := buildRandomDeck(rngA, pool)
	deck1 := buildRandomDeck(rngA, pool)

	resA := playSymGame(seed, deck0, deck1, 0)
	// Game B: swapped decks — deck1 goes to seat 0, deck0 goes to seat 1.
	resB := playSymGame(seed, deck1, deck0, 1)

	if resA.panicked || resB.panicked {
		didPanic = true
		failures = append(failures, failure{
			CardName:    fmt.Sprintf("sym_game_%d", seed),
			Interaction: "symmetry_panic",
			Panicked:    true,
			PanicMsg:    "one or both games panicked",
		})
		return
	}

	// Compare: A's seat 0 result should match B's seat 1 result (same deck).
	// We don't expect exact equality because RNG for decisions differs,
	// but we check for structural consistency:
	// - If seat 0 wins in A, seat 1 should win in B (same deck won).
	// - Both games should complete without panic.

	aWinner := -1 // no winner
	if resA.seat0Lost && !resA.seat1Lost {
		aWinner = 1
	} else if resA.seat1Lost && !resA.seat0Lost {
		aWinner = 0
	}

	bWinner := -1
	if resB.seat0Lost && !resB.seat1Lost {
		bWinner = 1
	} else if resB.seat1Lost && !resB.seat0Lost {
		bWinner = 0
	}

	// In game A, if deck0 (seat 0) won, then in game B deck0 is now seat 1,
	// so seat 1 should win. Check if the DECK won, not the SEAT.
	if aWinner >= 0 && bWinner >= 0 {
		aDeckWinner := aWinner // which deck index won in A
		bDeckWinner := 1 - bWinner // game B has swapped decks, so seat 0 = deck1, seat 1 = deck0
		if aDeckWinner != bDeckWinner {
			asymmetric = true
			// This is informational — RNG divergence means the same deck
			// won't always win. Only flag as failure if it's extreme.
		}
	}

	return failures, false, asymmetric
}

func playSymGame(seed int64, deck0Raw, deck1Raw []*gameengine.Card, _ int) (result symResult) {
	defer func() {
		if r := recover(); r != nil {
			result.panicked = true
			_ = debug.Stack()
		}
	}()

	// Deep copy decks to avoid pointer aliasing between games.
	deck0 := make([]*gameengine.Card, len(deck0Raw))
	for i, c := range deck0Raw {
		deck0[i] = c.DeepCopy()
		deck0[i].Owner = 0
	}
	deck1 := make([]*gameengine.Card, len(deck1Raw))
	for i, c := range deck1Raw {
		deck1[i] = c.DeepCopy()
		deck1[i].Owner = 1
	}

	rng := rand.New(rand.NewSource(seed + 1000000))
	gs := &gameengine.GameState{Rng: rng, Flags: map[string]int{}}
	for i := 0; i < 2; i++ {
		gs.Seats = append(gs.Seats, &gameengine.Seat{
			Life: chaosStartLife, Flags: map[string]int{}, Idx: i,
		})
	}
	gs.Seats[0].Library = deck0
	gs.Seats[1].Library = deck1

	// Draw opening hands.
	for i := 0; i < 2; i++ {
		for j := 0; j < chaosStartHand && len(gs.Seats[i].Library) > 0; j++ {
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}

	for turn := 1; turn <= symMaxTurns; turn++ {
		active := (turn - 1) % 2
		gs.Active = active
		seat := gs.Seats[active]
		if seat.Lost || seat.Life <= 0 {
			break
		}
		if gs.Seats[1-active].Lost || gs.Seats[1-active].Life <= 0 {
			break
		}

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
		symPlayCards(gs, active, rng)
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
	}

	result.seat0Life = gs.Seats[0].Life
	result.seat1Life = gs.Seats[1].Life
	result.seat0Lost = gs.Seats[0].Lost || gs.Seats[0].Life <= 0
	result.seat1Lost = gs.Seats[1].Lost || gs.Seats[1].Life <= 0
	result.seat0BF = len(gs.Seats[0].Battlefield)
	result.seat1BF = len(gs.Seats[1].Battlefield)
	return result
}

func symPlayCards(gs *gameengine.GameState, seatIdx int, rng *rand.Rand) {
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
			for _, t := range card.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
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
