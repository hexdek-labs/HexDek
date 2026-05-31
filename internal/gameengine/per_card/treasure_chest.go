package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Treasure Chest — {3} Artifact. {4}, Sacrifice this artifact: Roll a d20.
//   1       Trapped! — You lose 3 life.
//   2-9     Create five Treasure tokens.
//   10-19   You gain 3 life and draw three cards.
//   20      Search your library for a card. If it's an artifact card, you
//           may put it onto the battlefield. Otherwise, put that card into
//           your hand. Then shuffle.
//
// The engine's {4}+sac cost-pay path runs before this handler fires, so
// the artifact is already sacrificed by the time we reach the d20 dispatch.
// Tier-20 search: greedy — pick the first artifact in library (battlefield
// path) if one exists, else first card (hand path). Same library scan
// convention as Hermit Druid (`s.Library[0]` = top per state.go:913).

func init() {
	registerTreasureChest(Global())
	AddResetHook(registerTreasureChest)
}

func registerTreasureChest(r *Registry) {
	r.OnActivated("Treasure Chest", treasureChestActivate)
}

func treasureChestActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "treasure_chest"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	roll := 1
	if gs.Rng != nil {
		roll = gs.Rng.Intn(20) + 1
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "roll_die",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Amount: roll,
		Details: map[string]interface{}{"sides": 20},
	})
	gameengine.FireCardTrigger(gs, "roll_dice", map[string]interface{}{
		"seat": seat, "result": roll, "sides": 20,
	})

	armDetails := map[string]interface{}{"seat": seat, "roll": roll}
	switch {
	case roll == 1:
		gameengine.LoseLife(gs, seat, 3, src.Card.DisplayName())
		armDetails["arm"] = "trapped"

	case roll >= 2 && roll <= 9:
		for i := 0; i < 5; i++ {
			gameengine.CreateTreasureToken(gs, seat)
		}
		armDetails["arm"] = "five_treasures"

	case roll >= 10 && roll <= 19:
		gameengine.GainLife(gs, seat, 3, src.Card.DisplayName())
		drawn := 0
		for i := 0; i < 3; i++ {
			if drawOne(gs, seat, src.Card.DisplayName()) != nil {
				drawn++
			}
		}
		armDetails["arm"] = "gain_and_draw"
		armDetails["drawn"] = drawn

	default: // 20
		picked := pickTreasureChestArtifact(s)
		if picked == nil {
			picked = pickTreasureChestAnyCard(s)
		}
		if picked != nil {
			if cardHasType(picked, "artifact") && gameengine.CardCanEnterBattlefield(picked) {
				removeFromLibrary(s, picked)
				enterBattlefieldWithETB(gs, seat, picked, false)
				armDetails["arm"] = "search_to_battlefield"
				armDetails["card"] = picked.DisplayName()
			} else {
				moveCardBetweenZones(gs, seat, picked, "library", "hand", "treasure_chest_search")
				armDetails["arm"] = "search_to_hand"
				armDetails["card"] = picked.DisplayName()
			}
		} else {
			armDetails["arm"] = "search_empty"
		}
		shuffleSeatLibrary(gs, seat)
	}
	emit(gs, slug, src.Card.DisplayName(), armDetails)
}

func pickTreasureChestArtifact(s *gameengine.Seat) *gameengine.Card {
	if s == nil {
		return nil
	}
	for _, c := range s.Library {
		if c != nil && cardHasType(c, "artifact") {
			return c
		}
	}
	return nil
}

func pickTreasureChestAnyCard(s *gameengine.Seat) *gameengine.Card {
	if s == nil || len(s.Library) == 0 {
		return nil
	}
	return s.Library[0]
}

func removeFromLibrary(s *gameengine.Seat, c *gameengine.Card) {
	for i, x := range s.Library {
		if x == c {
			s.Library = append(s.Library[:i], s.Library[i+1:]...)
			return
		}
	}
}

func shuffleSeatLibrary(gs *gameengine.GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Rng == nil {
		return
	}
	lib := gs.Seats[seat].Library
	gs.Rng.Shuffle(len(lib), func(i, j int) { lib[i], lib[j] = lib[j], lib[i] })
}
