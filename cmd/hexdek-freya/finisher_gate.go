package main

import "strings"

// finisherEvasionKeywords is the curated set of true-evasion keywords
// that justify counting a high-power creature as a commander-damage
// finisher. The bar is "the creature reliably connects in 4-player EDH
// despite chump blockers" — flying, menace (2-blocker tax), fear /
// shadow / horsemanship / intimidate (color-restricted unblockability),
// skulk (size-restricted unblockability), and the "can't be blocked"
// / "unblockable" phrasings. Trample is intentionally excluded — a
// 6/6 trampler gets chumped for 1, leaving 5 through to a single
// opponent, which is a 5-turn clock against one of three players, not
// a finisher. Trample only matters at extreme power (pwr≥10+) where
// the math overrides the chump, but those creatures are usually
// already tagged via another path (variable power → Fling target;
// MakesInfiniteTokens; HasETBDamage; mass-pump-recipient).
var finisherEvasionKeywords = []string{
	"flying",
	"menace",
	"fear",
	"shadow",
	"horsemanship",
	"intimidate",
	"skulk",
	"can't be blocked",
	"unblockable",
}

// finisherProtectionKeywords is the curated set of "this creature
// survives a removal spell" signals. The bar is "the opponent must
// commit a SECOND answer or the creature swings again next turn":
// hexproof / shroud (instant-speed targeting denial), indestructible
// (board wipes and damage fail), ward (taxes spot removal),
// protection from <X> (color-shifted hexproof + can't-be-blocked
// against the same color). Phasing self-protection (Teferi's
// Protection, Soul Echo) is excluded because it lives on supporting
// cards, not on the creature itself.
var finisherProtectionKeywords = []string{
	"hexproof",
	"shroud",
	"indestructible",
	"ward",
	"protection from",
}

// hasFinisherEvasionOrProtection reports whether the given oracle
// text (already-lowercased by the caller) contains at least one
// keyword from either curated set. The gate is OR, not AND — a card
// only needs one of "you can't block this" or "you can't remove
// this" to deserve finisher status at high power.
func hasFinisherEvasionOrProtection(oracleText string) bool {
	for _, kw := range finisherEvasionKeywords {
		if strings.Contains(oracleText, kw) {
			return true
		}
	}
	for _, kw := range finisherProtectionKeywords {
		if strings.Contains(oracleText, kw) {
			return true
		}
	}
	return false
}
