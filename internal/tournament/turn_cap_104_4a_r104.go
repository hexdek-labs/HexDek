package tournament

import "github.com/hexdek/hexdek/internal/gameengine"

// turn_cap_104_4a_r104.go — the turn-cap winner rule, in one place.
//
// THE DEFECT. The turn cap is not a Magic rule; it is a harness
// construct that stops a game which has run too long and asks "who was
// ahead?". Every copy of that question answered it as "the living seat
// with the highest life", where "living" meant `!Seat.Lost` alone:
//
//	bestLife := -999
//	for i, s := range gs.Seats {
//	    if s != nil && !s.Lost && s.Life > bestLife { … }
//	}
//
// `!Lost` is not the same as alive. CR §104.3b: "If a player's life
// total is 0 or less, that player loses the game the next time a player
// would receive priority. (This is a state-based action.)" — and
// §704.5a restates it. The SBA is what sets `Lost`, and
// StateBasedActions returns early once the game has ended, so at the
// moment of adjudication a seat can sit at 0 or less life with `Lost`
// still false. Measured on AdjudicateGameEnd: a sole survivor at -3
// life is crowned `turn_cap_leader` with `Won = true`, and a table
// where all four seats are at or below 0 crowns the one at exactly 0.
//
// A seat at 0 life has lost. If no seat is left above 0, every player
// remaining has lost, and CR §104.4a is explicit: "If all the players
// remaining in a game lose simultaneously, the game is a draw."
//
// `bestLife := -999` is the defect in one line — the sentinel is below
// every reachable life total, so the loop cannot return "nobody".
//
// SIX COPIES, not the two the issue was filed with: AdjudicateGameEnd
// here, paritycheck (whose comment says "mirror runner.runOneGame"),
// and four in hexapi — spectate_rooms, the grinder, the bracket and the
// showmatch adjudicator. They disagree about more than this rule: only
// AdjudicateGameEnd marks the losing seats `Lost`, and only showmatch's
// copy sets `Flags["turn_capped"]`.
//
// THAT LAST ONE MATTERS FOR THE EVIDENCE. The Judge's §704.5a check
// reports `turn_capped=` so a reader can tell a post-end residue from a
// genuine crowning of a dead seat, and showmatch.go's r67c comment says
// the flag is set there precisely so "the Judge would not read
// turn_capped=false on a genuinely capped showmatch game and point the
// investigation the wrong way". The grinder's copy and the bracket's
// copy both run BEFORE their own Feynman pass and set no such flag — so
// across 674 `turn_cap_leader` games in the live log, all 494 §704.5a
// alerts read `turn_capped=false`, including the capped ones. The
// triage field was added to one of three paths and silently lies on the
// other two. Routing every copy through here fixes that as a
// side-effect of there being one copy.
//
// VOCABULARY IS DELIBERATELY UNCHANGED. `AdjudicateGameEnd` feeds the
// anti-cheat outcome digest from BOTH the sealing side and the replay
// side (see outcome_canon.go), and that file warns that the two sides
// drifting on end-reason vocabulary is what broke verification before.
// An all-at-or-below-zero table therefore reports the EXISTING
// "turn_cap_all_dead", not a new reason: the reading is the same one
// ("nobody is left alive to win") and no new string enters the digest.
// New seals do change for this shape, and a game sealed before r104
// replayed after it will disagree — that is the intended correction,
// not a regression.

// TurnCapEligible returns the seat indices that may win a turn-cap
// adjudication: not already marked Lost, AND at a life total above 0.
//
// The life test is the §104.3b / §704.5a one the SBA would have applied
// had it still been running. Seats failing it have lost; they are
// simply not marked yet.
func TurnCapEligible(gs *gameengine.GameState) []int {
	if gs == nil {
		return nil
	}
	var eligible []int
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Life <= 0 {
			continue
		}
		eligible = append(eligible, i)
	}
	return eligible
}

// TurnCapWinner is the whole turn-cap question as one call: the
// highest-life eligible seat, ties broken by lowest seat index.
//
// Returns (-1, "turn_cap_all_dead") when no seat is eligible — the
// CR §104.4a draw. Callers that only want the seat may ignore the
// reason; callers that report an end reason should use it, because
// "-1 with a reason" and "-1 because something went wrong" are
// different outcomes and the sentinel loop could not tell them apart.
//
// This does NOT mutate the game state. AdjudicateGameEnd owns the
// Lost/Won/flag bookkeeping; the hexapi read-only call sites must not
// acquire it by switching to this helper.
func TurnCapWinner(gs *gameengine.GameState) (int, string) {
	eligible := TurnCapEligible(gs)
	if len(eligible) == 0 {
		return -1, "turn_cap_all_dead"
	}
	winner := eligible[0]
	for _, i := range eligible[1:] {
		if gs.Seats[i].Life > gs.Seats[winner].Life {
			winner = i
		}
	}
	return winner, "turn_cap_leader"
}
