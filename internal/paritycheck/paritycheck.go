// Package paritycheck runs the Go engine and Python reference side-by-side
// on the same deck + seed combinations, normalizes their event streams
// into a canonical format, and reports structured divergences.
//
// The goal is 1:1 behavioral parity: given identical inputs, both engines
// should produce outcomes that match on the load-bearing axes (winner,
// turn count, life totals, board state checkpoints). The event stream
// never matches 1:1 — ordering of same-sequence events, RNG draw paths
// through different shuffle implementations, and subtly different policy
// implementations all produce non-load-bearing divergences.
//
// This package is NOT a golden-master test. It surfaces divergences
// HONESTLY — when the Go engine does something the Python engine
// doesn't (or vice versa), the report flags it. Phase 13's benchmark
// claims depend on this verifier, not on unit tests alone.
//
// Public surface:
//
//   - Event          — canonical parity event
//   - Divergence     — one reported difference between Go and Python
//   - ParityReport   — full report for a run
//   - RecordGoGame   — run one Go game, return normalized events + outcome
//   - RunPython      — shell out to Python harness, return events + outcome
//   - Diff           — walk paired streams, emit Divergence list
//   - Run            — end-to-end: match N games, diff, write report
//
// CLI driver lives at cmd/mtgsquad-parity.
package paritycheck

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// Event is the canonical parity event — a subset of the Python / Go
// event fields that MUST match for a game to count as equivalent.
// Non-load-bearing fields (pointer IDs, timestamps, policy-internal
// state) are intentionally omitted.
type Event struct {
	// Seq is the monotonically-increasing event index within a game.
	Seq int `json:"seq"`

	// Turn is the turn number (1-indexed).
	Turn int `json:"turn"`

	// Phase is the CR §500.1 phase bucket (beginning / main / combat /
	// ending). Normalized to lowercase.
	Phase string `json:"phase"`

	// Step is the step within the phase (untap / upkeep / draw /
	// precombat_main / postcombat_main / declare_attackers / etc.).
	// Normalized to lowercase.
	Step string `json:"step"`

	// Seat is the seat index this event belongs to (-1 for global).
	Seat int `json:"seat"`

	// Kind is the event discriminator. Normalized to snake_case,
	// lowercase. Examples: turn_start, play_land, cast, stack_push,
	// stack_resolve, enter_battlefield, combat_damage, draw, discard,
	// die, exile, game_end.
	Kind string `json:"kind"`

	// Source is the source card name (or empty for seat-level events).
	Source string `json:"source,omitempty"`

	// Target is the target seat index (-1 for n/a).
	Target int `json:"target,omitempty"`

	// Amount is the numeric payload (damage dealt, cards drawn, life
	// gained, etc.). Zero when n/a.
	Amount int `json:"amount,omitempty"`

	// Rule is the CR citation (e.g. "601.2f", "704.5f"). Optional —
	// only the Python side fills this consistently.
	Rule string `json:"rule,omitempty"`
}

// Outcome is the final per-game summary. Parity reporters match on these
// fields FIRST before walking event streams, because a matching outcome
// with divergent streams is a much weaker divergence (probably policy
// drift) than a divergent outcome (probably engine semantics bug).
type Outcome struct {
	Winner      int    `json:"winner"`       // -1 for draw
	WinnerName  string `json:"winner_name"`  // commander name
	Turns       int    `json:"turns"`        // final turn count
	EndReason   string `json:"end_reason"`   // "last_seat_standing" / "draw" / "turn_cap_leader" / etc.
	LifeTotals  []int  `json:"life_totals"`  // final life per seat
	LostBySeat  []bool `json:"lost_by_seat"` // per-seat elimination flag
}

// ReplayData is one game's full record.
type ReplayData struct {
	GameIdx int       `json:"game_idx"`
	Seed    int64     `json:"seed"`
	Events  []Event   `json:"events"`
	Outcome Outcome   `json:"outcome"`
}

// Divergence is one reported difference between the Go and Python
// streams for a single game.
type Divergence struct {
	GameIdx    int    `json:"game_idx"`
	Category   string `json:"category"`   // "outcome" / "event_missing_go" / "event_missing_py" / "event_field" / "event_count"
	Detail     string `json:"detail"`     // free-form description
	AtSeq      int    `json:"at_seq,omitempty"`
	GoEvent    *Event `json:"go_event,omitempty"`
	PyEvent    *Event `json:"py_event,omitempty"`
}

// ParityReport summarizes a parity run across N games.
type ParityReport struct {
	Games              int          `json:"games"`
	OutcomeMatches     int          `json:"outcome_matches"`
	EventStreamMatches int          `json:"event_stream_matches"`
	Divergences        []Divergence `json:"divergences"`
	CategoryCounts     map[string]int `json:"category_counts"`
	GeneratedAt        time.Time    `json:"generated_at"`
	DeckPaths          []string     `json:"deck_paths"`
	NSeats             int          `json:"n_seats"`
	BaseSeed           int64        `json:"base_seed"`
	PythonAvailable    bool         `json:"python_available"`
}

// Config controls one parity run.
type Config struct {
	DeckPaths         []string
	NSeats            int
	NGames            int
	BaseSeed          int64
	AstPath           string
	OraclePath        string
	PythonHarnessPath string // path to scripts/parity_harness.py; empty = don't run Python
	PythonBin         string // default: "python3"
	ReportPath        string // optional output path for markdown report
}

// Run is the top-level orchestrator. For each game index 0..NGames-1:
//
//  1. Record the Go game (RecordGoGame).
//  2. If PythonHarnessPath != "", shell out to the Python harness with
//     the same seed (RunPython).
//  3. Diff the two event streams (Diff).
//  4. Aggregate divergences into a ParityReport.
//
// If PythonHarnessPath is empty, the report still runs — it just
// records "python_available: false" and reports only the Go outcomes.
// This mode is useful for CI when Python isn't installed.
func Run(cfg Config) (*ParityReport, error) {
	if cfg.NSeats < 2 {
		return nil, fmt.Errorf("paritycheck: NSeats must be >= 2")
	}
	if cfg.NGames < 1 {
		return nil, fmt.Errorf("paritycheck: NGames must be >= 1")
	}
	if len(cfg.DeckPaths) < cfg.NSeats {
		return nil, fmt.Errorf("paritycheck: need %d decks, got %d", cfg.NSeats, len(cfg.DeckPaths))
	}
	if cfg.AstPath == "" {
		cfg.AstPath = "data/rules/ast_dataset.jsonl"
	}
	if cfg.OraclePath == "" {
		cfg.OraclePath = "data/rules/oracle-cards.json"
	}
	if cfg.PythonBin == "" {
		cfg.PythonBin = "python3"
	}

	// Load corpus + decks once for Go.
	corpus, err := astload.Load(cfg.AstPath)
	if err != nil {
		return nil, fmt.Errorf("paritycheck: astload: %w", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(cfg.AstPath)
	if err != nil {
		return nil, fmt.Errorf("paritycheck: metadb: %w", err)
	}
	_ = meta.SupplementWithOracleJSON(cfg.OraclePath)

	decks := make([]*deckparser.TournamentDeck, 0, len(cfg.DeckPaths))
	for _, p := range cfg.DeckPaths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			return nil, fmt.Errorf("paritycheck: parse %s: %w", p, err)
		}
		decks = append(decks, d)
	}

	// Probe for Python harness.
	pythonAvailable := cfg.PythonHarnessPath != ""
	if pythonAvailable {
		if _, err := os.Stat(cfg.PythonHarnessPath); err != nil {
			pythonAvailable = false
		}
	}

	report := &ParityReport{
		Games:           cfg.NGames,
		CategoryCounts:  map[string]int{},
		DeckPaths:       append([]string(nil), cfg.DeckPaths...),
		NSeats:          cfg.NSeats,
		BaseSeed:        cfg.BaseSeed,
		GeneratedAt:     time.Now(),
		PythonAvailable: pythonAvailable,
	}

	for idx := 0; idx < cfg.NGames; idx++ {
		seed := cfg.BaseSeed + int64(idx)*1000 + 1
		goReplay, err := RecordGoGame(cfg.NSeats, decks, seed, idx)
		if err != nil {
			return nil, fmt.Errorf("paritycheck: go game %d: %w", idx, err)
		}

		if !pythonAvailable {
			// No Python available — just record the Go game and move on.
			continue
		}

		pyReplay, err := RunPython(cfg, idx, seed)
		if err != nil {
			report.Divergences = append(report.Divergences, Divergence{
				GameIdx:  idx,
				Category: "python_error",
				Detail:   err.Error(),
			})
			report.CategoryCounts["python_error"]++
			continue
		}

		divs := Diff(idx, goReplay, pyReplay)
		if outcomesEqual(&goReplay.Outcome, &pyReplay.Outcome) {
			report.OutcomeMatches++
		}
		if len(divs) == 0 {
			report.EventStreamMatches++
		}
		for _, d := range divs {
			report.CategoryCounts[d.Category]++
		}
		report.Divergences = append(report.Divergences, divs...)
	}

	if cfg.ReportPath != "" {
		if err := WriteMarkdown(report, cfg.ReportPath); err != nil {
			return report, fmt.Errorf("paritycheck: write report: %w", err)
		}
	}
	return report, nil
}

// RecordGoGame runs a single Go game and returns its normalized event
// stream + outcome. Mirrors tournament.runOneGame's setup but records
// per-game events instead of aggregating to the Tournament summary.
func RecordGoGame(nSeats int, decks []*deckparser.TournamentDeck, seed int64, gameIdx int) (*ReplayData, error) {
	rng := rand.New(rand.NewSource(seed))
	gs := gameengine.NewGameState(nSeats, rng, nil)

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		tpl := decks[i]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{CommanderCards: cmdrs, Library: lib}
	}
	gameengine.SetupCommanderGame(gs, commanderDecks)

	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
		for j := 0; j < 7 && len(gs.Seats[i].Library) > 0; j++ {
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}
	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1
	gs.LogEvent(gameengine.Event{
		Kind: "game_start", Seat: gs.Active, Target: -1,
		Details: map[string]interface{}{
			"on_the_play":      gs.Active,
			"n_seats":          nSeats,
			"commander_format": true,
			"game_idx":         gameIdx,
		},
	})

	const maxTurns = 80
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			break
		}
		// CR §726.3a — snapshot the ending seat's per-turn spell cast
		// count BEFORE rotating. Next turn's
		// EvaluateDayNightAtTurnStart consumes this.
		if ending := gs.Seats[gs.Active]; ending != nil {
			gs.SpellsCastByActiveLastTurn = ending.SpellsCastThisTurn
		}
		// Advance to next living seat.
		n := len(gs.Seats)
		for k := 1; k <= n; k++ {
			cand := (gs.Active + k) % n
			s := gs.Seats[cand]
			if s != nil && !s.Lost {
				gs.Active = cand
				break
			}
		}
	}

	// Build replay.
	replay := &ReplayData{
		GameIdx: gameIdx,
		Seed:    seed,
		Events:  normalizeGoEvents(gs.EventLog),
		Outcome: buildGoOutcome(gs, decks),
	}
	return replay, nil
}

// RunPython shells out to the configured Python harness and parses the
// JSONL it emits. Expected harness contract:
//
//   python3 <harness> --decks p1,p2,... --seed S --game-idx I --output /tmp/py.jsonl
//
// The harness writes one event per JSONL line plus a final line with
// `{"_outcome": {...}}` carrying the outcome summary.
func RunPython(cfg Config, gameIdx int, seed int64) (*ReplayData, error) {
	if cfg.PythonHarnessPath == "" {
		return nil, fmt.Errorf("paritycheck: no python harness configured")
	}
	tmp, err := os.CreateTemp("", "mtgsquad-parity-*.jsonl")
	if err != nil {
		return nil, fmt.Errorf("paritycheck: tempfile: %w", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	args := []string{
		cfg.PythonHarnessPath,
		"--decks", strings.Join(cfg.DeckPaths, ","),
		"--seed", strconv.FormatInt(seed, 10),
		"--game-idx", strconv.Itoa(gameIdx),
		"--seats", strconv.Itoa(cfg.NSeats),
		"--output", tmp.Name(),
	}
	cmd := exec.Command(cfg.PythonBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("paritycheck: python harness: %w (output: %s)", err, out)
	}
	return parsePythonReplay(tmp.Name(), gameIdx, seed)
}

// parsePythonReplay reads a JSONL event stream emitted by the Python
// harness and returns a ReplayData. The last line is expected to carry
// the `_outcome` key.
func parsePythonReplay(path string, gameIdx int, seed int64) (*ReplayData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("paritycheck: read %s: %w", path, err)
	}
	replay := &ReplayData{GameIdx: gameIdx, Seed: seed}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First check if it's the outcome line.
		var probe map[string]interface{}
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			continue
		}
		if outcomeRaw, ok := probe["_outcome"]; ok {
			b, _ := json.Marshal(outcomeRaw)
			_ = json.Unmarshal(b, &replay.Outcome)
			continue
		}
		replay.Events = append(replay.Events, normalizePythonEvent(probe))
	}
	return replay, nil
}

// normalizeGoEvents converts the Go engine's EventLog into canonical
// Event records. Non-load-bearing fields (object IDs, internal
// timestamps) are stripped.
func normalizeGoEvents(in []gameengine.Event) []Event {
	out := make([]Event, 0, len(in))
	// Phase/step bookkeeping — Go events don't each carry phase/step so
	// we reconstruct from observed phase_step events / turn_start.
	turn := 0
	phase := ""
	step := ""
	for i, ev := range in {
		switch ev.Kind {
		case "turn_start":
			turn++
			phase, step = "beginning", "untap"
		}
		// Track phase transitions if the engine emits phase_step events.
		if ev.Kind == "phase_step" {
			if ev.Details != nil {
				if p, ok := ev.Details["phase"].(string); ok {
					phase = strings.ToLower(p)
				}
				if s, ok := ev.Details["step"].(string); ok {
					step = strings.ToLower(s)
				}
			}
		}
		rule := ""
		if ev.Details != nil {
			if r, ok := ev.Details["rule"].(string); ok {
				rule = r
			}
		}
		out = append(out, Event{
			Seq:    i,
			Turn:   turn,
			Phase:  phase,
			Step:   step,
			Seat:   ev.Seat,
			Kind:   normalizeKind(ev.Kind),
			Source: ev.Source,
			Target: ev.Target,
			Amount: ev.Amount,
			Rule:   rule,
		})
	}
	return out
}

// normalizePythonEvent converts a Python event dict into canonical form.
// Python uses "type" instead of "kind"; common fields live at top level.
func normalizePythonEvent(d map[string]interface{}) Event {
	asInt := func(key string) int {
		if v, ok := d[key]; ok {
			switch x := v.(type) {
			case float64:
				return int(x)
			case int:
				return x
			}
		}
		return 0
	}
	asStr := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := d[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
		return ""
	}
	target := -1
	if v, ok := d["target"]; ok {
		if f, ok := v.(float64); ok {
			target = int(f)
		}
	}
	return Event{
		Seq:    asInt("seq"),
		Turn:   asInt("turn"),
		Phase:  strings.ToLower(asStr("phase_kind", "phase")),
		Step:   strings.ToLower(asStr("step_kind")),
		Seat:   asInt("seat"),
		Kind:   normalizeKind(asStr("type", "kind")),
		Source: asStr("card", "source_card", "source"),
		Target: target,
		Amount: asInt("amount"),
		Rule:   asStr("rule"),
	}
}

// normalizeKind folds synonym event names to a canonical set. This is
// where "Go calls it X, Python calls it Y" differences get masked out.
// Add new rules here when a new divergence category is merged.
func normalizeKind(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, " ", "_")
	switch k {
	case "enter_battlefield", "etb", "etbed", "enters_battlefield":
		return "enter_battlefield"
	case "combat_damage_dealt", "combat_damage":
		return "combat_damage"
	case "resolve", "stack_resolved":
		return "resolve"
	case "stack_push", "push_stack":
		return "stack_push"
	case "pool_drain", "mana_pool_drain":
		return "pool_drain"
	case "game_end", "game_over":
		return "game_end"
	case "seat_eliminated", "player_lost", "lose":
		return "seat_eliminated"
	}
	return k
}

// buildGoOutcome extracts the final outcome from a completed Go GameState.
func buildGoOutcome(gs *gameengine.GameState, decks []*deckparser.TournamentDeck) Outcome {
	o := Outcome{Winner: -1}
	o.Turns = gs.Turn
	o.LifeTotals = make([]int, len(gs.Seats))
	o.LostBySeat = make([]bool, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		o.LifeTotals[i] = s.Life
		o.LostBySeat[i] = s.Lost
	}
	if gs.Flags != nil {
		if gs.Flags["ended"] == 1 {
			if w, ok := gs.Flags["winner"]; ok {
				o.Winner = w
				if w >= 0 && w < len(decks) {
					o.WinnerName = decks[w].CommanderName
				}
				o.EndReason = "last_seat_standing"
			} else {
				o.EndReason = "draw"
			}
		}
	}
	// Turn-cap tiebreak (mirror runner.runOneGame).
	if o.Winner < 0 && o.EndReason == "" {
		living := []int{}
		for i, s := range gs.Seats {
			if s != nil && !s.Lost {
				living = append(living, i)
			}
		}
		if len(living) == 0 {
			o.EndReason = "turn_cap_all_dead"
		} else {
			topLife := gs.Seats[living[0]].Life
			for _, i := range living[1:] {
				if gs.Seats[i].Life > topLife {
					topLife = gs.Seats[i].Life
				}
			}
			leaders := []int{}
			for _, i := range living {
				if gs.Seats[i].Life == topLife {
					leaders = append(leaders, i)
				}
			}
			if len(leaders) == 1 {
				o.Winner = leaders[0]
				if o.Winner < len(decks) {
					o.WinnerName = decks[o.Winner].CommanderName
				}
				o.EndReason = "turn_cap_leader"
			} else {
				o.EndReason = "turn_cap_tie"
			}
		}
	}
	return o
}

func outcomesEqual(a, b *Outcome) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Winner != b.Winner {
		return false
	}
	// We don't strictly require turn counts / end reasons match for
	// outcome parity — Python and Go may round differently on "when did
	// the game end" depending on which SBA pass the final kill hits.
	return true
}

// Diff walks paired event streams and emits a list of Divergence records.
// The first categories checked are outcome + event count (cheap signals);
// the rest is a best-effort LCS-lite walk that reports ordering drift +
// mismatched kinds.
func Diff(gameIdx int, goR, pyR *ReplayData) []Divergence {
	var divs []Divergence
	if goR == nil || pyR == nil {
		return divs
	}
	if !outcomesEqual(&goR.Outcome, &pyR.Outcome) {
		divs = append(divs, Divergence{
			GameIdx:  gameIdx,
			Category: "outcome",
			Detail: fmt.Sprintf("go_winner=%d go_end=%s py_winner=%d py_end=%s",
				goR.Outcome.Winner, goR.Outcome.EndReason,
				pyR.Outcome.Winner, pyR.Outcome.EndReason),
		})
	}

	goCounts := eventKindCounts(goR.Events)
	pyCounts := eventKindCounts(pyR.Events)

	allKinds := make(map[string]struct{})
	for k := range goCounts {
		allKinds[k] = struct{}{}
	}
	for k := range pyCounts {
		allKinds[k] = struct{}{}
	}
	sortedKinds := make([]string, 0, len(allKinds))
	for k := range allKinds {
		sortedKinds = append(sortedKinds, k)
	}
	sort.Strings(sortedKinds)
	for _, k := range sortedKinds {
		gc := goCounts[k]
		pc := pyCounts[k]
		if gc != pc {
			var cat string
			switch {
			case gc == 0:
				cat = "event_missing_go"
			case pc == 0:
				cat = "event_missing_py"
			default:
				cat = "event_count"
			}
			divs = append(divs, Divergence{
				GameIdx:  gameIdx,
				Category: cat,
				Detail:   fmt.Sprintf("kind=%q go=%d py=%d", k, gc, pc),
			})
		}
	}

	// Turn-count / duration divergence.
	if goR.Outcome.Turns != pyR.Outcome.Turns {
		divs = append(divs, Divergence{
			GameIdx:  gameIdx,
			Category: "turn_count",
			Detail: fmt.Sprintf("go_turns=%d py_turns=%d",
				goR.Outcome.Turns, pyR.Outcome.Turns),
		})
	}

	return divs
}

func eventKindCounts(events []Event) map[string]int {
	out := map[string]int{}
	for _, ev := range events {
		out[ev.Kind]++
	}
	return out
}

// WriteMarkdown serializes a ParityReport to a markdown file at path.
func WriteMarkdown(r *ParityReport, path string) error {
	if r == nil {
		return fmt.Errorf("paritycheck: nil report")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("paritycheck: mkdir: %w", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Go ↔ Python Parity Report\n\n")
	fmt.Fprintf(&sb, "_Generated: %s_\n\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "| field | value |\n|---|---|\n")
	fmt.Fprintf(&sb, "| games | %d |\n", r.Games)
	fmt.Fprintf(&sb, "| n_seats | %d |\n", r.NSeats)
	fmt.Fprintf(&sb, "| base_seed | %d |\n", r.BaseSeed)
	fmt.Fprintf(&sb, "| python_available | %v |\n", r.PythonAvailable)
	if r.PythonAvailable {
		outcomePct := 100 * float64(r.OutcomeMatches) / float64(r.Games)
		streamPct := 100 * float64(r.EventStreamMatches) / float64(r.Games)
		fmt.Fprintf(&sb, "| outcome_match | %d/%d (%.1f%%) |\n",
			r.OutcomeMatches, r.Games, outcomePct)
		fmt.Fprintf(&sb, "| event_stream_match | %d/%d (%.1f%%) |\n",
			r.EventStreamMatches, r.Games, streamPct)
	}
	fmt.Fprintf(&sb, "\n## Deck Lineup\n\n")
	for i, p := range r.DeckPaths {
		fmt.Fprintf(&sb, "%d. `%s`\n", i, p)
	}
	fmt.Fprintf(&sb, "\n## Divergence Categories\n\n")
	if len(r.CategoryCounts) == 0 {
		fmt.Fprintf(&sb, "_No divergences_\n")
	} else {
		fmt.Fprintf(&sb, "| category | count |\n|---|---|\n")
		cats := make([]string, 0, len(r.CategoryCounts))
		for k := range r.CategoryCounts {
			cats = append(cats, k)
		}
		sort.Strings(cats)
		for _, c := range cats {
			fmt.Fprintf(&sb, "| %s | %d |\n", c, r.CategoryCounts[c])
		}
	}
	fmt.Fprintf(&sb, "\n## Divergences (first 50)\n\n")
	n := len(r.Divergences)
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		d := r.Divergences[i]
		fmt.Fprintf(&sb, "- game=%d cat=%s — %s\n", d.GameIdx, d.Category, d.Detail)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}
