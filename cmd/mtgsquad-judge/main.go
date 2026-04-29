// mtgsquad-judge — Interactive rules-engine REPL for adversarial testing.
//
// Construct board states by hand and verify engine behavior. The judge
// loads the full card corpus so permanents get real AST data, continuous
// effects, and layer calculations.
//
// Usage:
//
//	go run ./cmd/mtgsquad-judge/ [--ast data/rules/ast_dataset.jsonl] [--oracle data/rules/oracle-cards.json]
//
// Commands:
//
//	create game --seats N            fresh game state
//	seat N add_permanent "Card"      add a permanent to battlefield
//	seat N add_to_hand "Card"        add card to hand
//	seat N set_life N                set life total
//	seat N cast "Card" [targeting ...] cast a spell
//	resolve                          resolve top of stack
//	sba                              run state-based actions
//	query seat N permanent "Name" characteristics
//	query seat N life
//	query seat N hand
//	query seat N battlefield
//	query layers                     show all continuous effects
//	assert <condition>               verify condition PASS/FAIL
//	step                             advance one game step
//	events                           show recent event log
//	invariants                       run all invariants
//	state                            dump full game state
//	help                             show commands
//	exit / quit                      exit
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

var (
	corpus *astload.Corpus
	meta   *deckparser.MetaDB
	gs     *gameengine.GameState
)

func main() {
	var (
		astPath    = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
	)
	flag.Parse()

	// Load corpus.
	fmt.Println("mtgsquad-judge — Interactive MTG Rules Engine REPL")
	fmt.Println("Loading card corpus...")
	t0 := time.Now()
	var err error
	corpus, err = astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	meta, err = deckparser.LoadMetaFromJSONL(*astPath)
	if err != nil {
		log.Fatalf("deckparser meta: %v", err)
	}
	if *oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			fmt.Printf("  oracle supplement: %v (continuing without)\n", err)
		}
	}
	fmt.Printf("  %d cards loaded in %s\n", corpus.Count(), time.Since(t0).Round(time.Millisecond))
	fmt.Println("Type 'help' for commands. Type 'exit' to quit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("mtgsquad-judge> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		execCommand(line)
	}
}

func execCommand(line string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("  ERROR (panic): %v\n", r)
		}
	}()

	tokens := tokenize(line)
	if len(tokens) == 0 {
		return
	}

	switch tokens[0] {
	case "help":
		printHelp()
	case "create":
		cmdCreate(tokens[1:])
	case "seat":
		cmdSeat(tokens[1:])
	case "query":
		cmdQuery(tokens[1:])
	case "resolve":
		cmdResolve()
	case "sba":
		cmdSBA()
	case "assert":
		cmdAssert(tokens[1:])
	case "step":
		cmdStep()
	case "events":
		cmdEvents(tokens[1:])
	case "invariants":
		cmdInvariants()
	case "state":
		cmdState()
	default:
		fmt.Printf("  unknown command: %s (type 'help' for commands)\n", tokens[0])
	}
}

// tokenize splits a line into tokens, respecting quoted strings.
func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		} else if ch == '"' || ch == '\'' {
			inQuote = true
			quoteChar = ch
		} else if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func requireGame() bool {
	if gs == nil {
		fmt.Println("  No game created. Use 'create game --seats N' first.")
		return false
	}
	return true
}

func parseSeat(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Printf("  invalid seat number: %s\n", s)
		return 0, false
	}
	if gs == nil || n < 0 || n >= len(gs.Seats) {
		fmt.Printf("  seat %d out of range\n", n)
		return 0, false
	}
	return n, true
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func cmdCreate(args []string) {
	if len(args) < 1 || args[0] != "game" {
		fmt.Println("  usage: create game --seats N")
		return
	}
	seats := 4
	for i := 1; i < len(args); i++ {
		if args[i] == "--seats" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fmt.Printf("  invalid seats: %s\n", args[i+1])
				return
			}
			seats = n
			i++
		}
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	gs = gameengine.NewGameState(seats, rng, corpus)
	gs.CommanderFormat = false
	// Set reasonable defaults.
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
	}
	gs.Phase = "main"
	gs.Step = "precombat_main"
	fmt.Printf("  Game created with %d seats (40 life each)\n", seats)
}

func cmdSeat(args []string) {
	if !requireGame() || len(args) < 2 {
		fmt.Println("  usage: seat N <action> [args...]")
		return
	}
	seatIdx, ok := parseSeat(args[0])
	if !ok {
		return
	}
	action := args[1]
	rest := args[2:]

	switch action {
	case "add_permanent":
		cmdAddPermanent(seatIdx, rest)
	case "add_to_hand":
		cmdAddToHand(seatIdx, rest)
	case "set_life":
		cmdSetLife(seatIdx, rest)
	case "cast":
		cmdCast(seatIdx, rest)
	default:
		fmt.Printf("  unknown seat action: %s\n", action)
		fmt.Println("  valid: add_permanent, add_to_hand, set_life, cast")
	}
}

func cmdAddPermanent(seatIdx int, args []string) {
	if len(args) < 1 {
		fmt.Println("  usage: seat N add_permanent \"Card Name\"")
		return
	}
	cardName := strings.Join(args, " ")
	perm := createPermanent(seatIdx, cardName)
	if perm == nil {
		return
	}
	gs.Seats[seatIdx].Battlefield = append(gs.Seats[seatIdx].Battlefield, perm)

	// Register continuous effects and replacements (for layer-active cards).
	gameengine.RegisterContinuousEffectsForPermanent(gs, perm)
	gameengine.RegisterReplacementsForPermanent(gs, perm)

	gs.LogEvent(gameengine.Event{
		Kind:   "judge_add_permanent",
		Seat:   seatIdx,
		Source: cardName,
	})
	fmt.Printf("  Added %s to seat %d battlefield\n", cardName, seatIdx)
}

func cmdAddToHand(seatIdx int, args []string) {
	if len(args) < 1 {
		fmt.Println("  usage: seat N add_to_hand \"Card Name\"")
		return
	}
	cardName := strings.Join(args, " ")
	card := createCard(seatIdx, cardName)
	if card == nil {
		return
	}
	gs.Seats[seatIdx].Hand = append(gs.Seats[seatIdx].Hand, card)
	fmt.Printf("  Added %s to seat %d hand\n", cardName, seatIdx)
}

func cmdSetLife(seatIdx int, args []string) {
	if len(args) < 1 {
		fmt.Println("  usage: seat N set_life N")
		return
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("  invalid life total: %s\n", args[0])
		return
	}
	gs.Seats[seatIdx].Life = n
	fmt.Printf("  Seat %d life set to %d\n", seatIdx, n)
}

func cmdCast(seatIdx int, args []string) {
	if len(args) < 1 {
		fmt.Println("  usage: seat N cast \"Card Name\" [targeting seat M permanent \"Name\"]")
		return
	}

	// Parse card name (everything before "targeting").
	cardNameParts := []string{}
	targetArgs := []string{}
	inTargeting := false
	for _, a := range args {
		if strings.ToLower(a) == "targeting" {
			inTargeting = true
			continue
		}
		if inTargeting {
			targetArgs = append(targetArgs, a)
		} else {
			cardNameParts = append(cardNameParts, a)
		}
	}
	cardName := strings.Join(cardNameParts, " ")

	// Create the card.
	card := createCard(seatIdx, cardName)
	if card == nil {
		return
	}

	// Parse targets.
	var targets []gameengine.Target
	if len(targetArgs) > 0 {
		t, ok := parseTarget(targetArgs)
		if ok {
			targets = append(targets, t)
		}
	}

	// Push onto stack.
	item := &gameengine.StackItem{
		Controller: seatIdx,
		Card:       card,
		Kind:       "spell",
		Targets:    targets,
	}
	gameengine.PushStackItem(gs, item)
	fmt.Printf("  %s pushed onto stack (stack size: %d)\n", cardName, len(gs.Stack))
}

func parseTarget(args []string) (gameengine.Target, bool) {
	// Parse: seat M permanent "Name"
	// Parse: seat M (player target)
	if len(args) < 2 || strings.ToLower(args[0]) != "seat" {
		fmt.Println("  target format: seat N [permanent \"Name\"]")
		return gameengine.Target{}, false
	}
	seatIdx, err := strconv.Atoi(args[1])
	if err != nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		fmt.Printf("  invalid target seat: %s\n", args[1])
		return gameengine.Target{}, false
	}

	if len(args) >= 4 && strings.ToLower(args[2]) == "permanent" {
		permName := strings.Join(args[3:], " ")
		// Find the permanent.
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p != nil && p.Card != nil &&
				strings.EqualFold(p.Card.DisplayName(), permName) {
				return gameengine.Target{
					Kind:      gameengine.TargetKindPermanent,
					Permanent: p,
					Seat:      seatIdx,
				}, true
			}
		}
		fmt.Printf("  permanent %q not found on seat %d battlefield\n", permName, seatIdx)
		return gameengine.Target{}, false
	}

	// Player target.
	return gameengine.Target{
		Kind: gameengine.TargetKindSeat,
		Seat: seatIdx,
	}, true
}

func cmdResolve() {
	if !requireGame() {
		return
	}
	if len(gs.Stack) == 0 {
		fmt.Println("  Stack is empty, nothing to resolve")
		return
	}
	top := gs.Stack[len(gs.Stack)-1]
	name := "<unknown>"
	if top.Card != nil {
		name = top.Card.DisplayName()
	} else if top.Source != nil && top.Source.Card != nil {
		name = top.Source.Card.DisplayName() + " (ability)"
	}
	fmt.Printf("  Resolving: %s\n", name)
	gameengine.ResolveStackTop(gs)
	fmt.Printf("  Stack size: %d\n", len(gs.Stack))
}

func cmdSBA() {
	if !requireGame() {
		return
	}
	changed := gameengine.StateBasedActions(gs)
	if changed {
		fmt.Println("  SBA: changes occurred")
	} else {
		fmt.Println("  SBA: no changes")
	}
}

func cmdQuery(args []string) {
	if !requireGame() || len(args) < 1 {
		fmt.Println("  usage: query seat N <what> | query layers")
		return
	}

	if args[0] == "layers" {
		cmdQueryLayers()
		return
	}

	if len(args) < 3 || args[0] != "seat" {
		fmt.Println("  usage: query seat N permanent|life|hand|battlefield [...]")
		return
	}
	seatIdx, ok := parseSeat(args[1])
	if !ok {
		return
	}
	what := args[2]

	switch what {
	case "life":
		fmt.Printf("  Seat %d life: %d\n", seatIdx, gs.Seats[seatIdx].Life)
	case "hand":
		seat := gs.Seats[seatIdx]
		fmt.Printf("  Seat %d hand (%d cards):\n", seatIdx, len(seat.Hand))
		for i, c := range seat.Hand {
			if c == nil {
				continue
			}
			fmt.Printf("    [%d] %s\n", i, c.DisplayName())
		}
	case "battlefield":
		seat := gs.Seats[seatIdx]
		fmt.Printf("  Seat %d battlefield (%d permanents):\n", seatIdx, len(seat.Battlefield))
		for i, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			chars := gameengine.GetEffectiveCharacteristics(gs, p)
			phased := ""
			if p.PhasedOut {
				phased = " [PHASED OUT]"
			}
			tapped := ""
			if p.Tapped {
				tapped = " [T]"
			}
			fmt.Printf("    [%d] %s — types: %s, P/T: %d/%d, colors: %s%s%s\n",
				i, chars.Name,
				strings.Join(chars.Types, " "),
				chars.Power, chars.Toughness,
				strings.Join(chars.Colors, ","),
				tapped, phased)
		}
	case "permanent":
		if len(args) < 4 {
			fmt.Println("  usage: query seat N permanent \"Name\" [characteristics]")
			return
		}
		// Find what comes after "permanent".
		permNameParts := []string{}
		showChars := false
		for i := 3; i < len(args); i++ {
			if strings.ToLower(args[i]) == "characteristics" {
				showChars = true
				continue
			}
			permNameParts = append(permNameParts, args[i])
		}
		permName := strings.Join(permNameParts, " ")
		found := false
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if strings.EqualFold(p.Card.DisplayName(), permName) {
				found = true
				chars := gameengine.GetEffectiveCharacteristics(gs, p)
				if showChars {
					fmt.Printf("  types: %s, P/T: %d/%d, abilities: [%s], colors: [%s], keywords: [%s]\n",
						strings.Join(chars.Types, " "),
						chars.Power, chars.Toughness,
						formatAbilities(chars),
						strings.Join(chars.Colors, ", "),
						strings.Join(chars.Keywords, ", "))
				} else {
					fmt.Printf("  %s — types: %s, P/T: %d/%d, dmg=%d, tapped=%v, counters=%v\n",
						chars.Name,
						strings.Join(chars.Types, " "),
						chars.Power, chars.Toughness,
						p.MarkedDamage, p.Tapped, p.Counters)
				}
				break
			}
		}
		if !found {
			fmt.Printf("  NOT FOUND (%s not on seat %d battlefield)\n", permName, seatIdx)
		}
	case "graveyard":
		seat := gs.Seats[seatIdx]
		fmt.Printf("  Seat %d graveyard (%d cards):\n", seatIdx, len(seat.Graveyard))
		for i, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			fmt.Printf("    [%d] %s\n", i, c.DisplayName())
		}
	case "exile":
		seat := gs.Seats[seatIdx]
		fmt.Printf("  Seat %d exile (%d cards):\n", seatIdx, len(seat.Exile))
		for i, c := range seat.Exile {
			if c == nil {
				continue
			}
			fmt.Printf("    [%d] %s\n", i, c.DisplayName())
		}
	case "library":
		seat := gs.Seats[seatIdx]
		fmt.Printf("  Seat %d library: %d cards\n", seatIdx, len(seat.Library))
	default:
		fmt.Printf("  unknown query target: %s\n", what)
		fmt.Println("  valid: permanent, life, hand, battlefield, graveyard, exile, library")
	}
}

func cmdQueryLayers() {
	fmt.Printf("  Continuous effects: %d\n", len(gs.ContinuousEffects))
	for i, ce := range gs.ContinuousEffects {
		if ce == nil {
			continue
		}
		srcName := ce.SourceCardName
		if srcName == "" && ce.SourcePerm != nil && ce.SourcePerm.Card != nil {
			srcName = ce.SourcePerm.Card.DisplayName()
		}
		sublayer := ""
		if ce.Sublayer != "" {
			sublayer = ce.Sublayer
		}
		fmt.Printf("    [%d] layer=%d%s timestamp=%d source=%q duration=%s handler=%s\n",
			i, ce.Layer, sublayer, ce.Timestamp, srcName, ce.Duration, ce.HandlerID)
	}
}

func cmdAssert(args []string) {
	if !requireGame() || len(args) < 1 {
		fmt.Println("  usage: assert seat N graveyard contains \"Card\"")
		fmt.Println("  usage: assert seat N life <op> N")
		fmt.Println("  usage: assert seat N battlefield contains \"Card\"")
		fmt.Println("  usage: assert seat N battlefield empty")
		fmt.Println("  usage: assert stack empty")
		return
	}

	full := strings.Join(args, " ")

	// assert stack empty
	if strings.HasPrefix(strings.ToLower(full), "stack empty") {
		if len(gs.Stack) == 0 {
			fmt.Println("  PASS (stack is empty)")
		} else {
			fmt.Printf("  FAIL (stack has %d items)\n", len(gs.Stack))
		}
		return
	}

	// assert seat N ...
	if len(args) < 4 || strings.ToLower(args[0]) != "seat" {
		fmt.Println("  invalid assert format")
		return
	}
	seatIdx, ok := parseSeat(args[1])
	if !ok {
		return
	}
	zone := strings.ToLower(args[2])

	switch {
	case zone == "graveyard" && len(args) >= 5 && strings.ToLower(args[3]) == "contains":
		cardName := strings.Join(args[4:], " ")
		found := false
		for _, c := range gs.Seats[seatIdx].Graveyard {
			if c != nil && strings.EqualFold(c.DisplayName(), cardName) {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("  PASS (%s in seat %d graveyard)\n", cardName, seatIdx)
		} else {
			fmt.Printf("  FAIL (%s NOT in seat %d graveyard)\n", cardName, seatIdx)
		}

	case zone == "battlefield" && len(args) >= 5 && strings.ToLower(args[3]) == "contains":
		cardName := strings.Join(args[4:], " ")
		found := false
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p != nil && p.Card != nil && strings.EqualFold(p.Card.DisplayName(), cardName) {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("  PASS (%s on seat %d battlefield)\n", cardName, seatIdx)
		} else {
			fmt.Printf("  FAIL (%s NOT on seat %d battlefield)\n", cardName, seatIdx)
		}

	case zone == "battlefield" && len(args) >= 4 && strings.ToLower(args[3]) == "empty":
		if len(gs.Seats[seatIdx].Battlefield) == 0 {
			fmt.Printf("  PASS (seat %d battlefield is empty)\n", seatIdx)
		} else {
			fmt.Printf("  FAIL (seat %d battlefield has %d permanents)\n", seatIdx, len(gs.Seats[seatIdx].Battlefield))
		}

	case zone == "life" && len(args) >= 5:
		op := args[3]
		val, err := strconv.Atoi(args[4])
		if err != nil {
			fmt.Printf("  invalid number: %s\n", args[4])
			return
		}
		actual := gs.Seats[seatIdx].Life
		pass := false
		switch op {
		case "==", "=":
			pass = actual == val
		case ">=":
			pass = actual >= val
		case "<=":
			pass = actual <= val
		case ">":
			pass = actual > val
		case "<":
			pass = actual < val
		case "!=":
			pass = actual != val
		default:
			fmt.Printf("  unknown operator: %s\n", op)
			return
		}
		if pass {
			fmt.Printf("  PASS (seat %d life=%d %s %d)\n", seatIdx, actual, op, val)
		} else {
			fmt.Printf("  FAIL (seat %d life=%d, expected %s %d)\n", seatIdx, actual, op, val)
		}

	case zone == "hand" && len(args) >= 5 && strings.ToLower(args[3]) == "contains":
		cardName := strings.Join(args[4:], " ")
		found := false
		for _, c := range gs.Seats[seatIdx].Hand {
			if c != nil && strings.EqualFold(c.DisplayName(), cardName) {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("  PASS (%s in seat %d hand)\n", cardName, seatIdx)
		} else {
			fmt.Printf("  FAIL (%s NOT in seat %d hand)\n", cardName, seatIdx)
		}

	case zone == "exile" && len(args) >= 5 && strings.ToLower(args[3]) == "contains":
		cardName := strings.Join(args[4:], " ")
		found := false
		for _, c := range gs.Seats[seatIdx].Exile {
			if c != nil && strings.EqualFold(c.DisplayName(), cardName) {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("  PASS (%s in seat %d exile)\n", cardName, seatIdx)
		} else {
			fmt.Printf("  FAIL (%s NOT in seat %d exile)\n", cardName, seatIdx)
		}

	default:
		fmt.Println("  unrecognized assert format")
	}
}

func cmdStep() {
	if !requireGame() {
		return
	}
	// Advance through the phase/step sequence.
	type phaseStep struct {
		phase, step string
	}
	sequence := []phaseStep{
		{"beginning", "untap"},
		{"beginning", "upkeep"},
		{"beginning", "draw"},
		{"main", "precombat_main"},
		{"combat", "beginning_of_combat"},
		{"combat", "declare_attackers"},
		{"combat", "declare_blockers"},
		{"combat", "combat_damage"},
		{"combat", "end_of_combat"},
		{"main", "postcombat_main"},
		{"ending", "end"},
		{"ending", "cleanup"},
	}

	current := -1
	for i, ps := range sequence {
		if ps.phase == gs.Phase && ps.step == gs.Step {
			current = i
			break
		}
	}
	next := (current + 1) % len(sequence)
	gs.Phase = sequence[next].phase
	gs.Step = sequence[next].step
	if next == 0 {
		gs.Turn++
	}
	fmt.Printf("  Advanced to Turn %d %s/%s\n", gs.Turn, gs.Phase, gs.Step)
}

func cmdEvents(args []string) {
	if !requireGame() {
		return
	}
	n := 20
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			n = parsed
		}
	}
	events := gameengine.RecentEvents(gs, n)
	if len(events) == 0 {
		fmt.Println("  No events")
		return
	}
	for _, e := range events {
		fmt.Printf("  %s\n", e)
	}
}

func cmdInvariants() {
	if !requireGame() {
		return
	}
	violations := gameengine.RunAllInvariants(gs)
	if len(violations) == 0 {
		fmt.Println("  All invariants PASS")
		return
	}
	for _, v := range violations {
		fmt.Printf("  FAIL %s: %s\n", v.Name, v.Message)
	}
}

func cmdState() {
	if !requireGame() {
		return
	}
	fmt.Println(gameengine.GameStateSummary(gs))
}

// ---------------------------------------------------------------------------
// Card / Permanent creation helpers
// ---------------------------------------------------------------------------

func createCard(owner int, name string) *gameengine.Card {
	card := &gameengine.Card{
		Name:  name,
		Owner: owner,
	}

	// Try to find AST data.
	if corpus != nil {
		if ast, ok := corpus.Get(name); ok {
			card.AST = ast
			card.Name = ast.Name // canonical name
		}
	}

	// Try to find metadata.
	if meta != nil {
		if m := meta.Get(name); m != nil {
			card.Types = m.Types
			card.Colors = m.Colors
			card.CMC = m.CMC
			card.BasePower = m.Power
			card.BaseToughness = m.Toughness
			card.TypeLine = m.TypeLine
			if card.Name == name && m.Name != "" {
				card.Name = m.Name // prefer meta canonical name
			}
		}
	}

	return card
}

func createPermanent(owner int, name string) *gameengine.Permanent {
	card := createCard(owner, name)
	perm := &gameengine.Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	return perm
}

func formatAbilities(chars *gameengine.Characteristics) string {
	if chars == nil || len(chars.Abilities) == 0 {
		return ""
	}
	names := make([]string, 0, len(chars.Abilities))
	for _, ab := range chars.Abilities {
		names = append(names, fmt.Sprintf("%T", ab))
	}
	return strings.Join(names, ", ")
}

func printHelp() {
	fmt.Print(`
Commands:
  create game --seats N              Create fresh game state
  seat N add_permanent "Card"        Add permanent to battlefield
  seat N add_to_hand "Card"          Add card to hand
  seat N set_life N                  Set life total
  seat N cast "Card" [targeting seat M [permanent "Name"]]
                                     Cast a spell onto the stack
  resolve                            Resolve top of stack
  sba                                Run state-based actions
  query seat N permanent "Name" [characteristics]
                                     Show permanent details
  query seat N life                  Show life total
  query seat N hand                  Show hand contents
  query seat N battlefield           Show all permanents
  query seat N graveyard             Show graveyard
  query seat N exile                 Show exile
  query seat N library               Show library count
  query layers                       Show all continuous effects
  assert seat N graveyard contains "Card"
  assert seat N battlefield contains "Card"
  assert seat N battlefield empty
  assert seat N hand contains "Card"
  assert seat N exile contains "Card"
  assert seat N life == N
  assert stack empty
  step                               Advance one phase/step
  events [N]                         Show last N events (default 20)
  invariants                         Run all invariant checks
  state                              Dump full game state
  help                               Show this help
  exit / quit                        Exit
`)
}
