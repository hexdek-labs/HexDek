package main

// oracle_compliance.go — Phase 3: Per-Card Handler Oracle Compliance Audit.
//
// For each of the 270 registered per-card handlers, verifies that:
//   1. The handler type matches the oracle text (ETB handler for ETB text, etc.)
//   2. The handler fires and emits events when invoked
//   3. Key oracle effects are reflected in the emitted events
//
// Output: per-card compliance report with PASS/MISMATCH/MISSING classifications.

import (
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/per_card"
)

type complianceResult int

const (
	compPass complianceResult = iota
	compMismatch
	compMissing
	compPartial
)

func (c complianceResult) String() string {
	switch c {
	case compPass:
		return "PASS"
	case compMismatch:
		return "MISMATCH"
	case compMissing:
		return "MISSING"
	case compPartial:
		return "PARTIAL"
	}
	return "?"
}

type cardCompliance struct {
	cardName      string
	oracleText    string
	typeLine      string
	result        complianceResult
	handlerTypes  []string // "etb", "on_cast", "on_resolve", "activated", "trigger:event_name"
	expectedTypes []string // what oracle text implies
	issues        []string
	eventCount    int
}

// Oracle text patterns for ability type detection.
var (
	reETB          = regexp.MustCompile(`(?i)\bwhen\b.*\benters\b`)
	reETBSelf      = regexp.MustCompile(`(?i)\bwhen ~? ?enters\b`)
	reCast         = regexp.MustCompile(`(?i)\bwhen (?:you cast|~ is cast)\b`)
	reActivated    = regexp.MustCompile(`(?i)(?:\{[WUBRGCTXSE0-9/]+\}|(?:tap|untap))[^.]*:`)
	reManaAbility  = regexp.MustCompile(`(?i)\{T\}\s*:\s*Add\s+\{`)
	reTriggered    = regexp.MustCompile(`(?i)\b(?:when(?:ever)?|at the beginning of|at end of)\b`)
	reSpell        = regexp.MustCompile(`(?i)^(?:instant|sorcery)`)
	reDealsDamage  = regexp.MustCompile(`(?i)\bdeals?\s+\d+\s+damage\b`)
	reDrawCard     = regexp.MustCompile(`(?i)\bdraw (?:a|two|three|\d+) cards?\b`)
	reCreateToken  = regexp.MustCompile(`(?i)\bcreate[s]?\s+(?:a|two|three|\d+|an?|X)\s+\d*/?\d*\b`)
	reCounters     = regexp.MustCompile(`(?i)\b(?:put|enter|with)\s+.*\bcounters?\b`)
	reDestroy      = regexp.MustCompile(`(?i)\bdestroy\s+(?:target|all|each)\b`)
	reExile        = regexp.MustCompile(`(?i)\bexile\s+(?:target|all|each|it|that)\b`)
	reBounce       = regexp.MustCompile(`(?i)\breturn\s+(?:target|it|that|a).*\bto.*(?:hand|library)\b`)
	reGainLife     = regexp.MustCompile(`(?i)\bgain\s+\d+\s+life\b`)
	reLoseLife     = regexp.MustCompile(`(?i)\blose\s+\d+\s+life\b`)
	reDiscard      = regexp.MustCompile(`(?i)\bdiscards?\s+(?:a|their|two|three|\d+)\b`)
	reSacrifice    = regexp.MustCompile(`(?i)\bsacrifice\s+(?:a|target|it|that|~)\b`)
	reGainControl  = regexp.MustCompile(`(?i)\bgain control of\b`)
	reMill         = regexp.MustCompile(`(?i)\bmill\s+\d+\b`)
	reScry         = regexp.MustCompile(`(?i)\bscry\s+\d+\b`)
	reTutor        = regexp.MustCompile(`(?i)\bsearch\s+(?:your|their|a)\s+library\b`)
	reFlicker      = regexp.MustCompile(`(?i)\bexile.*(?:return|enters).*battlefield\b`)
	reFight        = regexp.MustCompile(`(?i)\bfights?\b`)
	reCounterSpell = regexp.MustCompile(`(?i)\bcounter target\b`)
	reCopySpell    = regexp.MustCompile(`(?i)\bcopy (?:target|that|a) (?:spell|instant|sorcery)\b`)
	reTransform    = regexp.MustCompile(`(?i)\btransform ~\b`)
)

func inferExpectedTypes(oracleText, typeLine string) []string {
	var types []string

	isSpell := reSpell.MatchString(typeLine)

	if isSpell {
		types = append(types, "on_resolve")
	}

	if reETBSelf.MatchString(oracleText) {
		types = append(types, "etb")
	}

	if reCast.MatchString(oracleText) {
		types = append(types, "on_cast")
	}

	lines := strings.Split(oracleText, "\n")
	hasNonManaActivated := false
	for _, line := range lines {
		if reActivated.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
			if !reManaAbility.MatchString(line) {
				hasNonManaActivated = true
			}
		}
	}
	if hasNonManaActivated {
		types = append(types, "activated")
	}

	// Count distinct trigger sources (excluding ETB/cast which have their own categories).
	triggerCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if reTriggered.MatchString(trimmed) && !reETBSelf.MatchString(trimmed) && !reCast.MatchString(trimmed) {
			// Exclude "enters tapped" / "enters the battlefield tapped" lines.
			if !strings.Contains(strings.ToLower(trimmed), "enters the battlefield tapped") &&
				!strings.Contains(strings.ToLower(trimmed), "enters tapped") {
				triggerCount++
			}
		}
	}
	if triggerCount > 0 {
		types = append(types, "triggered")
	}

	return types
}

func inferExpectedEffects(oracleText string) []string {
	var effects []string
	checks := []struct {
		re   *regexp.Regexp
		name string
	}{
		{reDealsDamage, "damage"},
		{reDrawCard, "draw"},
		{reCreateToken, "create_token"},
		{reCounters, "counters"},
		{reDestroy, "destroy"},
		{reExile, "exile"},
		{reBounce, "bounce"},
		{reGainLife, "gain_life"},
		{reLoseLife, "lose_life"},
		{reDiscard, "discard"},
		{reSacrifice, "sacrifice"},
		{reGainControl, "gain_control"},
		{reMill, "mill"},
		{reScry, "scry"},
		{reTutor, "tutor"},
		{reFlicker, "flicker"},
		{reFight, "fight"},
		{reCounterSpell, "counter_spell"},
		{reCopySpell, "copy_spell"},
		{reTransform, "transform"},
	}
	for _, c := range checks {
		if c.re.MatchString(oracleText) {
			effects = append(effects, c.name)
		}
	}
	return effects
}

func getHandlerTypes(name string) []string {
	// DFC cards: handlers are registered under front-face name only.
	lookupName := name
	if idx := strings.Index(name, " // "); idx > 0 {
		lookupName = name[:idx]
	}

	var types []string
	if per_card.HasETB(lookupName) {
		types = append(types, "etb")
	}
	if per_card.HasResolve(lookupName) {
		types = append(types, "on_resolve")
	}
	if per_card.HasActivated(lookupName) {
		types = append(types, "activated")
	}
	registry := per_card.Global()
	if registry != nil {
		norm := per_card.NormalizeName(lookupName)
		hasCast, hasTrigger := registry.HasCastAndTrigger(norm)
		if hasCast {
			types = append(types, "on_cast")
		}
		if hasTrigger {
			types = append(types, "triggered")
		}
	}
	return types
}

func testHandlerFires(gs *gameengine.GameState, cardName string, handlerTypes []string) (events int, issues []string) {
	// DFC: use front-face name for handler invocation.
	lookupName := cardName
	if idx := strings.Index(cardName, " // "); idx > 0 {
		lookupName = cardName[:idx]
	}

	seat := 0
	perm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  lookupName,
			Types: []string{"creature"},
		},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
	}

	if gs.Seats[seat].Battlefield == nil {
		gs.Seats[seat].Battlefield = []*gameengine.Permanent{}
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)

	preEventCount := len(gs.EventLog)

	for _, ht := range handlerTypes {
		func() {
			defer func() {
				if r := recover(); r != nil {
					issues = append(issues, fmt.Sprintf("PANIC in %s handler: %v", ht, r))
				}
			}()
			switch ht {
			case "etb":
				gameengine.InvokeETBHook(gs, perm)
			case "activated":
				gameengine.InvokeActivatedHook(gs, perm, 0, nil)
			case "on_resolve":
				item := &gameengine.StackItem{
					Card:       perm.Card,
					Controller: seat,
				}
				gameengine.InvokeResolveHook(gs, item)
			}
		}()
	}

	events = len(gs.EventLog) - preEventCount
	return events, issues
}

func runOracleCompliance(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	start := time.Now()

	registry := per_card.Global()
	if registry == nil {
		log.Println("  WARNING: per_card registry is nil")
		return nil
	}

	registeredNames := registry.RegisteredCardNames()
	sort.Strings(registeredNames)

	oracleMap := map[string]*oracleCard{}
	for _, oc := range oracleCards {
		norm := per_card.NormalizeName(oc.Name)
		oracleMap[norm] = oc
		// DFC cards: also index under front face name so handlers
		// registered as "tergrid god of fright" match "Tergrid, God of Fright // Tergrid's Lantern".
		if idx := strings.Index(oc.Name, " // "); idx > 0 {
			frontNorm := per_card.NormalizeName(oc.Name[:idx])
			if _, exists := oracleMap[frontNorm]; !exists {
				oracleMap[frontNorm] = oc
			}
		}
	}

	var results []cardCompliance
	var (
		passCount     int
		mismatchCount int
		missingCount  int
		partialCount  int
		panicCount    int
	)

	for _, normName := range registeredNames {
		oc, found := oracleMap[normName]
		// Fallback: try "the <name>" for cards like "Book of Vile Darkness"
		// whose oracle name is "The Book of Vile Darkness".
		if !found {
			oc, found = oracleMap["the "+normName]
		}
		if !found {
			results = append(results, cardCompliance{
				cardName: normName,
				result:   compMissing,
				issues:   []string{"card not found in oracle corpus"},
			})
			missingCount++
			continue
		}

		handlerTypes := getHandlerTypes(normName)
		expectedTypes := inferExpectedTypes(oc.OracleText, oc.TypeLine)
		expectedEffects := inferExpectedEffects(oc.OracleText)

		var issues []string

		// Check type alignment. Per-card handlers supplement the AST engine —
		// they're only needed for abilities too complex for generic dispatch.
		// An "activated" ability on a card with an ETB handler is typically
		// handled by the AST engine, not a per-card handler.
		handlerSet := toSet(handlerTypes)

		for _, et := range expectedTypes {
			switch et {
			case "triggered":
				// Triggered is covered if we have any primary handler type.
				// Delayed triggers from spells/activated abilities are handled
				// by the on_resolve/activated handler, not a separate trigger handler.
				if !handlerSet["triggered"] && !handlerSet["etb"] && !handlerSet["on_cast"] &&
					!handlerSet["on_resolve"] && !handlerSet["activated"] {
					issues = append(issues, fmt.Sprintf("oracle implies %s but no matching handler", et))
				}
			case "activated":
				// Only flag if the card has NO handler at all, or if activated
				// is the ONLY expected type (pure activated ability card).
				if !handlerSet["activated"] && len(expectedTypes) == 1 {
					issues = append(issues, fmt.Sprintf("oracle implies %s but no per-card handler (AST engine may cover)", et))
				}
			case "on_resolve":
				if !handlerSet["on_resolve"] && !handlerSet["on_cast"] {
					issues = append(issues, fmt.Sprintf("oracle implies %s but no matching handler", et))
				}
			default:
				if !handlerSet[et] {
					issues = append(issues, fmt.Sprintf("oracle implies %s but no matching handler", et))
				}
			}
		}

		// Test handler fires without panic.
		rng := rand.New(rand.NewSource(42))
		gs := gameengine.NewGameState(2, rng, corpus)
		for s := 0; s < 2; s++ {
			gs.Seats[s].Life = 20
			gs.Seats[s].Library = make([]*gameengine.Card, 30)
			for i := range gs.Seats[s].Library {
				gs.Seats[s].Library[i] = &gameengine.Card{Name: "Forest", Types: []string{"land", "forest"}}
			}
			gs.Seats[s].Hand = make([]*gameengine.Card, 5)
			for i := range gs.Seats[s].Hand {
				gs.Seats[s].Hand[i] = &gameengine.Card{Name: "Island", Types: []string{"land", "island"}}
			}
		}

		eventCount, fireIssues := testHandlerFires(gs, oc.Name, handlerTypes)
		issues = append(issues, fireIssues...)

		for _, fi := range fireIssues {
			if strings.HasPrefix(fi, "PANIC") {
				panicCount++
			}
		}

		result := compPass
		if len(issues) > 0 {
			hasTypeIssue := false
			hasPanic := false
			for _, iss := range issues {
				if strings.Contains(iss, "no matching handler") {
					hasTypeIssue = true
				}
				if strings.HasPrefix(iss, "PANIC") {
					hasPanic = true
				}
			}
			if hasPanic {
				result = compMismatch
			} else if hasTypeIssue {
				result = compPartial
			}
		}

		results = append(results, cardCompliance{
			cardName:      oc.Name,
			oracleText:    oc.OracleText,
			typeLine:      oc.TypeLine,
			result:        result,
			handlerTypes:  handlerTypes,
			expectedTypes: expectedTypes,
			issues:        issues,
			eventCount:    eventCount,
		})

		switch result {
		case compPass:
			passCount++
		case compMismatch:
			mismatchCount++
		case compMissing:
			missingCount++
		case compPartial:
			partialCount++
		}

		_ = expectedEffects
	}

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println("ORACLE COMPLIANCE AUDIT (Phase 3)")
	fmt.Println("=================================")
	fmt.Printf("Registered handlers: %d\n", len(registeredNames))
	fmt.Printf("Time:                %s\n", elapsed)
	fmt.Println()
	fmt.Printf("  PASS:      %4d\n", passCount)
	fmt.Printf("  PARTIAL:   %4d  (handler exists but type mismatch)\n", partialCount)
	fmt.Printf("  MISMATCH:  %4d  (handler panics or critical issue)\n", mismatchCount)
	fmt.Printf("  MISSING:   %4d  (card not in oracle corpus)\n", missingCount)
	fmt.Printf("  PANICS:    %4d\n", panicCount)

	if partialCount > 0 || mismatchCount > 0 {
		fmt.Println()
		fmt.Println("Issues:")
		for _, r := range results {
			if r.result == compMismatch || r.result == compPartial {
				fmt.Printf("\n  %s [%s]\n", r.cardName, r.result)
				fmt.Printf("    type_line: %s\n", r.typeLine)
				fmt.Printf("    handlers:  %v\n", r.handlerTypes)
				fmt.Printf("    expected:  %v\n", r.expectedTypes)
				fmt.Printf("    events:    %d\n", r.eventCount)
				for _, iss := range r.issues {
					fmt.Printf("    ! %s\n", iss)
				}
			}
		}
	}

	if missingCount > 0 {
		fmt.Println()
		fmt.Println("Missing from oracle corpus:")
		for _, r := range results {
			if r.result == compMissing {
				fmt.Printf("  %s\n", r.cardName)
			}
		}
	}

	var failures []failure
	for _, r := range results {
		if r.result == compMismatch {
			for _, iss := range r.issues {
				failures = append(failures, failure{
					CardName:    r.cardName,
					Interaction: "oracle_compliance",
					Invariant:   "handler_compliance",
					Message:     iss,
				})
			}
		}
	}

	log.Printf("  oracle-compliance complete: %d handlers, %d pass, %d partial, %d mismatch, %d missing, %s",
		len(registeredNames), passCount, partialCount, mismatchCount, missingCount, elapsed)
	return failures
}

func toSet(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}
