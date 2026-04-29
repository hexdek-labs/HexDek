package analytics

// WeaknessSignals captures cross-game vulnerability patterns for a
// specific deck (identified by commander name). Derived from Heimdall
// analytics over multiple games.
type WeaknessSignals struct {
	VulnerableToWipes   float64
	VulnerableToCounter float64
	SlowToClose         float64
	ManaScrew           float64
	OverExtends         float64
}

// DeriveWeakness aggregates game analyses for a single commander and
// returns weakness signals. Requires at least 10 games for meaningful
// signal.
func DeriveWeakness(analyses []*GameAnalysis, commanderName string) *WeaknessSignals {
	if len(analyses) < 10 {
		return nil
	}

	var losses, wipeLoss, counterLoss, stallLoss, manaLoss, overextendLoss int
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		for _, pa := range ga.Players {
			if pa.CommanderName != commanderName {
				continue
			}
			if pa.Won {
				continue
			}
			losses++

			if ga.FirstWipe > 0 && pa.TurnOfDeath > 0 && pa.TurnOfDeath-ga.FirstWipe <= 3 {
				wipeLoss++
			}

			if pa.SpellsCountered >= 2 {
				counterLoss++
			}

			if ga.StallIndicators != nil && ga.StallIndicators.HitTurnCap {
				stallLoss++
			}

			if pa.LandsPlayed <= 3 && pa.TurnOfDeath > 0 && pa.TurnOfDeath <= 8 {
				manaLoss++
			}

			if pa.PeakBoardSize >= 6 && pa.CardsInHand <= 1 {
				overextendLoss++
			}
		}
	}

	if losses < 5 {
		return nil
	}

	d := float64(losses)
	return &WeaknessSignals{
		VulnerableToWipes:   float64(wipeLoss) / d,
		VulnerableToCounter: float64(counterLoss) / d,
		SlowToClose:         float64(stallLoss) / d,
		ManaScrew:           float64(manaLoss) / d,
		OverExtends:         float64(overextendLoss) / d,
	}
}
