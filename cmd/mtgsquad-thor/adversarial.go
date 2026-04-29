package main

// adversarial.go — Adversarial targeting stress tests.
//
// Instead of random play, uses a greedy AI that always targets the most
// complex permanent, fights the largest creature, and casts the highest
// CMC spell. Maximizes interaction density to stress effect resolution.

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	advGameCount = 2000
	advMaxTurns  = 40
	advWorkers   = 8
)

func runAdversarial(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	var pool []*oracleCard
	for _, oc := range oracleCards {
		if oc.ast != nil {
			pool = append(pool, oc)
		}
	}
	if len(pool) < chaosDeckSize*2 {
		log.Printf("  adversarial: not enough cards (%d), skipping", len(pool))
		return nil
	}

	var (
		fails     []failure
		mu        sync.Mutex
		gamesDone int64
		panics    int64
	)

	start := time.Now()
	work := make(chan int64, 256)
	go func() {
		for i := int64(0); i < advGameCount; i++ {
			work <- i
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < advWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range work {
				gf, gp := playAdversarialGame(seed, pool)
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
					log.Printf("  adversarial: %d games, %d panics, %d fails",
						done, atomic.LoadInt64(&panics), len(fails))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("  adversarial complete: %d games, %d panics, %d fails, %s",
		atomic.LoadInt64(&gamesDone), atomic.LoadInt64(&panics),
		len(fails), time.Since(start))
	return fails
}

func playAdversarialGame(seed int64, pool []*oracleCard) (failures []failure, didPanic bool) {
	defer func() {
		if r := recover(); r != nil {
			didPanic = true
			stack := string(debug.Stack())
			failures = append(failures, failure{
				CardName:    fmt.Sprintf("adv_game_%d", seed),
				Interaction: "adversarial_panic",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, stack),
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

	for turn := 1; turn <= advMaxTurns; turn++ {
		active := (turn - 1) % 2
		gs.Active = active
		seat := gs.Seats[active]
		if seat.Lost || seat.Life <= 0 {
			break
		}
		if gs.Seats[1-active].Lost || gs.Seats[1-active].Life <= 0 {
			break
		}

		gs.EventLog = gs.EventLog[:0]

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
		advPlayCards(gs, active)
		gameengine.StateBasedActions(gs)

		// Combat: attack with ALL eligible creatures (greedy).
		gs.Phase, gs.Step = "combat", "declare_attackers"
		attackers := advAttackAll(gs, active)
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
		gameengine.StateBasedActions(gs)

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

	// Run invariants on final state.
	delete(gs.Flags, "_zone_conservation_total")
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		failures = append(failures, failure{
			CardName:    fmt.Sprintf("adv_game_%d", seed),
			Interaction: "adversarial_final",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}

	return failures, false
}

func advPlayCards(gs *gameengine.GameState, seatIdx int) {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Sort hand by CMC descending — always cast the most expensive card first.
	type handEntry struct {
		card *gameengine.Card
		idx  int
	}
	var entries []handEntry
	for i, c := range seat.Hand {
		if c != nil {
			entries = append(entries, handEntry{c, i})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].card.CMC > entries[j].card.CMC
	})

	cast := 0
	for _, e := range entries {
		if cast >= 5 || seat.ManaPool <= 0 {
			break
		}
		card := e.card
		if card.CMC > seat.ManaPool {
			continue
		}
		// Find and remove from hand. If a prior ETB moved this card
		// out of hand (discard, etc.), skip it.
		foundInHand := false
		for hi, hc := range seat.Hand {
			if hc == card {
				seat.Hand = append(seat.Hand[:hi], seat.Hand[hi+1:]...)
				foundInHand = true
				break
			}
		}
		if !foundInHand {
			continue
		}
		seat.ManaPool -= card.CMC
		gameengine.SyncManaAfterSpend(seat)
		cast++

		isPermanent := false
		for _, t := range card.Types {
			switch t {
			case "creature", "artifact", "enchantment", "planeswalker", "land", "battle":
				isPermanent = true
			}
		}

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

func advAttackAll(gs *gameengine.GameState, seatIdx int) []*gameengine.Permanent {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	var attackers []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Tapped || p.SummoningSick {
			continue
		}
		p.Tapped = true
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["attacking"] = 1
		attackers = append(attackers, p)
	}
	return attackers
}
