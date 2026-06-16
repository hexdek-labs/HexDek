// DeckRadarPanel — the deck-page "DECK FINGERPRINT" panel (staging mock).
//
// Slots into the DeckArchive analysis tab. Shows THIS deck's radar fed from
// its live Freya analysis (freyaToRadar), plus 7174n1c's three real B3 decks
// as small-multiple reference fingerprints so they are viewable + comparable
// side by side. Staging-only — the caller gates render on VITE_STAGING.

import { Panel } from './chrome'
import DeckRadar, { DECK_AXES, freyaToRadar, REFERENCE_FINGERPRINTS } from './DeckRadar'

function ReferenceCard({ fp }) {
  return (
    <div style={{ flex: '1 1 150px', minWidth: 140, maxWidth: 220 }}>
      <DeckRadar values={fp.values} color={fp.color} size={170} showLabels={false} />
      <div style={{ marginTop: 2, textAlign: 'center' }}>
        <div className="t-xs" style={{ fontWeight: 700, color: fp.color }}>{fp.name}</div>
        <div className="t-xs muted-2" style={{ letterSpacing: '0.04em' }}>{fp.sub}</div>
      </div>
    </div>
  )
}

function AxisLegend() {
  return (
    <div className="t-xs muted-2" style={{ marginTop: 6, lineHeight: 1.5 }}>
      {DECK_AXES.map((a) => a.label).join(' · ')}
      <div style={{ marginTop: 2, opacity: 0.7 }}>
        Axes 0–1 normalized from Freya: power_percentile · avg_turn_to_four_mana ·
        draw/mana-grade/backup · keepable_hand_pct · interaction_avg_cmc+counters ·
        win_line_count+finishers.
      </div>
    </div>
  )
}

export default function DeckRadarPanel({ analysis, deckName }) {
  const live = freyaToRadar(analysis)

  return (
    <Panel code="04.R" title="DECK FINGERPRINT // STAGING MOCK">
      <div className="t-xs muted" style={{ marginBottom: 6 }}>
        Six-axis deck profile, sourced from Freya’s analysis (no invented metrics).
      </div>

      {live && (
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 10 }}>
          <div style={{ width: 240, maxWidth: '100%' }}>
            <DeckRadar values={live} color="var(--ok)" size={240} showLabels />
            <div className="t-xs" style={{ textAlign: 'center', fontWeight: 700, marginTop: 2 }}>
              {deckName || 'THIS DECK'}
            </div>
          </div>
        </div>
      )}

      <div className="t-xs muted" style={{ margin: '8px 0 4px', letterSpacing: '0.06em' }}>
        REFERENCE FINGERPRINTS — 7174N1C B3 DECKS (COMPARE)
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', justifyContent: 'space-around' }}>
        {REFERENCE_FINGERPRINTS.map((fp) => (
          <ReferenceCard key={fp.id} fp={fp} />
        ))}
      </div>

      <AxisLegend />
    </Panel>
  )
}
