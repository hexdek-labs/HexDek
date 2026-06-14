package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFearlessSwashbuckler wires Fearless Swashbuckler (Muninn
// parser-gap #71, ~9.3K hits).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{1}{U}{R}
//	Creature — Fish Pirate
//	Haste
//	Vehicles you control have haste.
//	Whenever you attack, if a Pirate and a Vehicle attacked this combat,
//	draw three cards, then discard two cards.
//
// Implementation:
//   - Haste (self) is AST-side.
//   - "Vehicles you control have haste" §613 static anthem (R60 stub
//     sweep batch 3): Abzan-Falconer-style flag sweep on ETB and
//     permanent_etb stamps Flags["kw:haste"] on every Vehicle the
//     controller owns. permanent_ltb on Fearless Swashbuckler strips
//     only flags we granted (separate marker so native AST haste on
//     Vehicles isn't clobbered when Swashbuckler leaves).
//   - "Whenever you attack" — fires once per combat per controller, not
//     once per attacker. We use the "creature_attacks" trigger as a
//     proxy and dedup via a per-turn flag so multi-attacker combats only
//     trigger once. Gate on attacker_perm.Controller == self.Controller
//     so opponents' attacks don't satisfy the "you attack" clause.
//   - Condition: among attackers this combat, at least one Pirate AND
//     at least one Vehicle. Vehicle includes both creature-Vehicles
//     (crewed) and non-creature Vehicles attacking as creatures.
//   - Effect: draw 3, then discard 2. Discard policy: drop the two
//     highest-CMC lands if any, else the two highest-CMC cards.
func registerFearlessSwashbuckler(r *Registry) {
	r.OnETB("Fearless Swashbuckler", fearlessSwashbucklerETB)
	r.OnTrigger("Fearless Swashbuckler", "permanent_etb", fearlessSwashbucklerPermETB)
	r.OnTrigger("Fearless Swashbuckler", "permanent_ltb", fearlessSwashbucklerOwnLTB)
	r.OnTrigger("Fearless Swashbuckler", "creature_attacks", fearlessSwashbucklerOnAttack)
}

func fearlessSwashbucklerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	fearlessSwashbucklerHasteSweep(gs, perm)
}

func fearlessSwashbucklerPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	fearlessSwashbucklerHasteSweep(gs, perm)
}

func fearlessSwashbucklerOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fearless_swashbuckler_haste_strip"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stripped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		if p.Flags["fearless_swashbuckler_haste_grant"] != 1 {
			continue
		}
		delete(p.Flags, "kw:haste")
		delete(p.Flags, "fearless_swashbuckler_haste_grant")
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"stripped": stripped,
	})
}

func fearlessSwashbucklerHasteSweep(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "fearless_swashbuckler_vehicles_haste"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	granted := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "vehicle") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["fearless_swashbuckler_haste_grant"] == 1 {
			continue
		}
		if p.HasKeyword("haste") {
			continue
		}
		p.Flags["kw:haste"] = 1
		p.Flags["fearless_swashbuckler_haste_grant"] = 1
		granted++
	}
	if granted > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"granted": granted,
		})
	}
}

func fearlessSwashbucklerOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fearless_swashbuckler_attack_loot"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Controller != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Dedup per combat: first attacker resolves the trigger; later
	// attackers in the same combat short-circuit on the flag.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	flagKey := fearlessSwashbucklerFlagKey(gs.Turn)
	if seat.Flags[flagKey] != 0 {
		return
	}

	hasPirate, hasVehicle := false, false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		_, attacking := gameengine.AttackerDefender(p)
		if !attacking {
			continue
		}
		if cardHasType(p.Card, "pirate") {
			hasPirate = true
		}
		if cardHasType(p.Card, "vehicle") {
			hasVehicle = true
		}
	}
	if !(hasPirate && hasVehicle) {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"pirate":    hasPirate,
			"vehicle":   hasVehicle,
		})
		return
	}
	seat.Flags[flagKey] = 1
	// Draw three, then discard two (printed order matters for triggers).
	for i := 0; i < 3; i++ {
		drawOne(gs, perm.Controller, perm.Card.DisplayName())
	}
	discarded := fearlessSwashbucklerDiscardTwo(gs, perm.Controller, slug)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"drew":      3,
		"discarded": discarded,
	})
}

func fearlessSwashbucklerFlagKey(turn int) string {
	// One key per turn — close enough; "this combat" granularity would
	// also need a combat-counter, which the engine doesn't surface.
	return "fearless_swashbuckler_fired_t" + itoaPerCard(turn)
}

// fearlessSwashbucklerDiscardTwo picks two cards from hand: the two
// highest-CMC lands if present, else two highest-CMC non-lands.
func fearlessSwashbucklerDiscardTwo(gs *gameengine.GameState, seatIdx int, slug string) []string {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	picks := []*gameengine.Card{}
	// Lands first.
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			picks = append(picks, c)
			if len(picks) == 2 {
				break
			}
		}
	}
	if len(picks) < 2 {
		// Top up with highest-CMC non-lands.
		type cand struct {
			c   *gameengine.Card
			cmc int
		}
		pool := []cand{}
		taken := map[*gameengine.Card]bool{}
		for _, c := range picks {
			taken[c] = true
		}
		for _, c := range seat.Hand {
			if c == nil || taken[c] {
				continue
			}
			pool = append(pool, cand{c, cardCMC(c)})
		}
		for len(picks) < 2 && len(pool) > 0 {
			bi := 0
			for i := 1; i < len(pool); i++ {
				if pool[i].cmc > pool[bi].cmc {
					bi = i
				}
			}
			picks = append(picks, pool[bi].c)
			pool = append(pool[:bi], pool[bi+1:]...)
		}
	}
	names := []string{}
	for _, c := range picks {
		// Route through the DiscardCard chokepoint (NOT a raw hand→graveyard
		// MoveCard) so the discard fires §702.34 Madness, card_discarded
		// observer triggers, and discard-count stats — a loot's "discard" is a
		// real discard, not a silent zone move.
		gameengine.DiscardCard(gs, c, seatIdx)
		names = append(names, c.DisplayName())
	}
	return names
}

// itoaPerCard is a tiny local int→string formatter so handler files
// don't reach into "strconv" or "fmt" unnecessarily for a single int.
func itoaPerCard(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
