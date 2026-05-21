package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSheoldredTrueScriptures wires Sheoldred // The True Scriptures.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
// Front — Sheoldred ({3}{B}{B}, Phyrexian Praetor, 4/5):
//
//	Menace
//	When Sheoldred enters, each opponent sacrifices a nontoken
//	creature or planeswalker of their choice.
//	{4}{B}: Exile Sheoldred, then return it to the battlefield
//	transformed under its owner's control. Activate only as a sorcery
//	and only if an opponent has eight or more cards in their
//	graveyard.
//
// Back — The True Scriptures (Enchantment — Saga):
//
//	I — For each opponent, destroy up to one target creature or
//	planeswalker that player controls.
//	II — Each opponent discards three cards, then mills three cards.
//	III — Put all creature cards from all graveyards onto the
//	battlefield under your control. Exile this Saga, then return it
//	to the battlefield (front face up).
//
// Implementation:
//   - ETB front: each opponent sacs their lowest-CMC nontoken creature
//     or planeswalker.
//   - Activated ability: gate on opponent having 8+ in graveyard, then
//     transform via TransformPermanent.
//   - Saga lore counter triggers (chapter I/II/III): handled via
//     "lore_counter_added" trigger.
//   - Chapter III "exile then return front face up" is partial — we
//     transform back rather than exile/return.
func registerSheoldredTrueScriptures(r *Registry) {
	r.OnETB("Sheoldred // The True Scriptures", sheoldredTSEtb)
	r.OnETB("Sheoldred", sheoldredTSEtb)
	r.OnActivated("Sheoldred // The True Scriptures", sheoldredTSActivate)
	r.OnActivated("Sheoldred", sheoldredTSActivate)
	r.OnTrigger("Sheoldred // The True Scriptures", "lore_counter_added", sheoldredTSLore)
	r.OnTrigger("The True Scriptures", "lore_counter_added", sheoldredTSLore)
}

func sheoldredTSEtb(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sheoldred_true_scriptures_etb_sac"
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		return
	}
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		var pick *gameengine.Permanent
		low := 1 << 30
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "token") {
				continue
			}
			if !p.IsCreature() && !p.IsPlaneswalker() {
				continue
			}
			cm := cardCMC(p.Card)
			if cm < low {
				low = cm
				pick = p
			}
		}
		if pick != nil {
			gameengine.SacrificePermanent(gs, pick, "sheoldred_true_scriptures_etb")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func sheoldredTSActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sheoldred_true_scriptures_transform_activate"
	if gs == nil || src == nil {
		return
	}
	if src.Transformed {
		return
	}
	hasEight := false
	for _, oppIdx := range gs.Opponents(src.Controller) {
		s := gs.Seats[oppIdx]
		if s != nil && len(s.Graveyard) >= 8 {
			hasEight = true
			break
		}
	}
	if !hasEight {
		emitFail(gs, slug, src.Card.DisplayName(), "no_opponent_with_eight_graveyard", nil)
		return
	}
	if !gameengine.TransformPermanent(gs, src, "sheoldred_true_scriptures_activate") {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
		"to":   "The True Scriptures",
	})
}

func sheoldredTSLore(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sheoldred_true_scriptures_chapter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if !perm.Transformed {
		return
	}
	chapter, _ := ctx["chapter"].(int)
	switch chapter {
	case 1:
		// Each opponent: destroy up to one target creature or planeswalker.
		destroyed := 0
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			var best *gameengine.Permanent
			bestCMC := -1
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if !p.IsCreature() && !p.IsPlaneswalker() {
					continue
				}
				cm := cardCMC(p.Card)
				if cm > bestCMC {
					bestCMC = cm
					best = p
				}
			}
			if best != nil {
				gameengine.SacrificePermanent(gs, best, "scriptures_chapter_1")
				destroyed++
			}
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"chapter":   1,
			"destroyed": destroyed,
		})
	case 2:
		// Each opponent discards 3, then mills 3.
		for _, oppIdx := range gs.Opponents(perm.Controller) {
			s := gs.Seats[oppIdx]
			if s == nil || s.Lost {
				continue
			}
			// Discard 3 (highest-CMC first).
			for i := 0; i < 3 && len(s.Hand) > 0; i++ {
				idx := 0
				high := -1
				for j, c := range s.Hand {
					if c == nil {
						continue
					}
					if cardCMC(c) > high {
						high = cardCMC(c)
						idx = j
					}
				}
				c := s.Hand[idx]
				gameengine.MoveCard(gs, c, oppIdx, "hand", "graveyard", "scriptures_discard")
			}
			for i := 0; i < 3 && len(s.Library) > 0; i++ {
				top := s.Library[0]
				gameengine.MoveCard(gs, top, oppIdx, "library", "graveyard", "scriptures_mill")
			}
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"chapter": 2,
		})
	case 3:
		// Reanimate all creatures from all graveyards under your control.
		count := 0
		for i, s := range gs.Seats {
			if s == nil {
				continue
			}
			creatures := []*gameengine.Card{}
			for _, c := range s.Graveyard {
				if c != nil && cardHasType(c, "creature") {
					creatures = append(creatures, c)
				}
			}
			for _, c := range creatures {
				gameengine.MoveCard(gs, c, i, "graveyard", "battlefield", "scriptures_chapter_3")
				createPermanent(gs, perm.Controller, c, false)
				count++
			}
		}
		// R52 batchM: chapter III also flips Scriptures back to front.
		// The printed text is "exile this Saga, then return it to the
		// battlefield (front face up)"; TransformPermanent is the
		// mechanical equivalent in this engine.
		if perm.Transformed {
			gameengine.TransformPermanent(gs, perm, "scriptures_chapter_iii_flip_front")
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"chapter":      3,
			"reanimated":   count,
			"flipped_front": true,
		})
	}
}
