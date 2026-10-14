package gameengine

import "strings"

// r87 — refcounted keyword-flag grants.
//
// THE DEFECT (filed 6f332cbc, probed with two real handlers). The
// convention every per_card keyword-grant handler follows is: stamp
// Flags["kw:<x>"] = 1 plus a private provenance marker of its own, and
// on leave strip both, gated on the marker. Flags["kw:<x>"] is ONE
// SHARED BOOLEAN with no notion of how many sources are holding it, so
// two sources granting the same keyword to the same permanent collide:
// whichever leaves first deletes the shared flag and takes the keyword
// away while the other source is still on the battlefield.
//
// Nearly every handler also runs a guard — "don't stamp over a card
// that already has the keyword, else our strip would remove the native
// one on the way out". That justification is FALSE. Every strip in the
// tree is provenance-gated and deletes Flags["kw:<x>"], which is not
// where an AST keyword lives; stamping over a printed keyword was never
// the risk. What the guard actually does is make the SECOND source skip
// recording its own grant, which is the collision above — so the guard
// causes the exact harm it names. Probe: Brudiclad (haste to creature
// tokens) + Vihaan (vigilance/haste to outlaws) + a Pirate creature
// token, which qualifies for both. Vihaan never records vihaan_grant_haste;
// Brudiclad leaves and the token loses haste with Vihaan still out.
//
// WHAT THIS FIXES AND WHAT IT DOES NOT. Migrating the handlers' keyword
// READS to keywordActive does not fix it — the layer query folds the
// same shared flag. Dropping the guard does not fix it either — it moves
// the collision to whichever source strips first. The fix has to be a
// refcount, which is what this is.
//
// THE SHAPE. Flags["kw:<x>"] stays exactly as it is, because it is the
// surface four readers already consult (Permanent.HasKeyword surface 3,
// runtimeKeywordSources surface 1, and the two through the layer fold).
// Alongside it each source records a marker key in a SEPARATE namespace,
// "kwsrc:<keyword>:<marker>". The shared flag is present iff at least
// one marker is. Revoking removes one marker and only deletes the shared
// flag when it was the last.
//
// The marker namespace is deliberately NOT under "kw:".
// runtimeKeywordSources folds every "kw:"-prefixed key VERBATIM into
// chars.Keywords, so a provenance tag written as "kw:menace_from_gisa"
// (the convention this migration replaced, a84fc750) lands in the
// layer-computed keyword set as a keyword literally named
// "menace_from_gisa". Harmless only because nothing asks for that name.
// "kwsrc:" keeps provenance out of the keyword set entirely.
//
// UPDATED r91: that convention is GONE from production code. The last
// four tags — menace_from_gisa, haste_from_kolodin,
// trample_from_old_one_eye, lifelink_from_heliod_ueot — migrated in
// batch 2, and `grep -o 'kw:[a-z_]*_from_[a-z_]*'` over internal/ now
// returns nothing outside tests that synthesize the flag to pin the
// fold's behaviour. The hazard the namespace poses is unchanged, so the
// rule stands for anything written next; what changed is that no live
// handler relies on it any more.
//
// INTEROP WITH THE ~230 UNMIGRATED RAW STAMP SITES. A raw stamper writes
// Flags["kw:<x>"] with no marker. If a migrated source then granted and
// later revoked as the last marker holder, it would delete a flag it did
// not place and clobber the raw grant. GrantKeywordFlag therefore
// records a synthetic marker for a shared flag it finds ALREADY SET with
// no markers behind it, so the pre-existing grant survives the migrated
// source's revoke. Revoke only ever deletes, never re-adds, so a raw
// source deleting the shared flag directly (which is what every
// unmigrated strip does) leaves nothing stuck.
//
// NEITHER FUNCTION TOUCHES THE CHARACTERISTICS CACHE. Both report
// whether the permanent's keyword set actually changed; the caller
// invalidates when it did, which is the existing contract at every one
// of these call sites and keeps a battlefield sweep to one invalidation
// rather than one per permanent.

const keywordGrantMarkerPrefix = "kwsrc:"

// keywordFlagPreexistingMarker stands in for a grant that was already on
// the shared flag when the first refcounted source arrived — either an
// unmigrated handler's raw stamp or generic engine code. It is never
// revoked by name, so it pins the shared flag for as long as any
// refcounted source might try to remove it.
const keywordFlagPreexistingMarker = "__preexisting"

// normalizeKeywordFlagName matches Permanent.HasKeyword's normalization
// exactly (lowercase, whitespace-trimmed), because HasKeyword looks up
// the literal key "kw:"+want. A stamp written in any other case would be
// a different key to the reader and would grant nothing.
func normalizeKeywordFlagName(kw string) string {
	return strings.ToLower(strings.TrimSpace(kw))
}

func keywordFlagKey(kw string) string {
	return "kw:" + kw
}

func keywordGrantMarkerKey(kw, marker string) string {
	return keywordGrantMarkerPrefix + kw + ":" + marker
}

// keywordGrantMarkerCount counts the live per-source markers for kw.
// kw must already be normalized.
func keywordGrantMarkerCount(p *Permanent, kw string) int {
	if p == nil || len(p.Flags) == 0 {
		return 0
	}
	prefix := keywordGrantMarkerPrefix + kw + ":"
	n := 0
	for k := range p.Flags {
		if strings.HasPrefix(k, prefix) {
			n++
		}
	}
	return n
}

// KeywordFlagGrantCount reports how many distinct sources currently hold
// a refcounted grant of kw on p. A pre-existing raw stamp adopted by the
// first refcounted grant counts as one.
func KeywordFlagGrantCount(p *Permanent, kw string) int {
	return keywordGrantMarkerCount(p, normalizeKeywordFlagName(kw))
}

// GrantKeywordFlag records that marker grants kw to p through the
// Flags["kw:<x>"] surface, and ensures the shared flag is set.
//
// marker identifies the SOURCE of the grant, not the permanent receiving
// it — one stable string per granting effect (e.g. "brudiclad_haste").
// Granting twice under the same marker is idempotent, which matters
// because these handlers run as repeated battlefield sweeps.
//
// Returns true if the permanent's keyword set changed, i.e. the shared
// flag was not previously present. The caller must then invalidate the
// characteristics cache.
func GrantKeywordFlag(p *Permanent, kw, marker string) bool {
	if p == nil {
		return false
	}
	kw = normalizeKeywordFlagName(kw)
	if kw == "" || marker == "" {
		return false
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	flagKey := keywordFlagKey(kw)
	_, hadFlag := p.Flags[flagKey]

	// Adopt a grant that was already on the shared flag without a marker
	// behind it, so our revoke cannot take away something we did not put
	// there. Only when there are no markers at all: once this source or
	// any other has registered one, a set flag is accounted for.
	if hadFlag && keywordGrantMarkerCount(p, kw) == 0 {
		p.Flags[keywordGrantMarkerKey(kw, keywordFlagPreexistingMarker)] = 1
	}

	p.Flags[keywordGrantMarkerKey(kw, marker)] = 1
	p.Flags[flagKey] = 1
	return !hadFlag
}

// RevokeKeywordFlag withdraws marker's grant of kw from p. The shared
// Flags["kw:<x>"] is deleted only when marker was the last source
// holding it — which is the whole point: a second source's grant keeps
// the keyword alive after the first source leaves.
//
// Returns true if the permanent's keyword set changed, i.e. the shared
// flag was actually removed. The caller must then invalidate the
// characteristics cache.
func RevokeKeywordFlag(p *Permanent, kw, marker string) bool {
	if p == nil || len(p.Flags) == 0 {
		return false
	}
	kw = normalizeKeywordFlagName(kw)
	if kw == "" || marker == "" {
		return false
	}
	markerKey := keywordGrantMarkerKey(kw, marker)
	if _, ok := p.Flags[markerKey]; !ok {
		return false
	}
	delete(p.Flags, markerKey)
	if keywordGrantMarkerCount(p, kw) > 0 {
		return false
	}
	flagKey := keywordFlagKey(kw)
	if _, ok := p.Flags[flagKey]; !ok {
		return false
	}
	delete(p.Flags, flagKey)
	return true
}

// HasKeywordFlagGrant reports whether marker currently holds a
// refcounted grant of kw on p. It replaces the private
// "<handler>_grant" boolean each handler kept beside its stamp, so a
// sweep can skip a permanent it has already granted to.
func HasKeywordFlagGrant(p *Permanent, kw, marker string) bool {
	if p == nil || len(p.Flags) == 0 {
		return false
	}
	_, ok := p.Flags[keywordGrantMarkerKey(normalizeKeywordFlagName(kw), marker)]
	return ok
}
