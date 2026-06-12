package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Possibility Storm
// ---------------------------------------------------------------------------
//
// "Whenever a player casts a spell from their hand, that player exiles it,
// then exiles cards from the top of their library until they exile a card
// that shares a card type with it. That player may cast that card without
// paying its mana cost. Then they put all cards exiled with this
// enchantment on the bottom of their library in a random order."
//
// Post-#685 §400.7c-compliance: every card the caster exiles goes to
// the caster's OWN exile (the caster IS the owner here — both for the
// just-cast spell and for cards revealed off the caster's own library).
// The fix in this file replaces the previous "move matched card to
// hand" path with a proper ZoneCastGrant on the matched card while it
// stays in exile — preserving the "without paying its mana cost"
// semantics and letting the engine's cast-from-exile pipeline drive
// the actual cast. The original pre-#685 implementation also exiled
// the freshly-cast spell to the wrong physical pile occasionally;
// the canonical MoveCard("hand"→"exile") at casterSeat routes it
// correctly per §400.7c since caster == owner for hand-casts.

func registerPossibilityStorm(r *Registry) {
	r.OnTrigger("Possibility Storm", "spell_cast", possibilityStormTrigger)
}

func possibilityStormTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	casterSeat, _ := ctx["caster_seat"].(int)
	castZone, _ := ctx["cast_zone"].(string)
	if card == nil || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	// Oracle says "casts a spell from their hand" — gate to hand-casts
	// only (not flashback / cascade / adventure / etc.).
	if castZone != "" && castZone != gameengine.ZoneHand {
		return
	}
	seat := gs.Seats[casterSeat]
	if seat == nil {
		return
	}

	origTypes := card.Types
	origName := card.DisplayName()

	// The cast spell itself is exiled by Possibility Storm (caster's
	// own exile per §400.7c since caster == owner for hand-casts).
	gameengine.MoveCard(gs, card, casterSeat, "stack", "exile", "possibility-storm-exile-cast-spell")

	var exiled []*gameengine.Card
	var found *gameengine.Card

	for len(seat.Library) > 0 {
		before := len(seat.Library)
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, casterSeat, "library", "exile", "possibility-storm-reveal")
		if len(seat.Library) >= before {
			break // no progress (replacement bounce / §800.4a no-op) — never spin
		}
		exiled = append(exiled, top)
		if top != nil && sharesCardType(top, origTypes) {
			found = top
			break
		}
	}

	if found != nil {
		gs.LogEvent(gameengine.Event{
			Kind:   "possibility_storm",
			Seat:   casterSeat,
			Source: "Possibility Storm",
			Details: map[string]interface{}{
				"original_spell": origName,
				"cast_for_free":  found.DisplayName(),
				"exiled_count":   len(exiled),
			},
		})
		// Remove the matched card from the rest-to-bottom pile — it
		// stays in exile under a free-cast grant. (If the AI doesn't
		// cast it, the card sits in exile permanently in the current
		// MVP. Per oracle, the unused match would go to library bottom
		// with the others, but that requires a post-cast-window
		// callback that we don't have yet.)
		for i, c := range exiled {
			if c == found {
				exiled = append(exiled[:i], exiled[i+1:]...)
				break
			}
		}
		// Register the free-cast grant. RequireController = caster
		// (the player whose hand-cast triggered Possibility Storm —
		// the oracle says "that player may cast"). Duration until_end
		// _of_turn is a conservative bound — the actual cast window
		// is during this trigger's resolution, but EOT cleanup is the
		// engine's smallest expiry hammer.
		grant := gameengine.NewFreeCastFromExilePermission(casterSeat, "Possibility Storm")
		grant.Duration = "until_end_of_turn"
		grant.GrantTurn = gs.Turn
		gameengine.RegisterZoneCastGrant(gs, found, grant)
	}

	if gs.Rng != nil && len(exiled) > 1 {
		gs.Rng.Shuffle(len(exiled), func(i, j int) {
			exiled[i], exiled[j] = exiled[j], exiled[i]
		})
	}
	// Route exile→library_bottom through MoveCard so §614 replacements and
	// exile-leave triggers fire for each returning card.
	pile := exiled
	exiled = nil
	for _, c := range pile {
		gameengine.MoveCard(gs, c, casterSeat, "exile", "library_bottom", "cascade-miss-return")
	}
}

func sharesCardType(c *gameengine.Card, types []string) bool {
	if c == nil {
		return false
	}
	for _, t := range types {
		tl := strings.ToLower(t)
		if tl == "basic" || tl == "legendary" || tl == "token" || tl == "snow" {
			continue // supertypes don't count
		}
		for _, ct := range c.Types {
			if strings.ToLower(ct) == tl {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Chaos Wand
// ---------------------------------------------------------------------------
//
// "{4}, {T}: Target opponent exiles cards from the top of their library
// until they exile an instant or sorcery card. You may cast that card
// without paying its mana cost. Then put the exiled cards that weren't
// cast this way on the bottom of that library in a random order."

func registerChaosWand(r *Registry) {
	r.OnActivated("Chaos Wand", chaosWandActivated)
}

func chaosWandActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		return
	}
	src.Tapped = true
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Pick an opponent with cards in library.
	targetSeat := -1
	for _, opp := range gs.Opponents(seat) {
		if gs.Seats[opp] != nil && len(gs.Seats[opp].Library) > 0 {
			targetSeat = opp
			break
		}
	}
	if targetSeat < 0 {
		return
	}
	oppSeat := gs.Seats[targetSeat]

	var exiled []*gameengine.Card
	var found *gameengine.Card

	if os := gs.Seats[targetSeat]; os == nil || os.Lost || os.LeftGame {
		return // eliminated target: retained zones are forensic, not real (CR §800.4a)
	}
	for len(oppSeat.Library) > 0 {
		before := len(oppSeat.Library)
		top := oppSeat.Library[0]
		gameengine.MoveCard(gs, top, targetSeat, "library", "exile", "chaos-wand-reveal")
		if len(oppSeat.Library) >= before {
			break // no progress (replacement bounce / §800.4a no-op) — never spin
		}
		exiled = append(exiled, top)
		if top != nil && isInstantOrSorceryCard(top) {
			found = top
			break
		}
	}

	if found != nil {
		gs.LogEvent(gameengine.Event{
			Kind:   "chaos_wand",
			Seat:   seat,
			Target: targetSeat,
			Source: "Chaos Wand",
			Details: map[string]interface{}{
				"cast_for_free": found.DisplayName(),
				"exiled_count":  len(exiled),
			},
		})
		for i, c := range exiled {
			if c == found {
				exiled = append(exiled[:i], exiled[i+1:]...)
				break
			}
		}
		// Cross-seat: card is in targetSeat's exile, caster wants to cast it.
		// MoveCard doesn't support cross-seat moves; remove from opponent's
		// exile then place in caster's hand.
		gameengine.MoveCard(gs, found, targetSeat, "exile", "hand", "chaos-wand-cast")
		// Move from opponent's hand to caster's hand (cross-seat fixup).
		for i, c := range gs.Seats[targetSeat].Hand {
			if c == found {
				gs.Seats[targetSeat].Hand = append(gs.Seats[targetSeat].Hand[:i], gs.Seats[targetSeat].Hand[i+1:]...)
				break
			}
		}
		gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, found)
	}

	if gs.Rng != nil && len(exiled) > 1 {
		gs.Rng.Shuffle(len(exiled), func(i, j int) {
			exiled[i], exiled[j] = exiled[j], exiled[i]
		})
	}
	// Route exile→library_bottom through MoveCard for the opponent's
	// library. Per CR §614 replacements and exile-leave triggers fire
	// for each card returning to the opponent's library.
	pile := exiled
	exiled = nil
	for _, c := range pile {
		gameengine.MoveCard(gs, c, targetSeat, "exile", "library_bottom", "cascade-miss-return")
	}
}

func isInstantOrSorceryCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	tl := strings.ToLower(strings.Join(c.Types, " "))
	return strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery")
}

// ---------------------------------------------------------------------------
// Thousand-Year Storm
// ---------------------------------------------------------------------------
//
// "Whenever you cast an instant or sorcery spell, copy it for each other
// instant and sorcery spell you've cast before it this turn. You may
// choose new targets for the copies."

func registerThousandYearStorm(r *Registry) {
	r.OnTrigger("Thousand-Year Storm", "instant_or_sorcery_cast", thousandYearStormTrigger)
}

func thousandYearStormTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return // only your own spells
	}
	spellName, _ := ctx["spell_name"].(string)

	// Count instant/sorcery spells cast this turn BEFORE this one.
	priorCasts := 0
	if gs.Seats[casterSeat] != nil {
		priorCasts = gs.Seats[casterSeat].Turn.NthCastOfType("instant", "sorcery")
		if priorCasts > 0 {
			priorCasts-- // subtract the current cast
		}
	}

	if priorCasts <= 0 {
		return
	}

	// Cap at 10 copies to prevent infinite loops in edge cases.
	if priorCasts > 10 {
		priorCasts = 10
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "thousand_year_storm",
		Seat:   casterSeat,
		Source: "Thousand-Year Storm",
		Amount: priorCasts,
		Details: map[string]interface{}{
			"spell_copied": spellName,
			"copy_count":   priorCasts,
		},
	})
}

// ---------------------------------------------------------------------------
// Phylactery Lich
// ---------------------------------------------------------------------------
//
// "Indestructible. As this creature enters, put a phylactery counter on
// an artifact you control. When you control no permanents with phylactery
// counters on them, sacrifice this creature."

func registerPhylacteryLich(r *Registry) {
	r.OnETB("Phylactery Lich", phylacteryLichETB)
	r.OnTrigger("Phylactery Lich", "permanent_ltb", phylacteryLichCheck)
	r.OnTrigger("Phylactery Lich", "creature_dies", phylacteryLichCheck)
}

func phylacteryLichETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Find an artifact to put a phylactery counter on.
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if strings.Contains(strings.ToLower(p.Card.TypeLine), "artifact") {
			p.AddCounter("phylactery", 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "phylactery_counter",
				Seat:   seat,
				Source: "Phylactery Lich",
				Details: map[string]interface{}{
					"target_artifact": p.Card.DisplayName(),
					"rule":            "phylactery_lich_etb",
				},
			})
			return
		}
	}
}

func phylacteryLichCheck(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	// Check if controller still has any permanent with a phylactery counter.
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil {
			continue
		}
		if p.Counters != nil && p.Counters["phylactery"] > 0 {
			return // still have one, Lich is safe
		}
	}
	// No phylactery counters remain — sacrifice the Lich.
	gameengine.SacrificePermanent(gs, perm, "phylactery_lich_no_counters")
	gs.LogEvent(gameengine.Event{
		Kind:   "phylactery_lich_sacrifice",
		Seat:   seat,
		Source: "Phylactery Lich",
		Details: map[string]interface{}{
			"reason": "no_permanents_with_phylactery_counters",
		},
	})
}
