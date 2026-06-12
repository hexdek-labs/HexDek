package per_card

// registerTiamatCustom is intentionally a no-op (Jodah pattern: the
// registration site is kept so zz_handler_coverage_2_register.go's call
// list stays stable, but the handler defers to the sibling file).
//
// History: this file originally registered tiamatETBSearch, written when
// only an auto-generated stub existed for Tiamat. The Muninn parser-gap
// wave #51-70 later landed a full handler in tiamat.go (tiamatETB,
// registered via registerDefaults) without noticing this one. fireETB
// runs EVERY handler appended under the normalized key, and both
// handlers gated on was_cast and both executed — so a cast Tiamat
// tutored up to TEN Dragons (two stacked 5-card searches: the second
// handler's name-dedup was handler-local, and the first handler's picks
// had already left the library) plus a double shuffle. r61 review 03
// bug C-1.
//
// tiamat.go's handler is the keeper: case-insensitive name dedup (the
// oracle's "that each have different names"), highest-CMC-first hat
// policy, and emitFail telemetry on the not-cast path. Regression
// coverage for single-fire dispatch lives in
// tiamat_single_etb_r61_test.go.
func registerTiamatCustom(_ *Registry) {}
