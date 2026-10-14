package gameengine

// keywords_heroic.go — CR §702.123 Heroic (Theros, 2013).
//
// Heroic is a printed-on-the-creature triggered ability:
//
//   "Whenever you cast a spell that targets this creature, [effect]."
//
// Key rules from §702.123a-b:
//
//   - The trigger checks AT CAST TIME (the moment targets are locked),
//     NOT at resolution. Countering the targeting spell doesn't prevent
//     the heroic trigger.
//   - "You" means the heroic creature's controller. Only spells YOU
//     cast that target YOUR heroic creature fire the trigger — an
//     opponent targeting your heroic creature (or you targeting an
//     opponent's heroic creature) does NOT trigger.
//   - Each heroic creature targeted fires its own trigger. A spell with
//     multiple targets that hits two of your heroic creatures fires
//     two triggers.
//   - Triggers fire ONCE per cast even if the same heroic creature is
//     somehow listed as a target multiple times by the spell (rare,
//     but possible with effects like "target two creatures, they get
//     +N/+N each"). The de-dup is intentional: §702.123a says "a spell
//     that targets this creature," which the rules glossary treats as
//     a single targeting event per heroic creature per spell, not one
//     per target slot.
//
// Architecture:
//
//   - HasHeroic(card)             keyword detection.
//   - FireHeroicTriggers(gs, item) cast-time fan-out. Iterates
//                                   item.Targets, picks out
//                                   TargetKindPermanent entries whose
//                                   permanent has heroic AND whose
//                                   controller == item.Controller, and
//                                   fires one "heroic" trigger per
//                                   unique heroic permanent.
//
// Wired into CastSpellWithCosts right after CheckWardOnTargeting so
// cast-time triggers all converge in one place (stack.go).
//
// The trigger dispatches via FireCardTrigger("heroic", ctx) with:
//
//   "source": *Permanent  — the heroic creature whose ability fires
//   "spell":  *Card       — the spell being cast that targeted it
//   "caster_seat": int    — the spell's controller (== source's
//                            controller, per the trigger gate)
//
// An "heroic_trigger" event is also logged for analytics / test
// observability. The event mirrors the singular FireHeroicTrigger
// helper in keywords_misc.go (rule "HRC") but uses rule "702.123a"
// for the §-cite trail.

// HasHeroic reports whether the card carries the heroic keyword.
func HasHeroic(card *Card) bool {
	return cardHasKeywordByName(card, "heroic")
}

// FireHeroicTriggers runs the cast-time heroic fan-out for a spell
// `item` that has just had its targets locked in. CR §702.123a.
//
// For each permanent target whose:
//   - permanent has the heroic keyword AND
//   - permanent's controller == item.Controller (the spell's caster),
//
// we fire exactly one "heroic" FireCardTrigger and log one
// "heroic_trigger" event. De-dups by permanent identity so a spell
// that lists the same heroic creature twice doesn't double-fire.
//
// Defensive early returns: nil gs/item/Card, item with no targets,
// item with Source set (heroic only triggers on SPELL casts per
// §702.123a — abilities being put on the stack don't count, even if
// they target the heroic creature). The item.Card == nil case also
// returns early (triggered/activated stack items have item.Source
// rather than item.Card; this enforces the "spell" gate).
func FireHeroicTriggers(gs *GameState, item *StackItem) {
	if gs == nil || item == nil || item.Card == nil {
		return
	}
	// Spell-only gate: skip activated / triggered ability stack items.
	if item.Source != nil {
		return
	}
	if len(item.Targets) == 0 {
		return
	}

	casterSeat := item.Controller
	spellName := item.Card.DisplayName()
	fired := map[*Permanent]bool{}

	for _, tgt := range item.Targets {
		if tgt.Kind != TargetKindPermanent || tgt.Permanent == nil {
			continue
		}
		perm := tgt.Permanent
		if fired[perm] {
			continue
		}
		// Same-controller gate: "you cast a spell that targets this
		// creature" — caster must control the heroic creature.
		if perm.Controller != casterSeat {
			continue
		}
		if !HasHeroic(perm.Card) {
			continue
		}
		// Face-down permanents have no abilities per §708.2.
		if perm.Flags != nil && perm.Flags["face_down"] == 1 {
			continue
		}
		fired[perm] = true

		FireCardTrigger(gs, "heroic", map[string]interface{}{
			"source":      perm,
			"spell":       item.Card,
			"caster_seat": casterSeat,
		})
		gs.LogEvent(Event{
			Kind:   "heroic_trigger",
			Seat:   casterSeat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"spell": spellName,
				"rule":  "702.123a",
			},
		})
	}
}
