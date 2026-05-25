package heimdall

import (
	"bytes"
	"fmt"
	"strings"
)

// RenderGameSummaryPDF serializes a GameSummary to a self-contained
// PDF byte stream. The output is a minimal PDF 1.4 document built
// from stdlib only — no third-party library, no font file
// embedding (uses the built-in Helvetica Type1 font that every PDF
// reader carries). Pages are added automatically when the content
// overflows.
//
// Layout:
//
//	HEXDEK / GAME SUMMARY · GAME #N
//	─────────────────────────────────
//	WINNER · TURNS · END_REASON · DATA_SOURCE
//	─────────────────────────────────
//	[ZONE VISITS]
//	  commander_name (SEAT N) — Nx visited · cast Kx · in zone
//	  ...
//	[REGRET CARDS]
//	  card_name (SEAT N) — CMC X · stranded_in_hand
//	  ...
//	[MVP CARDS]
//	  card_name (SEAT N) — CMC X · score Y · ±counters · CMDR
//	  ...
//	[TURNING POINTS]
//	  T4  COMMANDER FIRST CAST · seat 1 cast "X" on turn 4
//	  T9  GAME END · seat 1 won on turn 9
//
// Designed for sharing post-game reports outside hexdek.dev —
// every PDF reader can open it, the file is text-streamable, and
// the content is plain-readable even if opened in a text editor.
func RenderGameSummaryPDF(summary GameSummary) []byte {
	lines := buildSummaryLines(summary)
	return buildPDF(lines)
}

// buildSummaryLines is the layout step — converts a GameSummary
// into the ordered slice of text lines that go onto the page.
// Pure function; the PDF byte-encoding lives in buildPDF.
func buildSummaryLines(s GameSummary) []string {
	var L []string
	add := func(format string, args ...any) {
		L = append(L, fmt.Sprintf(format, args...))
	}
	addBlank := func() { L = append(L, "") }

	add("HEXDEK / GAME SUMMARY")
	add("GAME #%s", s.GameID)
	addBlank()
	winnerLabel := fmt.Sprintf("SEAT %d", s.Winner)
	if s.Winner < 0 {
		winnerLabel = "DRAW"
	}
	add("WINNER: %s · TURNS: %d · END: %s",
		winnerLabel, s.Turns, strings.ToUpper(s.EndReason))
	add("DATA SOURCE: %s", strings.ToUpper(s.DataSource))
	addBlank()

	add("== COMMANDER ZONE VISITS ==")
	if len(s.CommanderZoneVisits) == 0 {
		add("  (no commander zone activity recorded)")
	} else {
		for _, v := range s.CommanderZoneVisits {
			extras := []string{
				fmt.Sprintf("%dx visited", v.Visits),
				fmt.Sprintf("cast %dx", v.CastCount),
			}
			if v.InZoneAtEnd {
				extras = append(extras, "in zone at end")
			}
			add("  %s (SEAT %d) - %s", v.CommanderName, v.Seat, strings.Join(extras, " · "))
		}
	}
	addBlank()

	add("== REGRET CARDS (DREW · NEVER CAST) ==")
	if len(s.RegretCards) == 0 {
		add("  (no stranded-in-hand cards)")
	} else {
		for _, c := range s.RegretCards {
			add("  %s (SEAT %d) - CMC %d · %s",
				c.CardName, c.Seat, c.CMC, c.Reason)
		}
	}
	addBlank()

	add("== MVP CARDS ==")
	if len(s.MVPCards) == 0 {
		add("  (no standout permanents at game end)")
	} else {
		for _, c := range s.MVPCards {
			extras := []string{
				fmt.Sprintf("CMC %d", c.CMC),
				fmt.Sprintf("score %d", c.Score),
			}
			if c.Counters > 0 {
				extras = append(extras, fmt.Sprintf("+%d counters", c.Counters))
			}
			if c.IsCommander {
				extras = append(extras, "CMDR")
			}
			add("  %s (SEAT %d) - %s",
				c.CardName, c.Seat, strings.Join(extras, " · "))
		}
	}
	addBlank()

	add("== MULLIGAN STATS ==")
	if len(s.MulliganStats) == 0 {
		add("  (no mulligan record)")
	} else {
		for _, m := range s.MulliganStats {
			keep := "keep"
			if !m.Keepable {
				keep = "MARGINAL"
			}
			add("  SEAT %d - %d mull · %d-card hand · %dL/%dA · %s",
				m.Seat, m.MulligansTaken, m.OpeningHandSize, m.Lands, m.ActionCards, keep)
		}
	}
	addBlank()

	add("== TURNING POINTS ==")
	if len(s.TurningPoints) == 0 {
		add("  (no timeline events)")
	} else {
		for _, p := range s.TurningPoints {
			add("  T%-3d %s - %s",
				p.Turn, strings.ToUpper(strings.ReplaceAll(p.Kind, "_", " ")), p.Description)
		}
	}

	return L
}

// PDF layout constants — US Letter portrait, 72-pt margins, 12-pt
// line height in Helvetica 10. Roughly 60 lines per page.
const (
	pdfPageWidth   = 612
	pdfPageHeight  = 792
	pdfLeftMargin  = 50
	pdfTopMargin   = 50
	pdfLineHeight  = 12
	pdfFontSize    = 10
	pdfLinesPerPg  = (pdfPageHeight - 2*pdfTopMargin) / pdfLineHeight
	pdfMaxLineCols = 100 // soft cap — wrap long lines to avoid running off the right margin
)

// buildPDF takes layout lines and serializes a minimal PDF.
// Helvetica is a built-in PDF font so no font file is embedded.
// Multi-page output triggers automatically once the line count
// exceeds pdfLinesPerPg.
func buildPDF(lines []string) []byte {
	// Wrap any line longer than pdfMaxLineCols to keep content
	// inside the right margin.
	wrapped := wrapLines(lines, pdfMaxLineCols)

	// Paginate.
	var pages [][]string
	for i := 0; i < len(wrapped); i += pdfLinesPerPg {
		end := i + pdfLinesPerPg
		if end > len(wrapped) {
			end = len(wrapped)
		}
		pages = append(pages, wrapped[i:end])
	}
	if len(pages) == 0 {
		pages = append(pages, []string{""})
	}

	// Object ids: 1=catalog, 2=pages, 3=font, 4..3+P=page objs,
	// 4+P..3+2P=content streams.
	P := len(pages)
	pageObjStart := 4
	contentObjStart := 4 + P

	// Build the object bodies first; offsets get filled in during
	// the serialization pass below.
	objBodies := make([][]byte, 3+2*P)

	// Object 1: Catalog
	objBodies[0] = []byte("<< /Type /Catalog /Pages 2 0 R >>")

	// Object 2: Pages
	var kidsBuf bytes.Buffer
	kidsBuf.WriteString("[")
	for i := 0; i < P; i++ {
		if i > 0 {
			kidsBuf.WriteString(" ")
		}
		fmt.Fprintf(&kidsBuf, "%d 0 R", pageObjStart+i)
	}
	kidsBuf.WriteString("]")
	objBodies[1] = []byte(fmt.Sprintf(
		"<< /Type /Pages /Kids %s /Count %d >>", kidsBuf.String(), P))

	// Object 3: Font (built-in Helvetica)
	objBodies[2] = []byte(
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")

	// Objects 4..3+P: page objects, each referencing its content
	// stream.
	for i := 0; i < P; i++ {
		contentObjID := contentObjStart + i
		objBodies[2+i+1] = []byte(fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Contents %d 0 R /Resources << /Font << /F1 3 0 R >> >> >>",
			pdfPageWidth, pdfPageHeight, contentObjID))
	}

	// Objects 4+P..3+2P: content streams, one per page.
	for i, page := range pages {
		stream := buildContentStream(page)
		body := fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream)
		objBodies[2+P+i+1] = []byte(body)
	}

	// Serialize. Track byte offsets so the xref table is correct.
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	// PDF spec recommends a high-bit "binary" marker in a comment
	// on line 2 so transports that sniff text-vs-binary classify
	// correctly.
	out.WriteString("%\xE2\xE3\xCF\xD3\n")

	offsets := make([]int, len(objBodies))
	for i, body := range objBodies {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(body)
		out.WriteString("\nendobj\n")
	}

	xrefOffset := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(objBodies)+1)
	out.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objBodies)+1, xrefOffset)

	return out.Bytes()
}

// buildContentStream emits the BT/ET text-object body for one
// page's lines. Each line begins with a Td that moves the cursor
// from the prior line down by pdfLineHeight.
func buildContentStream(lines []string) string {
	var b bytes.Buffer
	b.WriteString("BT\n")
	fmt.Fprintf(&b, "/F1 %d Tf\n", pdfFontSize)
	// Start cursor at top-left of the writable area.
	fmt.Fprintf(&b, "%d %d Td\n", pdfLeftMargin, pdfPageHeight-pdfTopMargin)
	for i, line := range lines {
		if i > 0 {
			fmt.Fprintf(&b, "0 -%d Td\n", pdfLineHeight)
		}
		fmt.Fprintf(&b, "(%s) Tj\n", escapePDFString(line))
	}
	b.WriteString("ET\n")
	return b.String()
}

// escapePDFString escapes the three characters that have special
// meaning inside a PDF literal string: '(' ')' '\\'. Also strips
// any non-ASCII so the WinAnsiEncoding rendering stays stable
// without embedding a custom encoding map.
func escapePDFString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 127 {
			// Replace non-ASCII with a safe placeholder; the
			// alternative is shipping a CMap which is overkill
			// for this surface.
			b.WriteByte('?')
			continue
		}
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrapLines hard-wraps each input line at the given column count.
// Empty input lines pass through; long lines split at the last
// space before the cap to preserve word boundaries when possible.
func wrapLines(lines []string, cols int) []string {
	if cols <= 0 {
		return lines
	}
	var out []string
	for _, line := range lines {
		for len(line) > cols {
			cut := cols
			if sp := strings.LastIndex(line[:cols], " "); sp > cols/2 {
				cut = sp
			}
			out = append(out, line[:cut])
			line = strings.TrimLeft(line[cut:], " ")
		}
		out = append(out, line)
	}
	return out
}
