import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Panel, KV, Tag, Tape } from '../components/chrome'
import { api } from '../services/api'

// GameSummary renders the consolidated post-game blob from
// GET /api/games/{id}/summary — the read surface backed by the R60
// heimdall snapshot pipeline (#177 schema, #180 persistence, #183
// replay, #186 live snapshot). Four sections:
//   * Commander zone visits — who came back, who stuck
//   * Regret cards — drew but never cast
//   * MVP cards — top permanents at game end
//   * Turning points — commander_first_cast + game_end timeline
//
// The screen shows a tape with the data_source discriminator so
// users know whether they're looking at the rich path (snapshot
// hydrated) or the db_only fallback. Empty signal arrays render
// as a single "no data" row instead of a blank panel.

const formatSeat = (seat) => {
  if (seat == null) return '—'
  if (seat < 0) return 'DRAW'
  return `SEAT ${seat}`
}

const formatVisits = (visit) => {
  const parts = []
  parts.push(`${visit.visits ?? 0}× visited`)
  parts.push(`cast ${visit.cast_count ?? 0}×`)
  if (visit.in_zone_at_end) parts.push('in zone at end')
  return parts.join(' · ')
}

const formatMVP = (mvp) => {
  const parts = [`CMC ${mvp.cmc ?? 0}`, `score ${mvp.score ?? 0}`]
  if (mvp.counters) parts.push(`+${mvp.counters} counters`)
  if (mvp.turn_played) parts.push(`played T${mvp.turn_played}`)
  return parts.join(' · ')
}

const dataSourceTag = (source) => {
  if (source === 'rich') return <Tag kind="good" solid>RICH</Tag>
  if (source === 'db_only') return <Tag kind="warn">METADATA ONLY</Tag>
  return <Tag>{(source || 'unknown').toUpperCase()}</Tag>
}

function ZoneVisitsPanel({ visits }) {
  if (!visits?.length) {
    return (
      <Panel title="COMMANDER ZONE VISITS" code="GS.A">
        <p className="muted">No commander zone activity recorded.</p>
      </Panel>
    )
  }
  return (
    <Panel title="COMMANDER ZONE VISITS" code="GS.A">
      <KV rows={visits.map((v) => [
        <>
          <strong>{v.commander_name}</strong> <span className="muted-2">({formatSeat(v.seat)})</span>
        </>,
        formatVisits(v),
      ])} />
    </Panel>
  )
}

function RegretCardsPanel({ cards }) {
  if (!cards?.length) {
    return (
      <Panel title="REGRET CARDS" code="GS.B">
        <p className="muted">No stranded-in-hand cards — everyone deployed their hand.</p>
      </Panel>
    )
  }
  return (
    <Panel title="REGRET CARDS" code="GS.B"
           right={<span className="muted-2 t-xs">DREW · NEVER CAST</span>}>
      <KV rows={cards.map((c) => [
        <>
          <strong>{c.card_name}</strong> <span className="muted-2">({formatSeat(c.seat)})</span>
        </>,
        <>CMC {c.cmc} · <span className="muted-2">{c.reason}</span></>,
      ])} />
    </Panel>
  )
}

function MVPCardsPanel({ cards }) {
  if (!cards?.length) {
    return (
      <Panel title="MVP CARDS" code="GS.C">
        <p className="muted">No standout permanents at game end.</p>
      </Panel>
    )
  }
  return (
    <Panel title="MVP CARDS" code="GS.C"
           right={<span className="muted-2 t-xs">TOP PERMANENTS · SCORED</span>}>
      <KV rows={cards.map((c) => [
        <>
          <strong>{c.card_name}</strong>
          {c.is_commander && <Tag kind="good" style={{ marginLeft: 6, fontSize: 8 }}>CMDR</Tag>}
          <span className="muted-2"> ({formatSeat(c.seat)})</span>
        </>,
        formatMVP(c),
      ])} />
    </Panel>
  )
}

function TurningPointsPanel({ points }) {
  if (!points?.length) {
    return (
      <Panel title="TURNING POINTS" code="GS.D">
        <p className="muted">No timeline events available.</p>
      </Panel>
    )
  }
  return (
    <Panel title="TURNING POINTS" code="GS.D">
      <ol className="turning-points" style={{ paddingLeft: 18, margin: 0 }}>
        {points.map((p, i) => (
          <li key={i} style={{ marginBottom: 6 }}>
            <span className="muted-2" style={{ marginRight: 8 }}>T{p.turn}</span>
            <strong>{p.kind.replace(/_/g, ' ').toUpperCase()}</strong>
            <span className="muted" style={{ marginLeft: 8 }}>
              {p.description}
            </span>
          </li>
        ))}
      </ol>
    </Panel>
  )
}

export default function GameSummary() {
  const { gameId } = useParams()
  const [summary, setSummary] = useState(null)
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!gameId) return
    let cancelled = false
    api.getGameSummary(gameId)
      .then((s) => {
        if (cancelled) return
        setSummary(s)
        setError(null)
        setLoading(false)
      })
      .catch((e) => {
        if (cancelled) return
        setError(e?.message || String(e))
        setLoading(false)
      })
    return () => { cancelled = true }
  }, [gameId])

  if (loading) {
    return (
      <div className="container" style={{ padding: 24 }}>
        <Tape left="GAME SUMMARY" mid="LOADING" right={`GAME #${gameId}`} />
      </div>
    )
  }

  if (error) {
    return (
      <div className="container" style={{ padding: 24 }}>
        <Tape left="GAME SUMMARY" mid="ERROR" right={`GAME #${gameId}`} />
        <Panel title="LOAD FAILED" code="GS.!">
          <p>{error}</p>
        </Panel>
      </div>
    )
  }

  if (!summary) {
    return (
      <div className="container" style={{ padding: 24 }}>
        <Tape left="GAME SUMMARY" mid="NO DATA" right={`GAME #${gameId}`} />
      </div>
    )
  }

  return (
    <div className="container game-summary" style={{ padding: 24, display: 'grid', gap: 16 }}>
      <Tape
        left={`GAME #${summary.game_id}`}
        mid={
          <>
            {summary.turns} TURNS · WINNER {formatSeat(summary.winner)}
            {summary.winner_name && <> · {summary.winner_name.toUpperCase()}</>}
            {summary.end_reason && <> · {summary.end_reason.toUpperCase()}</>}
          </>
        }
        right={dataSourceTag(summary.data_source)}
      />
      <ZoneVisitsPanel visits={summary.commander_zone_visits} />
      <RegretCardsPanel cards={summary.regret_cards} />
      <MVPCardsPanel cards={summary.mvp_cards} />
      <TurningPointsPanel points={summary.turning_points} />
    </div>
  )
}
