import { useEffect, useState, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Panel, KV, Tape, Tag } from '../components/chrome'
import DeckShelf from '../components/DeckShelf'
import { api } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { countryFlagEmoji } from '../lib/flag'
import { useT } from '../i18n'

// PublicProfile — read-only player page at /profile/:owner.
// Aggregates data from three sources:
//   - /api/decks?owner=:owner       (deck collection + earliest imported_at as
//                                     "member since" proxy)
//   - /api/achievements/:owner      (TotalGames/TotalWins/streaks + badges)
//   - useLiveSocket().elo           (per-deck Rating/HexRating + W/L)
//
// The settings page at /profile (no owner) is a separate screen
// (Profile.jsx) — this one renders nothing of the auth user's local
// preferences, only public state.

const RARITY_COLOR = {
  common:   'var(--ink-2)',
  uncommon: 'var(--ok)',
  rare:     'var(--warn)',
  mythic:   'var(--danger)',
  secret:   'var(--accent)',
}

// Catalog PATHS, mirroring RARITY_COLOR above. badge.rarity is a WIRE
// value, and t() returns the raw key when a path is missing, so an
// unrecognised rarity would otherwise render the literal string
// "achievements.rarity.legendary" on screen. Unknown values fall back to
// the server's own word instead.
const RARITY_KEY = {
  common:   'achievements.rarity.common',
  uncommon: 'achievements.rarity.uncommon',
  rare:     'achievements.rarity.rare',
  mythic:   'achievements.rarity.mythic',
  secret:   'achievements.rarity.secret',
}

// Mirrors decks.slice(0, SHELF_LIMIT) below. The count used to be written
// twice -- once in the slice and once in the "SHOWING 60 /" line -- which
// is a label that silently lies the moment the slice changes.
const SHELF_LIMIT = 60

function fmtMemberSince(decks) {
  let earliest = null
  for (const d of decks || []) {
    const t = d.imported_at ? new Date(d.imported_at) : null
    if (t && !isNaN(t) && (!earliest || t < earliest)) earliest = t
  }
  if (!earliest) return '—'
  return earliest.toISOString().slice(0, 10)
}

function BadgeTile({ badge, earned, awardedAt }) {
  const t = useT()
  const rarityColor = RARITY_COLOR[badge.rarity] || 'var(--ink-2)'
  const opacity = earned ? 1 : 0.32
  const filter = earned ? 'none' : 'grayscale(1)'
  // op.badge_tip_earned is '{name} — {desc}\nEarned {date}' and
  // op.badge_tip_locked is '{name} — {desc} (locked)' — byte-for-byte the
  // two branches this nested template was building. The sibling screen
  // already owned both; name and desc are server values, echoed.
  const tip = earned
    ? t('op.badge_tip_earned', {
        name: badge.name,
        desc: badge.description,
        date: new Date(awardedAt).toISOString().slice(0, 10),
      })
    : t('op.badge_tip_locked', { name: badge.name, desc: badge.description })
  return (
    <div
      title={tip}
      style={{
        border: `1px solid ${earned ? rarityColor : 'var(--rule-2)'}`,
        background: 'var(--panel)',
        padding: '10px 8px',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 4,
        opacity,
      }}
    >
      <span style={{ fontSize: 24, lineHeight: 1, filter }}>{badge.icon || '◆'}</span>
      <span style={{
        fontSize: 9, fontWeight: 700, letterSpacing: '0.08em',
        color: earned ? 'var(--ink)' : 'var(--ink-3)',
        textAlign: 'center', lineHeight: 1.15,
      }}>
        {badge.name}
      </span>
      <span style={{
        fontSize: 8, letterSpacing: '0.1em', textTransform: 'uppercase',
        color: earned ? rarityColor : 'var(--ink-3)',
      }}>
        {RARITY_KEY[badge.rarity] ? t(RARITY_KEY[badge.rarity]) : badge.rarity}
      </span>
    </div>
  )
}

export default function PublicProfile() {
  const t = useT()
  const { owner } = useParams()
  const navigate = useNavigate()
  const { elo } = useLiveSocket()

  const [decks, setDecks] = useState([])
  const [decksLoading, setDecksLoading] = useState(true)
  const [achievements, setAchievements] = useState(null)
  const [achLoading, setAchLoading] = useState(true)
  const [profile, setProfile] = useState(null)

  useEffect(() => {
    if (!owner) return
    setDecksLoading(true)
    api.getDecks({ owner })
      .then(d => setDecks(Array.isArray(d) ? d : []))
      .catch(() => setDecks([]))
      .finally(() => setDecksLoading(false))
  }, [owner])

  useEffect(() => {
    if (!owner) return
    setAchLoading(true)
    api.getAchievements(owner)
      .then(setAchievements)
      .catch(() => setAchievements(null))
      .finally(() => setAchLoading(false))
  }, [owner])

  useEffect(() => {
    if (!owner) return
    api.getOwnerProfile(owner)
      .then(setProfile)
      .catch(() => setProfile(null))
  }, [owner])

  // Filter ELO entries to this owner so per-deck rating/record overlays
  // populate on the shelf, plus the aggregate W/L summary.
  const ownerElo = useMemo(
    () => (elo || []).filter(e => e.owner?.toLowerCase() === owner?.toLowerCase()),
    [elo, owner],
  )

  const eloByDeckId = useMemo(() => {
    const m = {}
    for (const e of ownerElo) {
      if (e.deck_id) {
        m[`${e.owner}/${e.deck_id}`] = e
        m[e.deck_id] = e
      }
    }
    return m
  }, [ownerElo])

  // Aggregate W/L: prefer the achievements snapshot (TotalGames /
  // TotalWins are the canonical owner-level counts the engine writes
  // per finished game). Fall back to summing per-deck ELO entries when
  // achievements is empty (new player, no game data yet).
  const aggregate = useMemo(() => {
    const a = achievements
    if (a && a.total_games > 0) {
      return {
        games: a.total_games,
        wins: a.total_wins,
        losses: Math.max(0, a.total_games - a.total_wins),
        winRate: a.total_games > 0 ? (a.total_wins / a.total_games) * 100 : 0,
        source: 'achievements',
      }
    }
    let games = 0, wins = 0, losses = 0
    for (const e of ownerElo) {
      games += e.games || 0
      wins += e.wins || 0
      losses += e.losses || 0
    }
    return {
      games, wins, losses,
      winRate: games > 0 ? (wins / games) * 100 : 0,
      source: ownerElo.length > 0 ? 'elo' : 'none',
    }
  }, [achievements, ownerElo])

  // Best ELO across this owner's decks (HexRating preferred — it's the
  // engine's display rating; falls back to TrueSkill conservative).
  const bestRating = useMemo(() => {
    let best = null
    for (const e of ownerElo) {
      if ((e.games || 0) < 5) continue // require minimum sample
      const r = e.hex_rating || e.rating || 0
      if (!best || r > best.rating) best = { rating: r, deck: e }
    }
    return best
  }, [ownerElo])

  if (!owner) {
    return (
      <>
        <Tape left={t('pp.profile')} mid={t('pp.missing_owner')} right="DOC HX-500" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">{t('pp.no_owner_url')}</div>
        </div>
      </>
    )
  }

  const memberSince = fmtMemberSince(decks)
  const upperOwner = owner.toUpperCase()
  const flag = countryFlagEmoji(profile?.country)
  const earnedById = useMemo(() => {
    const m = {}
    for (const b of achievements?.badges || []) m[b.id] = b
    return m
  }, [achievements])
  const catalog = achievements?.catalog || []

  return (
    <>
      <Tape
        left={t('pp.tape_left', { name: `${upperOwner}${flag ? ' ' + flag : ''}` })}
        mid={achLoading || decksLoading ? t('common.loading') : t('pp.tape_mid')}
        right="DOC HX-500"
      />

      <div style={{ padding: 18, flex: 1, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>

        {/* Header — name + summary stats */}
        <Panel
          code="USR.0"
          title={
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              {flag && (
                <span
                  className="player-flag"
                  title={profile?.country}
                  aria-label={t('op.country_aria', { code: profile?.country })}
                  style={{ fontSize: 18, lineHeight: 1 }}
                >
                  {flag}
                </span>
              )}
              {t('pp.player_record')}
            </span>
          }
          right={<Tag solid>{upperOwner}</Tag>}
        >
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }}>
            <KV rows={[
              [t('op.kv_display_name'),
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                  {flag && <span style={{ fontSize: 14, lineHeight: 1 }}>{flag}</span>}
                  {upperOwner}
                </span>,
              ],
              [t('op.kv_country'), profile?.country || '—'],
              [t('op.kv_member_since'), memberSince, 'member_since'],
              [t('op.kv_decks'), String(decks.length)],
              [t('pp.kv_opponents'), String(achievements?.opponents_faced ?? '—')],
            ]} />
            <KV rows={[
              [t('op.kv_total_games'), aggregate.games > 0 ? String(aggregate.games) : '—', 'games'],
              [t('op.kv_record'),
                aggregate.games > 0
                  ? <span>
                      <span style={{ color: 'var(--ok)' }}>{t('op.wins_suffix', { count: aggregate.wins })}</span>
                      <span className="muted"> / </span>
                      <span style={{ color: 'var(--danger)' }}>{t('op.losses_suffix', { count: aggregate.losses })}</span>
                    </span>
                  : '—',
                'record',
              ],
              [t('op.kv_win_rate'), aggregate.games > 0 ? `${aggregate.winRate.toFixed(1)}%` : '—', 'win_rate'],
              [t('op.kv_best_elo'),
                bestRating
                  ? <span>
                      <span style={{ fontWeight: 700 }}>{Math.round(bestRating.rating)}</span>
                      <span className="muted" style={{ marginLeft: 6, fontSize: 9 }}>
                        {bestRating.deck.commander?.toUpperCase()}
                      </span>
                    </span>
                  : '—',
                'best_elo',
              ],
            ]} />
          </div>
          {(achievements?.current_win_streak > 0 || achievements?.max_win_streak > 0) && (
            <div style={{ marginTop: 10, display: 'flex', gap: 14, fontSize: 10, letterSpacing: '0.08em', color: 'var(--ink-2)', textTransform: 'uppercase' }}>
              <span>{t('pp.current_streak')} <span style={{ color: 'var(--ok)', fontWeight: 700 }}>{t('op.wins_suffix', { count: achievements?.current_win_streak || 0 })}</span></span>
              <span className="muted-2">·</span>
              <span>{t('pp.best_streak')} <span style={{ color: 'var(--accent)', fontWeight: 700 }}>{t('op.wins_suffix', { count: achievements?.max_win_streak || 0 })}</span></span>
            </div>
          )}
        </Panel>

        {/* Achievement badges */}
        <Panel code="USR.1" title={t('achievements.title')} right={
          <span className="t-xs muted">
            {achLoading
              ? t('dka.loading_ell')
              : t('op.earned_count', { earned: achievements?.badges?.length || 0, total: catalog.length })}
          </span>
        }>
          {achLoading ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('pp.loading_achievements')}<span className="blink">_</span>
            </div>
          ) : catalog.length === 0 ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('op.no_catalog')}
            </div>
          ) : (
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(110px, 1fr))',
              gap: 8,
            }}>
              {catalog.map(badge => {
                const earned = earnedById[badge.id]
                return (
                  <BadgeTile
                    key={badge.id}
                    badge={badge}
                    earned={!!earned}
                    awardedAt={earned?.awarded_at}
                  />
                )
              })}
            </div>
          )}
        </Panel>

        {/* Deck collection */}
        <Panel code="USR.2" title={t('pp.owner_decks', { name: upperOwner })} right={
          <span className="t-xs muted">
            {t(decks.length === 1 ? 'op.builds_one' : 'op.builds', { count: decks.length })}
          </span>
        }>
          {decksLoading ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('op.loading_decks')}<span className="blink">_</span>
            </div>
          ) : decks.length === 0 ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('pp.no_public_decks')}
            </div>
          ) : (
            <DeckShelf
              decks={decks.slice(0, SHELF_LIMIT)}
              eloByDeckId={eloByDeckId}
              navigate={navigate}
            />
          )}
          {decks.length > SHELF_LIMIT && (
            <div className="t-xs muted" style={{ textAlign: 'center', marginTop: 10 }}>
              {t('pp.showing_n', { shown: SHELF_LIMIT, total: decks.length })}
            </div>
          )}
        </Panel>
      </div>
    </>
  )
}
