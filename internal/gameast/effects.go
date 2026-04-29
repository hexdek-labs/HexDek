package gameast

// Effect is the sum type for all effect nodes. Every concrete effect type
// below implements this interface.
//
// Kind() returns the same string the Python AST's `kind` discriminator
// returns — this is what the resolver switches on.
//
// Mirrors scripts/mtg_ast.py :: EffectNode (and all subclasses).
type Effect interface {
	Kind() string
	isEffect()
}

// baseEffect is embedded in every effect struct to satisfy isEffect().
type baseEffect struct{}

func (baseEffect) isEffect() {}

// -----------------------------------------------------------------------------
// NumberOrRef — discriminated union for amount/count fields.
// -----------------------------------------------------------------------------

// NumberOrRef represents the Python `Union[int, str, ScalingAmount]` pattern
// used on `amount` and `count` fields across many effect types.
//
// Exactly one of (IsInt, IsStr, IsScaling) is set. Use the kind methods for
// safe access.
//
// JSON shape (as emitted by scripts/export_ast_dataset.py):
//
//	3                                        → IsInt = true, Int = 3
//	"x" / "var"                              → IsStr = true, Str = "x"
//	{"__ast_type__": "ScalingAmount", ...}   → IsScaling = true, Scaling = *ScalingAmount
type NumberOrRef struct {
	IsInt     bool
	Int       int
	IsStr     bool
	Str       string
	IsScaling bool
	Scaling   *ScalingAmount
}

// IntVal returns (value, ok) if the NumberOrRef is a plain integer.
func (n *NumberOrRef) IntVal() (int, bool) {
	if n == nil || !n.IsInt {
		return 0, false
	}
	return n.Int, true
}

// StrVal returns (value, ok) if the NumberOrRef is a string literal ("x", "var").
func (n *NumberOrRef) StrVal() (string, bool) {
	if n == nil || !n.IsStr {
		return "", false
	}
	return n.Str, true
}

// ScalingVal returns (value, ok) if the NumberOrRef is a ScalingAmount.
func (n *NumberOrRef) ScalingVal() (*ScalingAmount, bool) {
	if n == nil || !n.IsScaling {
		return nil, false
	}
	return n.Scaling, true
}

// NumInt constructs a NumberOrRef carrying an integer literal.
func NumInt(v int) *NumberOrRef { return &NumberOrRef{IsInt: true, Int: v} }

// NumStr constructs a NumberOrRef carrying a string literal ("x", "var").
func NumStr(v string) *NumberOrRef { return &NumberOrRef{IsStr: true, Str: v} }

// NumScaling constructs a NumberOrRef carrying a ScalingAmount reference.
func NumScaling(v *ScalingAmount) *NumberOrRef { return &NumberOrRef{IsScaling: true, Scaling: v} }

// -----------------------------------------------------------------------------
// ScalingAmount — dynamic integer expression evaluated at resolution time.
// -----------------------------------------------------------------------------

// ScalingAmount is a dynamic integer expression evaluated against game state
// at resolution. Used for "equal to your devotion to black", "equal to the
// number of creatures you control", etc.
//
// Canonical kinds (engine resolver must handle each):
//
//   - devotion                 args=(color,) or (color1, color2, ...)
//   - creatures_you_control    args=(filter,)
//   - permanents_you_control   args=(filter,)
//   - cards_in_zone            args=(zone, whose)
//   - life_lost_this_way       args=()
//   - life_gained_this_turn    args=(whose,)
//   - counters_on_self         args=(counter_kind,)
//   - x                        args=()
//   - literal                  args=(int,)
//   - raw                      args=(text,)
//
// Mirrors scripts/mtg_ast.py :: ScalingAmount.
type ScalingAmount struct {
	ScalingKind string        `json:"kind"`
	Args        []interface{} `json:"args,omitempty"`
}

// -----------------------------------------------------------------------------
// Control-flow effect nodes.
// -----------------------------------------------------------------------------

// Sequence is a comma- or period-joined list of effects executed in order.
type Sequence struct {
	baseEffect
	Items []Effect `json:"items"`
}

func (Sequence) Kind() string { return "sequence" }

// Choice models "choose one — A; B; C". Pick is the required number of
// options (1 for most modal spells, 2 for kicker modes, "all" for entwine-
// style, or "x" for X-based).
type Choice struct {
	baseEffect
	Options []Effect    `json:"options"`
	Pick    NumberOrRef `json:"pick"`
	OrMore  bool        `json:"or_more,omitempty"`
}

func (Choice) Kind() string { return "choice" }

// Optional_ is "You may [effect]." — an optional effect with no opportunity cost.
type Optional_ struct {
	baseEffect
	Body Effect `json:"body,omitempty"`
}

func (Optional_) Kind() string { return "optional" }

// Conditional is "If [condition], [body]; otherwise [else_body]."
type Conditional struct {
	baseEffect
	Condition *Condition `json:"condition,omitempty"`
	Body      Effect     `json:"body,omitempty"`
	ElseBody  Effect     `json:"else_body,omitempty"`
}

func (Conditional) Kind() string { return "conditional" }

// -----------------------------------------------------------------------------
// Leaf effect nodes.
// -----------------------------------------------------------------------------

// Damage models damage-dealing effects.
type Damage struct {
	baseEffect
	Amount  NumberOrRef `json:"amount"`
	Target  Filter      `json:"target"`
	Divided bool        `json:"divided,omitempty"`
}

func (Damage) Kind() string { return "damage" }

// Draw models card-draw effects.
type Draw struct {
	baseEffect
	Count  NumberOrRef `json:"count"`
	Target Filter      `json:"target"`
}

func (Draw) Kind() string { return "draw" }

// Discard models hand-discard effects.
type Discard struct {
	baseEffect
	Count    NumberOrRef `json:"count"`
	Target   Filter      `json:"target"`
	ChosenBy string      `json:"chosen_by,omitempty"` // "controller"/"discarder"/"random"
}

func (Discard) Kind() string { return "discard" }

// Mill models library-to-graveyard effects.
type Mill struct {
	baseEffect
	Count  NumberOrRef `json:"count"`
	Target Filter      `json:"target"`
}

func (Mill) Kind() string { return "mill" }

// Scry models scry N.
type Scry struct {
	baseEffect
	Count NumberOrRef `json:"count"`
}

func (Scry) Kind() string { return "scry" }

// Surveil models surveil N.
type Surveil struct {
	baseEffect
	Count NumberOrRef `json:"count"`
}

func (Surveil) Kind() string { return "surveil" }

// CounterSpell models counterspell effects.
type CounterSpell struct {
	baseEffect
	Target Filter `json:"target"`
	Unless *Cost  `json:"unless,omitempty"`
}

func (CounterSpell) Kind() string { return "counter_spell" }

// Destroy models destroy target X.
type Destroy struct {
	baseEffect
	Target Filter `json:"target"`
}

func (Destroy) Kind() string { return "destroy" }

// Exile models exile target X.
type Exile struct {
	baseEffect
	Target   Filter `json:"target"`
	Until    string `json:"until,omitempty"` // "next_end_step"/"leaves_battlefield"/""
	FaceDown bool   `json:"face_down,omitempty"`
}

func (Exile) Kind() string { return "exile" }

// Bounce models return-to-hand / library effects.
type Bounce struct {
	baseEffect
	Target Filter `json:"target"`
	To     string `json:"to,omitempty"` // "owners_hand" / "top_of_library" / "bottom_of_library"
}

func (Bounce) Kind() string { return "bounce" }

// Tutor models library-searching effects.
type Tutor struct {
	baseEffect
	Query        Filter      `json:"query"`
	Destination  string      `json:"destination,omitempty"` // "hand"/"battlefield"/"battlefield_tapped"/"graveyard"/"top_of_library"
	Count        NumberOrRef `json:"count"`
	Optional     bool        `json:"optional,omitempty"`
	ShuffleAfter bool        `json:"shuffle_after,omitempty"`
	Reveal       bool        `json:"reveal,omitempty"`
	Rest         string      `json:"rest,omitempty"` // "bottom"/"graveyard"/"exile"/""
}

func (Tutor) Kind() string { return "tutor" }

// Reanimate models graveyard-to-battlefield effects.
type Reanimate struct {
	baseEffect
	Query             Filter   `json:"query"`
	FromZone          string   `json:"from_zone,omitempty"`    // "your_graveyard" / "any_graveyard"
	Destination       string   `json:"destination,omitempty"`  // "battlefield" / "battlefield_tapped"
	Controller        string   `json:"controller,omitempty"`   // "you" / "owner"
	WithModifications []string `json:"with_modifications,omitempty"`
}

func (Reanimate) Kind() string { return "reanimate" }

// Recurse models graveyard-to-hand effects.
type Recurse struct {
	baseEffect
	Query       Filter `json:"query"`
	FromZone    string `json:"from_zone,omitempty"`
	Destination string `json:"destination,omitempty"`
}

func (Recurse) Kind() string { return "recurse" }

// GainLife models life-gain effects.
type GainLife struct {
	baseEffect
	Amount NumberOrRef `json:"amount"`
	Target Filter      `json:"target"`
}

func (GainLife) Kind() string { return "gain_life" }

// LoseLife models life-loss effects.
type LoseLife struct {
	baseEffect
	Amount NumberOrRef `json:"amount"`
	Target Filter      `json:"target"`
}

func (LoseLife) Kind() string { return "lose_life" }

// SetLife models "your life total becomes N".
type SetLife struct {
	baseEffect
	Amount NumberOrRef `json:"amount"`
	Target Filter      `json:"target"`
}

func (SetLife) Kind() string { return "set_life" }

// Sacrifice models effect-driven sacrifice (not a cost-side sacrifice).
type Sacrifice struct {
	baseEffect
	Query Filter `json:"query"`
	Actor string `json:"actor,omitempty"` // "controller" / "target_player" / "each_opponent"
}

func (Sacrifice) Kind() string { return "sacrifice" }

// CreateToken models token-creation effects.
type CreateToken struct {
	baseEffect
	Count     NumberOrRef `json:"count"`
	PT        *[2]int     `json:"pt,omitempty"`
	Types     []string    `json:"types,omitempty"`
	Color     []string    `json:"color,omitempty"`
	Keywords  []string    `json:"keywords,omitempty"`
	IsCopyOf  *Filter     `json:"is_copy_of,omitempty"`
	Legendary bool        `json:"legendary,omitempty"`
	Tapped    bool        `json:"tapped,omitempty"`
}

func (CreateToken) Kind() string { return "create_token" }

// CounterMod models put/remove/double/move counters.
type CounterMod struct {
	baseEffect
	Op          string      `json:"op,omitempty"` // "put"/"remove"/"double"/"move"
	Count       NumberOrRef `json:"count"`
	CounterKind string      `json:"counter_kind,omitempty"` // "+1/+1", "-1/-1", "charge", etc.
	Target      Filter      `json:"target"`
}

func (CounterMod) Kind() string { return "counter_mod" }

// Buff models "+N/+N until end of turn" and anthem static effects.
type Buff struct {
	baseEffect
	Power     int    `json:"power"`
	Toughness int    `json:"toughness"`
	Target    Filter `json:"target"`
	Duration  string `json:"duration,omitempty"` // "until_end_of_turn" / "permanent" / "until_your_next_turn"
}

func (Buff) Kind() string { return "buff" }

// GrantAbility models "target creature gains flying until end of turn",
// anthems like "creatures you control have vigilance", etc.
type GrantAbility struct {
	baseEffect
	AbilityName string `json:"ability_name"`
	Target      Filter `json:"target"`
	Duration    string `json:"duration,omitempty"`
}

func (GrantAbility) Kind() string { return "grant_ability" }

// TapEffect models effect-driven tap (not cost).
type TapEffect struct {
	baseEffect
	Target Filter `json:"target"`
}

func (TapEffect) Kind() string { return "tap" }

// UntapEffect models effect-driven untap.
type UntapEffect struct {
	baseEffect
	Target Filter `json:"target"`
}

func (UntapEffect) Kind() string { return "untap" }

// AddMana models mana-ability output.
type AddMana struct {
	baseEffect
	Pool          []ManaSymbol `json:"pool,omitempty"`
	AnyColorCount int          `json:"any_color_count,omitempty"`
}

func (AddMana) Kind() string { return "add_mana" }

// GainControl models control-change effects.
type GainControl struct {
	baseEffect
	Target   Filter `json:"target"`
	Duration string `json:"duration,omitempty"`
}

func (GainControl) Kind() string { return "gain_control" }

// CopySpell models fork-style spell copies.
type CopySpell struct {
	baseEffect
	Target              Filter `json:"target"`
	MayChooseNewTargets bool   `json:"may_choose_new_targets,omitempty"`
}

func (CopySpell) Kind() string { return "copy_spell" }

// CopyPermanent models clone effects.
type CopyPermanent struct {
	baseEffect
	Target  Filter `json:"target"`
	AsToken bool   `json:"as_token,omitempty"`
}

func (CopyPermanent) Kind() string { return "copy_permanent" }

// Fight models fight and bite effects.
// OneSided=false (default): both creatures deal damage (CR §701.12, "fight").
// OneSided=true: only A deals damage to B ("deals damage equal to its power to", "bite").
type Fight struct {
	baseEffect
	A        Filter `json:"a"`
	B        Filter `json:"b"`
	OneSided bool   `json:"one_sided,omitempty"`
}

func (Fight) Kind() string { return "fight" }

// Reveal models reveal-from-zone effects.
type Reveal struct {
	baseEffect
	Source string      `json:"source,omitempty"` // "your_hand"/"top_of_library"/"graveyard"/"exile"
	Count  NumberOrRef `json:"count"`
	Actor  string      `json:"actor,omitempty"`
}

func (Reveal) Kind() string { return "reveal" }

// LookAt models look-at-hidden-zone effects.
type LookAt struct {
	baseEffect
	Target Filter      `json:"target"`
	Zone   string      `json:"zone,omitempty"` // "hand"/"library_top_n"
	Count  NumberOrRef `json:"count"`
}

func (LookAt) Kind() string { return "look_at" }

// Shuffle models shuffle-library effects.
type Shuffle struct {
	baseEffect
	Target Filter `json:"target"`
}

func (Shuffle) Kind() string { return "shuffle" }

// ExtraTurn models extra-turn effects.
type ExtraTurn struct {
	baseEffect
	AfterThis bool   `json:"after_this,omitempty"`
	Target    Filter `json:"target"`
}

func (ExtraTurn) Kind() string { return "extra_turn" }

// ExtraCombat models extra-combat-phase effects.
type ExtraCombat struct {
	baseEffect
	AfterThis bool `json:"after_this,omitempty"`
}

func (ExtraCombat) Kind() string { return "extra_combat" }

// WinGame models "target player wins the game" (Coalition Victory etc.).
type WinGame struct {
	baseEffect
	Target Filter `json:"target"`
}

func (WinGame) Kind() string { return "win_game" }

// LoseGame models "target player loses the game".
type LoseGame struct {
	baseEffect
	Target Filter `json:"target"`
}

func (LoseGame) Kind() string { return "lose_game" }

// Replacement models "If [event] would happen, [replacement] instead."
// Only used as a leaf effect; for full §614 framework see internal/gameengine.
type Replacement struct {
	baseEffect
	TriggerEvent string `json:"trigger_event"`
	ReplaceWith  Effect `json:"replacement"`
}

func (Replacement) Kind() string { return "replacement" }

// Prevent models damage-prevention effects.
type Prevent struct {
	baseEffect
	Amount       NumberOrRef `json:"amount"`
	DamageFilter *Filter     `json:"damage_filter,omitempty"`
	Duration     string      `json:"duration,omitempty"`
}

func (Prevent) Kind() string { return "prevent" }

// UnknownEffect is the catch-all for effect text the parser could not
// structure into a typed node. The engine's resolver should log and skip,
// never panic. RawText preserves the original oracle text fragment so
// per-card snowflake handlers can pattern-match on it.
type UnknownEffect struct {
	baseEffect
	RawText string `json:"raw_text"`
}

func (UnknownEffect) Kind() string { return "unknown" }

// ModificationEffect is a Modification node used at an effect position.
// The Python parser's Wave 1a promotions emit Modification(kind="...", args=(...))
// as effects for labelled-but-not-yet-typed shapes (goad, phase_out_self,
// stun_target_next_untap, investigate, suspect, no_life_gained, etc.).
// The Go resolver dispatches on ModKind to handle each shape.
type ModificationEffect struct {
	baseEffect
	ModKind string        `json:"kind"`
	Args    []interface{} `json:"args,omitempty"`
}

func (ModificationEffect) Kind() string { return "modification_effect" }
