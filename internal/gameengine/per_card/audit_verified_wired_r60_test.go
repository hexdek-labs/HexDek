package per_card

import "testing"

// TestAuditVerifiedWired_PinsKnownPreAuditHandlers locks in the 6
// cards identified by the Phase 1B audit false-positive sweep as
// pre-wired (`docs/audit-percard-registry-r60.md` §3a). Each was
// listed or implied as "no per_card consumer" in the original audit
// table but in fact has a handler registered via a different dispatch
// surface or event. If this test fails, either:
//
//   1. A handler was accidentally deleted — verify against the audit
//      doc's "Verified-wired" table and restore.
//
//   2. The handler was intentionally relocated — update both this
//      test and the audit doc to reflect the new location.
//
// This guard prevents a future "wire X" PR from re-wiring one of
// these cards and silently double-firing the payoff (the original
// failure mode that triggered the sweep, where PR #515 would have
// double-wired Olivia's Treasure trigger).
func TestAuditVerifiedWired_PinsKnownPreAuditHandlers(t *testing.T) {
	cases := []struct {
		card string
		// At least one of these must return true. The audit-time
		// handler used different dispatch surfaces depending on the
		// card; we don't lock to a specific one — only the existence
		// of SOME handler.
		hasTriggerOnly bool
		hasETB         bool
		hasActivated   bool
	}{
		// Audit row: investigate (Notable: Lonis is listed as the example).
		// Wired via OnTrigger("...", "nonland_permanent_etb", ...) +
		// OnActivated.
		{card: "Lonis, Cryptozoologist", hasTriggerOnly: true, hasActivated: true},

		// Audit row: outlaw_etb (Mechanic: OTJ outlaw cycle).
		// Wired via OnTrigger("...", "combat_damage_player", ...).
		{card: "Olivia, Opulent Outlaw", hasTriggerOnly: true},

		// Audit row: outlaw_etb (Mechanic: OTJ outlaw cycle).
		// Wired via OnTrigger("...", "combat_begin", ...).
		{card: "Vihaan, Goldwaker", hasTriggerOnly: true},

		// Audit row: outlaw_etb (Mechanic: OTJ outlaw cycle).
		// Wired via OnTrigger("...", "upkeep_controller", ...).
		{card: "Laughing Jasper Flint", hasTriggerOnly: true},

		// Audit row: magecraft (Mechanic: STX magecraft trigger).
		// Wired via OnTrigger("...", "instant_or_sorcery_cast", ...).
		{card: "Veyran, Voice of Duality", hasTriggerOnly: true},

		// Audit row: creature_mutated (Mechanic: IKO Mutate).
		// Wired via OnETB snowflake hook.
		{card: "Illuna, Apex of Wishes", hasETB: true},
	}
	reg := Global()
	for _, tc := range cases {
		_, hasTrig := reg.HasCastAndTrigger(normalizeName(tc.card))
		hasETB := HasETB(tc.card)
		hasAct := HasActivated(tc.card)

		anyRequired := tc.hasTriggerOnly || tc.hasETB || tc.hasActivated
		if !anyRequired {
			t.Fatalf("test case for %q has no surface expectation", tc.card)
		}
		ok := true
		if tc.hasTriggerOnly && !hasTrig {
			ok = false
		}
		if tc.hasETB && !hasETB {
			ok = false
		}
		if tc.hasActivated && !hasAct {
			ok = false
		}
		if !ok {
			t.Errorf("audit verified-wired card %q lost expected handler(s); "+
				"got trigger=%v etb=%v activated=%v — see docs/audit-percard-registry-r60.md §3a",
				tc.card, hasTrig, hasETB, hasAct)
		}
	}
}
