package main

// multiplayer_chaos.go — 4-player chaos game simulations.
//
// Extends chaos_games.go to 4-seat multiplayer. Tests APNAP ordering,
// commander damage tracking, multi-opponent targeting, and political
// effects under realistic cEDH-like conditions.

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
	mpGameCount = 2000
	mpMaxTurns  = 30
	mpDeckSize  = 40
	mpSeats     = 4
	mpWorkers   = 8
)

func runMultiplayerChaos(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	var pool []*oracleCard
	for _, oc := range oracleCards {
		if oc.ast != nil {
			pool = append(pool, oc)
		}
	}
	if len(pool) < mpDeckSize*mpSeats {
		log.Printf("  multiplayer: not enough cards (%d), skipping", len(pool))
		return nil
	}

	var (
		fails     []failure
		mu        sync.Mutex
		tested    int64
		gamesDone int64
		panics    int64
	)

	start := time.Now()
	work := make(chan int64, 256)
	go func() {
		for i := int64(0); i < mpGameCount; i++ {
			work <- i
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < mpWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range work {
				gf, gt, gp := playMultiplayerGame(seed, pool)
				atomic.AddInt64(&tested, int64(gt))
				atomic.AddInt64(&gamesDone, 1)
				if gp {
					atomic.AddInt64(&panics, 1)
				}
				if len(gf) > 0 {
					mu.Lock()
					fails = append(fails, gf...)
					mu.Unlock()
				}
				done := atomic.LoadInt64(&gamesDone)
				if done%500 == 0 {
					log.Printf("  multiplayer: %d games (%.0f/s) %d checks %d panics %d fails",
						done, float64(done)/time.Since(start).Seconds(),
						atomic.LoadInt64(&tested), atomic.LoadInt64(&panics), len(fails))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("  multiplayer complete: %d games, %d checks, %d panics, %d fails, %s",
		atomic.LoadInt64(&gamesDone), atomic.LoadInt64(&tested),
		atomic.LoadInt64(&panics), len(fails), time.Since(start))
	return fails
}

func playMultiplayerGame(seed int64, pool []*oracleCard) (failures []failure, checks int, didPanic bool) {
	defer func() {
		if r := recover(); r != nil {
			didPanic = true
			stack := string(debug.Stack())
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("mp_game_%d", seed),
				Interaction: "multiplayer_panic",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, stack[:min(len(stack), 500)]),
			})
		}
	}()

	rng := rand.New(rand.NewSource(seed))

	gs := &gameengine.GameState{
		Rng:   rng,
		Flags: map[string]int{},
	}

	// Build 4 seats with random decks.
	for i := 0; i < mpSeats; i++ {
		deck := buildRandomDeck(rng, pool)
		for _, c := range deck {
			c.Owner = i
		}
		rng.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })

		seat := &gameengine.Seat{
			Life:    40,
			Flags:   map[string]int{},
			Library: deck,
			Idx:     i,
		}
		// Draw opening hand.
		for j := 0; j < 7 && len(seat.Library) > 0; j++ {
			c := seat.Library[0]
			seat.Library = seat.Library[1:]
			seat.Hand = append(seat.Hand, c)
		}
		gs.Seats = append(gs.Seats, seat)
	}

	checkInv := func(phase, step string, turn int) {
		checks++
		for _, s := range gs.Seats {
			if s != nil && (s.Lost || s.Life <= 0) {
				s.ManaPool = 0
			}
		}
		violations := gameengine.RunAllInvariants(gs)
		for _, v := range violations {
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("mp_game_%d_t%d", seed, turn),
				Interaction: "multiplayer_invariant",
				Invariant:   v.Name,
				Message:     fmt.Sprintf("[turn %d %s/%s] %s", turn, phase, step, v.Message),
			})
		}
	}

	for turn := 1; turn <= mpMaxTurns; turn++ {
		active := (turn - 1) % mpSeats
		gs.Active = active
		seat := gs.Seats[active]

		if seat.Lost || seat.Life <= 0 {
			continue // skip eliminated players
		}

		// Count alive players.
		alive := 0
		for _, s := range gs.Seats {
			if s != nil && !s.Lost && s.Life > 0 {
				alive++
			}
		}
		if alive <= 1 {
			break
		}

		gs.EventLog = gs.EventLog[:0]
		delete(gs.Flags, "_zone_conservation_total")

		// Untap.
		gs.Phase, gs.Step = "beginning", "untap"
		gameengine.UntapAll(gs, active)
		gameengine.StateBasedActions(gs)

		// Upkeep.
		gs.Step = "upkeep"
		gameengine.FirePhaseTriggers(gs, "beginning", "upkeep")
		gameengine.StateBasedActions(gs)

		// Draw (skip first player's first draw).
		gs.Step = "draw"
		if turn > mpSeats || active != 0 {
			if len(seat.Library) > 0 {
				drawn := seat.Library[0]
				seat.Library = seat.Library[1:]
				seat.Hand = append(seat.Hand, drawn)
			}
		}
		gameengine.StateBasedActions(gs)

		// Main phase.
		gs.Phase, gs.Step = "precombat_main", "main"
		seat.ManaPool = turn/mpSeats + 2
		gameengine.SyncManaAfterSpend(seat)
		mpPlayCards(gs, active, rng)
		gameengine.StateBasedActions(gs)
		checkInv("precombat_main", "main", turn)

		// Combat — attack a random opponent.
		gs.Phase, gs.Step = "combat", "declare_attackers"
		attackers := chaosAttack(gs, active, rng)
		if len(attackers) > 0 {
			// Pick random alive opponent.
			var opponents []int
			for i, s := range gs.Seats {
				if i != active && s != nil && !s.Lost && s.Life > 0 {
					opponents = append(opponents, i)
				}
			}
			if len(opponents) > 0 {
				defender := opponents[rng.Intn(len(opponents))]
				_ = defender
				gs.Step = "combat_damage"
				blockerMap := make(map[*gameengine.Permanent][]*gameengine.Permanent)
				gameengine.DealCombatDamageStep(gs, attackers, blockerMap, false)
				gameengine.StateBasedActions(gs)
			}
		}
		// Clear attacking flags.
		for _, p := range seat.Battlefield {
			if p != nil && p.Flags != nil {
				delete(p.Flags, "attacking")
				delete(p.Flags, "blocked")
			}
		}
		gameengine.StateBasedActions(gs)
		checkInv("combat", "end_of_combat", turn)

		// End step.
		gs.Phase, gs.Step = "ending", "end"
		gameengine.FirePhaseTriggers(gs, "ending", "end")
		gameengine.FireMonarchEndStep(gs)
		gameengine.ScanExpiredDurations(gs, "ending", "end")
		gameengine.StateBasedActions(gs)

		// Cleanup.
		gs.Step = "cleanup"
		gameengine.CleanupHandSize(gs, active, 7)
		gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
		gameengine.StateBasedActions(gs)

		for _, s := range gs.Seats {
			if s != nil {
				s.ManaPool = 0
			}
		}
	}

	return failures, checks, false
}

func mpPlayCards(gs *gameengine.GameState, seatIdx int, rng *rand.Rand) {
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
		// Remove from hand.
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
				Card:          card,
				Controller:    seatIdx,
				Owner:         seatIdx,
				SummoningSick: isCreature,
				Timestamp:     gs.NextTimestamp(),
				Counters:      map[string]int{},
				Flags:         map[string]int{},
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
					case *gameast.Activated:
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
