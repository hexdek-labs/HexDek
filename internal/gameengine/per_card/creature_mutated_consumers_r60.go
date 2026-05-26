package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// creature_mutated_consumers_r60.go — per_card OnTrigger handlers for
// the Ikoria (IKO) Mutate keyword family (CR §702.139).
//
// Per the Versailles Phase 1B audit (PR #477 §3) `creature_mutated`
// is one of 29 engine-emitted events with no per_card consumer wired.
// The engine has the full mutate plumbing — keywords_batch6.go's
// ApplyMutate merges the cards, marks the surviving permanent with
// Flags["mutated"] = 1, and fires FireCardTrigger("creature_mutated",
// ctx) — but no per_card handler subscribed to the payoff, so every
// "Whenever this creature mutates, ..." rider in the IKO corpus was
// dead text in production.
//
// Companion engine tweak in keywords_batch6.go: the FireCardTrigger
// call now passes "mutated_perm" — the surviving permanent — so
// per-card handlers can gate on "did I just mutate" rather than
// firing on every controller-level mutation event (a Mutate deck
// chains multiple mutations per turn).

func init() {
	registerCreatureMutatedConsumersR60(Global())
	AddResetHook(registerCreatureMutatedConsumersR60)
}

func registerCreatureMutatedConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Dreamtail Heron", "creature_mutated", dreamtailHeronMutated)
	r.OnTrigger("Cubwarden", "creature_mutated", cubwardenMutated)
	r.OnTrigger("Majestic Auricorn", "creature_mutated", majesticAuricornMutated)
	r.OnTrigger("Cavern Whisperer", "creature_mutated", cavernWhispererMutated)
	r.OnTrigger("Glowstone Recluse", "creature_mutated", glowstoneRecluseMutated)
	r.OnTrigger("Gemrazer", "creature_mutated", gemrazerMutated)
}

// mutatedGuard returns the surviving merged perm + its controller
// when the dispatch matches `perm` (the bearer just mutated). Returns
// nil + -1 when the ctx is malformed OR another creature mutated and
// this handler should abstain.
//
// The engine's per-mutation fan-out walks the battlefield by name; if
// Brokkos and a sibling Brokkos are both on the battlefield and a
// Cubwarden mutates somewhere else, every bearer-named handler is
// invoked. Only the one whose `perm` IS the mergedPerm should run the
// payoff.
func mutatedGuard(perm *gameengine.Permanent, ctx map[string]interface{}) (*gameengine.Permanent, int) {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil, -1
	}
	merged, _ := ctx["mutated_perm"].(*gameengine.Permanent)
	if merged != perm {
		return nil, -1
	}
	seat, ok := ctx["controller_seat"].(int)
	if !ok {
		seat = perm.Controller
	}
	if seat < 0 || seat >= len(perm_seat_cap()) {
		return nil, -1
	}
	return perm, seat
}

// perm_seat_cap is a fixed-cap shim used by mutatedGuard's seat-index
// bounds check (Commander pods top out at 6; 16 leaves plenty of
// headroom and avoids taking *GameState as a parameter).
func perm_seat_cap() []struct{} { return make([]struct{}, 16) }

// -----------------------------------------------------------------------------
// Dreamtail Heron — {4}{U} Creature — Elemental Bird, Mutate {3}{U}
//
// Flying. Whenever this creature mutates, draw a card.
// -----------------------------------------------------------------------------

func dreamtailHeronMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dreamtail_heron_draw"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
		return
	}
	drawOne(gs, seat, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{"seat": seat})
}

// -----------------------------------------------------------------------------
// Cubwarden — {3}{W} Creature — Cat, Mutate {2}{W}{W}
//
// Lifelink. Whenever this creature mutates, create two 1/1 white Cat
// creature tokens with lifelink.
// -----------------------------------------------------------------------------

func cubwardenMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cubwarden_cat_tokens"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
		return
	}
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, seat, "Cat",
			[]string{"creature", "cat", "pip:W", "kw:lifelink"}, 1, 1)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{"seat": seat})
}

// -----------------------------------------------------------------------------
// Majestic Auricorn — {4}{W} Creature — Unicorn, Mutate {3}{W}
//
// Vigilance. Whenever this creature mutates, you gain 4 life.
// -----------------------------------------------------------------------------

func majesticAuricornMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "majestic_auricorn_lifegain"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.GainLife(gs, seat, 4, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{"seat": seat})
}

// -----------------------------------------------------------------------------
// Cavern Whisperer — {4}{B} Creature — Nightmare, Mutate {3}{B}
//
// Menace. Whenever this creature mutates, each opponent discards a card.
//
// "Discards a card" — opponent chooses. Without a per-player Choose
// prompt, we approximate by discarding the FIRST card in each living
// opponent's hand. Same fallback used by commander_staples.go's
// existing forced-discard handlers.
// -----------------------------------------------------------------------------

func cavernWhispererMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cavern_whisperer_each_opp_discards"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
		return
	}
	hit := 0
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost || len(s.Hand) == 0 {
			continue
		}
		gameengine.DiscardCard(gs, s.Hand[0], opp)
		hit++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"opps_hit": hit,
	})
}

// -----------------------------------------------------------------------------
// Glowstone Recluse — {2}{G} Creature — Spider, Mutate {3}{G}
//
// Reach. Whenever this creature mutates, put two +1/+1 counters on it.
// -----------------------------------------------------------------------------

func glowstoneRecluseMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glowstone_recluse_counters"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
		return
	}
	src.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"counters": src.Counters["+1/+1"],
	})
}

// -----------------------------------------------------------------------------
// Gemrazer — {3}{G} Creature — Beast, Mutate {1}{G}{G}
//
// Reach, trample. Whenever this creature mutates, destroy target
// artifact or enchantment an opponent controls.
//
// Target picker: highest-CMC opp artifact-or-enchantment (snowball
// removes the biggest threat). Mirrors Aura Shards's pick policy from
// the eerie / aura families.
// -----------------------------------------------------------------------------

func gemrazerMutated(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gemrazer_destroy_artifact_or_enchantment"
	src, seat := mutatedGuard(perm, ctx)
	if src == nil {
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
			if !p.IsArtifact() && !p.IsEnchantment() {
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
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_artifact_or_enchantment", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	gameengine.DestroyPermanent(gs, best, src)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"destroyed": best.Card.DisplayName(),
	})
}
