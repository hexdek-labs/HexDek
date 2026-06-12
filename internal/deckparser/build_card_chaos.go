package deckparser

import (
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// BuildCardFromName builds a playable gameengine.Card from the AST
// corpus + meta DB, with the chaos-grade fallbacks loki accumulated
// (DFC per-face retry, ETB-choice 0/0 P/T defaults). PROMOTED from
// cmd/hexdek-loki's buildCardFromName (r63 Judge CI gate) so loki and
// hexdek-judge --run share one implementation; loki's function is now
// a delegate.
func BuildCardFromName(name string, corpus *astload.Corpus, meta *MetaDB) *gameengine.Card {
	// Mirrors deckparser.buildCard logic but is exported for chaos use.
	var cardAST *gameast.CardAST
	if corpus != nil {
		cardAST, _ = corpus.Get(name)
	}
	md := meta.Get(name)

	if cardAST == nil && md == nil {
		// DFC fallback (legality sweep round 3, set-aside #2): the corpus
		// and meta key some double-faced cards by single-face name while
		// the chaos deck generator carries the full "Front // Back"
		// oracle name. Pre-fix the miss fell through to the caller's
		// bare-bones path — a CMC-0 typeless card the chaos games then
		// "cast" for announced-0, weakening DFC coverage in every sweep
		// and polluting the 117.1a non-active sub-shape with typeless
		// cards riding odd cast paths. Retry per face — front first (the
		// castable face), then back — before giving up. Recursion is
		// bounded: face names contain no " // ".
		if strings.Contains(name, " // ") {
			for _, face := range strings.Split(name, " // ") {
				face = strings.TrimSpace(face)
				if face == "" || face == name {
					continue
				}
				if got := BuildCardFromName(face, corpus, meta); got != nil {
					return got
				}
			}
		}
		return nil
	}

	c := &gameengine.Card{
		AST:   cardAST,
		Name:  name,
		Owner: -1,
	}
	if md != nil {
		c.Name = md.Name
		if len(md.Types) > 0 {
			c.Types = append([]string(nil), md.Types...)
		}
		c.BasePower = md.Power
		c.BaseToughness = md.Toughness
		if len(md.Colors) > 0 {
			c.Colors = append([]string(nil), md.Colors...)
		}
		c.CMC = md.CMC
		c.TypeLine = md.TypeLine
	}

	// ETB-choice P/T fix: if this is a creature with 0/0 base P/T and
	// an "As ~ enters, choose" ability (detected via AST oracle text),
	// set a safe default so SBA 704.5f doesn't immediately kill it.
	// This covers cards like Primal Plasma, Primal Clay, Aquamorph
	// Entity, Corrupted Shapeshifter, etc.
	isCreature := false
	for _, t := range c.Types {
		if t == "creature" {
			isCreature = true
			break
		}
	}
	if isCreature && c.BasePower == 0 && c.BaseToughness == 0 {
		ot := gameengine.OracleTextLower(c)
		if gameengine.HasETBChoicePatternExported(ot) {
			c.BasePower = 3
			c.BaseToughness = 3
		}
	}

	return c
}
