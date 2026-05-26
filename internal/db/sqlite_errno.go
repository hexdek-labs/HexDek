package db

import (
	"errors"

	sqlitedrv "modernc.org/sqlite"
	sqliteerr "modernc.org/sqlite/lib"
)

// sqlite_errno.go — driver-specific errno introspection.
//
// modernc.org/sqlite returns a *sqlite.Error from sql.DB execs whose
// Code() method exposes the extended SQLite result code (e.g.
// SQLITE_CONSTRAINT_PRIMARYKEY = 1555, SQLITE_CONSTRAINT_FOREIGNKEY =
// 787). Callers that need to distinguish "constraint violation that
// retry can fix" (PRIMARY KEY collision on a randomly-generated code)
// from "constraint violation that retry can't fix" (FOREIGN KEY missing
// referent, NOT NULL violation) check the code directly.
//
// The helpers below isolate the modernc-specific import so call sites
// in internal/db/* don't have to know which driver is in use. If the
// driver is swapped, this file is the only place that changes.

// IsSQLitePrimaryKeyConflict reports whether `err` is a SQLite
// PRIMARY KEY constraint violation (extended result code 1555).
// Returns false for nil, non-sqlite errors, or other constraint codes
// (FOREIGN KEY, NOT NULL, UNIQUE on a non-PK column, etc.).
//
// Wrapped errors (fmt.Errorf("%w", ...)) are unwrapped via errors.As
// so callers can wrap their own context onto driver errors without
// breaking the check.
//
// Used by CreateParty to distinguish "the random 6-char code we
// generated collided — retry with a fresh code" (return true) from
// "the schema or transport is broken" (return false → bail).
func IsSQLitePrimaryKeyConflict(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlitedrv.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code() == sqliteerr.SQLITE_CONSTRAINT_PRIMARYKEY
}

// IsSQLiteConstraint reports whether `err` is ANY SQLite constraint
// violation (PRIMARY KEY, FOREIGN KEY, NOT NULL, UNIQUE, CHECK, etc.).
// The check matches both the bare SQLITE_CONSTRAINT (19) and any of
// the extended SQLITE_CONSTRAINT_* codes, since modernc.org/sqlite
// returns the extended form by default but some code paths surface
// the bare one.
//
// Currently unused in production code; exposed for symmetry with
// IsSQLitePrimaryKeyConflict and for future call sites that need
// "retry on any constraint" semantics.
func IsSQLiteConstraint(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlitedrv.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	if code == sqliteerr.SQLITE_CONSTRAINT {
		return true
	}
	// Extended codes are SQLITE_CONSTRAINT | (subtype << 8); the
	// low byte equals SQLITE_CONSTRAINT (19) for every extended
	// constraint variant.
	return (code & 0xff) == sqliteerr.SQLITE_CONSTRAINT
}
