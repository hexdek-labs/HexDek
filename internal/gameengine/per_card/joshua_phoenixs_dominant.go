package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJoshuaPhoenixsDominant wires Joshua, Phoenix's Dominant //
// Phoenix, Warden of Fire.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	  Joshua (front, Human Noble Wizard):
//		When Joshua enters, discard up to two cards, then draw that many
//		  cards.
//		{3}{R}{W}, {T}: Exile Joshua, then return it to the battlefield
//		  transformed under its owner's control. Activate only as a
//		  sorcery.
//
//	  Phoenix, Warden of Fire (back, Saga Phoenix):
//		(As this Saga enters and after your draw step, add a lore counter.)
//		I, II — Rising Flames — Phoenix deals 2 damage to each opponent.
//		III — Flames of Rebirth — Return any number of target creature
//		  cards with total mana value 6 or less from your graveyard to the
//		  battlefield. Exile Phoenix, then return it to the battlefield
//		  (front face up).
//		Flying, lifelink
//
// Implementation:
//   - OnETB (Joshua front): discard up to 2 highest-CMC cards in hand,
//     draw that many. Skip discard if hand is empty.
//   - Activated transform and Saga back-face mechanics are continuous
//     mechanics outside per-card scope — emitPartial.
func registerJoshuaPhoenixsDominant(r *Registry) {
	r.OnETB("Joshua, Phoenix's Dominant", joshuaPhoenixsETB)
	r.OnETB("Joshua, Phoenix's Dominant // Phoenix, Warden of Fire", joshuaPhoenixsETB)
	r.OnActivated("Joshua, Phoenix's Dominant", joshuaPhoenixsActivate)
	r.OnActivated("Joshua, Phoenix's Dominant // Phoenix, Warden of Fire", joshuaPhoenixsActivate)
	// R52 batchM: Saga back-face chapter dispatch via lore_counter_added.
	// Chapters I and II both fire the "Rising Flames — 2 damage to each
	// opponent" payload (one per lore tick). Chapter III reanimates
	// creature cards with total mana value ≤ 6 from the controller's
	// graveyard, then flips front face up.
	r.OnTrigger("Joshua, Phoenix's Dominant", "lore_counter_added", joshuaPhoenixsSagaChapter)
	r.OnTrigger("Joshua, Phoenix's Dominant // Phoenix, Warden of Fire", "lore_counter_added", joshuaPhoenixsSagaChapter)
	r.OnTrigger("Phoenix, Warden of Fire", "lore_counter_added", joshuaPhoenixsSagaChapter)
}

func joshuaPhoenixsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "joshua_phoenixs_etb_loot"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	maxDiscard := 2
	if len(s.Hand) < maxDiscard {
		maxDiscard = len(s.Hand)
	}
	discarded := []string{}
	for i := 0; i < maxDiscard; i++ {
		// Highest-CMC remaining card.
		idx := -1
		bestCMC := -1
		for j, c := range s.Hand {
			if c == nil {
				continue
			}
			if cm := gameengine.ManaCostOf(c); cm > bestCMC {
				bestCMC = cm
				idx = j
			}
		}
		if idx < 0 {
			break
		}
		card := s.Hand[idx]
		discarded = append(discarded, card.DisplayName())
		gameengine.MoveCard(gs, card, seat, "hand", "graveyard", "joshua_etb")
		gs.LogEvent(gameengine.Event{
			Kind:   "discard",
			Seat:   seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug": slug,
				"card": card.DisplayName(),
			},
		})
	}
	drawn := 0
	for i := 0; i < len(discarded); i++ {
		if drawOne(gs, seat, perm.Card.DisplayName()) != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"discarded": discarded,
		"drawn":     drawn,
	})
}

func joshuaPhoenixsActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "joshua_phoenixs_transform"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	if src.Transformed {
		emitFail(gs, slug, src.Card.DisplayName(), "already_back_face", nil)
		return
	}
	// {3}{R}{W}, {T}: 5 generic-equivalent.
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.ManaPool < 5 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", nil)
		return
	}
	seat.ManaPool -= 5
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	gameengine.TransformPermanent(gs, src, "joshua_phoenixs_trance")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// joshuaPhoenixsSagaChapter handles the Phoenix, Warden of Fire back
// face. Lore counters tick to chapters I, II, III and the printed text
// gives I+II the same effect ("Rising Flames — Phoenix deals 2 damage
// to each opponent") and III the reanimate-then-flip payload.
func joshuaPhoenixsSagaChapter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "joshua_phoenixs_saga_chapter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Saga effect only fires when Joshua is in his Saga back-face form.
	if !perm.Transformed {
		return
	}
	chapter, _ := ctx["chapter"].(int)
	switch chapter {
	case 1, 2:
		hits := 0
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			gameengine.DealDamage(gs, oppIdx, 2, perm.Card.DisplayName())
			hits++
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"chapter":      chapter,
			"opponents_hit": hits,
		})
		_ = gs.CheckEnd()
	case 3:
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		// Greedy: highest-CMC creatures first, stop once total > 6.
		type pick struct {
			c   *gameengine.Card
			cmc int
		}
		var candidates []pick
		for _, c := range seat.Graveyard {
			if c == nil || !cardHasType(c, "creature") {
				continue
			}
			candidates = append(candidates, pick{c, cardCMC(c)})
		}
		// Sort by CMC desc to fit as many big creatures as possible.
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				if candidates[j].cmc > candidates[i].cmc {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}
		total := 0
		returned := 0
		for _, p := range candidates {
			if total+p.cmc > 6 {
				continue
			}
			gameengine.MoveCard(gs, p.c, perm.Controller, "graveyard", "battlefield", "joshua_phoenixs_chapter_iii")
			total += p.cmc
			returned++
		}
		// Flip back to Joshua, front face up.
		gameengine.TransformPermanent(gs, perm, "joshua_phoenixs_chapter_iii_flip_front")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":           perm.Controller,
			"chapter":        chapter,
			"reanimated":     returned,
			"total_mv":       total,
		})
	}
}
