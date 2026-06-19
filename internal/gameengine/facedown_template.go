package gameengine

// facedown_template.go — the unified face-down OVERLAY registry (r63).
//
// DESIGN MODEL (CR §707.2 / §613.2b):
//
// A face-down permanent is the REAL card (the permanent of record —
// authoritative for zone moves, ownership, dies-triggers, commander
// identity) wearing a lightweight face-down OVERLAY. The overlay is NOT a
// token: a real token ceases to exist on leaving the battlefield per the
// §704.5d SBA, which would wrongly delete the underlying card. The card
// underneath must stay real, so the overlay is a characteristics mask
// (applied only while Card.FaceDown is set, on the battlefield) rather
// than a substitute object.
//
// The overlay's characteristics depend on WHAT turned the permanent face
// down — morph, disguise, cloak, manifest, or an external "turn face down"
// effect (Cyber Conversion / Ixidron). FaceDownTemplate captures that, and
// the registry below maps a template key to the §707.2 base the layers
// system applies (layers.go BaseCharacteristics).
//
// NOTE on the legacy wrapper model: morph / cloak / manifest historically
// wrap a synthetic "Face-Down Creature" *Card as perm.Card and stash the
// real card on perm.OriginalCard / FrontFaceAST. That predates this
// overlay model and is NOT the permanent-of-record design — migrating
// those three off the wrapper is tracked as a follow-on (too broad to land
// under one green-gate). Disguise and the new external-turn-down path
// already use the real-card overlay model. The registry + makeFaceDown
// below unify the FLAG + TEMPLATE stamping across all paths today, leaving
// each path's card-of-record model untouched.

// FaceDownTemplate is the §707.2 characteristics overlay a face-down
// permanent wears, keyed by the effect that turned it face down.
type FaceDownTemplate struct {
	// Key is the registry key ("morph" / "disguise" / "cloak" /
	// "manifest" / "cyber").
	Key string
	// Power / Toughness — the face-down base P/T (almost always 2/2).
	Power     int
	Toughness int
	// Types is the face-down type line, lowercased (e.g. ["creature"] or
	// ["artifact","creature"] for cyber).
	Types []string
	// Subtypes — e.g. ["cyberman"] for Cyber Conversion; nil for the
	// vanilla face-down family.
	Subtypes []string
	// Colors — WUBRG(C) letters; nil/empty means colorless.
	Colors []string
	// Ward is the ward {N} granted while face down; 0 = no ward. Disguise
	// and cloak face-downs have ward {2} (CR §702.166a / §702.171a).
	Ward int
	// Hidden reports whether the underlying identity is concealed from
	// opponents. True for the morph family; false for Cyber Conversion /
	// Ixidron, where the creature is turned face down in the open.
	Hidden bool
	// TurnUp names how the permanent can be turned face up:
	//   "morph"    — pay the morph cost (any time)
	//   "disguise" — pay the disguise cost (any time)
	//   "mana"     — pay the underlying card's mana cost (manifest, if the
	//                underlying is a creature card)
	//   "none"     — cannot be turned face up by its controller (cyber)
	TurnUp string
}

// faceDownTemplates is the canonical registry. Keys are the effect that
// created the face-down state.
var faceDownTemplates = map[string]FaceDownTemplate{
	// CR §702.36 — Morph: 2/2 colorless nameless creature, no abilities,
	// turn face up for the morph cost.
	"morph": {
		Key: "morph", Power: 2, Toughness: 2,
		Types: []string{"creature"}, Hidden: true, TurnUp: "morph",
	},
	// CR §702.166 — Disguise: like morph but with ward {2}; turn up for
	// the disguise cost.
	"disguise": {
		Key: "disguise", Power: 2, Toughness: 2,
		Types: []string{"creature"}, Ward: DisguiseFaceDownWardCost,
		Hidden: true, TurnUp: "disguise",
	},
	// CR §702.171 — Cloak: 2/2 colorless creature with ward {2}; turn up
	// for the underlying card's mana cost (if it's a creature card).
	"cloak": {
		Key: "cloak", Power: 2, Toughness: 2,
		Types: []string{"creature"}, Ward: CloakFaceDownWardCost,
		Hidden: true, TurnUp: "mana",
	},
	// CR §701.34 — Manifest: 2/2 colorless creature, no ward; turn up for
	// the underlying card's mana cost (if it's a creature card).
	"manifest": {
		Key: "manifest", Power: 2, Toughness: 2,
		Types: []string{"creature"}, Hidden: true, TurnUp: "mana",
	},
	// Cyber Conversion / Ixidron — turn an EXISTING creature face down:
	// a 2/2 colorless Cyberman ARTIFACT creature, NOT hidden, with no way
	// to turn it face up.
	"cyber": {
		Key: "cyber", Power: 2, Toughness: 2,
		Types: []string{"artifact", "creature"}, Subtypes: []string{"cyberman"},
		Hidden: false, TurnUp: "none",
	},
}

// FaceDownTemplateFor returns the template registered under key, falling
// back to the morph template (the §707.2 default 2/2 colorless nameless
// no-ability creature) for an unknown or empty key. The fallback is what
// keeps legacy face-down permanents (FaceDownTemplate == "") computing the
// correct base.
func FaceDownTemplateFor(key string) FaceDownTemplate {
	if t, ok := faceDownTemplates[key]; ok {
		return t
	}
	return faceDownTemplates["morph"]
}

// faceDownOpts carries per-path extras for makeFaceDown that aren't
// derivable from the template alone.
type faceDownOpts struct {
	// Markers are extra runtime flags set to 1 on the permanent so the
	// existing family-specific code paths keep recognizing it (e.g.
	// "manifested", "cloaked", "disguise_face_down", "morph_creature",
	// "face_down"). makeFaceDown always derives the ward flags from the
	// template; Markers cover everything else, preserving each mint path's
	// exact current flag set.
	Markers []string
}

// makeFaceDown stamps the face-down overlay described by templateKey onto
// perm. CR §707.2. The REAL card stays perm.Card (the permanent of
// record); makeFaceDown only sets Card.FaceDown, records the template, and
// raises the runtime flags. It does NOT register replacements, fire ETB,
// or move zones — callers own that, because the cast/manifest paths fire
// ETB while the external turn-down path (already on the battlefield) does
// not.
func makeFaceDown(gs *GameState, perm *Permanent, templateKey string, opts faceDownOpts) {
	if perm == nil || perm.Card == nil {
		return
	}
	tmpl := FaceDownTemplateFor(templateKey)
	perm.Card.FaceDown = true
	perm.FaceDownTemplate = tmpl.Key
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Ward {N} from the template (CR §702.166a / §702.171a) — read by
	// CheckWardOnTargeting via the runtime-flag path, since §613.2b strips
	// the AST keywords from the face-down characteristics.
	if tmpl.Ward > 0 {
		perm.Flags["kw:ward"] = 1
		perm.Flags["ward_cost"] = tmpl.Ward
	}
	for _, m := range opts.Markers {
		if m != "" {
			perm.Flags[m] = 1
		}
	}
}
