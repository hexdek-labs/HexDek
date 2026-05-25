package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
)

// CSV export columns. Order matches csvHeader; resultToCSVRecord must
// emit fields in the same order.
//
// Optimized for spreadsheet pattern-finding on uncovered cards: each
// column is a thing a pivot table or filter would key on:
//
//	name, class           — identity + bucket
//	type_line             — supertype/subtype slicing (saga, battle, etc.)
//	oracle_len            — proxy for complexity; sort to surface the
//	                        long-tail oracle bodies that defeat the parser
//	parse_error_count     — quick filter for "PARTIAL with multiple errors"
//	parse_errors          — semicolon-joined detail (PARTIAL only; "" otherwise)
//	pattern               — heuristic failure-pattern bucket from classifyPattern
//	oracle_text           — full oracle text, newlines collapsed; CSV-quoted
const (
	csvColName          = "name"
	csvColClass         = "class"
	csvColTypeLine      = "type_line"
	csvColOracleLen     = "oracle_len"
	csvColParseErrCount = "parse_error_count"
	csvColParseErrors   = "parse_errors"
	csvColPattern       = "pattern"
	csvColOracleText    = "oracle_text"
)

func csvHeader() []string {
	return []string{
		csvColName, csvColClass, csvColTypeLine, csvColOracleLen,
		csvColParseErrCount, csvColParseErrors, csvColPattern, csvColOracleText,
	}
}

// resultToCSVRecord renders one result as a CSV record (a slice of
// string fields in csvHeader order). Newlines in oracle text are
// collapsed to single spaces so the column doesn't wrap across rows
// in the spreadsheet — encoding/csv would otherwise emit the literal
// newline inside the quoted field, which most spreadsheet tools
// handle but readers find awkward. CSV-level escaping (quotes,
// commas, embedded quotes) is handled by encoding/csv.
func resultToCSVRecord(r result) []string {
	return []string{
		r.Name,
		r.Class.String(),
		r.TypeLine,
		strconv.Itoa(len(r.OracleText)),
		strconv.Itoa(len(r.ParseErrors)),
		strings.Join(r.ParseErrors, "; "),
		classifyPattern(r),
		csvOracleText(r.OracleText),
	}
}

// csvOracleText collapses CR/LF and runs of whitespace to single
// spaces. Empty input returns "" so downstream filters can detect
// missing oracle text via an `oracle_len == 0` filter or by string
// emptiness.
func csvOracleText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// writeCSV emits the CSV export at path. The header row is always
// written, even if no result rows match — that way a spreadsheet
// import never gets confused by a zero-byte file or a header-less
// stream.
//
// includeOK gates whether OK / OK_VANILLA rows are emitted. Default
// (false) keeps the export focused on uncovered cards, which is the
// pattern-finding use case. Setting it true also exports OK rows for
// callers who want to compute coverage rates in the spreadsheet.
func writeCSV(path string, results []result, includeOK bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(csvHeader()); err != nil {
		return err
	}
	for _, r := range results {
		if !includeOK && (r.Class == classOK || r.Class == classOKVanilla) {
			continue
		}
		if err := w.Write(resultToCSVRecord(r)); err != nil {
			return err
		}
	}
	return w.Error()
}
