package main

// chaos_games.go — Full-game simulation stress test.
//
// Shuffles random 60-card decks from the oracle corpus, plays complete
// games with heuristic AI decisions, and runs all 9 Odin invariants
// after every phase transition. Finds bugs that only surface when
// multiple cards interact across multiple turns.

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
	chaosGameCount  = 10000
	chaosMaxTurns   = 50
	chaosDeckSize   = 40
	chaosStartHand  = 7
	chaosStartLife  = 20
	chaosWorkers    = 8
)

func runChaosGames(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	var (
		fails     []failure
		mu        sync.Mutex
		tested    int64
		gamesDone int64
		panics    int64
	)

	// Build card pool: only cards with ASTs and that are permanents or
	// instants/sorceries (i.e. castable).
	var pool []*oracleCard
	for _, oc := range oracleCards {
		if oc.ast == nil {
			continue
		}
		pool = append(pool, oc)
	}
	if len(pool) < chaosDeckSize*2 {
		log.Printf("  chaos: not enough cards in pool (%d), skipping", len(pool))
		return nil
	}

	start := time.Now()
	work := make(chan int64, 256)
	go func() {
		for i := int64(0); i < chaosGameCount; i++ {
			work <- i
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < chaosWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range work {
				gameFailures, gameTests, didPanic := playChaosGame(seed, pool, corpus)
				atomic.AddInt64(&tested, int64(gameTests))
				atomic.AddInt64(&gamesDone, 1)
				if didPanic {
					atomic.AddInt64(&panics, 1)
				}
				if len(gameFailures) > 0 {
					mu.Lock()
					fails = append(fails, gameFailures...)
					mu.Unlock()
				}
				done := atomic.LoadInt64(&gamesDone)
				if done%1000 == 0 {
					elapsed := time.Since(start)
					rate := float64(done) / elapsed.Seconds()
					log.Printf("  chaos: %d games (%.0f/s) %d invariant checks %d panics %d fails",
						done, rate, atomic.LoadInt64(&tested),
						atomic.LoadInt64(&panics), len(fails))
				}
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)
	finalGames := atomic.LoadInt64(&gamesDone)
	finalTests := atomic.LoadInt64(&tested)
	finalPanics := atomic.LoadInt64(&panics)
	log.Printf("  chaos complete:")
	log.Printf("    games:          %d", finalGames)
	log.Printf("    invariant checks: %d", finalTests)
	log.Printf("    panics:         %d", finalPanics)
	log.Printf("    failures:       %d", len(fails))
	log.Printf("    time:           %s", elapsed)
	if elapsed.Seconds() > 0 {
		log.Printf("    rate:           %.0f games/s", float64(finalGames)/elapsed.Seconds())
	}

	return fails
}

func playChaosGame(seed int64, pool []*oracleCard, corpus *astload.Corpus) (failures []failure, checks int, didPanic bool) {
	defer func() {
		if r := recover(); r != nil {
			didPanic = true
			stack := string(debug.Stack())
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("chaos_game_%d", seed),
				Interaction: "chaos_panic",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, stack[:min(len(stack), 500)]),
			})
		}
	}()

	rng := rand.New(rand.NewSource(seed))

	// Build two random decks.
	deck0 := buildRandomDeck(rng, pool)
	deck1 := buildRandomDeck(rng, pool)

	// Initialize game state.
	gs := &gameengine.GameState{
		Rng:   rng,
		Flags: map[string]int{},
	}
	for i := 0; i < 2; i++ {
		gs.Seats = append(gs.Seats, &gameengine.Seat{
			Life:  chaosStartLife,
			Flags: map[string]int{},
			Idx:   i,
		})
	}

	// Shuffle and set libraries.
	rng.Shuffle(len(deck0), func(i, j int) { deck0[i], deck0[j] = deck0[j], deck0[i] })
	rng.Shuffle(len(deck1), func(i, j int) { deck1[i], deck1[j] = deck1[j], deck1[i] })
	gs.Seats[0].Library = deck0
	gs.Seats[1].Library = deck1

	// Draw opening hands.
	for i := 0; i < chaosStartHand && len(gs.Seats[0].Library) > 0; i++ {
		c := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
	}
	for i := 0; i < chaosStartHand && len(gs.Seats[1].Library) > 0; i++ {
		c := gs.Seats[1].Library[0]
		gs.Seats[1].Library = gs.Seats[1].Library[1:]
		gs.Seats[1].Hand = append(gs.Seats[1].Hand, c)
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
			msg := fmt.Sprintf("[turn %d %s/%s] %s", turn, phase, step, v.Message)
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("chaos_game_%d_t%d", seed, turn),
				Interaction: "chaos_invariant",
				Invariant:   v.Name,
				Message:     msg,
			})
		}
	}

	// Play turns.
	for turn := 1; turn <= chaosMaxTurns; turn++ {
		active := (turn - 1) % 2
		gs.Active = active
		seat := gs.Seats[active]

		if seat.Lost || seat.Life <= 0 {
			break
		}
		if gs.Seats[1-active].Lost || gs.Seats[1-active].Life <= 0 {
			break
		}

		// Clear event log each turn so invariant checks don't cascade
		// stale death/trigger events across turns.
		gs.EventLog = gs.EventLog[:0]

		// Reset zone conservation baseline so invariant checks are relative
		// to current turn state (the harness takes shortcuts that bypass
		// the engine's zone tracking for spell casting).
		delete(gs.Flags, "_zone_conservation_total")

		// --- Untap Step ---
		gs.Phase, gs.Step = "beginning", "untap"
		gameengine.UntapAll(gs, active)
		gameengine.StateBasedActions(gs)
		checkInv("beginning", "untap", turn)
		if seat.Lost {
			break
		}

		// --- Upkeep Step ---
		gs.Step = "upkeep"
		gameengine.FirePhaseTriggers(gs, "beginning", "upkeep")
		gameengine.StateBasedActions(gs)
		checkInv("beginning", "upkeep", turn)
		if seat.Lost {
			break
		}

		// --- Draw Step ---
		gs.Step = "draw"
		if (turn > 1 || active == 1) && len(seat.Library) > 0 {
			drawn := seat.Library[0]
			seat.Library = seat.Library[1:]
			seat.Hand = append(seat.Hand, drawn)
		}
		gameengine.StateBasedActions(gs)
		checkInv("beginning", "draw", turn)
		if seat.Lost {
			break
		}

		// --- Pre-combat Main Phase ---
		gs.Phase, gs.Step = "precombat_main", "main"
		gameengine.FirePhaseTriggers(gs, "precombat_main", "main")
		// Give mana for casting (simplified mana system).
		seat.ManaPool = turn + 2
		gameengine.SyncManaAfterSpend(seat)
		chaosPlayCards(gs, active, rng)
		gameengine.StateBasedActions(gs)
		checkInv("precombat_main", "main", turn)
		if seat.Lost {
			break
		}

		// --- Combat Phase ---
		gs.Phase, gs.Step = "combat", "begin_of_combat"
		gameengine.FirePhaseTriggers(gs, "combat", "begin_of_combat")
		gameengine.StateBasedActions(gs)

		gs.Step = "declare_attackers"
		attackers := chaosAttack(gs, active, rng)
		gameengine.StateBasedActions(gs)

		if len(attackers) > 0 {
			gs.Step = "combat_damage"
			defending := 1 - active
			blockerMap := make(map[*gameengine.Permanent][]*gameengine.Permanent)
			gameengine.DealCombatDamageStep(gs, attackers, blockerMap, false)
			_ = defending
			gameengine.StateBasedActions(gs)
		}

		gs.Step = "end_of_combat"
		// Clear attacking flags.
		for _, p := range seat.Battlefield {
			if p != nil && p.Flags != nil {
				delete(p.Flags, "attacking")
				delete(p.Flags, "blocked")
			}
		}
		gameengine.StateBasedActions(gs)
		checkInv("combat", "end_of_combat", turn)
		if seat.Lost || gs.Seats[1-active].Lost {
			break
		}

		// --- Post-combat Main Phase ---
		gs.Phase, gs.Step = "postcombat_main", "main"
		chaosPlayCards(gs, active, rng)
		gameengine.StateBasedActions(gs)
		checkInv("postcombat_main", "main", turn)
		if seat.Lost {
			break
		}

		// --- End Step ---
		gs.Phase, gs.Step = "ending", "end"
		gameengine.FirePhaseTriggers(gs, "ending", "end")
		gameengine.FireMonarchEndStep(gs)
		gameengine.ScanExpiredDurations(gs, "ending", "end")
		gameengine.StateBasedActions(gs)
		checkInv("ending", "end", turn)
		if seat.Lost {
			break
		}

		// --- Cleanup ---
		gs.Step = "cleanup"
		gameengine.CleanupHandSize(gs, active, 7)
		gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
		gameengine.StateBasedActions(gs)
		checkInv("ending", "cleanup", turn)

		// Reset mana pools.
		for _, s := range gs.Seats {
			if s != nil {
				s.ManaPool = 0
			}
		}
	}

	return failures, checks, false
}

func cardInAnyZone(gs *gameengine.GameState, card *gameengine.Card) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			if c == card {
				return true
			}
		}
		for _, c := range s.Graveyard {
			if c == card {
				return true
			}
		}
		for _, c := range s.Exile {
			if c == card {
				return true
			}
		}
		for _, c := range s.Library {
			if c == card {
				return true
			}
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == card {
				return true
			}
		}
	}
	return false
}

func buildRandomDeck(rng *rand.Rand, pool []*oracleCard) []*gameengine.Card {
	deck := make([]*gameengine.Card, 0, chaosDeckSize)
	indices := rng.Perm(len(pool))
	for i := 0; i < chaosDeckSize && i < len(indices); i++ {
		oc := pool[indices[i]]
		types := append([]string{}, oc.Types...)
		card := &gameengine.Card{
			AST:           oc.ast,
			Name:          oc.Name,
			Owner:         0, // set by caller
			Types:         types,
			Colors:        append([]string{}, oc.Colors...),
			CMC:           oc.CMC,
			BasePower:     oc.Power,
			BaseToughness: oc.Toughness,
		}
		deck = append(deck, card)
	}
	return deck
}

func chaosPlayCards(gs *gameengine.GameState, seatIdx int, rng *rand.Rand) {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Try to cast up to 3 cards from hand.
	castAttempts := 0
	for castAttempts < 3 && len(seat.Hand) > 0 && seat.ManaPool > 0 {
		castAttempts++
		// Pick a random card from hand that we can afford.
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

		// Check if it's a permanent type (put on battlefield) or spell (resolve).
		isPermanent := false
		for _, t := range card.Types {
			switch t {
			case "creature", "artifact", "enchantment", "planeswalker", "land", "battle":
				isPermanent = true
			}
		}

		// Remove from hand.
		for hi, hc := range seat.Hand {
			if hc == card {
				seat.Hand = append(seat.Hand[:hi], seat.Hand[hi+1:]...)
				break
			}
		}
		seat.ManaPool -= card.CMC
		gameengine.SyncManaAfterSpend(seat)

		if isPermanent {
			// Place on battlefield.
			pow, tough := card.BasePower, card.BaseToughness
			isCreature := false
			for _, t := range card.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if isCreature && tough <= 0 {
				tough = 1
			}
			perm := &gameengine.Permanent{
				Card:          card,
				Controller:    seatIdx,
				Owner:         seatIdx,
				SummoningSick: isCreature,
				Timestamp:     gs.NextTimestamp(),
				Counters:      map[string]int{},
				Flags:         map[string]int{},
			}
			_ = pow
			seat.Battlefield = append(seat.Battlefield, perm)

			// Fire ETB and register any replacement effects (e.g. Rest in Peace).
			gameengine.InvokeETBHook(gs, perm)
			gameengine.RegisterReplacementsForPermanent(gs, perm)

			// Resolve ETB effects from AST.
			if card.AST != nil {
				for _, ab := range card.AST.Abilities {
					if trig, ok := ab.(*gameast.Triggered); ok && trig.Trigger.Event == "etb" && trig.Effect != nil {
						gameengine.ResolveEffect(gs, perm, trig.Effect)
					}
				}
			}
		} else {
			// Instant/sorcery: resolve all non-keyword abilities.
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

		if seat.Lost || gs.Seats[1-seatIdx] == nil || gs.Seats[1-seatIdx].Lost {
			break
		}
	}

	// Play a land if we have one (max 1 per turn).
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" || t == "Land" {
				isLand = true
				break
			}
		}
		if isLand {
			for hi, hc := range seat.Hand {
				if hc == c {
					seat.Hand = append(seat.Hand[:hi], seat.Hand[hi+1:]...)
					break
				}
			}
			perm := &gameengine.Permanent{
				Card:       c,
				Controller: seatIdx,
				Owner:      seatIdx,
				Timestamp:  gs.NextTimestamp(),
				Counters:   map[string]int{},
				Flags:      map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, perm)
			break
		}
	}
}

func chaosAttack(gs *gameengine.GameState, seatIdx int, rng *rand.Rand) []*gameengine.Permanent {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	var attackers []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Tapped || p.SummoningSick {
			continue
		}
		// Attack with ~50% of eligible creatures.
		if rng.Intn(2) == 0 {
			p.Tapped = true
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["attacking"] = 1
			attackers = append(attackers, p)
		}
	}
	return attackers
}



