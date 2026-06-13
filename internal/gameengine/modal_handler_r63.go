package gameengine

// modal_handler_r63.go — GENERIC modal-spell handler (Charm / Command /
// Confluence families + entwine).
//
// The parser frequently fails to build a structured gameast.Choice node for
// modal spells; the whole "choose one — • A ; • B ; • C" body is dumped into
// a parsed_tail scaffold and routed (as raw text) through
// resolveResidualByText, where it previously matched nothing and the spell
// did NOTHING. ~1,800 parsed_tail nodes exist and the modal families are a
// large fully-inert chunk (Boros Charm, the Commands, the Confluences, …).
//
// This handler detects the modal structure from the raw text, chooses the
// legal mode(s) per the "choose N" header (or entwine = all), and executes
// each chosen mode by recognizing the dominant Charm/Command verbs
// (damage / destroy / draw / counter / gain-life / pump / bounce / tap /
// untap), falling back to resolveResidualByText for the residual vocabulary.
//
// It is hooked at the top of resolveResidualByText (one minimal branch) so
// every modal spell routed through the residual dispatcher is covered.
//
// Mode-selection policy (MVP, strictly additive over the prior do-nothing):
//   - choose one        → execute the FIRST mode (printed order) that does
//                         something legal.
//   - choose two/N      → execute up to N modes that do something, in order.
//   - choose one or more → greedily execute every executable mode.
//   - entwine present   → execute ALL modes (the engine's documented greedy
//                         entwine policy, ShouldEntwine).
// Modes the executor doesn't recognize are skipped (logged), so the spell is
// never worse than before and frequently does its dominant mode.

import (
	"regexp"
	"strings"
)

var (
	// "choose one —", "choose two —", "choose one or both —", "choose one
	// or more —", "choose up to two —". The em dash (— U+2014) or a hyphen
	// follows the header. We tolerate either.
	reModalHeader = regexp.MustCompile(`(?i)\bchoose\s+(up to\s+)?(one|two|three)(\s+or\s+(both|more))?\s*[—\-]`)
	reModeDamage  = regexp.MustCompile(`(?i)deals?\s+(\d+)\s+damage\s+to\s+(.+)`)
	reModeDraw    = regexp.MustCompile(`(?i)\b(?:you\s+)?draws?\s+(a|one|two|three|four|five|\d+)\s+cards?`)
	reModeGain    = regexp.MustCompile(`(?i)\byou\s+gain\s+(\d+)\s+life`)
	reModeDestroy = regexp.MustCompile(`(?i)\bdestroy\s+target\s+([a-z ]+?)(?:\s+(?:you|an opponent|that)\b|\.|$)`)
	reModePump    = regexp.MustCompile(`(?i)target\s+creature\s+(?:you control\s+)?gets\s+([+-]\d+)/([+-]\d+)`)
	reModeCounter = regexp.MustCompile(`(?i)\bcounter\s+target\s+(?:[a-z ]+\s+)?spell`)
)

// ResolveModalText detects and resolves a modal spell from raw residual
// text. Returns true iff the text was recognized as modal (whether or not
// any individual mode executed) — so the caller treats it as handled.
func ResolveModalText(gs *GameState, src *Permanent, raw string) bool {
	if gs == nil {
		return false
	}
	loc := reModalHeader.FindStringSubmatchIndex(raw)
	if loc == nil {
		return false
	}
	m := reModalHeader.FindStringSubmatch(raw)
	// m[1]="up to " or "", m[2]=one|two|three, m[4]=both|more|""
	pick := wordToCount(m[2])
	orMore := strings.EqualFold(m[4], "more")
	orBoth := strings.EqualFold(m[4], "both")
	upTo := strings.TrimSpace(m[1]) != ""
	entwine := strings.Contains(strings.ToLower(raw), "entwine")

	// Modes are the bulleted clauses after the header.
	body := raw[loc[1]:]
	modes := splitModes(body)
	if len(modes) == 0 {
		return false
	}

	// How many modes may we execute?
	maxPick := pick
	if orBoth {
		maxPick = 2
	}
	if orMore || entwine {
		maxPick = len(modes) // greedy: all
	}
	if maxPick > len(modes) {
		maxPick = len(modes)
	}
	// "up to N" allows fewer; we still greedily take executable modes up to N.
	_ = upTo

	gs.LogEvent(Event{
		Kind:   "modal_spell",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Details: map[string]interface{}{
			"modes":   len(modes),
			"pick":    maxPick,
			"entwine": entwine,
		},
	})

	executed := 0
	for _, mode := range modes {
		if executed >= maxPick {
			break
		}
		if executeModeText(gs, src, mode) {
			executed++
		}
	}
	return true
}

// splitModes extracts the bulleted mode clauses from a modal body. Modes
// are separated by the "•" bullet (U+2022) and/or ";". Empty fragments are
// dropped.
func splitModes(body string) []string {
	// Normalize separators: ";" between bullets, "•" as the bullet marker.
	repl := strings.NewReplacer("•", "\x00", "·", "\x00", ";", "\x00")
	parts := strings.Split(repl.Replace(body), "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		s = strings.TrimRight(s, ". ")
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// executeModeText runs a single mode's text. Returns true if it produced an
// effect. Recognizes the dominant Charm/Command verbs directly (via engine
// primitives + direct target selection), then falls back to the residual
// dispatcher for the rest of the vocabulary.
func executeModeText(gs *GameState, src *Permanent, mode string) bool {
	seat := controllerSeat(src)
	low := strings.ToLower(mode)

	// Counter target spell — counter the topmost opposing spell on the stack.
	if reModeCounter.MatchString(low) {
		if top := topCounterableSpell(gs, seat); top != nil {
			top.Countered = true
			gs.LogEvent(Event{Kind: "counter", Seat: seat, Source: sourceName(src),
				Details: map[string]interface{}{"via": "modal"}})
			return true
		}
		return false
	}

	// Draw N cards.
	if mm := reModeDraw.FindStringSubmatch(low); mm != nil {
		n := wordToCount(mm[1])
		if n > 0 && seat >= 0 {
			DrawN(gs, seat, n, src)
			return true
		}
	}

	// You gain N life.
	if mm := reModeGain.FindStringSubmatch(low); mm != nil {
		n, _ := atoiSafe(mm[1])
		if n > 0 && seat >= 0 {
			GainLife(gs, seat, n, sourceName(src))
			return true
		}
	}

	// Deals N damage to <target>.
	if mm := reModeDamage.FindStringSubmatch(low); mm != nil {
		n, _ := atoiSafe(mm[1])
		tgt := strings.TrimSpace(mm[2])
		if n > 0 {
			return applyModalDamage(gs, src, n, tgt)
		}
	}

	// Destroy target <type>.
	if mm := reModeDestroy.FindStringSubmatch(low); mm != nil {
		typ := strings.TrimSpace(mm[1])
		if p := pickHarmfulPermanentOfType(gs, seat, typ); p != nil {
			DestroyPermanent(gs, p, src)
			return true
		}
		return false
	}

	// Target creature gets +X/+X (until end of turn) — pump.
	if mm := reModePump.FindStringSubmatch(low); mm != nil {
		pw, _ := atoiSafe(mm[1])
		tf, _ := atoiSafe(mm[2])
		var p *Permanent
		if pw < 0 || tf < 0 {
			p = pickHarmfulCreature(gs, seat) // debuff → opponent creature
		} else {
			p = pickOwnCreature(gs, seat) // buff → own creature
		}
		if p != nil {
			p.Modifications = append(p.Modifications, Modification{
				Power: pw, Toughness: tf, Duration: "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			if tf < 0 {
				StateBasedActions(gs)
			}
			return true
		}
		return false
	}

	// Residual vocabulary (bounce / tap / untap / goad / regenerate / …).
	return resolveResidualByText(gs, src, mode)
}

// applyModalDamage routes "deals N damage to <target>" by reading the target
// phrase. Player / any-target / planeswalker → the lowest-life opponent's
// face; creature → the strongest opponent creature.
func applyModalDamage(gs *GameState, src *Permanent, n int, tgt string) bool {
	if strings.Contains(tgt, "creature") && !strings.Contains(tgt, "player") {
		if p := pickHarmfulCreature(gs, controllerSeat(src)); p != nil {
			applyDamage(gs, src, Target{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller}, n)
			return true
		}
		return false
	}
	// player / "any target" / "player or planeswalker" / "each opponent"
	if strings.Contains(tgt, "each opponent") {
		hit := false
		for _, opp := range gs.Opponents(controllerSeat(src)) {
			applyDamage(gs, src, Target{Kind: TargetKindSeat, Seat: opp}, n)
			hit = true
		}
		return hit
	}
	if opp := lowestLifeOpponent(gs, controllerSeat(src)); opp >= 0 {
		applyDamage(gs, src, Target{Kind: TargetKindSeat, Seat: opp}, n)
		return true
	}
	return false
}

// --- target-selection + small helpers --------------------------------------

func lowestLifeOpponent(gs *GameState, seat int) int {
	best, bestLife := -1, 1<<30
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			best = opp
		}
	}
	return best
}

func pickHarmfulCreature(gs *GameState, seat int) *Permanent {
	var best *Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut || !gs.IsCreatureOf(p) {
				continue
			}
			if pw := gs.PowerOf(p); pw > bestPow {
				bestPow, best = pw, p
			}
		}
	}
	return best
}

func pickOwnCreature(gs *GameState, seat int) *Permanent {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	var best *Permanent
	bestPow := -1
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.PhasedOut || !gs.IsCreatureOf(p) {
			continue
		}
		if pw := gs.PowerOf(p); pw > bestPow {
			bestPow, best = pw, p
		}
	}
	return best
}

// pickHarmfulPermanentOfType picks an opponent's permanent matching the
// destroy target type ("artifact"/"enchantment"/"creature"/"land"/
// "permanent"/"nonland permanent"). Prefers the highest-power creature; for
// non-creature types takes the first match.
func pickHarmfulPermanentOfType(gs *GameState, seat int, typ string) *Permanent {
	typ = strings.TrimSpace(typ)
	wantAny := typ == "permanent" || typ == "nonland permanent" || typ == ""
	var best *Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}
			match := wantAny
			if !match {
				match = gs.HasTypeOf(p, typ)
			}
			if typ == "nonland permanent" && gs.HasTypeOf(p, "land") {
				match = false
			}
			if !match {
				continue
			}
			pw := gs.PowerOf(p)
			if pw > bestPow {
				bestPow, best = pw, p
			}
		}
	}
	return best
}

// topCounterableSpell returns the topmost spell on the stack NOT controlled
// by `seat` (you don't counter your own modal spell).
func topCounterableSpell(gs *GameState, seat int) *StackItem {
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		it := gs.Stack[i]
		if it == nil || it.Card == nil || it.Countered {
			continue
		}
		if it.Controller == seat {
			continue
		}
		return it
	}
	return nil
}

func wordToCount(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "a", "an", "one":
		return 1
	case "two":
		return 2
	case "three":
		return 3
	case "four":
		return 4
	case "five":
		return 5
	}
	n, _ := atoiSafe(s) // existing helper (chaos_oracle.go): signed, (int,bool)
	return n
}
