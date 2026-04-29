package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Fetchland handler — shared helper for all fetch-style lands
// -----------------------------------------------------------------------------
//
// Fetchlands share identical oracle text modulo land types and cost:
//
//   {T}, Pay 1 life, Sacrifice ~: Search your library for a [type A] or
//   [type B] card, put it onto the battlefield, then shuffle.
//
// Slow fetches (Evolving Wilds, Terramorphic Expanse, Fabled Passage) skip
// the life payment and the fetched land enters tapped.
//
// We register a single OnActivated handler per fetch, all routing through
// fetchLandActivated with captured parameters.

// fetchLandConfig holds the per-card parameters for a fetchland.
type fetchLandConfig struct {
	// LandTypes is the list of basic land subtypes the fetch can find.
	// e.g. ["island", "mountain"] for Scalding Tarn.
	LandTypes []string

	// EntersTapped controls whether the fetched land enters tapped.
	EntersTapped bool

	// LifeCost is the life paid as an additional cost (0 for slow fetches).
	LifeCost int
}

// fetchLandDefs maps card names to their fetchland configuration.
var fetchLandDefs = map[string]fetchLandConfig{
	// Original Onslaught/Zendikar fetchlands — pay 1 life, untapped.
	"Scalding Tarn":       {LandTypes: []string{"island", "mountain"}, LifeCost: 1},
	"Misty Rainforest":    {LandTypes: []string{"forest", "island"}, LifeCost: 1},
	"Verdant Catacombs":   {LandTypes: []string{"swamp", "forest"}, LifeCost: 1},
	"Bloodstained Mire":   {LandTypes: []string{"swamp", "mountain"}, LifeCost: 1},
	"Wooded Foothills":    {LandTypes: []string{"mountain", "forest"}, LifeCost: 1},
	"Flooded Strand":      {LandTypes: []string{"plains", "island"}, LifeCost: 1},
	"Windswept Heath":     {LandTypes: []string{"forest", "plains"}, LifeCost: 1},
	"Arid Mesa":           {LandTypes: []string{"mountain", "plains"}, LifeCost: 1},
	"Polluted Delta":      {LandTypes: []string{"island", "swamp"}, LifeCost: 1},
	"Marsh Flats":         {LandTypes: []string{"plains", "swamp"}, LifeCost: 1},
	// Slow fetches — no life, enters tapped, any basic.
	"Evolving Wilds":      {LandTypes: []string{"plains", "island", "swamp", "mountain", "forest"}, EntersTapped: true},
	"Terramorphic Expanse": {LandTypes: []string{"plains", "island", "swamp", "mountain", "forest"}, EntersTapped: true},
	// Fabled Passage — no life cost; enters tapped unless you control 4+ lands.
	// MVP: we treat it as untapped (cEDH games usually have 4+ lands when cracked).
	"Fabled Passage":      {LandTypes: []string{"plains", "island", "swamp", "mountain", "forest"}, LifeCost: 0},
	// Prismatic Vista — pay 1 life, any basic, untapped.
	"Prismatic Vista":     {LandTypes: []string{"plains", "island", "swamp", "mountain", "forest"}, LifeCost: 1},
}

func registerFetchlands(r *Registry) {
	for name := range fetchLandDefs {
		n := name // capture
		r.OnActivated(n, func(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
			fetchLandActivated(gs, src, n)
		})
	}
}

// fetchLandActivated is the shared activated-ability handler for all fetchlands.
func fetchLandActivated(gs *gameengine.GameState, src *gameengine.Permanent, cardName string) {
	const slug = "fetchland_activate"
	if gs == nil || src == nil {
		return
	}
	cfg, ok := fetchLandDefs[cardName]
	if !ok {
		emitFail(gs, slug, cardName, "unknown_fetchland", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Pay life cost.
	if cfg.LifeCost > 0 {
		s.Life -= cfg.LifeCost
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   seat,
			Target: seat,
			Source: cardName,
			Amount: cfg.LifeCost,
			Details: map[string]interface{}{
				"reason": "fetchland_activation_cost",
			},
		})
	}

	// Sacrifice self.
	gameengine.SacrificePermanent(gs, src, "fetchland_activation")

	// Search library for a matching land card.
	foundIdx := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		if landMatchesFetchTypes(c, cfg.LandTypes) {
			foundIdx = i
			break
		}
	}

	if foundIdx < 0 {
		// No matching land found — still shuffle (you searched).
		shuffleLibraryPerCard(gs, seat)
		emit(gs, slug, cardName, map[string]interface{}{
			"seat":        seat,
			"found":       false,
			"life_paid":   cfg.LifeCost,
		})
		_ = gs.CheckEnd()
		return
	}

	// Remove the land from library.
	landCard := s.Library[foundIdx]
	s.Library = append(s.Library[:foundIdx], s.Library[foundIdx+1:]...)

	// Shuffle library.
	shuffleLibraryPerCard(gs, seat)

	// Put the land onto the battlefield with full ETB cascade.
	enterBattlefieldWithETB(gs, seat, landCard, cfg.EntersTapped)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: cardName,
		Details: map[string]interface{}{
			"found_card":    landCard.DisplayName(),
			"enters_tapped": cfg.EntersTapped,
			"reason":        "fetchland",
		},
	})
	emit(gs, slug, cardName, map[string]interface{}{
		"seat":           seat,
		"found":          true,
		"fetched_land":   landCard.DisplayName(),
		"enters_tapped":  cfg.EntersTapped,
		"life_paid":      cfg.LifeCost,
	})
	_ = gs.CheckEnd()
}

// landMatchesFetchTypes returns true if the card is a land that has at least
// one of the specified basic land subtypes in its Types slice.
func landMatchesFetchTypes(c *gameengine.Card, landTypes []string) bool {
	if c == nil {
		return false
	}
	isLand := false
	for _, t := range c.Types {
		if t == "land" {
			isLand = true
			break
		}
	}
	if !isLand {
		return false
	}
	for _, t := range c.Types {
		tLower := strings.ToLower(t)
		for _, wanted := range landTypes {
			if tLower == wanted {
				return true
			}
		}
	}
	return false
}

// shuffleLibraryPerCard shuffles a seat's library. Since the gameengine's
// shuffleLibrary is unexported, we replicate the logic here.
func shuffleLibraryPerCard(gs *gameengine.GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || gs.Rng == nil {
		return
	}
	lib := gs.Seats[seat].Library
	gs.Rng.Shuffle(len(lib), func(i, j int) {
		lib[i], lib[j] = lib[j], lib[i]
	})
}
