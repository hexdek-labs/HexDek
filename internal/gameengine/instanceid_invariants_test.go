package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// newPhase4GameState builds a 2-seat GameState with a fresh Minter and
// Phase 4 census maps initialized. Mirrors newPhase2GameState /
// newPhase3GameState in the sibling Phase 2+3 test files.
func newPhase4GameState(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, nil, nil)
	if gs == nil {
		t.Fatalf("NewGameState returned nil")
	}
	if gs.MintedInstanceIDs == nil {
		t.Fatalf("expected MintedInstanceIDs to be initialized")
	}
	if gs.CeasedInstanceIDs == nil {
		t.Fatalf("expected CeasedInstanceIDs to be initialized")
	}
	return gs
}

// --- CardIdentity (Phase 4 InstanceID equality) ---------------------------

// TestPhase4_CardIdentityFlagsDuplicateInstanceID pins the primary check:
// two distinct *Card pointers sharing the same non-empty InstanceID across
// zones are flagged. This is the strictly-stronger semantic than the
// pre-Phase-4 pointer-equality check.
func TestPhase4_CardIdentityFlagsDuplicateInstanceID(t *testing.T) {
	gs := newPhase4GameState(t)
	a := &Card{Name: "Bolt", Owner: 0, CMC: 1, Colors: []string{"R"}}
	MintOGInstanceID(gs, a)
	if a.InstanceID == "" {
		t.Fatalf("expected non-empty InstanceID after MintOG")
	}
	// Build a second *Card that fraudulently claims the same InstanceID.
	b := &Card{Name: "Bolt", Owner: 0, CMC: 1, Colors: []string{"R"}, InstanceID: a.InstanceID}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, a)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, b)
	err := checkCardIdentity(gs)
	if err == nil {
		t.Fatal("expected CardIdentity to flag duplicate InstanceID across zones")
	}
	if !strings.Contains(err.Error(), "InstanceID") || !strings.Contains(err.Error(), a.InstanceID) {
		t.Fatalf("expected error to cite InstanceID %q, got: %v", a.InstanceID, err)
	}
}

// TestPhase4_CardIdentityPassesUniqueInstanceIDs pins the negative path:
// every Card has a unique InstanceID + lives in exactly one zone → no
// violation.
func TestPhase4_CardIdentityPassesUniqueInstanceIDs(t *testing.T) {
	gs := newPhase4GameState(t)
	for i := 0; i < 5; i++ {
		c := &Card{Name: "Card", Owner: 0, CMC: i, Colors: []string{"U"}}
		MintOGInstanceID(gs, c)
		gs.Seats[0].Library = append(gs.Seats[0].Library, c)
	}
	if err := checkCardIdentity(gs); err != nil {
		t.Fatalf("clean state should not fire CardIdentity: %v", err)
	}
}

// TestPhase4_CardIdentityFallsBackToPointerForLegacy pins the legacy
// fallback: cards with empty InstanceID (pre-Phase-1 mode) still trip the
// pointer-equality check when they appear in two zones.
func TestPhase4_CardIdentityFallsBackToPointerForLegacy(t *testing.T) {
	gs := newPhase4GameState(t)
	// Construct a legacy *Card WITHOUT minting — InstanceID stays empty.
	leg := &Card{Name: "Legacy", Owner: 0}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, leg)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, leg) // same pointer in two zones
	err := checkCardIdentity(gs)
	if err == nil {
		t.Fatal("expected legacy pointer-equality check to fire")
	}
	if !strings.Contains(err.Error(), "ptr") {
		t.Fatalf("expected pointer-equality message, got: %v", err)
	}
}

// --- ZoneConservation (Phase 4 InstanceID census) -------------------------

// TestPhase4_ZoneConservationCleanCensusPasses pins the happy path:
// every minted ID is in some zone and no fabricated IDs are present.
func TestPhase4_ZoneConservationCleanCensusPasses(t *testing.T) {
	gs := newPhase4GameState(t)
	for i := 0; i < 4; i++ {
		c := &Card{Name: "Card", Owner: 0, CMC: i, Colors: []string{"R"}}
		MintOGInstanceID(gs, c)
		gs.Seats[0].Library = append(gs.Seats[0].Library, c)
	}
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("clean census should not fire: %v", err)
	}
}

// TestPhase4_ZoneConservationFlagsFabrication pins fabrication detection:
// a card in a zone whose InstanceID is NOT in MintedInstanceIDs trips
// the invariant.
func TestPhase4_ZoneConservationFlagsFabrication(t *testing.T) {
	gs := newPhase4GameState(t)
	// Seed one legit OG mint so MintedInstanceIDs is non-empty (else
	// the InstanceID branch short-circuits to the legacy count path).
	legit := &Card{Name: "Legit", Owner: 0, CMC: 1, Colors: []string{"G"}}
	MintOGInstanceID(gs, legit)
	gs.Seats[0].Library = append(gs.Seats[0].Library, legit)

	// Inject a card with a hand-rolled InstanceID that was never minted.
	fab := &Card{Name: "Fab", Owner: 0, InstanceID: "h0OGVC100099"}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, fab)
	err := checkZoneConservation(gs)
	if err == nil {
		t.Fatal("expected fabrication detection")
	}
	if !strings.Contains(err.Error(), "fabrication") {
		t.Fatalf("expected fabrication error, got: %v", err)
	}
}

// TestPhase4_ZoneConservationFlagsDisappearance pins disappearance
// detection: an ID is in MintedInstanceIDs (and not in CeasedInstanceIDs)
// but is not present in any zone.
func TestPhase4_ZoneConservationFlagsDisappearance(t *testing.T) {
	gs := newPhase4GameState(t)
	// Strict mode is opt-in for now (mint-coverage gap; see comment in
	// checkZoneConservationByInstanceID). Property tests enable it
	// explicitly so the regression pin holds.
	gs.Flags["instanceid_strict_census"] = 1
	ghost := &Card{Name: "Ghost", Owner: 0, CMC: 2, Colors: []string{"B"}}
	MintOGInstanceID(gs, ghost)
	// Do NOT add ghost to any zone — but it IS in MintedInstanceIDs.
	err := checkZoneConservation(gs)
	if err == nil {
		t.Fatal("expected disappearance detection")
	}
	if !strings.Contains(err.Error(), "disappeared") {
		t.Fatalf("expected disappearance error, got: %v", err)
	}
}

// TestPhase4_ZoneConservationDisappearanceOnByDefault pins the post-
// gap-walk policy: strict-census is now ON by default (the 2.9M-hit
// disappearance cluster from PR #755 has been closed by the gap-walk
// re-mint + zone-purge backstops, so the strict arm produces a clean
// signal at production-grade depths). Callers can opt OUT via
// SetStrictCensusDefault(false) for legacy struct-literal tests.
func TestPhase4_ZoneConservationDisappearanceOnByDefault(t *testing.T) {
	gs := newPhase4GameState(t)
	// Confirm the default flag is set.
	if gs.Flags["instanceid_strict_census"] != 1 {
		t.Fatalf("expected strict-census ON by default post-gap-walk; got flag=%d", gs.Flags["instanceid_strict_census"])
	}
	ghost := &Card{Name: "Ghost", Owner: 0, CMC: 2, Colors: []string{"B"}}
	MintOGInstanceID(gs, ghost)
	if err := checkZoneConservation(gs); err == nil {
		t.Fatal("expected disappearance detection in default (strict) mode")
	}
}

// TestPhase4_SetStrictCensusDefault_OptOut pins the legacy escape hatch
// — SetStrictCensusDefault(false) reverts to the pre-gap-walk gated
// behavior so struct-literal tests can stay quiet.
func TestPhase4_SetStrictCensusDefault_OptOut(t *testing.T) {
	SetStrictCensusDefault(false)
	defer SetStrictCensusDefault(true)
	gs := newPhase4GameState(t)
	if gs.Flags["instanceid_strict_census"] == 1 {
		t.Fatalf("expected strict-census OFF after opt-out; got flag=1")
	}
	ghost := &Card{Name: "Ghost", Owner: 0, CMC: 2, Colors: []string{"B"}}
	MintOGInstanceID(gs, ghost)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("opt-out mode must NOT flag disappearance: %v", err)
	}
}

// TestPhase4_ZoneConservationCessationExcludes pins §707.10 / §704.5d
// cessation semantics: a ceased ID drops out of the census expectation
// so a "disappeared" missing card doesn't false-positive.
func TestPhase4_ZoneConservationCessationExcludes(t *testing.T) {
	gs := newPhase4GameState(t)
	// Mint a TK token, then mark it ceased — invariant should not flag.
	tok := &Card{Name: "Thopter", Owner: 0, CMC: 0, Colors: nil, Types: []string{"token"}}
	MintTokenInstanceID(gs, tok, "", "")
	if tok.InstanceID == "" {
		t.Fatalf("expected TK InstanceID on Thopter")
	}
	MarkInstanceIDCeased(gs, tok.InstanceID)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("ceased token should not fire census: %v", err)
	}
}

// TestPhase4_ZoneConservationAbilityIDsExcluded pins that AB-provenance
// IDs do NOT enter the census expectation — they are ephemeral stack
// items, not zone-residing cards.
func TestPhase4_ZoneConservationAbilityIDsExcluded(t *testing.T) {
	gs := newPhase4GameState(t)
	// Build a permanent and mint an AbilityInstance from it.
	src := &Card{Name: "Etali, Primal Storm", Owner: 0, CMC: 6, Colors: []string{"R"}}
	MintOGInstanceID(gs, src)
	perm := &Permanent{Card: src, Owner: 0, Controller: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	ab := NewAbilityInstance(gs, perm, 0, "trig:creature_attacks", "", nil)
	if ab.InstanceID == "" {
		t.Fatalf("expected non-empty AB InstanceID")
	}
	if _, ok := gs.MintedInstanceIDs[ab.InstanceID]; !ok {
		t.Fatalf("AbilityInstance ID should be recorded in MintedInstanceIDs")
	}
	// AB IDs not in any zone — invariant must NOT flag disappearance.
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("AB-provenance ID should not enter census: %v", err)
	}
}

// TestPhase4_ZoneConservationLeftGameSeatExcluded pins §800.4a: a seat
// that has LeftGame=true is skipped in the census walk, since its owned
// cards have been marked ceased in HandleSeatElimination.
func TestPhase4_ZoneConservationLeftGameSeatExcluded(t *testing.T) {
	gs := newPhase4GameState(t)
	c := &Card{Name: "Card", Owner: 1, CMC: 1, Colors: []string{"W"}}
	MintOGInstanceID(gs, c)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, c)
	// Mark seat 1 left + cease its owned IDs (simulating HandleSeatElimination
	// post-condition without invoking the full multiplayer path).
	gs.Seats[1].LeftGame = true
	MarkInstanceIDCeased(gs, c.InstanceID)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("LeftGame seat must be excluded from census: %v", err)
	}
}

// --- ExileLinkageIntegrity (Phase 4 two-pronged §7) -----------------------

// TestPhase4_ELI_LTBReturnSourceHeldCleanPasses pins prong A's happy
// path: source on battlefield, ExiledByMe populated, card actually in
// some seat's exile.
func TestPhase4_ELI_LTBReturnSourceHeldCleanPasses(t *testing.T) {
	gs := newPhase4GameState(t)
	priest := banisherPriestShape(t, gs, 0)
	prey := targetCardOnBattlefield(t, gs, 1)
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	ExileLinked(gs, priest, prey.Card, prey.Owner, "battlefield")
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("clean LTBReturn state must pass: %v", err)
	}
}

// TestPhase4_ELI_BrokenLTBReturnFires pins prong A's bug detection: an
// InstanceID is in ExiledByMe but the card has been surgically removed
// from exile through a non-canonical path (the historical r60 anti-
// pattern that Phase 4 is designed to catch).
func TestPhase4_ELI_BrokenLTBReturnFires(t *testing.T) {
	gs := newPhase4GameState(t)
	priest := banisherPriestShape(t, gs, 0)
	prey := targetCardOnBattlefield(t, gs, 1)
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	ExileLinked(gs, priest, prey.Card, prey.Owner, "battlefield")
	// Surgical removal that does NOT clear priest.ExiledByMe.
	gs.Seats[1].Exile = gs.Seats[1].Exile[:0]
	err := checkExileLinkageIntegrity(gs)
	if err == nil {
		t.Fatal("expected prong A to fire on broken LTBReturn")
	}
	if !strings.Contains(err.Error(), prey.Card.InstanceID) {
		t.Fatalf("expected error to cite orphan InstanceID %q, got: %v",
			prey.Card.InstanceID, err)
	}
}

// TestPhase4_ELI_CastGrantSkipsSourceCheck pins prong B (§7 self-managed
// carveout): a CastGrant-tagged source's exiled card stays exiled after
// the source dies, and the invariant does NOT false-positive.
func TestPhase4_ELI_CastGrantSkipsSourceCheck(t *testing.T) {
	gs := newPhase4GameState(t)
	etali := &Card{Name: "Etali", Owner: 0, CMC: 6, Colors: []string{"R"}}
	MintOGInstanceID(gs, etali)
	perm := &Permanent{Card: etali, Owner: 0, Controller: 0, Timestamp: gs.NextTimestamp(), LinkageKind: LinkageNone}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	exiled := &Card{Name: "Bolt", Owner: 1, CMC: 1, Colors: []string{"R"}}
	MintOGInstanceID(gs, exiled)
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, exiled)
	// No ExiledByTimestamp set — CastGrant doesn't use timestamp linkage.

	ab := NewAbilityInstance(gs, perm, 0, "trig:creature_attacks", "", nil)
	grant := NewFreeCastFromExilePermission(0, "Etali")
	grant.Duration = "until_end_of_turn"
	grant.GrantTurn = gs.Turn
	grant.AbilityInstanceID = ab.InstanceID
	grant.LinkageKind = CastGrant
	RegisterZoneCastGrant(gs, exiled, grant)

	// Etali dies — CastGrant exile stays.
	gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("CastGrant must skip source-held check post-LTB: %v", err)
	}
}

// TestPhase4_ELI_PermanentExileNoLinkage pins prong B's second branch:
// cards exiled with no return mechanism (Settle the Wreckage, disturb-
// cast originals) carry no source back-reference and the invariant
// silently accepts them.
func TestPhase4_ELI_PermanentExileNoLinkage(t *testing.T) {
	gs := newPhase4GameState(t)
	exiled := &Card{Name: "Path Victim", Owner: 1, CMC: 3, Colors: []string{"G"}}
	MintOGInstanceID(gs, exiled)
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, exiled)
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("PermanentExile state must pass: %v", err)
	}
}

// --- Mint-set / Cease-set bookkeeping properties --------------------------

// TestPhase4_MintHelpersRecordIntoMintedSet pins that every Mint helper
// pipes its newly-issued ID into gs.MintedInstanceIDs. The census
// invariant relies on this — a Mint helper that skipped the record would
// produce silent fabrication false positives.
func TestPhase4_MintHelpersRecordIntoMintedSet(t *testing.T) {
	gs := newPhase4GameState(t)

	// OG
	og := &Card{Name: "OG", Owner: 0, CMC: 1, Colors: []string{"R"}}
	MintOGInstanceID(gs, og)
	if _, ok := gs.MintedInstanceIDs[og.InstanceID]; !ok {
		t.Errorf("OG mint not recorded: %q", og.InstanceID)
	}

	// TK
	tok := &Card{Name: "Tok", Owner: 0, Colors: nil, Types: []string{"token"}}
	MintTokenInstanceID(gs, tok, "", "")
	if _, ok := gs.MintedInstanceIDs[tok.InstanceID]; !ok {
		t.Errorf("TK mint not recorded: %q", tok.InstanceID)
	}

	// CP
	cp := &Card{Name: "Copy", Owner: 0, CMC: 1, Colors: []string{"R"}}
	MintCopyInstanceID(gs, cp, "src-OG-001", "src-AB-001")
	if _, ok := gs.MintedInstanceIDs[cp.InstanceID]; !ok {
		t.Errorf("CP mint not recorded: %q", cp.InstanceID)
	}

	// AB
	srcCard := &Card{Name: "Src", Owner: 0, CMC: 2, Colors: []string{"U"}}
	MintOGInstanceID(gs, srcCard)
	perm := &Permanent{Card: srcCard, Owner: 0, Controller: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	ab := NewAbilityInstance(gs, perm, 0, "act:0", "", nil)
	if _, ok := gs.MintedInstanceIDs[ab.InstanceID]; !ok {
		t.Errorf("AB mint not recorded: %q", ab.InstanceID)
	}
}

// TestPhase4_MarkInstanceIDCeasedHandlesNilSafely pins defensive nil /
// empty-id behavior. Empty ids and nil gs must not panic and must not
// pollute the ceased map.
func TestPhase4_MarkInstanceIDCeasedHandlesNilSafely(t *testing.T) {
	MarkInstanceIDCeased(nil, "h0OGVC100000")
	gs := newPhase4GameState(t)
	MarkInstanceIDCeased(gs, "")
	if len(gs.CeasedInstanceIDs) != 0 {
		t.Fatalf("empty id must not enter CeasedInstanceIDs, got: %v", gs.CeasedInstanceIDs)
	}
}

// TestPhase4_InstanceIDFormatRegexHoldsAfterCensus pins that every ID
// minted in this test file matches §3 FormatRegex — a defensive cross-
// check that Phase 4 didn't accidentally introduce non-canonical IDs.
func TestPhase4_InstanceIDFormatRegexHoldsAfterCensus(t *testing.T) {
	gs := newPhase4GameState(t)
	c := &Card{Name: "Card", Owner: 0, CMC: 3, Colors: []string{"W", "U"}}
	MintOGInstanceID(gs, c)
	tok := &Card{Name: "Tok", Owner: 1, Types: []string{"token"}}
	MintTokenInstanceID(gs, tok, "", "")
	cp := &Card{Name: "Copy", Owner: 0, CMC: 1, Colors: []string{"R"}}
	MintCopyInstanceID(gs, cp, "src-OG", "src-AB")
	for _, id := range []string{c.InstanceID, tok.InstanceID, cp.InstanceID} {
		if !instanceid.FormatRegex.MatchString(id) {
			t.Errorf("InstanceID %q does not match FormatRegex", id)
		}
	}
}
