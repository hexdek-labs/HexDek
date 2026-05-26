package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// outlaw_etb_consumers_r60.go — per_card handlers for the OTJ Outlaw
// type-group payoff family (CR §205.3m, the Assassin / Mercenary /
// Pirate / Rogue / Warlock supergroup).
//
// Per the Versailles Phase 1B audit (PR #477 §3) `outlaw_etb` is one
// of 29 engine-emitted events with no per_card consumer wired. The
// engine has the full plumbing — keywords_outlaw.go's
// FireOutlawETBTriggers fan-out and HasOutlawTrigger detector — but
// no per_card handler subscribed to the payoff, so every "whenever an
// outlaw enters" card and adjacent outlaw-payoff triggers in the OTJ
// corpus were inert in production.
//
// This file wires 6 handlers across four shapes:
//
//   (A) `outlaw_etb` listener — strict consumer of the engine event:
//        - Vial Smasher, Gleeful Grenadier  (another outlaw enters → 1 dmg)
//
//   (B) Self-ETB conditional on outlaw count — fires from
//       `permanent_etb` and reads CountOutlawsControlled:
//        - Mine Raider              (Treasure if you control another outlaw)
//        - Hellspur Posse Boss      (2 Mercenary tokens — unconditional)
//        - Charred Graverobber      (return target outlaw from GY)
//        - Rakish Crew (half A)     (1 Mercenary token)
//
//   (C) Outlaw-dies observer — fires from `permanent_sacrificed` AND
//       creature_dies (the engine emits both):
//        - Rakish Crew (half B)     (drain 1 when an outlaw of yours dies)
//
//   (D) Outlaw-spell-cast observer — fires from `spell_cast` and
//       filters on (a) controller match and (b) the spell being an
//       outlaw; first-outlaw-cast-each-turn semantics via a turn-
//       stamped flag:
//        - Discreet Retreat         (draw a card, lose 1 life)

func init() {
	registerOutlawETBConsumersR60(Global())
	AddResetHook(registerOutlawETBConsumersR60)
}

func registerOutlawETBConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	// (A)
	r.OnTrigger("Vial Smasher, Gleeful Grenadier", "outlaw_etb", vialSmasherOutlawETB)
	// (B) — self-ETB; the dispatcher walks the battlefield and only the
	// just-entered bearer matches via `enteringPerm == perm`.
	r.OnTrigger("Mine Raider", "permanent_etb", mineRaiderETB)
	r.OnTrigger("Hellspur Posse Boss", "permanent_etb", hellspurPosseBossETB)
	r.OnTrigger("Charred Graverobber", "permanent_etb", charredGraverobberETB)
	r.OnTrigger("Rakish Crew", "permanent_etb", rakishCrewETB)
	// (C) — dies listener.
	r.OnTrigger("Rakish Crew", "creature_sacrificed", rakishCrewOutlawDies)
	r.OnTrigger("Rakish Crew", "creature_dies", rakishCrewOutlawDies)
	// (D) — outlaw-spell-cast observer (first per turn).
	r.OnTrigger("Discreet Retreat", "spell_cast", discreetRetreatOutlawCast)
}

// -----------------------------------------------------------------------------
// Vial Smasher, Gleeful Grenadier — {B}{R}
//
// Whenever another outlaw you control enters, Vial Smasher deals 1
// damage to target opponent.
//
// outlaw_etb ctx (keywords_outlaw.go:225): perm, card, card_name,
// controller_seat, watcher, watcher_seat. The fan-out fires once per
// (entering outlaw, watcher) pair, so when Vial Smasher is the
// watcher, this handler fires with watcher == perm. The "another"
// filter is the handler's job: skip self-events.
// -----------------------------------------------------------------------------

func vialSmasherOutlawETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vial_smasher_outlaw_etb_ping"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	watcher, _ := ctx["watcher"].(*gameengine.Permanent)
	if watcher != perm {
		// The dispatcher hit a different Vial Smasher than the one
		// FireOutlawETBTriggers chose as the watcher. Only the matched
		// one runs.
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm == perm {
		// "another outlaw" — skip self-events.
		return
	}
	if enteringPerm.Controller != perm.Controller {
		// "outlaw YOU control" — skip opp outlaws.
		return
	}
	// Pick the highest-life living opponent (mirrors "any target"
	// damage convention of pinging the threat with the most runway).
	var pick int = -1
	bestLife := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			pick = opp
		}
	}
	if pick < 0 {
		return
	}
	gameengine.DealDamage(gs, pick, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   pick,
		"entering": enteringPerm.Card.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Mine Raider — {2}{R} Creature — Human Rogue
//
// When this creature enters, if you control ANOTHER outlaw, create a
// Treasure token.
// -----------------------------------------------------------------------------

func mineRaiderETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mine_raider_treasure"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering != perm {
		return
	}
	// "another outlaw" — Mine Raider IS an outlaw (Rogue), so the
	// count must be ≥ 2 for there to be ANOTHER one.
	if gameengine.CountOutlawsControlled(gs, perm.Controller) < 2 {
		return
	}
	gameengine.CreateTreasureToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{"seat": perm.Controller})
}

// -----------------------------------------------------------------------------
// Hellspur Posse Boss — {2}{R}{R} Creature — Lizard Rogue
//
// Other outlaws you control have haste. (Static, AST-handled.)
// When this creature enters, create two 1/1 red Mercenary creature
// tokens with the tap-pump-target ability (we mint the token; the
// tap ability is wired by the AST keyword pipeline).
// -----------------------------------------------------------------------------

func hellspurPosseBossETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hellspur_posse_boss_mercenaries"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering != perm {
		return
	}
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Mercenary",
			[]string{"creature", "mercenary", "pip:R"}, 1, 1)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{"seat": perm.Controller})
}

// -----------------------------------------------------------------------------
// Charred Graverobber — {2}{B} Creature — Skeleton Mercenary
//
// When this creature enters, return target outlaw card from your
// graveyard to your hand.
//
// Target picker: highest-CMC outlaw card in the controller's
// graveyard. Note: Charred Graverobber itself can't be the target
// because it just entered the battlefield (not in the graveyard).
// -----------------------------------------------------------------------------

func charredGraverobberETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "charred_graverobber_recur_outlaw"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering != perm {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	var target *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !gameengine.IsOutlaw(c) {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			target = c
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_outlaw_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.MoveCard(gs, target, perm.Controller, "graveyard", "hand",
		"charred_graverobber_return")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Rakish Crew — {2}{B} Enchantment
//
// When this enchantment enters, create a 1/1 red Mercenary token.
// Whenever an outlaw you control dies, each opponent loses 1 life
// and you gain 1 life.
// -----------------------------------------------------------------------------

func rakishCrewETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rakish_crew_mercenary_token"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering != perm {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Mercenary",
		[]string{"creature", "mercenary", "pip:R"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{"seat": perm.Controller})
}

func rakishCrewOutlawDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rakish_crew_outlaw_dies_drain"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if dyingPerm == nil || dyingPerm.Card == nil {
		return
	}
	// "outlaw you control" — must have been controlled by Rakish Crew's
	// controller AND must be an outlaw.
	deathSeat, _ := ctx["controller_seat"].(int)
	if deathSeat != perm.Controller {
		return
	}
	if !gameengine.IsOutlaw(dyingPerm.Card) {
		return
	}
	// Idempotency: the engine fires both `creature_dies` and
	// `creature_sacrificed` for the same outlaw on the same event when
	// the death was a sacrifice. Stamp a transient flag on the dying
	// perm's Flags so the second fire is a no-op. The flag is set per-
	// perm and the perm leaves play, so cleanup is implicit.
	if dyingPerm.Flags == nil {
		dyingPerm.Flags = map[string]int{}
	}
	if dyingPerm.Flags["rakish_crew_drained"] == 1 {
		return
	}
	dyingPerm.Flags["rakish_crew_drained"] = 1

	hit := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 1, perm.Card.DisplayName())
		hit++
	}
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"opps_hit": hit,
		"dying":    dyingPerm.Card.DisplayName(),
	})
}

// -----------------------------------------------------------------------------
// Discreet Retreat — {3}{B} Enchantment — Aura
//
// Enchant land
// Enchanted land has "{T}: Add two mana of any one color. Spend this
// mana only to cast outlaw spells or activate abilities of outlaw
// sources."
// Whenever you cast your first outlaw spell EACH TURN, you draw a
// card and you lose 1 life.
//
// This wires the cast-trigger half; the granted mana ability on the
// enchanted land is a continuous static handled by the AST keyword
// pipeline. Gates: (a) cast was the controller's, (b) the spell is
// an outlaw, (c) this is the first outlaw cast this turn (turn-
// stamped flag on the Aura perm).
// -----------------------------------------------------------------------------

func discreetRetreatOutlawCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "discreet_retreat_first_outlaw_cast"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !gameengine.IsOutlaw(card) {
		return
	}
	// First outlaw cast each turn — gate via Turn+1 stamped flag.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["discreet_retreat_fired_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["discreet_retreat_fired_turn"] = gs.Turn + 1

	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	gameengine.LoseLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
	})
}
