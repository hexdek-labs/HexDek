package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// class_level_up_consumers_r60.go — per_card OnTrigger handlers for
// the AFR / DSK / OTC Class enchantment family (CR §716, ability
// "level up").
//
// Per the Versailles Phase 1B audit (PR #477 §3) `class_level_up` is
// an engine-emitted event with no per_card consumer wired. The engine
// has the full plumbing — keywords_class.go's ActivateLevelUp pays
// the cost, increments `perm.Flags["class_level"]`, logs
// `class_level_up`, and fires FireCardTrigger("class_level_up", ctx)
// with `source` / `controller_seat` / `previous_level` / `new_level`
// — but no per_card handler consumed the trigger, so the
// "When this Class becomes level N" payoffs in the corpus were inert.
//
// This file wires 6 handlers covering one payoff per Class:
//
//   - Wizard Class           — becomes level 2 → draw 2
//   - Warlock Class          — becomes level 2 → top 3 → 1 to hand,
//                              rest to graveyard
//   - Artificer Class        — becomes level 2 → reveal until artifact
//   - Monk Class             — becomes level 2 → return target nonland
//                              permanent to its owner's hand
//   - Stormchaser's Talent   — becomes level 2 → return target i/s
//                              from your graveyard to your hand
//   - Builder's Talent       — becomes level 3 → return target
//                              noncreature/nonland permanent card from
//                              your graveyard to the battlefield
//
// All handlers gate on (a) `source == perm` (the dispatcher walks
// every battlefield permanent by name; only the leveling-up Class
// should fire) and (b) `new_level == expected_level` so the wrong-
// level trigger doesn't double-fire.

func init() {
	registerClassLevelUpConsumersR60(Global())
	AddResetHook(registerClassLevelUpConsumersR60)
}

func registerClassLevelUpConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Wizard Class", "class_level_up", wizardClassLevelUp)
	r.OnTrigger("Warlock Class", "class_level_up", warlockClassLevelUp)
	r.OnTrigger("Artificer Class", "class_level_up", artificerClassLevelUp)
	r.OnTrigger("Monk Class", "class_level_up", monkClassLevelUp)
	r.OnTrigger("Stormchaser's Talent", "class_level_up", stormchasersTalentLevelUp)
	r.OnTrigger("Builder's Talent", "class_level_up", buildersTalentLevelUp)
}

// classLevelGuard extracts the leveling-up Class perm + new level from
// ctx and verifies the dispatch matched the bearer. Returns (nil, -1,
// -1) when the ctx is malformed or the dispatcher fired the handler
// against an unrelated permanent.
func classLevelGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, int, int) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, -1, -1
	}
	src, _ := ctx["source"].(*gameengine.Permanent)
	if src != perm {
		// fireTrigger walks the battlefield matching by display name;
		// if two copies of the same Class exist, the dispatcher fires
		// the handler against each. Only the one that actually leveled
		// up should run the payoff.
		return nil, -1, -1
	}
	newLevel, _ := ctx["new_level"].(int)
	seat := src.Controller
	if seat < 0 || seat >= len(gs_class_seats_cap()) {
		return nil, -1, -1
	}
	return src, seat, newLevel
}

// gs_class_seats_cap returns a fixed seat-index cap for the bounds
// check in classLevelGuard. Pods top out at 6; 16 gives plenty of
// headroom. Using a slice of zero length keeps len() cheap.
func gs_class_seats_cap() []struct{} { return make([]struct{}, 16) }

// -----------------------------------------------------------------------------
// Wizard Class — {U} Enchantment — Class
//
//	You have no maximum hand size.
//	{2}{U}: Level 2
//	When this Class becomes level 2, draw two cards.
//	{4}{U}: Level 3
//	Whenever you draw a card, put a +1/+1 counter on target creature you
//	control.
//
// MVP wires the level-2 draw; the level-3 draw-trigger is a separate
// continuous static (whenever-you-draw) and is deliberately out of
// scope here.
// -----------------------------------------------------------------------------

func wizardClassLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wizard_class_level2_draw_two"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 2 {
		return
	}
	drawOne(gs, seat, src.Card.DisplayName())
	drawOne(gs, seat, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"level": newLevel,
	})
}

// -----------------------------------------------------------------------------
// Warlock Class — {B} Enchantment — Class
//
//	At the beginning of your end step, if a creature died this turn,
//	each opponent loses 1 life.
//	{1}{B}: Level 2
//	When this Class becomes level 2, look at the top three cards of
//	your library. Put one of them into your hand and the rest into
//	your graveyard.
//	...
//
// Wires the level-2 "top 3 → 1 to hand, rest to graveyard." Hat
// choice is deferred: take the FIRST card (deterministic and matches
// the engine's existing "no hat" fallback for related dig effects);
// later wiring through ChooseDigOne can refine.
// -----------------------------------------------------------------------------

func warlockClassLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "warlock_class_level2_dig3"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 2 {
		return
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "empty_library", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	// Move top card to hand, next up-to-2 cards to graveyard.
	top := s.Library[0]
	gameengine.MoveCard(gs, top, seat, "library", "hand", "warlock_class_pick")
	milled := 0
	for i := 0; i < 2 && len(s.Library) > 0; i++ {
		next := s.Library[0]
		gameengine.MoveCard(gs, next, seat, "library", "graveyard", "warlock_class_mill")
		milled++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"picked": top.DisplayName(),
		"milled": milled,
	})
}

// -----------------------------------------------------------------------------
// Artificer Class — {1}{U} Enchantment — Class
//
//	The first artifact spell you cast each turn costs {1} less to cast.
//	{1}{U}: Level 2
//	When this Class becomes level 2, reveal cards from the top of your
//	library until you reveal an artifact card. Put that card into your
//	hand and the rest on the bottom of your library in a random order.
//	...
// -----------------------------------------------------------------------------

func artificerClassLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "artificer_class_level2_reveal_artifact"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 2 {
		return
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "empty_library", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	// Walk the library top-to-bottom. The first artifact goes to hand;
	// every non-artifact passed-over goes to the bottom (the rules-text
	// "random order" is approximated by preserving scan order, which
	// the random shuffler in the engine will re-randomize on next draw
	// in any real game; for unit tests this is deterministic and clear).
	var revealed []*gameengine.Card
	var artifact *gameengine.Card
	for len(s.Library) > 0 {
		top := s.Library[0]
		s.Library = s.Library[1:]
		if top != nil && cardHasType(top, "artifact") {
			artifact = top
			break
		}
		revealed = append(revealed, top)
	}
	// Bottom the non-artifact reveals.
	s.Library = append(s.Library, revealed...)
	if artifact == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_in_library", map[string]interface{}{
			"seat":    seat,
			"revealed": len(revealed),
		})
		return
	}
	s.Hand = append(s.Hand, artifact)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"artifact": artifact.DisplayName(),
		"revealed": len(revealed),
	})
}

// -----------------------------------------------------------------------------
// Monk Class — {W}{U} Enchantment — Class
//
//	The second spell you cast each turn costs {1} less to cast.
//	{W}{U}: Level 2
//	When this Class becomes level 2, return up to one target nonland
//	permanent to its owner's hand.
//	...
//
// Target picker: the highest-CMC opponent-controlled nonland permanent
// (mirrors the rest of the corpus's "neutralize the biggest threat"
// default). "Up to one" — if no eligible target, do nothing without
// a hard fail.
// -----------------------------------------------------------------------------

func monkClassLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "monk_class_level2_bounce"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 2 {
		return
	}
	var best *gameengine.Permanent
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil {
			continue
		}
		for _, p := range os.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > bestCMC {
				bestCMC = cmc
				best = p
			}
		}
	}
	if best == nil {
		// "Up to one" — no eligible target is fine, not a failure.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":    seat,
			"bounced": false,
		})
		return
	}
	gameengine.BouncePermanent(gs, best, src, "hand")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"bounced": true,
		"target":  best.Card.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Stormchaser's Talent — {U} Enchantment — Class
//
//	When this Class enters, create a 1/1 blue and red Otter creature
//	token with prowess.
//	{3}{U}: Level 2
//	When this Class becomes level 2, return target instant or sorcery
//	card from your graveyard to your hand.
//	...
// -----------------------------------------------------------------------------

func stormchasersTalentLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "stormchasers_talent_level2_recur"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 2 {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(s.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_instant_or_sorcery_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	gameengine.MoveCard(gs, target, seat, "graveyard", "hand", "stormchasers_talent_return")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"target": target.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Builder's Talent — {1}{W} Enchantment — Class
//
//	When this Class enters, create a 0/4 white Wall creature token
//	with defender.
//	{W}: Level 2
//	Whenever one or more noncreature, nonland permanents you control
//	enter, put a +1/+1 counter on target creature you control.
//	{4}{W}: Level 3
//	When this Class becomes level 3, return target noncreature, nonland
//	permanent card from your graveyard to the battlefield.
//
// Wires the level-3 reanimation. Target picker: highest-CMC
// non-creature, non-land card in your graveyard.
// -----------------------------------------------------------------------------

func buildersTalentLevelUp(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "builders_talent_level3_reanimate"
	src, seat, newLevel := classLevelGuard(perm, ctx)
	if src == nil || newLevel != 3 {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	var target *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") || cardHasType(c, "land") {
			continue
		}
		// "Permanent" — must be one of artifact/enchantment/planeswalker/battle.
		if !cardHasType(c, "artifact") && !cardHasType(c, "enchantment") &&
			!cardHasType(c, "planeswalker") && !cardHasType(c, "battle") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			target = c
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_eligible_permanent_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	// Remove from graveyard, enter battlefield with full ETB cascade.
	newGY := s.Graveyard[:0]
	for _, c := range s.Graveyard {
		if c == target {
			continue
		}
		newGY = append(newGY, c)
	}
	s.Graveyard = newGY
	enterBattlefieldWithETB(gs, seat, target, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"target": target.DisplayName(),
	})
}
