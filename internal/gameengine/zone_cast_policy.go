package gameengine

// Zone-cast policy extension — R55.
//
// Complements the existing per-Card ZoneCastPermission registry
// (gs.ZoneCastGrants) with a per-FILTER policy registry
// (gs.ZoneCastPolicies). Per-Card grants are populated when the
// engine knows which specific *Card pointer is castable (e.g.,
// Eruth exiles the top two and stamps each); per-Filter policies
// are populated when the GRANT applies to any card matching a
// predicate (e.g., Aluren — "Any player may cast creature spells
// with mana value 3 or less without paying their mana cost").
//
// Policies are consulted by FindZoneCastPolicy at cast-legality
// check time. The hat / AI / Cast-from-zone path uses this to ask
// "can seat S cast card C from zone Z?" without the engine having
// to pre-register every matching card pointer.
//
// CR citations covered:
//
//   §112.6k    Static "you may cast this card from [zone]" — but
//              the filter-driven variant ("any player may cast
//              creature spells with mana value 3 or less...").
//   §117.9a    Casting from opponent's zones (Tinybones, Sen
//              Triplets, Gonti Lord of Luxury) — caster scope of
//              "opponents" lets the controller cast from an opp's
//              hand / graveyard.
//   §903.10b   Wishboard / outside-the-game (Karn the Great
//              Creator, Glittering Wish) — zone = "outside_the_game".

// ZoneCastPolicy is a filter-driven zone-cast permission. Unlike
// ZoneCastPermission (which is registered per *Card pointer in
// gs.ZoneCastGrants), a policy applies to any card matching its
// predicate at lookup time.
type ZoneCastPolicy struct {
	// SourcePerm is the permanent whose ability grants this
	// permission. UnregisterZoneCastPoliciesForPermanent matches on
	// this pointer.
	SourcePerm *Permanent

	// HandlerID — diagnostic tag.
	HandlerID string

	// Zone — source zone the cast must come FROM. Standard values
	// are ZoneHand / ZoneGraveyard / ZoneExile / ZoneLibrary /
	// "outside_the_game" for wishboard, "library_top" for top-only
	// (Bolas's Citadel pattern).
	Zone string

	// OwnerScope filters by who OWNS the card being cast. Values:
	//   "any"            — any card in the named zone matches
	//   "self"           — owner is the policy's source controller
	//   "caster"         — owner is the casting seat (per-caster
	//                       symmetry — Aluren: each player casts
	//                       from their OWN hand)
	//   "opponents"      — owner is an opponent of the SOURCE's
	//                       controller (Tinybones — controller casts
	//                       from opp's graveyard)
	//
	// Default behavior when empty: "any".
	OwnerScope string

	// CasterScope filters by WHICH PLAYER may use the permission.
	//   "any"        — every seat (Aluren — "any player may cast")
	//   "controller" — only the source's controller
	//   "opponents"  — only opponents of the source's controller
	//
	// Default behavior when empty: "controller".
	CasterScope string

	// ControllerSeat is the source's controller seat. Used to
	// resolve CasterScope/OwnerScope against the right anchor.
	ControllerSeat int

	// Predicate filters by the card's properties. Returns true if
	// the card matches the policy. nil = always matches.
	Predicate func(*Card) bool

	// ManaCost override. -1 = use the card's normal mana cost
	// (default), 0 = free cast (Aluren — "without paying their mana
	// cost"), N >= 1 = pay N generic mana instead.
	ManaCost int

	// LifeCostInsteadOfMana: pay this much life rather than mana
	// (Bolas's Citadel — pay life equal to mana value).
	LifeCostInsteadOfMana int

	// SpendAnyColor — CR §106.11. True for Gonti / Sen Triplets /
	// Tinybones / Hostage Taker.
	SpendAnyColor bool

	// ExileOnResolve — true for flashback / escape-style
	// "exile rather than graveyard" routing.
	ExileOnResolve bool

	// Duration — "while_source_on_bf" (static while source is in
	// play), "until_end_of_turn", "until_end_of_next_turn", or ""
	// for permanent.
	Duration string

	// SourceTimestamp — Permanent.Timestamp of the source. Used
	// by while_source_on_bf cleanup.
	SourceTimestamp int

	// GrantTurn — the turn this policy was created.
	GrantTurn int
}

// RegisterZoneCastPolicy appends a policy to the registry.
func (gs *GameState) RegisterZoneCastPolicy(p *ZoneCastPolicy) {
	if gs == nil || p == nil {
		return
	}
	gs.ZoneCastPolicies = append(gs.ZoneCastPolicies, p)
	gs.LogEvent(Event{
		Kind:   "zone_cast_policy_registered",
		Seat:   p.ControllerSeat,
		Source: p.HandlerID,
		Details: map[string]interface{}{
			"zone":         p.Zone,
			"owner_scope":  p.OwnerScope,
			"caster_scope": p.CasterScope,
			"mana_cost":    p.ManaCost,
		},
	})
}

// UnregisterZoneCastPoliciesForPermanent drops every policy whose
// SourcePerm == p. Called from LTB hooks so a bounced / exiled /
// destroyed source stops granting the alt-cast.
func (gs *GameState) UnregisterZoneCastPoliciesForPermanent(p *Permanent) {
	if gs == nil || p == nil || len(gs.ZoneCastPolicies) == 0 {
		return
	}
	out := gs.ZoneCastPolicies[:0]
	for _, pol := range gs.ZoneCastPolicies {
		if pol != nil && pol.SourcePerm == p {
			gs.LogEvent(Event{
				Kind:   "zone_cast_policy_expired",
				Seat:   pol.ControllerSeat,
				Source: pol.HandlerID,
				Details: map[string]interface{}{
					"reason": "source_ltb",
				},
			})
			continue
		}
		out = append(out, pol)
	}
	for i := len(out); i < len(gs.ZoneCastPolicies); i++ {
		gs.ZoneCastPolicies[i] = nil
	}
	gs.ZoneCastPolicies = out
}

// FindZoneCastPolicy returns the first policy that allows `castingSeat`
// to cast `card` from `zone`. `cardOwnerSeat` is the seat that owns
// the card being cast (matters for OwnerScope filtering — e.g.
// Tinybones casting from opp's graveyard requires the card's owner to
// be an opponent of the caster).
//
// Returns nil if no policy applies.
func FindZoneCastPolicy(gs *GameState, castingSeat int, card *Card, cardOwnerSeat int, zone string) *ZoneCastPolicy {
	if gs == nil || card == nil || castingSeat < 0 || castingSeat >= len(gs.Seats) {
		return nil
	}
	for _, p := range gs.ZoneCastPolicies {
		if p == nil {
			continue
		}
		if p.Zone != zone {
			continue
		}
		if !zoneCastPolicyCasterMatches(p, castingSeat) {
			continue
		}
		if !zoneCastPolicyOwnerMatches(p, cardOwnerSeat) {
			continue
		}
		// "caster" owner-scope: per-caster symmetry (Aluren style).
		// Verified here rather than in the static-scope check so we
		// can reference castingSeat.
		if p.OwnerScope == "caster" && cardOwnerSeat != castingSeat {
			continue
		}
		if p.Predicate != nil && !p.Predicate(card) {
			continue
		}
		return p
	}
	return nil
}

func zoneCastPolicyCasterMatches(p *ZoneCastPolicy, castingSeat int) bool {
	switch p.CasterScope {
	case "", "controller":
		return castingSeat == p.ControllerSeat
	case "any":
		return true
	case "opponents":
		return castingSeat != p.ControllerSeat
	}
	return false
}

func zoneCastPolicyOwnerMatches(p *ZoneCastPolicy, cardOwnerSeat int) bool {
	switch p.OwnerScope {
	case "", "any":
		return true
	case "self":
		return cardOwnerSeat == p.ControllerSeat
	case "opponents":
		return cardOwnerSeat != p.ControllerSeat
	case "caster":
		// "caster" is resolved at lookup time by the caller via
		// zoneCastPolicyOwnerMatchesForCaster. The static check
		// here passes through; FindZoneCastPolicy applies the
		// caster-symmetry check separately.
		return true
	}
	return false
}

// PolicyToPermission converts a matched ZoneCastPolicy into a
// ZoneCastPermission for the existing CastFromZone pipeline. Callers
// who want to invoke the standard cast machinery can use this to
// bridge the new primitive into the old one.
func PolicyToPermission(p *ZoneCastPolicy, castingSeat int) *ZoneCastPermission {
	if p == nil {
		return nil
	}
	reqCtl := -1
	if p.CasterScope == "" || p.CasterScope == "controller" {
		reqCtl = p.ControllerSeat
	} else if p.CasterScope == "opponents" {
		reqCtl = castingSeat
	}
	return &ZoneCastPermission{
		Zone:                  p.Zone,
		Keyword:               p.HandlerID,
		ManaCost:              p.ManaCost,
		LifeCostInsteadOfMana: p.LifeCostInsteadOfMana,
		ExileOnResolve:        p.ExileOnResolve,
		RequireController:     reqCtl,
		SpendAnyColor:         p.SpendAnyColor,
		SourceName:            p.HandlerID,
		Duration:              p.Duration,
		SourceTimestamp:       p.SourceTimestamp,
		GrantTurn:             p.GrantTurn,
	}
}
