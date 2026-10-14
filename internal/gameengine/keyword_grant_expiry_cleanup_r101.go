package gameengine

// r101 — "until end of turn" expiry fired one step early.
//
// THE DEFECT. A handler granting a keyword (or control) "until end of
// turn" scheduled its own undo as a delayed trigger with
// TriggerAt "next_end_step" / "end_of_turn". delayedTriggerMatches maps
// both to `step == "end"`, so the undo ran in the §513 END STEP.
//
// CR §514.2 — "When the cleanup step begins ... all 'until end of turn'
// and 'this turn' effects end." The end step is §513; the cleanup step
// is §514 and comes after it. Between the two sits the §513.1 priority
// window, the single highest-traffic instant-speed window in the game:
// end-of-turn removal, flash creatures, Teferi's Protection. For the
// whole of that window the grant was already gone.
//
// Measured on the production entry point before the fix: Skithiryx
// activates {B} for haste, FireDelayedTriggers(gs, "ending", "end")
// runs, and haste is already absent — one full priority window early.
//
// It is worse when someone ends the turn. CR §724.1d skips straight to
// the cleanup step, so the §513 end step never happens — and CR §724.1e
// says so explicitly: "Even though the turn ends, 'at the beginning of
// the end step' triggered abilities don't trigger because the end step
// is skipped." Sundial of the Infinite's handler then purges every
// end-step delayed trigger outright. The undo therefore never ran at
// all: the keyword, or a stolen creature, was kept for good.
//
// That is also the clearest argument that this was the wrong shape. A
// §514.2 expiry is a turn-based action, not a triggered ability, so the
// rule that silences end-step TRIGGERS must not silence it.
//
// THE FIX. A new TriggerAt value, CleanupStepTrigger, matched on
// `step == "cleanup"` and fired from the cleanup arm of
// ScanExpiredDurations. Hooking the sweep rather than the turn driver
// means every driver that performs §514.2 gets it, including
// fastForwardCleanup.
//
// WHY NOT THE kw_eot: MARKER. ExpireKeywordGrantsEOT already expires
// keyword flags at the correct time, and the five handlers that opted
// in are correct. It is not sufficient here: a good number of these
// closures also revert a CONTROL change (Insurrection, Zidane, Dihada's
// -11, Kenrith), which is equally an "until end of turn" effect under
// §514.2 but cannot be expressed as a keyword marker. The closure is
// the right vehicle for the compound cases; only its timing was wrong.
// Grants that are purely a keyword should still prefer the marker.
const CleanupStepTrigger = "cleanup"

// FireCleanupStepExpiries runs the delayed triggers that model §514.2
// "until end of turn" expiry. Called from the cleanup arm of
// ScanExpiredDurations.
//
// The §514.3a loop can run the cleanup step several times in one turn;
// these triggers are registered OneShot and FireDelayedTriggers marks
// each Consumed as it fires, so a repeated pass is a no-op.
func FireCleanupStepExpiries(gs *GameState) int {
	if gs == nil {
		return 0
	}
	return FireDelayedTriggers(gs, "ending", "cleanup")
}
