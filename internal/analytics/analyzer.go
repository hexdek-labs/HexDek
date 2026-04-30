package analytics

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// AnalyzeGame walks the event log and game-end state to build a full
// GameAnalysis. This is the core analysis function -- it processes one
// completed game's event stream and produces structured analytics.
//
// Parameters:
//   - events: the game's event log (gs.EventLog)
//   - nSeats: number of seats in the game
//   - commanderNames: commander name per seat index
//   - winnerSeat: seat index of winner (-1 for draw)
//   - totalTurns: total turns the game ran
//   - handsAtEnd: number of cards in each seat's hand at game end (nil-safe)
//   - finalLife: life total per seat at game end
//   - boardSizes: number of permanents per seat at game end (used only for final snapshot)
func AnalyzeGame(
	events []gameengine.Event,
	nSeats int,
	commanderNames []string,
	winnerSeat int,
	totalTurns int,
	handsAtEnd []int,
	finalLife []int,
) *GameAnalysis {
	ga := &GameAnalysis{
		Players:     make([]PlayerAnalysis, nSeats),
		WinnerSeat:  winnerSeat,
		TotalTurns:  totalTurns,
		TotalEvents: len(events),
	}

	// Initialize player analyses.
	for i := 0; i < nSeats; i++ {
		ga.Players[i].Seat = i
		if i < len(commanderNames) {
			ga.Players[i].CommanderName = commanderNames[i]
		}
		ga.Players[i].Won = (i == winnerSeat)
		if i < len(finalLife) {
			ga.Players[i].FinalLife = finalLife[i]
			ga.Players[i].PeakLife = finalLife[i] // will be updated below
		}
		if i < len(handsAtEnd) {
			ga.Players[i].CardsInHand = handsAtEnd[i]
		}
	}

	// Track card lifecycles: cardName -> seat -> CardPerformance.
	// We use a map of seat -> map of cardName -> *CardPerformance for
	// efficient lookup during event processing.
	cardPerf := make([]map[string]*CardPerformance, nSeats)
	for i := range cardPerf {
		cardPerf[i] = make(map[string]*CardPerformance)
	}

	// Track which card names are tokens (created by create_token events)
	// so they can be excluded from dead-card analysis.
	tokenNames := make(map[string]bool)

	// Track which card names are lands (played via play_land events)
	// so they can be excluded from dead-card analysis.
	landNames := make(map[string]bool)

	// Track per-turn board sizes for peak calculation.
	currentBoardSize := make([]int, nSeats)
	// Track life for peak calculation.
	currentLife := make([]int, nSeats)
	for i := 0; i < nSeats; i++ {
		currentLife[i] = 40 // commander default; will be corrected by life_change events
	}

	// Track per-turn creature deaths for board wipe detection.
	currentTurn := 1
	creaturesDestroyedThisTurn := 0

	// Walk the event log.
	for idx := range events {
		ev := &events[idx]
		seat := ev.Seat
		if seat < 0 || seat >= nSeats {
			// Some events have seat=-1 (no actor); skip seat-specific tracking.
			seat = -1
		}

		// Track turn transitions for per-turn stats.
		if ev.Kind == "turn_start" {
			if t, ok := detailInt(ev, "turn"); ok && t > currentTurn {
				// Snapshot board sizes at turn boundary.
				for i := 0; i < nSeats; i++ {
					if currentBoardSize[i] > ga.Players[i].PeakBoardSize {
						ga.Players[i].PeakBoardSize = currentBoardSize[i]
					}
				}
				creaturesDestroyedThisTurn = 0
				currentTurn = t
			}
		}

		switch ev.Kind {

		case "cast":
			if seat < 0 {
				continue
			}
			ga.Players[seat].SpellsCast++
			cardName := ev.Source
			if cardName == "" {
				continue
			}
			cp := getOrCreateCardPerf(cardPerf[seat], cardName)
			if cp.TurnCast == 0 {
				cp.TurnCast = currentTurn
			}

		case "play_land":
			if seat < 0 {
				continue
			}
			ga.Players[seat].LandsPlayed++
			// Track land in card performance so it doesn't appear as "never cast."
			// Lands are played, not cast, so we mark them with TurnCast to indicate
			// they were used.
			cardName := ev.Source
			if cardName != "" {
				landNames[cardName] = true
				cp := getOrCreateCardPerf(cardPerf[seat], cardName)
				if cp.TurnCast == 0 {
					cp.TurnCast = currentTurn
				}
				cp.IsLand = true
			}

		case "damage":
			if seat < 0 {
				continue
			}
			amount := ev.Amount
			targetKind, _ := detailString(ev, "target_kind")
			isCombat, _ := detailBool(ev, "combat")

			if targetKind == "player" {
				targetSeat := ev.Target
				if targetSeat >= 0 && targetSeat < nSeats {
					ga.Players[targetSeat].DamageTaken += amount
				}
				ga.Players[seat].DamageDealt += amount

				// First blood tracking.
				if ga.FirstBlood == 0 && amount > 0 {
					ga.FirstBlood = currentTurn
				}
			}

			// Attribute damage to card.
			if ev.Source != "" {
				cp := getOrCreateCardPerf(cardPerf[seat], ev.Source)
				cp.DamageDealt += amount
			}

			// Track attacks.
			if isCombat && targetKind == "player" {
				ga.Players[seat].AttacksDeclared++ // approximate: one event per attacker dealing damage
			}

			// Track creature kills via combat.
			if targetKind == "creature" {
				targetCard, _ := detailString(ev, "target_card")
				_ = targetCard // damage to creature; kill attribution happens at creature_dies
			}

		case "life_change":
			if seat < 0 {
				continue
			}
			amount := ev.Amount
			// life_change Amount is the delta (positive = gain, negative = loss).
			currentLife[seat] += amount
			if currentLife[seat] > ga.Players[seat].PeakLife {
				ga.Players[seat].PeakLife = currentLife[seat]
			}

		case "creature_dies":
			creaturesDestroyedThisTurn++
			// First death tracking.
			if ga.FirstDeath == 0 {
				ga.FirstDeath = currentTurn
			}
			// Board wipe detection: 3+ creature deaths in same turn.
			if ga.FirstWipe == 0 && creaturesDestroyedThisTurn >= 3 {
				ga.FirstWipe = currentTurn
			}
			// Attribute to the controller losing a creature.
			if seat >= 0 {
				ga.Players[seat].CreaturesLost++
			}

		case "destroy":
			if seat < 0 {
				continue
			}
			// Track removal cast: the Source is the card doing the destroying.
			if ev.Source != "" {
				ga.Players[seat].RemovalCast++
				cp := getOrCreateCardPerf(cardPerf[seat], ev.Source)
				cp.KillsAttributed++
			}
			// Track the target creature's seat losing a creature.
			targetSeat := ev.Target
			if targetSeat >= 0 && targetSeat < nSeats && targetSeat != seat {
				ga.Players[seat].CreaturesKilled++
			}
			// Mark the destroyed card.
			targetCard, _ := detailString(ev, "target_card")
			if targetCard != "" && targetSeat >= 0 && targetSeat < nSeats {
				cp := getOrCreateCardPerf(cardPerf[targetSeat], targetCard)
				if cp.TurnDestroyed == 0 {
					cp.TurnDestroyed = currentTurn
				}
			}

		case "sacrifice":
			if seat >= 0 {
				ga.Players[seat].CreaturesLost++
			}
			targetCard, _ := detailString(ev, "target_card")
			if targetCard != "" && seat >= 0 {
				cp := getOrCreateCardPerf(cardPerf[seat], targetCard)
				if cp.TurnDestroyed == 0 {
					cp.TurnDestroyed = currentTurn
				}
			}

		case "counter_spell":
			if seat < 0 {
				continue
			}
			ga.Players[seat].CountersCast++
			// The target seat had their spell countered.
			targetSeat := ev.Target
			if targetSeat >= 0 && targetSeat < nSeats {
				ga.Players[targetSeat].SpellsCountered++
			}
			// Mark the countered card.
			targetCard, _ := detailString(ev, "target_card")
			if targetCard != "" && targetSeat >= 0 && targetSeat < nSeats {
				cp := getOrCreateCardPerf(cardPerf[targetSeat], targetCard)
				cp.WasCountered = true
			}

		case "create_token":
			// Mark token names so they are excluded from dead-card analysis.
			if ev.Source != "" {
				tokenNames[ev.Source] = true
			}

		case "pool_drain":
			if seat < 0 {
				continue
			}
			ga.Players[seat].ManaWasted += ev.Amount

		case "pay_mana":
			if seat < 0 {
				continue
			}
			ga.Players[seat].ManaSpent += ev.Amount

		case "enter_battlefield":
			if seat >= 0 {
				currentBoardSize[seat]++
			}

		case "leave_battlefield":
			if seat >= 0 && currentBoardSize[seat] > 0 {
				currentBoardSize[seat]--
			}

		case "player_loss":
			if seat >= 0 {
				ga.Players[seat].TurnOfDeath = currentTurn
			}

		case "delayed_trigger_fires":
			if seat >= 0 && ev.Source != "" {
				cp := getOrCreateCardPerf(cardPerf[seat], ev.Source)
				cp.TriggeredCount++
			}

		case "per_card_win":
			// Combo / alternate win condition detected.
			if ga.ComboAssembled == 0 {
				ga.ComboAssembled = currentTurn
			}

		case "triggered_ability":
			// Track trigger frequency per card (value engines like Blood
			// Artist, Rhystic Study).
			if seat >= 0 && ev.Source != "" {
				ga.Players[seat].TriggersFired++
				cp := getOrCreateCardPerf(cardPerf[seat], ev.Source)
				cp.TriggeredCount++
			}

		case "trigger_evaluated":
			// Trigger was checked but may not have fired (condition
			// failed). Informational only — no stats tracked.
		}
	}

	// Final board size snapshot.
	for i := 0; i < nSeats; i++ {
		if currentBoardSize[i] > ga.Players[i].PeakBoardSize {
			ga.Players[i].PeakBoardSize = currentBoardSize[i]
		}
	}

	// Flatten card performances into player analyses.
	// Mark tokens and lands so they can be excluded from dead-card analysis.
	for i := 0; i < nSeats; i++ {
		perfs := make([]CardPerformance, 0, len(cardPerf[i]))
		for _, cp := range cardPerf[i] {
			if tokenNames[cp.Name] {
				cp.IsToken = true
			}
			if landNames[cp.Name] {
				cp.IsLand = true
			}
			perfs = append(perfs, *cp)
		}
		ga.Players[i].CardsPlayed = perfs
	}

	// Detect win condition.
	ga.WinCondition, ga.WinningCard = detectWinCondition(events, winnerSeat, nSeats, totalTurns)

	// Mark cards that contributed to win.
	if winnerSeat >= 0 && winnerSeat < nSeats && ga.WinningCard != "" {
		for j := range ga.Players[winnerSeat].CardsPlayed {
			if ga.Players[winnerSeat].CardsPlayed[j].Name == ga.WinningCard {
				ga.Players[winnerSeat].CardsPlayed[j].ContributedToWin = true
			}
		}
	}

	// Stall detection.
	ga.StallIndicators = detectStall(events, ga, nSeats, totalTurns, finalLife)

	// Upgrade "unknown" win condition when stall is detected.
	if ga.WinCondition == "unknown" && ga.StallIndicators != nil && ga.StallIndicators.HitTurnCap {
		ga.WinCondition = "turn_cap"
	}

	// Co-trigger detection: find causally linked card pairs that fired in
	// the same turn. gameIdx 0 is used here; callers with multi-game
	// context (tournament runner) can re-run with the real game index.
	ga.CoTriggerObservations = DetectCoTriggers(events, nSeats, 0)

	return ga
}

// detectWinCondition examines the tail of the event log to determine how
// the game ended and which card delivered the killing blow.
//
// Priority order:
//  1. per_card_win → "combo" (Thassa's Oracle, Laboratory Maniac, etc.)
//  2. seat_eliminated / sba with poison reason → "poison"
//  3. seat_eliminated / sba with commander damage reason → "commander_damage"
//  4. seat_eliminated / sba with Oracle/library reason → "combo"
//  5. Last combat damage by winner → "combat_damage"
//  6. Last non-combat damage / life drain → "life_drain"
//  7. Fallback → "unknown"
func detectWinCondition(events []gameengine.Event, winnerSeat, nSeats, totalTurns int) (condition, killingCard string) {
	if winnerSeat < 0 {
		// Check for timeout vs draw.
		for i := len(events) - 1; i >= 0 && i >= len(events)-20; i-- {
			if events[i].Kind == "infinite_loop_draw" {
				return "draw", ""
			}
		}
		return "timeout", ""
	}

	// Scan backwards through the last ~200 events to find the decisive
	// moment. 50 was too narrow — a Thassa's Oracle win triggers
	// HandleSeatElimination for 3 seats, each of which can emit dozens of
	// events (permanent removal, stack purge, etc.), pushing the per_card_win
	// event well past the 50-event window.
	scanStart := len(events) - 200
	if scanStart < 0 {
		scanStart = 0
	}

	// 1. Look for per_card_win (combo/alternate wincon). This is the
	//    highest-priority check because it's emitted by explicit "you win
	//    the game" card effects (Thassa's Oracle, Lab Man, etc.).
	for i := len(events) - 1; i >= scanStart; i-- {
		ev := &events[i]
		if ev.Kind == "per_card_win" && ev.Seat == winnerSeat {
			return "combo", ev.Source
		}
	}

	// 2-4. Check seat_eliminated and SBA events for specific loss reasons
	//      (poison, commander damage, Oracle/library-based wins).
	for i := len(events) - 1; i >= scanStart; i-- {
		ev := &events[i]
		switch ev.Kind {
		case "seat_eliminated", "sba_704_5a", "sba_704_5c", "sba_704_6c", "player_loss", "lose_game":
			reason, _ := detailString(ev, "reason")
			reasonLower := strings.ToLower(reason)

			// Poison kills (CR 704.5c).
			if strings.Contains(reasonLower, "poison") {
				return "poison", ev.Source
			}

			// Commander damage kills (CR 704.6c).
			if strings.Contains(reasonLower, "commander") || strings.Contains(reasonLower, "704.6c") {
				return "commander_damage", ev.Source
			}

			// Oracle / Lab Man wins are combo.
			if strings.Contains(reasonLower, "oracle") ||
				strings.Contains(reasonLower, "labman") ||
				strings.Contains(reasonLower, "laboratory") {
				return "combo", ev.Source
			}

			// Empty library death (CR 704.5b) — not a combo, just decking.
			if strings.Contains(reasonLower, "library") || strings.Contains(reasonLower, "704.5b") {
				return "decking", ev.Source
			}

			// Sanguine Bond / Exquisite Blood infinite drain combo.
			if strings.Contains(reasonLower, "sanguine") || strings.Contains(reasonLower, "exquisite") {
				return "combo", ev.Source
			}

			// Pact failure — classify as combo (the opponent set up a pact win).
			if strings.Contains(reasonLower, "failed_to_pay") {
				return "combo", ev.Source
			}
		}
	}

	// 5. Find the LETHAL damage source: the damage event that pushed a
	//    losing player's life to 0 or below. We simulate life tracking
	//    backwards from seat_eliminated events to find the exact source
	//    that crossed the threshold.
	//
	//    Strategy: find the last seat_eliminated event for a non-winner,
	//    then scan backwards for damage events targeting that player.
	//    Track life backwards to find the one that crossed 0.
	var lastDamageCard string
	var lastDamageKind string

	// Find eliminated players (non-winners).
	for i := len(events) - 1; i >= scanStart; i-- {
		ev := &events[i]
		if (ev.Kind == "seat_eliminated" || ev.Kind == "player_loss") &&
			ev.Seat != winnerSeat && ev.Seat >= 0 && ev.Seat < nSeats {
			eliminatedSeat := ev.Seat
			// Scan backwards from this elimination to find the lethal
			// damage event. Walk damage events targeting this player and
			// track running life total to find the one that crossed 0.
			runningLife := 0 // life at elimination (0 or negative)
			for j := i - 1; j >= scanStart; j-- {
				ej := &events[j]
				if ej.Kind == "damage" && ej.Amount > 0 {
					targetKind, _ := detailString(ej, "target_kind")
					targetSeat := ej.Target
					if targetKind == "player" && targetSeat == eliminatedSeat {
						// This damage event reduced life. If undoing it would
						// bring life above 0, this is the lethal source.
						if runningLife+ej.Amount > 0 {
							lastDamageCard = ej.Source
							isCombat, _ := detailBool(ej, "combat")
							if isCombat {
								lastDamageKind = "combat_damage"
							} else {
								lastDamageKind = "life_drain"
							}
							break
						}
						runningLife += ej.Amount
					}
				}
				if ej.Kind == "life_change" && ej.Seat == eliminatedSeat {
					// Undo the life change to walk backwards.
					runningLife -= ej.Amount
				}
			}
			if lastDamageKind != "" {
				break
			}
		}
	}

	// Fallback: if lethal-tracking didn't find a source, use the last
	// damage event by the winner (original behavior).
	if lastDamageKind == "" {
		for i := len(events) - 1; i >= scanStart; i-- {
			ev := &events[i]
			if ev.Kind == "damage" && ev.Seat == winnerSeat && ev.Amount > 0 {
				targetKind, _ := detailString(ev, "target_kind")
				if targetKind == "player" {
					lastDamageCard = ev.Source
					isCombat, _ := detailBool(ev, "combat")
					if isCombat {
						lastDamageKind = "combat_damage"
					} else {
						lastDamageKind = "life_drain"
					}
					break
				}
			}
		}
	}

	if lastDamageKind != "" {
		return lastDamageKind, lastDamageCard
	}

	// 6. Look for life_change events (drain effects like Exsanguinate).
	for i := len(events) - 1; i >= scanStart; i-- {
		ev := &events[i]
		if ev.Kind == "life_change" && ev.Seat != winnerSeat && ev.Amount < 0 {
			return "life_drain", ev.Source
		}
	}

	return "unknown", ""
}

// DetectMissedCombos scans the game-end state to find seats where a known
// combo was available (all pieces on the battlefield, mana sufficient) but
// wasn't executed. Each hit represents a Hat intelligence gap — the AI had
// the win available but didn't see it.
//
// This examines end-of-game state only. A combo is considered "missed" if
// the seat had all pieces on board with enough mana and did not win.
func DetectMissedCombos(gs *gameengine.GameState) []MissedCombo {
	if gs == nil {
		return nil
	}

	var missed []MissedCombo

	for seatIdx, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}

		// Build a set of card names on this seat's battlefield.
		onBoard := map[string]bool{}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			onBoard[p.Card.DisplayName()] = true
		}

		// Check each known combo.
		for _, combo := range KnownCombos {
			allPresent := true
			for _, piece := range combo.Pieces {
				if !onBoard[piece] {
					allPresent = false
					break
				}
			}
			if !allPresent {
				continue
			}

			// Check mana availability — prefer typed pool, fall back to legacy.
			availMana := seat.ManaPool
			if seat.Mana != nil {
				availMana = seat.Mana.Total()
			}
			if availMana < combo.ManaCost {
				continue
			}

			// Special case: Aetherflux Reservoir needs 51+ life.
			if combo.Name == "Aetherflux Reservoir Storm" && seat.Life < 51 {
				continue
			}

			// Combo was live — check if the player actually won.
			if !seat.Won {
				missed = append(missed, MissedCombo{
					Seat:      seatIdx,
					ComboName: combo.Name,
					Turn:      gs.Turn,
					Pieces:    combo.Pieces,
					WinType:   combo.WinType,
					ManaAvail: availMana,
				})
			}
		}
	}

	return missed
}

// DetectMissedCombosWithStrategy extends DetectMissedCombos by also
// checking Freya's per-deck ComboPieces, not just the hardcoded list.
func DetectMissedCombosWithStrategy(gs *gameengine.GameState, strategyCombos [][]string) []MissedCombo {
	missed := DetectMissedCombos(gs)
	if gs == nil || len(strategyCombos) == 0 {
		return missed
	}

	for seatIdx, seat := range gs.Seats {
		if seat == nil || seat.Lost || seat.Won {
			continue
		}
		onBoard := map[string]bool{}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			onBoard[p.Card.DisplayName()] = true
		}
		for _, combo := range strategyCombos {
			if len(combo) < 2 {
				continue
			}
			allPresent := true
			for _, piece := range combo {
				if !onBoard[piece] {
					allPresent = false
					break
				}
			}
			if allPresent {
				missed = append(missed, MissedCombo{
					Seat:      seatIdx,
					ComboName: strings.Join(combo, " + "),
					Turn:      gs.Turn,
					Pieces:    combo,
					WinType:   "freya_combo",
				})
			}
		}
	}
	return missed
}

// DetectMissedFinishers checks game-end state for seats that had a
// Freya-classified finisher on the battlefield but didn't win.
func DetectMissedFinishers(gs *gameengine.GameState, finisherSets map[int]map[string]bool) []MissedFinisher {
	if gs == nil {
		return nil
	}
	var out []MissedFinisher
	for seatIdx, seat := range gs.Seats {
		if seat == nil || seat.Lost || seat.Won {
			continue
		}
		finishers, ok := finisherSets[seatIdx]
		if !ok || len(finishers) == 0 {
			continue
		}
		boardPow := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				boardPow += p.Power()
			}
		}
		oppMinLife := 999
		for i, opp := range gs.Seats {
			if i == seatIdx || opp == nil || opp.Lost || opp.LeftGame {
				continue
			}
			if opp.Life < oppMinLife {
				oppMinLife = opp.Life
			}
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if finishers[p.Card.DisplayName()] {
				out = append(out, MissedFinisher{
					Seat:         seatIdx,
					FinisherName: p.Card.DisplayName(),
					Turn:         gs.Turn,
					BoardPower:   boardPow,
					OppLifeMin:   oppMinLife,
				})
			}
		}
	}
	return out
}

// getOrCreateCardPerf retrieves or creates a CardPerformance entry.
func getOrCreateCardPerf(m map[string]*CardPerformance, name string) *CardPerformance {
	if cp, ok := m[name]; ok {
		return cp
	}
	cp := &CardPerformance{Name: name}
	m[name] = cp
	return cp
}

// Detail extraction helpers for type-safe access to Event.Details.

func detailInt(ev *gameengine.Event, key string) (int, bool) {
	if ev.Details == nil {
		return 0, false
	}
	v, ok := ev.Details[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	case int64:
		return int(x), true
	}
	return 0, false
}

func detailString(ev *gameengine.Event, key string) (string, bool) {
	if ev.Details == nil {
		return "", false
	}
	v, ok := ev.Details[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func detailBool(ev *gameengine.Event, key string) (bool, bool) {
	if ev.Details == nil {
		return false, false
	}
	v, ok := ev.Details[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// detectStall analyzes a completed game for board-stall patterns.
// Returns nil if the game wasn't stalled.
func detectStall(events []gameengine.Event, ga *GameAnalysis, nSeats, totalTurns int, finalLife []int) *StallReport {
	if ga == nil {
		return nil
	}

	survivors := 0
	for i := range ga.Players {
		if ga.Players[i].TurnOfDeath == 0 {
			survivors++
		}
	}

	// A game that eliminated all but one player decisively is not a stall.
	if survivors <= 1 {
		return nil
	}

	// Find the last elimination turn.
	lastKillTurn := 0
	for i := range ga.Players {
		if ga.Players[i].TurnOfDeath > lastKillTurn {
			lastKillTurn = ga.Players[i].TurnOfDeath
		}
	}
	turnsSinceKill := totalTurns - lastKillTurn

	// Life spread among survivors.
	highLife, lowLife, highSeat := 0, 999999, -1
	for i := range finalLife {
		if i >= nSeats {
			break
		}
		if i < len(ga.Players) && ga.Players[i].TurnOfDeath > 0 {
			continue
		}
		if finalLife[i] > highLife {
			highLife = finalLife[i]
			highSeat = i
		}
		if finalLife[i] < lowLife {
			lowLife = finalLife[i]
		}
	}
	spread := highLife - lowLife

	// Classify cause.
	cause := "unknown"
	switch {
	case highLife > 60:
		cause = "pillow_fort"
	case spread < 10 && survivors >= 3:
		cause = "board_parity"
	case turnsSinceKill > totalTurns/2:
		cause = "low_aggression"
	}

	// Total combat damage across all players.
	totalDmg := 0
	for i := range ga.Players {
		totalDmg += ga.Players[i].DamageDealt
	}
	if totalDmg < totalTurns*2 && cause == "unknown" {
		cause = "low_aggression"
	}

	return &StallReport{
		HitTurnCap:         totalTurns >= 55,
		SurvivorsAtEnd:     survivors,
		TurnsSinceLastKill: turnsSinceKill,
		LifeLeader:         highSeat,
		LifeLeaderTotal:    highLife,
		LifeSpread:         spread,
		Cause:              cause,
	}
}
