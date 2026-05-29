// Package counters implements the Counter DB foundation per
// docs/counter-db-implementation-plan-r60.md.
//
// Phase 1 scope: registry types + the 10 most common counter types, API
// skeleton (AddCounters / RemoveCounters / CounterCount), §122.1g doubling
// pipeline placeholder, §122.6 persistence pinned via property test.
//
// The package is deliberately gameengine-independent. Callers in
// internal/gameengine/ adapt their *Permanent to the Target interface so
// later phases can wire AddCounters into the engine without creating an
// import cycle. Phase 2+ layers on keyword counter ability grants, SBA
// pair-removal (§704.5r), proliferate (CR §701.27), the full replacement-
// effect doubling pipeline (§122.1g), and the long-tail 242 remaining
// counter types catalogued in docs/counter-types-catalog-r60.md.
package counters

// CounterCategory classifies a counter type for invariant and AI use.
type CounterCategory int

const (
	// CategoryUnknown is the zero value; registry lookups never return it.
	CategoryUnknown CounterCategory = iota
	// StatModifier covers +1/+1, -1/-1, and any +N/+M printed variant.
	StatModifier
	// KeywordGrant covers counters whose presence grants a keyword
	// ability per CR §122.1c (lifelink, deathtouch, flying, etc.).
	KeywordGrant
	// ResourceMarker covers depletable charge-shaped trackers placed by
	// activated/triggered abilities (charge, stun, shield, oil).
	ResourceMarker
	// LoreCounter is the saga chapter marker per CR §714.
	LoreCounter
	// LoyaltyCounter is the planeswalker resource per CR §306.
	LoyaltyCounter
	// TimeCounter covers suspend / fade / age trackers per §702.61.
	TimeCounter
	// OtherTracker is the catch-all for rules-text-specific markers.
	OtherTracker
)

// String returns the lowercased identifier used in serialized output.
func (c CounterCategory) String() string {
	switch c {
	case StatModifier:
		return "stat_modifier"
	case KeywordGrant:
		return "keyword_grant"
	case ResourceMarker:
		return "resource_marker"
	case LoreCounter:
		return "lore_counter"
	case LoyaltyCounter:
		return "loyalty_counter"
	case TimeCounter:
		return "time_counter"
	case OtherTracker:
		return "other_tracker"
	}
	return "unknown"
}

// TargetType enumerates the kinds of game objects a counter type may be
// placed on. Validated against the registry's ValidTargets per CR §122.1.
type TargetType int

const (
	TargetCreature TargetType = iota
	TargetPlaneswalker
	TargetArtifact
	TargetEnchantment
	TargetLand
	TargetBattle
	TargetSaga
	TargetPlayer
)

// String returns the lowercased identifier used in serialized output and
// error messages.
func (t TargetType) String() string {
	switch t {
	case TargetCreature:
		return "creature"
	case TargetPlaneswalker:
		return "planeswalker"
	case TargetArtifact:
		return "artifact"
	case TargetEnchantment:
		return "enchantment"
	case TargetLand:
		return "land"
	case TargetBattle:
		return "battle"
	case TargetSaga:
		return "saga"
	case TargetPlayer:
		return "player"
	}
	return "unknown"
}

// PlacementRule is a bitmask of the legal placement contexts for a counter
// type. A counter type may permit multiple contexts (e.g. +1/+1 enters via
// ETB, ability resolution, AND proliferate).
type PlacementRule uint8

const (
	// PlaceEnterCondition — placed as part of ETB ("enters with N counters").
	PlaceEnterCondition PlacementRule = 1 << iota
	// PlaceAbilityCounter — placed by a triggered or activated ability.
	PlaceAbilityCounter
	// PlaceAbilityCost — placed/removed as a cost (loyalty, charge spend).
	PlaceAbilityCost
	// PlaceProliferateOnly — only legal to receive via proliferate.
	PlaceProliferateOnly
	// PlaceAutoUpkeep — placed automatically at a phase step (saga lore,
	// time-counter tick).
	PlaceAutoUpkeep
)

// Allows reports whether the rule set permits the given placement context.
func (p PlacementRule) Allows(ctx PlacementRule) bool {
	return p&ctx != 0
}

// StackingRule captures the §704.5r-style pair-removal behavior. The zero
// value (NoPair) means counters of this type never cancel against another
// counter type during the SBA pass.
type StackingRule struct {
	// PairsWith names another counter type that cancels equal counts of
	// this type during the SBA pass. Empty string for NoPair.
	PairsWith string
}

// NoPair is the zero StackingRule — explicit for readability at registry
// construction sites.
var NoPair = StackingRule{}

// PairsWith returns a StackingRule that cancels with the named counter type.
func PairsWith(other string) StackingRule {
	return StackingRule{PairsWith: other}
}

// AbilityRef points at a granted ability per CR §122.1c. The Phase 1 form
// only carries a Keyword string; Phase 2 extends to full ability shapes
// (triggered/static patterns) as the keyword-counter family lands.
type AbilityRef struct {
	Keyword string
}

// TriggerSpec is the Phase 1 placeholder for OnPlaced / OnRemoved triggers.
// Pattern is a free-form identifier the engine's trigger dispatcher will
// pattern-match against in Phase 2+. No dispatch happens in Phase 1.
type TriggerSpec struct {
	Pattern string
}

// CounterTypeDef is the registry entry for a single counter type. Every
// field traces back to a CR §-citation in Notes.
type CounterTypeDef struct {
	Name             string
	Aliases          []string
	Category         CounterCategory
	ValidTargets     []TargetType
	Placement        PlacementRule
	DoublingApplies  bool
	Proliferate      bool
	StackingBehavior StackingRule
	GrantedAbility   *AbilityRef
	OnPlacedTrigger  []TriggerSpec
	OnRemovedTrigger []TriggerSpec
	Notes            string
}

// AcceptsTarget reports whether the counter type's ValidTargets includes
// the given target type. A nil receiver returns false defensively (caller
// shouldn't reach this with a missing registry entry, but the bound keeps
// the API safe).
func (d *CounterTypeDef) AcceptsTarget(t TargetType) bool {
	if d == nil {
		return false
	}
	for _, vt := range d.ValidTargets {
		if vt == t {
			return true
		}
	}
	return false
}

// registry is the populated map; reads only after init(). Mutation outside
// init paths is forbidden — the registry is process-lifetime immutable.
var registry = map[string]*CounterTypeDef{}

// aliasIndex maps Aliases → canonical name. Built by registerDefinition.
var aliasIndex = map[string]string{}

// registerDefinition installs a CounterTypeDef into the registry. Called
// only from registry_init.go's init(). Panics on duplicate names so the
// startup failure is loud.
func registerDefinition(def *CounterTypeDef) {
	if def == nil || def.Name == "" {
		panic("counters: registerDefinition called with nil or unnamed definition")
	}
	if _, exists := registry[def.Name]; exists {
		panic("counters: duplicate counter type " + def.Name)
	}
	registry[def.Name] = def
	for _, alias := range def.Aliases {
		if alias == "" {
			continue
		}
		if existing, dup := aliasIndex[alias]; dup {
			panic("counters: alias " + alias + " already maps to " + existing)
		}
		aliasIndex[alias] = def.Name
	}
}

// Lookup returns the CounterTypeDef for the given name or one of its
// aliases. Returns nil if no registry entry matches.
func Lookup(name string) *CounterTypeDef {
	if def, ok := registry[name]; ok {
		return def
	}
	if canonical, ok := aliasIndex[name]; ok {
		return registry[canonical]
	}
	return nil
}

// CanonicalName returns the registry's canonical name for an alias input,
// or the input unchanged if it's already canonical / unregistered. Use
// this to normalize counter type strings before storage so equal counters
// aggregate correctly.
func CanonicalName(name string) string {
	if _, ok := registry[name]; ok {
		return name
	}
	if canonical, ok := aliasIndex[name]; ok {
		return canonical
	}
	return name
}

// All returns a snapshot slice of every registered CounterTypeDef. Order
// is unspecified; callers that need determinism should sort by Name.
func All() []*CounterTypeDef {
	out := make([]*CounterTypeDef, 0, len(registry))
	for _, def := range registry {
		out = append(out, def)
	}
	return out
}
