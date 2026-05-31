package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// hexdek-judge --interactive — a question/answer REPL over a loaded
// game-state snapshot or Loki replay.
//
// The existing REPL in main.go is a state-construction tool ("create
// game --seats 4", "seat 0 add_permanent …"). This interactive mode is
// a different surface: it loads a CONTEXT (either an SBASnapshot or a
// LokiReplay) at startup, then accepts free-form questions like:
//
//   why didn't Tergrid steal that
//   is this combat legal
//   what SBAs are pending
//   explain LifeConsistency
//   cite combat
//   show state
//   list rules
//
// The intent classifier is deliberately simple — substring + regex
// patterns on the input, dispatched to handlers that reuse the
// existing probe helpers (detectSBAViolations from sba_probe.go,
// invariantCRCitations from loki_replay.go). No NLP, no LLM call;
// everything maps to a deterministic rule lookup.
//
// Use case: a human player or judge has a snapshot of a confusing
// board state and wants to know "what does the engine say about
// this?" — they get pointed at the right CR section, the §704
// findings (if any), and a one-line explanation. The transcript
// is plain text so the answer can be copy-pasted into a forum
// thread / rules-question doc.
//
// Plumbed via injectable io.Reader / io.Writer so the test suite can
// feed scripted lines and assert on output without spinning a
// terminal.

// InteractiveContext is the loaded state the REPL answers questions
// against. Exactly one of Snapshot / Replay is populated; the
// interactive loop checks for nil before dispatching state-dependent
// intents.
type InteractiveContext struct {
	Snapshot     *SBASnapshot
	Replay       *LokiReplay
	SnapshotPath string
	ReplayPath   string
}

// loadInteractiveContext reads either a snapshot or a replay JSON
// from path. Both formats decode against the same loader because
// snapshots are a strict subset of replay violation payloads — the
// presence of "violations" indicates a replay, otherwise it's a
// raw snapshot.
func loadInteractiveContext(snapshotPath, replayPath string) (*InteractiveContext, error) {
	ctx := &InteractiveContext{
		SnapshotPath: snapshotPath,
		ReplayPath:   replayPath,
	}
	if snapshotPath != "" {
		f, err := os.Open(snapshotPath)
		if err != nil {
			return nil, fmt.Errorf("open snapshot: %w", err)
		}
		defer f.Close()
		var snap SBASnapshot
		if err := json.NewDecoder(f).Decode(&snap); err != nil {
			return nil, fmt.Errorf("decode snapshot: %w", err)
		}
		ctx.Snapshot = &snap
	}
	if replayPath != "" {
		f, err := os.Open(replayPath)
		if err != nil {
			return nil, fmt.Errorf("open replay: %w", err)
		}
		defer f.Close()
		var rep LokiReplay
		if err := json.NewDecoder(f).Decode(&rep); err != nil {
			return nil, fmt.Errorf("decode replay: %w", err)
		}
		ctx.Replay = &rep
	}
	if ctx.Snapshot == nil && ctx.Replay == nil {
		return nil, fmt.Errorf("--interactive requires --snapshot <path> or --replay <path>")
	}
	return ctx, nil
}

// runInteractive runs the Q&A loop against the loaded context. r is
// the input stream (os.Stdin in production; bytes.NewReader for
// tests); w is the output stream (os.Stdout / a *bytes.Buffer for
// tests). The loop exits cleanly on EOF or on "exit" / "quit".
func runInteractive(r io.Reader, w io.Writer, ctx *InteractiveContext) error {
	fmt.Fprintln(w, "hexdek-judge — Interactive Q&A mode")
	if ctx.Snapshot != nil {
		fmt.Fprintf(w, "  loaded snapshot: %s (%d seats, turn %d)\n",
			ctx.SnapshotPath, len(ctx.Snapshot.Seats), ctx.Snapshot.Turn)
	}
	if ctx.Replay != nil {
		fmt.Fprintf(w, "  loaded replay: %s (%d violations across %d games)\n",
			ctx.ReplayPath, len(ctx.Replay.Violations), ctx.Replay.Meta.TotalGames)
	}
	fmt.Fprintln(w, "  Ask a question, or type 'help' / 'exit'.")
	fmt.Fprintln(w)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for {
		fmt.Fprint(w, "judge> ")
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
		answerQuestion(line, ctx, w)
	}
	return scanner.Err()
}

// answerQuestion is the intent dispatcher. Pattern matching is
// substring-based + a few regexes; first match wins. Handlers write
// directly to w so each answer is a self-contained transcript block.
func answerQuestion(q string, ctx *InteractiveContext, w io.Writer) {
	lo := strings.ToLower(strings.TrimSpace(q))

	switch {
	case lo == "help" || lo == "?":
		printInteractiveHelp(w)

	// "list rules" / "list invariants" / "list seats"
	case lo == "list rules" || lo == "list citations":
		listAllCRCitations(w)
	case lo == "list invariants":
		listAllInvariants(w)
	case lo == "list seats" || lo == "show seats":
		listSeats(ctx, w)
	case lo == "list violations" || lo == "show violations":
		listReplayViolations(ctx, w)

	// "explain X" / "cite X"
	case strings.HasPrefix(lo, "explain "):
		topic := strings.TrimSpace(strings.TrimPrefix(lo, "explain "))
		explainTopic(topic, w)
	case strings.HasPrefix(lo, "cite "):
		topic := strings.TrimSpace(strings.TrimPrefix(lo, "cite "))
		citeTopic(topic, w)

	// "index <rule>" — direct lookup against the comprehensive citation index.
	case strings.HasPrefix(lo, "index "):
		rule := strings.TrimSpace(strings.TrimPrefix(lo, "index "))
		showIndexEntry(rule, w)

	// "coverage <probe>" — inverse lookup: which rules does the named probe check?
	case strings.HasPrefix(lo, "coverage "):
		probe := strings.TrimSpace(strings.TrimPrefix(lo, "coverage "))
		showProbeCoverage(probe, w)

	// "list probes" — enumerate the probes registered in the index.
	case lo == "list probes":
		listProbes(w)

	// "show state" / "show seat N"
	case lo == "show state" || lo == "state":
		showState(ctx, w)
	case showSeatRE.MatchString(lo):
		m := showSeatRE.FindStringSubmatch(lo)
		showSeat(ctx, m[1], w)

	// "what SBAs..." / "what's pending" / "check sba"
	case strings.Contains(lo, "what sba") || strings.Contains(lo, "sba") &&
		(strings.Contains(lo, "fire") || strings.Contains(lo, "pending") || strings.Contains(lo, "check")):
		runInteractiveSBACheck(ctx, w)

	// "is this combat legal" / "is combat legal" — point at CombatLegality
	case strings.Contains(lo, "combat") && (strings.Contains(lo, "legal") || strings.Contains(lo, "legality")):
		answerCombatLegality(w)

	// "why didn't X ..." / "why did X not ..."
	case strings.HasPrefix(lo, "why ") || strings.Contains(lo, "didn't") || strings.Contains(lo, "did not") ||
		strings.Contains(lo, "didnt"):
		answerWhy(q, ctx, w)

	default:
		fmt.Fprintf(w, "  I don't recognize that question. Try 'help' for the question patterns I know.\n\n")
	}
}

var showSeatRE = regexp.MustCompile(`^\s*(?:show|inspect)\s+seat\s+(\d+)\s*$`)

func printInteractiveHelp(w io.Writer) {
	fmt.Fprintln(w, "  Question patterns:")
	fmt.Fprintln(w, "    why did <X> not <action>     — point at the CR rule that governs <action>")
	fmt.Fprintln(w, "    is this combat legal         — explain the §506 combat-legality rule chain")
	fmt.Fprintln(w, "    what SBAs are pending        — run the §704 SBA probe against the loaded snapshot")
	fmt.Fprintln(w, "    explain <invariant>          — show CR citations for an engine invariant")
	fmt.Fprintln(w, "    cite <topic>                 — fuzzy-search CR citations for a topic word")
	fmt.Fprintln(w, "    index <rule>                 — show full citation-index entry for a rule slug")
	fmt.Fprintln(w, "    coverage <probe>             — list rules the named probe checks")
	fmt.Fprintln(w, "    show state / show seat N     — inspect the loaded snapshot")
	fmt.Fprintln(w, "    list invariants / list rules — full citation catalog")
	fmt.Fprintln(w, "    list probes                  — probes registered in the citation index")
	fmt.Fprintln(w, "    list violations              — Loki replay violation summary")
	fmt.Fprintln(w, "    exit                         — leave Q&A mode")
	fmt.Fprintln(w)
}

func listAllCRCitations(w io.Writer) {
	seen := map[string]string{} // rule → description
	for _, cites := range invariantCRCitations {
		for _, c := range cites {
			if _, ok := seen[c.Rule]; !ok {
				seen[c.Rule] = c.Description
			}
		}
	}
	rules := make([]string, 0, len(seen))
	for r := range seen {
		rules = append(rules, r)
	}
	sort.Strings(rules)
	fmt.Fprintf(w, "  %d CR sub-sections in the citation map:\n", len(rules))
	for _, r := range rules {
		fmt.Fprintf(w, "    §%s — %s\n", r, seen[r])
	}
	fmt.Fprintln(w)
}

func listAllInvariants(w io.Writer) {
	names := make([]string, 0, len(invariantCRCitations))
	for n := range invariantCRCitations {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(w, "  %d engine invariants in the citation map:\n", len(names))
	for _, n := range names {
		cites := invariantCRCitations[n]
		ruleSlugs := make([]string, 0, len(cites))
		for _, c := range cites {
			ruleSlugs = append(ruleSlugs, "§"+c.Rule)
		}
		fmt.Fprintf(w, "    %-25s → %s\n", n, strings.Join(ruleSlugs, ", "))
	}
	fmt.Fprintln(w)
}

func listSeats(ctx *InteractiveContext, w io.Writer) {
	if ctx.Snapshot == nil {
		fmt.Fprintf(w, "  no snapshot loaded — 'list seats' needs --snapshot or a replay with embedded snapshots\n\n")
		return
	}
	for _, s := range ctx.Snapshot.Seats {
		fmt.Fprintf(w, "  seat %d: life=%d poison=%d battlefield=%d permanents%s\n",
			s.Idx, s.Life, s.PoisonCounters, len(s.Battlefield),
			lostFlag(s))
	}
	fmt.Fprintln(w)
}

func lostFlag(s SBASeat) string {
	flags := []string{}
	if s.Lost {
		flags = append(flags, "Lost")
	}
	if s.LeftGame {
		flags = append(flags, "LeftGame")
	}
	if len(flags) == 0 {
		return ""
	}
	return " [" + strings.Join(flags, ",") + "]"
}

func listReplayViolations(ctx *InteractiveContext, w io.Writer) {
	if ctx.Replay == nil {
		fmt.Fprintf(w, "  no replay loaded — 'list violations' needs --replay\n\n")
		return
	}
	counts := map[string]int{}
	for _, v := range ctx.Replay.Violations {
		counts[v.InvariantName]++
	}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "  %d violations across %d invariants:\n", len(ctx.Replay.Violations), len(names))
	for _, n := range names {
		fmt.Fprintf(w, "    %-25s × %d\n", n, counts[n])
	}
	fmt.Fprintln(w)
}

func explainTopic(topic string, w io.Writer) {
	if topic == "" {
		fmt.Fprintf(w, "  usage: explain <invariant-name>\n\n")
		return
	}
	// Case-insensitive lookup.
	var cites []CRCitation
	matched := ""
	for name, c := range invariantCRCitations {
		if strings.EqualFold(name, topic) {
			cites = c
			matched = name
			break
		}
	}
	if cites == nil {
		fmt.Fprintf(w, "  no invariant named %q. Try 'list invariants'.\n\n", topic)
		return
	}
	fmt.Fprintf(w, "  %s — %s\n", matched, explainInvariant(matched, 0))
	for _, c := range cites {
		fmt.Fprintf(w, "    §%s — %s\n", c.Rule, c.Description)
	}
	fmt.Fprintln(w)
}

func citeTopic(topic string, w io.Writer) {
	if topic == "" {
		fmt.Fprintf(w, "  usage: cite <topic-word>  (e.g. cite combat, cite legend, cite life)\n\n")
		return
	}
	low := strings.ToLower(topic)
	type hit struct {
		Rule, Description, Invariant string
	}
	var hits []hit
	seenRule := map[string]bool{}
	for name, cites := range invariantCRCitations {
		for _, c := range cites {
			if strings.Contains(strings.ToLower(c.Description), low) ||
				strings.Contains(strings.ToLower(name), low) ||
				strings.Contains(c.Rule, low) {
				key := c.Rule
				if seenRule[key] {
					continue
				}
				seenRule[key] = true
				hits = append(hits, hit{Rule: c.Rule, Description: c.Description, Invariant: name})
			}
		}
	}
	if len(hits) == 0 {
		fmt.Fprintf(w, "  no citations match %q. Try 'list rules' for the full set.\n\n", topic)
		return
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Rule < hits[j].Rule })
	fmt.Fprintf(w, "  %d citation(s) match %q:\n", len(hits), topic)
	for _, h := range hits {
		fmt.Fprintf(w, "    §%s — %s  (via %s)\n", h.Rule, h.Description, h.Invariant)
	}
	fmt.Fprintln(w)
}

func showState(ctx *InteractiveContext, w io.Writer) {
	if ctx.Snapshot == nil {
		fmt.Fprintf(w, "  no snapshot loaded\n\n")
		return
	}
	fmt.Fprintf(w, "  Snapshot summary: format=%q turn=%d seats=%d\n",
		ctx.Snapshot.Format, ctx.Snapshot.Turn, len(ctx.Snapshot.Seats))
	for _, s := range ctx.Snapshot.Seats {
		fmt.Fprintf(w, "    seat %d: life=%d poison=%d bf=%d cmdr=%v\n",
			s.Idx, s.Life, s.PoisonCounters, len(s.Battlefield), s.CommandZone)
	}
	fmt.Fprintln(w)
}

func showSeat(ctx *InteractiveContext, idxStr string, w io.Writer) {
	if ctx.Snapshot == nil {
		fmt.Fprintf(w, "  no snapshot loaded\n\n")
		return
	}
	idx := 0
	fmt.Sscanf(idxStr, "%d", &idx)
	var found *SBASeat
	for i := range ctx.Snapshot.Seats {
		if ctx.Snapshot.Seats[i].Idx == idx {
			found = &ctx.Snapshot.Seats[i]
			break
		}
	}
	if found == nil {
		fmt.Fprintf(w, "  seat %d not in snapshot\n\n", idx)
		return
	}
	fmt.Fprintf(w, "  seat %d: life=%d poison=%d lost=%v left_game=%v\n",
		found.Idx, found.Life, found.PoisonCounters, found.Lost, found.LeftGame)
	for _, p := range found.Battlefield {
		marks := []string{}
		if p.Tapped {
			marks = append(marks, "T")
		}
		if p.PhasedOut {
			marks = append(marks, "phased-out")
		}
		mark := ""
		if len(marks) > 0 {
			mark = " [" + strings.Join(marks, ",") + "]"
		}
		fmt.Fprintf(w, "    %-30s %s P/T=%d/%d damage=%d counters=%v%s\n",
			p.Name, strings.Join(p.Types, "+"),
			p.BasePower, p.BaseToughness, p.MarkedDamage, p.Counters, mark)
	}
	fmt.Fprintln(w)
}

func runInteractiveSBACheck(ctx *InteractiveContext, w io.Writer) {
	if ctx.Snapshot == nil {
		fmt.Fprintf(w, "  no snapshot loaded — 'what SBAs' needs --snapshot\n\n")
		return
	}
	vs := detectSBAViolations(ctx.Snapshot)
	if len(vs) == 0 {
		fmt.Fprintf(w, "  No §704.5 / §704.6 conditions detected. State is post-SBA clean.\n\n")
		return
	}
	fmt.Fprintf(w, "  %d §704 condition(s) pending:\n", len(vs))
	for _, v := range vs {
		if v.PermanentName != "" {
			fmt.Fprintf(w, "    §%s — %s — seat %d / %s — %s\n",
				v.Rule, v.RuleName, v.Seat, v.PermanentName, v.Detail)
		} else {
			fmt.Fprintf(w, "    §%s — %s — seat %d — %s\n",
				v.Rule, v.RuleName, v.Seat, v.Detail)
		}
	}
	fmt.Fprintln(w)
}

func answerCombatLegality(w io.Writer) {
	fmt.Fprintln(w, "  Combat legality is governed by CR §506 (combat phase structure):")
	fmt.Fprintln(w, "    §506.2 — phase steps + declarations")
	fmt.Fprintln(w, "    §508   — declare attackers step (untapped, attacking restrictions, summoning sickness)")
	fmt.Fprintln(w, "    §509   — declare blockers step (block legality, requirements/restrictions)")
	fmt.Fprintln(w, "  Engine invariant: CombatLegality. Run 'explain CombatLegality' for the full citation.")
	fmt.Fprintln(w)
}

func answerWhy(q string, ctx *InteractiveContext, w io.Writer) {
	// "why didn't X steal that" — match the action verb against
	// invariant descriptions. This is the broadest intent, so it does
	// the most work: search the CR map for keywords from the question
	// body, list candidate rules, suggest running the §704 probe if a
	// snapshot is loaded.
	low := strings.ToLower(q)
	// Drop the "why" framing words to leave the verb-y body.
	for _, prefix := range []string{
		"why didn't ", "why did not ", "why didnt ", "why ", "why is ",
	} {
		if strings.HasPrefix(low, prefix) {
			low = strings.TrimPrefix(low, prefix)
			break
		}
	}
	low = strings.TrimSuffix(low, "?")
	low = strings.TrimSpace(low)
	// Extract verb candidates — anything after "not" / "didn't" tail.
	for _, sep := range []string{"not ", "didn't ", "did not "} {
		if i := strings.Index(low, sep); i >= 0 {
			low = strings.TrimSpace(low[i+len(sep):])
		}
	}
	// Match remaining text against descriptions in the citation map.
	type hit struct {
		Rule, Description, Invariant string
	}
	var hits []hit
	seen := map[string]bool{}
	tokens := strings.Fields(low)
	if len(tokens) == 0 {
		fmt.Fprintf(w, "  I couldn't extract an action from that question. Try 'cite <topic>' or 'list rules'.\n\n")
		return
	}
	for name, cites := range invariantCRCitations {
		for _, c := range cites {
			desc := strings.ToLower(c.Description)
			matched := false
			for _, tk := range tokens {
				if len(tk) < 3 {
					continue
				}
				if strings.Contains(desc, tk) || strings.Contains(strings.ToLower(name), tk) {
					matched = true
					break
				}
			}
			if matched && !seen[c.Rule] {
				seen[c.Rule] = true
				hits = append(hits, hit{Rule: c.Rule, Description: c.Description, Invariant: name})
			}
		}
	}
	if len(hits) == 0 {
		fmt.Fprintf(w, "  No direct CR citation matched the action words. Try 'cite <topic>' with a single keyword.\n\n")
		return
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Rule < hits[j].Rule })
	fmt.Fprintf(w, "  Likely-relevant CR sub-sections for that question:\n")
	for _, h := range hits {
		fmt.Fprintf(w, "    §%s — %s  (via %s)\n", h.Rule, h.Description, h.Invariant)
	}
	if ctx.Snapshot != nil {
		fmt.Fprintf(w, "  Snapshot is loaded; run 'what SBAs are pending' to see if §704 conditions apply.\n")
	}
	fmt.Fprintln(w)
}

// showIndexEntry prints the full citation-index entry for a rule slug.
// "index 704.5a" surfaces description, section title, probes that
// check it, related invariants, and related CR sub-sections — the
// most-complete single-line answer the judge can give about a rule.
func showIndexEntry(rule string, w io.Writer) {
	if rule == "" {
		fmt.Fprintf(w, "  usage: index <rule-slug>  (e.g. index 704.5a)\n\n")
		return
	}
	idx := BuildCitationIndex()
	e := idx.LookupByRule(rule)
	if e == nil {
		fmt.Fprintf(w, "  no index entry for §%s. Try 'list rules' or 'cite <topic>'.\n\n", rule)
		return
	}
	fmt.Fprintf(w, "  §%s — %s\n", e.Rule, e.Description)
	fmt.Fprintf(w, "    section:            %s\n", e.SectionTitle)
	if len(e.CheckedBy) > 0 {
		fmt.Fprintf(w, "    checked by:         %s\n", strings.Join(e.CheckedBy, ", "))
	}
	if len(e.RelatedInvariants) > 0 {
		fmt.Fprintf(w, "    related invariants: %s\n", strings.Join(e.RelatedInvariants, ", "))
	}
	if len(e.RelatedRules) > 0 {
		slugs := make([]string, 0, len(e.RelatedRules))
		for _, r := range e.RelatedRules {
			slugs = append(slugs, "§"+r)
		}
		fmt.Fprintf(w, "    related rules:      %s\n", strings.Join(slugs, ", "))
	}
	if len(e.HistoricalFixes) > 0 {
		// Cap at 5 so a heavily-cited rule like §704.5a doesn't flood
		// the transcript; user can run --citation-index for the full
		// JSON dump.
		shown := e.HistoricalFixes
		if len(shown) > 5 {
			shown = shown[:5]
		}
		fmt.Fprintf(w, "    historical fixes (%d):\n", len(e.HistoricalFixes))
		for _, fx := range shown {
			summary := fx.IssueSummary
			if len(summary) > 90 {
				summary = summary[:90] + "…"
			}
			fmt.Fprintf(w, "      %s [%s] %s\n", fx.Date, fx.Source, summary)
		}
		if len(e.HistoricalFixes) > len(shown) {
			fmt.Fprintf(w, "      … and %d more (run --citation-index for the full list)\n",
				len(e.HistoricalFixes)-len(shown))
		}
	}
	fmt.Fprintln(w)
}

// showProbeCoverage prints the rules a probe checks. Inverse of
// showIndexEntry — "coverage sba_probe" answers "which CR sub-sections
// does the SBA probe implement?" in one line.
func showProbeCoverage(probe string, w io.Writer) {
	if probe == "" {
		fmt.Fprintf(w, "  usage: coverage <probe-name>  (e.g. coverage sba_probe, coverage commander_check)\n")
		fmt.Fprintf(w, "         try 'list probes' for the full list\n\n")
		return
	}
	idx := BuildCitationIndex()
	rules := idx.LookupByProbe(probe)
	if len(rules) == 0 {
		fmt.Fprintf(w, "  no probe named %q in the citation index. Try 'list probes'.\n\n", probe)
		return
	}
	fmt.Fprintf(w, "  %s checks %d CR sub-section(s):\n", probe, len(rules))
	for _, r := range rules {
		e := idx.LookupByRule(r)
		if e != nil {
			fmt.Fprintf(w, "    §%-8s — %s\n", r, e.Description)
		} else {
			fmt.Fprintf(w, "    §%s\n", r)
		}
	}
	fmt.Fprintln(w)
}

// listProbes prints every probe registered in the citation index,
// with the count of rules it covers. Use case: a maintainer wants to
// see the inverse coverage map at a glance.
func listProbes(w io.Writer) {
	idx := BuildCitationIndex()
	names := make([]string, 0, len(idx.ProbeToRules))
	for n := range idx.ProbeToRules {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(w, "  %d probe(s) registered in the citation index:\n", len(names))
	for _, n := range names {
		fmt.Fprintf(w, "    %-30s → %d CR sub-section(s)\n", n, len(idx.ProbeToRules[n]))
	}
	fmt.Fprintln(w)
}
