package gameengine

// CR §727 — Loop shortcut detection and projection.
//
// When the stack drain loop detects a repeating sequence of resolutions
// (same source cards in the same order), it projects the per-cycle state
// delta forward and applies the result in one shot. This mirrors how
// paper Magic handles mandatory loops: demonstrate the loop, name an
// iteration count, advance the game state.

const (
	loopMinReps    = 3  // require 3 full repetitions to confirm a loop
	loopMaxPeriod  = 6  // check periods 1-6 (covers all real MTG loops)
	loopMaxProject = 500 // max cycles to project forward
	loopPermCap    = 200 // per-seat permanent cap for projection
)

type loopFingerprint uint64

type loopSnapshot struct {
	life  []int
	perms []int
}

type loopDetector struct {
	prints    []loopFingerprint
	snapshots []loopSnapshot
}

func newLoopDetector() *loopDetector {
	return &loopDetector{
		prints:    make([]loopFingerprint, 0, 64),
		snapshots: make([]loopSnapshot, 0, 64),
	}
}

func (ld *loopDetector) record(gs *GameState, fp loopFingerprint) {
	ld.prints = append(ld.prints, fp)
	ld.snapshots = append(ld.snapshots, captureSnapshot(gs))
}

func captureSnapshot(gs *GameState) loopSnapshot {
	snap := loopSnapshot{
		life:  make([]int, len(gs.Seats)),
		perms: make([]int, len(gs.Seats)),
	}
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		snap.life[i] = s.Life
		snap.perms[i] = len(s.Battlefield)
	}
	return snap
}

func stackTopFingerprint(gs *GameState) loopFingerprint {
	if len(gs.Stack) == 0 {
		return 0
	}
	item := gs.Stack[len(gs.Stack)-1]
	var h loopFingerprint
	if item.Card != nil {
		h = fnvStr(item.Card.DisplayName())
	} else if item.Source != nil && item.Source.Card != nil {
		h = fnvStr(item.Source.Card.DisplayName())
	}
	h = h*31 + loopFingerprint(item.Controller)
	if item.Kind != "" {
		h = h*31 + fnvStr(item.Kind)
	}
	return h
}

func fnvStr(s string) loopFingerprint {
	var h loopFingerprint = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= loopFingerprint(s[i])
		h *= 1099511628211
	}
	return h
}

// detectLoop checks if the recorded fingerprints contain a repeating
// cycle of length 1..loopMaxPeriod with at least loopMinReps repetitions.
// Returns the period length (0 = no loop detected).
func (ld *loopDetector) detectLoop() int {
	n := len(ld.prints)
	for period := 1; period <= loopMaxPeriod; period++ {
		needed := period * loopMinReps
		if n < needed {
			continue
		}
		tail := ld.prints[n-needed:]
		match := true
		for i := period; i < needed; i++ {
			if tail[i] != tail[i%period] {
				match = false
				break
			}
		}
		if match {
			return period
		}
	}
	return 0
}

// projectAndApply detects a loop, computes the per-cycle delta, and
// projects it forward. Returns true if a shortcut was applied.
func (ld *loopDetector) projectAndApply(gs *GameState) bool {
	period := ld.detectLoop()
	if period == 0 {
		return false
	}

	n := len(ld.snapshots)
	if n < period*2 {
		return false
	}

	cycleEnd := ld.snapshots[n-1]
	cycleStart := ld.snapshots[n-1-period]

	nSeats := len(gs.Seats)
	deltaLife := make([]int, nSeats)
	deltaPerms := make([]int, nSeats)
	for i := 0; i < nSeats; i++ {
		if i < len(cycleEnd.life) && i < len(cycleStart.life) {
			deltaLife[i] = cycleEnd.life[i] - cycleStart.life[i]
		}
		if i < len(cycleEnd.perms) && i < len(cycleStart.perms) {
			deltaPerms[i] = cycleEnd.perms[i] - cycleStart.perms[i]
		}
	}

	// Verify delta is constant across the last two periods.
	if n >= period*3 {
		prevStart := ld.snapshots[n-1-2*period]
		prevEnd := ld.snapshots[n-1-period]
		for i := 0; i < nSeats; i++ {
			if i < len(prevEnd.life) && i < len(prevStart.life) {
				d := prevEnd.life[i] - prevStart.life[i]
				if d != deltaLife[i] {
					return false
				}
			}
			if i < len(prevEnd.perms) && i < len(prevStart.perms) {
				d := prevEnd.perms[i] - prevStart.perms[i]
				if d != deltaPerms[i] {
					return false
				}
			}
		}
	}

	// No-op loops (everything stays the same) — just break out.
	allZero := true
	for i := 0; i < nSeats; i++ {
		if deltaLife[i] != 0 || deltaPerms[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		evacuateStackSpellsToGraveyard(gs)
		gs.Stack = gs.Stack[:0]
		gs.LogEvent(Event{
			Kind:   "loop_shortcut",
			Source: "no_op_loop",
			Details: map[string]interface{}{
				"period": period,
				"action": "break_no_op",
				"rule":   "727",
			},
		})
		return true
	}

	// Calculate how many cycles until a termination condition.
	maxCycles := loopMaxProject
	for i, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame || i >= nSeats {
			continue
		}
		// Life drain termination: someone would die.
		if deltaLife[i] < 0 {
			cyclesUntilDead := s.Life / (-deltaLife[i])
			if cyclesUntilDead < maxCycles {
				maxCycles = cyclesUntilDead
			}
		}
		// Permanent cap: don't exceed loopPermCap per seat.
		if deltaPerms[i] > 0 {
			room := loopPermCap - len(s.Battlefield)
			if room <= 0 {
				maxCycles = 0
			} else {
				cyclesUntilCap := room / deltaPerms[i]
				if cyclesUntilCap < maxCycles {
					maxCycles = cyclesUntilCap
				}
			}
		}
	}
	if maxCycles <= 0 {
		evacuateStackSpellsToGraveyard(gs)
		gs.Stack = gs.Stack[:0]
		return true
	}

	// Apply the projected delta.
	for i, s := range gs.Seats {
		if s == nil || i >= nSeats {
			continue
		}
		s.Life += deltaLife[i] * maxCycles

		if deltaPerms[i] > 0 {
			tokensToCreate := deltaPerms[i] * maxCycles
			for t := 0; t < tokensToCreate; t++ {
				// r60 InstanceID source audit: this path collapses N
				// loop cycles to a single state mutation per CR §727
				// so doesn't dispatch through FirePermanentETBTriggers
				// (which would fire observer triggers we're explicitly
				// skipping). That bypasses the defensive
				// EnsureTokenInstanceID stamp — and the original Card
				// also lacked Types:["token"] so even a manual ensure
				// call would have no-op'd. Mint explicitly here so the
				// projected tokens enter the InstanceID census cleanly.
				tokenCard := &Card{
					Name:     "Loop Token",
					Owner:    i,
					Types:    []string{"token", "creature"},
					TypeLine: "Token Creature",
				}
				MintTokenInstanceID(gs, tokenCard, "", currentMintEnablerID(gs))
				token := &Permanent{
					Card:       tokenCard,
					Controller: i,
					Owner:      i,
				}
				s.Battlefield = append(s.Battlefield, token)
			}
		} else if deltaPerms[i] < 0 {
			toRemove := (-deltaPerms[i]) * maxCycles
			if toRemove > len(s.Battlefield) {
				toRemove = len(s.Battlefield)
			}
			s.Battlefield = s.Battlefield[:len(s.Battlefield)-toRemove]
		}
	}

	evacuateStackSpellsToGraveyard(gs)
	gs.Stack = gs.Stack[:0]

	gs.LogEvent(Event{
		Kind:   "loop_shortcut",
		Source: "cr_727",
		Details: map[string]interface{}{
			"period":     period,
			"cycles":     maxCycles,
			"delta_life": deltaLife,
			"delta_perm": deltaPerms,
			"rule":       "727",
		},
	})

	return true
}

// evacuateStackSpellsToGraveyard sweeps unresolved spell items off the
// stack and routes the underlying *Card to its owner's graveyard. Copies
// (CR §707.10) and ability items (triggered/activated, which have a
// Source permanent and no spell-card identity) are dropped without zone
// placement — those aren't real cards.
//
// Called by projectAndApply before clearing gs.Stack on either the no-op
// loop break or the maxCycles==0 cap path. Without this, when the stack
// holds a real spell that couldn't resolve (e.g. ResolveStackTop
// short-circuits because gs.Flags["ended"]==1 from an SBA infinite-loop
// cap), nuking gs.Stack vanishes the card and breaks zone conservation
// — see Loki r44 game 420 / Breya pod / Dragonborn Looter +
// A Tale for the Ages disappearing post-Unholy-Indenture reanimate loop.
func evacuateStackSpellsToGraveyard(gs *GameState) {
	if gs == nil {
		return
	}
	for _, item := range gs.Stack {
		if item == nil {
			continue
		}
		if item.IsCopy {
			// §707.10 — a copy of a spell ceases to exist when it leaves
			// the stack without resolving. Cease its InstanceID so the
			// Phase 4 census doesn't flag it as "minted-but-disappeared"
			// after projectAndApply truncates gs.Stack.
			if item.Card != nil && item.Card.InstanceID != "" {
				MarkInstanceIDCeased(gs, item.Card.InstanceID)
			}
			continue
		}
		if item.Source != nil || item.Card == nil {
			continue
		}
		owner := item.Card.Owner
		if owner < 0 || owner >= len(gs.Seats) || gs.Seats[owner] == nil {
			continue
		}
		gs.Seats[owner].Graveyard = append(gs.Seats[owner].Graveyard, item.Card)
		gs.LogEvent(Event{
			Kind:   "stack_evacuated",
			Seat:   owner,
			Source: item.Card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "loop_shortcut_break",
				"to":     "graveyard",
				"rule":   "727",
			},
		})
	}
}
