// Package gameast defines Go types mirroring the Python typed AST in
// scripts/mtg_ast.py. It is the data contract between the Python parser
// (which emits ast_dataset.jsonl) and the Go rules engine (which will
// consume it at runtime).
//
// Per MTG comp rules §113, every ability is one of:
//
//   - Static        — always-on rules text
//   - Activated     — cost/effect abilities
//   - Triggered     — When/Whenever/At-the-beginning-of events
//   - Keyword       — named shorthand (Flying, Trample, Flashback, ...)
//
// Plus ~40 typed effect nodes (Damage, Draw, Destroy, Sequence, Choice,
// ...) that can be nested recursively under Activated and Triggered.
//
// # Design choices mirroring Python
//
//   - Python uses @dataclass(frozen=True); Go uses plain structs. Immutability
//     is enforced by convention: once an AST is loaded it must not be
//     mutated. The engine passes AST nodes by pointer but never modifies them.
//
//   - Python uses duck-typed Union[EffectNode subclass, ...] for effects. Go
//     uses the Effect interface (Kind() string) implemented by every leaf
//     type. Type assertions and type switches discriminate.
//
//   - Python's amount/count fields accept int OR str OR ScalingAmount. Go
//     uses the NumberOrRef discriminated union to represent this safely.
//
//   - Python's tuple[T, ...] becomes []T in Go. Equality requires
//     slices.Equal; signatures are computed via Signature().
//
// # Loader
//
// See package github.com/hexdek/hexdek/internal/astload for
// the JSONL loader. This package only defines the types.
//
// # Engine
//
// See package github.com/hexdek/hexdek/internal/gameengine
// (not yet created) for the resolver that consumes these AST nodes.
package gameast
