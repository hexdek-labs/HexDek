package gameengine

import "math/rand"

// CloneForRollout creates a deep copy of the GameState suitable for MCTS
// simulation. The clone shares read-only data (Corpus, CardAST pointers)
// but owns all mutable state — mutating the clone does not affect the
// original.
//
// Function-bearing structures (ReplacementEffect, ContinuousEffect,
// DelayedTrigger) are shallow-copied: the struct is cloned but function
// fields point to the original closures. This is safe because those
// closures receive *GameState as a parameter (they operate on whatever
// state you pass). SourcePerm pointers are remapped to cloned permanents.
func (gs *GameState) CloneForRollout(rng *rand.Rand) *GameState {
	if gs == nil {
		return nil
	}

	// Phase 1: clone all cards, build identity map.
	cardMap := make(map[*Card]*Card, 256)
	cloneCard := func(c *Card) *Card {
		if c == nil {
			return nil
		}
		if cc, ok := cardMap[c]; ok {
			return cc
		}
		cc := c.DeepCopy()
		cardMap[c] = cc
		return cc
	}

	// Phase 2: clone all permanents, build identity map.
	permMap := make(map[*Permanent]*Permanent, 64)
	clonePerm := func(p *Permanent) *Permanent {
		if p == nil {
			return nil
		}
		if cp, ok := permMap[p]; ok {
			return cp
		}
		cp := &Permanent{
			Card:          cloneCard(p.Card),
			Controller:    p.Controller,
			Owner:         p.Owner,
			Tapped:        p.Tapped,
			SummoningSick: p.SummoningSick,
			PhasedOut:     p.PhasedOut,
			DoesNotUntap:  p.DoesNotUntap,
			Timestamp:     p.Timestamp,
			MarkedDamage:  p.MarkedDamage,
			Transformed:   p.Transformed,
			FrontFaceAST:  p.FrontFaceAST, // read-only
			BackFaceAST:   p.BackFaceAST,  // read-only
			FrontFaceName: p.FrontFaceName,
			BackFaceName:  p.BackFaceName,
		}
		if p.OriginalCard != nil {
			cp.OriginalCard = cloneCard(p.OriginalCard)
		}
		if p.Counters != nil {
			cp.Counters = make(map[string]int, len(p.Counters))
			for k, v := range p.Counters {
				cp.Counters[k] = v
			}
		}
		if p.Flags != nil {
			cp.Flags = make(map[string]int, len(p.Flags))
			for k, v := range p.Flags {
				cp.Flags[k] = v
			}
		}
		cp.Modifications = append([]Modification(nil), p.Modifications...)
		cp.GrantedAbilities = append([]string(nil), p.GrantedAbilities...)
		permMap[p] = cp
		return cp
	}

	// Phase 3: walk all seats, clone zones and permanents.
	// First pass: clone permanents to populate permMap.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			clonePerm(p)
		}
	}
	// Also clone permanents referenced by stack items.
	for _, si := range gs.Stack {
		if si != nil && si.Source != nil {
			clonePerm(si.Source)
		}
	}

	// Fix AttachedTo references.
	for orig, cp := range permMap {
		if orig.AttachedTo != nil {
			cp.AttachedTo = permMap[orig.AttachedTo]
		}
	}

	// Phase 4: clone seats.
	seats := make([]*Seat, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		castCounts := make(map[string]int, len(s.CommanderCastCounts))
		for k, v := range s.CommanderCastCounts {
			castCounts[k] = v
		}
		ns := &Seat{
			Idx:                 s.Idx,
			Life:                s.Life,
			Lost:                s.Lost,
			Won:                 s.Won,
			PoisonCounters:      s.PoisonCounters,
			StartingLife:        s.StartingLife,
			LeftGame:            s.LeftGame,
			ManaPool:            s.ManaPool,
			CommanderNames:      append([]string(nil), s.CommanderNames...),
			CommanderCastCounts: castCounts,
			CommanderTax:        castCounts, // same underlying map
			LossReason:          s.LossReason,
			SBA704_5a_emitted:   s.SBA704_5a_emitted,
			CommanderDamageNextSeq: s.CommanderDamageNextSeq,
			AttemptedEmptyDraw:  s.AttemptedEmptyDraw,
			SpellsCastThisTurn:  s.SpellsCastThisTurn,
			SpellsCastLastTurn:  s.SpellsCastLastTurn,
			DescendedThisTurn:   s.DescendedThisTurn,
			SkipUntapStep:       s.SkipUntapStep,
			ControlledBy:        s.ControlledBy,
			CompanionMoved:      s.CompanionMoved,
		}

		// Zones.
		ns.Library = cloneCardSlice(s.Library, cloneCard)
		ns.Hand = cloneCardSlice(s.Hand, cloneCard)
		ns.Graveyard = cloneCardSlice(s.Graveyard, cloneCard)
		ns.Exile = cloneCardSlice(s.Exile, cloneCard)
		ns.CommandZone = cloneCardSlice(s.CommandZone, cloneCard)

		// Battlefield: use pre-cloned permanents.
		ns.Battlefield = make([]*Permanent, len(s.Battlefield))
		for j, p := range s.Battlefield {
			ns.Battlefield[j] = permMap[p]
		}

		// Commander damage.
		if len(s.CommanderDamage) > 0 {
			ns.CommanderDamage = make(map[int]map[string]int, len(s.CommanderDamage))
			for dealer, inner := range s.CommanderDamage {
				m := make(map[string]int, len(inner))
				for k, v := range inner {
					m[k] = v
				}
				ns.CommanderDamage[dealer] = m
			}
		} else {
			ns.CommanderDamage = map[int]map[string]int{}
		}

		// Mana pool.
		if s.Mana != nil {
			mp := *s.Mana
			mp.Restricted = append([]RestrictedMana(nil), s.Mana.Restricted...)
			ns.Mana = &mp
		}

		// Companion.
		if s.Companion != nil {
			ns.Companion = cloneCard(s.Companion)
		}

		// Flags.
		if s.Flags != nil {
			ns.Flags = make(map[string]int, len(s.Flags))
			for k, v := range s.Flags {
				ns.Flags[k] = v
			}
		}

		seats[i] = ns
	}

	// Phase 5: clone stack.
	stack := make([]*StackItem, len(gs.Stack))
	for i, si := range gs.Stack {
		if si == nil {
			continue
		}
		nsi := &StackItem{
			ID:         si.ID,
			Controller: si.Controller,
			Card:       cloneCard(si.Card),
			Source:      permMap[si.Source],
			Effect:     si.Effect, // interface value, not mutated
			Countered:  si.Countered,
			Kind:       si.Kind,
			AbilityIdx: si.AbilityIdx,
			IsCopy:     si.IsCopy,
			CastZone:   si.CastZone,
			ChosenX:    si.ChosenX,
		}
		// Targets.
		if len(si.Targets) > 0 {
			nsi.Targets = make([]Target, len(si.Targets))
			for j, t := range si.Targets {
				nt := t
				nt.Permanent = permMap[t.Permanent]
				nt.Card = cloneCard(t.Card)
				nt.Stack = nil // cross-stack refs not remapped
				nsi.Targets[j] = nt
			}
		}
		// CostMeta: shallow copy is sufficient for rollout.
		if si.CostMeta != nil {
			nsi.CostMeta = make(map[string]interface{}, len(si.CostMeta))
			for k, v := range si.CostMeta {
				nsi.CostMeta[k] = v
			}
		}
		stack[i] = nsi
	}

	// Phase 6: assemble the clone.
	clone := &GameState{
		Seats:                      seats,
		Rng:                        rng,
		Turn:                       gs.Turn,
		Phase:                      gs.Phase,
		Step:                       gs.Step,
		Active:                     gs.Active,
		Stack:                      stack,
		Cards:                      gs.Cards, // read-only corpus, shared
		EffectTimestamp:             gs.EffectTimestamp,
		CommanderFormat:             gs.CommanderFormat,
		PainterColor:               gs.PainterColor,
		PendingExtraCombats:        gs.PendingExtraCombats,
		SpellsCastThisTurn:         gs.SpellsCastThisTurn,
		SpellsCastByActiveLastTurn: gs.SpellsCastByActiveLastTurn,
		DayNight:                   gs.DayNight,
	}

	// EventLog: copy values (Events are value types, but Details map needs shallow copy).
	if len(gs.EventLog) > 0 {
		clone.EventLog = make([]Event, len(gs.EventLog))
		copy(clone.EventLog, gs.EventLog)
	}

	// Flags.
	if gs.Flags != nil {
		clone.Flags = make(map[string]int, len(gs.Flags))
		for k, v := range gs.Flags {
			clone.Flags[k] = v
		}
	}

	// Replacement effects: shallow-clone structs, remap SourcePerm.
	if len(gs.Replacements) > 0 {
		clone.Replacements = make([]*ReplacementEffect, len(gs.Replacements))
		for i, re := range gs.Replacements {
			nre := *re
			nre.SourcePerm = permMap[re.SourcePerm]
			clone.Replacements[i] = &nre
		}
	}

	// Continuous effects.
	if len(gs.ContinuousEffects) > 0 {
		clone.ContinuousEffects = make([]*ContinuousEffect, len(gs.ContinuousEffects))
		for i, ce := range gs.ContinuousEffects {
			nce := *ce
			nce.SourcePerm = permMap[ce.SourcePerm]
			clone.ContinuousEffects[i] = &nce
		}
	}

	// Delayed triggers.
	if len(gs.DelayedTriggers) > 0 {
		clone.DelayedTriggers = make([]*DelayedTrigger, len(gs.DelayedTriggers))
		for i, dt := range gs.DelayedTriggers {
			ndt := *dt
			clone.DelayedTriggers[i] = &ndt
		}
	}

	// Prevention shields: value types, safe to copy.
	if len(gs.PreventionShields) > 0 {
		clone.PreventionShields = append([]PreventionShield(nil), gs.PreventionShields...)
	}

	// ZoneCastGrants: remap card keys.
	if len(gs.ZoneCastGrants) > 0 {
		clone.ZoneCastGrants = make(map[*Card]*ZoneCastPermission, len(gs.ZoneCastGrants))
		for card, perm := range gs.ZoneCastGrants {
			cc := cloneCard(card)
			np := *perm
			clone.ZoneCastGrants[cc] = &np
		}
	}

	// charCache is intentionally NOT copied — it will rebuild on demand.

	return clone
}

func cloneCardSlice(src []*Card, cloneCard func(*Card) *Card) []*Card {
	if src == nil {
		return nil
	}
	dst := make([]*Card, len(src))
	for i, c := range src {
		dst[i] = cloneCard(c)
	}
	return dst
}
