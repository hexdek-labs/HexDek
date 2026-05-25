package heimdall

import "context"

// HuginnSink receives co-trigger observations for pattern discovery.
type HuginnSink interface {
	IngestCoTriggers(pairs []CoTriggerPair, deckNames []string)
}

// HuginnZoneCastSink is an OPTIONAL extension to HuginnSink. Implementers
// receive zone-cast lifecycle events for "casts from elsewhere" pattern
// discovery. Observer.RecordObservation type-asserts and skips silently
// if not implemented.
type HuginnZoneCastSink interface {
	IngestZoneCastEvents(events []ZoneCastEvent, deckNames []string, gameID string)
}

// HuginnExileLinkSink is an OPTIONAL extension to HuginnSink. Implementers
// receive O-Ring style link/unlink events for blink-pattern discovery.
type HuginnExileLinkSink interface {
	IngestExileLinkEvents(events []ExileLinkEvent, deckNames []string, gameID string)
}

// MuninnSink receives bug signals for persistent memory.
type MuninnSink interface {
	RecordParserGaps(gaps []string, gameID string)
	RecordDeadTriggers(triggers []DeadTrigger, gameID string)
	RecordCrash(panicMsg string, stackTrace string, deckKeys []string)
}

// MuninnExileLinkSink is an OPTIONAL extension to MuninnSink. Implementers
// receive exile-link events to detect O-Ring parity divergences (linked
// cards that fail to return when the source leaves the battlefield).
type MuninnExileLinkSink interface {
	RecordExileLinkEvents(events []ExileLinkEvent, gameID string)
}

// TelemetrySink sends health pulses (GA4).
type TelemetrySink interface {
	Pulse(stats HealthPulse)
}

// SnapshotSink persists per-game observation snapshots so the
// "rich path" of /api/games/{id}/summary and /api/games/{id}/replay
// can be served for historical games without re-running them. The
// canonical implementation wraps db.InsertGameObservation (an
// UPSERT), so calls are idempotent — Observer.RecordSnapshot is
// expected to be invoked multiple times during a live game so the
// persisted snapshot tracks game progress; each call overwrites
// the prior row.
//
// gameID is the showmatch_game.game_id primary key; callers must
// have already inserted the parent game row so the FK satisfies.
// The interface takes the snapshot struct (not the marshaled
// string) so alternative sinks can choose their own serialization
// — e.g., an S3-backed sink might use a binary encoding while the
// DB sink uses JSON.
type SnapshotSink interface {
	PersistGameObservation(ctx context.Context, gameID int64, snap GameObservationSnapshot) error
}
