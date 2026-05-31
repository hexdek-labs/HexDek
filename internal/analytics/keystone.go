package analytics

import "sort"

// ComputeKeystoneImpacts walks the analyses and rolls up per-card
// cast-vs-not-cast win-rate shift. See CardKeystoneImpact for the
// metric definition.
//
// Per-game dedupe: the same card may appear multiple times in the
// same seat's CardsPlayed (rare — usually one entry per name) or in
// multiple seats (mirror). We dedupe by (gameIdx, cardName) so a
// card cast in two seats of the same game still counts as ONE
// `cast` sample with the win flag set if ANY of those seats won.
// "Not cast" is the per-game complement: if any seat cast the card
// the game counts as cast, never as not-cast. This matches the
// metric's intent — the question is whether the card hitting play
// correlated with winning, not whether each individual cast event
// did.
//
// Lands and tokens are excluded entirely from both buckets — neither
// represents "casting" the card.
//
// Returns the list sorted by absolute Shift descending (most
// impactful card first) so reports can take the top-N directly.
// Cards with Confidence == "" (zero in either bucket) are appended
// at the end in alphabetical order; they have no shift signal but
// callers may still want to surface them as "always cast" / "never
// cast" annotations.
func ComputeKeystoneImpacts(analyses []*GameAnalysis) []CardKeystoneImpact {
	if len(analyses) == 0 {
		return nil
	}

	type acc struct {
		gamesCast    int
		gamesNotCast int
		winsCast     int
		winsNotCast  int
	}
	byCard := make(map[string]*acc)

	// Per-game dedupe key: which cards were cast this game (any seat),
	// and which seats won.
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		// Sets per game.
		castThisGame := make(map[string]bool)
		// For each card, did any winning seat cast it?
		castAndWonThisGame := make(map[string]bool)
		// For the not-cast bucket, we still need to know if the
		// controller-of-record won — but "controller-of-record" for a
		// card that never got cast is ambiguous. The natural reading
		// of "the card was in deck X and deck X didn't cast it" is to
		// credit the win to deck X's controller. So: for each card
		// present in seat S's CardsPlayed with TurnCast == 0, if seat
		// S won, count toward winsNotCast.
		//
		// Mirror seats (same card in 2 seats, both with TurnCast == 0)
		// would double-count without per-game dedupe. We track the
		// (cardName -> any-not-cast-controller-won) flag instead of
		// summing.
		notCastThisGame := make(map[string]bool)
		notCastAndWonThisGame := make(map[string]bool)

		for seatIdx := range ga.Players {
			pa := &ga.Players[seatIdx]
			won := pa.Won
			for i := range pa.CardsPlayed {
				cp := &pa.CardsPlayed[i]
				if cp.IsLand || cp.IsToken || isTokenByName(cp.Name) {
					continue
				}
				if cp.TurnCast > 0 {
					castThisGame[cp.Name] = true
					if won {
						castAndWonThisGame[cp.Name] = true
					}
				} else {
					// TurnCast == 0: card had SOME event (countered,
					// graveyard interaction, etc.) but was never cast.
					// Counts as "in deck, not cast this game".
					notCastThisGame[cp.Name] = true
					if won {
						notCastAndWonThisGame[cp.Name] = true
					}
				}
			}
		}

		// "Cast" bucket wins over "not cast" if both appeared this
		// game (e.g. seat A cast it, seat B mirror-deck never cast it).
		// The card hit play in this game → cast bucket. Per-game
		// semantics.
		for name := range castThisGame {
			a, ok := byCard[name]
			if !ok {
				a = &acc{}
				byCard[name] = a
			}
			a.gamesCast++
			if castAndWonThisGame[name] {
				a.winsCast++
			}
		}
		for name := range notCastThisGame {
			if castThisGame[name] {
				continue // already counted as a cast game
			}
			a, ok := byCard[name]
			if !ok {
				a = &acc{}
				byCard[name] = a
			}
			a.gamesNotCast++
			if notCastAndWonThisGame[name] {
				a.winsNotCast++
			}
		}
	}

	out := make([]CardKeystoneImpact, 0, len(byCard))
	for name, a := range byCard {
		k := CardKeystoneImpact{
			Name:         name,
			GamesCast:    a.gamesCast,
			GamesNotCast: a.gamesNotCast,
			WinsCast:     a.winsCast,
			WinsNotCast:  a.winsNotCast,
		}
		if a.gamesCast > 0 {
			k.WinRateWhenCast = float64(a.winsCast) / float64(a.gamesCast)
		}
		if a.gamesNotCast > 0 {
			k.WinRateWhenNotCast = float64(a.winsNotCast) / float64(a.gamesNotCast)
		}
		if a.gamesCast > 0 && a.gamesNotCast > 0 {
			k.Shift = k.WinRateWhenCast - k.WinRateWhenNotCast
			k.Confidence = keystoneConfidence(a.gamesCast, a.gamesNotCast)
		}
		out = append(out, k)
	}

	// Two-stage sort: rows with signal first (by absolute shift desc,
	// then by GamesCast desc as tiebreak), then no-signal rows
	// alphabetically.
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
			if out[i].GamesCast != out[j].GamesCast {
				return out[i].GamesCast > out[j].GamesCast
			}
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// keystoneConfidence maps the smaller bucket size to a confidence
// label. High (≥20) → enough samples for a credible difference;
// Medium (≥5) → suggestive, footnote-worthy; Low (<5) → noise.
func keystoneConfidence(castN, notCastN int) string {
	min := castN
	if notCastN < min {
		min = notCastN
	}
	switch {
	case min >= 20:
		return "high"
	case min >= 5:
		return "medium"
	default:
		return "low"
	}
}
