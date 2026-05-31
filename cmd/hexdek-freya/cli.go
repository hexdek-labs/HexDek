package main

import (
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
)

// cli — Freya CLI UX helpers: --version output, --json-schema doc, and
// the multi-section Usage banner. Extracted so the strings can be
// regression-pinned in tests without spawning the binary.

// VersionString returns the canonical multi-line version banner. The
// FreyaVersion constant doubles as the cache-invalidation token (see
// cache.go), so the two stay in sync by construction — a single bump
// invalidates both stored caches and the runtime version stamp.
func VersionString() string {
	var b strings.Builder
	fmt.Fprintf(&b, "hexdek-freya %s\n", FreyaVersion)
	fmt.Fprintf(&b, "  cache schema: v%s (see %s/<key>-v%s.json)\n",
		FreyaVersion, DefaultCacheDir, FreyaVersion)
	fmt.Fprintf(&b, "  go runtime:   %s\n", runtime.Version())
	return b.String()
}

// JSONSchemaDoc returns a concise human-readable summary of the JSON
// output emitted by `--format json`. The renderer's full struct is
// FreyaReport (analysis.go); the doc here surfaces the top-level
// fields most consumers care about, plus the cache entry envelope so
// anyone reading a file under data/freya-cache/ knows the layout.
//
// Kept intentionally short — this is a discovery aid, not a full
// schema. Downstream consumers should generate JSON-schema bindings
// from the Go structs directly if they need stricter contracts.
func JSONSchemaDoc() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Freya JSON Output Schema (v%s)\n", FreyaVersion)
	fmt.Fprintln(&b, "================================")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "TOP LEVEL: FreyaReport")
	fmt.Fprintln(&b, "  DeckName            string   — basename of the deck file")
	fmt.Fprintln(&b, "  DeckPath            string   — absolute path of the source deck file")
	fmt.Fprintln(&b, "  Commander           string   — commander card name (empty when not detected)")
	fmt.Fprintln(&b, "  TotalCards          int      — count after deck list parsing")
	fmt.Fprintln(&b, "  Profiles            []CardProfile        — per-unique-card classification")
	fmt.Fprintln(&b, "  TrueInfinites       []ComboResult        — mandatory-loop combos")
	fmt.Fprintln(&b, "  Determined          []ComboResult        — player-choice loop combos")
	fmt.Fprintln(&b, "  Finishers           []ComboResult        — closer pieces (1+ cards)")
	fmt.Fprintln(&b, "  Synergies           []ComboResult        — non-winning value pairs/triples")
	fmt.Fprintln(&b, "  GraveyardLoops      []ComboResult        — combo + graveyard-recursion enabler")
	fmt.Fprintln(&b, "  LandCycleSynergies  []ComboResult        — dual-cycle land pairs")
	fmt.Fprintln(&b, "  TutorCount          int                  — all tutors (incl. land tutors)")
	fmt.Fprintln(&b, "  NonLandTutorCount   int                  — consistency-engine tutors only")
	fmt.Fprintln(&b, "  ManaCurve           [8]int               — CMC 0-6 + 7+ bucket")
	fmt.Fprintln(&b, "  AvgCMC              float64              — nonland mean CMC")
	fmt.Fprintln(&b, "  ColorDemand         map[WUBRG]int        — total pips needed")
	fmt.Fprintln(&b, "  ColorSupply         map[WUBRG]int        — total sources")
	fmt.Fprintln(&b, "  Stats               *DeckStatistics      — Phase-1 numeric rollups")
	fmt.Fprintln(&b, "  Roles               *RoleAnalysis        — per-card role tags + counts")
	fmt.Fprintln(&b, "  Archetype           *ArchetypeClassification  — Primary, PrimaryConfidence, Bracket")
	fmt.Fprintln(&b, "  WinLines            *WinLineAnalysis     — assembled win conditions + tutor coverage")
	fmt.Fprintln(&b, "  Profile             *DeckProfile         — unified per-deck rollup")
	fmt.Fprintln(&b, "  ComboInteraction    *ComboInteractionMatrix   — pairwise combo dependencies")
	fmt.Fprintln(&b, "  Legality            *LegalityReport      — format / banlist findings")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "COMBO ENTRY: ComboResult")
	fmt.Fprintln(&b, "  Cards            []string  — piece names")
	fmt.Fprintln(&b, "  LoopType         string    — determined / true_infinite / finisher / synergy")
	fmt.Fprintln(&b, "  Class            string    — what the combo produces (infinite_mana, etc.)")
	fmt.Fprintln(&b, "  Description      string    — human-readable summary")
	fmt.Fprintln(&b, "  Confirmed        bool      — true when matched from KnownCombos database")
	fmt.Fprintln(&b, "  NonDeterministic bool      — depends on random selection")
	fmt.Fprintln(&b, "  Annotation       *LoopAnnotation  — graph-walked loop metadata (5+-card)")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "METRICS MODE: FreyaMetrics  (--mode metrics --format json)")
	fmt.Fprintln(&b, "  ConfidenceHighPct    float64  — % of items >= 0.8 confidence")
	fmt.Fprintln(&b, "  CardWithExplicitTagPct float64 — % of non-land cards with a role tag")
	fmt.Fprintln(&b, "  ComboWithSummaryPct  float64  — % of combos with a non-empty description")
	fmt.Fprintln(&b, "  Consistency          *ConsistencyResult  — Freya-vs-Freya bit-equal probe")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "CACHE ENTRY ENVELOPE (data/freya-cache/<key>-v<version>.json)")
	fmt.Fprintln(&b, "  freya_version  string       — must match FreyaVersion at load time")
	fmt.Fprintln(&b, "  deck_hash      string       — sha256 of normalized card list (matches filename)")
	fmt.Fprintln(&b, "  report         *FreyaReport — the cached report payload")
	return b.String()
}

// PrintUsage writes the multi-section Usage banner to w. Replaces
// flag.PrintDefaults's bare flag dump with a synopsis + examples +
// flags layout that matches what users see from `--help`.
//
// Pass the flag.CommandLine FlagSet (or the test's local FlagSet) so
// PrintDefaults reflects whichever flags are actually registered. The
// indirection lets the test inject a clean FlagSet without running
// the real main().
func PrintUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "hexdek-freya — Commander deck analysis & combo detection")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  hexdek-freya --deck <file> [options]")
	fmt.Fprintln(w, "  hexdek-freya --all-decks <dir> [options]")
	fmt.Fprintln(w, "  hexdek-freya --version")
	fmt.Fprintln(w, "  hexdek-freya --json-schema")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  # Render the default full report for one deck:")
	fmt.Fprintln(w, "  hexdek-freya --deck my_deck.txt")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  # JSON output piped into jq:")
	fmt.Fprintln(w, "  hexdek-freya --deck my_deck.txt --format json | jq .Archetype.Primary")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  # Under-25-line prioritized summary:")
	fmt.Fprintln(w, "  hexdek-freya --deck my_deck.txt --mode focus")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  # Output-quality scorecard + Freya-vs-Freya determinism probe:")
	fmt.Fprintln(w, "  hexdek-freya --deck my_deck.txt --mode metrics")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  # Corpus run with progress indicator:")
	fmt.Fprintln(w, "  hexdek-freya --all-decks data/decks/ --format markdown")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  # Bypass cache (forced fresh analysis):")
	fmt.Fprintln(w, "  hexdek-freya --deck my_deck.txt --no-cache")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "FLAGS:")
	// PrintDefaults writes to fs.Output (default os.Stderr); redirect by
	// flipping the FlagSet's output before the call.
	prev := fs.Output()
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(prev)
}

// ProgressLine formats a "[N/M] basename" prefix for use in
// --all-decks per-deck log lines. Pads the index to the width of M so
// the column stays aligned across a multi-hundred-deck run.
//
//	ProgressLine(  1, 100, "ragost_artifact_engine.txt")
//	  → "[  1/100] ragost_artifact_engine.txt"
//	ProgressLine( 47,  47, "azula.txt")
//	  → "[47/47] azula.txt"
func ProgressLine(idx, total int, basename string) string {
	width := 1
	for x := total; x >= 10; x /= 10 {
		width++
	}
	return fmt.Sprintf("[%*d/%d] %s", width, idx, total, basename)
}
