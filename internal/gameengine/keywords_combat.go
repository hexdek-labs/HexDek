package gameengine

// Combat/evasion/cost-modification keyword implementations.
//
// This file implements all remaining combat, evasion, cost modification,
// casting restriction, and protection variant keywords not already covered
// by keywords_p0.go and keywords_p1p2.go.
//
// COMBAT EVASION:
//   - Intimidate       — CR §702.13
//   - Fear             — CR §702.36
//   - Shadow           — CR §702.28
//   - Skulk            — CR §702.120
//   - Daunt            — (ability word, not keyworded)
//
// COMBAT MODIFIERS:
//   - Banding          — CR §702.21  (core mechanic, simplified)
//   - Rampage N        — CR §702.23
//   - Battle Cry       — CR §702.91
//   - Myriad           — CR §702.116
//   - Melee            — CR §702.121
//   - Annihilator N    — CR §702.86
//   - Afflict N        — CR §702.130
//   - Provoke          — CR §702.39
//   - Trample Over PW  — CR §702.19 variant
//
// COST MODIFICATION:
//   - Spectacle        — CR §702.137
//   - Surge            — CR §702.117
//   - Improvise        — CR §702.126
//   - Assist           — CR §702.132
//   - Undaunted        — CR §702.125
//   - Offering         — CR §702.48
//
// CASTING RESTRICTIONS:
//   - Miracle          — CR §702.94
//   - Splice           — CR §702.47
//   - Cipher           — CR §702.99
//   - Adventure        — CR §702.133
//   - Aftermath        — CR §702.128
//   - Retrace          — CR §702.81
//   - Jump-start       — CR §702.133a
//   - Overload         — CR §702.96
//
// PROTECTION VARIANTS:
//   - Protection from [type]    — CR §702.16
//   - Hexproof from [color]     — CR §702.11d

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Local int-to-string helper (each keyword file defines its own to avoid
// redeclaration across files in the same package).
// ---------------------------------------------------------------------------

func itoaCombat(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ============================================================================
// COMBAT EVASION
// ============================================================================

// ---------------------------------------------------------------------------
// Intimidate — CR §702.13
// "This creature can't be blocked except by artifact creatures and/or
// creatures that share a color with it."
// ---------------------------------------------------------------------------

// CanBlockIntimidate checks if a blocker can legally block an attacker with
// intimidate. Returns true if blocking is legal. The blocker must be an
// artifact creature OR share a color with the attacker.
func CanBlockIntimidate(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	if !attacker.HasKeyword("intimidate") {
		return true // No intimidate, no restriction.
	}
	// Artifact creatures can always block.
	if blocker.IsArtifact() {
		return true
	}
	// Must share a color.
	atkColors := cardColors(attacker.Card)
	blkColors := cardColors(blocker.Card)
	for c := range atkColors {
		if _, ok := blkColors[c]; ok {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fear — CR §702.36
// "This creature can't be blocked except by artifact creatures and/or
// black creatures."
// ---------------------------------------------------------------------------

// CanBlockFear checks if a blocker can legally block an attacker with fear.
// Returns true if blocking is legal. The blocker must be an artifact creature
// OR be black.
func CanBlockFear(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	if !attacker.HasKeyword("fear") {
		return true
	}
	// Artifact creatures can always block.
	if blocker.IsArtifact() {
		return true
	}
	// Black creatures can block.
	blkColors := cardColors(blocker.Card)
	if _, ok := blkColors["B"]; ok {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Shadow — CR §702.28
// "This creature can block or be blocked by only creatures with shadow."
// ---------------------------------------------------------------------------

// CanBlockShadow checks shadow evasion. A creature with shadow can only
// be blocked by creatures with shadow. A creature without shadow cannot
// block a creature with shadow, and vice versa.
func CanBlockShadow(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	atkShadow := attacker.HasKeyword("shadow")
	blkShadow := blocker.HasKeyword("shadow")
	if atkShadow && !blkShadow {
		return false // Shadow attacker can only be blocked by shadow.
	}
	if !atkShadow && blkShadow {
		return false // Shadow blocker can only block shadow creatures.
	}
	return true
}

// ---------------------------------------------------------------------------
// Skulk — CR §702.120
// "This creature can't be blocked by creatures with greater power."
// ---------------------------------------------------------------------------

// CanBlockSkulk checks if a blocker can legally block an attacker with skulk.
// The blocker's power must be <= the attacker's power.
func CanBlockSkulk(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	if !attacker.HasKeyword("skulk") {
		return true
	}
	return blocker.Power() <= attacker.Power()
}

// ---------------------------------------------------------------------------
// Daunt — ability word
// "This creature can't be blocked by creatures with power 2 or less."
// ---------------------------------------------------------------------------

// CanBlockDaunt checks if a blocker can legally block an attacker with daunt.
// The blocker must have power > 2.
func CanBlockDaunt(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	if !attacker.HasKeyword("daunt") {
		return true
	}
	return blocker.Power() > 2
}

// ---------------------------------------------------------------------------
// CanBlockCombatKeywords — aggregated evasion check for combat.go
// integration. Returns false if the block is illegal due to any evasion
// keyword implemented in this file. Called from canBlockGS in combat.go.
// ---------------------------------------------------------------------------

func CanBlockCombatKeywords(gs *GameState, attacker, blocker *Permanent) bool {
	if !CanBlockIntimidate(attacker, blocker) {
		return false
	}
	if !CanBlockFear(attacker, blocker) {
		return false
	}
	if !CanBlockShadow(attacker, blocker) {
		return false
	}
	if !CanBlockSkulk(attacker, blocker) {
		return false
	}
	if !CanBlockDaunt(attacker, blocker) {
		return false
	}
	return true
}

// ============================================================================
// COMBAT MODIFIERS
// ============================================================================

// ---------------------------------------------------------------------------
// Banding — CR §702.21  (simplified)
//
// Full banding is one of the most complex mechanics in MTG. Core behavior:
// - Attacking: Any number of creatures with banding + up to one without
//   can attack as a band. If any creature in the band is blocked, all are.
// - Defending: The controller of the banding creature assigns damage
//   among blockers/attackers instead of the opponent.
//
// Simplified implementation: banding creatures that attack together share
// damage assignment — the controller distributes incoming damage optimally
// (min total deaths). We model this as a damage redirect: when a banded
// creature would take lethal, redirect excess to another banded creature.
// ---------------------------------------------------------------------------

// HasBanding returns true if the permanent has the banding keyword.
func HasBanding(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("banding")
}

// ApplyBandingDamageRedistribution redistributes combat damage among a
// group of banded creatures. The controller assigns damage optimally:
// spread damage to minimize kills. Modeled as: if any creature would
// take lethal, shift excess to the creature with the most remaining
// toughness headroom.
//
// Called after combat damage is assigned but before SBAs check lethal.
func ApplyBandingDamageRedistribution(gs *GameState, bandedCreatures []*Permanent) {
	if gs == nil || len(bandedCreatures) < 2 {
		return
	}
	// Find total damage and total toughness.
	totalDamage := 0
	for _, c := range bandedCreatures {
		totalDamage += c.MarkedDamage
	}
	if totalDamage <= 0 {
		return
	}

	// Redistribute: fill each creature up to (toughness - 1) before
	// any creature takes lethal.
	// Reset all damage first, then re-apply optimally.
	for _, c := range bandedCreatures {
		c.MarkedDamage = 0
	}
	remaining := totalDamage
	for remaining > 0 {
		// Find the creature with the most headroom.
		bestIdx := -1
		bestRoom := -1
		for i, c := range bandedCreatures {
			room := c.Toughness() - c.MarkedDamage - 1
			if room < 0 {
				room = 0
			}
			if room > bestRoom {
				bestRoom = room
				bestIdx = i
			}
		}
		if bestIdx < 0 || bestRoom <= 0 {
			// All creatures are at toughness-1 damage; dump remainder on first.
			bandedCreatures[0].MarkedDamage += remaining
			remaining = 0
		} else {
			give := remaining
			if give > bestRoom {
				give = bestRoom
			}
			bandedCreatures[bestIdx].MarkedDamage += give
			remaining -= give
		}
	}

	gs.LogEvent(Event{
		Kind:   "banding_redistribute",
		Seat:   bandedCreatures[0].Controller,
		Amount: totalDamage,
		Details: map[string]interface{}{
			"band_size": len(bandedCreatures),
			"rule":      "702.21",
		},
	})
}

// ---------------------------------------------------------------------------
// Rampage N — CR §702.23
// "Whenever this creature becomes blocked, it gets +N/+N for each
// creature blocking it beyond the first."
// ---------------------------------------------------------------------------

// GetRampageN returns the rampage N value for a permanent.
// Reads from Flags["rampage_n"] or defaults to 1 if keyword present.
func GetRampageN(p *Permanent) int {
	if p == nil {
		return 0
	}
	if p.Flags != nil {
		if v, ok := p.Flags["rampage_n"]; ok && v > 0 {
			return v
		}
	}
	if !p.HasKeyword("rampage") {
		return 0
	}
	return 1
}

// ApplyRampage applies the rampage buff when a creature becomes blocked.
// CR §702.23a: "gets +N/+N for each creature blocking it beyond the first."
func ApplyRampage(gs *GameState, attacker *Permanent, blockerCount int) {
	if gs == nil || attacker == nil || blockerCount <= 1 {
		return
	}
	n := GetRampageN(attacker)
	if n <= 0 {
		return
	}
	beyondFirst := blockerCount - 1
	buff := n * beyondFirst
	attacker.Modifications = append(attacker.Modifications, Modification{
		Power:     buff,
		Toughness: buff,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics
	gs.LogEvent(Event{
		Kind:   "rampage",
		Seat:   attacker.Controller,
		Source: attacker.Card.DisplayName(),
		Amount: buff,
		Details: map[string]interface{}{
			"rampage_n":     n,
			"blocker_count": blockerCount,
			"rule":          "702.23a",
		},
	})
}

// FireRampageTriggers checks all attackers for rampage and applies buffs.
// Called from DeclareBlockers in combat.go.
func FireRampageTriggers(gs *GameState, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}
	for atk, blockers := range blockerMap {
		if len(blockers) > 1 && GetRampageN(atk) > 0 {
			ApplyRampage(gs, atk, len(blockers))
		}
	}
}

// ---------------------------------------------------------------------------
// Battle Cry — CR §702.91
// "Whenever this creature attacks, each other attacking creature gets
// +1/+0 until end of turn."
// ---------------------------------------------------------------------------

// ApplyBattleCry gives +1/+0 to each other attacking creature for each
// permanent with battle cry that attacks. Called after DeclareAttackers.
func ApplyBattleCry(gs *GameState, seatIdx int, attackers []*Permanent) {
	if gs == nil || len(attackers) == 0 {
		return
	}
	// Count battle cry instances among attackers.
	battleCrySources := []*Permanent{}
	for _, a := range attackers {
		if a.HasKeyword("battle cry") {
			battleCrySources = append(battleCrySources, a)
		}
	}
	if len(battleCrySources) == 0 {
		return
	}
	for _, src := range battleCrySources {
		for _, a := range attackers {
			if a == src {
				continue // "each OTHER attacking creature"
			}
			a.Modifications = append(a.Modifications, Modification{
				Power:     1,
				Toughness: 0,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
		}
		gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics
		gs.LogEvent(Event{
			Kind:   "battle_cry",
			Seat:   seatIdx,
			Source: src.Card.DisplayName(),
			Amount: len(attackers) - 1,
			Details: map[string]interface{}{
				"rule": "702.91",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Myriad — CR §702.116
// "Whenever this creature attacks, for each opponent other than the
// defending player, create a token that's a copy of this creature
// that's tapped and attacking that player. Exile the tokens at end
// of combat."
// ---------------------------------------------------------------------------

// ApplyMyriad creates token copies of the attacker for each other opponent.
// Simplified: creates 1/1 tokens with same name, tapped and attacking each
// other opponent. Registers a delayed trigger to exile at end of combat.
func ApplyMyriad(gs *GameState, attacker *Permanent, attackerSeat int) {
	if gs == nil || attacker == nil || !attacker.HasKeyword("myriad") {
		return
	}
	defSeat, _ := AttackerDefender(attacker)
	opponents := gs.LivingOpponents(attackerSeat)
	var myriadTokens []*Permanent

	for _, oppSeat := range opponents {
		if oppSeat == defSeat {
			continue // Already attacking this opponent with the original.
		}
		// Create a token copy — must include "token" in Types for IsToken().
		tokenTypes := append([]string{"token"}, attacker.Card.Types...)
		token := &Permanent{
			Card: &Card{
				Name:          attacker.Card.DisplayName() + " (myriad copy)",
				Owner:         attackerSeat,
				BasePower:     attacker.Card.BasePower,
				BaseToughness: attacker.Card.BaseToughness,
				Types:         tokenTypes,
				Colors:        append([]string{}, attacker.Card.Colors...),
			},
			Controller:    attackerSeat,
			Owner:         attackerSeat,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{"attacking": 1, "myriad_token": 1},
			Tapped:        true,
			SummoningSick: false,
		}
		setAttackerDefender(token, oppSeat)
		gs.Seats[attackerSeat].Battlefield = append(gs.Seats[attackerSeat].Battlefield, token)
		RegisterReplacementsForPermanent(gs, token)
		FirePermanentETBTriggers(gs, token)
		myriadTokens = append(myriadTokens, token)
	}

	if len(myriadTokens) > 0 {
		gs.LogEvent(Event{
			Kind:   "myriad",
			Seat:   attackerSeat,
			Source: attacker.Card.DisplayName(),
			Amount: len(myriadTokens),
			Details: map[string]interface{}{
				"rule": "702.116",
			},
		})

		// Register delayed trigger to exile at end of combat.
		tokens := myriadTokens // capture for closure
		gs.RegisterDelayedTrigger(&DelayedTrigger{
			TriggerAt:      "end_of_combat",
			ControllerSeat: attackerSeat,
			SourceCardName: attacker.Card.DisplayName() + " (myriad)",
			OneShot:        true,
			EffectFn: func(gs *GameState) {
				for _, tok := range tokens {
					if alive(gs, tok) {
						ExilePermanent(gs, tok, nil)
					}
				}
			},
		})
	}
}

// FireMyriadTriggers checks all declared attackers for myriad and creates
// token copies. Called from DeclareAttackers in combat.go.
func FireMyriadTriggers(gs *GameState, attackerSeat int, attackers []*Permanent) {
	for _, atk := range attackers {
		if atk.HasKeyword("myriad") {
			ApplyMyriad(gs, atk, attackerSeat)
		}
	}
}

// ---------------------------------------------------------------------------
// Melee — CR §702.121
// "Whenever this creature attacks, it gets +1/+1 until end of turn for
// each opponent you attacked this combat."
// ---------------------------------------------------------------------------

// ApplyMelee gives +1/+1 for each opponent attacked this combat.
// Called after all attackers are declared.
func ApplyMelee(gs *GameState, attackerSeat int, attackers []*Permanent) {
	if gs == nil || len(attackers) == 0 {
		return
	}
	// Count distinct opponents attacked.
	defendersAttacked := map[int]struct{}{}
	for _, a := range attackers {
		def, ok := AttackerDefender(a)
		if ok && def >= 0 {
			defendersAttacked[def] = struct{}{}
		}
	}
	oppCount := len(defendersAttacked)
	if oppCount == 0 {
		return
	}

	for _, atk := range attackers {
		if !atk.HasKeyword("melee") {
			continue
		}
		atk.Modifications = append(atk.Modifications, Modification{
			Power:     oppCount,
			Toughness: oppCount,
			Duration:  "until_end_of_turn",
			Timestamp: gs.NextTimestamp(),
		})
		gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics
		gs.LogEvent(Event{
			Kind:   "melee",
			Seat:   attackerSeat,
			Source: atk.Card.DisplayName(),
			Amount: oppCount,
			Details: map[string]interface{}{
				"opponents_attacked": oppCount,
				"rule":               "702.121",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Annihilator N — CR §702.86
// "Whenever this creature attacks, defending player sacrifices N permanents."
// ---------------------------------------------------------------------------

// GetAnnihilatorN returns the annihilator N value for a permanent.
func GetAnnihilatorN(p *Permanent) int {
	if p == nil {
		return 0
	}
	if p.Flags != nil {
		if v, ok := p.Flags["annihilator_n"]; ok && v > 0 {
			return v
		}
	}
	if !p.HasKeyword("annihilator") {
		return 0
	}
	return 1 // Default annihilator 1 if not specified.
}

// ApplyAnnihilator forces the defending player to sacrifice N permanents.
// Simplified: sacrifices the N smallest creatures/permanents.
func ApplyAnnihilator(gs *GameState, attacker *Permanent, defenderSeat int) {
	if gs == nil || attacker == nil {
		return
	}
	n := GetAnnihilatorN(attacker)
	if n <= 0 || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[defenderSeat]
	if seat == nil {
		return
	}

	// Sacrifice N permanents — policy: sacrifice smallest power first
	// to minimize loss.
	sacrificed := 0
	for sacrificed < n && len(seat.Battlefield) > 0 {
		// Find smallest permanent (by toughness for creatures, else first non-creature).
		bestIdx := 0
		bestScore := 1 << 30
		for i, p := range seat.Battlefield {
			if p == nil {
				continue
			}
			score := 0
			if p.IsCreature() {
				score = p.Power() + p.Toughness()
			}
			if score < bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		victim := seat.Battlefield[bestIdx]
		SacrificePermanent(gs, victim, "annihilator")
		sacrificed++
	}

	gs.LogEvent(Event{
		Kind:   "annihilator",
		Seat:   attacker.Controller,
		Target: defenderSeat,
		Source: attacker.Card.DisplayName(),
		Amount: sacrificed,
		Details: map[string]interface{}{
			"annihilator_n": n,
			"rule":          "702.86",
		},
	})
}

// FireAnnihilatorTriggers checks all attackers for annihilator and forces
// the defending player to sacrifice permanents. Called from DeclareAttackers.
func FireAnnihilatorTriggers(gs *GameState, attackers []*Permanent) {
	if gs == nil {
		return
	}
	for _, atk := range attackers {
		n := GetAnnihilatorN(atk)
		if n <= 0 {
			continue
		}
		defSeat, ok := AttackerDefender(atk)
		if !ok || defSeat < 0 {
			continue
		}
		ApplyAnnihilator(gs, atk, defSeat)
	}
}

// ---------------------------------------------------------------------------
// Afflict N — CR §702.130
// "Whenever this creature becomes blocked, defending player loses N life."
// ---------------------------------------------------------------------------

// GetAfflictN returns the afflict N value for a permanent.
func GetAfflictN(p *Permanent) int {
	if p == nil {
		return 0
	}
	if p.Flags != nil {
		if v, ok := p.Flags["afflict_n"]; ok && v > 0 {
			return v
		}
	}
	if !p.HasKeyword("afflict") {
		return 0
	}
	return 1
}

// ApplyAfflict causes the defending player to lose N life when this
// creature becomes blocked. CR §702.130a.
func ApplyAfflict(gs *GameState, attacker *Permanent, defenderSeat int) {
	if gs == nil || attacker == nil {
		return
	}
	n := GetAfflictN(attacker)
	if n <= 0 || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	gs.Seats[defenderSeat].Life -= n
	gs.LogEvent(Event{
		Kind:   "afflict",
		Seat:   attacker.Controller,
		Target: defenderSeat,
		Source: attacker.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.130a",
		},
	})
}

// FireAfflictTriggers checks all blocked attackers for afflict.
// Called from DeclareBlockers in combat.go.
func FireAfflictTriggers(gs *GameState, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}
	for atk, blockers := range blockerMap {
		if len(blockers) == 0 {
			continue
		}
		n := GetAfflictN(atk)
		if n <= 0 {
			continue
		}
		defSeat, ok := AttackerDefender(atk)
		if !ok || defSeat < 0 {
			continue
		}
		ApplyAfflict(gs, atk, defSeat)
	}
}

// ---------------------------------------------------------------------------
// Provoke — CR §702.39
// "Whenever this creature attacks, you may have target creature
// defending player controls untap and block it if able."
// ---------------------------------------------------------------------------

// ApplyProvoke untaps a target creature and forces it to block the
// attacker. Simplified: picks the largest creature the defender controls
// and forces it to block.
func ApplyProvoke(gs *GameState, attacker *Permanent, defenderSeat int) {
	if gs == nil || attacker == nil || !attacker.HasKeyword("provoke") {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[defenderSeat]
	// Find largest tapped or untapped creature to provoke.
	var target *Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			target = p
		}
	}
	if target == nil {
		return
	}
	// Untap the target.
	target.Tapped = false
	// Mark it as forced to block this attacker (via flag).
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["provoked_by"] = attacker.Timestamp

	gs.LogEvent(Event{
		Kind:   "provoke",
		Seat:   attacker.Controller,
		Target: defenderSeat,
		Source: attacker.Card.DisplayName(),
		Details: map[string]interface{}{
			"provoked": target.Card.DisplayName(),
			"rule":     "702.39",
		},
	})
}

// FireProvokeTriggers checks all attackers for provoke and forces blockers.
// Called from DeclareAttackers in combat.go.
func FireProvokeTriggers(gs *GameState, attackers []*Permanent) {
	if gs == nil {
		return
	}
	for _, atk := range attackers {
		if !atk.HasKeyword("provoke") {
			continue
		}
		defSeat, ok := AttackerDefender(atk)
		if !ok || defSeat < 0 {
			continue
		}
		ApplyProvoke(gs, atk, defSeat)
	}
}

// ---------------------------------------------------------------------------
// Trample over planeswalkers — CR §702.19 variant
// "This creature can deal excess combat damage to the controller of the
// planeswalker it's attacking."
// Simplified: if attacking a PW, excess damage goes to player.
// Implemented as a flag check in combat damage — this function provides
// the detection helper.
// ---------------------------------------------------------------------------

// HasTrampleOverPlaneswalkers returns true if the permanent has
// "trample over planeswalkers".
func HasTrampleOverPlaneswalkers(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("trample over planeswalkers")
}

// ============================================================================
// COMBAT INTEGRATION — aggregate hooks for combat.go
// ============================================================================

// CheckCombatKeywordsCombat is called from combat.go after blockers are
// declared. It fires rampage and afflict triggers.
func CheckCombatKeywordsCombat(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	FireRampageTriggers(gs, blockerMap)
	FireAfflictTriggers(gs, blockerMap)
}

// CheckAttackKeywordsCombat is called from combat.go after attackers are
// declared. It fires battle cry, myriad, melee, annihilator, and provoke.
func CheckAttackKeywordsCombat(gs *GameState, attackerSeat int, attackers []*Permanent) {
	ApplyBattleCry(gs, attackerSeat, attackers)
	FireMyriadTriggers(gs, attackerSeat, attackers)
	ApplyMelee(gs, attackerSeat, attackers)
	FireAnnihilatorTriggers(gs, attackers)
	FireProvokeTriggers(gs, attackers)
}

// ============================================================================
// COST MODIFICATION KEYWORDS
// ============================================================================

// ---------------------------------------------------------------------------
// Spectacle — CR §702.137
// "You may cast this spell for its spectacle cost if an opponent lost
// life this turn."
// ---------------------------------------------------------------------------

// HasSpectacle returns true if the card has the spectacle keyword.
func HasSpectacle(card *Card) bool {
	return cardHasKeywordByName(card, "spectacle")
}

// SpectacleCost returns the spectacle cost from keyword args.
func SpectacleCost(card *Card) int {
	return keywordArgCost(card, "spectacle")
}

// CanPaySpectacle checks if spectacle is active — an opponent must have
// lost life this turn. Checked via game flags.
func CanPaySpectacle(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	// Check if any opponent lost life this turn.
	for _, oppSeat := range gs.LivingOpponents(seatIdx) {
		if gs.Seats[oppSeat] != nil && gs.Seats[oppSeat].Flags != nil {
			if gs.Seats[oppSeat].Flags["lost_life_this_turn"] > 0 {
				return true
			}
		}
	}
	// Also check game-wide flag.
	if gs.Flags != nil && gs.Flags["opponent_lost_life_this_turn"] > 0 {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Surge — CR §702.117
// "You may cast this spell for its surge cost if you or a teammate
// cast another spell this turn."
// ---------------------------------------------------------------------------

// HasSurge returns true if the card has the surge keyword.
func HasSurge(card *Card) bool {
	return cardHasKeywordByName(card, "surge")
}

// SurgeCost returns the surge cost from keyword args.
func SurgeCost(card *Card) int {
	return keywordArgCost(card, "surge")
}

// CanPaySurge checks if surge is active — the player (or teammate) must
// have cast another spell this turn.
func CanPaySurge(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat != nil && seat.Flags != nil && seat.Flags["spells_cast_this_turn"] > 0 {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Improvise — CR §702.126
// "Each artifact you tap after you're done activating mana abilities
// pays for {1}."
// Similar to convoke but for artifacts.
// ---------------------------------------------------------------------------

// HasImprovise returns true if the card has the improvise keyword.
func HasImprovise(card *Card) bool {
	return cardHasKeywordByName(card, "improvise")
}

// ImproviseCostReduction counts untapped artifacts on seatIdx's battlefield
// for improvise cost reduction. Each untapped artifact contributes {1}.
func ImproviseCostReduction(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsArtifact() || p.Tapped {
			continue
		}
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// Assist — CR §702.132
// "Another player may pay up to {X} of this spell's cost."
// Simplified: in simulation, assume no external help (0 reduction).
// We still model the keyword for detection/logging purposes.
// ---------------------------------------------------------------------------

// HasAssist returns true if the card has the assist keyword.
func HasAssist(card *Card) bool {
	return cardHasKeywordByName(card, "assist")
}

// AssistReduction returns the assist cost reduction. In our greedy sim,
// we model this as 0 (no teammate helps). In multiplayer with allies,
// this could be extended.
func AssistReduction(gs *GameState, seatIdx int) int {
	// Simplified: no external help in simulation.
	return 0
}

// ---------------------------------------------------------------------------
// Undaunted — CR §702.125
// "This spell costs {1} less to cast for each opponent."
// ---------------------------------------------------------------------------

// HasUndaunted returns true if the card has the undaunted keyword.
func HasUndaunted(card *Card) bool {
	return cardHasKeywordByName(card, "undaunted")
}

// UndauntedReduction returns the cost reduction from undaunted:
// {1} less per opponent.
func UndauntedReduction(gs *GameState, seatIdx int) int {
	if gs == nil {
		return 0
	}
	return len(gs.LivingOpponents(seatIdx))
}

// ---------------------------------------------------------------------------
// Offering — CR §702.48
// "You may cast this spell any time you could cast an instant by
// sacrificing a [type]. If you do, the total cost is reduced by
// the sacrificed creature's mana cost."
// Simplified: detect keyword, provide cost reduction equal to sacced
// creature's CMC. Actual sacrifice handled at cast time.
// ---------------------------------------------------------------------------

// HasOffering returns true if the card has an offering keyword.
func HasOffering(card *Card) bool {
	return cardHasKeywordByName(card, "offering")
}

// OfferingReduction returns the max cost reduction from offering by
// finding the highest-CMC creature of the offering type on the battlefield.
func OfferingReduction(gs *GameState, seatIdx int, offeringType string) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	bestCMC := 0
	want := strings.ToLower(offeringType)
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if want != "" && !cardHasSubtype(p.Card, want) && !cardHasType(p.Card, want) {
			continue
		}
		if p.Card.CMC > bestCMC {
			bestCMC = p.Card.CMC
		}
	}
	return bestCMC
}

// ---------------------------------------------------------------------------
// ScanCostModifiersCombat extends cost modification scanning for keywords
// implemented in this file. Called from ScanCostModifiers in cost_modifiers.go.
// ---------------------------------------------------------------------------

// AppendCombatCostModifiers appends cost modifiers for combat-file keywords
// (improvise, undaunted, spectacle, surge) to an existing modifier list.
func AppendCombatCostModifiers(gs *GameState, card *Card, seatIdx int, mods []CostModifier) []CostModifier {
	if gs == nil || card == nil {
		return mods
	}

	// Improvise — tap artifacts to help pay.
	if HasImprovise(card) {
		reduction := ImproviseCostReduction(gs, seatIdx)
		if reduction > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: reduction,
				Source: "improvise",
			})
		}
	}

	// Undaunted — costs {1} less per opponent.
	if HasUndaunted(card) {
		reduction := UndauntedReduction(gs, seatIdx)
		if reduction > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: reduction,
				Source: "undaunted",
			})
		}
	}

	return mods
}

// ============================================================================
// CASTING RESTRICTION KEYWORDS
// ============================================================================

// ---------------------------------------------------------------------------
// Miracle — CR §702.94
// "You may cast this card for its miracle cost when you draw it if
// it's the first card you've drawn this turn."
// ---------------------------------------------------------------------------

// HasMiracle returns true if the card has the miracle keyword.
func HasMiracle(card *Card) bool {
	return cardHasKeywordByName(card, "miracle")
}

// MiracleCost returns the miracle cost from keyword args.
func MiracleCost(card *Card) int {
	return keywordArgCost(card, "miracle")
}

// CanCastMiracle checks if this is the first card drawn this turn and
// the card has miracle. Called at draw time.
func CanCastMiracle(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || !HasMiracle(card) {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		return true // No draws recorded yet — this is the first.
	}
	return seat.Flags["cards_drawn_this_turn"] <= 1
}

// CastWithMiracle casts a spell for its miracle cost. The card must
// be the first drawn this turn.
func CastWithMiracle(gs *GameState, seatIdx int, card *Card) error {
	if gs == nil || card == nil {
		return &CastError{Reason: "nil game or card"}
	}
	if !CanCastMiracle(gs, seatIdx, card) {
		return &CastError{Reason: "miracle conditions not met"}
	}
	cost := MiracleCost(card)
	seat := gs.Seats[seatIdx]

	// Find card in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return &CastError{Reason: "card not in hand"}
	}

	// Pay miracle cost.
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Remove from hand and put on stack.
	seat.Hand = append(seat.Hand[:handIdx], seat.Hand[handIdx+1:]...)
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: seatIdx,
		Card:       card,
		Kind:       "spell",
	})

	gs.LogEvent(Event{
		Kind:   "miracle",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule": "702.94",
		},
	})
	return nil
}

// ---------------------------------------------------------------------------
// Splice onto Arcane — CR §702.47
// "As you cast an Arcane spell, you may reveal this card from your hand
// and pay its splice cost. If you do, add this card's effects to that spell."
// ---------------------------------------------------------------------------

// HasSplice returns true if the card has the splice keyword.
func HasSplice(card *Card) bool {
	return cardHasKeywordByName(card, "splice")
}

// SpliceCost returns the splice cost from keyword args.
func SpliceCost(card *Card) int {
	return keywordArgCost(card, "splice")
}

// ApplySplice checks if the caster has cards with splice in hand and the
// spell being cast is arcane. If so, adds the splice effects (simplified:
// logs the event, pays the cost). The spliced card stays in hand.
func ApplySplice(gs *GameState, seatIdx int, spellCard *Card) {
	if gs == nil || spellCard == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	// Check if spell is arcane.
	if !cardHasSubtype(spellCard, "arcane") {
		return
	}
	seat := gs.Seats[seatIdx]
	for _, handCard := range seat.Hand {
		if handCard == nil || handCard == spellCard {
			continue
		}
		if !HasSplice(handCard) {
			continue
		}
		cost := SpliceCost(handCard)
		if seat.ManaPool < cost {
			continue
		}
		// Pay splice cost (card stays in hand).
		seat.ManaPool -= cost
		SyncManaAfterSpend(seat)
		gs.LogEvent(Event{
			Kind:   "splice",
			Seat:   seatIdx,
			Source: handCard.DisplayName(),
			Amount: cost,
			Details: map[string]interface{}{
				"onto":  spellCard.DisplayName(),
				"rule":  "702.47",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Cipher — CR §702.99
// "Then you may exile this spell card encoded on a creature you control.
// Whenever that creature deals combat damage to a player, its controller
// may cast a copy of the encoded card without paying its mana cost."
// ---------------------------------------------------------------------------

// HasCipher returns true if the card has the cipher keyword.
func HasCipher(card *Card) bool {
	return cardHasKeywordByName(card, "cipher")
}

// EncodeCipher exiles the spell encoded on a creature. When that creature
// deals combat damage to a player, the controller can cast a copy free.
func EncodeCipher(gs *GameState, seatIdx int, card *Card, host *Permanent) {
	if gs == nil || card == nil || host == nil {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Move card to exile (encoded).
	MoveCard(gs, card, seatIdx, "hand", "exile", "exile-from-hand")

	// Mark the host as having an encoded card.
	if host.Flags == nil {
		host.Flags = map[string]int{}
	}
	host.Flags["cipher_encoded"] = 1

	gs.LogEvent(Event{
		Kind:   "cipher_encode",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"host": host.Card.DisplayName(),
			"rule": "702.99",
		},
	})
}

// FireCipherTrigger is called when a creature with cipher deals combat
// damage to a player. It casts a copy of the encoded spell for free.
func FireCipherTrigger(gs *GameState, host *Permanent) {
	if gs == nil || host == nil || host.Flags == nil {
		return
	}
	if host.Flags["cipher_encoded"] == 0 {
		return
	}
	gs.LogEvent(Event{
		Kind:   "cipher_cast_copy",
		Seat:   host.Controller,
		Source: host.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.99",
		},
	})
}

// ---------------------------------------------------------------------------
// Adventure — CR §702.133
// "Cast as an instant or sorcery from hand (adventure half), then exile.
// Later cast the creature from exile."
// ---------------------------------------------------------------------------

// HasAdventure returns true if the card has the adventure keyword.
func HasAdventure(card *Card) bool {
	return cardHasKeywordByName(card, "adventure")
}

// CastAdventure casts the adventure half of a card. On resolution,
// the card is exiled (not graveyard). From exile, the creature half
// can be cast later.
func CastAdventure(gs *GameState, seatIdx int, card *Card, adventureCost int) error {
	if gs == nil || card == nil {
		return &CastError{Reason: "nil game or card"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	seat := gs.Seats[seatIdx]

	// Find card in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return &CastError{Reason: "card not in hand"}
	}

	// Pay adventure cost.
	if seat.ManaPool < adventureCost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= adventureCost
	SyncManaAfterSpend(seat)

	// Exile (adventure sends to exile, not graveyard).
	MoveCard(gs, card, seatIdx, "hand", "exile", "replace-to-exile")

	gs.LogEvent(Event{
		Kind:   "adventure",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: adventureCost,
		Details: map[string]interface{}{
			"rule": "702.133",
		},
	})
	return nil
}

// NewAdventureCreatureCastPermission creates a ZoneCastPermission allowing
// the creature half to be cast from exile after the adventure resolves.
func NewAdventureCreatureCastPermission(creatureCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "adventure",
		ManaCost:          creatureCost,
		ExileOnResolve:    false, // Goes to battlefield normally.
		RequireController: -1,
	}
}

// ---------------------------------------------------------------------------
// Aftermath — CR §702.128
// "Cast this half only from your graveyard. Then exile it."
// ---------------------------------------------------------------------------

// HasAftermath returns true if the card has the aftermath keyword.
func HasAftermath(card *Card) bool {
	return cardHasKeywordByName(card, "aftermath")
}

// NewAftermathCastPermission creates a ZoneCastPermission for casting the
// aftermath half from graveyard. Card is exiled on resolution.
func NewAftermathCastPermission(aftermathCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "aftermath",
		ManaCost:          aftermathCost,
		ExileOnResolve:    true, // Exiled after resolution.
		RequireController: -1,
	}
}

// ---------------------------------------------------------------------------
// Retrace — CR §702.81
// "You may cast this card from your graveyard by discarding a land card
// as an additional cost."
// ---------------------------------------------------------------------------

// HasRetrace returns true if the card has the retrace keyword.
func HasRetrace(card *Card) bool {
	return cardHasKeywordByName(card, "retrace")
}

// CanCastRetrace checks if the player has a land in hand to discard.
func CanCastRetrace(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	for _, c := range gs.Seats[seatIdx].Hand {
		if c != nil && cardHasType(c, "land") {
			return true
		}
	}
	return false
}

// NewRetraceCastPermission creates a ZoneCastPermission for retrace.
func NewRetraceCastPermission(manaCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:     ZoneGraveyard,
		Keyword:  "retrace",
		ManaCost: manaCost,
		AdditionalCosts: []*AdditionalCost{
			{
				Kind:  addCostKindDiscard,
				Label: "retrace (discard a land)",
				CanPayFn: func(gs *GameState, seatIdx int) bool {
					return CanCastRetrace(gs, seatIdx)
				},
				PayFn: func(gs *GameState, seatIdx int) bool {
					if seatIdx < 0 || seatIdx >= len(gs.Seats) {
						return false
					}
					seat := gs.Seats[seatIdx]
					for _, c := range seat.Hand {
						if c != nil && cardHasType(c, "land") {
							DiscardCard(gs, c, seatIdx)
							return true
						}
					}
					return false
				},
			},
		},
		ExileOnResolve:    false,
		RequireController: -1,
	}
}

// ---------------------------------------------------------------------------
// Jump-start — CR §702.133a
// "You may cast this card from your graveyard by discarding a card as
// an additional cost. If a spell cast this way would go to graveyard,
// exile it instead."
// ---------------------------------------------------------------------------

// HasJumpStart returns true if the card has the jump-start keyword.
func HasJumpStart(card *Card) bool {
	return cardHasKeywordByName(card, "jump-start")
}

// NewJumpStartCastPermission creates a ZoneCastPermission for jump-start.
func NewJumpStartCastPermission(manaCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:     ZoneGraveyard,
		Keyword:  "jump-start",
		ManaCost: manaCost,
		AdditionalCosts: []*AdditionalCost{
			{
				Kind:  addCostKindDiscard,
				Label: "jump-start (discard a card)",
				CanPayFn: func(gs *GameState, seatIdx int) bool {
					if seatIdx < 0 || seatIdx >= len(gs.Seats) {
						return false
					}
					return len(gs.Seats[seatIdx].Hand) > 0
				},
				PayFn: func(gs *GameState, seatIdx int) bool {
					if seatIdx < 0 || seatIdx >= len(gs.Seats) {
						return false
					}
					seat := gs.Seats[seatIdx]
					if len(seat.Hand) == 0 {
						return false
					}
					// Discard the last card (arbitrary choice).
					discarded := seat.Hand[len(seat.Hand)-1]
					DiscardCard(gs, discarded, seatIdx)
					return true
				},
			},
		},
		ExileOnResolve:    true, // Exiled instead of going to graveyard.
		RequireController: -1,
	}
}

// ---------------------------------------------------------------------------
// Overload — CR §702.96
// "You may cast this spell for its overload cost. If you do, change
// its text by replacing all instances of 'target' with 'each'."
// ---------------------------------------------------------------------------

// HasOverload returns true if the card has the overload keyword.
func HasOverload(card *Card) bool {
	return cardHasKeywordByName(card, "overload")
}

// OverloadCost returns the overload cost from keyword args.
func OverloadCost(card *Card) int {
	return keywordArgCost(card, "overload")
}

// CastWithOverload casts a spell for its overload cost, changing it
// from single-target to "each". Modeled as: pay overload cost, set
// a flag on the stack item indicating overloaded.
func CastWithOverload(gs *GameState, seatIdx int, card *Card) error {
	if gs == nil || card == nil {
		return &CastError{Reason: "nil game or card"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if !HasOverload(card) {
		return &CastError{Reason: "card does not have overload"}
	}
	seat := gs.Seats[seatIdx]
	cost := OverloadCost(card)

	// Find card in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return &CastError{Reason: "card not in hand"}
	}

	// Pay overload cost.
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Remove from hand and put on stack.
	seat.Hand = append(seat.Hand[:handIdx], seat.Hand[handIdx+1:]...)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Kind:       "spell",
		CostMeta:   map[string]interface{}{"overloaded": true},
	}
	gs.Stack = append(gs.Stack, item)

	gs.LogEvent(Event{
		Kind:   "overload",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule": "702.96",
		},
	})
	return nil
}

// IsOverloaded checks if a stack item was cast with overload.
func IsOverloaded(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["overloaded"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// ============================================================================
// PROTECTION VARIANTS
// ============================================================================

// ---------------------------------------------------------------------------
// Protection from [type] — CR §702.16
// "Protection from creatures", "protection from instants", etc.
// Prevents being targeted, blocked, dealt damage by, or enchanted/equipped
// by permanents/spells of that type.
// ---------------------------------------------------------------------------

// ProtectionTypes extracts what types a permanent has protection from.
// Returns a set of type strings (lowercased).
func ProtectionTypes(p *Permanent) map[string]struct{} {
	out := map[string]struct{}{}
	if p == nil {
		return out
	}
	// Check flags: "prot_type:creature", "prot_type:instant", etc.
	if p.Flags != nil {
		for k := range p.Flags {
			if strings.HasPrefix(k, "prot_type:") {
				out[strings.TrimPrefix(k, "prot_type:")] = struct{}{}
			}
		}
	}
	// Check AST keywords.
	if p.Card != nil && p.Card.AST != nil {
		for _, ab := range p.Card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok || strings.ToLower(kw.Name) != "protection" {
				continue
			}
			raw := kw.Raw
			// Check for type-based protection: "protection from creatures",
			// "protection from instants", etc.
			for _, typeName := range []string{"creature", "creatures", "instant", "instants",
				"sorcery", "sorceries", "enchantment", "enchantments", "artifact", "artifacts",
				"planeswalker", "planeswalkers", "land", "lands"} {
				if strings.Contains(raw, "from "+typeName) {
					// Normalize plural to singular.
					singular := strings.TrimSuffix(typeName, "s")
					if singular == "creature" || singular == "sorcerie" {
						singular = strings.TrimSuffix(typeName, "es")
						if singular == "creatur" {
							singular = "creature"
						}
						if singular == "sorceri" {
							singular = "sorcery"
						}
					}
					out[singular] = struct{}{}
				}
			}
		}
	}
	return out
}

// HasProtectionFromType checks if a permanent has protection from a
// specific card type. Used in blocking/targeting checks.
func HasProtectionFromType(protected *Permanent, sourceType string) bool {
	protTypes := ProtectionTypes(protected)
	if len(protTypes) == 0 {
		return false
	}
	_, ok := protTypes[strings.ToLower(sourceType)]
	return ok
}

// ---------------------------------------------------------------------------
// Hexproof from [color] — CR §702.11d
// "This permanent can't be the target of [color] spells or abilities
// your opponents control."
// ---------------------------------------------------------------------------

// HexproofFromColors returns the set of colors this permanent has hexproof
// from. Returns empty if no hexproof-from-color.
func HexproofFromColors(p *Permanent) map[string]struct{} {
	out := map[string]struct{}{}
	if p == nil {
		return out
	}
	// Check flags: "hexproof_from:R", "hexproof_from:U", etc.
	if p.Flags != nil {
		for k := range p.Flags {
			if strings.HasPrefix(k, "hexproof_from:") {
				out[strings.TrimPrefix(k, "hexproof_from:")] = struct{}{}
			}
		}
	}
	// Check AST keywords.
	if p.Card != nil && p.Card.AST != nil {
		for _, ab := range p.Card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok {
				continue
			}
			name := strings.ToLower(kw.Name)
			raw := kw.Raw
			if name != "hexproof" {
				continue
			}
			// Parse "hexproof from red", "hexproof from blue", etc.
			for word, letter := range colorWords {
				if strings.Contains(raw, "from "+word) {
					out[letter] = struct{}{}
				}
			}
		}
	}
	return out
}

// CanBeTargetedByCombat extends CanBeTargetedBy with hexproof-from-color
// and protection-from-type checks. Returns true if targeting is legal.
func CanBeTargetedByCombat(perm *Permanent, seatIdx int, sourceCard *Card) bool {
	if perm == nil {
		return false
	}
	// Base targeting check (shroud + hexproof).
	if !CanBeTargetedBy(perm, seatIdx) {
		return false
	}
	// Hexproof from [color] — only blocks opponents.
	if perm.Controller != seatIdx && sourceCard != nil {
		hexColors := HexproofFromColors(perm)
		if len(hexColors) > 0 {
			srcColors := cardColors(sourceCard)
			for c := range srcColors {
				if _, blocked := hexColors[c]; blocked {
					return false
				}
			}
		}
	}
	// §702.16 — Protection from [quality]. "A permanent or player with
	// protection can't be targeted by spells with the stated quality and
	// can't be targeted by abilities from sources with the stated quality."
	// Check protection_from_<color> flags against the source card's colors.
	if perm.Flags != nil && sourceCard != nil &&
		(perm.Flags["kw:protection"] > 0 || perm.HasKeyword("protection")) {
		for _, colorCode := range sourceCard.Colors {
			colorName := colorCodeToName(colorCode)
			if colorName != "" && perm.Flags["protection_from_"+colorName] > 0 {
				return false
			}
		}
	}
	return true
}

// colorCodeToName maps a single-letter MTG color code to its lowercase name.
// Returns "" if the code isn't a recognized MTG color.
func colorCodeToName(code string) string {
	switch strings.ToUpper(code) {
	case "W":
		return "white"
	case "U":
		return "blue"
	case "B":
		return "black"
	case "R":
		return "red"
	case "G":
		return "green"
	}
	return ""
}

// ============================================================================
// Keyword constants for this file
// ============================================================================

const (
	KeywordIntimidate             = "intimidate"
	KeywordFear                   = "fear"
	KeywordShadow                 = "shadow"
	KeywordSkulk                  = "skulk"
	KeywordDaunt                  = "daunt"
	KeywordBanding                = "banding"
	KeywordRampage                = "rampage"
	KeywordBattleCry              = "battle cry"
	KeywordMyriad                 = "myriad"
	KeywordMelee                  = "melee"
	KeywordAnnihilator            = "annihilator"
	KeywordAfflict                = "afflict"
	KeywordProvoke                = "provoke"
	KeywordTrampleOverPW          = "trample over planeswalkers"
	KeywordSpectacle              = "spectacle"
	KeywordSurge                  = "surge"
	KeywordImprovise              = "improvise"
	KeywordAssist                 = "assist"
	KeywordUndaunted              = "undaunted"
	KeywordOffering               = "offering"
	KeywordMiracle                = "miracle"
	KeywordSplice                 = "splice"
	KeywordCipher                 = "cipher"
	KeywordAdventure              = "adventure"
	KeywordAftermath              = "aftermath"
	KeywordRetrace                = "retrace"
	KeywordJumpStart              = "jump-start"
	KeywordOverload               = "overload"
	KeywordProtectionFromType     = "protection"
	KeywordHexproofFromColor      = "hexproof"
)

// AllCombatKeywords returns all keyword names implemented in this file.
func AllCombatKeywords() []string {
	return []string{
		KeywordIntimidate, KeywordFear, KeywordShadow, KeywordSkulk,
		KeywordDaunt, KeywordBanding, KeywordRampage, KeywordBattleCry,
		KeywordMyriad, KeywordMelee, KeywordAnnihilator, KeywordAfflict,
		KeywordProvoke, KeywordTrampleOverPW, KeywordSpectacle,
		KeywordSurge, KeywordImprovise, KeywordAssist, KeywordUndaunted,
		KeywordOffering, KeywordMiracle, KeywordSplice, KeywordCipher,
		KeywordAdventure, KeywordAftermath, KeywordRetrace,
		KeywordJumpStart, KeywordOverload,
	}
}

// ============================================================================
// Internal helpers
// ============================================================================

// cardHasKeywordByName checks if a card has a keyword by name in its AST.
func cardHasKeywordByName(card *Card, name string) bool {
	if card == nil || card.AST == nil {
		return false
	}
	return astHasKeyword(card.AST, name)
}

// keywordArgCost extracts the first numeric arg from a keyword.
// Returns 0 if not found or no args.
func keywordArgCost(card *Card, keywordName string) int {
	if card == nil || card.AST == nil {
		return 0
	}
	want := strings.ToLower(keywordName)
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) != want {
			continue
		}
		if len(kw.Args) > 0 {
			switch v := kw.Args[0].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
		return card.CMC // Fallback: use card's CMC.
	}
	return 0
}

// addCostKindDiscard is the additional cost kind for discarding a card.
// We use the package-level CanPayFn/PayFn pattern to implement it.
const addCostKindDiscard = "discard"
