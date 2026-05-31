package analytics

import "sort"

// ComputeThreatResolutions rolls up per-commander
// resolution-vs-no-resolution win-rate shift across analyses. See
// ThreatResolution for the metric definition.
//
// "Resolution" = at least one `commander_cast_from_command_zone`
// event with Seat == this commander's seat in this game. The first
// such event's turn (read from event order, not Details["turn"] —
// we walk forward and track currentTurn from `turn_start`) feeds
// AvgTurnToFirstResolution.
//
// Per-game dedupe by commander name (a commander seat is unique
// per-game in standard play; even Hivemind / mirror commanders count
// as one per-game sample).
//
// Returns the list sorted by absolute Shift descending (most-shifted
// commander first); no-signal rows (Confidence == "") tail
// alphabetically. Nil-safe; empty input returns nil.
func ComputeThreatResolutions(analyses []*GameAnalysis) []ThreatResolution {
	if len(analyses) == 0 {
		return nil
	}

	type acc struct {
		gamesPlayed       int
		gamesResolved     int
		winsResolved      int
		winsUnresolved    int
		firstResolutionTurnSum int
		firstResolutionGameCount int
		totalResolutions  int
	}
	byCmdr := make(map[string]*acc)

	// AnalyzeGame doesn't keep the event slice on GameAnalysis, so the
	// aggregator relies on per-card lifecycle data already materialized
	// on PlayerAnalysis. A commander resolution leaves a record in
	// CardsPlayed[name].TurnCast > 0 (its first successful resolution
	// turn). This is the same signal AggregateDecks::AvgTurnToFinisher
	// reads, so this metric stays consistent with the existing
	// turn-to-finisher rollup. Commander-tax recasts SHARE the same
	// CardPerformance row — TotalResolutions therefore captures the
	// "at least once" semantics (under-count vs. true recast count by
	// design; recast-count belongs in a separate metric).
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		for seatIdx := range ga.Players {
			pa := &ga.Players[seatIdx]
			name := pa.CommanderName
			if name == "" {
				continue
			}
			a, ok := byCmdr[name]
			if !ok {
				a = &acc{}
				byCmdr[name] = a
			}
			a.gamesPlayed++

			turn := findCardTurnCast(pa, name)
			if turn > 0 {
				a.gamesResolved++
				if pa.Won {
					a.winsResolved++
				}
				a.firstResolutionTurnSum += turn
				a.firstResolutionGameCount++
				a.totalResolutions++
			} else if pa.Won {
				a.winsUnresolved++
			}
		}
	}

	out := make([]ThreatResolution, 0, len(byCmdr))
	for name, a := range byCmdr {
		tr := ThreatResolution{
			CommanderName:    name,
			GamesPlayed:      a.gamesPlayed,
			GamesResolved:    a.gamesResolved,
			GamesUnresolved:  a.gamesPlayed - a.gamesResolved,
			WinsResolved:     a.winsResolved,
			WinsUnresolved:   a.winsUnresolved,
			TotalResolutions: a.totalResolutions,
		}
		if a.gamesResolved > 0 {
			tr.WinRateResolved = float64(a.winsResolved) / float64(a.gamesResolved)
		}
		if tr.GamesUnresolved > 0 {
			tr.WinRateUnresolved = float64(a.winsUnresolved) / float64(tr.GamesUnresolved)
		}
		if a.gamesResolved > 0 && tr.GamesUnresolved > 0 {
			tr.Shift = tr.WinRateResolved - tr.WinRateUnresolved
			tr.Confidence = keystoneConfidence(a.gamesResolved, tr.GamesUnresolved)
		}
		if a.firstResolutionGameCount > 0 {
			tr.AvgTurnToFirstResolution = float64(a.firstResolutionTurnSum) /
				float64(a.firstResolutionGameCount)
		}
		out = append(out, tr)
	}

	sort.SliceStable(out, func(i, j int) bool {
		signalI := out[i].Confidence != ""
		signalJ := out[j].Confidence != ""
		if signalI != signalJ {
			return signalI
		}
		if signalI {
			absI := out[i].Shift
			if absI < 0 {
				absI = -absI
			}
			absJ := out[j].Shift
			if absJ < 0 {
				absJ = -absJ
			}
			if absI != absJ {
				return absI > absJ
			}
			if out[i].GamesResolved != out[j].GamesResolved {
				return out[i].GamesResolved > out[j].GamesResolved
			}
		}
		return out[i].CommanderName < out[j].CommanderName
	})

	return out
}
