// Package seedcontract is the Phase 1 anti-cheat primitive: a
// per-game cryptographic commitment that binds a game's inputs (RNG
// seed, deck keys, engine version, n seats) to its outcome (winner,
// turn count, kill method, elimination order, final life totals).
//
// At game start the runner assembles a *SeedContract from the inputs
// and computes InputDigest. At game end the runner calls Seal() with
// the observed outcome and Sign(key) with an HMAC key derived from the
// server's master secret. The fully-signed contract travels with the
// GameOutcome and is appended to the heimdall seed JSONL stream.
//
// Verification is two-stage:
//
//  1. Verify(key) confirms the HMAC matches the (input || outcome)
//     digest pair. A tampered claim — e.g. a forged winner — flips the
//     outcome digest, so Verify returns false even if the attacker
//     re-uses a valid signature from a different game.
//  2. heimdall.VerifyReplay re-executes the game from the contract's
//     inputs and recomputes the outcome digest. If the replay's digest
//     differs from the contract's claim, the result was tampered after
//     the fact (or the replay is non-deterministic — Phase 1 needs
//     deterministic replay to be a hard guarantee).
//
// This package deliberately has no dependency on tournament or
// gameengine so both can import it without creating a cycle. The
// replay verifier lives in heimdall (which already imports tournament)
// and consumes the contract's inputs.
package seedcontract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Schema is the on-disk version. Bump on any change to the digest
// canonicalization so old contracts surface as "wrong schema" rather
// than silently failing Verify.
const Schema = 1

// inputDigestPrefix and outcomeDigestPrefix are domain separators that
// prevent a digest from one stage from being interpreted as the other.
// Without them an attacker who controls one half of the digest could
// in principle craft a collision; the prefix forces SHA256 inputs to
// live in disjoint namespaces.
const (
	inputDigestPrefix   = "hxdk-seed-input-v1\x00"
	outcomeDigestPrefix = "hxdk-seed-outcome-v1\x00"
	sigPrefix           = "hxdk-seed-sig-v1\x00"
)

// MaxSeats is the upper bound on n_seats for Commander pods. The
// digest format reserves fixed slots up to this size so contracts
// remain comparable across NSeats variants.
const MaxSeats = 4

// Inputs is the set of fields fingerprinted at game start. Every field
// that influences game determinism must be present — adding a new
// influencer (e.g. a hat profile id) without including it here breaks
// the replay invariant.
type Inputs struct {
	RNGSeed       int64
	DeckKeys      [MaxSeats]string
	NSeats        int
	EngineVersion string
	SealedAtUnix  int64
}

// Outcome is the set of fields fingerprinted at game end. Anything
// that downstream tooling treats as "the result" should land here so a
// tampered post-game payload is detectable.
type Outcome struct {
	Winner           int
	Turns            int
	KillMethod       string
	EndReason        string
	EliminationOrder [MaxSeats]int
	FinalLife        [MaxSeats]int
}

// SeedContract is the signed claim about a game's inputs and outcome.
type SeedContract struct {
	Schema        int                `json:"schema"`
	RNGSeed       int64              `json:"rng_seed"`
	DeckKeys      [MaxSeats]string   `json:"deck_keys"`
	NSeats        int                `json:"n_seats"`
	EngineVersion string             `json:"engine_version"`
	SealedAtUnix  int64              `json:"sealed_at"`

	InputDigest   string `json:"input_digest"`
	OutcomeDigest string `json:"outcome_digest,omitempty"`
	Sig           string `json:"sig,omitempty"`

	// Outcome fields are stored on the contract so a verifier can
	// re-derive OutcomeDigest from the same fields it would have
	// produced post-game. The post-game runner also sets these on
	// Seal(); they are NOT trusted by Verify until the digest matches.
	Outcome Outcome `json:"outcome,omitempty"`
}

// New returns a fresh contract with the input digest computed. The
// returned contract is unsigned and has no outcome yet — call Seal
// when the game ends and Sign with the per-context HMAC key.
func New(in Inputs) *SeedContract {
	c := &SeedContract{
		Schema:        Schema,
		RNGSeed:       in.RNGSeed,
		DeckKeys:      in.DeckKeys,
		NSeats:        in.NSeats,
		EngineVersion: in.EngineVersion,
		SealedAtUnix:  in.SealedAtUnix,
	}
	c.InputDigest = digestInputs(in)
	return c
}

// Seal records the game's outcome on the contract and computes the
// outcome digest. Call once when the game ends, before Sign.
//
// The outcome is CANONICALIZED first (see CanonicalizeOutcome): the
// KillMethod field is derived from (Winner, EndReason) rather than
// trusted from the caller. r62 anti-cheat fix — the seal side
// (tournament runner) and the verify side (heimdall replay) previously
// computed KillMethod through two different hand-synced mappers that
// disagreed on draws, so honest games failed digest verification.
// Deriving it inside Seal makes both sides identical by construction:
// the replay verifier also routes its recomputed outcome through Seal.
func (c *SeedContract) Seal(out Outcome) {
	out = CanonicalizeOutcome(out)
	c.Outcome = out
	c.OutcomeDigest = digestOutcome(out)
}

// CanonicalizeOutcome normalizes an Outcome into the single canonical
// form both the seal path and the replay-verify path digest:
//
//   - Winner < 0 (draw / crash / unresolved) → KillMethod "" — a game
//     with no winner has no kill. This matches what the pre-r62 seal
//     side always produced, so contracts signed by older builds
//     re-digest identically (no schema bump needed).
//   - Winner >= 0 → KillMethod = KillMethodFromEndReason(EndReason).
//
// Every other field is passed through untouched — they must come from
// the game itself (live or replayed), and none of them may be derived
// from wall-clock time (digests must be reproducible from the RNG seed
// and decks alone).
func CanonicalizeOutcome(out Outcome) Outcome {
	if out.Winner < 0 {
		out.KillMethod = ""
	} else {
		out.KillMethod = KillMethodFromEndReason(out.EndReason)
	}
	return out
}

// KillMethodFromEndReason maps a runner EndReason to the kill-method
// enum stored on the contract (and used by heimdall's 1-byte binary
// seed encoding). This is THE single mapper — the tournament seal path
// and the heimdall replay-verify path both route through it (directly
// or via Seal's canonicalization). It previously existed as two
// hand-synced copies (tournament.inferKillMethodFromOutcome and
// heimdall.killMethodFromEndReason) which had already drifted.
func KillMethodFromEndReason(endReason string) string {
	switch endReason {
	case "turn_cap", "turn_cap_leader", "turn_cap_tie", "turn_cap_all_dead":
		return "timeout"
	case "draw":
		return "draw"
	case "crash":
		return "crash"
	default:
		return endReason
	}
}

// Sign HMAC-SHA256s the (input_digest || outcome_digest) pair with the
// supplied key and stores the hex-encoded result on the contract.
// Calling Sign before Seal produces a signature over only the input
// half — useful for "pre-commitment" workflows but not the default.
func (c *SeedContract) Sign(key []byte) {
	c.Sig = hex.EncodeToString(c.macTag(key))
}

// Verify recomputes the HMAC and compares it against c.Sig with a
// constant-time check. Returns false if Sig is empty, malformed, or
// does not match the expected tag.
//
// IMPORTANT: Verify checks signature integrity only. A passing Verify
// means the (InputDigest, OutcomeDigest, Sig) triple is internally
// consistent under the supplied key — it does NOT prove the outcome
// digest matches the inputs' actual replay outcome. For that, call
// heimdall.VerifyReplay.
func (c *SeedContract) Verify(key []byte) bool {
	if c == nil || c.Sig == "" {
		return false
	}
	want := c.macTag(key)
	got, err := hex.DecodeString(c.Sig)
	if err != nil {
		return false
	}
	return hmac.Equal(want, got)
}

// RederiveInputDigest recomputes the input digest from the inputs
// stored on the contract. Used by verifiers to detect mutation of any
// input field after signing — e.g. swapping a deck key.
func (c *SeedContract) RederiveInputDigest() string {
	return digestInputs(Inputs{
		RNGSeed:       c.RNGSeed,
		DeckKeys:      c.DeckKeys,
		NSeats:        c.NSeats,
		EngineVersion: c.EngineVersion,
		SealedAtUnix:  c.SealedAtUnix,
	})
}

// RederiveOutcomeDigest recomputes the outcome digest from the
// outcome stored on the contract.
func (c *SeedContract) RederiveOutcomeDigest() string {
	return digestOutcome(c.Outcome)
}

// CheckIntegrity confirms (a) the stored InputDigest matches the
// stored inputs, (b) the stored OutcomeDigest matches the stored
// outcome (or both are empty for unsigned pre-game contracts), and
// (c) Verify(key) passes. Returns the first mismatch description, or
// nil if all three checks pass.
func (c *SeedContract) CheckIntegrity(key []byte) error {
	if c == nil {
		return errors.New("nil contract")
	}
	if c.Schema != Schema {
		return fmt.Errorf("schema mismatch: have %d want %d", c.Schema, Schema)
	}
	if got := c.RederiveInputDigest(); got != c.InputDigest {
		return fmt.Errorf("input digest tampered: stored %s computed %s", c.InputDigest, got)
	}
	if c.OutcomeDigest != "" {
		if got := c.RederiveOutcomeDigest(); got != c.OutcomeDigest {
			return fmt.Errorf("outcome digest tampered: stored %s computed %s", c.OutcomeDigest, got)
		}
	}
	if !c.Verify(key) {
		return errors.New("signature invalid for this key")
	}
	return nil
}

// macTag is the raw HMAC-SHA256 over the domain-separated digest pair.
func (c *SeedContract) macTag(key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(sigPrefix))
	mac.Write([]byte(c.InputDigest))
	mac.Write([]byte{0x1f}) // unit separator between fields
	mac.Write([]byte(c.OutcomeDigest))
	return mac.Sum(nil)
}

// DeriveContractKey produces a per-context HMAC key from a master
// secret. Use a stable context string ("tournament/<id>",
// "showmatch/<build>", etc.) so verifiers can reconstruct the same
// key by re-hashing master+context.
func DeriveContractKey(masterSecret []byte, context string) []byte {
	mac := hmac.New(sha256.New, masterSecret)
	mac.Write([]byte("hxdk-key-derive-v1\x00"))
	mac.Write([]byte(context))
	return mac.Sum(nil)
}

// ----------------------------------------------------------------------
// Canonical digesting
// ----------------------------------------------------------------------
// Both digests are SHA256 over a length-prefixed, fixed-order
// concatenation of the input fields. The fixed order is critical —
// any other canonicalization (JSON, sorting by name, etc.) risks
// silent reorderings on Go version upgrades.

func digestInputs(in Inputs) string {
	h := sha256.New()
	h.Write([]byte(inputDigestPrefix))
	writeI64(h, in.RNGSeed)
	writeI64(h, in.SealedAtUnix)
	writeI32(h, int32(in.NSeats))
	writeStr(h, in.EngineVersion)
	for i := 0; i < MaxSeats; i++ {
		writeStr(h, in.DeckKeys[i])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func digestOutcome(out Outcome) string {
	h := sha256.New()
	h.Write([]byte(outcomeDigestPrefix))
	writeI32(h, int32(out.Winner))
	writeI32(h, int32(out.Turns))
	writeStr(h, out.KillMethod)
	writeStr(h, out.EndReason)
	for i := 0; i < MaxSeats; i++ {
		writeI32(h, int32(out.EliminationOrder[i]))
	}
	for i := 0; i < MaxSeats; i++ {
		writeI32(h, int32(out.FinalLife[i]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeI64(h interface{ Write([]byte) (int, error) }, v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	h.Write(b[:])
}

func writeI32(h interface{ Write([]byte) (int, error) }, v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	h.Write(b[:])
}

func writeStr(h interface{ Write([]byte) (int, error) }, s string) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(len(s)))
	h.Write(b[:])
	h.Write([]byte(s))
}

// CanonicalContextForTournament builds a stable context string from a
// list of identifying tags. Order doesn't matter — tags are sorted —
// so callers can pass them in arbitrary order.
func CanonicalContextForTournament(tags []string) string {
	cp := append([]string(nil), tags...)
	sort.Strings(cp)
	return "tournament:" + strings.Join(cp, "|")
}
