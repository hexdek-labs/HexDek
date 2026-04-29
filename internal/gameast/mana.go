package gameast

// ManaSymbol is a single mana symbol from a cost. Examples:
//
//	{2}       -> ManaSymbol{Raw: "{2}", Generic: 2}
//	{U}       -> ManaSymbol{Raw: "{U}", Color: []string{"U"}}
//	{U/B}     -> ManaSymbol{Raw: "{U/B}", Color: []string{"U", "B"}}
//	{2/W}     -> ManaSymbol{Raw: "{2/W}", Generic: 2, Color: []string{"W"}}
//	{X}       -> ManaSymbol{Raw: "{X}", IsX: true}
//	{U/P}     -> ManaSymbol{Raw: "{U/P}", Color: []string{"U"}, IsPhyrexian: true}
//	{S}       -> ManaSymbol{Raw: "{S}", IsSnow: true}
//
// Mirrors scripts/mtg_ast.py :: ManaSymbol.
type ManaSymbol struct {
	Raw         string   `json:"raw"`
	Generic     int      `json:"generic"`
	Color       []string `json:"color,omitempty"`
	IsX         bool     `json:"is_x"`
	IsPhyrexian bool     `json:"is_phyrexian"`
	IsSnow      bool     `json:"is_snow"`
}

// ManaCost is a sequence of mana symbols. Mirrors scripts/mtg_ast.py :: ManaCost.
type ManaCost struct {
	Symbols []ManaSymbol `json:"symbols"`
}

// CMC returns the converted mana cost / mana value of the cost. X counts 0.
func (c ManaCost) CMC() int {
	total := 0
	for _, s := range c.Symbols {
		total += s.Generic
		if len(s.Color) > 0 {
			total++
		}
	}
	return total
}
