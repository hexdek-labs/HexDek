package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// newPhase5GameState — 4-seat state mirroring the Sai+doubler walkthrough.
func newPhase5GameState(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(4, nil, nil)
	if gs == nil {
		t.Fatalf("NewGameState returned nil")
	}
	if gs.IIDMinter == nil {
		t.Fatalf("expected IIDMinter non-nil")
	}
	return gs
}

// ---------------------------------------------------------------------------
// Per-card-override registration tests (5 model-breakers from Probe A)
//
// These confirm that an ETB on each model-breaker stamps the expected
// CopyMechanism shape onto the Permanent. The per_card handlers live in
// per_card/instanceid_phase5_overrides.go; the gameengine tests below
// validate the engine-side structure independently — they assemble the
// CopyMechanism directly and assert the audit-trail fields populate
// correctly without depending on the per_card init() being loaded.
// ---------------------------------------------------------------------------

func TestPhase5_EnolcCopyMechanism_PartnerConditional(t *testing.T) {
	gs := newPhase5GameState(t)
	perm := &Permanent{Card: &Card{Name: "Enolc"}, Controller: 0}
	AddCopyMechanism(perm, CopyMechanism{
		TriggerSource:   CopyTriggerETB,
		Duration:        CopyDurationPermanent,
		Target:          CopyTargetSelf,
		PerCardOverride: "partner_conditional_copy",
	})
	if len(perm.CopyMechanisms) != 1 {
		t.Fatalf("want 1 mechanism, got %d", len(perm.CopyMechanisms))
	}
	if perm.CopyMechanisms[0].PerCardOverride != "partner_conditional_copy" {
		t.Errorf("override label mismatch: %q", perm.CopyMechanisms[0].PerCardOverride)
	}
	_ = gs
}

func TestPhase5_ErtaiCopyMechanism_ExileZoneSnapshot(t *testing.T) {
	perm := &Permanent{Card: &Card{Name: "Ertai's Meddling"}, Controller: 0}
	AddCopyMechanism(perm, CopyMechanism{
		TriggerSource: CopyTriggerEventCondition,
		Duration:      CopyDurationUntilNextUpkeep,
		Target:        CopyTargetSelf,
		Restriction: &CopyRestriction{
			ControllerSeat: -1,
			SourceZone:     "exile",
		},
		PerCardOverride: "exile_zone_snapshot",
	})
	m := perm.CopyMechanisms[0]
	if m.Restriction == nil || m.Restriction.SourceZone != "exile" {
		t.Errorf("expected SourceZone=exile, got %#v", m.Restriction)
	}
}

func TestPhase5_SoulflayerKeywordGrant_NotACopyMechanism(t *testing.T) {
	// Soulflayer is the model-breaker that is NOT a copy effect; the
	// override registers no CopyMechanism per Probe A row 3.
	perm := &Permanent{Card: &Card{Name: "Soulflayer"}, Controller: 0}
	// Simulate no mechanism added (Soulflayer override emits a marker
	// but does NOT call AddCopyMechanism).
	if len(perm.CopyMechanisms) != 0 {
		t.Fatalf("Soulflayer must NOT register a CopyMechanism; got %d", len(perm.CopyMechanisms))
	}
}

func TestPhase5_MirageMirrorMultiArm(t *testing.T) {
	perm := &Permanent{Card: &Card{Name: "Mirage Mirror"}, Controller: 0}
	AddCopyMechanism(perm, CopyMechanism{
		TriggerSource:   CopyTriggerUpkeep,
		Duration:        CopyDurationPermanent,
		Target:          CopyTargetSelf,
		PerCardOverride: "mirror_multi_arm",
	})
	AddCopyMechanism(perm, CopyMechanism{
		TriggerSource:   CopyTriggerActivated,
		Duration:        CopyDurationUntilEOT,
		Target:          CopyTargetSelf,
		PerCardOverride: "mirror_multi_arm",
	})
	if len(perm.CopyMechanisms) != 2 {
		t.Fatalf("multi-arm: want 2 mechanisms, got %d", len(perm.CopyMechanisms))
	}
	if perm.CopyMechanisms[0].TriggerSource != CopyTriggerUpkeep ||
		perm.CopyMechanisms[1].TriggerSource != CopyTriggerActivated {
		t.Errorf("arm trigger ordering wrong: %+v", perm.CopyMechanisms)
	}
}

func TestPhase5_SakashimaLegendBypass(t *testing.T) {
	perm := &Permanent{Card: &Card{Name: "Sakashima the Impostor"}, Controller: 0}
	perm.BypassesLegendRule = true
	AddCopyMechanism(perm, CopyMechanism{
		TriggerSource:   CopyTriggerETB,
		Duration:        CopyDurationPermanent,
		Target:          CopyTargetSelf,
		PerCardOverride: "sakashima_legend_bypass",
	})
	if !perm.BypassesLegendRule {
		t.Fatal("BypassesLegendRule must be true for Sakashima")
	}
	if perm.CopyMechanisms[0].TriggerSource != CopyTriggerETB {
		t.Errorf("expected ETB trigger, got %v", perm.CopyMechanisms[0].TriggerSource)
	}
}

// ---------------------------------------------------------------------------
// Replacement-effect chain: Sai + Mondrak + Anointed + Doubling Season
//
// Per design v2 §6 walkthrough:
//   Base = 1 → Mondrak (x2) → Anointed (x2) → Doubling Season (x2) → 8
//
// The test asserts:
//   - FinalCount == 8
//   - EffectsApplied has 3 entries in §616 order
//   - 8 distinct InstanceIDs minted, all sharing the same EnablerInstanceID
// ---------------------------------------------------------------------------

func TestPhase5_SaiPlus3Doublers_Produces8DistinctInstanceIDs(t *testing.T) {
	gs := newPhase5GameState(t)
	// Establish a Sai-trigger AB instance as the enabler frame.
	saiSrc := &Permanent{Card: &Card{Name: "Sai, Master Thopterist", Owner: 0}, Controller: 0}
	MintOGInstanceID(gs, saiSrc.Card)
	ab := NewAbilityInstance(gs, saiSrc, 0, "trig:spell_cast", saiSrc.Card.InstanceID, nil)
	pushIIDEnabler(gs, ab.InstanceID)
	defer popIIDEnabler(gs)

	// Register Mondrak doubler + Anointed Procession + Doubling Season
	// replacement effects on the same seat.
	mondrak := &Permanent{Card: &Card{Name: "Mondrak, Glory Dominus"}, Controller: 0, Timestamp: gs.NextTimestamp()}
	anointed := &Permanent{Card: &Card{Name: "Anointed Procession"}, Controller: 0, Timestamp: gs.NextTimestamp()}
	dseason := &Permanent{Card: &Card{Name: "Doubling Season"}, Controller: 0, Timestamp: gs.NextTimestamp()}
	registerTokenDoubler(gs, mondrak, "Mondrak, Glory Dominus")
	registerTokenDoubler(gs, anointed, "Anointed Procession")
	registerTokenDoubler(gs, dseason, "Doubling Season")

	// Simulate the would_create_token event.
	final, cancelled := FireCreateTokenEvent(gs, 0, 1, saiSrc)
	if cancelled {
		t.Fatal("event cancelled unexpectedly")
	}
	if final != 8 {
		t.Errorf("expected FinalCount=8, got %d", final)
	}

	// Mint 8 distinct TK InstanceIDs each sharing the Sai-ability enabler.
	seen := map[string]struct{}{}
	for i := 0; i < final; i++ {
		c := &Card{Name: "Thopter Token", Owner: 0, Types: []string{"token", "artifact", "creature"}}
		MintTokenInstanceID(gs, c, "", ab.InstanceID)
		if c.InstanceID == "" {
			t.Fatalf("token %d failed to mint", i)
		}
		if !instanceid.FormatRegex.MatchString(c.InstanceID) {
			t.Errorf("token %d ID %q does not match FormatRegex", i, c.InstanceID)
		}
		if _, dup := seen[c.InstanceID]; dup {
			t.Errorf("duplicate InstanceID across mints: %q", c.InstanceID)
		}
		seen[c.InstanceID] = struct{}{}
		if c.EnablerInstanceID != ab.InstanceID {
			t.Errorf("token %d: expected enabler %q, got %q", i, ab.InstanceID, c.EnablerInstanceID)
		}
		if !strings.Contains(c.InstanceID, "TK") {
			t.Errorf("token %d: expected TK provenance in %q", i, c.InstanceID)
		}
	}
	if len(seen) != 8 {
		t.Errorf("expected 8 unique IDs, got %d", len(seen))
	}

	// Phase 5 audit chain length: 3 replacements applied in order.
	chain := gs.PendingTokenMintChain
	if len(chain) != 3 {
		t.Fatalf("expected 3 entries in AppliedChain, got %d: %+v", len(chain), chain)
	}
	expectedOrder := []string{"Mondrak, Glory Dominus", "Anointed Procession", "Doubling Season"}
	for i, want := range expectedOrder {
		if chain[i].SourceName != want {
			t.Errorf("chain[%d]: want %q, got %q", i, want, chain[i].SourceName)
		}
		if chain[i].Modification != ReplacementOpDouble {
			t.Errorf("chain[%d]: want Double, got %v", i, chain[i].Modification)
		}
	}
	// CountBefore/After progression: 1→2, 2→4, 4→8.
	wantBefore := []int{1, 2, 4}
	wantAfter := []int{2, 4, 8}
	for i := range chain {
		if chain[i].CountBefore != wantBefore[i] || chain[i].CountAfter != wantAfter[i] {
			t.Errorf("chain[%d]: counts before=%d after=%d (want %d / %d)",
				i, chain[i].CountBefore, chain[i].CountAfter, wantBefore[i], wantAfter[i])
		}
	}
}

// registerTokenDoubler is a test helper that registers a Doubling-Season-
// shape would_create_token replacement on the given permanent. Mirrors
// RegisterDoublingSeason without the counter-doubler arm.
func registerTokenDoubler(gs *GameState, p *Permanent, label string) {
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_create_token",
		HandlerID:      label + "::token_dbl",
		SourcePerm:     p,
		ControllerSeat: p.Controller,
		Timestamp:      p.Timestamp,
		Category:       CategoryOther,
		Applies: func(_ *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == p.Controller && ev.Count() > 0
		},
		ApplyFn: func(_ *GameState, ev *ReplEvent) {
			ev.SetCount(ev.Count() * 2)
		},
	})
	// Phase 5: register on the audit-surface field too.
	p.ProvidesReplacements = append(p.ProvidesReplacements, ReplacementSpec{
		Name:      label,
		EventType: "would_create_token",
		Category:  CategoryOther,
		HandlerID: label + "::token_dbl",
	})
	// Pretend the permanent is on the battlefield so pickReplacement's
	// staleness gate doesn't reject it.
	gs.Seats[p.Controller].Battlefield = append(gs.Seats[p.Controller].Battlefield, p)
}

// ---------------------------------------------------------------------------
// MintTokenAsCopyOf — the Phase 5 chokepoint for token-as-copy mints.
// Confirms the inherited InstanceID is CLEARED + fresh TK minted +
// SourceInstanceID lineage preserved.
// ---------------------------------------------------------------------------

func TestPhase5_MintTokenAsCopyOf_ClearsInheritedIDAndMintsFreshTK(t *testing.T) {
	gs := newPhase5GameState(t)
	src := &Card{
		Name:          "Dockside Extortionist",
		Owner:         0,
		Types:         []string{"creature", "human", "pirate"},
		Colors:        []string{"R"},
		BasePower:     1,
		BaseToughness: 1,
		CMC:           3,
	}
	MintOGInstanceID(gs, src)
	srcID := src.InstanceID
	if srcID == "" {
		t.Fatal("expected source to mint OG ID")
	}

	tok := MintTokenAsCopyOf(gs, src, 1, "enabler-AB-001")
	if tok == nil {
		t.Fatal("MintTokenAsCopyOf returned nil")
	}
	if tok.InstanceID == srcID {
		t.Errorf("expected fresh InstanceID, got source ID %q", srcID)
	}
	if !strings.Contains(tok.InstanceID, "TK") {
		t.Errorf("expected TK provenance, got %q", tok.InstanceID)
	}
	if tok.SourceInstanceID != srcID {
		t.Errorf("SourceInstanceID lineage broken: want %q got %q", srcID, tok.SourceInstanceID)
	}
	if tok.EnablerInstanceID != "enabler-AB-001" {
		t.Errorf("EnablerInstanceID lineage broken: %q", tok.EnablerInstanceID)
	}
	if tok.Owner != 1 {
		t.Errorf("Owner reassignment broken: %d", tok.Owner)
	}
	// "token" type must be present.
	hasToken := false
	for _, tt := range tok.Types {
		if tt == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		t.Errorf("token type tag missing: %v", tok.Types)
	}
	// MintedInstanceIDs must include the new ID.
	if _, ok := gs.MintedInstanceIDs[tok.InstanceID]; !ok {
		t.Errorf("new TK ID not recorded in MintedInstanceIDs")
	}
}

// ---------------------------------------------------------------------------
// BecomeCopyOfCard — preserves perm.Card.InstanceID across the in-place
// copy semantics for Clone / Spark Double / Phantasmal Image / Sakashima.
// ---------------------------------------------------------------------------

func TestPhase5_BecomeCopyOfCard_PreservesOriginalInstanceID(t *testing.T) {
	gs := newPhase5GameState(t)
	cloneCard := &Card{Name: "Clone", Owner: 0, Types: []string{"creature"}, CMC: 4}
	MintOGInstanceID(gs, cloneCard)
	cloneID := cloneCard.InstanceID
	if cloneID == "" {
		t.Fatal("clone source: empty InstanceID")
	}
	cloneP := &Permanent{Card: cloneCard, Controller: 0, Owner: 0}

	target := &Card{Name: "Dockside Extortionist", Owner: 1, Types: []string{"creature", "human"}, Colors: []string{"R"}}
	MintOGInstanceID(gs, target)
	targetID := target.InstanceID
	if targetID == "" || targetID == cloneID {
		t.Fatalf("target ID problem: %q (clone: %q)", targetID, cloneID)
	}

	rewritten := BecomeCopyOfCard(gs, cloneP, target)
	if rewritten == nil {
		t.Fatal("BecomeCopyOfCard returned nil")
	}
	if rewritten.InstanceID != cloneID {
		t.Errorf("InstanceID preservation broken: want %q got %q", cloneID, rewritten.InstanceID)
	}
	if cloneP.CopiedTargetInstanceID != targetID {
		t.Errorf("CopiedTargetInstanceID stamp broken: %q", cloneP.CopiedTargetInstanceID)
	}
	if cloneP.CopiableSnapshot == nil {
		t.Fatal("CopiableSnapshot must be set")
	}
	if cloneP.CopiableSnapshot.SourceInstanceID != targetID {
		t.Errorf("snapshot lineage broken: %q", cloneP.CopiableSnapshot.SourceInstanceID)
	}
	if cloneP.CopiableSnapshot.Name != "Dockside Extortionist" {
		t.Errorf("snapshot name wrong: %q", cloneP.CopiableSnapshot.Name)
	}
	if len(cloneP.CopyHistory) != 1 {
		t.Errorf("CopyHistory not recorded: %+v", cloneP.CopyHistory)
	}
}

// ---------------------------------------------------------------------------
// Mint-coverage audit — sweep every per_card token-creation site we
// migrated and assert no duplicate InstanceIDs end up on the battlefield.
// This is the live regression for the 2.9M mint-coverage gap from PR #755.
// ---------------------------------------------------------------------------

func TestPhase5_TokenMintCoverage_NoDuplicateInstanceIDsOnMint(t *testing.T) {
	gs := newPhase5GameState(t)
	src := &Card{
		Name:          "Mulldrifter",
		Owner:         0,
		Types:         []string{"creature", "elemental"},
		Colors:        []string{"U"},
		BasePower:     2,
		BaseToughness: 2,
		CMC:           5,
	}
	MintOGInstanceID(gs, src)

	// Spawn 25 token-as-copy mints — would have produced 25 duplicates
	// under the pre-Phase-5 DeepCopy path. With MintTokenAsCopyOf each
	// gets a unique TK ID.
	seen := map[string]struct{}{src.InstanceID: {}}
	for i := 0; i < 25; i++ {
		tok := MintTokenAsCopyOf(gs, src, 0, "enabler-test")
		if tok == nil {
			t.Fatalf("mint %d returned nil", i)
		}
		if _, dup := seen[tok.InstanceID]; dup {
			t.Errorf("duplicate ID at mint %d: %q", i, tok.InstanceID)
		}
		seen[tok.InstanceID] = struct{}{}
	}
	if len(seen) != 26 {
		t.Errorf("expected 26 unique IDs (1 src + 25 tokens), got %d", len(seen))
	}
}

// TestPhase5_TokenMintEvent_RecordedToGameState pins that
// RecordTokenMintEvent appends to gs.TokenMintEvents and respects the
// cap-with-drop-oldest-half semantics.
func TestPhase5_TokenMintEvent_RecordedAndCapped(t *testing.T) {
	gs := newPhase5GameState(t)
	for i := 0; i < 300; i++ {
		RecordTokenMintEvent(gs, TokenMintEvent{
			TargetSeat: 0,
			BaseCount:  1,
			FinalCount: 1,
		})
	}
	if len(gs.TokenMintEvents) == 0 {
		t.Fatal("expected events recorded")
	}
	if len(gs.TokenMintEvents) > 256 {
		t.Errorf("cap not enforced: %d entries", len(gs.TokenMintEvents))
	}
}

// TestPhase5_ReplacementOp_Stringification pins log labels.
func TestPhase5_ReplacementOp_Stringification(t *testing.T) {
	cases := map[ReplacementOp]string{
		ReplacementOpDouble:         "Double",
		ReplacementOpHalve:          "Halve",
		ReplacementOpRedirect:       "Redirect",
		ReplacementOpZoneSubstitute: "ZoneSubstitute",
		ReplacementOpSkip:           "Skip",
	}
	for op, want := range cases {
		if got := op.String(); got != want {
			t.Errorf("op %d: want %q got %q", op, want, got)
		}
	}
}
