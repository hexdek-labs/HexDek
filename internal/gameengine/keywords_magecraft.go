package gameengine

// keywords_magecraft.go — Magecraft (CR §702.137, Strixhaven 2021).
//
// CR §702.137a: Magecraft is an ability word. It indicates a triggered
//               ability of the form "Whenever you cast or copy an
//               instant or sorcery spell, ..." that functions while
//               the permanent with magecraft is on the battlefield.
// CR §702.137b: Magecraft triggers fire when the controller of the
//               permanent with magecraft EITHER casts an instant or
//               sorcery spell, OR creates a copy of an instant or
//               sorcery spell on the stack. The "copy" branch is the
//               distinguishing feature — Storm, Replicate, Demonstrate,
//               Ripple, Cascade, etc. all trigger magecraft just like a
//               regular cast does.
//
// Implementation: HasMagecraft detects the keyword on a Card.
// FireMagecraftTriggers fans out the "magecraft" card trigger to every
// permanent the caster controls that has magecraft, but ONLY when the
// originating spell/copy is an instant or sorcery. Wired into:
//
//   - fireCastTriggersFromZone (stack.go) with isCopy=false — the
//     spell_cast event, fires for every real cast (hand, alt-cost,
//     zone-cast, alike).
//   - resolveCopySpell (resolve.go) with isCopy=true — the canonical
//     §707.10 "copy a target spell" path used by Twincast, Reverberate,
//     Fork, and the generic gameast.CopySpell effect node.
//
// Other copy paths (storm.go's storm copies, keywords_batch3.go's
// Replicate, keywords_demonstrate.go's Demonstrate, keywords_ripple.go's
// Ripple) make their own StackItem copies without routing through
// resolveCopySpell. Wiring magecraft into those is a follow-up — the
// API here is ready for it: call
//
//     FireMagecraftTriggers(gs, controllerSeat, copyCard, true)
//
// at the point of push. The single source of truth keeps the
// "instant/sorcery only" + "own spells only" filtering in one place.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasMagecraft
// ---------------------------------------------------------------------------

// HasMagecraft returns true if the card has the magecraft keyword in
// its AST. Magecraft is technically an "ability word" (CR §702.137a)
// rather than a keyword in the strict gameast sense, but the engine
// normalizes both into Keyword AST nodes so the same detector works
// for either spelling.
func HasMagecraft(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "magecraft") {
			return true
		}
	}
	return false
}

// permanentHasMagecraft is the Permanent-level variant. Reads from the
// card AST and from runtime grants (Permanent.GrantedAbilities,
// Flags["kw:magecraft"]) — magecraft is sometimes granted by triggered
// effects (e.g. Galazeth Prismari-style enchant-yourself riders in
// custom corpora), so we honor the same machinery the rest of the
// engine uses for keyword detection on permanents.
func permanentHasMagecraft(p *Permanent) bool {
	if p == nil {
		return false
	}
	if HasMagecraft(p.Card) {
		return true
	}
	for _, g := range p.GrantedAbilities {
		if equalFoldTrimmed(g, "magecraft") {
			return true
		}
	}
	if p.Flags != nil && p.Flags["kw:magecraft"] > 0 {
		return true
	}
	// Legacy path: pre-existing handlers may register magecraft via the
	// trigger-event registry rather than via the keyword AST. Honor that
	// so the new fan-out doesn't silently double-count or skip those.
	if permHasTriggerEvent(p, "magecraft") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// FireMagecraftTriggers
// ---------------------------------------------------------------------------

// FireMagecraftTriggers fires the magecraft trigger for `casterSeat`
// having just cast or copied `spell`. CR §702.137a.
//
// Filtering:
//   - spell must be an instant or sorcery (creature/artifact/enchantment
//     casts do NOT trigger magecraft — only instant/sorcery does)
//   - only permanents controlled by `casterSeat` are scanned
//     (CR §702.137a "Whenever YOU cast or copy ..." — opponent casts
//     never trigger another player's magecraft)
//
// Side effects:
//   - emits "magecraft_trigger" per fan-out (Seat=casterSeat,
//     Source=the magecraft permanent's name, Details carries
//     spell name and is_copy)
//   - calls FireCardTrigger("magecraft", ctx) for each magecraft
//     permanent so per_card handlers can implement card-specific
//     payoffs (Archmage Emeritus draw, Symmetry Sage pump, etc.)
//
// ctx keys forwarded to per_card handlers:
//   "source":     *Permanent  — the magecraft-bearing permanent
//   "controller": int          — casterSeat (matches source.Controller)
//   "spell":      *Card        — the cast/copied spell
//   "spell_name": string       — display name for logging
//   "is_copy":    bool         — true if this fan-out is from a copy
func FireMagecraftTriggers(gs *GameState, casterSeat int, spell *Card, isCopy bool) {
	if gs == nil || spell == nil {
		return
	}
	if casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	if gs.Seats[casterSeat] == nil {
		return
	}
	// CR §702.137a — only instant or sorcery spells trigger magecraft.
	if !cardHasType(spell, "instant") && !cardHasType(spell, "sorcery") {
		return
	}

	for _, p := range gs.Seats[casterSeat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Controller != casterSeat {
			// Defensive: scanning casterSeat's Battlefield should already
			// imply Controller==casterSeat, but Gilded Drake / Threaten
			// can leave a controller-swapped permanent parked on the
			// original owner's battlefield slice in some engine states.
			// The "you" in magecraft is the CONTROLLER of the magecraft
			// permanent — skip mismatches.
			continue
		}
		if !permanentHasMagecraft(p) {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "magecraft_trigger",
			Seat:   casterSeat,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"spell":   spellName(spell),
				"is_copy": isCopy,
				"rule":    "702.137a",
			},
		})
		FireCardTrigger(gs, "magecraft", map[string]interface{}{
			"source":     p,
			"controller": casterSeat,
			"spell":      spell,
			"spell_name": spell.DisplayName(),
			"is_copy":    isCopy,
		})
	}
}

// FireSpellCopyTriggers is the canonical "a spell was copied" trigger fan-out.
// EVERY code path that creates a copy of a spell on the stack must call this
// exactly once per copy, immediately after pushing the copy, with the player
// who created the copy as `controllerSeat` (the "you" in both triggers — for a
// demonstrate opponent-copy that is the opponent). It fires:
//
//   - magecraft (CR §702.137a — "whenever you cast OR COPY an instant or
//     sorcery spell"). FireMagecraftTriggers self-filters to instant/sorcery,
//     so a copy of a permanent spell (which resolves to a token) correctly
//     fires no magecraft here.
//   - the generic "spell_copied" trigger ("whenever you copy a spell" — The
//     Twelfth Doctor et al.), which fires for a copy of ANY spell.
//
// resolveCopySpell (Fork/Twincast/Reverberate) was the only path firing both;
// storm fired only magecraft; conspire / replicate / demonstrate / epic /
// tiered and the per_card copy-target-spell cards (Kalamax, Alania, Ivy, Krark,
// Aziza, Fire-Lord Azula, Mendicant Core) fired neither. Routing all of them
// through this one helper closes the r63 copy-trigger bypass class.
func FireSpellCopyTriggers(gs *GameState, controllerSeat int, copyCard, sourceCard *Card) {
	if gs == nil || copyCard == nil {
		return
	}
	FireMagecraftTriggers(gs, controllerSeat, copyCard, true)
	FireCardTrigger(gs, "spell_copied", map[string]interface{}{
		"caster_seat": controllerSeat,
		"copied_card": copyCard,
		"source_card": sourceCard,
	})
}
