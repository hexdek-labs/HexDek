package astload

import (
	"bytes"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r135 — Adventurer / Omen halves (CR §715.2, §720.2).
//
// Before r135 the exporter joined card_faces[*].oracle_text with "\n" and
// parsed the concatenation as ONE card, so:
//
//   - the half's instant/sorcery text landed in the CREATURE's top-level
//     abilities (Faerie Guidemother's creature side carried "target
//     creature gets +2/+1 and gains flying"), and
//   - no Keyword{Name:"adventure"|"omen"} was ever emitted, so
//     gameengine.HasAdventure / HasOmen were false for all 38,754 cards
//     in the corpus and CastAdventure / CastOmen were unreachable.
//
// Measured on the live corpus at r135: 140 adventure + 13 omen keywords
// emitted where there had been 0 and 0, 171 half-abilities removed from
// parent cards, and zero change to any card outside layout="adventure".

// r135Row is one exporter-shaped JSONL line for an Adventurer card: the
// parent keeps only its own ability, the half rides under "faces", and
// the parent carries the keyword that the engine's readers gate on.
const r135Row = `{"name":"Faerie Guidemother // Gift of the Fae","oracle_text":"Flying","type_line":"Creature — Faerie","mana_cost":"{W}","cmc":1.0,"colors":["W"],` +
	`"faces":[{"name":"Gift of the Fae","oracle_text":"Target creature gets +2/+1 and gains flying until end of turn.","type_line":"Sorcery — Adventure","mana_cost":"{1}{W}"}],` +
	`"ast":{"__ast_type__":"CardAST","name":"Faerie Guidemother // Gift of the Fae",` +
	`"abilities":[{"__ast_type__":"Keyword","name":"flying","args":[],"raw":"flying"},` +
	`{"__ast_type__":"Keyword","name":"adventure","args":["{1}{W}"],"raw":"Sorcery — Adventure"}],` +
	`"faces":[{"__ast_type__":"CardAST","name":"Gift of the Fae",` +
	`"abilities":[{"__ast_type__":"Keyword","name":"flying","args":[],"raw":"gains flying until end of turn"}],` +
	`"parse_errors":[],"fully_parsed":true}],` +
	`"parse_errors":[],"fully_parsed":true}}`

// r135OmenRow is the Omen twin — same shape, different keyword, and the
// half carries a real mana cost so OmenCost has something to read.
const r135OmenRow = `{"name":"Marang River Regent // Coil and Catch","oracle_text":"Flying","type_line":"Creature — Dragon","mana_cost":"{4}{U}{U}","cmc":6.0,"colors":["U"],` +
	`"ast":{"__ast_type__":"CardAST","name":"Marang River Regent // Coil and Catch",` +
	`"abilities":[{"__ast_type__":"Keyword","name":"flying","args":[],"raw":"flying"},` +
	`{"__ast_type__":"Keyword","name":"omen","args":["{3}{U}"],"raw":"Instant — Omen"}],` +
	`"faces":[{"__ast_type__":"CardAST","name":"Coil and Catch",` +
	`"abilities":[],"parse_errors":[],"fully_parsed":true}],` +
	`"parse_errors":[],"fully_parsed":true}}`

func r135Load(t *testing.T, row, name string) *gameast.CardAST {
	t.Helper()
	c, err := LoadReader(bytes.NewBufferString(row + "\n"))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if c.Count() != 1 {
		t.Fatalf("want 1 card decoded, got %d", c.Count())
	}
	ast, ok := c.Get(name)
	if !ok || ast == nil {
		t.Fatalf("card %q absent from corpus (names: %v)", name, c.Names())
	}
	return ast
}

func r135HasKeyword(ast *gameast.CardAST, name string) bool {
	if ast == nil {
		return false
	}
	for _, ab := range ast.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && kw.Name == name {
			return true
		}
	}
	return false
}

// TestR135_AdventureHalfDecodesAsItsOwnFace is the load-bearing direction:
// the half must arrive as a separate CardAST, not as abilities bolted onto
// the parent. Pre-change the decoder ignored "faces" entirely and this read
// zero.
func TestR135_AdventureHalfDecodesAsItsOwnFace(t *testing.T) {
	ast := r135Load(t, r135Row, "Faerie Guidemother // Gift of the Fae")

	if len(ast.Faces) != 1 {
		t.Fatalf("want 1 decoded face (the Adventure half), got %d", len(ast.Faces))
	}
	half := ast.Faces[0]
	if half.Name != "Gift of the Fae" {
		t.Errorf("half name: want %q, got %q", "Gift of the Fae", half.Name)
	}
	if len(half.Abilities) != 1 {
		t.Errorf("want the half to carry its own 1 ability, got %d", len(half.Abilities))
	}

	// The other direction: the parent must NOT have absorbed the half.
	// Before r135 the creature carried the sorcery's text at top level.
	if got := len(ast.Abilities); got != 2 {
		t.Errorf("parent abilities: want 2 (flying + the adventure keyword), got %d", got)
	}
	if !r135HasKeyword(ast, "flying") {
		t.Error("parent lost its own printed keyword 'flying'")
	}
}

// TestR135_AdventureAndOmenKeywordsReachTheEngineReaders pins the thing that
// was dead corpus-wide: gameengine.HasAdventure / HasOmen / OmenCost all gate
// on a Keyword node, and the exporter never emitted one. These assert the
// node the readers look for is present and carries the half's cost.
func TestR135_AdventureAndOmenKeywordsReachTheEngineReaders(t *testing.T) {
	adv := r135Load(t, r135Row, "Faerie Guidemother // Gift of the Fae")
	if !r135HasKeyword(adv, "adventure") {
		t.Error("no Keyword{Name:\"adventure\"} on the parent — HasAdventure stays false, " +
			"CastAdventure stays unreachable (the pre-r135 state for all 140 adventure cards)")
	}

	om := r135Load(t, r135OmenRow, "Marang River Regent // Coil and Catch")
	if !r135HasKeyword(om, "omen") {
		t.Error("no Keyword{Name:\"omen\"} on the parent — HasOmen stays false for all 13 omen cards")
	}

	// OmenCost reads args[0] as a mana string; assert it survived decode.
	var omenArgs []any
	for _, ab := range om.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && kw.Name == "omen" {
			omenArgs = kw.Args
		}
	}
	if len(omenArgs) != 1 {
		t.Fatalf("omen keyword args: want 1 (the half's mana cost), got %d", len(omenArgs))
	}
	if s, ok := omenArgs[0].(string); !ok || s != "{3}{U}" {
		t.Errorf("omen cost arg: want %q, got %#v", "{3}{U}", omenArgs[0])
	}
}

// TestR135_NonAdventureCardsCarryNoFaces is the must-never direction. r135
// de-concatenates ONLY layout="adventure"; transform / modal_dfc / split /
// flip still join their faces and must decode with Faces empty. Measured:
// 0 of 32,264 non-adventure cards changed.
func TestR135_NonAdventureCardsCarryNoFaces(t *testing.T) {
	const plain = `{"name":"Lightning Bolt","oracle_text":"","type_line":"Instant","mana_cost":"{R}","cmc":1.0,"colors":["R"],` +
		`"ast":{"__ast_type__":"CardAST","name":"Lightning Bolt","abilities":[],"parse_errors":[],"fully_parsed":true}}`
	ast := r135Load(t, plain, "Lightning Bolt")
	if len(ast.Faces) != 0 {
		t.Errorf("a card with no \"faces\" key must decode to zero faces, got %d", len(ast.Faces))
	}
}
