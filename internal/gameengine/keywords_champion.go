package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_champion.go — Champion a [type] (CR §702.72, Lorwyn 2007).
//
// CR §702.72b: "When this permanent enters the battlefield, sacrifice it unless
//                you exile another [quality] permanent you control."
// CR §702.72c: "When this permanent leaves the battlefield, return the exiled
//                card to the battlefield under its owner's control."
//
// Champion is the canonical CR §406.7 "exile until ~ leaves" linked-exile shape
// (same machinery as Banisher Priest), with two twists: the exiled permanent is
// YOUR OWN (a creature of the named type), and if you control none the champion
// is SACRIFICED. This file provides the engine primitives; they build on the
// proven ExileLinked / ReturnLinkedExile linked-exile infra.
//
// NOTE (wiring): these primitives are not yet auto-invoked on the live ETB/LTB
// pipeline — see the audit. A champion creature's self-ETB would have to fire
// from BOTH resolvePermanentSpellETB (cast) and FirePermanentETBTriggers
// (non-cast), and the champion-keyword's type AST isn't verified against a
// real corpus card. Same unwired-primitive state as crew/saddle/enlist/exert.

// HasChampion reports whether the card has the champion keyword.
func HasChampion(card *Card) bool {
	return cardHasKeywordByName(card, "champion")
}

// ChampionType returns the creature type a champion permanent must champion
// ("Champion an Elf" → "elf"), lowercased. Returns "" when unspecified (any
// creature qualifies). Reads keyword Args[0] (string) first, else parses the
// "Champion a/an <Type>" reminder/Raw text.
func ChampionType(card *Card) string {
	if card == nil || card.AST == nil {
		return ""
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok || !keywordNameEquals(kw, "champion") {
			continue
		}
		if len(kw.Args) > 0 {
			if s, ok := kw.Args[0].(string); ok && strings.TrimSpace(s) != "" {
				return strings.ToLower(strings.TrimSpace(s))
			}
		}
		return parseChampionTypeFromRaw(kw.Raw)
	}
	return ""
}

func parseChampionTypeFromRaw(raw string) string {
	low := strings.ToLower(raw)
	idx := strings.Index(low, "champion ")
	if idx < 0 {
		return ""
	}
	fields := strings.Fields(low[idx+len("champion "):])
	i := 0
	if i < len(fields) && (fields[i] == "a" || fields[i] == "an") {
		i++
	}
	if i < len(fields) {
		return strings.TrimRight(fields[i], ".,)")
	}
	return ""
}

// championMatches reports whether `p` is a legal championing target for the
// champion permanent: ANOTHER creature you control (CR §702.72b) whose types/
// subtypes include `typ` (any creature when typ == "").
func championMatches(p, champion *Permanent, typ string) bool {
	if p == nil || p == champion || p.Card == nil || !p.IsCreature() {
		return false
	}
	if typ == "" {
		return true
	}
	for _, t := range p.Card.Types {
		if strings.ToLower(strings.TrimSpace(t)) == typ {
			return true
		}
	}
	return false
}

// ApplyChampion resolves a champion permanent's ETB (CR §702.72b): exile
// another [type] creature you control — linked to the champion, to be returned
// when the champion leaves (ChampionReturnOnLTB) — or, if you control no
// eligible creature, SACRIFICE the champion (you can't keep it unchampioned).
//
// The championed creature goes to EXILE (not the graveyard), tracked linked to
// the champion via the canonical ExileLinked primitive (§406.7). Auto-policy:
// exile the least valuable eligible creature (lowest P+T) so the champion (the
// usually-better body) stays. Returns the exiled permanent's card, or nil if
// the champion was sacrificed.
func ApplyChampion(gs *GameState, perm *Permanent) *Card {
	if gs == nil || perm == nil || perm.Card == nil {
		return nil
	}
	if !HasChampion(perm.Card) {
		return nil
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return nil
	}
	typ := ChampionType(perm.Card)

	var victim *Permanent
	bestScore := 1 << 30
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if !championMatches(p, perm, typ) {
			continue
		}
		score := p.Power() + p.Toughness()
		if score < bestScore {
			bestScore = score
			victim = p
		}
	}

	if victim == nil {
		// CR §702.72b — sacrifice the champion if you don't exile one.
		gs.LogEvent(Event{
			Kind:   "champion_sacrifice",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule":   "702.72b",
				"reason": "no_eligible_permanent_to_champion",
			},
		})
		SacrificePermanent(gs, perm, "champion")
		return nil
	}

	owner := victim.Card.Owner
	victimCard := victim.Card
	// Remove + exile-link the championed permanent (mirrors the proven
	// Banisher-Priest removePermanent → ExileLinked sequence). ExileLinked
	// stamps the §406.7 linkage (LinkedExile / ExiledByMe / LTBReturn) so the
	// ExileLinkageIntegrity invariant tracks it and the LTB return finds it.
	gs.removePermanent(victim)
	gs.UnregisterReplacementsForPermanent(victim)
	gs.UnregisterContinuousEffectsForPermanent(victim)
	ExileLinked(gs, perm, victimCard, owner, "battlefield")

	gs.LogEvent(Event{
		Kind:   "champion",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"championed": victimCard.DisplayName(),
			"type":       typ,
			"rule":       "702.72b",
		},
	})
	return victimCard
}

// ChampionReturnOnLTB returns the championed card to the battlefield when the
// champion leaves (CR §702.72c). Reuses the canonical ReturnLinkedExile, so the
// return happens even if the champion and championed would leave simultaneously
// (the linked card is in exile, returned off the champion's LTB).
func ChampionReturnOnLTB(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !HasChampion(perm.Card) || len(perm.LinkedExile) == 0 {
		return
	}
	ReturnLinkedExile(gs, perm, "battlefield")
}
