package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYisanTheWandererBard wires Yisan, the Wanderer Bard.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Yisan%2C%20the%20Wanderer%20Bard):
//
//	{2}{G}, {T}, Put a verse counter on Yisan: Search your library for
//	a creature card with mana value equal to the number of verse
//	counters on Yisan, put it onto the battlefield, then shuffle.
//
// {2}{G} Legendary Creature — Human Rogue Bard 2/2. Top-tier mono-G
// toolbox commander — every activation tutors the next size up,
// chaining from 1-mana dorks (Llanowar Elves) → 2-mana value (Eternal
// Witness, Selvala) → 3-mana ramp/draw (Reclamation Sage) → 4-mana
// engines (Acidic Slime, Quirion Ranger sibling) → 5+ bombs.
//
// Implementation:
//   - OnActivated(0): pay {2}{G} (ManaPool >= 3), tap Yisan, add a
//     verse counter, then search the library for a creature with
//     mana value EQUAL to the new verse-counter count, put onto
//     battlefield, shuffle.
//   - The "search-shuffle" path is the same as Yisan's flavor: the
//     library is shuffled regardless of search success (CR §701.18b).
//   - Tutor picker walks library in order looking for the first
//     creature whose c.CMC == counters. Library walk is order-stable
//     so test fixtures with multiple-CMC creatures land deterministically.
func registerYisanTheWandererBard(r *Registry) {
	r.OnActivated("Yisan, the Wanderer Bard", yisanTheWandererBardActivate)
}

func yisanTheWandererBardActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yisan_wanderer_bard"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Yisan, the Wanderer Bard", "already_tapped", nil)
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
	if s.ManaPool < 3 {
		emitFail(gs, slug, "Yisan, the Wanderer Bard", "insufficient_mana", map[string]interface{}{
			"mana_pool": s.ManaPool,
		})
		return
	}

	// Pay cost.
	s.ManaPool -= 3
	gameengine.SyncManaAfterSpend(s)
	src.Tapped = true

	// Add a verse counter.
	src.AddCounter("verse", 1)
	verses := src.Counters["verse"]

	// Search for a creature with mana value == verses.
	foundIdx := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		isCreature := false
		for _, t := range c.Types {
			if t == "creature" {
				isCreature = true
				break
			}
		}
		if !isCreature {
			continue
		}
		// Match on CMC field OR cmc:N type tag (test fixtures often use the tag).
		if cardCMC(c) == verses || c.CMC == verses {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Yisan, the Wanderer Bard", "no_creature_at_cmc", map[string]interface{}{
			"seat":   seat,
			"verses": verses,
		})
		return
	}

	found := s.Library[foundIdx]
	s.Library = append(s.Library[:foundIdx], s.Library[foundIdx+1:]...)
	enterBattlefieldWithETB(gs, seat, found, false)
	shuffleLibraryPerCard(gs, seat)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: "Yisan, the Wanderer Bard",
		Details: map[string]interface{}{
			"found_card":  found.DisplayName(),
			"destination": "battlefield",
			"verses":      verses,
		},
	})
	emit(gs, slug, "Yisan, the Wanderer Bard", map[string]interface{}{
		"seat":   seat,
		"verses": verses,
		"found":  found.DisplayName(),
	})
}
