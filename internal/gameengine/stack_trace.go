package gameengine

// Stack resolution audit tool — traces push, resolve, priority pass, SBA
// check, and trigger activity on the stack for CR compliance verification.
//
// The trace logger is intentionally lightweight: a global singleton that
// can be enabled/disabled per test or debugging session. When disabled,
// the Log call is a single branch + return — no allocations.
//
// Key stack events traced:
//   - "push"             — spell or ability pushed onto the stack (CR §601.2a / §603.3)
//   - "resolve"          — top of stack resolves (CR §608.2)
//   - "priority_pass"    — all players pass priority (CR §117.4)
//   - "sba_check"        — state-based actions checked (CR §704.3)
//   - "trigger_push"     — triggered ability pushed (CR §603.2)
//   - "trigger_resolve"  — triggered ability resolves (§608.2 for triggers)

// StackTraceEntry is one logged event in the stack trace.
type StackTraceEntry struct {
	Action    string // "push", "resolve", "priority_pass", "sba_check", "trigger_push", "trigger_resolve"
	Card      string // card/ability name
	Seat      int
	StackSize int    // stack depth at this point
	Detail    string // additional context
}

// StackTrace accumulates trace entries when Enabled.
type StackTrace struct {
	Entries []StackTraceEntry
	Enabled bool
}

// GlobalStackTrace is the process-wide trace sink. Tests enable it,
// run game actions, then inspect Entries for CR compliance.
var GlobalStackTrace = &StackTrace{}

// Log appends one trace entry. No-op when disabled.
func (st *StackTrace) Log(action, card string, seat, stackSize int, detail string) {
	if !st.Enabled {
		return
	}
	st.Entries = append(st.Entries, StackTraceEntry{
		Action:    action,
		Card:      card,
		Seat:      seat,
		StackSize: stackSize,
		Detail:    detail,
	})
}

// Reset clears the trace buffer without reallocating.
func (st *StackTrace) Reset() {
	st.Entries = st.Entries[:0]
}

// Enable turns on trace logging.
func (st *StackTrace) Enable() { st.Enabled = true }

// Disable turns off trace logging.
func (st *StackTrace) Disable() { st.Enabled = false }
