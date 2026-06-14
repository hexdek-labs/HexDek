package gameengine

// Counter DB Phase 6 — gameengine-side bridge for the §122.1g doubling
// pipeline. Registers the new counter-doubler card family
// (Primal Vigor, Branching Evolution, Vorinclex Monstrous Raider's two
// arms, Ozolith the Shattered Spire, Conclave Mentor) onto gs.Replacements
// using the existing "would_put_counter" event type, and provides
// AddCountersToPermanent — the Phase 6 engine entry point that fires the
// replacement chain, then routes the post-§616 final count through
// counters.AddCounters so the CounterStack lineage stays InstanceID-aware.
//
// The earlier Phase 1-5 registrations (RegisterDoublingSeason +
// RegisterHardenedScales in replacement.go) gain ProvidesReplacements
// stamping here too — the Permanent.ProvidesReplacements audit surface
// landed in Phase 5 (instanceid_phase5.go) and the doubler family is the
// load-bearing population for it.

import (
	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// addProvidesReplacements appends a ReplacementSpec to perm.ProvidesReplacements
// idempotently — duplicate HandlerIDs are skipped. Used by every Counter DB
// Phase 6 doubler registration to populate the Phase 5 audit surface without
// risk of double-registration when a per_card handler re-runs on flicker.
func addProvidesReplacements(perm *Permanent, spec ReplacementSpec) {
	if perm == nil {
		return
	}
	for _, existing := range perm.ProvidesReplacements {
		if existing.HandlerID == spec.HandlerID {
			return
		}
	}
	perm.ProvidesReplacements = append(perm.ProvidesReplacements, spec)
}

// RegisterPrimalVigor — "If one or more +1/+1 counters would be placed on
// a creature, twice that many +1/+1 counters are placed on that creature
// instead." Plus the token-doubler arm: "If an effect would create one or
// more tokens, it creates twice that many of those tokens instead."
//
// Symmetric: applies to ALL creatures on the battlefield regardless of
// controller — distinct from Doubling Season (controller's permanents
// only) and Branching Evolution (controller's creatures only). Two
// Primal Vigors stack additively per §616 (each instance applies once;
// the second sees the doubled count from the first and doubles again).
func RegisterPrimalVigor(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	// Counter doubler — +1/+1 only, ANY creature.
	counterHandler := handlerKey("Primal Vigor", "plus1_dbl", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      counterHandler,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil || !ev.TargetPerm.IsCreature() {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Primal Vigor",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "plus1_doubler"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Primal Vigor",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: counterHandler,
	})

	// Token doubler — same shape as Doubling Season's token arm.
	tokenHandler := handlerKey("Primal Vigor", "token_dbl", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_create_token",
		HandlerID:      tokenHandler,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Primal Vigor",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "614", "effect": "token_doubler"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Primal Vigor",
		EventType: "would_create_token",
		Category:  CategoryOther,
		HandlerID: tokenHandler,
	})
}

// RegisterBranchingEvolution — "If one or more +1/+1 counters would be
// placed on a creature you control, twice that many +1/+1 counters are
// placed on it instead." Narrower than Primal Vigor (creatures-you-control
// only) and Doubling Season (which covers all counter types).
func RegisterBranchingEvolution(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	hid := handlerKey("Branching Evolution", "plus1_dbl", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      hid,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller != p.Controller {
				return false
			}
			if !ev.TargetPerm.IsCreature() {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Branching Evolution",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "plus1_doubler"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Branching Evolution",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: hid,
	})
}

// RegisterVorinclexMonstrousRaider — asymmetric two-arm doubler from
// Kaldheim. Spec callout names "Vorinclex Voice of Hunger" but the actual
// card with the asymmetric counter-doubling ability is Vorinclex,
// Monstrous Raider; Voice of Hunger is the Mirrodin Besieged snow-forest
// card.
//
// Oracle text:
//   - "If you would put one or more counters on a permanent or player, put
//     twice that many of those counters on that permanent or player instead."
//     (controller arm — symmetric Double for all counter types)
//   - "If an opponent would put one or more counters on a permanent or
//     player, that player puts half that many of those counters on that
//     permanent or player instead, rounded down." (opponent arm — Halve)
//
// Both arms register on "would_put_counter". The Applies gate distinguishes
// controller from opponent by comparing ev.TargetPerm.Controller to
// p.Controller — same shape as Doubling Season's gate but mirrored.
//
// The half-rounded-down arm is the §122.1g asymmetric carve-out — Vorinclex
// is the only canonical card requiring DoublerKindHalve in Phase 6.
func RegisterVorinclexMonstrousRaider(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	// Controller arm — doubles ALL counter types on controller's permanents
	// and on controller-as-player (poison received in a multiplayer match
	// going onto Vorinclex's controller would also double — but the §122.1g
	// rule sub-clause for player counters routes through a separate
	// codepath; Phase 6 wiring covers the permanent path).
	selfHandler := handlerKey("Vorinclex, Monstrous Raider", "self_dbl", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      selfHandler,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller != p.Controller {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Vorinclex, Monstrous Raider",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "self_doubler"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Vorinclex, Monstrous Raider",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: selfHandler,
	})

	// Opponent arm — halve, round down.
	oppHandler := handlerKey("Vorinclex, Monstrous Raider", "opp_halve", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      oppHandler,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller == p.Controller {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() / 2) // integer division rounds toward zero
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Vorinclex, Monstrous Raider",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "opp_halve"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Vorinclex, Monstrous Raider",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: oppHandler,
	})
}

// RegisterOzolithShatteredSpire — "If a creature entering the battlefield
// under your control would have a +1/+1 counter put on it, it enters with
// that many plus one +1/+1 counters on it instead." Modern Horizons 3
// legendary artifact.
//
// Per the card text the gate is BOTH "creature you control" AND "entering
// the battlefield" — the latter narrowing distinguishes Ozolith from
// Hardened Scales (which fires on any +1/+1 placement, not just ETB).
// Phase 6 implements the controller + creature gate; the ETB-only narrowing
// rides on the would_put_counter dispatch site — `resolvePermanentSpellETB`
// in resolve.go fires the event with Payload["etb"]=true (added in this
// commit). For pre-Phase-6 call sites the etb gate degrades gracefully:
// absent the payload, the handler is conservative and DOES NOT fire,
// matching Ozolith's "entering the battlefield" gate.
func RegisterOzolithShatteredSpire(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	hid := handlerKey("Ozolith, the Shattered Spire", "etb_plus1", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      hid,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller != p.Controller {
				return false
			}
			if !ev.TargetPerm.IsCreature() {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			// ETB gate — only fire when the placement is part of the
			// creature's enter-the-battlefield event. Phase 6 callers
			// stamp Payload["etb"]=true; non-ETB placements skip.
			if etb, ok := ev.Payload["etb"].(bool); !ok || !etb {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Ozolith, the Shattered Spire",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "etb_plus_one"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Ozolith, the Shattered Spire",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: hid,
	})
}

// RegisterConclaveMentor — "If one or more +1/+1 counters would be put on
// a creature you control, that many plus one +1/+1 counters are put on it
// instead." Strixhaven uncommon. Same shape as Hardened Scales — the two
// stack additively (each adds +1 in turn) per §122.1g.
func RegisterConclaveMentor(gs *GameState, p *Permanent) {
	if p == nil {
		return
	}
	hid := handlerKey("Conclave Mentor", "plus1", p)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      hid,
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller != p.Controller {
				return false
			}
			if !ev.TargetPerm.IsCreature() {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(Event{
				Kind: "replacement_applied", Seat: p.Controller,
				Source:  "Conclave Mentor",
				Amount:  ev.Count(),
				Details: map[string]interface{}{"rule": "122.1g", "effect": "plus_one_counter"},
			})
		},
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Conclave Mentor",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: hid,
	})
}

// StampDoublingSeasonAudit / StampHardenedScalesAudit retrofit the
// pre-Phase-6 RegisterDoublingSeason + RegisterHardenedScales handlers
// (which already register replacement effects but do not populate the
// Phase 5 ProvidesReplacements audit surface) with the same audit slot the
// new doublers use. Callers should invoke these immediately after the
// Register* call; they are idempotent so re-flickered permanents don't
// double-stamp.
func StampDoublingSeasonAudit(p *Permanent) {
	if p == nil {
		return
	}
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Doubling Season",
		EventType: "would_create_token",
		Category:  CategoryOther,
		HandlerID: handlerKey("Doubling Season", "token_dbl", p),
	})
	addProvidesReplacements(p, ReplacementSpec{
		Name:      "Doubling Season",
		EventType: "would_put_counter",
		Category:  CategoryOther,
		HandlerID: handlerKey("Doubling Season", "counter_dbl", p),
	})
}

// EnterCounterCountWithDoublers fires the §122.1g would_put_counter
// replacement chain for `base` counters of `counterType` ENTERING on `perm`
// (a planeswalker's starting loyalty, a battle's defense) and returns the
// post-doubling count. It does NOT place the counters — the caller writes the
// returned count into the permanent's loyalty/defense slot, which lives in the
// legacy perm.Counters map (the loyalty-cost machinery reads/writes it there).
// This is the §306.5g / §310 entry that lets Doubling Season double a
// planeswalker's starting loyalty (and Vorinclex halve an opponent's), which
// the old direct `perm.Counters["loyalty"] = n` assignment silently skipped.
func EnterCounterCountWithDoublers(gs *GameState, perm *Permanent, counterType string, base int) int {
	if gs == nil || perm == nil || base <= 0 {
		return base
	}
	ev := NewReplEvent("would_put_counter")
	ev.TargetPerm = perm
	ev.TargetSeat = perm.Controller
	ev.Payload["counter_type"] = counters.CanonicalName(counterType)
	ev.Payload["etb"] = true
	ev.SetCount(base)
	FireEvent(gs, ev)
	if ev.Cancelled {
		return 0
	}
	final := ev.Count()
	if final < 0 {
		final = 0
	}
	return final
}

// AddCountersToPermanent is the Phase 6 engine-side entry point for placing
// counters on a permanent. Fires the §616 would_put_counter replacement
// chain through gs.Replacements (so Doubling Season / Hardened Scales /
// Primal Vigor / etc. all get a chance to modify the count), then routes
// the post-§616 final count through counters.AddCounters so the resulting
// CounterStack carries the InstanceID lineage required by §122.6
// persistence + Phase 4 Proliferate accounting.
//
// Returns the actual count placed (post-doubling) and the §616 audit
// chain (ReplacementRef slice) — callers that need the chain for
// downstream event records (Heimdall, Loki, hat analysis) consume it
// directly; callers that just want the count discard the chain.
//
// nil-safe: nil gs or nil perm yield (0, nil). count <= 0 short-circuits
// to (0, nil). Cancelled replacements (Cancelled=true in ApplyFn) result
// in 0 counters placed; the audit chain is still returned so the cancel
// is forensically visible.
//
// etbContext, when true, sets Payload["etb"]=true on the fired event so
// ETB-gated doublers (Ozolith the Shattered Spire) fire correctly. Non-ETB
// counter placements pass false; in-flight ETB placements
// (resolvePermanentSpellETB and equivalents) pass true.
//
// sourceInstanceID is the InstanceID of the ability/effect placing the
// counter — Phase 4 Proliferate and Phase 5 lineage paths both rely on
// this for the §122 lineage chain.
func AddCountersToPermanent(
	gs *GameState,
	perm *Permanent,
	counterType string,
	count int,
	sourceInstanceID string,
	etbContext bool,
) (int, []ReplacementRef) {
	if gs == nil || perm == nil || count <= 0 {
		return 0, nil
	}
	def := counters.Lookup(counterType)
	canonical := counters.CanonicalName(counterType)
	// Skip the doubling pipeline for §106.11 energy and the player-counter
	// family (DoublingApplies=false). counters.AddCounters still places the
	// requested count; we just don't fire the chain for types §122.1g
	// excludes.
	if def == nil || !def.DoublingApplies {
		placed, err := counters.AddCounters(perm.AsCounterTarget(), canonical, count, sourceInstanceID, gs.EffectTimestamp)
		if err != nil {
			return 0, nil
		}
		return placed, nil
	}

	ev := NewReplEvent("would_put_counter")
	ev.TargetPerm = perm
	ev.TargetSeat = perm.Controller
	ev.Payload["counter_type"] = canonical
	ev.Payload["etb"] = etbContext
	ev.SetCount(count)
	FireEvent(gs, ev)
	if ev.Cancelled {
		return 0, ev.AppliedChain
	}
	final := ev.Count()
	if final <= 0 {
		return 0, ev.AppliedChain
	}
	placed, err := counters.AddCounters(perm.AsCounterTarget(), canonical, final, sourceInstanceID, gs.EffectTimestamp)
	if err != nil {
		return 0, ev.AppliedChain
	}
	return placed, ev.AppliedChain
}
