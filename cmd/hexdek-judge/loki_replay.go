package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Loki replay integration — judge ↔ Loki bridge.
//
// hexdek-loki runs randomized chaos games and records every invariant
// violation. The violation record today is a free-text "InvariantName +
// Message" pair with a state-summary blob — useful for human triage
// but hard to map mechanically to a Comprehensive Rules citation. This
// file adds the `--replay <path>` flag to hexdek-judge: load a Loki
// replay JSON, walk each violation event, run the §704 SBA probe
// against the embedded game-state snapshot, and surface a structured
// "this invariant maps to CR §X.Y.Z, here is the failing condition"
// analysis per event.
//
// Use case: a Loki ZoneConservation hit gets explained as "indicates a
// CR §400.6 zone-change integrity issue or a §704.5d/e cleanup miss —
// here is the embedded SBA snapshot, and here are the §704.5 / §704.6
// conditions the probe finds in that state." That collapses the
// "stare at the message + summary, manually replay the events,
// re-derive the CR citation" loop into a structured artifact.
//
// Loki itself doesn't yet emit this JSON format. The shape defined
// here is the FORWARD contract: a stable schema that downstream
// tooling (Loki, replay-analyzers, CI gates) can target. The probe
// works equally well against hand-rolled replay fixtures (the test
// suite uses this), Loki output once the corresponding loki-side
// writer lands, and any other producer that conforms to the
// LokiReplay shape.

// LokiReplay is the top-level JSON shape produced by Loki (or hand-
// authored by a test fixture). Each entry in Violations carries
// enough metadata to identify the chaos game + turn it came from
// PLUS optionally an SBASnapshot the judge can probe against.
type LokiReplay struct {
	Meta       LokiReplayMeta       `json:"meta"`
	Violations []LokiReplayViolation `json:"violations"`
}

// LokiReplayMeta is the run-level metadata.
type LokiReplayMeta struct {
	Source     string `json:"source,omitempty"`      // "hexdek-loki r60" / similar
	CorpusSize int    `json:"corpus_size,omitempty"`
	TotalGames int    `json:"total_games,omitempty"`
	Seed       int64  `json:"seed,omitempty"`
	RunAt      string `json:"run_at,omitempty"`      // RFC3339 timestamp
}

// LokiReplayViolation is one chaos invariant violation. Mirrors the
// fields chaosViolation tracks in cmd/hexdek-loki/main.go plus an
// optional embedded SBASnapshot — when present, the judge runs the
// §704 SBA probe against it. When absent, the probe degrades
// gracefully (the CR-citation explanation still ships).
type LokiReplayViolation struct {
	GameIdx       int          `json:"game_idx"`
	GameSeed      int64        `json:"game_seed,omitempty"`
	Permutation   int          `json:"permutation,omitempty"`
	InvariantName string       `json:"invariant_name"`
	Message       string       `json:"message"`
	Turn          int          `json:"turn,omitempty"`
	Phase         string       `json:"phase,omitempty"`
	Step          string       `json:"step,omitempty"`
	StateSummary  string       `json:"state_summary,omitempty"`
	RecentEvents  []string     `json:"recent_events,omitempty"`
	Commanders    []string     `json:"commanders,omitempty"`
	SBASnapshot   *SBASnapshot `json:"sba_snapshot,omitempty"`
}

// invariantCRCitations maps each engine invariant name to one or more
// Comprehensive Rules sub-sections the invariant probes.
//
// A single invariant can map to multiple CR rules because the engine
// invariants are higher-level than the CR sub-sections — e.g.
// LifeConsistency catches both §704.5a (life ≤ 0 should trigger loss)
// and §119 (life total bookkeeping). The map captures every CR
// section a maintainer should consult when triaging a hit.
//
// Sync invariant: when AllInvariants() in internal/gameengine/invariants.go
// adds an entry, this map should pick up the corresponding CR
// citation. Unknown invariant names default to the §700 catch-all
// and a "no specific citation" reason in explainInvariant.
var invariantCRCitations = map[string][]CRCitation{
	"LifeConsistency": {
		{Rule: "704.5a", Description: "A player with life total 0 or less loses the game"},
		{Rule: "119", Description: "Life total bookkeeping"},
	},
	"SBACompleteness": {
		{Rule: "704.3", Description: "Whenever a player would get priority, the game first performs all applicable state-based actions as a single event"},
		{Rule: "704.5", Description: "Some §704.5 state-based action did not fire when its condition was met"},
	},
	"WinCondition": {
		{Rule: "104.2", Description: "The game ends when a player wins, loses, or draws"},
		{Rule: "104.3", Description: "Game-end conditions are checked"},
		{Rule: "704.5a", Description: "Life ≤ 0 loss check"},
		{Rule: "704.5b", Description: "Draw-from-empty-library loss check"},
		{Rule: "704.5c", Description: "10+ poison counters loss check"},
	},
	"ZoneConservation": {
		{Rule: "400.6", Description: "Zone changes preserve object identity unless the rules explicitly say otherwise"},
		{Rule: "109.3", Description: "Object identity across zone changes"},
		{Rule: "704.5d", Description: "Token in non-battlefield zone ceases to exist"},
		{Rule: "704.5e", Description: "Copy of a spell/ability in non-stack zone ceases to exist"},
	},
	"CardIdentity": {
		{Rule: "109.3", Description: "An object's characteristics persist across zone changes"},
		{Rule: "400.7", Description: "Object's information is re-established when it moves zones"},
	},
	"AttachmentConsistency": {
		{Rule: "704.5k", Description: "Aura attached to an illegal object goes to graveyard"},
		{Rule: "704.5m", Description: "Aura not attached to anything goes to graveyard"},
		{Rule: "704.5n", Description: "Equipment/Fortification attached to an illegal target detaches"},
		{Rule: "303.4", Description: "Aura targeting / attachment legality"},
	},
	"TriggerCompleteness": {
		{Rule: "603.2", Description: "Whenever a triggered ability's trigger event occurs, the ability triggers"},
		{Rule: "603.3", Description: "Triggered abilities are put on the stack the next time a player would receive priority"},
	},
	"ReplacementCompleteness": {
		{Rule: "614.1", Description: "Replacement effects modify or replace events as they would happen"},
		{Rule: "614.6", Description: "Replacement effects that need the source's presence apply only while the source is in the right zone"},
	},
	"CounterAccuracy": {
		{Rule: "122.1", Description: "Counter placement and removal bookkeeping"},
		{Rule: "704.5q", Description: "+1/+1 and -1/-1 counter pair removal"},
		{Rule: "704.5p", Description: "Pair-removal cleanup"},
	},
	"CombatLegality": {
		{Rule: "506.2", Description: "Combat phase steps and declarations"},
		{Rule: "508", Description: "Declare attackers step legality"},
		{Rule: "509", Description: "Declare blockers step legality"},
	},
	"TurnStructure": {
		{Rule: "500.1", Description: "Turns are made up of phases, each of which contains steps"},
		{Rule: "500.4", Description: "Phase and step transitions"},
	},
	"Timing": {
		{Rule: "405", Description: "The stack and priority"},
		{Rule: "117", Description: "Timing and priority"},
	},
	"ResourceConservation": {
		{Rule: "106", Description: "Mana"},
		{Rule: "105", Description: "Life total"},
		{Rule: "122", Description: "Counters"},
	},
	"StackOrderCorrectness": {
		{Rule: "608", Description: "Resolving spells and abilities"},
		{Rule: "608.2", Description: "Stack resolution order"},
	},
	"ExileLinkageIntegrity": {
		{Rule: "406.6", Description: "Exile zone is a public zone; cards remain trackable"},
		{Rule: "400.7", Description: "Object identity across exile + return"},
	},
	"ZoneCastGrantExpiry": {
		{Rule: "611", Description: "Continuous effects and their duration"},
		{Rule: "702", Description: "Keyword abilities and their grants"},
	},
	"IndestructibleRespected": {
		{Rule: "702.12", Description: "Indestructible permanents aren't destroyed by damage or destroy effects"},
	},
	"LayerIdempotency": {
		{Rule: "613", Description: "Interaction of continuous effects (layer system)"},
	},
	"StackIntegrity": {
		{Rule: "608", Description: "Resolving spells and abilities"},
		{Rule: "405", Description: "The stack"},
	},
}

// CRCitation is one mapped Comprehensive Rules sub-section.
type CRCitation struct {
	Rule        string `json:"rule"`        // "704.5a"
	Description string `json:"description"` // "A player with life total 0 or less loses the game"
}

// ReplayAnalysis is the top-level JSON output shape from --replay.
type ReplayAnalysis struct {
	Rule              string                    `json:"rule"`        // "CR §700 (judge ↔ Loki replay)"
	ReplayPath        string                    `json:"replay_path"`
	Meta              LokiReplayMeta            `json:"meta"`
	ViolationCount    int                       `json:"violation_count"`
	WithSnapshot      int                       `json:"with_snapshot"`
	Analyses          []ViolationAnalysis       `json:"analyses"`
	InvariantsSeen    map[string]int            `json:"invariants_seen"`
	UnmappedInvariants []string                 `json:"unmapped_invariants,omitempty"`
	Valid             bool                      `json:"valid"`
}

// ViolationAnalysis is one Loki violation's enriched explanation.
type ViolationAnalysis struct {
	GameIdx       int               `json:"game_idx"`
	Turn          int               `json:"turn,omitempty"`
	InvariantName string            `json:"invariant_name"`
	Message       string            `json:"message"`
	CRCitations   []CRCitation      `json:"cr_citations"`
	SBAFindings   []SBAViolation    `json:"sba_findings,omitempty"`
	Summary       string            `json:"summary"`
	Commanders    []string          `json:"commanders,omitempty"`
}

// runReplayAnalysis is the CLI entry point. Loads the replay JSON,
// walks each violation, enriches it with CR citations and SBA probe
// output, and writes a structured ReplayAnalysis JSON.
func runReplayAnalysis(replayPath, outPath string) (*ReplayAnalysis, error) {
	if replayPath == "" {
		return nil, fmt.Errorf("--replay requires a path")
	}
	f, err := os.Open(replayPath)
	if err != nil {
		return nil, fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()
	var replay LokiReplay
	if err := json.NewDecoder(f).Decode(&replay); err != nil {
		return nil, fmt.Errorf("decode replay: %w", err)
	}

	rep := &ReplayAnalysis{
		Rule:           "CR §700 (judge ↔ Loki replay)",
		ReplayPath:     replayPath,
		Meta:           replay.Meta,
		ViolationCount: len(replay.Violations),
		InvariantsSeen: map[string]int{},
		Analyses:       []ViolationAnalysis{},
	}
	unmappedSeen := map[string]bool{}
	for _, v := range replay.Violations {
		v := v
		rep.InvariantsSeen[v.InvariantName]++
		analysis := analyzeViolation(&v)
		if v.SBASnapshot != nil {
			rep.WithSnapshot++
		}
		if len(analysis.CRCitations) == 0 && !unmappedSeen[v.InvariantName] {
			rep.UnmappedInvariants = append(rep.UnmappedInvariants, v.InvariantName)
			unmappedSeen[v.InvariantName] = true
		}
		rep.Analyses = append(rep.Analyses, analysis)
	}
	sort.Strings(rep.UnmappedInvariants)
	// Replay analysis is informational: presence of violations isn't
	// itself an "error" — the violations ARE the artifact. Valid
	// reports whether the replay file parsed and was structurally
	// well-formed (it did, so true).
	rep.Valid = true

	return rep, writeReplayAnalysis(rep, outPath)
}

// analyzeViolation enriches one Loki violation with the CR citation
// map + SBA probe output (when the snapshot is embedded).
func analyzeViolation(v *LokiReplayViolation) ViolationAnalysis {
	a := ViolationAnalysis{
		GameIdx:       v.GameIdx,
		Turn:          v.Turn,
		InvariantName: v.InvariantName,
		Message:       v.Message,
		Commanders:    v.Commanders,
	}
	cites := invariantCRCitations[v.InvariantName]
	a.CRCitations = cites
	if v.SBASnapshot != nil {
		a.SBAFindings = detectSBAViolations(v.SBASnapshot)
	}
	a.Summary = explainInvariant(v.InvariantName, len(a.SBAFindings))
	return a
}

// explainInvariant returns a one-line summary suitable for human
// triage. The format matches what a judge would say at a tournament:
// invariant name, what it generally means, how many §704 conditions
// the embedded snapshot tickles.
func explainInvariant(name string, sbaCount int) string {
	cites := invariantCRCitations[name]
	if len(cites) == 0 {
		base := fmt.Sprintf("%s — no CR citation mapped (engine-specific invariant)", name)
		if sbaCount > 0 {
			return fmt.Sprintf("%s. Embedded snapshot also tickles %d §704 SBA condition(s)", base, sbaCount)
		}
		return base
	}
	primary := cites[0]
	primaryStr := fmt.Sprintf("%s indicates CR §%s — %s", name, primary.Rule, primary.Description)
	if len(cites) > 1 {
		extras := make([]string, 0, len(cites)-1)
		for _, c := range cites[1:] {
			extras = append(extras, "§"+c.Rule)
		}
		primaryStr += fmt.Sprintf(" (also %s)", strings.Join(extras, ", "))
	}
	if sbaCount > 0 {
		primaryStr += fmt.Sprintf(". Embedded snapshot tickles %d §704 SBA condition(s)", sbaCount)
	}
	return primaryStr
}

func writeReplayAnalysis(rep *ReplayAnalysis, outPath string) error {
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
