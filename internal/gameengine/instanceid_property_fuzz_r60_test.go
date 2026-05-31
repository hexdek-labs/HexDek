package gameengine

// InstanceID property fuzz — r60 (docs/instanceid-property-fuzz-r60.md).
//
// Bypasses the engine entirely. Drives the raw mint / move / cease /
// merge primitives in random sequences and asserts the 5 InstanceID
// invariants after every step. Catches lifecycle bugs the engine-based
// phase tests miss because real games don't probe the primitive space
// densely enough.

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	gameast "github.com/hexdek/hexdek/internal/gameast"
)

// stringsIndex is a thin alias to strings.Index — keeping it private here
// so failure-dump code can be relocated as a single unit.
func stringsIndex(s, sub string) int { return strings.Index(s, sub) }

// ----------------------------------------------------------------------
// fuzzOp — discriminated union over the operation space
// ----------------------------------------------------------------------

type fuzzOp struct {
	Kind string // mint_og / mint_token / mint_token_copy / mint_spell_copy /
	//        move / exile_with_linkage / cease_battlefield_leave /
	//        cease_explicit / mutate_merge / meld_merge

	Seat int // target seat for mint or move

	// Card selection by index into st.indexed.
	CardIdx int

	// Move op fields.
	FromZone string
	ToZone   string

	// Source InstanceID for token-copy / spell-copy ops.
	SourceID string

	// Mutate / Meld op fields — both reference indices into
	// st.battlefieldPerms.
	PermIdxA int
	PermIdxB int

	// MintAttrs for mint_og / mint_token: synthesized name + cmc + color
	// from the rng so fabrications don't collide.
	NameTag string
}

func (op fuzzOp) String() string {
	switch op.Kind {
	case "mint_og", "mint_token":
		return fmt.Sprintf("%s seat=%d name=%s", op.Kind, op.Seat, op.NameTag)
	case "mint_token_copy", "mint_spell_copy":
		return fmt.Sprintf("%s seat=%d src=%s", op.Kind, op.Seat, op.SourceID)
	case "move":
		return fmt.Sprintf("move card_idx=%d %s -> %s", op.CardIdx, op.FromZone, op.ToZone)
	case "exile_with_linkage":
		return fmt.Sprintf("exile_with_linkage card_idx=%d perm_idx=%d", op.CardIdx, op.PermIdxA)
	case "cease_battlefield_leave":
		return fmt.Sprintf("cease_bf_leave perm_idx=%d", op.PermIdxA)
	case "cease_explicit":
		return fmt.Sprintf("cease_explicit card_idx=%d", op.CardIdx)
	case "mutate_merge":
		return fmt.Sprintf("mutate_merge surv=%d dying=%d", op.PermIdxA, op.PermIdxB)
	case "meld_merge":
		return fmt.Sprintf("meld_merge perm_a=%d perm_b=%d", op.PermIdxA, op.PermIdxB)
	}
	return op.Kind
}

// ----------------------------------------------------------------------
// fuzzState — wraps *GameState plus indexes for cheap random selection
// ----------------------------------------------------------------------

type fuzzState struct {
	gs *GameState

	// indexed holds every *Card that has ever been minted (OG / TK / CP),
	// in mint order. Lets generateOp pick "the Nth known card" by index.
	indexed []*Card

	// Per-card current zone tracker — we maintain this in sync with our
	// own moves so generators can look up where a card lives without
	// scanning every zone. Keyed by *Card pointer.
	cardZone map[*Card]string // "library" / "hand" / "graveyard" / "exile" / "battlefield" / "ceased"

	// cardPerm tracks the *Permanent wrapping a *Card while on the
	// battlefield. Cleared when the card leaves the battlefield.
	cardPerm map[*Card]*Permanent
}

func newFuzzState(rng *rand.Rand) *fuzzState {
	gs := NewGameState(4, rng, nil)
	// Strict-census mode is the whole point of the disappearance check.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1
	return &fuzzState{
		gs:       gs,
		cardZone: map[*Card]string{},
		cardPerm: map[*Card]*Permanent{},
	}
}

// indexCard registers a freshly-minted card so future ops can pick it
// by random index. Refuses duplicates (would inflate selection weight).
func (st *fuzzState) indexCard(c *Card, initialZone string) {
	if c == nil || c.InstanceID == "" {
		return
	}
	if _, dup := st.cardZone[c]; dup {
		return
	}
	st.indexed = append(st.indexed, c)
	st.cardZone[c] = initialZone
}

// findPermFor scans seat battlefields for the Permanent wrapping c.
// Returns (nil, -1) if absent.
func (st *fuzzState) findPermFor(c *Card) (*Permanent, int) {
	if c == nil {
		return nil, -1
	}
	for seatIdx, seat := range st.gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Card == c {
				return p, seatIdx
			}
		}
	}
	return nil, -1
}

// collectBattlefieldPerms returns every Permanent currently on any
// seat's battlefield. Used by mutate/meld generators.
func (st *fuzzState) collectBattlefieldPerms() []*Permanent {
	var out []*Permanent
	for _, seat := range st.gs.Seats {
		if seat == nil {
			continue
		}
		out = append(out, seat.Battlefield...)
	}
	return out
}

// fuzzRemoveCard scrubs c out of the named zone slice. Returns true
// when the card was found and removed.
func fuzzRemoveCard(slice []*Card, c *Card) ([]*Card, bool) {
	for i, x := range slice {
		if x == c {
			return append(slice[:i], slice[i+1:]...), true
		}
	}
	return slice, false
}

// fuzzUnmergePerm dissolves any merge state on perm. Constituents
// (cards in MergedCardPtrs other than the surviving card) are ceased
// when they're tokens, otherwise routed to their owner's graveyard.
// Mirrors UnmergeOnLeavePlay's semantics but uses the fuzz tracker.
// Safe to call on perms with no merge state (no-op).
func fuzzUnmergePerm(gs *GameState, st *fuzzState, perm *Permanent, survCard *Card) {
	if perm == nil || perm.MergeKind == MergeNone || len(perm.MergedCards) == 0 {
		return
	}
	mergedIDs := append([]string(nil), perm.MergedCards...)
	mergedPtrs := map[string]*Card{}
	for k, v := range perm.MergedCardPtrs {
		mergedPtrs[k] = v
	}
	for _, mid := range mergedIDs {
		if survCard != nil && mid == survCard.InstanceID {
			continue // surviving perm's own card handled by the LTB path
		}
		mc := mergedPtrs[mid]
		if mc == nil {
			continue
		}
		mIsToken := false
		for _, ty := range mc.Types {
			if ty == "token" {
				mIsToken = true
				break
			}
		}
		if mIsToken && len(mc.InstanceID) >= 4 && mc.InstanceID[2:4] == "TK" {
			MarkInstanceIDCeased(gs, mc.InstanceID)
			st.cardZone[mc] = "ceased"
			continue
		}
		owner := mc.Owner
		if owner < 0 || owner >= len(gs.Seats) {
			continue
		}
		gs.Seats[owner].Graveyard = append(gs.Seats[owner].Graveyard, mc)
		st.cardZone[mc] = "graveyard"
	}
	perm.MergedCards = nil
	perm.MergedCardPtrs = nil
	perm.MergeKind = MergeNone
}

// scrubExiledByMe removes any ExiledByMe entry matching id from every
// permanent on every battlefield. Mirrors the real-engine semantics
// where moving an exiled card out of exile (LTB return / blink return /
// cease) resolves the source-held linkage.
func scrubExiledByMe(gs *GameState, id string) {
	if id == "" {
		return
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || len(p.ExiledByMe) == 0 {
				continue
			}
			out := p.ExiledByMe[:0]
			for _, ex := range p.ExiledByMe {
				if ex != id {
					out = append(out, ex)
				}
			}
			p.ExiledByMe = out
		}
	}
}

// fuzzRemovePerm scrubs p out of seat's battlefield slice.
func fuzzRemovePerm(slice []*Permanent, p *Permanent) ([]*Permanent, bool) {
	for i, x := range slice {
		if x == p {
			return append(slice[:i], slice[i+1:]...), true
		}
	}
	return slice, false
}

// ----------------------------------------------------------------------
// generateOp — picks a weighted-random fuzz op
// ----------------------------------------------------------------------

// opKindWeights — relative weights per op kind. Roll a random int in
// [0, sum) and walk the table.
var opKindWeights = []struct {
	Kind   string
	Weight int
}{
	{"mint_og", 30},
	{"mint_token", 15},
	{"mint_token_copy", 15},
	{"mint_spell_copy", 10},
	{"move", 20},
	{"exile_with_linkage", 5},
	{"cease_battlefield_leave", 2},
	{"cease_explicit", 1},
	{"mutate_merge", 1},
	{"meld_merge", 1},
}

var opKindWeightSum = func() int {
	s := 0
	for _, e := range opKindWeights {
		s += e.Weight
	}
	return s
}()

// pickOpKind rolls a weighted-random op kind from opKindWeights.
func pickOpKind(rng *rand.Rand) string {
	r := rng.Intn(opKindWeightSum)
	for _, e := range opKindWeights {
		if r < e.Weight {
			return e.Kind
		}
		r -= e.Weight
	}
	return "mint_og" // unreachable
}

// pickCardWithFilter rolls a random card index satisfying pred. Returns
// (-1, nil) when no such card exists.
func (st *fuzzState) pickCardWithFilter(rng *rand.Rand, pred func(*Card) bool) (int, *Card) {
	if len(st.indexed) == 0 {
		return -1, nil
	}
	// Try up to 8 random picks first (cheap path), then linear scan as
	// a fallback so we always find one if any qualifies.
	for tries := 0; tries < 8; tries++ {
		i := rng.Intn(len(st.indexed))
		c := st.indexed[i]
		if pred(c) {
			return i, c
		}
	}
	// Linear scan: collect all matches, pick one.
	var matches []int
	for i, c := range st.indexed {
		if pred(c) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return -1, nil
	}
	idx := matches[rng.Intn(len(matches))]
	return idx, st.indexed[idx]
}

// generateOp picks the next op for the current fuzz state. Skips ops
// whose preconditions aren't met by retrying with a fallback kind.
func generateOp(rng *rand.Rand, st *fuzzState, opIdx int) fuzzOp {
	for tries := 0; tries < 8; tries++ {
		kind := pickOpKind(rng)
		op := tryBuildOp(rng, st, kind, opIdx)
		if op.Kind != "" {
			return op
		}
	}
	// Fallback — always succeeds since mint_og has no precondition.
	return tryBuildOp(rng, st, "mint_og", opIdx)
}

func tryBuildOp(rng *rand.Rand, st *fuzzState, kind string, opIdx int) fuzzOp {
	switch kind {
	case "mint_og":
		return fuzzOp{
			Kind:    "mint_og",
			Seat:    rng.Intn(len(st.gs.Seats)),
			NameTag: fmt.Sprintf("Fuzz_OG_%d", opIdx),
		}
	case "mint_token":
		return fuzzOp{
			Kind:    "mint_token",
			Seat:    rng.Intn(len(st.gs.Seats)),
			NameTag: fmt.Sprintf("Fuzz_TK_%d", opIdx),
		}
	case "mint_token_copy", "mint_spell_copy":
		// Need a source card with a non-empty InstanceID.
		i, src := st.pickCardWithFilter(rng, func(c *Card) bool {
			return c != nil && c.InstanceID != "" && st.cardZone[c] != "ceased"
		})
		if i < 0 {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:     kind,
			Seat:     rng.Intn(len(st.gs.Seats)),
			SourceID: src.InstanceID,
			CardIdx:  i,
			NameTag:  fmt.Sprintf("Fuzz_COPY_%d", opIdx),
		}
	case "move":
		// Need a card that's NOT on battlefield (battlefield moves require
		// wrapping in a Permanent, which we handle via separate ops).
		i, c := st.pickCardWithFilter(rng, func(c *Card) bool {
			z := st.cardZone[c]
			return z != "" && z != "ceased" && z != "battlefield"
		})
		if i < 0 {
			return fuzzOp{}
		}
		from := st.cardZone[c]
		// Pick a different destination from the non-battlefield set.
		dests := []string{"library", "hand", "graveyard", "exile", "battlefield"}
		to := dests[rng.Intn(len(dests))]
		if to == from {
			// nudge to next
			to = dests[(rng.Intn(len(dests))+1)%len(dests)]
		}
		return fuzzOp{
			Kind:     "move",
			CardIdx:  i,
			FromZone: from,
			ToZone:   to,
		}
	case "exile_with_linkage":
		// Need a battlefield permanent (the exiler) AND a non-bf card to exile.
		perms := st.collectBattlefieldPerms()
		if len(perms) == 0 {
			return fuzzOp{}
		}
		permIdx := rng.Intn(len(perms))
		i, _ := st.pickCardWithFilter(rng, func(c *Card) bool {
			z := st.cardZone[c]
			return z == "library" || z == "hand" || z == "graveyard"
		})
		if i < 0 {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:     "exile_with_linkage",
			CardIdx:  i,
			PermIdxA: permIdx,
		}
	case "cease_battlefield_leave":
		perms := st.collectBattlefieldPerms()
		if len(perms) == 0 {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:     "cease_battlefield_leave",
			PermIdxA: rng.Intn(len(perms)),
		}
	case "cease_explicit":
		i, _ := st.pickCardWithFilter(rng, func(c *Card) bool {
			return c != nil && c.InstanceID != "" && st.cardZone[c] != "ceased"
		})
		if i < 0 {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:    "cease_explicit",
			CardIdx: i,
		}
	case "mutate_merge":
		perms := st.collectBattlefieldPerms()
		if len(perms) < 2 {
			return fuzzOp{}
		}
		a := rng.Intn(len(perms))
		b := rng.Intn(len(perms))
		if a == b {
			b = (b + 1) % len(perms)
		}
		// Surviving and dying perms must be different *Permanent objects
		// AND not already related by merge state (avoid weird re-merge).
		if perms[a] == perms[b] {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:     "mutate_merge",
			PermIdxA: a,
			PermIdxB: b,
		}
	case "meld_merge":
		perms := st.collectBattlefieldPerms()
		if len(perms) < 2 {
			return fuzzOp{}
		}
		a := rng.Intn(len(perms))
		b := rng.Intn(len(perms))
		if a == b {
			b = (b + 1) % len(perms)
		}
		if perms[a] == perms[b] {
			return fuzzOp{}
		}
		// meld_merge requires fresh perms (not already merged) to keep
		// the model simple.
		if perms[a].MergeKind != MergeNone || perms[b].MergeKind != MergeNone {
			return fuzzOp{}
		}
		return fuzzOp{
			Kind:     "meld_merge",
			PermIdxA: a,
			PermIdxB: b,
		}
	}
	return fuzzOp{}
}

// ----------------------------------------------------------------------
// applyOp — execute the op against gs via the primitive helpers
// ----------------------------------------------------------------------

func applyOp(t *testing.T, st *fuzzState, op fuzzOp) error {
	t.Helper()
	gs := st.gs
	switch op.Kind {
	case "mint_og":
		c := fuzzCard(op.NameTag, op.Seat, op.Seat)
		MintOGInstanceID(gs, c)
		if c.InstanceID == "" {
			return nil // mint refused (shouldn't happen with valid seat)
		}
		// OG cards start in the owner's library.
		gs.Seats[op.Seat].Library = append(gs.Seats[op.Seat].Library, c)
		st.indexCard(c, "library")
		return nil

	case "mint_token":
		c := fuzzCard(op.NameTag, op.Seat, op.Seat)
		// Tag as token type so EnsureTokenInstanceID-style sweeps would
		// see it, and so cessation rules apply on bf-leave.
		c.Types = append([]string{"token"}, c.Types...)
		MintTokenInstanceID(gs, c, "", "")
		if c.InstanceID == "" {
			return nil
		}
		// Tokens enter the battlefield (CR §111.4). Wrap in a Permanent.
		perm := &Permanent{
			Card:       c,
			Controller: op.Seat,
			Owner:      op.Seat,
			Timestamp:  gs.NextTimestamp(),
		}
		gs.Seats[op.Seat].Battlefield = append(gs.Seats[op.Seat].Battlefield, perm)
		st.indexCard(c, "battlefield")
		st.cardPerm[c] = perm
		return nil

	case "mint_token_copy":
		if op.CardIdx < 0 || op.CardIdx >= len(st.indexed) {
			return nil
		}
		src := st.indexed[op.CardIdx]
		if src == nil || src.InstanceID == "" {
			return nil
		}
		tok := MintTokenAsCopyOf(gs, src, op.Seat, "")
		if tok == nil || tok.InstanceID == "" {
			return nil
		}
		perm := &Permanent{
			Card:       tok,
			Controller: op.Seat,
			Owner:      op.Seat,
			Timestamp:  gs.NextTimestamp(),
		}
		gs.Seats[op.Seat].Battlefield = append(gs.Seats[op.Seat].Battlefield, perm)
		st.indexCard(tok, "battlefield")
		st.cardPerm[tok] = perm
		return nil

	case "mint_spell_copy":
		if op.CardIdx < 0 || op.CardIdx >= len(st.indexed) {
			return nil
		}
		src := st.indexed[op.CardIdx]
		if src == nil || src.InstanceID == "" {
			return nil
		}
		cp := MintSpellCopy(gs, src)
		if cp == nil || cp.InstanceID == "" {
			return nil
		}
		// Spell copies live on the stack briefly per CR §707.10; for the
		// fuzz we drop them straight to graveyard with a cease mark on
		// resolution (the primitive contract — they don't persist in a
		// real zone).
		cp.Owner = op.Seat
		// Stash in the seat's graveyard transiently, then cease.
		gs.Seats[op.Seat].Graveyard = append(gs.Seats[op.Seat].Graveyard, cp)
		st.indexCard(cp, "graveyard")
		// Per §707.10 / §704.5e the copy ceases to exist anywhere but the
		// stack. We model this as immediate cease + zone removal.
		MarkInstanceIDCeased(gs, cp.InstanceID)
		gs.Seats[op.Seat].Graveyard, _ = fuzzRemoveCard(gs.Seats[op.Seat].Graveyard, cp)
		st.cardZone[cp] = "ceased"
		return nil

	case "move":
		if op.CardIdx < 0 || op.CardIdx >= len(st.indexed) {
			return nil
		}
		c := st.indexed[op.CardIdx]
		if c == nil {
			return nil
		}
		from := op.FromZone
		to := op.ToZone
		// Re-derive from in case prior ops moved it.
		actualFrom := st.cardZone[c]
		if actualFrom != from {
			// stale generator decision; trust actual
			from = actualFrom
		}
		if from == "" || from == "ceased" {
			return nil
		}
		if from == "battlefield" {
			// Battlefield moves require unwrapping the Permanent.
			perm, seatIdx := st.findPermFor(c)
			if perm == nil {
				return nil
			}
			// Phase 8 unmerge: dissolve any merge state before perm leaves.
			fuzzUnmergePerm(gs, st, perm, c)
			gs.Seats[seatIdx].Battlefield, _ = fuzzRemovePerm(gs.Seats[seatIdx].Battlefield, perm)
			delete(st.cardPerm, c)
			// CR §707.10 / §704.5e: copies and tokens cease on bf-leave.
			if perm.IsToken() || c.IsCopy {
				markPermanentCeaseIfToken(gs, perm)
				if c.IsCopy {
					MarkInstanceIDCeased(gs, c.InstanceID)
				}
				st.cardZone[c] = "ceased"
				return nil
			}
		} else {
			// Non-bf source — find seat and remove.
			owner := c.Owner
			if owner < 0 || owner >= len(gs.Seats) {
				return nil
			}
			seat := gs.Seats[owner]
			switch from {
			case "library":
				seat.Library, _ = fuzzRemoveCard(seat.Library, c)
			case "hand":
				seat.Hand, _ = fuzzRemoveCard(seat.Hand, c)
			case "graveyard":
				seat.Graveyard, _ = fuzzRemoveCard(seat.Graveyard, c)
			case "exile":
				seat.Exile, _ = fuzzRemoveCard(seat.Exile, c)
				// Moving OUT of exile implies the source-held linkage
				// resolved (LTB return, blink, etc.). Scrub matching
				// ExiledByMe entries so invariant D doesn't see a stale
				// pointer to a card that's no longer in exile.
				scrubExiledByMe(gs, c.InstanceID)
				c.ExiledByTimestamp = 0
			}
		}
		// Insert at destination.
		owner := c.Owner
		if owner < 0 || owner >= len(gs.Seats) {
			return nil
		}
		seat := gs.Seats[owner]
		switch to {
		case "library":
			seat.Library = append(seat.Library, c)
			st.cardZone[c] = "library"
		case "hand":
			seat.Hand = append(seat.Hand, c)
			st.cardZone[c] = "hand"
		case "graveyard":
			seat.Graveyard = append(seat.Graveyard, c)
			st.cardZone[c] = "graveyard"
		case "exile":
			seat.Exile = append(seat.Exile, c)
			st.cardZone[c] = "exile"
		case "battlefield":
			perm := &Permanent{
				Card:       c,
				Controller: owner,
				Owner:      owner,
				Timestamp:  gs.NextTimestamp(),
			}
			seat.Battlefield = append(seat.Battlefield, perm)
			st.cardZone[c] = "battlefield"
			st.cardPerm[c] = perm
		}
		return nil

	case "exile_with_linkage":
		perms := st.collectBattlefieldPerms()
		if op.PermIdxA < 0 || op.PermIdxA >= len(perms) {
			return nil
		}
		exiler := perms[op.PermIdxA]
		if op.CardIdx < 0 || op.CardIdx >= len(st.indexed) {
			return nil
		}
		c := st.indexed[op.CardIdx]
		if c == nil || c.InstanceID == "" {
			return nil
		}
		from := st.cardZone[c]
		if from != "library" && from != "hand" && from != "graveyard" {
			return nil
		}
		owner := c.Owner
		seat := gs.Seats[owner]
		switch from {
		case "library":
			seat.Library, _ = fuzzRemoveCard(seat.Library, c)
		case "hand":
			seat.Hand, _ = fuzzRemoveCard(seat.Hand, c)
		case "graveyard":
			seat.Graveyard, _ = fuzzRemoveCard(seat.Graveyard, c)
		}
		seat.Exile = append(seat.Exile, c)
		st.cardZone[c] = "exile"
		// Stamp source-held linkage per Phase 3.
		exiler.ExiledByMe = append(exiler.ExiledByMe, c.InstanceID)
		exiler.LinkageKind = LTBReturn
		c.ExiledByTimestamp = exiler.Timestamp
		return nil

	case "cease_battlefield_leave":
		perms := st.collectBattlefieldPerms()
		if op.PermIdxA < 0 || op.PermIdxA >= len(perms) {
			return nil
		}
		perm := perms[op.PermIdxA]
		if perm == nil || perm.Card == nil {
			return nil
		}
		// Locate seat.
		seatIdx := -1
		for i, s := range gs.Seats {
			for _, p := range s.Battlefield {
				if p == perm {
					seatIdx = i
					break
				}
			}
			if seatIdx >= 0 {
				break
			}
		}
		if seatIdx < 0 {
			return nil
		}
		c := perm.Card
		// Resolve source-held linkages: any cards this perm was exiling
		// return to their owner's graveyard (LTBReturn semantics) and
		// the ExiledByMe slice clears so invariant D doesn't see a
		// stale pointer.
		for _, exID := range perm.ExiledByMe {
			retCard := findCardByInstanceID(gs, exID)
			if retCard == nil {
				continue
			}
			owner := retCard.Owner
			if owner < 0 || owner >= len(gs.Seats) {
				continue
			}
			ownerSeat := gs.Seats[owner]
			ownerSeat.Exile, _ = fuzzRemoveCard(ownerSeat.Exile, retCard)
			ownerSeat.Graveyard = append(ownerSeat.Graveyard, retCard)
			retCard.ExiledByTimestamp = 0
			st.cardZone[retCard] = "graveyard"
		}
		perm.ExiledByMe = nil
		// Phase 8 unmerge: dissolve merge state before perm leaves.
		fuzzUnmergePerm(gs, st, perm, c)
		gs.Seats[seatIdx].Battlefield, _ = fuzzRemovePerm(gs.Seats[seatIdx].Battlefield, perm)
		delete(st.cardPerm, c)
		// §704.5d / §111.8 token cessation.
		markPermanentCeaseIfToken(gs, perm)
		if c.IsCopy {
			MarkInstanceIDCeased(gs, c.InstanceID)
		}
		isToken := false
		for _, ty := range c.Types {
			if ty == "token" {
				isToken = true
				break
			}
		}
		if isToken || c.IsCopy {
			st.cardZone[c] = "ceased"
		} else {
			// Real card heads to owner's graveyard.
			gs.Seats[c.Owner].Graveyard = append(gs.Seats[c.Owner].Graveyard, c)
			st.cardZone[c] = "graveyard"
		}
		return nil

	case "cease_explicit":
		if op.CardIdx < 0 || op.CardIdx >= len(st.indexed) {
			return nil
		}
		c := st.indexed[op.CardIdx]
		if c == nil || c.InstanceID == "" {
			return nil
		}
		// Only cease if the card is not currently in any zone — otherwise
		// we'd create a fabricated-present-but-ceased state (which is
		// exactly the kind of bug invariant B catches, but we want clean
		// ops). So: scrub from current zone first.
		zone := st.cardZone[c]
		if zone == "ceased" {
			return nil
		}
		owner := c.Owner
		if owner < 0 || owner >= len(gs.Seats) {
			return nil
		}
		seat := gs.Seats[owner]
		switch zone {
		case "library":
			seat.Library, _ = fuzzRemoveCard(seat.Library, c)
		case "hand":
			seat.Hand, _ = fuzzRemoveCard(seat.Hand, c)
		case "graveyard":
			seat.Graveyard, _ = fuzzRemoveCard(seat.Graveyard, c)
		case "exile":
			seat.Exile, _ = fuzzRemoveCard(seat.Exile, c)
		case "battlefield":
			perm := st.cardPerm[c]
			if perm != nil {
				fuzzUnmergePerm(gs, st, perm, c)
				for i, s := range gs.Seats {
					s.Battlefield, _ = fuzzRemovePerm(s.Battlefield, perm)
					_ = i
				}
				delete(st.cardPerm, c)
			}
		}
		// Cease also requires removing c from any sibling perm's
		// MergedCardPtrs (where it sits in "merged limbo") to avoid
		// a present-and-ceased false signal in invariant B.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil {
					continue
				}
				if p.MergedCardPtrs != nil {
					if _, ok := p.MergedCardPtrs[c.InstanceID]; ok {
						delete(p.MergedCardPtrs, c.InstanceID)
						out := p.MergedCards[:0]
						for _, mid := range p.MergedCards {
							if mid != c.InstanceID {
								out = append(out, mid)
							}
						}
						p.MergedCards = out
					}
				}
			}
		}
		MarkInstanceIDCeased(gs, c.InstanceID)
		// A ceased card's linkage is also resolved.
		scrubExiledByMe(gs, c.InstanceID)
		st.cardZone[c] = "ceased"
		return nil

	case "mutate_merge":
		perms := st.collectBattlefieldPerms()
		if op.PermIdxA < 0 || op.PermIdxA >= len(perms) {
			return nil
		}
		if op.PermIdxB < 0 || op.PermIdxB >= len(perms) {
			return nil
		}
		surv := perms[op.PermIdxA]
		dying := perms[op.PermIdxB]
		if surv == nil || dying == nil || surv == dying {
			return nil
		}
		if surv.Card == nil || dying.Card == nil {
			return nil
		}
		// Skip if dying is already merged INTO something (avoid double-link).
		if dying.MergeKind == MergeMeld {
			return nil
		}
		// Remove dying perm from its battlefield; its card pointer goes
		// into the merged-card stash, not into a zone. The InstanceID
		// stays alive in MergedCardPtrs per Phase 8 contract.
		seatIdx := -1
		for i, s := range gs.Seats {
			for _, p := range s.Battlefield {
				if p == dying {
					seatIdx = i
					break
				}
			}
			if seatIdx >= 0 {
				break
			}
		}
		if seatIdx < 0 {
			return nil
		}
		gs.Seats[seatIdx].Battlefield, _ = fuzzRemovePerm(gs.Seats[seatIdx].Battlefield, dying)
		dyingCard := dying.Card
		delete(st.cardPerm, dyingCard)
		// The dying card transitions to "merged limbo" — represented in
		// our zone tracker as "battlefield" since MergedCardPtrs is
		// counted by checkZoneConservationByInstanceID as a battlefield
		// presence.
		st.cardZone[dyingCard] = "battlefield"
		RecordMutateMerge(gs, surv, dying, true)
		return nil

	case "meld_merge":
		perms := st.collectBattlefieldPerms()
		if op.PermIdxA < 0 || op.PermIdxA >= len(perms) {
			return nil
		}
		if op.PermIdxB < 0 || op.PermIdxB >= len(perms) {
			return nil
		}
		a := perms[op.PermIdxA]
		b := perms[op.PermIdxB]
		if a == nil || b == nil || a == b {
			return nil
		}
		if a.Card == nil || b.Card == nil {
			return nil
		}
		if a.MergeKind != MergeNone || b.MergeKind != MergeNone {
			return nil
		}
		// For meld we keep `a` as the result perm; b's Card slides into
		// MergedCardPtrs. The "result" of meld is a NEW combined card per
		// §712, but for the fuzz we use `a` as the carrier — what
		// matters is that both InstanceIDs survive in MergedCardPtrs.
		seatIdx := -1
		for i, s := range gs.Seats {
			for _, p := range s.Battlefield {
				if p == b {
					seatIdx = i
					break
				}
			}
			if seatIdx >= 0 {
				break
			}
		}
		if seatIdx < 0 {
			return nil
		}
		gs.Seats[seatIdx].Battlefield, _ = fuzzRemovePerm(gs.Seats[seatIdx].Battlefield, b)
		delete(st.cardPerm, b.Card)
		st.cardZone[b.Card] = "battlefield"
		RecordMeldMergeWithCards(gs, a, a.Card.InstanceID, b.Card.InstanceID, a.Card, b.Card)
		return nil
	}
	return nil
}

// fuzzCard builds a minimal Card the fuzz can mint and move. owner is
// the seat that owns the card; the AST is a stub so DisplayName works.
func fuzzCard(name string, owner, seat int) *Card {
	_ = seat
	return &Card{
		Name:   name,
		Owner:  owner,
		Types:  []string{"creature"},
		Colors: []string{"R"},
		CMC:    2,
		AST:    &gameast.CardAST{Name: name},
	}
}

// ----------------------------------------------------------------------
// checkInvariants — the 5 InstanceID invariants
// ----------------------------------------------------------------------

// checkInvariants runs the 5-invariant battery and returns the first
// failure tagged with its letter. Nil on success.
func checkInvariants(st *fuzzState) error {
	gs := st.gs
	if err := checkCardIdentity(gs); err != nil {
		return fmt.Errorf("[A] %w", err)
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		return fmt.Errorf("[B] %w", err)
	}
	if err := propFuzzCheckLineage(gs); err != nil {
		return fmt.Errorf("[C] %w", err)
	}
	if err := propFuzzCheckExiledByMe(gs); err != nil {
		return fmt.Errorf("[D] %w", err)
	}
	if err := propFuzzCheckEnablerHistory(gs); err != nil {
		return fmt.Errorf("[E] %w", err)
	}
	return nil
}

// propFuzzCheckLineage: every CP/TK Card with non-empty SourceInstanceID
// must have its source in MintedInstanceIDs. Walks every zone + merged
// stashes.
func propFuzzCheckLineage(gs *GameState) error {
	var first error
	visit := func(c *Card) {
		if c == nil || c.SourceInstanceID == "" {
			return
		}
		// CP/TK only — OG never has lineage, AB is ephemeral.
		idLen := len(c.InstanceID)
		if idLen < 4 {
			return
		}
		prov := c.InstanceID[2:4]
		if prov != "CP" && prov != "TK" {
			return
		}
		if _, ok := gs.MintedInstanceIDs[c.SourceInstanceID]; !ok {
			if first == nil {
				first = fmt.Errorf("Lineage: card %q (id=%s prov=%s) references SourceInstanceID=%s which was never minted",
					c.DisplayName(), c.InstanceID, prov, c.SourceInstanceID)
			}
		}
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Library {
			visit(c)
		}
		for _, c := range s.Hand {
			visit(c)
		}
		for _, c := range s.Graveyard {
			visit(c)
		}
		for _, c := range s.Exile {
			visit(c)
		}
		for _, c := range s.CommandZone {
			visit(c)
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			visit(p.Card)
			for _, mc := range p.MergedCardPtrs {
				visit(mc)
			}
		}
	}
	return first
}

// propFuzzCheckExiledByMe: every entry in any permanent's ExiledByMe must
// either correspond to a Card present in some seat's Exile zone, or
// appear in CeasedInstanceIDs.
func propFuzzCheckExiledByMe(gs *GameState) error {
	// Build a set of InstanceIDs currently in any seat's Exile zone.
	inExile := map[string]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Exile {
			if c != nil && c.InstanceID != "" {
				inExile[c.InstanceID] = true
			}
		}
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			for _, id := range p.ExiledByMe {
				if id == "" {
					continue
				}
				if inExile[id] {
					continue
				}
				if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
					continue
				}
				return fmt.Errorf("ExiledByMe: perm %q (id=%s) references InstanceID=%s which is neither in any Exile zone nor in CeasedInstanceIDs",
					permDisplayName(p), permInstanceID(p), id)
			}
		}
	}
	return nil
}

// propFuzzCheckEnablerHistory: when EnablerHistory is non-empty, its
// last entry equals EnablerInstanceID.
func propFuzzCheckEnablerHistory(gs *GameState) error {
	var first error
	visit := func(c *Card) {
		if c == nil || len(c.EnablerHistory) == 0 {
			return
		}
		last := c.EnablerHistory[len(c.EnablerHistory)-1]
		if last != c.EnablerInstanceID {
			if first == nil {
				first = fmt.Errorf("EnablerHistory: card %q (id=%s) — last history entry %q != EnablerInstanceID %q",
					c.DisplayName(), c.InstanceID, last, c.EnablerInstanceID)
			}
		}
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Library {
			visit(c)
		}
		for _, c := range s.Hand {
			visit(c)
		}
		for _, c := range s.Graveyard {
			visit(c)
		}
		for _, c := range s.Exile {
			visit(c)
		}
		for _, c := range s.CommandZone {
			visit(c)
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			visit(p.Card)
			for _, mc := range p.MergedCardPtrs {
				visit(mc)
			}
		}
	}
	return first
}

func permDisplayName(p *Permanent) string {
	if p == nil || p.Card == nil {
		return "<nil>"
	}
	return p.Card.DisplayName()
}

func permInstanceID(p *Permanent) string {
	if p == nil || p.Card == nil {
		return ""
	}
	return p.Card.InstanceID
}

// ----------------------------------------------------------------------
// Driver
// ----------------------------------------------------------------------

func TestInstanceID_PropertyFuzz_R60(t *testing.T) {
	sequences := envInt("FUZZ_SEQUENCES", 10000)
	baseSeed := int64(envInt("FUZZ_BASE_SEED", 1))
	if testing.Short() && sequences > 1000 {
		sequences = 1000
	}

	kindCounts := map[string]int{}
	totalOps := 0

	for seq := 0; seq < sequences; seq++ {
		seed := baseSeed*1_000_003 + int64(seq)
		rng := rand.New(rand.NewSource(seed))
		st := newFuzzState(rng)
		opCount := 20 + rng.Intn(81) // [20, 100]
		history := make([]fuzzOp, 0, opCount)

		for i := 0; i < opCount; i++ {
			op := generateOp(rng, st, i)
			if op.Kind == "" {
				continue
			}
			if err := applyOp(t, st, op); err != nil {
				t.Fatalf("applyOp returned unexpected error on seed=%d op_idx=%d op=%s: %v",
					seed, i, op, err)
			}
			history = append(history, op)
			if err := checkInvariants(st); err != nil {
				dumpFuzzFailure(t, baseSeed, seq, i, seed, err, history, st)
				t.Fatalf("invariant failure (see dump above)")
			}
			kindCounts[op.Kind]++
			totalOps++
		}
	}

	// Per-op summary.
	var kinds []string
	for k := range kindCounts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	t.Logf("InstanceID property fuzz: %d sequences, %d total ops applied (base_seed=%d)",
		sequences, totalOps, baseSeed)
	for _, k := range kinds {
		t.Logf("  %-26s %d", k, kindCounts[k])
	}
}

// presenceOnBF returns true when any seat's Battlefield contains a Perm
// whose Card has the given InstanceID (also walks MergedCardPtrs).
func presenceOnBF(gs *GameState, id string) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Card.InstanceID == id {
				return true
			}
			for mid := range p.MergedCardPtrs {
				if mid == id {
					return true
				}
			}
		}
	}
	return false
}

// TestInstanceID_PropertyFuzz_R60_Repro re-runs a single seed for failure
// reproduction. Reads FUZZ_REPRO_SEED. Skips when env var is unset.
func TestInstanceID_PropertyFuzz_R60_Repro(t *testing.T) {
	envSeed := os.Getenv("FUZZ_REPRO_SEED")
	if envSeed == "" {
		t.Skip("FUZZ_REPRO_SEED not set")
	}
	seed, err := strconv.ParseInt(envSeed, 10, 64)
	if err != nil {
		t.Fatalf("invalid FUZZ_REPRO_SEED %q: %v", envSeed, err)
	}
	rng := rand.New(rand.NewSource(seed))
	st := newFuzzState(rng)
	opCount := 20 + rng.Intn(81)
	history := make([]fuzzOp, 0, opCount)
	for i := 0; i < opCount; i++ {
		op := generateOp(rng, st, i)
		if op.Kind == "" {
			continue
		}
		_ = applyOp(t, st, op)
		history = append(history, op)
		if errInv := checkInvariants(st); errInv != nil {
			dumpFuzzFailure(t, 0, 0, i, seed, errInv, history, st)
			t.Fatalf("invariant failure at op_idx=%d (see dump above)", i)
		}
	}
	t.Logf("Repro seed=%d completed cleanly across %d ops", seed, len(history))
}

// dumpFuzzFailure prints the failure shape documented in the prompt.
func dumpFuzzFailure(t *testing.T, baseSeed int64, seq, opIdx int, seed int64,
	err error, history []fuzzOp, st *fuzzState) {
	t.Helper()
	inv := "?"
	msg := err.Error()
	if len(msg) > 3 && msg[0] == '[' && msg[2] == ']' {
		inv = string(msg[1])
	}
	fmt.Fprintf(os.Stderr, "=== INSTANCEID PROPERTY FUZZ FAILURE ===\n")
	fmt.Fprintf(os.Stderr, "  base_seed=%d  sequence=%d  op_index=%d  derived_seed=%d\n",
		baseSeed, seq, opIdx, seed)
	fmt.Fprintf(os.Stderr, "  invariant=%s\n", inv)
	fmt.Fprintf(os.Stderr, "  error=%s\n", err.Error())
	fmt.Fprintf(os.Stderr, "  op_history=\n")
	for i, op := range history {
		fmt.Fprintf(os.Stderr, "    [%d] %s\n", i, op.String())
	}
	fmt.Fprintf(os.Stderr, "  state_summary=\n")
	gs := st.gs
	fmt.Fprintf(os.Stderr, "    seats=%d  total_indexed=%d  minted=%d  ceased=%d\n",
		len(gs.Seats), len(st.indexed), len(gs.MintedInstanceIDs), len(gs.CeasedInstanceIDs))
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "    seat%d={library:%d hand:%d gy:%d exile:%d bf:%d cmd:%d}\n",
			i, len(s.Library), len(s.Hand), len(s.Graveyard), len(s.Exile),
			len(s.Battlefield), len(s.CommandZone))
	}
	// Per-ID trace: extract the offending ID from the message and dump
	// what the fuzz model thinks its zone is.
	if idStart := stringsIndex(msg, "\""); idStart >= 0 {
		rest := msg[idStart+1:]
		if idEnd := stringsIndex(rest, "\""); idEnd >= 0 {
			id := rest[:idEnd]
			fmt.Fprintf(os.Stderr, "    offending_id=%s\n", id)
			_, ceased := gs.CeasedInstanceIDs[id]
			_, minted := gs.MintedInstanceIDs[id]
			fmt.Fprintf(os.Stderr, "      minted=%v ceased=%v\n", minted, ceased)
			for c, z := range st.cardZone {
				if c != nil && c.InstanceID == id {
					fmt.Fprintf(os.Stderr, "      tracker_zone=%q owner=%d types=%v\n",
						z, c.Owner, c.Types)
				}
			}
		}
	}
	// Phantom-bf scan — any card the fuzz tracker thinks is on the
	// battlefield but isn't actually in any seat's Battlefield slice
	// (and isn't covered by a MergedCardPtrs entry).
	bfPresent := map[string]bool{}
	mergedPresent := map[string]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil {
				bfPresent[p.Card.InstanceID] = true
				for mid := range p.MergedCardPtrs {
					mergedPresent[mid] = true
				}
			}
		}
	}
	for c, z := range st.cardZone {
		if z != "battlefield" || c == nil || c.InstanceID == "" {
			continue
		}
		if !bfPresent[c.InstanceID] && !mergedPresent[c.InstanceID] {
			fmt.Fprintf(os.Stderr, "    phantom_bf id=%s name=%q owner=%d\n",
				c.InstanceID, c.DisplayName(), c.Owner)
		}
	}
	fmt.Fprintf(os.Stderr, "=== END FAILURE ===\n")
	fmt.Fprintf(os.Stderr, "  reproduce: FUZZ_REPRO_SEED=%d go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60_Repro -count=1 -v\n",
		seed)
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
