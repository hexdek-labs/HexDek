package gameast

// Trigger is the event side of a triggered ability.
//
// Mirrors scripts/mtg_ast.py :: Trigger.
type Trigger struct {
	// Event: "etb" / "die" / "ltb" / "attack" / "block" / "deal_combat_damage" /
	//        "cast" / "phase" / "untap" / "transform" / "discover" / etc.
	Event string `json:"event"`

	// Actor: what triggers it (e.g. "a creature you control").
	Actor *Filter `json:"actor,omitempty"`

	// TargetFilter: for damage/cast triggers, what was targeted/cast.
	TargetFilter *Filter `json:"target_filter,omitempty"`

	// Phase: for phase triggers: "upkeep" / "end_step" / "combat_start".
	Phase string `json:"phase,omitempty"`

	// Controller: "you" / "each" / "active_player".
	Controller string `json:"controller,omitempty"`

	// Condition: intervening "if" clause.
	Condition *Condition `json:"condition,omitempty"`
}

// Condition is a boolean condition like "if you control a Plains",
// "as long as you have 20 or more life", "if it had no -1/-1 counters on it".
//
// Kind discriminates the condition variant; Args carries kind-specific
// parameters. Canonical kinds (engine must handle each):
//
//   - you_control       : args may be (filter,) for "you control X"
//   - life_threshold    : args = (op, n)  e.g. (">=", 20)
//   - card_count_zone   : args = (zone, op, n)
//   - tribal            : args = (creature_type,)
//   - color_permanent   : args = (color,)
//   - raw               : args = (text,) — unparsed; engine should log
//
// Mirrors scripts/mtg_ast.py :: Condition.
type Condition struct {
	Kind string        `json:"kind"`
	Args []interface{} `json:"args,omitempty"`
}

// Cost is a composite cost for activated abilities (or additional costs on
// a spell). Every field is optional / zero-valued.
//
// Mirrors scripts/mtg_ast.py :: Cost.
type Cost struct {
	Mana              *ManaCost `json:"mana,omitempty"`
	Tap               bool      `json:"tap,omitempty"`
	Untap             bool      `json:"untap,omitempty"` // {Q}
	Sacrifice         *Filter   `json:"sacrifice,omitempty"`
	Discard           *int      `json:"discard,omitempty"`
	PayLife           *int      `json:"pay_life,omitempty"`
	ExileSelf         bool      `json:"exile_self,omitempty"`
	ReturnSelfToHand  bool      `json:"return_self_to_hand,omitempty"`
	RemoveCountersN   *int      `json:"remove_counters_n,omitempty"`
	RemoveCountersKnd string    `json:"remove_counters_kind,omitempty"`
	Extra             []string  `json:"extra,omitempty"`
}
