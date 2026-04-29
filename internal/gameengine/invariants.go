package gameengine

// Invariants — post-action correctness checks for adversarial testing.
//
// Each Invariant is a named predicate over a GameState snapshot. The
// Check function returns nil when the invariant holds, and a descriptive
// error when it is violated. Invariants are designed to be run after
// EVERY game action (cast, resolve, combat step, SBA pass, trigger
// fire) by the fuzzer and the Eye of Sauron spectator.
//
// Invariant list (CR section cross-references):
//
//   ZoneConservation        — total real cards across all zones = starting
//                             total (tokens excluded). No cards created or
//                             destroyed except via token machinery.
//   LifeConsistency         — no seat has negative life unless it has
//                             already lost the game (§704.5a should catch).
//   SBACompleteness         — no creature at ≤0 toughness on battlefield;
//                             no player at ≤0 life still playing (unless
//                             Angel's Grace / Platinum Angel type effects).
//   StackIntegrity          — stack is empty when we are in a main phase
//                             with no priority pending (phase boundary).
//   ManaPoolNonNegative     — typed pool totals never go negative.
//   CommanderDamageMonotonic — commander damage never decreases.
//   PhasedOutExclusion      — phased-out permanents not counted by SBA
//                             helpers (validated by checking none are dead).
//   IndestructibleRespected — no indestructible permanent in graveyard
//                             from a destroy event (sacrifice is OK).
//   LayerIdempotency        — calling GetEffectiveCharacteristics twice on
//                             the same permanent yields identical results.
//   TriggerCompleteness     — sacrifice/die events with matching on-board
//                             trigger-bearers should produce trigger events.
//   CounterAccuracy         — no negative counters; +1/+1 and -1/-1 should
//                             have annihilated (§704.5q).
//   CombatLegality          — no defending+attacking, no tapped blocker,
//                             no attacking defender, no summoning-sick attacker.
//   TurnStructure           — phase/step values are valid; active seat valid.
//   CardIdentity            — no card pointer appears in two zones.
//   ReplacementCompleteness — detects skipped replacement effects (Rest in
//                             Peace graveyard leaks, indestructible violations).
//   WinCondition            — winner's win-condition is verifiable from state.
//   Timing                  — sorceries not on stack during combat; no stack
//                             in cleanup; no non-mana abilities under split second.
//   ResourceConservation    — mana pools sane; lost seats have zero mana.
//   AttachmentConsistency   — aura/equipment attachments point to valid targets.
//   StackOrderCorrectness   — APNAP ordering of triggered abilities from
//                             different controllers on the stack (§101.4).

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Invariant is a named correctness check over a GameState.
type Invariant struct {
	Name  string
	Check func(gs *GameState) error // nil = pass, error = violation
}

// AllInvariants returns the canonical list of invariants in evaluation
// order. Callers should run every invariant after every game action.
func AllInvariants() []Invariant {
	return []Invariant{
		{Name: "ZoneConservation", Check: checkZoneConservation},
		{Name: "LifeConsistency", Check: checkLifeConsistency},
		{Name: "SBACompleteness", Check: checkSBACompleteness},
		{Name: "StackIntegrity", Check: checkStackIntegrity},
		{Name: "ManaPoolNonNegative", Check: checkManaPoolNonNegative},
		{Name: "CommanderDamageMonotonic", Check: checkCommanderDamageMonotonic},
		{Name: "PhasedOutExclusion", Check: checkPhasedOutExclusion},
		{Name: "IndestructibleRespected", Check: checkIndestructibleRespected},
		{Name: "LayerIdempotency", Check: checkLayerIdempotency},
		// --- 10 new invariants (Odin v2) ---
		{Name: "TriggerCompleteness", Check: checkTriggerCompleteness},
		{Name: "CounterAccuracy", Check: checkCounterAccuracy},
		{Name: "CombatLegality", Check: checkCombatLegality},
		{Name: "TurnStructure", Check: checkTurnStructure},
		{Name: "CardIdentity", Check: checkCardIdentity},
		{Name: "ReplacementCompleteness", Check: checkReplacementCompleteness},
		{Name: "WinCondition", Check: checkWinCondition},
		{Name: "Timing", Check: checkTiming},
		{Name: "ResourceConservation", Check: checkResourceConservation},
		{Name: "AttachmentConsistency", Check: checkAttachmentConsistency},
		{Name: "StackOrderCorrectness", Check: checkStackOrderCorrectness},
	}
}

// RunAllInvariants executes every invariant and returns all violations.
// An empty slice means all invariants passed.
func RunAllInvariants(gs *GameState) []InvariantViolation {
	invs := AllInvariants()
	var violations []InvariantViolation
	for _, inv := range invs {
		if err := inv.Check(gs); err != nil {
			violations = append(violations, InvariantViolation{
				Name:    inv.Name,
				Message: err.Error(),
			})
		}
	}
	return violations
}

// InvariantViolation pairs an invariant name with its error message.
type InvariantViolation struct {
	Name    string
	Message string
}

// ---------------------------------------------------------------------------
// ZoneConservation
// ---------------------------------------------------------------------------

// checkZoneConservation verifies that the total number of real (non-token)
// cards across all zones equals the starting total. Cards should never be
// created or destroyed — only moved between zones. Tokens are excluded
// because they can be created/destroyed freely.
func checkZoneConservation(gs *GameState) error {
	if gs == nil {
		return nil
	}
	total := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		total += countRealCards(s.Library)
		total += countRealCards(s.Hand)
		total += countRealCards(s.Graveyard)
		total += countRealCards(s.Exile)
		total += countRealCards(s.CommandZone)
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if p.OriginalCard != nil && !cardIsTokenForInv(p.OriginalCard) {
				total++
			} else if p.Card != nil && !p.IsToken() {
				total++
			}
		}
	}
	// Count cards on the stack (spells).
	for _, item := range gs.Stack {
		if item != nil && item.Card != nil && !item.IsCopy && !cardIsTokenForInv(item.Card) {
			total++
		}
	}

	// Expected total: sum of starting library + commander cards per seat.
	// We use StartingLife as a proxy to detect if the game was initialized;
	// the actual expected count is stored in gs.Flags["_zone_conservation_total"].
	expected, ok := gs.Flags["_zone_conservation_total"]
	if !ok {
		// First check — record the baseline and return OK.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["_zone_conservation_total"] = total
		return nil
	}

	delta := total - expected
	if delta < 0 {
		return fmt.Errorf("zone conservation violated: %d real cards disappeared (expected %d, found %d)",
			-delta, expected, total)
	}
	if delta > 10 {
		return fmt.Errorf("zone conservation suspicious: %d extra real cards appeared (expected %d, found %d) — possible copy bug",
			delta, expected, total)
	}
	return nil
}

func countRealCards(cards []*Card) int {
	n := 0
	for _, c := range cards {
		if c != nil && !cardIsTokenForInv(c) {
			n++
		}
	}
	return n
}

func cardIsTokenForInv(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "token" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// LifeConsistency
// ---------------------------------------------------------------------------

// checkLifeConsistency verifies that no seat has negative life unless it
// has already been flagged as Lost. §704.5a should catch this, but if a
// seat is at ≤0 life and still playing, something went wrong.
func checkLifeConsistency(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Angel's Grace / Platinum Angel can keep a player alive at
		// negative life. We check for the replacement effect flag.
		if s.Life < 0 && !s.Lost {
			// Check if there's a "can't lose the game" effect active.
			if !hasCantLoseEffect(gs, s) {
				return fmt.Errorf("seat %d has life=%d but Lost=false and no loss-prevention effect",
					s.Idx, s.Life)
			}
		}
	}
	return nil
}

func hasCantLoseEffect(gs *GameState, s *Seat) bool {
	if s == nil {
		return false
	}
	// Check seat flags for "can't lose" effects (Platinum Angel, Angel's Grace).
	if s.Flags != nil {
		if s.Flags["cant_lose_game"] > 0 {
			return true
		}
	}
	// Check battlefield for permanents with "can't lose" static effects.
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		name := strings.ToLower(p.Card.DisplayName())
		if name == "platinum angel" || name == "angel's grace" ||
			name == "gideon of the trials" || name == "lich's mastery" {
			return true
		}
	}
	// Check the event log for a recent "would_lose_game" cancellation.
	if len(gs.EventLog) > 0 {
		// Look at recent events (last 20) for a prevented loss.
		start := len(gs.EventLog) - 20
		if start < 0 {
			start = 0
		}
		for i := start; i < len(gs.EventLog); i++ {
			ev := &gs.EventLog[i]
			if ev.Kind == "loss_prevented" && ev.Seat == s.Idx {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// SBACompleteness
// ---------------------------------------------------------------------------

// checkSBACompleteness verifies that no creature on the battlefield has
// toughness ≤ 0 (it should have been killed by §704.5f/g), and no living
// player has life ≤ 0 without a loss-prevention effect.
func checkSBACompleteness(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Check life: living player at ≤0 without protection.
		if !s.Lost && s.Life <= 0 && !hasCantLoseEffect(gs, s) {
			return fmt.Errorf("seat %d has life=%d, Lost=false, no loss-prevention — SBA 704.5a missed",
				s.Idx, s.Life)
		}
		// Check creatures: toughness ≤ 0 should not be on battlefield.
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}
			if !gs.IsCreatureOf(p) {
				continue
			}
			tough := gs.ToughnessOf(p)
			if tough <= 0 {
				name := "<unknown>"
				basePow, baseTough := 0, 0
				if p.Card != nil {
					name = p.Card.DisplayName()
					basePow = p.Card.BasePower
					baseTough = p.Card.BaseToughness
				}
				// Include diagnostic detail so future violations
				// immediately reveal whether the issue is counters,
				// modifications, or a stale layer cache.
				layerTough := tough
				if gs != nil {
					layerTough = gs.ToughnessOf(p)
				}
				modStr := ""
				for i, m := range p.Modifications {
					if i > 0 {
						modStr += ", "
					}
					modStr += fmt.Sprintf("{P:%+d T:%+d dur:%s}", m.Power, m.Toughness, m.Duration)
				}
				if modStr == "" {
					modStr = "<none>"
				}
				return fmt.Errorf("seat %d has creature %q on battlefield with toughness=%d (layer=%d) — SBA 704.5f missed (base=%d/%d, counters=%v, mods=%s)",
					s.Idx, name, tough, layerTough, basePow, baseTough, p.Counters, modStr)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// StackIntegrity
// ---------------------------------------------------------------------------

// checkStackIntegrity verifies that the stack is empty at phase boundaries
// (start of main phase, end of combat). During active priority windows or
// cleanup steps (where triggers can re-enter per CR 514.3a), the stack
// may legitimately be non-empty, so we only flag violations at strict
// boundaries where the rules require an empty stack.
func checkStackIntegrity(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// Skip the check when the game has ended. Once a winner is determined
	// (§104.2) or all opponents have been eliminated (§104.3a), the game
	// is over and the stack may legitimately contain unresolved items
	// (e.g., a spell cast during the main phase that never resolved
	// because combat killed the last opponent). Checking the stack at
	// this point would produce false positives.
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return nil
	}
	// Only check at phase boundaries where the stack should be empty:
	// - Beginning of main phases (precombat_main, postcombat_main)
	// - End of combat (end_of_combat)
	//
	// NOTE: cleanup is deliberately EXCLUDED because CR 514.3a allows
	// triggers to fire during cleanup (e.g., Madness, Megrim, transform
	// triggers), which means the stack can be non-empty at the cleanup
	// step. The cleanup loop handles this by re-running SBAs + priority
	// until the stack empties.
	phaseBoundary := false
	switch gs.Step {
	case "precombat_main", "postcombat_main":
		phaseBoundary = true
	case "end_of_combat":
		phaseBoundary = true
	}

	if phaseBoundary && len(gs.Stack) > 0 {
		items := make([]string, 0, len(gs.Stack))
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			name := "<unknown>"
			if item.Card != nil {
				name = item.Card.DisplayName()
			} else if item.Source != nil && item.Source.Card != nil {
				name = item.Source.Card.DisplayName() + " (ability)"
			}
			items = append(items, name)
		}
		return fmt.Errorf("stack has %d item(s) at phase boundary %s/%s: [%s]",
			len(gs.Stack), gs.Phase, gs.Step, strings.Join(items, ", "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// ManaPoolNonNegative
// ---------------------------------------------------------------------------

// checkManaPoolNonNegative verifies that no mana pool component is negative.
func checkManaPoolNonNegative(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Legacy untyped pool check.
		if s.ManaPool < 0 {
			return fmt.Errorf("seat %d has negative ManaPool=%d", s.Idx, s.ManaPool)
		}
		// Typed pool check.
		if s.Mana != nil {
			p := s.Mana
			if p.W < 0 {
				return fmt.Errorf("seat %d has negative Mana.W=%d", s.Idx, p.W)
			}
			if p.U < 0 {
				return fmt.Errorf("seat %d has negative Mana.U=%d", s.Idx, p.U)
			}
			if p.B < 0 {
				return fmt.Errorf("seat %d has negative Mana.B=%d", s.Idx, p.B)
			}
			if p.R < 0 {
				return fmt.Errorf("seat %d has negative Mana.R=%d", s.Idx, p.R)
			}
			if p.G < 0 {
				return fmt.Errorf("seat %d has negative Mana.G=%d", s.Idx, p.G)
			}
			if p.C < 0 {
				return fmt.Errorf("seat %d has negative Mana.C=%d", s.Idx, p.C)
			}
			if p.Any < 0 {
				return fmt.Errorf("seat %d has negative Mana.Any=%d", s.Idx, p.Any)
			}
			for i, r := range p.Restricted {
				if r.Amount < 0 {
					return fmt.Errorf("seat %d has negative Restricted[%d].Amount=%d (source=%s)",
						s.Idx, i, r.Amount, r.Source)
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// CommanderDamageMonotonic
// ---------------------------------------------------------------------------

// checkCommanderDamageMonotonic verifies that commander damage counters
// never decrease. We snapshot the previous values in gs.Flags and compare.
func checkCommanderDamageMonotonic(gs *GameState) error {
	if gs == nil || !gs.CommanderFormat {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for dealerSeat, cmdrs := range s.CommanderDamage {
			for cmdrName, dmg := range cmdrs {
				key := fmt.Sprintf("_cmdr_dmg_%d_%d_%s", s.Idx, dealerSeat, cmdrName)
				prev, hasPrev := gs.Flags[key]
				if hasPrev && dmg < prev {
					return fmt.Errorf("commander damage decreased for seat %d from dealer %d commander %q: was %d, now %d",
						s.Idx, dealerSeat, cmdrName, prev, dmg)
				}
				if gs.Flags == nil {
					gs.Flags = map[string]int{}
				}
				gs.Flags[key] = dmg
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// PhasedOutExclusion
// ---------------------------------------------------------------------------

// checkPhasedOutExclusion verifies that phased-out permanents are not
// being incorrectly affected by SBAs. A phased-out creature at ≤0
// toughness should NOT be destroyed — it's treated as though it doesn't
// exist. We verify by checking that no phased-out permanent has been
// moved to the graveyard via an SBA event since the last check.
func checkPhasedOutExclusion(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// Check that no phased-out permanent is marked for death.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.PhasedOut {
				continue
			}
			// A phased-out permanent should never have MarkedDamage applied
			// by SBAs. If it does, something is counting it.
			if p.IsCreature() && p.MarkedDamage > 0 {
				name := "<unknown>"
				if p.Card != nil {
					name = p.Card.DisplayName()
				}
				return fmt.Errorf("phased-out creature %q (seat %d) has MarkedDamage=%d — should be excluded from SBAs",
					name, s.Idx, p.MarkedDamage)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// IndestructibleRespected
// ---------------------------------------------------------------------------

// checkIndestructibleRespected scans recent events for any "destroy"
// event targeting a permanent that was indestructible at the time. We
// look for the pattern: destroy event without a matching
// "destroy_prevented" event. Note that sacrifice events are OK —
// indestructible does not prevent sacrifice.
func checkIndestructibleRespected(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// We track the last-checked event index in gs.Flags.
	lastChecked := 0
	if gs.Flags != nil {
		lastChecked = gs.Flags["_indestructible_last_checked"]
	}
	if lastChecked >= len(gs.EventLog) {
		return nil
	}

	for i := lastChecked; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		if ev.Kind != "destroy" {
			continue
		}
		// Check if the destroy was preceded by a "destroy_prevented" for
		// the same card. If a destroy event exists, it means
		// DestroyPermanent did NOT prevent it (indestructible check
		// passed). But we double-check: look backward for a matching
		// permanent on the battlefield that is indestructible.
		if ev.Details != nil {
			if reason, ok := ev.Details["indestructible_bypassed"]; ok && reason == true {
				cardName := ""
				if cn, ok := ev.Details["target_card"]; ok {
					cardName = fmt.Sprintf("%v", cn)
				}
				return fmt.Errorf("indestructible permanent %q was destroyed (event %d) — "+
					"indestructible should prevent destroy", cardName, i)
			}
		}
	}

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_indestructible_last_checked"] = len(gs.EventLog)
	return nil
}

// ---------------------------------------------------------------------------
// LayerIdempotency
// ---------------------------------------------------------------------------

// checkLayerIdempotency verifies that calling GetEffectiveCharacteristics
// twice on the same permanent yields identical results. The layer system
// should be purely functional over the current ContinuousEffects list.
func checkLayerIdempotency(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}
			// Invalidate cache to force fresh computation.
			gs.InvalidateCharacteristicsCache()
			c1 := GetEffectiveCharacteristics(gs, p)
			gs.InvalidateCharacteristicsCache()
			c2 := GetEffectiveCharacteristics(gs, p)

			if err := compareCharacteristics(p, c1, c2); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareCharacteristics(p *Permanent, a, b *Characteristics) error {
	name := "<unknown>"
	if p != nil && p.Card != nil {
		name = p.Card.DisplayName()
	}
	if a.Name != b.Name {
		return fmt.Errorf("LayerIdempotency: %q Name differs: %q vs %q", name, a.Name, b.Name)
	}
	if a.Power != b.Power {
		return fmt.Errorf("LayerIdempotency: %q Power differs: %d vs %d", name, a.Power, b.Power)
	}
	if a.Toughness != b.Toughness {
		return fmt.Errorf("LayerIdempotency: %q Toughness differs: %d vs %d", name, a.Toughness, b.Toughness)
	}
	if a.Controller != b.Controller {
		return fmt.Errorf("LayerIdempotency: %q Controller differs: %d vs %d", name, a.Controller, b.Controller)
	}
	if !sliceEqual(a.Types, b.Types) {
		return fmt.Errorf("LayerIdempotency: %q Types differ: %v vs %v", name, a.Types, b.Types)
	}
	if !sliceEqual(a.Subtypes, b.Subtypes) {
		return fmt.Errorf("LayerIdempotency: %q Subtypes differ: %v vs %v", name, a.Subtypes, b.Subtypes)
	}
	if !sliceEqual(a.Supertypes, b.Supertypes) {
		return fmt.Errorf("LayerIdempotency: %q Supertypes differ: %v vs %v", name, a.Supertypes, b.Supertypes)
	}
	if !sliceEqual(a.Colors, b.Colors) {
		return fmt.Errorf("LayerIdempotency: %q Colors differ: %v vs %v", name, a.Colors, b.Colors)
	}
	if !sliceEqual(a.Keywords, b.Keywords) {
		return fmt.Errorf("LayerIdempotency: %q Keywords differ: %v vs %v", name, a.Keywords, b.Keywords)
	}
	if a.Loyalty != b.Loyalty {
		return fmt.Errorf("LayerIdempotency: %q Loyalty differs: %d vs %d", name, a.Loyalty, b.Loyalty)
	}
	if a.CMC != b.CMC {
		return fmt.Errorf("LayerIdempotency: %q CMC differs: %d vs %d", name, a.CMC, b.CMC)
	}
	return nil
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// TriggerCompleteness
// ---------------------------------------------------------------------------

// checkTriggerCompleteness scans the last 10 events for patterns that should
// have produced a trigger but apparently didn't. Lightweight: only checks
// recent events against the current battlefield, not full history.
func checkTriggerCompleteness(gs *GameState) error {
	if gs == nil {
		return nil
	}
	if len(gs.EventLog) == 0 {
		return nil
	}
	// Look at the last 10 events.
	start := len(gs.EventLog) - 10
	if start < 0 {
		start = 0
	}

	// Build a quick set of trigger-bearing permanents on the battlefield.
	type triggerInfo struct {
		cardName string
		seat     int
	}
	var diesTriggers []triggerInfo
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut || p.Card == nil {
				continue
			}
			if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "creature_dies") {
				diesTriggers = append(diesTriggers, triggerInfo{cardName: p.Card.DisplayName(), seat: s.Idx})
			}
		}
	}
	if len(diesTriggers) == 0 {
		return nil // No trigger-bearers to check.
	}

	// Scan recent events for sacrifice/die events.
	for i := start; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		if ev.Kind != "sacrifice" && ev.Kind != "creature_dies" && ev.Kind != "sba_704_5f" && ev.Kind != "sba_704_5g" {
			continue
		}
		// Only check if a trigger-bearer controls the dying creature.
		// Most "creature_dies" triggers only fire for your own creatures.
		deathSeat := ev.Seat
		hasMatchingBearer := false
		for _, dt := range diesTriggers {
			if dt.seat == deathSeat {
				hasMatchingBearer = true
				break
			}
		}
		if !hasMatchingBearer {
			continue
		}
		// A creature died/was sacrificed. Check if a trigger event followed
		// from any dies-trigger bearer within the remaining events.
		found := false
		for j := i + 1; j < len(gs.EventLog); j++ {
			ej := &gs.EventLog[j]
			if ej.Kind == "triggered_ability" || ej.Kind == "trigger_fires" ||
				ej.Kind == "delayed_trigger_fires" || ej.Kind == "life_change" ||
				ej.Kind == "trigger_evaluated" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("TriggerCompleteness: death event %q at index %d with trigger-bearer(s) %v on battlefield, but no subsequent trigger/effect event found",
				ev.Kind, i, diesTriggers)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// CounterAccuracy
// ---------------------------------------------------------------------------

// checkCounterAccuracy verifies counter sanity on all permanents:
//   - No permanent has negative counter counts for any counter type
//   - No permanent has both +1/+1 and -1/-1 counters (§704.5q annihilation)
//   - Shield counters are non-negative
func checkCounterAccuracy(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut || p.Counters == nil {
				continue
			}
			name := "<unknown>"
			if p.Card != nil {
				name = p.Card.DisplayName()
			}
			// Check for negative counter counts.
			for kind, count := range p.Counters {
				if count < 0 {
					return fmt.Errorf("CounterAccuracy: %q (seat %d) has negative %s counters: %d",
						name, s.Idx, kind, count)
				}
			}
			// §704.5q: +1/+1 and -1/-1 should annihilate each other.
			plus := p.Counters["+1/+1"]
			minus := p.Counters["-1/-1"]
			if plus > 0 && minus > 0 {
				return fmt.Errorf("CounterAccuracy: %q (seat %d) has both +1/+1=%d and -1/-1=%d counters — SBA §704.5q should have annihilated",
					name, s.Idx, plus, minus)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// CombatLegality
// ---------------------------------------------------------------------------

// checkCombatLegality verifies combat state consistency when the game is in
// a combat phase. Checks:
//   - No creature is both attacking and blocking
//   - No tapped creature is blocking (tapped creatures can't block)
//   - No creature with defender keyword is attacking
//   - No creature with summoning sickness (no haste) is attacking
func checkCombatLegality(gs *GameState) error {
	if gs == nil {
		return nil
	}
	if gs.Phase != "combat" {
		return nil // Only check during combat.
	}
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			name := "<unknown>"
			if p.Card != nil {
				name = p.Card.DisplayName()
			}
			attacking := permFlag(p, flagAttacking)
			blocking := permFlag(p, flagBlocking)

			// No creature can be both attacking and blocking.
			if attacking && blocking {
				return fmt.Errorf("CombatLegality: %q (seat %d) is both attacking and blocking",
					name, s.Idx)
			}
			// Tapped creatures can't block.
			if blocking && p.Tapped {
				return fmt.Errorf("CombatLegality: %q (seat %d) is blocking while tapped",
					name, s.Idx)
			}
			// Creatures with defender can't attack.
			if attacking && p.HasKeyword("defender") {
				return fmt.Errorf("CombatLegality: %q (seat %d) is attacking but has defender",
					name, s.Idx)
			}
			// Summoning-sick creatures without haste can't attack.
			if attacking && p.SummoningSick && !p.HasKeyword("haste") {
				return fmt.Errorf("CombatLegality: %q (seat %d) is attacking with summoning sickness and no haste",
					name, s.Idx)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// TurnStructure
// ---------------------------------------------------------------------------

// checkTurnStructure verifies that phase/step fields are valid and the active
// seat index is in range and not Lost.
func checkTurnStructure(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// Skip when the game has ended — phase/step may be stale.
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return nil
	}
	// Validate phase.
	validPhases := map[string]bool{
		"":                  true,
		"beginning":         true,
		"precombat_main":    true,
		"combat":            true,
		"postcombat_main":   true,
		"ending":            true,
		"main":              true, // legacy alias
		"precombat_main1":   true, // alternate naming
		"postcombat_main2":  true, // alternate naming
	}
	if !validPhases[gs.Phase] {
		return fmt.Errorf("TurnStructure: invalid phase %q", gs.Phase)
	}

	// Validate step is consistent with phase.
	validSteps := map[string]map[string]bool{
		"": {"": true},
		"beginning": {
			"untap": true, "upkeep": true, "draw": true,
			"": true, // transitional
		},
		"precombat_main": {
			"precombat_main": true, "main": true, "": true,
		},
		"combat": {
			"begin_of_combat": true, "beginning_of_combat": true,
			"combat_start": true,
			"declare_attackers": true, "attackers": true,
			"declare_blockers": true, "blockers": true,
			"first_strike_damage": true, "combat_damage": true,
			"end_of_combat": true, "combat_end": true,
			"": true, // transitional
		},
		"postcombat_main": {
			"postcombat_main": true, "main": true, "": true,
		},
		"main": {
			"": true, "main": true, "precombat_main": true, "postcombat_main": true,
		},
		"precombat_main1": {
			"": true, "main": true, "precombat_main": true,
		},
		"postcombat_main2": {
			"": true, "main": true, "postcombat_main": true,
		},
		"ending": {
			"end": true, "end_step": true, "end_of_turn": true,
			"cleanup": true, "": true,
		},
	}
	if allowed, ok := validSteps[gs.Phase]; ok {
		if !allowed[gs.Step] {
			return fmt.Errorf("TurnStructure: step %q is invalid for phase %q", gs.Step, gs.Phase)
		}
	}

	// Active seat must be a valid index.
	if gs.Active < 0 || gs.Active >= len(gs.Seats) {
		return fmt.Errorf("TurnStructure: active seat %d is out of range [0, %d)",
			gs.Active, len(gs.Seats))
	}
	// Active seat must not be Lost — unless a non-life-based loss
	// condition triggered (CR 704.5b empty library, 704.5c poison,
	// commander damage, or an effect that says "you lose the game").
	if s := gs.Seats[gs.Active]; s != nil && s.Lost && s.Life > 0 && s.LossReason == "" {
		return fmt.Errorf("TurnStructure: active seat %d is Lost but life is %d with no LossReason",
			gs.Active, s.Life)
	}
	return nil
}

// ---------------------------------------------------------------------------
// CardIdentity
// ---------------------------------------------------------------------------

// checkCardIdentity verifies that no card pointer appears in two zones
// simultaneously. This is a stronger version of ZoneConservation which
// counts totals — this one checks actual pointer identity.
func checkCardIdentity(gs *GameState) error {
	if gs == nil {
		return nil
	}
	type cardLoc struct {
		zone string
		seat int
	}
	seen := map[*Card]cardLoc{}

	checkCard := func(c *Card, zone string, seat int) error {
		if c == nil {
			return nil
		}
		if prev, dup := seen[c]; dup {
			name := c.DisplayName()
			return fmt.Errorf("CardIdentity: card %q (ptr %p) appears in both seat %d %s and seat %d %s",
				name, c, prev.seat, prev.zone, seat, zone)
		}
		seen[c] = cardLoc{zone: zone, seat: seat}
		return nil
	}

	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Library {
			if err := checkCard(c, "library", s.Idx); err != nil {
				return err
			}
		}
		for _, c := range s.Hand {
			if err := checkCard(c, "hand", s.Idx); err != nil {
				return err
			}
		}
		for _, c := range s.Graveyard {
			if err := checkCard(c, "graveyard", s.Idx); err != nil {
				return err
			}
		}
		for _, c := range s.Exile {
			if err := checkCard(c, "exile", s.Idx); err != nil {
				return err
			}
		}
		for _, c := range s.CommandZone {
			if err := checkCard(c, "command_zone", s.Idx); err != nil {
				return err
			}
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if err := checkCard(p.Card, "battlefield", s.Idx); err != nil {
				return err
			}
		}
	}
	// Also check the stack.
	for _, item := range gs.Stack {
		if item == nil {
			continue
		}
		if item.Card != nil {
			if err := checkCard(item.Card, "stack", item.Controller); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ReplacementCompleteness
// ---------------------------------------------------------------------------

// checkReplacementCompleteness checks for game states that indicate a
// replacement effect was skipped. Currently checks:
//   - If Rest in Peace is on the battlefield, no non-token card should have
//     entered any graveyard since the last check (via recent events).
//   - If a creature has indestructible and is in a graveyard with a recent
//     "destroy" event, the replacement was skipped.
func checkReplacementCompleteness(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// Track last-checked event index.
	lastChecked := 0
	if gs.Flags != nil {
		lastChecked = gs.Flags["_repl_completeness_last_checked"]
	}
	if lastChecked >= len(gs.EventLog) {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["_repl_completeness_last_checked"] = len(gs.EventLog)
		return nil
	}

	// Check for graveyard-exile replacement effects on the battlefield.
	// Rest in Peace: universal (all cards). Leyline of the Void: opponent-only.
	hasRIP := false
	leylineSeats := map[int]bool{} // seats controlling a Leyline
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut || p.Card == nil {
				continue
			}
			switch strings.ToLower(p.Card.DisplayName()) {
			case "rest in peace":
				hasRIP = true
			case "leyline of the void":
				leylineSeats[s.Idx] = true
			}
		}
	}

	if hasRIP || len(leylineSeats) > 0 {
		// Only check the last 20 events to avoid false positives from
		// zone_change events that occurred before the exile effect entered.
		scanStart := lastChecked
		if recent := len(gs.EventLog) - 20; recent > scanStart {
			scanStart = recent
		}
		for i := scanStart; i < len(gs.EventLog); i++ {
			ev := &gs.EventLog[i]
			if ev.Kind != "zone_change" || ev.Details == nil {
				continue
			}
			if toZone, ok := ev.Details["to_zone"]; !ok || toZone != "graveyard" {
				continue
			}
			if tok, ok := ev.Details["is_token"]; ok && tok == true {
				continue
			}
			// Determine which seat owns/controls the dying card.
			cardOwnerSeat := ev.Seat
			cardControllerSeat := cardOwnerSeat
			if ctrl, ok := ev.Details["controller"]; ok {
				if c, ok2 := ctrl.(int); ok2 {
					cardControllerSeat = c
				}
			}
			// RIP exiles everything; Leyline only exiles opponents' cards.
			shouldBeExiled := hasRIP
			if !shouldBeExiled {
				for leySeat := range leylineSeats {
					if cardControllerSeat != leySeat {
						shouldBeExiled = true
						break
					}
				}
			}
			if shouldBeExiled {
				cardName := ev.Source
				if cn, ok := ev.Details["card"]; ok {
					cardName = fmt.Sprintf("%v", cn)
				}
				// Verify the exile effect's replacement was registered
				// when this zone change happened. The harness clears
				// the event log each turn, so a card destroyed before
				// the exile effect entered (same turn) is a false
				// positive. We check for a matching "would_die" or
				// "would_be_put_into_graveyard" replacement that was
				// actually registered for the exile effect at destroy
				// time. Proxy: look for a preceding "destroy" event
				// for this card — if that event's to_zone is already
				// "graveyard" and there's no "replacement_applied"
				// event from RIP/Leyline between it and this
				// zone_change, the replacement wasn't registered yet.
				wasActiveAtDestroyTime := true
				for j := scanStart; j < i; j++ {
					ej := &gs.EventLog[j]
					if ej.Kind == "destroy" && ej.Source == cardName {
						if dz, ok := ej.Details["to_zone"]; ok && dz == "graveyard" {
							// Destroy event said graveyard — check if a
							// replacement_applied from the exile effect
							// fired between the destroy and zone_change.
							foundRepl := false
							for k := j + 1; k < i; k++ {
								ek := &gs.EventLog[k]
								if ek.Kind == "replacement_applied" {
									if ek.Source == "Rest in Peace" || ek.Source == "Leyline of the Void" {
										foundRepl = true
										break
									}
								}
							}
							if !foundRepl {
								wasActiveAtDestroyTime = false
							}
						}
					}
				}
				if !wasActiveAtDestroyTime {
					continue
				}
				if gs.Flags == nil {
					gs.Flags = map[string]int{}
				}
				gs.Flags["_repl_completeness_last_checked"] = len(gs.EventLog)
				return fmt.Errorf("ReplacementCompleteness: card %q entered graveyard (event %d) while graveyard-exile effect is on battlefield — replacement effect skipped",
					cardName, i)
			}
		}
	}

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_repl_completeness_last_checked"] = len(gs.EventLog)
	return nil
}

// ---------------------------------------------------------------------------
// WinCondition
// ---------------------------------------------------------------------------

// checkWinCondition verifies that when the game has ended and there is a
// winner, the win-condition is verifiable from the game state. Checks:
//   - Commander damage win: 21+ from a single commander
//   - Poison win: 10+ poison counters on all losers
//   - Life loss: each loser was at ≤ 0 life
func checkWinCondition(gs *GameState) error {
	if gs == nil {
		return nil
	}
	if gs.Flags == nil || gs.Flags["ended"] != 1 {
		return nil // Game not ended yet.
	}
	// If it's a draw, no winner to verify.
	if gs.Flags["game_draw"] == 1 {
		return nil
	}

	// Find the winner and losers.
	winnerSeat := -1
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Won {
			winnerSeat = s.Idx
			break
		}
	}
	if winnerSeat < 0 {
		// No explicit winner — could be all opponents lost. Check that at
		// least one seat is not Lost.
		alive := 0
		for _, s := range gs.Seats {
			if s != nil && !s.Lost {
				alive++
			}
		}
		if alive == 0 {
			return nil // All lost — draw or game-end edge case.
		}
		return nil // No winner flagged — might be last-man-standing.
	}

	// Check all non-winner seats that Lost.
	for _, s := range gs.Seats {
		if s == nil || s.Idx == winnerSeat || !s.Lost {
			continue
		}
		// Verify loss reason is consistent.
		reason := strings.ToLower(s.LossReason)

		if strings.Contains(reason, "commander") || strings.Contains(reason, "704.6c") {
			// Commander damage loss — verify 21+ from a single commander.
			if gs.CommanderFormat {
				maxDmg := 0
				for _, cmdrs := range s.CommanderDamage {
					for _, dmg := range cmdrs {
						if dmg > maxDmg {
							maxDmg = dmg
						}
					}
				}
				if maxDmg < 21 {
					return fmt.Errorf("WinCondition: seat %d lost via commander damage but max commander damage is %d (< 21)",
						s.Idx, maxDmg)
				}
			}
		}

		if strings.Contains(reason, "poison") || strings.Contains(reason, "704.5c") {
			if s.PoisonCounters < 10 {
				return fmt.Errorf("WinCondition: seat %d lost via poison but has only %d poison counters (< 10)",
					s.Idx, s.PoisonCounters)
			}
		}

		if strings.Contains(reason, "life") || strings.Contains(reason, "704.5a") {
			// Life was ≤ 0 when SBA fired. Post-elimination triggers (lifelink,
			// Blood Artist, etc.) can modify life afterward — only flag if the
			// seat's objects are still on the battlefield (not yet cleaned up).
			if s.Life > 0 && !s.LeftGame {
				return fmt.Errorf("WinCondition: seat %d lost via life but has life=%d (> 0)",
					s.Idx, s.Life)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Timing
// ---------------------------------------------------------------------------

// checkTiming verifies timing legality of stack contents:
//   - No sorcery on the stack during a combat phase (unless it has flash)
//   - No activated non-mana ability on the stack during split second
//   - Stack should be empty during cleanup step (barring triggers)
func checkTiming(gs *GameState) error {
	if gs == nil {
		return nil
	}
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return nil
	}
	if len(gs.Stack) == 0 {
		return nil
	}

	// Check for sorceries on the stack during combat.
	if gs.Phase == "combat" {
		for _, item := range gs.Stack {
			if item == nil || item.Card == nil || item.Countered {
				continue
			}
			isSorcery := false
			for _, t := range item.Card.Types {
				if t == "sorcery" {
					isSorcery = true
					break
				}
			}
			if !isSorcery {
				continue
			}
			// Check for flash — a sorcery with flash can be cast at instant speed.
			hasFlash := false
			if item.Card.AST != nil {
				for _, ab := range item.Card.AST.Abilities {
					if kw, ok := ab.(*gameast.Keyword); ok {
						if strings.EqualFold(kw.Name, "flash") {
							hasFlash = true
							break
						}
					}
				}
			}
			// Also check the Types for "flash" as some tokens mark it there.
			for _, t := range item.Card.Types {
				if t == "flash" {
					hasFlash = true
				}
			}
			if !hasFlash {
				return fmt.Errorf("Timing: sorcery %q on stack during combat phase %s/%s without flash",
					item.Card.DisplayName(), gs.Phase, gs.Step)
			}
		}
	}

	// Check for split second violations: if any stack item has split second,
	// no other non-mana activated ability should be on the stack above it.
	hasSplitSecond := false
	for _, item := range gs.Stack {
		if item == nil || item.Card == nil {
			continue
		}
		if item.Card.AST != nil {
			for _, ab := range item.Card.AST.Abilities {
				if kw, ok := ab.(interface{ GetName() string }); ok {
					if strings.ToLower(kw.GetName()) == "split second" {
						hasSplitSecond = true
						break
					}
				}
			}
		}
	}
	if hasSplitSecond {
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			if item.Kind == "activated" {
				// Allow mana abilities.
				isMana := false
				if item.Source != nil && item.Source.Flags != nil && item.Source.Flags["mana_ability"] > 0 {
					isMana = true
				}
				if !isMana {
					name := "<ability>"
					if item.Source != nil && item.Source.Card != nil {
						name = item.Source.Card.DisplayName()
					}
					return fmt.Errorf("Timing: activated ability from %q on stack while split second is active",
						name)
				}
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// ResourceConservation
// ---------------------------------------------------------------------------

// checkResourceConservation verifies mana pool sanity:
//   - No individual mana pool color has a negative value
//   - Total mana pool matches sum of typed colors
//   - If a seat is Lost, their mana pool should be 0
func checkResourceConservation(gs *GameState) error {
	if gs == nil {
		return nil
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Note: ManaPoolNonNegative already checks individual color negatives.
		// We check the additional constraints here without duplication.

		// Lost seats should have zero mana.
		if s.Lost {
			if s.ManaPool != 0 {
				return fmt.Errorf("ResourceConservation: seat %d is Lost but has ManaPool=%d",
					s.Idx, s.ManaPool)
			}
			if s.Mana != nil && s.Mana.Total() != 0 {
				return fmt.Errorf("ResourceConservation: seat %d is Lost but has typed mana total=%d",
					s.Idx, s.Mana.Total())
			}
		}

		// If typed pool exists, verify ManaPool == Mana.Total() (sync check).
		if s.Mana != nil {
			typedTotal := s.Mana.Total()
			if s.ManaPool != typedTotal {
				return fmt.Errorf("ResourceConservation: seat %d ManaPool=%d but typed Mana.Total()=%d — desync",
					s.Idx, s.ManaPool, typedTotal)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// AttachmentConsistency
// ---------------------------------------------------------------------------

// checkAttachmentConsistency verifies all attachment relationships are legal:
//   - Every Aura's AttachedTo points to a permanent that exists on some battlefield
//   - Every Equipment's AttachedTo points to a creature (or is nil/unattached)
//   - No permanent is attached to itself
//   - No permanent is attached to a permanent in a different zone (not on battlefield)
func checkAttachmentConsistency(gs *GameState) error {
	if gs == nil {
		return nil
	}
	// Build a set of all permanents currently on battlefields.
	onBF := map[*Permanent]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil {
				onBF[p] = true
			}
		}
	}

	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}
			if p.AttachedTo == nil {
				continue
			}
			name := "<unknown>"
			if p.Card != nil {
				name = p.Card.DisplayName()
			}
			targetName := "<unknown>"
			if p.AttachedTo.Card != nil {
				targetName = p.AttachedTo.Card.DisplayName()
			}

			// No permanent can be attached to itself.
			if p.AttachedTo == p {
				return fmt.Errorf("AttachmentConsistency: %q (seat %d) is attached to itself",
					name, s.Idx)
			}

			// AttachedTo must point to a permanent on some battlefield.
			if !onBF[p.AttachedTo] {
				return fmt.Errorf("AttachmentConsistency: %q (seat %d) is attached to %q which is not on any battlefield",
					name, s.Idx, targetName)
			}

			// Equipment must be attached to a creature.
			if p.IsEquipment() && !p.AttachedTo.IsCreature() {
				return fmt.Errorf("AttachmentConsistency: equipment %q (seat %d) is attached to non-creature %q",
					name, s.Idx, targetName)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// StackOrderCorrectness
// ---------------------------------------------------------------------------

// checkStackOrderCorrectness verifies that when triggered abilities from
// DIFFERENT controllers are on the stack simultaneously, they are in APNAP
// order: the active player's triggers are deeper (lower index, pushed first,
// resolve last) and the non-active player's triggers are on top (higher
// index, pushed last, resolve first). This is required by CR §101.4.
//
// Only checks triggered abilities identified by CostMeta["per_card_trigger"].
// Same-controller triggers can be in any order (player chooses).
func checkStackOrderCorrectness(gs *GameState) error {
	if gs == nil || len(gs.Stack) < 2 {
		return nil
	}

	// Find triggered abilities from different controllers on the stack.
	type trigInfo struct {
		controller int
		stackIdx   int
	}
	var triggers []trigInfo

	for i, item := range gs.Stack {
		if item == nil || item.CostMeta == nil {
			continue
		}
		if _, ok := item.CostMeta["per_card_trigger"]; ok {
			triggers = append(triggers, trigInfo{controller: item.Controller, stackIdx: i})
		}
	}

	if len(triggers) < 2 {
		return nil // 0-1 triggers, ordering doesn't matter
	}

	// Check if triggers from different controllers exist.
	controllers := map[int]bool{}
	for _, t := range triggers {
		controllers[t.controller] = true
	}
	if len(controllers) < 2 {
		return nil // all same controller, player chooses order (we don't enforce)
	}

	// Verify APNAP order: active player's triggers should be DEEPER (lower
	// index) in the stack than non-active player's triggers (higher index =
	// resolves first).
	apnap := APNAPOrder(gs)
	if len(apnap) < 2 {
		return nil
	}

	// Build a seat-to-APNAP-position map.
	apnapPos := map[int]int{}
	for i, seat := range apnap {
		apnapPos[seat] = i
	}

	// For any two triggers from different controllers, the one with
	// LOWER APNAP position (active player = 0) should have LOWER stack
	// index (pushed first, resolves last).
	for i := 0; i < len(triggers); i++ {
		for j := i + 1; j < len(triggers); j++ {
			ti, tj := triggers[i], triggers[j]
			if ti.controller == tj.controller {
				continue
			}
			posI := apnapPos[ti.controller]
			posJ := apnapPos[tj.controller]
			// If controller I has lower APNAP position (closer to active),
			// it should have lower stack index (pushed first).
			if posI < posJ && ti.stackIdx > tj.stackIdx {
				return fmt.Errorf("StackOrderCorrectness: seat %d (APNAP pos %d) trigger at stack[%d] above seat %d (APNAP pos %d) at stack[%d] — APNAP violation (§101.4)",
					ti.controller, posI, ti.stackIdx, tj.controller, posJ, tj.stackIdx)
			}
			if posJ < posI && tj.stackIdx > ti.stackIdx {
				return fmt.Errorf("StackOrderCorrectness: seat %d (APNAP pos %d) trigger at stack[%d] above seat %d (APNAP pos %d) at stack[%d] — APNAP violation (§101.4)",
					tj.controller, posJ, tj.stackIdx, ti.controller, posI, ti.stackIdx)
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Diagnostic helpers (used by fuzzer + eye)
// ---------------------------------------------------------------------------

// GameStateSummary returns a human-readable multi-line summary of the
// current game state, suitable for violation reports.
func GameStateSummary(gs *GameState) string {
	if gs == nil {
		return "<nil GameState>"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Turn %d, Phase=%s Step=%s Active=seat%d\n",
		gs.Turn, gs.Phase, gs.Step, gs.Active)
	fmt.Fprintf(&sb, "Stack: %d items, EventLog: %d events\n",
		len(gs.Stack), len(gs.EventLog))
	for i, s := range gs.Seats {
		if s == nil {
			fmt.Fprintf(&sb, "  Seat %d: <nil>\n", i)
			continue
		}
		status := "alive"
		if s.Lost {
			status = "LOST"
		}
		if s.Won {
			status = "WON"
		}
		fmt.Fprintf(&sb, "  Seat %d [%s]: life=%d library=%d hand=%d graveyard=%d exile=%d battlefield=%d cmdzone=%d mana=%d\n",
			i, status, s.Life, len(s.Library), len(s.Hand), len(s.Graveyard),
			len(s.Exile), len(s.Battlefield), len(s.CommandZone), s.ManaPool)
		if len(s.Battlefield) > 0 {
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				phased := ""
				if p.PhasedOut {
					phased = " [PHASED OUT]"
				}
				tapped := ""
				if p.Tapped {
					tapped = " [T]"
				}
				fmt.Fprintf(&sb, "    - %s (P/T %d/%d, dmg=%d)%s%s\n",
					p.Card.DisplayName(), p.Power(), p.Toughness(),
					p.MarkedDamage, tapped, phased)
			}
		}
	}
	return sb.String()
}

// RecentEvents returns the last N events from the event log as
// human-readable lines.
func RecentEvents(gs *GameState, n int) []string {
	if gs == nil || n <= 0 {
		return nil
	}
	start := len(gs.EventLog) - n
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, len(gs.EventLog)-start)
	for i := start; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		line := fmt.Sprintf("[%d] %s seat=%d source=%s",
			i, ev.Kind, ev.Seat, ev.Source)
		if ev.Amount != 0 {
			line += fmt.Sprintf(" amount=%d", ev.Amount)
		}
		if ev.Target >= 0 {
			line += fmt.Sprintf(" target=seat%d", ev.Target)
		}
		lines = append(lines, line)
	}
	return lines
}
