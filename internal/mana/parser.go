// Package mana parses Magic: the Gathering mana cost strings and tracks
// per-color mana pools.
//
// The parser handles standard mana cost notation: {2}{U}{B} → 2 generic +
// 1 blue + 1 black. The pool tracks current available mana per color and
// provides a CanPay/Pay method pair for spell casting.
//
// This package is the authoritative arbiter of "can the player cast this
// spell?" — no nicely-arranged human override, no honor system. The
// system reads the printed cost and the available pool and answers.
package mana

import (
	"fmt"
	"strconv"
	"strings"
)

// Color represents a single mana color or generic/colorless.
type Color string

const (
	White     Color = "W"
	Blue      Color = "U"
	Black     Color = "B"
	Red       Color = "R"
	Green     Color = "G"
	Colorless Color = "C"
	Generic   Color = "G_GEN" // generic (numeric) mana, payable with any color
	Variable  Color = "X"     // variable cost paid at cast time
)

// Cost describes the mana cost of a spell, broken out by color.
type Cost struct {
	Generic  int            // {2} → 2 generic
	Variable int            // {X} → 1 (multiplied at cast time)
	Colored  map[Color]int  // {U}{U} → {Blue: 2}
	Raw      string         // original cost string for round-tripping
}

// CMC returns the converted mana value, treating each Variable as 0 (since
// X is paid at cast time and not part of the printed cost).
func (c Cost) CMC() int {
	total := c.Generic
	for _, n := range c.Colored {
		total += n
	}
	return total
}

// Parse parses a mana cost string in standard Magic notation.
//
// Examples:
//
//	Parse("{2}{U}{B}") → Cost{Generic: 2, Colored: {Blue: 1, Black: 1}}
//	Parse("{B}{B}{B}") → Cost{Colored: {Black: 3}}
//	Parse("{X}{G}{G}") → Cost{Variable: 1, Colored: {Green: 2}}
//	Parse("{16}")      → Cost{Generic: 16}
//	Parse("")          → Cost{} (lands and other free-cost cards)
func Parse(s string) (Cost, error) {
	cost := Cost{
		Colored: make(map[Color]int),
		Raw:     s,
	}
	if s == "" {
		return cost, nil
	}

	// Tokenize by braces. Each token is a single mana symbol enclosed in {}.
	for i := 0; i < len(s); {
		if s[i] != '{' {
			return cost, fmt.Errorf("mana parse: expected '{' at offset %d, got %q", i, s[i])
		}
		end := strings.IndexByte(s[i:], '}')
		if end == -1 {
			return cost, fmt.Errorf("mana parse: unclosed brace at offset %d", i)
		}
		end += i
		token := s[i+1 : end]
		i = end + 1

		if err := cost.applyToken(token); err != nil {
			return cost, fmt.Errorf("mana parse token %q: %w", token, err)
		}
	}
	return cost, nil
}

func (c *Cost) applyToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty mana symbol")
	}

	// Handle numeric (generic) costs: {0}, {1}, {2}, ..., {16}, {20}, ...
	if n, err := strconv.Atoi(token); err == nil {
		if n < 0 {
			return fmt.Errorf("negative generic cost %d", n)
		}
		c.Generic += n
		return nil
	}

	// Single-letter color symbols
	switch strings.ToUpper(token) {
	case "W":
		c.Colored[White]++
	case "U":
		c.Colored[Blue]++
	case "B":
		c.Colored[Black]++
	case "R":
		c.Colored[Red]++
	case "G":
		c.Colored[Green]++
	case "C":
		c.Colored[Colorless]++
	case "X":
		c.Variable++
	default:
		// Hybrid mana ({W/U}, {2/W}), Phyrexian ({W/P}), snow ({S}) — not
		// handled in MVP. These are rare in our deck list and can be added
		// later if needed.
		return fmt.Errorf("unsupported mana symbol %q (hybrid/phyrexian/snow not yet implemented)", token)
	}
	return nil
}

// String returns a canonical mana cost string from a Cost value.
// Note: this may differ from the raw input in symbol ordering; round-trip
// equality is not guaranteed for inputs with non-canonical symbol order.
func (c Cost) String() string {
	if c.Generic == 0 && c.Variable == 0 && len(c.Colored) == 0 {
		return ""
	}
	var sb strings.Builder
	for i := 0; i < c.Variable; i++ {
		sb.WriteString("{X}")
	}
	if c.Generic > 0 {
		sb.WriteByte('{')
		sb.WriteString(strconv.Itoa(c.Generic))
		sb.WriteByte('}')
	}
	for _, color := range []Color{White, Blue, Black, Red, Green, Colorless} {
		for i := 0; i < c.Colored[color]; i++ {
			sb.WriteByte('{')
			sb.WriteString(string(color))
			sb.WriteByte('}')
		}
	}
	return sb.String()
}
