import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Panel, KV, Tag, Tape, Btn } from '../components/chrome'
import { api, API_BASE } from '../services/api'
import { useT } from '../i18n'

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
//
// Every formatter below takes `t` rather than closing over a catalog, so
// the words are chosen on each render and follow a language switch.

// Known timeline event kinds get a translated label; anything the backend
// starts emitting later falls back to its own identifier, readably spaced.
const TURNING_POINT_KEYS = {
  commander_first_cast: 'gs.kind_commander_first_cast',
  game_end: 'gs.kind_game_end',
}

const formatSeat = (t, seat) => {
  if (seat == null) return '—'
  if (seat < 0) return t('gs.draw')
  return t('gs.seat', { n: seat })
}

const formatVisits = (t, visit) => {
  const parts = []
  parts.push(t('gs.visits', { count: visit.visits ?? 0 }))
  parts.push(t('gs.cast_count', { count: visit.cast_count ?? 0 }))
  if (visit.in_zone_at_end) parts.push(t('gs.in_zone_at_end'))
  return parts.join(' · ')
}

const formatMVP = (t, mvp) => {
  const parts = [t('gs.cmc', { n: mvp.cmc ?? 0 }), t('gs.score', { n: mvp.score ?? 0 })]
  if (mvp.counters) {
    parts.push(t(mvp.counters === 1 ? 'gs.counters_one' : 'gs.counters', { count: mvp.counters }))
  }
  if (mvp.turn_played) parts.push(t('gs.played_turn', { n: mvp.turn_played }))
  return parts.join(' · ')
}

const dataSourceTag = (t, source) => {
  if (source === 'rich') return <Tag kind="good" solid>{t('gs.tag_rich')}</Tag>
  if (source === 'db_only') return <Tag kind="warn">{t('gs.tag_db_only')}</Tag>
  // An unrecognised discriminator is echoed back as-is; only the
  // absent case needs a word, and 'unknown' already has a key.
  return <Tag>{(source || '').toUpperCase() || t('dash.unknown')}</Tag>
}

function ZoneVisitsPanel({ visits }) {
  const t = useT()
  if (!visits?.length) {
    return (
      <Panel title={t('gs.zone_visits')} code="GS.A">
        <p className="muted">{t('gs.no_zone')}</p>
      </Panel>
    )
  }
  return (
    <Panel title={t('gs.zone_visits')} code="GS.A">
      <KV rows={visits.map((v) => [
        <>
          <strong>{v.commander_name}</strong> <span className="muted-2">({formatSeat(t, v.seat)})</span>
        </>,
        formatVisits(t, v),
      ])} />
    </Panel>
  )
}

function RegretCardsPanel({ cards }) {
  const t = useT()
  if (!cards?.length) {
    return (
      <Panel title={t('gs.regret')} code="GS.B">
        <p className="muted">{t('gs.no_regret')}</p>
      </Panel>
    )
  }
  return (
    <Panel title={t('gs.regret')} code="GS.B"
           right={<span className="muted-2 t-xs">{t('gs.regret_right')}</span>}>
      <KV rows={cards.map((c) => [
        <>
          <strong>{c.card_name}</strong> <span className="muted-2">({formatSeat(t, c.seat)})</span>
        </>,
        <>{t('gs.cmc', { n: c.cmc })} · <span className="muted-2">{c.reason}</span></>,
      ])} />
    </Panel>
  )
}

function MVPCardsPanel({ cards }) {
  const t = useT()
  if (!cards?.length) {
    return (
      <Panel title={t('gs.mvp')} code="GS.C">
        <p className="muted">{t('gs.no_mvp')}</p>
      </Panel>
    )
  }
  return (
    <Panel title={t('gs.mvp')} code="GS.C"
           right={<span className="muted-2 t-xs">{t('gs.mvp_right')}</span>}>
      <KV rows={cards.map((c) => [
        <>
          <strong>{c.card_name}</strong>
          {c.is_commander && <Tag kind="good" style={{ marginLeft: 6, fontSize: 8 }}>{t('gs.cmdr')}</Tag>}
          <span className="muted-2"> ({formatSeat(t, c.seat)})</span>
        </>,
        formatMVP(t, c),
      ])} />
    </Panel>
  )
}

function TurningPointsPanel({ points }) {
  const t = useT()
  if (!points?.length) {
    return (
      <Panel title={t('gs.turning')} code="GS.D">
        <p className="muted">{t('gs.no_turning')}</p>
      </Panel>
    )
  }
  return (
    <Panel title={t('gs.turning')} code="GS.D">
      <ol className="turning-points" style={{ paddingLeft: 18, margin: 0 }}>
        {points.map((p, i) => (
          <li key={i} style={{ marginBottom: 6 }}>
            <span className="muted-2" style={{ marginRight: 8 }}>T{p.turn}</span>
            <strong>
              {TURNING_POINT_KEYS[p.kind]
                ? t(TURNING_POINT_KEYS[p.kind])
                : p.kind.replace(/_/g, ' ').toUpperCase()}
            </strong>
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
  const t = useT()
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
        <Tape left={t('gs.title')} mid={t('common.loading')} right={t('gs.game_n', { id: gameId })} />
      </div>
    )
  }

  if (error) {
    return (
      <div className="container" style={{ padding: 24 }}>
        <Tape left={t('gs.title')} mid={t('gs.error')} right={t('gs.game_n', { id: gameId })} />
        <Panel title={t('gs.load_failed')} code="GS.!">
          <p>{error}</p>
        </Panel>
      </div>
    )
  }

  if (!summary) {
    return (
      <div className="container" style={{ padding: 24 }}>
        <Tape left={t('gs.title')} mid={t('gs.no_data')} right={t('gs.game_n', { id: gameId })} />
      </div>
    )
  }

  return (
    <div className="container game-summary" style={{ padding: 24, display: 'grid', gap: 16 }}>
      <Tape
        left={t('gs.game_n', { id: summary.game_id })}
        mid={
          <>
            {t(summary.turns === 1 ? 'gs.turns_one' : 'gs.turns', { count: summary.turns })}
            {' · '}
            {t('gs.winner', { seat: formatSeat(t, summary.winner) })}
            {summary.winner_name && <> · {summary.winner_name.toUpperCase()}</>}
            {summary.end_reason && <> · {summary.end_reason.toUpperCase()}</>}
          </>
        }
        right={
          <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
            <a
              href={`${API_BASE}/api/games/${summary.game_id}/summary.pdf`}
              data-testid="export-pdf"
              style={{ textDecoration: 'none' }}
            >
              <Btn sm ghost arrow={null} title={t('gs.export_tip')}>
                {t('gs.export_pdf')}
              </Btn>
            </a>
            {dataSourceTag(t, summary.data_source)}
          </span>
        }
      />
      <ZoneVisitsPanel visits={summary.commander_zone_visits} />
      <RegretCardsPanel cards={summary.regret_cards} />
      <MVPCardsPanel cards={summary.mvp_cards} />
      <TurningPointsPanel points={summary.turning_points} />
    </div>
  )
}
