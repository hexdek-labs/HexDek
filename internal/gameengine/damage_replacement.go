package gameengine

// Damage replacement primitive — R54.
//
// CR §615 (prevention effects) is implemented separately via
// PreventDamageToPlayer / PreventDamageToPermanent. This file
// implements CR §614 damage REPLACEMENT effects: "If a source would
// deal damage to X, [instead Y]". Replacements are consulted before
// prevention shields per §616 application ordering.
//
// Currently wired to per-card replacements (Torbran, Sokrates,
// Lightning, Neriv, Kuja Flare-Star, etc.). The §614 multiple-
// replacement-effect ordering choice (controller of the affected
// object picks which to apply first) is approximated by registration
// order — when multiple replacements all apply, they fire in the
// order they were registered. This matches the deterministic AI
// rollout contract and is correct for the common case of one or two
// replacements being active at once.

// DamageKind discriminates the four damage-application surfaces the
// engine exposes. Combat vs noncombat matters for Sokrates-style
// "if this creature would deal COMBAT damage" filters; player vs
// creature matters for Torbran-style "to an opponent or a permanent
// an opponent controls" filters.
type DamageKind int

const (
	DamageCombatPlayer DamageKind = iota
	DamageCombatCreature
	DamageNonCombatPlayer
	DamageNonCombatCreature
	DamageNonCombatPlaneswalker
)

// DamageContext is the mutable replacement-effect probe passed to
// each registered DamageReplacementFn. Replacement fns may modify
// Amount (Torbran +2 / Kuja doubling) or set Prevented=true with
// an optional side effect (Sokrates dialogue prevent-and-draw).
//
// Source may be nil for damage from spells/abilities not tied to a
// battlefield permanent (e.g. instants like Lightning Bolt — in
// HexDek's model these still funnel through DealDamage with a
// SourceName string but no SourcePerm). Replacement fns that filter
// on source identity must nil-check.
//
// For player-target damage TargetSeat is the seat being damaged and
// TargetPerm is nil. For creature/planeswalker-target damage
// TargetPerm is the affected permanent and TargetSeat is the
// permanent's controller seat (a convenience for "deals damage to
// an opponent or a permanent an opponent controls" filters that
// only care about the eventual life total / counter target).
type DamageContext struct {
	Source     *Permanent
	SourceName string
	TargetSeat int
	TargetPerm *Permanent
	Kind       DamageKind
	Amount     int  // in/out — replacement fns mutate
	Prevented  bool // set true to fully prevent (Sokrates-style)
}

// DamageReplacementFn is a closure registered by a per-card handler
// at ETB and dropped at LTB. The fn inspects ctx, applies any
// replacement, and returns. Multiple fns may run in sequence; the
// first one to set ctx.Prevented short-circuits the chain.
type DamageReplacementFn func(gs *GameState, ctx *DamageContext)

// DamageReplacement bundles a closure with metadata for
// unregistration on LTB.
type DamageReplacement struct {
	// SourcePerm is the permanent whose ability creates the
	// replacement. UnregisterDamageReplacementsForPermanent matches
	// on this pointer.
	SourcePerm *Permanent

	// HandlerID is a string tag for diagnostics + the rare case where
	// a card needs multiple replacements registered under the same
	// SourcePerm. Empty is fine.
	HandlerID string

	// Fn is the replacement closure.
	Fn DamageReplacementFn
}

// RegisterDamageReplacement appends a replacement to the registry.
// Idempotency is up to the caller — registering the same effect
// twice will cause it to fire twice (matching the §614 stack
// semantics where two copies of the same replacement effect are
// both active).
func (gs *GameState) RegisterDamageReplacement(rep *DamageReplacement) {
	if gs == nil || rep == nil || rep.Fn == nil {
		return
	}
	gs.DamageReplacements = append(gs.DamageReplacements, rep)
}

// UnregisterDamageReplacementsForPermanent drops every replacement
// whose SourcePerm == p. Called from LTB hooks so a bounced /
// exiled / destroyed source doesn't keep replacing damage from the
// graveyard.
func (gs *GameState) UnregisterDamageReplacementsForPermanent(p *Permanent) {
	if gs == nil || p == nil || len(gs.DamageReplacements) == 0 {
		return
	}
	out := gs.DamageReplacements[:0]
	for _, rep := range gs.DamageReplacements {
		if rep != nil && rep.SourcePerm == p {
			continue
		}
		out = append(out, rep)
	}
	// Clear tail to release pointers.
	for i := len(out); i < len(gs.DamageReplacements); i++ {
		gs.DamageReplacements[i] = nil
	}
	gs.DamageReplacements = out
}

// ApplyDamageReplacement walks every registered replacement, calling
// fn(gs, ctx) in order. Each fn may mutate ctx.Amount, set
// ctx.Prevented, or both. The returned (amount, prevented) reflects
// the post-replacement state; callers should treat Prevented=true OR
// amount <= 0 as "no damage applied".
//
// Callers MUST pass a fully-populated DamageContext — every
// replacement fn is allowed to read every field. In particular,
// Source / SourceName / TargetSeat / TargetPerm / Kind / Amount
// must all be set before the call.
func ApplyDamageReplacement(gs *GameState, ctx *DamageContext) (int, bool) {
	if gs == nil || ctx == nil {
		return 0, false
	}
	for _, rep := range gs.DamageReplacements {
		if rep == nil || rep.Fn == nil {
			continue
		}
		rep.Fn(gs, ctx)
		if ctx.Prevented {
			break
		}
		if ctx.Amount <= 0 {
			ctx.Amount = 0
			break
		}
	}
	return ctx.Amount, ctx.Prevented
}
