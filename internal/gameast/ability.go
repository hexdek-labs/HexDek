package gameast

// Ability is the sum type for comp-rules §113 ability types: Static,
// Activated, Triggered, Keyword. Each implements Kind() string with the
// discriminator Python uses.
type Ability interface {
	Kind() string
	isAbility()
}

type baseAbility struct{}

func (baseAbility) isAbility() {}

// Modification is a static-ability body — anthems, replacement effects,
// restrictions, type-adds.
//
// Layer tags the §613 layer this modification operates in:
//
//	"1"  — copy effects
//	"2"  — control-changing
//	"3"  — text-changing
//	"4"  — type/subtype/supertype changes
//	"5"  — color-changing
//	"6"  — ability add/remove
//	"7a" — characteristic-defining P/T
//	"7b" — P/T set ("becomes N/N")
//	"7c" — P/T modify (anthems +N/+N)
//	"7d" — counters (+1/+1, -1/-1)
//	"7e" — P/T switching
//	""   — not a layered effect (costs, triggers, spell effects, timing)
//
// Mirrors scripts/mtg_ast.py :: Modification.
type Modification struct {
	ModKind string        `json:"kind"`
	Args    []interface{} `json:"args,omitempty"`
	Layer   string        `json:"layer,omitempty"`
}

// Static is an always-on ability.
type Static struct {
	baseAbility
	Condition    *Condition    `json:"condition,omitempty"`
	Modification *Modification `json:"modification,omitempty"`
	Raw          string        `json:"raw,omitempty"`
}

func (Static) Kind() string { return "static" }

// Activated is a "cost: effect" ability.
type Activated struct {
	baseAbility
	Cost               Cost   `json:"cost"`
	Effect             Effect `json:"effect"`
	TimingRestriction  string `json:"timing_restriction,omitempty"` // "sorcery" / "once_per_turn" / etc.
	Raw                string `json:"raw,omitempty"`
}

func (Activated) Kind() string { return "activated" }

// Triggered is a When/Whenever/At-the-beginning-of ability.
type Triggered struct {
	baseAbility
	Trigger        Trigger    `json:"trigger"`
	Effect         Effect     `json:"effect"`
	InterveningIf  *Condition `json:"intervening_if,omitempty"`
	Raw            string     `json:"raw,omitempty"`
}

func (Triggered) Kind() string { return "triggered" }

// Keyword is a named shorthand ability (Flying, Trample, Flashback, etc.).
// Args carries keyword-specific parameters (e.g. Flashback cost, Cycling cost).
type Keyword struct {
	baseAbility
	Name string        `json:"name"`
	Args []interface{} `json:"args,omitempty"`
	Raw  string        `json:"raw,omitempty"`
}

func (Keyword) Kind() string { return "keyword" }
