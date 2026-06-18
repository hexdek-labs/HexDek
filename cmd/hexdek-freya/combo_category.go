package main

// Three-axis combo / value / synergy classification (r63).
//
// This file defines the DATA MODEL only (Phase 0). Population logic lands in
// later phases:
//
//	Phase 1 — COMBO = intrinsic compounding math (combo_math.go), populates
//	          FreyaReport.Combos.
//	Phase 2 — VALUE = deck-independent advantage baseline, populates
//	          FreyaReport.ValueEngines.
//	Phase 3 — SYNERGY = commander-relative fit, populates
//	          FreyaReport.SynergyPackages.
//
// See /tmp/fable-review/freya-classification-architecture-r63.md for the full
// design. The three axes are co-taggable on the VALUE/SYNERGY side; COMBO is
// its own axis (intrinsic math, deck-independent). Everything here is purely
// additive — the legacy LoopType buckets (TrueInfinites/Determined/Finishers/
// Synergies) are untouched and remain the source of truth for existing
// consumers (bracket scoring, current JSON).

// Category tags carried on ComboResult.Categories.
const (
	CategoryCombo   = "combo"   // intrinsic compounding-math assembly or self-contained loop
	CategoryValue   = "value"   // card / mana / board advantage (deck-independent)
	CategorySynergy = "synergy" // advances THIS commander's plan (context-dependent)
)

// MathOp kinds — intrinsic compounding-math operations. A COMBO is built from
// these (≥2 assembled, an enabler feeding one, or a self-contained loop).
const (
	MathOpDoubler        = "doubler"         // ×2 a resource (damage/tokens/counters/life/mana/triggers)
	MathOpForEach        = "for_each"        // output scales by a board / permanent / counter count
	MathOpCostReduction  = "cost_reduction"  // reduces a cost toward free / enables looping casts
	MathOpManaMultiplier = "mana_multiplier" // multiplies mana production
	MathOpAdditiveEngine = "additive_engine" // per-trigger accumulation that compounds
)

// MathOp source seam (Q1: Thor AST cross-referenced with Freya oracle
// analysis). MathSignatures are built behind this seam so the parse source is
// swappable / unionable.
const (
	MathSourceOracle = "oracle"
	MathSourceAST    = "ast"
)

// MathOp is one detected compounding-math operation on a single card.
type MathOp struct {
	Kind     string // one of the MathOp* kinds
	Resource string // coarse resource the math acts on: damage/tokens/counters/mana/life/triggers/cards/power
	Detail   string // human-readable detail, e.g. "for each creature you control"
	Source   string // MathSourceOracle | MathSourceAST
}

// MathSignature is the deck-independent compounding-math profile of a card.
// A non-empty signature marks the card as a COMBO building block.
type MathSignature struct {
	Ops []MathOp
}

// HasCompoundingMath reports whether the card carries any intrinsic
// compounding-math operation.
func (m MathSignature) HasCompoundingMath() bool { return len(m.Ops) > 0 }

// ValueProfile is the deck-independent advantage baseline (VALUE axis).
// Populated in Phase 2.
type ValueProfile struct {
	Advantage []string // subset of {"card","mana","board","life"}
	Symmetry  string   // "asymmetric" | "symmetric" | "n/a"
	OneShot   bool     // one-time effect vs repeatable
}

// SynergyFit is the commander-relative fit (SYNERGY axis). Populated in
// Phase 3 from the existing commander-synergy / theme / cluster-role signals.
type SynergyFit struct {
	Score     float64  // 0..1 commander-relative fit
	Themes    []string // which commander themes this advances
	Role      string   // "enabler" | "payoff" | "engine" | "support" | "none"
	Symmetric bool     // symmetric theme-supporting effect (everyone sacrifices, group hug)
}
