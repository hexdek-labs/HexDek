package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZethiArcaneBlademaster wires Zethi, Arcane Blademaster.
//
// Oracle text:
//
//	Multikicker {W/U}
//	When Zethi, Arcane Blademaster enters, exile up to X target
//	instant cards from your graveyard, where X is the number of times
//	Zethi was kicked. Put a kick counter on each of them.
//	Whenever Zethi attacks, copy each exiled card you own with a kick
//	counter on it. You may cast the copies.
//
// Implementation:
//   - ETB: read kick count from perm.Flags["multikick_count"] (set by
//     the engine's kicker resolver when the cast supplied multikicker
//     payments). Exile that many top-power instants from the
//     controller's graveyard, marking each with a "kick" counter so
//     the attack trigger can find them later.
//   - creature_attacks: gate on attacker == Zethi. For each exiled
//     instant the controller owns with a kick counter, fire a copy
//     event. The actual "you may cast the copies" routing requires
//     the spell-copy + cast-without-paying pipeline that doesn't yet
//     exist for per_card hooks; we emitPartial for the cast step.
func registerZethiArcaneBlademaster(r *Registry) {
	r.OnETB("Zethi, Arcane Blademaster", zethiETB)
	r.OnTrigger("Zethi, Arcane Blademaster", "creature_attacks", zethiAttacks)
	// R52 batch L: LTB sweeps the gs.Flags["zethi_kicked_*"] reverse-
	// lookup keys. The keys are read by the attack trigger to find
	// kicked-counter-bearing exiled cards. Once Zethi leaves play the
	// attack trigger can't fire again, so the keys are orphaned dead
	// memory until a new Zethi enters. The kick counters themselves
	// live on the exiled cards (CR card-attached state) and are
	// preserved — only the reverse-lookup keys are dropped.
	r.OnTrigger("Zethi, Arcane Blademaster", "permanent_ltb", zethiLTBSweepLookup)
}

func zethiLTBSweepLookup(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// Bail if another Zethi remains anywhere in play (any seat).
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if normalizeName(p.Card.DisplayName()) == normalizeName("Zethi, Arcane Blademaster") {
				return
			}
		}
	}
	if gs.Flags == nil {
		return
	}
	const prefix = "zethi_kicked_"
	for k := range gs.Flags {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(gs.Flags, k)
		}
	}
}

func zethiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zethi_etb_exile_instants"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	kickCount := 0
	if perm.Flags != nil {
		kickCount = perm.Flags["multikick_count"]
	}
	if kickCount <= 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"multikick_count_unavailable_no_exiles_made")
		return
	}
	exiled := 0
	for _, c := range append([]*gameengine.Card(nil), seat.Graveyard...) {
		if exiled >= kickCount {
			break
		}
		if c == nil || !cardHasType(c, "instant") {
			continue
		}
		mover := gameengine.MoveCard(gs, c, perm.Controller, "graveyard", "exile", "zethi_etb_exile")
		if mover.FinalZone != "exile" {
			continue
		}
		// R52 batch K: tag the exiled Card directly via its Types
		// slice (canonical per-card side-channel for transient tags
		// — see Lara Croft's discovery_counter tag) instead of
		// keying gs.Flags by display name. Name-keying was fragile
		// for cards that share names across copies.
		hasTag := false
		for _, t := range c.Types {
			if t == "zethi_kick_counter" {
				hasTag = true
				break
			}
		}
		if !hasTag {
			c.Types = append(c.Types, "zethi_kick_counter")
		}
		exiled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"kicked":  kickCount,
		"exiled":  exiled,
	})
}

func zethiAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zethi_attack_copy_kicked"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	copies := 0
	for _, c := range seat.Exile {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if t == "zethi_kick_counter" {
				copies++
				break
			}
		}
	}
	if copies > 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"copy_count": copies,
		})
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"copy_and_cast_without_paying_pipeline_not_wired_for_per_card")
}
