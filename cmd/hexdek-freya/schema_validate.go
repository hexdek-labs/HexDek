package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ---------------------------------------------------------------------------
// Freya output JSON schema validation (R60).
//
// The schema (freya_output.schema.json) is the canonical contract between
// Freya and downstream consumers. ValidateFreyaJSON runs a sample of
// `--format json` output against the schema and returns the list of
// validation failures.
//
// --validate-output flag wires this into the CLI so an explicit
// post-render check can run on demand. CI enforcement is via
// TestFreyaSchema_GeneratedOutputValidates which marshals a fully-
// populated synthetic FreyaReport and asserts it passes the schema —
// the schema-vs-code drift detector.
// ---------------------------------------------------------------------------

//go:embed freya_output.schema.json
var freyaOutputSchemaJSON []byte

// FreyaOutputSchemaURL is the canonical identifier embedded in the
// schema's $id. Used as the resource location when registering the
// embedded schema with the compiler.
const FreyaOutputSchemaURL = "https://hexdek.dev/schemas/freya_output.schema.json"

// compiledFreyaOutputSchema is the lazily-built validator instance.
// Built once on first use and reused — schema compilation is
// non-trivial (parses + resolves all $refs) and a per-call rebuild
// would dominate the validation cost.
var compiledFreyaOutputSchema *jsonschema.Schema

// FreyaOutputSchema returns the compiled validator. Panics on
// internal schema-compile failure since the schema is embedded at
// build time — a compile error is a programming bug, not a runtime
// condition.
func FreyaOutputSchema() *jsonschema.Schema {
	if compiledFreyaOutputSchema != nil {
		return compiledFreyaOutputSchema
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(freyaOutputSchemaJSON)))
	if err != nil {
		panic(fmt.Sprintf("freya schema: unmarshal embedded schema: %v", err))
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(FreyaOutputSchemaURL, doc); err != nil {
		panic(fmt.Sprintf("freya schema: add resource: %v", err))
	}
	schema, err := compiler.Compile(FreyaOutputSchemaURL)
	if err != nil {
		panic(fmt.Sprintf("freya schema: compile: %v", err))
	}
	compiledFreyaOutputSchema = schema
	return compiledFreyaOutputSchema
}

// ValidateFreyaJSON validates a Freya `--format json` output payload
// against the canonical schema. Returns the slice of validation-error
// strings (each suitable for printing to stderr); empty when the
// payload is valid. The error return is reserved for catastrophic
// JSON-parse failures of the input bytes themselves — schema-rule
// violations come back via the slice.
//
// Schema compilation cost is amortized via the package-level
// compiledFreyaOutputSchema cache, so repeated calls in tight loops
// (e.g. validating every deck in a --all-decks run) are cheap.
func ValidateFreyaJSON(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, errors.New("validate: empty input")
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("validate: parse input JSON: %w", err)
	}
	schema := FreyaOutputSchema()
	if err := schema.Validate(doc); err != nil {
		return flattenValidationError(err), nil
	}
	return nil, nil
}

// flattenValidationError walks a jsonschema.ValidationError tree into
// a flat slice of one-line strings. The default Error() output is
// indented multi-line per failure; for CLI / test reporting we want
// each leaf failure as its own line so callers can sort, count, and
// pin specific expectations.
func flattenValidationError(err error) []string {
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []string{err.Error()}
	}
	var out []string
	walkValidationError(ve, &out)
	if len(out) == 0 {
		out = append(out, ve.Error())
	}
	return out
}

// walkValidationError descends the error tree, recording only leaf
// failures (entries with no Causes). The intermediate nodes describe
// the schema path traversed, which is useful in debugging but noisy
// for surfacing.
func walkValidationError(ve *jsonschema.ValidationError, out *[]string) {
	if ve == nil {
		return
	}
	if len(ve.Causes) == 0 {
		path := strings.Join(ve.InstanceLocation, "/")
		if path == "" {
			path = "(root)"
		}
		*out = append(*out, fmt.Sprintf("at /%s: %v", path, ve.ErrorKind))
		return
	}
	for _, c := range ve.Causes {
		walkValidationError(c, out)
	}
}
