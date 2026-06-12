package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVishgrazTheDoomhive wires Vishgraz, the Doomhive.
//
// Oracle text (Scryfall, verified):
//
//	Menace, toxic 1
//	When Vishgraz enters, create three 1/1 colorless Phyrexian Mite
//	artifact creature tokens with toxic 1 and "This token can't
//	block."
//	Vishgraz gets +1/+1 for each poison counter your opponents have.
//
// Implementation (R45 stub port):
//   - Menace, toxic 1: AST keyword pipeline.
//   - ETB: mint three 1/1 colorless Phyrexian Mite artifact creature
//     tokens with kw:toxic = 1 and a cant_block flag. (The auto-gen
//     stub created a single unnamed token; this port produces three
//     correctly-typed Mites.)
//   - Dynamic +1/+1 buff: the engine doesn't fire a poison-counter-
//     changed trigger, so the buff is recomputed at trigger points
//     where Vishgraz's stats actually matter — own ETB, own
//     combat_begin, and any creature dying / leaving the battlefield
//     (poison counts shift via death triggers, removal-on-toxic, or
//     other plays). The recompute strips a Modification tagged
//     "vishgraz_poison_buff" and appends a fresh one sized to
//     Σ_opp PoisonCounters.
func registerVishgrazTheDoomhive(r *Registry) {
	r.OnETB("Vishgraz, the Doomhive", vishgrazTheDoomhiveETB)
	r.OwnsETBTrigger("Vishgraz, the Doomhive")
	r.OnTrigger("Vishgraz, the Doomhive", "combat_begin", vishgrazRecomputeBuff)
	r.OnTrigger("Vishgraz, the Doomhive", "creature_dies", vishgrazRecomputeBuff)
	r.OnTrigger("Vishgraz, the Doomhive", "permanent_ltb", vishgrazRecomputeBuff)
}

const vishgrazPoisonBuffTag = "vishgraz_poison_buff"

func vishgrazTheDoomhiveETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "vishgraz_the_doomhive_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 3; i++ {
		tok := gameengine.CreateCreatureToken(
			gs,
			seat,
			"Phyrexian Mite",
			[]string{"artifact", "creature", "phyrexian"},
			1, 1,
		)
		if tok == nil {
			continue
		}
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:toxic"] = 1
		tok.Flags["cant_block"] = 1
	}
	vishgrazSyncBuff(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": 3,
		"type":   "Phyrexian Mite",
	})
}

func vishgrazRecomputeBuff(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	vishgrazSyncBuff(gs, perm)
}

// vishgrazSyncBuff strips any prior poison-buff Modification on perm
// and appends a fresh one sized to the sum of opponents' poison
// counters.
func vishgrazSyncBuff(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	mods := perm.Modifications[:0]
	for _, m := range perm.Modifications {
		if m.Duration == vishgrazPoisonBuffTag {
			continue
		}
		mods = append(mods, m)
	}
	perm.Modifications = mods

	total := 0
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller {
			continue
		}
		if s.PoisonCounters > 0 {
			total += s.PoisonCounters
		}
	}
	if total > 0 {
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:     total,
			Toughness: total,
			Duration:  vishgrazPoisonBuffTag,
			Timestamp: gs.NextTimestamp(),
		})
	}
	gs.InvalidateCharacteristicsCache()
}
