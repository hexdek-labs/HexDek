package gameast

// Filter is a target/filter expression. Covers "target creature you control",
// "each opponent", "a basic Plains card", "two target nonland permanents",
// etc. Stored structurally so two filters with the same shape compare equal.
//
// Mirrors scripts/mtg_ast.py :: Filter.
type Filter struct {
	// Base is the core type: "creature" / "land" / "permanent" / "spell" /
	// "player" / "opponent" / "card" / "any_target" / etc.
	Base string `json:"base"`

	// Quantifier: "one" / "n" / "all" / "each" / "any" / "up_to_n" / "each_player".
	Quantifier string `json:"quantifier,omitempty"`

	// Count: an integer, or the string "x" for X-costs, or nil.
	// Stored as a NumberOrRef so it can carry a ScalingAmount if the parser
	// emits one (rare but possible — "target N creatures, where N is ...").
	Count *NumberOrRef `json:"count,omitempty"`

	// Targeted is true for "target X" phrasing, false for "a/an X" (untargeted).
	Targeted bool `json:"targeted"`

	// Control flags: set by "you control" / "opponent controls" modifiers.
	YouControl       bool `json:"you_control,omitempty"`
	OpponentControls bool `json:"opponent_controls,omitempty"`

	// Other adjectives.
	NonToken      bool     `json:"nontoken,omitempty"`
	CreatureTypes []string `json:"creature_types,omitempty"`
	ColorFilter   []string `json:"color_filter,omitempty"`
	ColorExclude  []string `json:"color_exclude,omitempty"`

	// ManaValueOp / ManaValue: "<=", ">=", "==", or empty.
	ManaValueOp string `json:"mana_value_op,omitempty"`
	ManaValue   *int   `json:"mana_value,omitempty"`

	// Extra adjectives the parser didn't normalize (e.g. "attacking", "tapped").
	Extra []string `json:"extra,omitempty"`
}

// Built-in shorthand filters, matching the Python module-level constants.
// These are returned by value from the functions below — treat as immutable.
// (Go doesn't have true immutable struct values; we use functions to deter
// mutation of a package-level global.)

// TargetAny returns a Filter matching "any target".
func TargetAny() Filter { return Filter{Base: "any_target", Targeted: true} }

// TargetCreature returns "target creature".
func TargetCreature() Filter { return Filter{Base: "creature", Targeted: true} }

// TargetPlayer returns "target player".
func TargetPlayer() Filter { return Filter{Base: "player", Targeted: true} }

// TargetOpponent returns "target opponent".
func TargetOpponent() Filter { return Filter{Base: "opponent", Targeted: true} }

// EachOpponent returns "each opponent" (untargeted, fan-out).
func EachOpponent() Filter {
	return Filter{Base: "opponent", Quantifier: "each", Targeted: false}
}

// EachPlayer returns "each player".
func EachPlayer() Filter {
	return Filter{Base: "player", Quantifier: "each", Targeted: false}
}

// Self returns the "self" filter (the source permanent/spell).
func Self() Filter { return Filter{Base: "self", Targeted: false} }
