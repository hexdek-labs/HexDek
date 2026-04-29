package moxfield

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParsedEntry is a single line from a pasted decklist — quantity + card name.
type ParsedEntry struct {
	Quantity int
	Name     string
	Section  string // "commander" | "mainboard" | "sideboard" | "companion" | "maybeboard"
}

// ParsedDecklist is the output of parsing a decklist text blob, before
// Scryfall enrichment.
type ParsedDecklist struct {
	Commander []ParsedEntry
	Mainboard []ParsedEntry
}

var lineRE = regexp.MustCompile(`^\s*(\d+)\s*[xX]?\s+(.+?)\s*$`)

// ParseDecklist accepts the text exported by Moxfield / Archidekt / Goldfish /
// any tool that emits the "{qty} {name}" decklist convention.
//
// Supported section markers (case-insensitive, // optional):
//
//	Commander / // Commander / SB: (sideboard) / // Sideboard / // Companion
//
// When no section markers are present, the first entry is treated as the
// commander — this matches the convention when copying a commander deck's
// mainboard from Moxfield's default export.
//
// Lines that don't match the qty+name shape are ignored (blank lines,
// random comments, set codes in parens, etc. — we strip parenthetical set
// codes and collector numbers before matching).
func ParseDecklist(text string) (*ParsedDecklist, error) {
	out := &ParsedDecklist{}
	section := "mainboard"
	sawMarker := false

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Section headers
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "#"):
			lower = strings.TrimLeft(lower, "/# \t")
			if strings.HasPrefix(lower, "commander") {
				section = "commander"
				sawMarker = true
				continue
			}
			if strings.HasPrefix(lower, "mainboard") || strings.HasPrefix(lower, "deck") {
				section = "mainboard"
				sawMarker = true
				continue
			}
			if strings.HasPrefix(lower, "sideboard") || strings.HasPrefix(lower, "companion") ||
				strings.HasPrefix(lower, "maybeboard") || strings.HasPrefix(lower, "considering") {
				section = "sideboard"
				sawMarker = true
				continue
			}
			// Unknown comment: skip
			continue
		case strings.HasPrefix(lower, "sb:"):
			section = "sideboard"
			line = strings.TrimSpace(line[3:])
		}

		// Strip trailing parenthetical set/collector info:
		//   "1 Yuriko, the Tiger's Shadow (C18) 275"  →  "1 Yuriko, the Tiger's Shadow"
		if idx := strings.Index(line, "("); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}

		m := lineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		qty, err := strconv.Atoi(m[1])
		if err != nil || qty < 1 {
			continue
		}
		name := cleanName(m[2])
		if name == "" {
			continue
		}

		entry := ParsedEntry{Quantity: qty, Name: name, Section: section}
		switch section {
		case "commander":
			out.Commander = append(out.Commander, entry)
		case "sideboard":
			// skip for commander format
		default:
			out.Mainboard = append(out.Mainboard, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read decklist: %w", err)
	}

	// If no marker saw a commander block, promote the first mainboard entry.
	if !sawMarker && len(out.Commander) == 0 && len(out.Mainboard) > 0 {
		out.Commander = []ParsedEntry{out.Mainboard[0]}
		out.Mainboard = out.Mainboard[1:]
	}

	if len(out.Commander) == 0 && len(out.Mainboard) == 0 {
		return nil, fmt.Errorf("no cards found in decklist")
	}
	return out, nil
}

// cleanName strips trailing foil markers, DFC slashes normalize, and trims.
//
// "Malakir Rebirth // Malakir Mire"       → "Malakir Rebirth // Malakir Mire"  (preserve for Scryfall)
// "Lightning Bolt *F*"                    → "Lightning Bolt"
// "Lightning Bolt [foil]"                 → "Lightning Bolt"
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	// Strip foil markers like *F*, *E*, [foil], [etched]
	s = regexp.MustCompile(`\s*\*[A-Z]+\*\s*$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s*\[[^\]]+\]\s*$`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
