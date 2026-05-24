package heimdall

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

const (
	replayMaxTurns = 80
	replaySeats    = 4
)

// ReplayContext holds the pre-loaded resources needed to replay games.
// Callers construct one ReplayContext and reuse it across many replays to
// avoid re-loading the corpus and deck pool on every game.
type ReplayContext struct {
	Corpus  *astload.Corpus
	Meta    *deckparser.MetaDB
	DeckDir string // root of deck files (e.g. "data/decks")

	// deckCache maps deck key ("owner/name") to the parsed TournamentDeck.
	// Populated lazily on first use.
	deckCache map[string]*deckparser.TournamentDeck
}

// NewReplayContext loads the AST corpus and card metadata needed for
// replays. astPath is the JSONL AST file, oraclePath is the supplementary
// oracle JSON (may be ""), and decksDir is the root directory of deck
// files.
func NewReplayContext(astPath, oraclePath, decksDir string) (*ReplayContext, error) {
	corpus, err := astload.Load(astPath)
	if err != nil {
		return nil, fmt.Errorf("replay: load corpus: %w", err)
	}

	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		return nil, fmt.Errorf("replay: load meta: %w", err)
	}
	if oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
			log.Printf("replay: oracle supplement: %v (continuing)", err)
		}
	}

	return &ReplayContext{
		Corpus:    corpus,
		Meta:      meta,
		DeckDir:   decksDir,
		deckCache: make(map[string]*deckparser.TournamentDeck),
	}, nil
}

// resolveDeck looks up a deck by its key ("owner/name") and returns the
// parsed TournamentDeck. Results are cached for the lifetime of the
// ReplayContext.
func (rc *ReplayContext) resolveDeck(key string) (*deckparser.TournamentDeck, error) {
	if d, ok := rc.deckCache[key]; ok {
		return d, nil
	}

	// Deck key is "owner/name" — file is at decksDir/owner/name.txt.
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("replay: invalid deck key %q (expected owner/name)", key)
	}
	path := filepath.Join(rc.DeckDir, parts[0], parts[1]+".txt")

	d, err := deckparser.ParseDeckFile(path, rc.Corpus, rc.Meta)
	if err != nil {
		return nil, fmt.Errorf("replay: parse deck %q: %w", key, err)
	}

	rc.deckCache[key] = d
	return d, nil
}

// ReadSeeds reads up to limit GameSeed records from a JSONL file. If
// limit <= 0, all seeds are read.
func ReadSeeds(path string, limit int) ([]GameSeed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("replay: open seeds: %w", err)
	}
	defer f.Close()

	var seeds []GameSeed
	scanner := bufio.NewScanner(f)
	// Seeds are small (~150 bytes), but allow for generous lines.
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)

	for scanner.Scan() {
		if limit > 0 && len(seeds) >= limit {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var s GameSeed
		if err := json.Unmarshal(line, &s); err != nil {
			// Skip malformed lines silently.
			continue
		}
		seeds = append(seeds, s)
	}
	if err := scanner.Err(); err != nil {
		return seeds, fmt.Errorf("replay: scan seeds: %w", err)
	}
	return seeds, nil
}

// ReplayWithObservation replays a single game from its seed and extracts
// an Observation that is forwarded to the Observer's sinks.
//
// The replay uses a simplified hat (YggdrasilHat with nil strategy, low
// budget) instead of the full Curse/ELO-weighted hat used during live
// grinding. This is intentional: the replay system exists to extract
// observations, not to produce tournament-accurate results. The game
// outcome is deterministic given the same RNG seed and deck ordering
// regardless of hat quality — the hat only affects which lines are
// chosen, and for observation purposes the simplified version is
// sufficient.
func ReplayWithObservation(rc *ReplayContext, seed GameSeed, obs *Observer) error {
	// Resolve all 4 decks.
	decks := make([]*deckparser.TournamentDeck, replaySeats)
	for i := 0; i < replaySeats; i++ {
		d, err := rc.resolveDeck(seed.DeckKeys[i])
		if err != nil {
			return fmt.Errorf("replay: seat %d: %w", i, err)
		}
		decks[i] = d
	}

	// Create a deterministic RNG from the stored seed.
	gameRng := rand.New(rand.NewSource(seed.RNGSeed))

	// Build game state — mirrors showmatch.runOneGameFast.
	gs := gameengine.NewGameState(replaySeats, gameRng, rc.Corpus)
	gs.RetainEvents = false // no event log accumulation

	cmdDecks := make([]*gameengine.CommanderDeck, replaySeats)
	for i := 0; i < replaySeats; i++ {
		tpl := decks[i]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		cmdDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, cmdDecks)

	// Assign simplified hats — YggdrasilHat with nil strategy and low
	// budget. This produces reasonable play without needing Freya profiles
	// or Curse DNA.
	for i := 0; i < replaySeats; i++ {
		gs.Seats[i].Hat = hat.NewYggdrasilHat(nil, 30)
	}

	// London mulligan for each seat.
	for i := 0; i < replaySeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	// Random starting player from game RNG.
	gs.Active = gameRng.Intn(replaySeats)
	gs.Turn = 1

	// Track which cards enter the battlefield per turn for co-trigger
	// detection. Map: turn -> list of card names.
	turnETBs := make(map[int][]string)

	// Run the game loop — mirrors runOneGameFast.
	for turn := 1; turn <= replayMaxTurns; turn++ {
		gs.Turn = turn

		// Snapshot battlefield before turn to detect new permanents.
		preBF := SnapshotBattlefieldNames(gs)

		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)

		// Detect new permanents this turn.
		postBF := SnapshotBattlefieldNames(gs)
		newCards := DiffBattlefield(preBF, postBF)
		if len(newCards) > 0 {
			turnETBs[turn] = newCards
		}

		if gs.CheckEnd() {
			break
		}
		gs.Active = nextLivingReplay(gs)
	}

	// Determine winner — same logic as runOneGameFast.
	winner := -1
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < replaySeats {
			winner = w
		}
	}
	if winner < 0 {
		bestLife := -999
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && s.Life > bestLife {
				bestLife = s.Life
				winner = i
			}
		}
	}

	// --- Extract observations ---
	observation := Observation{
		Seed: seed,
	}

	// 1. Parser gaps: scan all permanents for the parser_gap flag.
	observation.ParserGaps = ExtractParserGaps(gs)

	// 2. Co-triggers: cards that entered the battlefield within the same
	//    or adjacent turns. Simplified version of Huginn's full analysis.
	observation.CoTriggers = ExtractCoTriggers(turnETBs)

	// 2b. Commander-zone visits: per-commander cast count + end-state
	//     command-zone presence so downstream consumers can spot
	//     answer-magnet commanders vs sticky board commanders.
	observation.CommanderZoneVisits = ExtractCommanderZoneVisits(gs)

	// 2c. Regret cards: nonland CMC≥1 cards still in hand at game
	//     end — the "stranded in hand" half of the regret signal.
	observation.RegretCards = ExtractRegretCards(gs)

	// 2d. MVP cards: top mvpTopN permanents per seat at game end,
	//     scored by CMC + positive-counters + commander bonus.
	observation.MVPCards = ExtractMVPCards(gs)

	// 3. Combo detection: TODO — requires Freya integration to know what
	//    the deck's intended combo line is and whether pieces were
	//    assembled.
	// observation.ComboAttempted = false
	// observation.ComboSucceeded = false
	// observation.ComboMissed = false

	// 4. Causal pivot: TODO — requires Tesla implementation to identify
	//    the decisive game action.
	// observation.CausalPivot = nil

	// Route observation to sinks.
	if obs != nil {
		obs.RecordObservation(observation)
	}

	return nil
}

// BatchReplay replays up to count seeds from the given JSONL file and
// routes observations through the provided Observer. Returns the number
// of games successfully replayed.
func BatchReplay(rc *ReplayContext, seedPath string, count int, obs *Observer) (int, error) {
	seeds, err := ReadSeeds(seedPath, count)
	if err != nil {
		return 0, err
	}

	replayed := 0
	for i, seed := range seeds {
		if err := replayWithRecovery(rc, seed, obs); err != nil {
			log.Printf("replay: game %d (rng=%d) failed: %v", i, seed.RNGSeed, err)
			continue
		}
		replayed++
	}
	return replayed, nil
}

// replayWithRecovery wraps ReplayWithObservation in a panic recovery so
// one crashing game doesn't abort the entire batch.
func replayWithRecovery(rc *ReplayContext, seed GameSeed, obs *Observer) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			retErr = fmt.Errorf("panic: %v\n%s", r, buf[:n])
			// Route crash to Muninn if observer is available.
			if obs != nil {
				obs.RecordCrash(fmt.Sprintf("%v", r), buf[:n], seed.DeckKeys[:])
			}
		}
	}()
	return ReplayWithObservation(rc, seed, obs)
}

// ---------------------------------------------------------------------------
// Observation extraction helpers
// ---------------------------------------------------------------------------

// ExtractParserGaps scans all permanents across all seats for the
// "parser_gap" flag, which the engine's resolver sets when it encounters
// an unhandled ability kind.
//
// Names that look like runtime-generated token permanents (anything
// that's a bare "Token" or pure type-word(s) + "Token") are dropped:
// tokens carry no parseable oracle text of their own, so a parser_gap
// signal on a "Construct Token" or "creature token scorpion dragon
// Token" permanent is noise — the gap (if any) belongs to the source
// spell that minted the token, not the token's display name.
func ExtractParserGaps(gs *gameengine.GameState) []string {
	if gs == nil {
		return nil
	}
	seen := make(map[string]bool)
	var gaps []string

	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		// Check battlefield permanents.
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Flags != nil && p.Flags["parser_gap"] > 0 {
				name := p.Card.DisplayName()
				if isTypeOnlyTokenName(name) {
					continue
				}
				if !seen[name] {
					seen[name] = true
					gaps = append(gaps, name)
				}
			}
		}
		// Check graveyard — cards that hit parser_gap then died still
		// carry the flag on their source permanent's Card object, but
		// the permanent no longer exists. We can't recover those from
		// graveyard cards alone since the flag lives on the Permanent
		// struct. This is acceptable — the battlefield scan catches
		// most gaps, and Muninn accumulates across many replays.
	}
	return gaps
}

// mvpTopN bounds how many MVP records ExtractMVPCards returns per
// seat. Three is the user-visible default; the type docstring uses
// the constant so consumers can audit if it ever shifts.
const mvpTopN = 3

// mvpCommanderBonus is the score adder applied when an MVP candidate
// is one of the controlling seat's commanders. Reflects that a
// commander on the battlefield carries game-defining weight beyond
// its raw mana value.
const mvpCommanderBonus = 4

// mvpPositiveCounterKeys is the set of counter types that contribute
// to the MVP positive-counter tally. +1/+1, loyalty, charge, and
// level meaningfully reflect "the card grew or leveled up during
// the game." Other counter types (-1/-1, stun, time, etc.) are
// noise or negative for MVP purposes and are ignored.
var mvpPositiveCounterKeys = []string{"+1/+1", "loyalty", "charge", "level"}

// ExtractMVPCards walks every seat's battlefield at game end and
// returns the top mvpTopN permanents per seat by score, where:
//
//	score = EffectiveCMC + sum(positive counters) + commander bonus
//
// Basic lands are filtered out. Tokens and copies surface normally
// — their CMC is whatever the engine set on their backing Card.
// Results are appended seat-by-seat in seat-ascending order; within
// each seat's window, sort is score descending then card name
// ascending for determinism.
func ExtractMVPCards(gs *gameengine.GameState) []MVPCard {
	if gs == nil {
		return nil
	}
	var out []MVPCard
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		commanderSet := make(map[string]bool, len(seat.CommanderNames))
		for _, n := range seat.CommanderNames {
			commanderSet[n] = true
		}
		var perSeat []MVPCard
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "basic") {
				continue
			}
			cmc := p.Card.EffectiveCMC()
			counters := 0
			if p.Counters != nil {
				for _, key := range mvpPositiveCounterKeys {
					if v := p.Counters[key]; v > 0 {
						counters += v
					}
				}
			}
			isCmdr := commanderSet[p.Card.Name]
			score := cmc + counters
			if isCmdr {
				score += mvpCommanderBonus
			}
			turnPlayed := 0
			if gs.CardFirstPlayed != nil {
				turnPlayed = gs.CardFirstPlayed[p.Card.Name]
			}
			perSeat = append(perSeat, MVPCard{
				Seat:        seatIdx,
				CardName:    p.Card.Name,
				CMC:         cmc,
				TurnPlayed:  turnPlayed,
				Counters:    counters,
				Score:       score,
				IsCommander: isCmdr,
			})
		}
		sort.SliceStable(perSeat, func(i, j int) bool {
			if perSeat[i].Score != perSeat[j].Score {
				return perSeat[i].Score > perSeat[j].Score
			}
			return perSeat[i].CardName < perSeat[j].CardName
		})
		if len(perSeat) > mvpTopN {
			perSeat = perSeat[:mvpTopN]
		}
		out = append(out, perSeat...)
	}
	return out
}

// ExtractRegretCards walks every seat's hand at game end and returns
// one RegretCard per nonland CMC≥1 card still held — the "stranded
// in hand" half of the regret signal. Results are sorted by Seat
// ascending then CMC descending so the biggest stranded spell per
// seat surfaces first.
//
// Filtering rules:
//   - Skip cards with the "land" type (lands in hand aren't
//     intrinsically regretful; a flood/screw signal would be a
//     separate metric).
//   - Skip cards with EffectiveCMC < 1 (free spells, 0-cost
//     artifacts, modal DFC back-faces that report 0 — these are
//     held for reasons other than mana-affordability regret).
//   - Skip nil Cards defensively.
func ExtractRegretCards(gs *gameengine.GameState) []RegretCard {
	if gs == nil {
		return nil
	}
	var out []RegretCard
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, c := range seat.Hand {
			if c == nil {
				continue
			}
			if cardHasType(c, "land") {
				continue
			}
			cmc := c.EffectiveCMC()
			if cmc < 1 {
				continue
			}
			out = append(out, RegretCard{
				Seat:     seatIdx,
				CardName: c.Name,
				CMC:      cmc,
				Reason:   "stranded_in_hand",
			})
		}
	}
	// Stable sort: seat ascending, CMC descending within seat. A
	// tie-breaker on card name keeps the output deterministic when
	// two equal-CMC cards land in the same hand.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Seat != out[j].Seat {
			return out[i].Seat < out[j].Seat
		}
		if out[i].CMC != out[j].CMC {
			return out[i].CMC > out[j].CMC
		}
		return out[i].CardName < out[j].CardName
	})
	return out
}

// cardHasType is a hand-side mirror of Permanent.hasType — the engine
// stores Card.Types as lowercased tokens at parse time, so direct
// equality is sufficient.
func cardHasType(c *gameengine.Card, t string) bool {
	if c == nil {
		return false
	}
	for _, x := range c.Types {
		if x == t {
			return true
		}
	}
	return false
}

// ExtractCommanderZoneVisits walks every seat's commander list and
// returns one CommanderZoneVisit per (seat, commander) pair. Reads
// the cast count from Seat.CommanderCastCounts (§903.8 tax counter)
// and the end-state presence from Seat.CommandZone.
func ExtractCommanderZoneVisits(gs *gameengine.GameState) []CommanderZoneVisit {
	if gs == nil {
		return nil
	}
	var out []CommanderZoneVisit
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		// Build a quick lookup of commander names currently sitting in
		// this seat's command zone.
		inZone := make(map[string]bool, len(seat.CommandZone))
		for _, c := range seat.CommandZone {
			if c != nil {
				inZone[c.Name] = true
			}
		}
		for _, name := range seat.CommanderNames {
			cast := 0
			if seat.CommanderCastCounts != nil {
				cast = seat.CommanderCastCounts[name]
			}
			present := inZone[name]
			visits := cast
			if present {
				visits++
			}
			out = append(out, CommanderZoneVisit{
				Seat:          seatIdx,
				CommanderName: name,
				CastCount:     cast,
				InZoneAtEnd:   present,
				Visits:        visits,
			})
		}
	}
	return out
}

// ExtractCoTriggers finds pairs of cards that entered the battlefield
// within the same or adjacent turns. This is a simplified approximation
// of Huginn's full synergy analysis — real co-trigger scoring requires
// causal chain tracking, which lives in Huginn proper.
func ExtractCoTriggers(turnETBs map[int][]string) []CoTriggerPair {
	if len(turnETBs) == 0 {
		return nil
	}

	var pairs []CoTriggerPair

	// Collect all turns that had ETBs.
	turns := make([]int, 0, len(turnETBs))
	for t := range turnETBs {
		turns = append(turns, t)
	}

	// For each turn, pair cards within the same turn (window=0) and with
	// cards from the adjacent turn (window=1).
	for _, t := range turns {
		cards := turnETBs[t]
		// Same-turn pairs.
		for i := 0; i < len(cards); i++ {
			for j := i + 1; j < len(cards); j++ {
				pairs = append(pairs, CoTriggerPair{
					CardA:       cards[i],
					CardB:       cards[j],
					ImpactScore: 1.0, // same-turn co-entry baseline
					TurnWindow:  0,
				})
			}
		}
		// Adjacent-turn pairs (t and t+1).
		if nextCards, ok := turnETBs[t+1]; ok {
			for _, a := range cards {
				for _, b := range nextCards {
					if a == b {
						continue // skip self-pairs (e.g. tokens with same name)
					}
					pairs = append(pairs, CoTriggerPair{
						CardA:       a,
						CardB:       b,
						ImpactScore: 0.5, // adjacent-turn proximity discount
						TurnWindow:  1,
					})
				}
			}
		}
	}

	return pairs
}

// SnapshotBattlefieldNames returns a multiset (name -> count) of all
// permanent names currently on the battlefield.
func SnapshotBattlefieldNames(gs *gameengine.GameState) map[string]int {
	counts := make(map[string]int)
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			counts[p.Card.DisplayName()]++
		}
	}
	return counts
}

// DiffBattlefield returns names of permanents that are new in post
// relative to pre (accounting for counts).
func DiffBattlefield(pre, post map[string]int) []string {
	var newCards []string
	for name, postCount := range post {
		preCount := pre[name]
		for i := 0; i < postCount-preCount; i++ {
			newCards = append(newCards, name)
		}
	}
	return newCards
}

// isTypeOnlyTokenName returns true if name is a runtime-generated
// token permanent name (the bare "Token" sentinel or any name ending
// in " Token"). The engine's tokenName helper (resolve.go) and the
// hand-written token constructors (tokens.go, keywords_*.go, etc.)
// only ever produce names of the form "<type-words> Token" — no real
// MTG card name ends in "Token", so the suffix check is a safe proxy
// for "this is engine-minted, not a parseable oracle card."
func isTypeOnlyTokenName(name string) bool {
	if name == "Token" {
		return true
	}
	return strings.HasSuffix(name, " Token")
}

// nextLivingReplay is the replay-local version of nextLiving (which
// lives in the hexapi package and can't be imported from heimdall).
func nextLivingReplay(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for k := 1; k <= n; k++ {
		cand := (gs.Active + k) % n
		s := gs.Seats[cand]
		if s != nil && !s.Lost {
			return cand
		}
	}
	return gs.Active
}
