package gameengine

import (
	"testing"
)

// TestPlayFromGraveyard_PolicyLeaksWhenSourceDestroyed pins a silent
// correctness leak adjacent to the r60 ZoneCastGrant fixes: when
// RegisterPlayFromGraveyard is called with Permanent=true (Yawgmoth's
// Agenda, the static permanent variant of Yawgmoth's Will), the
// primitive registers THREE things tied to the source's Timestamp:
//
//  1. Per-Card ZoneCastGrants (cleaned synchronously on source LTB by
//     ExpireSourceGrants — added in the r60 zonecast-residual fix).
//  2. §614 GY→exile ReplacementEffects with SourcePerm bound (cleaned
//     by UnregisterReplacementsForPermanent on engine LTB pathway).
//  3. A ZoneCastPolicy with SourcePerm bound, Duration="while_source_on_bf"
//     (cleaned ONLY by UnregisterZoneCastPoliciesForPermanent — which
//     the engine LTB pathway in zone_change.go does NOT call).
//
// Yawgmoth's Agenda's per_card LTB handler calls
// UnregisterPlayFromGraveyardForPermanent, but that helper doesn't drop
// the policy either (only the per-Card grants + the seat flag). The
// docstring on yawgmoths_agenda.go even claims the engine LTB pathway
// invokes UnregisterZoneCastPoliciesForPermanent, but it does not.
//
// Net effect: after Agenda is destroyed, the ZoneCastPolicy persists.
// Any card entering its controller's graveyard later in the game still
// matches the policy and is treated as free-castable. The ZoneCastGrantExpiry
// invariant doesn't cover policies (it only walks gs.ZoneCastGrants),
// so this leak is silent — but it would let the AI / cast pipeline cast
// graveyard spells whose granting source no longer exists.
func TestPlayFromGraveyard_PolicyLeaksWhenSourceDestroyed(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Turn = 5
	agenda := addBattlefield(gs, 0, "Yawgmoth's Agenda", 0, 0, "enchantment")

	RegisterPlayFromGraveyard(gs, PlayFromGraveyardOptions{
		SeatIdx:    0,
		SourceName: agenda.Card.DisplayName(),
		SourcePerm: agenda,
		Permanent:  true,
	})

	policyCount := 0
	for _, p := range gs.ZoneCastPolicies {
		if p != nil && p.SourcePerm == agenda {
			policyCount++
		}
	}
	if policyCount != 1 {
		t.Fatalf("setup: expected 1 policy registered, got %d", policyCount)
	}

	// Destroy Agenda. Per the LTB chain (engine destroy + Agenda's
	// per_card LTB) the policy MUST be unregistered — its Duration is
	// "while_source_on_bf" and the source is now gone.
	if !DestroyPermanent(gs, agenda, nil) {
		t.Fatal("DestroyPermanent failed")
	}

	for _, p := range gs.ZoneCastPolicies {
		if p != nil && p.SourcePerm == agenda {
			t.Fatalf("policy %q with while_source_on_bf duration survived source LTB", p.HandlerID)
		}
	}
}
