package hat

// R63 empirical harness — quantifies the tutor-target-selection gap and the
// fix using the REAL engine resolution path (gameengine.ResolveTutorGeneric)
// over many randomized library orderings. Run with:
//
//	go test ./internal/hat/ -run TestEmpirical_TutorTargetGrabRate -v
//
// It models a midrange deck whose actual wincon is a deck-synergy engine
// (a high-density synergy-cluster member, CMC 4) sitting in a library
// alongside a strictly bigger generic bomb (CMC 8) and chaff creatures, all
// matching a single-target "search for a creature" tutor. We measure how
// often each regime tutors up the SYNERGY ENGINE — the correct, deck-shaped
// pick — across N shuffles:
//
//   - random  : models the PRE-FIX engine, which broke out of the library
//     scan at the first match and so grabbed whatever sat earliest in
//     post-shuffle order (uniform over the matching cards).
//   - cmc      : GreedyHat — the post-fix collect-all baseline with no
//     strategy chooser; deterministically grabs the highest-CMC match
//     (the generic bomb), so it never finds the synergy engine here.
//   - strategy : YggdrasilHat.ChooseTutorTargets — strategy-aware.
//
// This is a measurement test: it logs the table and asserts only that the
// strategy regime dominates, so it stays stable across engine tuning.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestEmpirical_TutorTargetGrabRate(t *testing.T) {
	const (
		N           = 3000
		synergyName = "Tireless Provisioner" // CMC 4 — the deck's engine
		bombName    = "Craterhoof Behemoth"  // CMC 8 — generic bomb, highest CMC
	)

	// creatureCMC builds a creature with BOTH the cost: tag (ManaCostOf) and
	// the .CMC field set, so the engine's CMC-baseline selector (which reads
	// .CMC) genuinely ranks by mana value rather than degenerating to
	// first-match.
	creatureCMC := func(name string, cmc int) *gameengine.Card {
		c := libCreature(name, cmc)
		c.CMC = cmc
		return c
	}

	// Library template: synergy engine + bomb + 8 chaff creatures + 5 lands.
	buildLibrary := func() []*gameengine.Card {
		lib := []*gameengine.Card{
			creatureCMC(synergyName, 4),
			creatureCMC(bombName, 8),
		}
		for i := 0; i < 8; i++ {
			lib = append(lib, creatureCMC("Chaff Creature "+itoa(i), 2+i%4))
		}
		for i := 0; i < 5; i++ {
			lib = append(lib, newTestCardMinimal("Forest "+itoa(i), []string{"land"}, 0, nil))
		}
		return lib
	}

	synergyStrategy := func() *StrategyProfile {
		return &StrategyProfile{
			Archetype:       ArchetypeMidrange,
			ValueEngineKeys: []string{bombName}, // generic bomb flagged as value
			SynergyClusters: []SynergyCluster{{
				Name:        "Landfall Treasure",
				Theme:       "landfall",
				Members:     []string{synergyName, "Lotus Cobra", "Scute Swarm", "Field of the Dead", "Tireless Tracker"},
				HighDensity: true,
			}},
		}
	}

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	rng := rand.New(rand.NewSource(20260613))
	shuffle := func(lib []*gameengine.Card) {
		rng.Shuffle(len(lib), func(i, j int) { lib[i], lib[j] = lib[j], lib[i] })
	}

	// grabRate runs N shuffled tutors and returns the fraction that put the
	// synergy engine into hand. hatFor builds the seat 0 hat per trial; when
	// nil, no hat is attached (engine CMC baseline).
	grabRate := func(hatFor func() gameengine.Hat) float64 {
		hits := 0
		for i := 0; i < N; i++ {
			gs := newTestGame(t, 2)
			lib := buildLibrary()
			shuffle(lib)
			gs.Seats[0].Library = lib
			if hatFor != nil {
				gs.Seats[0].Hat = hatFor()
			}
			gameengine.ResolveTutorGeneric(gs, 0, tutor)
			for _, c := range gs.Seats[0].Hand {
				if c.Name == synergyName {
					hits++
					break
				}
			}
		}
		return float64(hits) / float64(N)
	}

	// random regime: model the pre-fix first-match-in-shuffled-order pick by
	// sampling a uniformly random MATCHING card per shuffle.
	randomRate := func() float64 {
		hits := 0
		for i := 0; i < N; i++ {
			lib := buildLibrary()
			shuffle(lib)
			var firstCreature string
			for _, c := range lib {
				isCreature := false
				for _, ty := range c.Types {
					if ty == "creature" {
						isCreature = true
						break
					}
				}
				if isCreature {
					firstCreature = c.Name
					break
				}
			}
			if firstCreature == synergyName {
				hits++
			}
		}
		return float64(hits) / float64(N)
	}

	random := randomRate()
	cmc := grabRate(func() gameengine.Hat { return &GreedyHat{} })
	strategy := grabRate(func() gameengine.Hat { return NewYggdrasilHat(synergyStrategy(), 0) })

	t.Logf("tutor synergy-engine grab-rate over N=%d shuffled libraries:", N)
	t.Logf("  random   (pre-fix first-match): %.1f%%", random*100)
	t.Logf("  cmc      (post-fix baseline)  : %.1f%%", cmc*100)
	t.Logf("  strategy (YggdrasilHat)       : %.1f%%", strategy*100)

	if strategy < 0.99 {
		t.Errorf("strategy regime should grab the synergy engine ~always, got %.1f%%", strategy*100)
	}
	if strategy <= random || strategy <= cmc {
		t.Errorf("strategy regime must dominate baselines (strategy=%.2f random=%.2f cmc=%.2f)", strategy, random, cmc)
	}
}
