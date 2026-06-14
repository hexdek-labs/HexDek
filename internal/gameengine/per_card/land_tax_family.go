package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// land_tax_family.go — generic handler for the "if opponent controls more
// lands than you, search your library for a [basic] <type> card" family.
//
// One shape, many printings. Every card in this family runs the same
// algorithm:
//   1. Gate on a land-count comparator (controller vs opponent, or
//      defending-player vs controller for attack triggers).
//   2. Search the controller's library for the first card matching a
//      subtype/type filter.
//   3. Put it onto the battlefield (tapped or untapped), shuffle.
//
// The differences are confined to the trigger event (ETB / upkeep / attack),
// the land filter (Plains, Desert, "basic land"), the destination (hand
// vs battlefield), enters-tapped, the count (Land Tax fetches three), and
// the comparator side (attacker vs defending player). All of those are
// configuration — the algorithm is one body.
//
// Hand-rolled siblings registered earlier (Knight of the White Orchid,
// Claim Jumper, Land Tax) are left as-is so this file only stitches in
// the gap cards. New family members drop in as a one-line entry below.

type landFetchTriggerKind int

const (
	landFetchTriggerETB landFetchTriggerKind = iota
	landFetchTriggerUpkeep
	landFetchTriggerAttack
)

// landFetchFamilyConfig is one row of the family table.
type landFetchFamilyConfig struct {
	cardName   string
	trigger    landFetchTriggerKind
	filter     landFetchFilter // describes which library cards qualify
	destZone   string          // "battlefield" or "hand"
	tapped     bool            // entered tapped (battlefield only)
	count      int             // how many cards to fetch (Land Tax = 3)
	may        bool            // optional ability (Hat auto-accepts)
	repeat     int             // re-evaluate gate and fetch again (Claim Jumper = 2)
	comparator landFetchComparator
}

type landFetchComparator int

const (
	// "if an opponent controls more lands than you" — any opponent.
	cmpAnyOpponentMoreLands landFetchComparator = iota
	// "if defending player controls more lands than you" — attack-trigger
	// shape where the gate looks at the *defending* player only.
	cmpDefendingPlayerMoreLands
)

// landFetchFilter picks which library cards count.
type landFetchFilter struct {
	requireBasic bool   // "basic Plains card" vs "Plains card"
	subtype      string // "plains", "desert", "" for any
	anyLand      bool   // "a land card" / "basic land card"
}

// landTaxFamilyEntries lists every Muninn-gap card this file claims.
// Adding a new family member is one line here.
var landTaxFamilyEntries = []landFetchFamilyConfig{
	{
		// Loyal Warhound: "When this creature enters, if an opponent
		// controls more lands than you, search your library for a basic
		// Plains card, put it onto the battlefield tapped, then shuffle."
		cardName:   "Loyal Warhound",
		trigger:    landFetchTriggerETB,
		filter:     landFetchFilter{requireBasic: true, subtype: "plains"},
		destZone:   "battlefield",
		tapped:     true,
		count:      1,
		comparator: cmpAnyOpponentMoreLands,
	},
	{
		// Sand Scout: same shape, Desert filter.
		cardName:   "Sand Scout",
		trigger:    landFetchTriggerETB,
		filter:     landFetchFilter{subtype: "desert"},
		destZone:   "battlefield",
		tapped:     true,
		count:      1,
		comparator: cmpAnyOpponentMoreLands,
	},
	{
		// Aerial Surveyor: "Whenever this Vehicle attacks, if defending
		// player controls more lands than you, search your library for a
		// basic Plains card, put it onto the battlefield tapped, then
		// shuffle."
		cardName:   "Aerial Surveyor",
		trigger:    landFetchTriggerAttack,
		filter:     landFetchFilter{requireBasic: true, subtype: "plains"},
		destZone:   "battlefield",
		tapped:     true,
		count:      1,
		comparator: cmpDefendingPlayerMoreLands,
	},
}

func registerLandTaxFamily(r *Registry) {
	for _, cfg := range landTaxFamilyEntries {
		cfg := cfg
		switch cfg.trigger {
		case landFetchTriggerETB:
			r.OnETB(cfg.cardName, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
				runLandFetchFamily(gs, perm, cfg, nil)
			})
		case landFetchTriggerUpkeep:
			r.OnTrigger(cfg.cardName, "upkeep", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
				activeSeat, _ := ctx["active_seat"].(int)
				if activeSeat != perm.Controller {
					return
				}
				runLandFetchFamily(gs, perm, cfg, ctx)
			})
		case landFetchTriggerAttack:
			r.OnTrigger(cfg.cardName, "attacks", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
				// CR §603.2 self-gate: "Whenever ~ attacks" (Aerial Surveyor)
				// fires only when this permanent itself attacks. creature_attacks
				// is dispatched once per declared attacker, so without this the
				// land fetch ran once per attacker. See the Aang and Katara fix
				// (5254f9cf).
				if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
					return
				}
				runLandFetchFamily(gs, perm, cfg, ctx)
			})
		}
	}
}

func runLandFetchFamily(gs *gameengine.GameState, perm *gameengine.Permanent, cfg landFetchFamilyConfig, ctx map[string]interface{}) {
	slug := "land_tax_family:" + landFetchSlug(cfg.cardName)
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	passes := cfg.repeat
	if passes < 1 {
		passes = 1
	}
	totalCount := cfg.count
	if totalCount < 1 {
		totalCount = 1
	}

	found := []string{}
	for pass := 0; pass < passes; pass++ {
		if !landFetchGateOpen(gs, seat, cfg.comparator, ctx) {
			break
		}
		// Each pass fetches up to `totalCount` cards.
		for k := 0; k < totalCount; k++ {
			card := pickLandFromLibrary(s.Library, cfg.filter)
			if card == nil {
				break
			}
			if cfg.destZone == "battlefield" {
				// MoveCard's "battlefield"/"battlefield_tapped" arm
				// wraps the Card in a Permanent and fires the ETB
				// cascade (RegisterReplacementsForPermanent +
				// FirePermanentETBTriggers). Picking the right zone
				// string here lets the engine set Tapped correctly
				// — passing it through enterBattlefieldWithETB after
				// the fact would dedup back to the already-placed
				// Permanent and lose the tap.
				toZone := "battlefield"
				if cfg.tapped {
					toZone = "battlefield_tapped"
				}
				gameengine.MoveCard(gs, card, seat, "library", toZone, slug+"_search")
			} else {
				gameengine.MoveCard(gs, card, seat, "library", "hand", slug+"_search")
			}
			found = append(found, card.DisplayName())
		}
	}

	if len(found) > 0 {
		shuffleLibraryPerCard(gs, seat)
		gs.LogEvent(gameengine.Event{
			Kind:   "search_library",
			Seat:   seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"found":  found,
				"reason": slug,
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"triggered": len(found) > 0,
		"found":     found,
	})
}

// landFetchGateOpen evaluates the comparator.
func landFetchGateOpen(gs *gameengine.GameState, mySeat int, cmp landFetchComparator, ctx map[string]interface{}) bool {
	mine := countBattlefieldLands(gs.Seats[mySeat].Battlefield)
	switch cmp {
	case cmpAnyOpponentMoreLands:
		for i, s := range gs.Seats {
			if i == mySeat || s == nil || s.Lost {
				continue
			}
			if countBattlefieldLands(s.Battlefield) > mine {
				return true
			}
		}
		return false
	case cmpDefendingPlayerMoreLands:
		// On an attack trigger, the defending player is in ctx as
		// "defender_seat" (engine-set) — fall back to "any opponent"
		// if absent (typical when triggered outside combat).
		dseat, ok := ctx["defender_seat"].(int)
		if !ok {
			dseat = -1
		}
		if dseat < 0 || dseat >= len(gs.Seats) || gs.Seats[dseat] == nil {
			// No specific defending player: treat as any-opponent gate
			// so the trigger still produces value when defending_seat
			// isn't routed through ctx.
			for i, s := range gs.Seats {
				if i == mySeat || s == nil || s.Lost {
					continue
				}
				if countBattlefieldLands(s.Battlefield) > mine {
					return true
				}
			}
			return false
		}
		return countBattlefieldLands(gs.Seats[dseat].Battlefield) > mine
	}
	return false
}

// pickLandFromLibrary returns the first card matching the filter.
func pickLandFromLibrary(library []*gameengine.Card, f landFetchFilter) *gameengine.Card {
	for _, c := range library {
		if c == nil {
			continue
		}
		if !cardHasType(c, "land") {
			continue
		}
		if f.requireBasic && !cardHasType(c, "basic") {
			continue
		}
		if f.subtype != "" && !cardHasSubtype(c, f.subtype) {
			continue
		}
		// f.anyLand needs no subtype check.
		return c
	}
	return nil
}

// landFetchSlug builds a stable slug fragment from a card name.
func landFetchSlug(name string) string {
	s := strings.ToLower(name)
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ', r == '_', r == '-':
			out = append(out, '_')
		}
	}
	return string(out)
}
