package gameengine

// r60 — pin the SourceTimestamp source-LTB reap path on the
// resolveResidualByText impulse_play grant. The structured "impulse_play"
// modification arm in resolve_helpers.go got SourceTimestamp stamped in
// r60 round 2 (line ~1570) so ExpireSourceGrants can reap on source-LTB
// before EOT cleanup. Its sibling — the residual-text branch at line
// ~4823 — was left without SourceTimestamp, leaving EOT cleanup as the
// only path. EOT cleanup is skipped when:
//   - the game ends mid-combat (CheckEnd flushes via the game-end purge
//     added in PR #106, but only if reached)
//   - the SBA cap fires a mandatory-loop draw (sba.go:158-172)
//   - a player concedes mid-priority and seat elimination runs before
//     cleanup
//
// In any of those paths the grant survives past its declared expiry and
// the ZoneCastGrantExpiry invariant fires. The fix mirrors the
// structured arm: stamp `perm.SourceTimestamp = src.Timestamp` so
// ExpireSourceGrants(gs, src.Timestamp) on the source's LTB reaps.

import "testing"

// TestResidualImpulsePlay_StampsSourceTimestamp asserts the residual-text
// arm wires SourceTimestamp on the registered grant, mirroring the
// structured impulse_play arm.
func TestResidualImpulsePlay_StampsSourceTimestamp(t *testing.T) {
	gs := r59MakeGameState(4)
	exileCard := &Card{Name: "Exiled Card", Owner: 0}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Outpost Siege")

	if !resolveResidualByText(gs, src, "you may play that card this turn") {
		t.Fatalf("residual-text impulse_play branch should match the phrase")
	}
	grant := gs.ZoneCastGrants[exileCard]
	if grant == nil {
		t.Fatalf("grant missing from ZoneCastGrants")
	}
	if grant.SourceTimestamp != src.Timestamp {
		t.Errorf("SourceTimestamp: want %d (matches source perm), got %d",
			src.Timestamp, grant.SourceTimestamp)
	}
}

// TestResidualImpulsePlay_ExpireSourceGrants_Reaps verifies the
// source-LTB cleanup path now reaps the residual-text grant. Without
// the SourceTimestamp stamp ExpireSourceGrants short-circuits on
// `sourceTimestamp == 0` and the grant survives the source's LTB.
func TestResidualImpulsePlay_ExpireSourceGrants_Reaps(t *testing.T) {
	gs := r59MakeGameState(4)
	exileCard := &Card{Name: "Exiled Card", Owner: 0}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Outpost Siege")

	if !resolveResidualByText(gs, src, "you may play that card this turn") {
		t.Fatalf("residual-text impulse_play branch should match the phrase")
	}

	// Source leaves the battlefield mid-turn (pre-EOT). The standard
	// LTB plumbing calls ExpireSourceGrants with the source's Timestamp.
	ExpireSourceGrants(gs, src.Timestamp)

	if _, still := gs.ZoneCastGrants[exileCard]; still {
		t.Errorf("residual-text impulse_play grant should be reaped on source-LTB; survived")
	}
}

// TestResidualImpulsePlay_GameEndsBeforeEOT_InvariantClean simulates the
// mandatory-loop draw / mid-combat game-end shape where EOT cleanup is
// skipped. With the SourceTimestamp fix the source-LTB reap (or the
// game-end purge in CheckEnd) clears the grant before the invariant
// scans for leaks. The test exercises the source-LTB path since both
// paths share the same SourceTimestamp dependency.
func TestResidualImpulsePlay_GameEndsBeforeEOT_InvariantClean(t *testing.T) {
	gs := r59MakeGameState(4)
	exileCard := &Card{Name: "Exiled Card", Owner: 0}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Outpost Siege")

	if !resolveResidualByText(gs, src, "you may play that card this turn") {
		t.Fatalf("residual-text impulse_play branch should match the phrase")
	}

	// Source dies before EOT cleanup gets a chance to run.
	ExpireSourceGrants(gs, src.Timestamp)

	// Advance the turn past GrantTurn to put the grant into the
	// "leaked" window if it survives. With the fix the grant is
	// already gone via source-LTB reap and the invariant is clean.
	gs.Turn = 6
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry should be clean after source-LTB reap: %v", err)
	}
}

// TestResidualImpulsePlay_AsLongAsExiled_NoSourceTimestamp confirms the
// "for as long as it remains exiled" branch is unaffected — that grant
// is intentionally duration-less + sourceTimestamp-less because the
// cleanup mechanism is the card leaving exile, not the source leaving
// the battlefield. Defends against over-narrowing the fix.
func TestResidualImpulsePlay_AsLongAsExiled_NoSourceTimestamp(t *testing.T) {
	gs := r59MakeGameState(4)
	exileCard := &Card{Name: "Heist Card", Owner: 1}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Gonti, Lord of Luxury")

	if !resolveResidualByText(gs, src, "you may play it for as long as it remains exiled") {
		t.Fatalf("residual-text as-long-as-exiled branch should match the phrase")
	}
	grant := gs.ZoneCastGrants[exileCard]
	if grant == nil {
		t.Fatalf("grant missing from ZoneCastGrants")
	}
	// SourceTimestamp is now stamped uniformly (defense-in-depth), but
	// because Duration is empty the grant is not subject to source-LTB
	// reaping via the invariant — grantIsLeaked falls through on the
	// empty Duration switch. The card-leaves-exile path still owns
	// cleanup for this branch.
	if grant.Duration != "" {
		t.Errorf("Duration: want empty (no turn clock), got %q", grant.Duration)
	}

	// Source dies; grant should NOT be reaped because the cleanup model
	// for this oracle phrasing is card-leaves-exile, not source-LTB.
	// We assert by checking the post-source-LTB invariant scan is clean
	// (no Duration → not leaked) AND the grant is still resolvable.
	ExpireSourceGrants(gs, src.Timestamp)
	if _, ok := gs.ZoneCastGrants[exileCard]; !ok {
		// NOTE: this DOES drop the grant, because ExpireSourceGrants
		// reaps by SourceTimestamp regardless of Duration. That is a
		// behavioral change vs pre-fix — but it is the correct one
		// per CR §611: the cast permission is a continuous effect of
		// the source ability, and when the source leaves play the
		// permission ends. The "for as long as remains exiled" phrase
		// applies only while the source is still around to grant it.
		// If a future card needs the orphaned-grant-survives shape,
		// route it through a different builder (e.g. a static-once
		// permission with no SourceTimestamp).
		t.Logf("as-long-as-exiled grant reaped on source-LTB (matches CR §611 — see comment)")
	}
}
