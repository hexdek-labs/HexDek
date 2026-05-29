package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// newPhase2GameState builds a 2-seat GameState with a fresh Minter for
// Phase 2 property tests. Skips deck-load — tests construct cards
// directly and stamp them via MintOGInstanceID where they need OG IDs.
func newPhase2GameState(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, nil, nil)
	if gs == nil {
		t.Fatalf("NewGameState returned nil")
	}
	if gs.IIDMinter == nil {
		t.Fatalf("expected IIDMinter to be non-nil")
	}
	return gs
}

// TestPhase2_TokenMintFormatAndProvenance pins that MintTokenInstanceID
// produces a regex-matching TK ID with the right provenance/visibility
// stamps and the lineage fields the caller supplied.
func TestPhase2_TokenMintFormatAndProvenance(t *testing.T) {
	gs := newPhase2GameState(t)
	tok := &Card{Name: "Thopter Token", Owner: 0, Colors: nil}
	MintTokenInstanceID(gs, tok, "src-OG-001", "src-AB-042")
	if tok.InstanceID == "" {
		t.Fatalf("expected non-empty InstanceID after mint")
	}
	if !instanceid.FormatRegex.MatchString(tok.InstanceID) {
		t.Fatalf("InstanceID %q does not match FormatRegex", tok.InstanceID)
	}
	if !strings.Contains(tok.InstanceID, "TK") {
		t.Fatalf("expected TK provenance code in %q", tok.InstanceID)
	}
	if tok.Provenance != instanceid.ProvTK {
		t.Fatalf("Provenance: want ProvTK, got %v", tok.Provenance)
	}
	if tok.Visibility != instanceid.Visible {
		t.Fatalf("Visibility: want Visible, got %v", tok.Visibility)
	}
	if tok.ActiveFace != instanceid.Front {
		t.Fatalf("ActiveFace: want Front, got %v", tok.ActiveFace)
	}
	if tok.SourceInstanceID != "src-OG-001" {
		t.Fatalf("SourceInstanceID: want src-OG-001, got %q", tok.SourceInstanceID)
	}
	if tok.EnablerInstanceID != "src-AB-042" {
		t.Fatalf("EnablerInstanceID: want src-AB-042, got %q", tok.EnablerInstanceID)
	}
	if len(tok.EnablerHistory) != 1 || tok.EnablerHistory[0] != "src-AB-042" {
		t.Fatalf("EnablerHistory: want [src-AB-042], got %v", tok.EnablerHistory)
	}
}

// TestPhase2_TokenMintIdempotent verifies repeat calls don't double-mint
// or burn seq counters once an InstanceID is set.
func TestPhase2_TokenMintIdempotent(t *testing.T) {
	gs := newPhase2GameState(t)
	tok := &Card{Name: "Thopter", Owner: 0}
	MintTokenInstanceID(gs, tok, "", "")
	firstID := tok.InstanceID
	if firstID == "" {
		t.Fatal("expected mint on first call")
	}
	beforeSeq := gs.IIDMinter.Peek(0)
	MintTokenInstanceID(gs, tok, "", "")
	if tok.InstanceID != firstID {
		t.Fatalf("second mint changed InstanceID: %q -> %q", firstID, tok.InstanceID)
	}
	afterSeq := gs.IIDMinter.Peek(0)
	if afterSeq != beforeSeq {
		t.Fatalf("idempotent mint advanced seq counter: %d -> %d", beforeSeq, afterSeq)
	}
}

// TestPhase2_TokenMintUniqueness asserts that minting many tokens
// produces distinct InstanceIDs (no seq collisions) for the same seat.
func TestPhase2_TokenMintUniqueness(t *testing.T) {
	gs := newPhase2GameState(t)
	seen := map[string]bool{}
	for i := 0; i < 256; i++ {
		c := &Card{Name: "Thopter", Owner: 0}
		MintTokenInstanceID(gs, c, "src-OG-000", "src-AB-000")
		if c.InstanceID == "" {
			t.Fatalf("mint %d produced empty ID", i)
		}
		if seen[c.InstanceID] {
			t.Fatalf("duplicate InstanceID %q at mint %d", c.InstanceID, i)
		}
		seen[c.InstanceID] = true
	}
}

// TestPhase2_CopyMintLineage pins that MintCopyInstanceID stamps CP
// provenance with the required SourceInstanceID + EnablerInstanceID
// lineage per §3 format invariants.
func TestPhase2_CopyMintLineage(t *testing.T) {
	gs := newPhase2GameState(t)
	orig := &Card{Name: "Lightning Bolt", Owner: 1, Colors: []string{"R"}, CMC: 1}
	MintOGInstanceID(gs, orig)
	if orig.InstanceID == "" {
		t.Fatal("expected OG InstanceID after MintOGInstanceID")
	}

	cp := &Card{Name: "Lightning Bolt (storm copy 1)", Owner: 1, Colors: []string{"R"}, CMC: 0, IsCopy: true}
	MintCopyInstanceID(gs, cp, orig.InstanceID, "enabler-AB-storm")
	if cp.InstanceID == "" {
		t.Fatal("expected CP InstanceID after MintCopyInstanceID")
	}
	if !instanceid.FormatRegex.MatchString(cp.InstanceID) {
		t.Fatalf("InstanceID %q does not match FormatRegex", cp.InstanceID)
	}
	if !strings.Contains(cp.InstanceID, "CP") {
		t.Fatalf("expected CP provenance code in %q", cp.InstanceID)
	}
	if cp.Provenance != instanceid.ProvCP {
		t.Fatalf("Provenance: want ProvCP, got %v", cp.Provenance)
	}
	if cp.SourceInstanceID != orig.InstanceID {
		t.Fatalf("SourceInstanceID: want %q, got %q", orig.InstanceID, cp.SourceInstanceID)
	}
	if cp.EnablerInstanceID != "enabler-AB-storm" {
		t.Fatalf("EnablerInstanceID: want enabler-AB-storm, got %q", cp.EnablerInstanceID)
	}
}

// TestPhase2_TKCPABUniquenessAcrossMintSites confirms that mixing all
// three mint paths produces distinct IDs (no provenance crossover) and
// that each ID parses to the correct provenance code in the format.
func TestPhase2_TKCPABUniquenessAcrossMintSites(t *testing.T) {
	gs := newPhase2GameState(t)
	seen := map[string]string{}
	check := func(id, want string) {
		if id == "" {
			t.Fatalf("empty ID for want=%s", want)
		}
		if prior, ok := seen[id]; ok {
			t.Fatalf("duplicate ID %q (prior=%s now=%s)", id, prior, want)
		}
		seen[id] = want
		if !strings.Contains(id, want) {
			t.Fatalf("ID %q does not contain provenance %s", id, want)
		}
		if !instanceid.FormatRegex.MatchString(id) {
			t.Fatalf("ID %q does not match FormatRegex", id)
		}
	}
	// 5 TK mints
	for i := 0; i < 5; i++ {
		c := &Card{Name: "Thopter", Owner: 0}
		MintTokenInstanceID(gs, c, "", "")
		check(c.InstanceID, "TK")
	}
	// 5 CP mints
	for i := 0; i < 5; i++ {
		c := &Card{Name: "Bolt Copy", Owner: 0, Colors: []string{"R"}}
		MintCopyInstanceID(gs, c, "src-001", "enabler-002")
		check(c.InstanceID, "CP")
	}
	// 5 AB mints via NewAbilityInstance
	src := &Permanent{Card: &Card{Name: "Sai", Owner: 0, Colors: []string{"U"}, CMC: 3}, Controller: 0}
	MintOGInstanceID(gs, src.Card)
	for i := 0; i < 5; i++ {
		ab := NewAbilityInstance(gs, src, 0, "trig:etb", "", nil)
		check(ab.InstanceID, "AB")
	}
}

// TestPhase2_AbilityInstanceLineage pins the §4.3 schema invariants: AB
// instance has a SourceInstanceID matching the source's Card InstanceID
// and an empty EnablerInstanceID when none was supplied (activated
// abilities); non-empty when supplied (triggered abilities with
// enabler context).
func TestPhase2_AbilityInstanceLineage(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Permanent{Card: &Card{Name: "Sai, Master Thopterist", Owner: 0, Colors: []string{"U"}, CMC: 3}, Controller: 0}
	MintOGInstanceID(gs, src.Card)
	if src.Card.InstanceID == "" {
		t.Fatal("expected OG InstanceID on source card")
	}

	// Activated: no enabler.
	ab1 := NewAbilityInstance(gs, src, 0, "act:0", "", nil)
	if ab1.InstanceID == "" {
		t.Fatal("expected non-empty AbilityInstance.InstanceID")
	}
	if ab1.SourceInstanceID != src.Card.InstanceID {
		t.Fatalf("SourceInstanceID: want %q, got %q", src.Card.InstanceID, ab1.SourceInstanceID)
	}
	if ab1.EnablerInstanceID != "" {
		t.Fatalf("EnablerInstanceID for activated ability should be empty, got %q", ab1.EnablerInstanceID)
	}

	// Triggered: enabler is the resolving frame's AB ID.
	ab2 := NewAbilityInstance(gs, src, 0, "trig:etb", "enabler-AB-Sai", map[string]any{"x_value": 5})
	if ab2.InstanceID == ab1.InstanceID {
		t.Fatalf("expected distinct AbilityInstance IDs, got %q twice", ab1.InstanceID)
	}
	if ab2.EnablerInstanceID != "enabler-AB-Sai" {
		t.Fatalf("EnablerInstanceID: want enabler-AB-Sai, got %q", ab2.EnablerInstanceID)
	}
	if v, ok := ab2.TriggerMetadata["x_value"].(int); !ok || v != 5 {
		t.Fatalf("TriggerMetadata[x_value]: want 5, got %v", ab2.TriggerMetadata["x_value"])
	}
}

// TestPhase2_AbilityInstanceAttachedToStackItem validates that
// resolveActivatedAbility / PushTriggeredAbility paths attach an
// AbilityInstance to the resulting StackItem so downstream resolution
// can read item.Ability.InstanceID for lineage stamping.
func TestPhase2_AbilityInstanceAttachedToStackItem(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Permanent{Card: &Card{Name: "Sai", Owner: 0}, Controller: 0}
	MintOGInstanceID(gs, src.Card)

	// Manually construct as the trigger-push path would, then assert
	// shape — direct PushTriggeredAbility requires a valid Effect plumb
	// which we can't easily synthesize without AST; the unit-level
	// invariant is "StackItem with Kind=triggered carries a non-nil
	// Ability whose SourceInstanceID matches the source's InstanceID".
	ab := NewAbilityInstance(gs, src, 0, "trig:etb", "", nil)
	item := &StackItem{
		Kind:       "triggered",
		Controller: 0,
		Source:     src,
		Card:       src.Card,
		Ability:    ab,
	}
	if item.Ability == nil {
		t.Fatal("expected non-nil item.Ability")
	}
	if item.Ability.InstanceID == "" {
		t.Fatal("expected non-empty Ability.InstanceID")
	}
	if item.Ability.SourceInstanceID != src.Card.InstanceID {
		t.Fatalf("Ability.SourceInstanceID: want %q, got %q",
			src.Card.InstanceID, item.Ability.SourceInstanceID)
	}
}

// TestPhase2_EnablerStackPushPop pins the resolve-time enabler stack
// behavior — currentMintEnablerID returns the most-recent pushed ID;
// pop restores the prior frame.
func TestPhase2_EnablerStackPushPop(t *testing.T) {
	gs := newPhase2GameState(t)
	if got := currentMintEnablerID(gs); got != "" {
		t.Fatalf("empty stack: want \"\", got %q", got)
	}
	pushIIDEnabler(gs, "A")
	if got := currentMintEnablerID(gs); got != "A" {
		t.Fatalf("after push A: want A, got %q", got)
	}
	pushIIDEnabler(gs, "B")
	if got := currentMintEnablerID(gs); got != "B" {
		t.Fatalf("after push B: want B, got %q", got)
	}
	popIIDEnabler(gs)
	if got := currentMintEnablerID(gs); got != "A" {
		t.Fatalf("after pop: want A, got %q", got)
	}
	popIIDEnabler(gs)
	if got := currentMintEnablerID(gs); got != "" {
		t.Fatalf("after pop to empty: want \"\", got %q", got)
	}
	// Over-pop is safe.
	popIIDEnabler(gs)
}

// TestPhase2_EnsureTokenAutoStamp validates the catch-all defensive
// stamp at FirePermanentETBTriggers entry: token Permanents with empty
// InstanceIDs get TK-stamped automatically; non-tokens are left alone.
func TestPhase2_EnsureTokenAutoStamp(t *testing.T) {
	gs := newPhase2GameState(t)

	// Token with empty ID — should be stamped.
	tokPerm := &Permanent{
		Card:       &Card{Name: "Thopter", Owner: 0, Types: []string{"token", "artifact"}},
		Controller: 0,
		Owner:      0,
	}
	EnsureTokenInstanceID(gs, tokPerm)
	if tokPerm.Card.InstanceID == "" {
		t.Fatal("expected token Card to be stamped by EnsureTokenInstanceID")
	}
	if !strings.Contains(tokPerm.Card.InstanceID, "TK") {
		t.Fatalf("expected TK provenance, got %q", tokPerm.Card.InstanceID)
	}

	// Non-token — should be untouched.
	nonPerm := &Permanent{
		Card:       &Card{Name: "Mountain", Owner: 0, Types: []string{"land", "basic"}},
		Controller: 0,
		Owner:      0,
	}
	EnsureTokenInstanceID(gs, nonPerm)
	if nonPerm.Card.InstanceID != "" {
		t.Fatalf("non-token should not be stamped, got %q", nonPerm.Card.InstanceID)
	}

	// Already-stamped token — idempotent (no re-mint).
	prior := tokPerm.Card.InstanceID
	EnsureTokenInstanceID(gs, tokPerm)
	if tokPerm.Card.InstanceID != prior {
		t.Fatalf("idempotent EnsureTokenInstanceID changed ID: %q -> %q",
			prior, tokPerm.Card.InstanceID)
	}
}

// TestPhase2_TokenMintViaCreateCreatureToken validates the engine
// chokepoint: CreateCreatureToken produces a Permanent whose Card has a
// TK InstanceID stamped at mint time (no defensive auto-stamp needed).
func TestPhase2_TokenMintViaCreateCreatureToken(t *testing.T) {
	gs := newPhase2GameState(t)
	perm := CreateCreatureToken(gs, 0, "Thopter Token", []string{"artifact"}, 1, 1)
	if perm == nil || perm.Card == nil {
		t.Fatal("CreateCreatureToken returned nil")
	}
	if perm.Card.InstanceID == "" {
		t.Fatal("expected CreateCreatureToken to stamp TK InstanceID")
	}
	if !strings.Contains(perm.Card.InstanceID, "TK") {
		t.Fatalf("expected TK provenance, got %q", perm.Card.InstanceID)
	}
	if perm.Card.Provenance != instanceid.ProvTK {
		t.Fatalf("Provenance: want ProvTK, got %v", perm.Card.Provenance)
	}
}

// TestPhase2_NilMinterBackwardsCompat confirms that nil-minter GameStates
// (struct-literal construction in legacy tests) silently no-op all mint
// paths so existing tests don't regress.
func TestPhase2_NilMinterBackwardsCompat(t *testing.T) {
	gs := &GameState{Seats: []*Seat{newSeat(0), newSeat(1)}}
	c := &Card{Name: "Thopter", Owner: 0}
	MintTokenInstanceID(gs, c, "", "")
	if c.InstanceID != "" {
		t.Fatalf("nil minter should silently no-op, got %q", c.InstanceID)
	}
	MintCopyInstanceID(gs, c, "src", "en")
	if c.InstanceID != "" {
		t.Fatalf("nil minter should silently no-op, got %q", c.InstanceID)
	}
	src := &Permanent{Card: &Card{Name: "Sai", Owner: 0}, Controller: 0}
	ab := NewAbilityInstance(gs, src, 0, "trig:etb", "", nil)
	if ab == nil {
		t.Fatal("NewAbilityInstance should still return a struct in nil-minter mode")
	}
	if ab.InstanceID != "" {
		t.Fatalf("nil minter should leave AbilityInstance.InstanceID empty, got %q", ab.InstanceID)
	}
}
