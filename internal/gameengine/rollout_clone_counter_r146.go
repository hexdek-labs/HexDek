package gameengine

import "sync/atomic"

// r146 — rollout clone accounting.
//
// The 12:37 outage and the three post-restart heap excursions were an
// allocation RATE event (~300MB/s), not a leak. Both existing guards
// counted the wrong things: drainPendingTriggers caps total resolutions
// at 500 and pushTriggeredAbility caps fires at 1000/turn, so neither
// can account for 18GB — 1000 fires would need ~18MB each.
//
// Every excursion coincided with a SLOW GAME whose s/turn was an order
// of magnitude above normal (22.0, 7.4, 4.5 against a 0.2-0.4 baseline,
// 3 of 3, with zero false positives across the other seven slow games
// in the same window). High s/turn is what a game looks like when the
// hat is running rollouts: each rollout deep-copies the GameState and
// then simulates up to rolloutDepth FULL TURNS on the copy, so a single
// real turn can cost K candidates x n determinizations x depth turns of
// allocation. That multiplier is the right order of magnitude for the
// observed slowdown; the token count never was.
//
// This counter is the r142 pattern: PUBLISH a process-wide number, do
// not inspect another worker's state. It is pure instrumentation — it
// changes no decision and cannot be made to fail by reverting
// semantics, because it has none.
var rolloutClones atomic.Int64

// RolloutClones returns the process-wide count of CloneForRollout calls
// that produced a clone. Callers delta it themselves.
func RolloutClones() int64 { return rolloutClones.Load() }
