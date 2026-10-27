import { useState, useEffect, useCallback, useRef } from 'react'
import { api } from '../services/api'
import {
  MOCK_PROFILE,
  MOCK_DECKS,
  MOCK_GAMES,
  MOCK_LIVE_STATS,
} from '../services/mock'

function useAsync(fetcher, fallback) {
  const [data, setData] = useState(fallback)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const fetcherRef = useRef(fetcher)
  const fallbackRef = useRef(fallback)
  fetcherRef.current = fetcher
  fallbackRef.current = fallback

  const refetch = useCallback(() => {
    setLoading(true)
    setError(null)
    fetcherRef.current()
      .then(setData)
      .catch((err) => {
        console.warn('API unavailable, using mock data:', err.message)
        setData(fallbackRef.current)
        setError(err)
      })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { refetch() }, [refetch])

  return { data, loading, error, refetch }
}

function transformProfile(raw) {
  return {
    username: raw.username || null,
    userId: raw.userId || '—',
    joined: raw.joined || '—',
    elo: raw.elo || 0,
    eloChange: raw.eloChange || 0,
    tier: raw.tier || null,
    streak: raw.streak || '—',
    primaryColor: raw.primaryColor || '—',
    archetype: raw.archetype || null,
    percentile: raw.percentile || '—',
    gamesPlayed: raw.gamesPlayed || 0,
    winRate: typeof raw.winRate === 'number' ? Math.round(raw.winRate * 10) / 10 : 0,
    avgWinTurn: raw.avgWinTurn || 0,
    skills: raw.skills || [
      { name: null, value: 0 },
    ],
    achievements: raw.achievements || [
      { icon: '◇', nameKey: 'dash.ach_first_blood', unlocked: false },
      { icon: '◇', nameKey: 'dash.ach_observer', unlocked: false },
    ],
  }
}

function transformGames(raw) {
  if (!Array.isArray(raw)) return MOCK_GAMES
  return raw.map((g) => {
    const winner = g.winner >= 0 && g.winner < (g.commanders?.length || 0)
      ? g.commanders[g.winner]
      : 'DRAW'
    const losers = (g.commanders || []).filter((_, i) => i !== g.winner)
    return {
      id: `G.${g.game_id}`,
      deck: winner?.split(',')[0]?.toUpperCase() || null,
      opponentCount: losers.length,
      won: g.winner >= 0,
      turns: g.turns,
      endReason: (g.end_reason || '').replace(/_/g, ' ').toUpperCase(),
      finishedAt: Date.parse(g.finished_at),
      kind: g.winner >= 0 ? 'ok' : 'warn',
    }
  })
}

function transformDecks(raw) {
  if (!Array.isArray(raw)) return MOCK_DECKS
  return raw.map((d, i) => ({
    id: d.id || d.file_path,
    slot: String(i + 1).padStart(2, '0'),
    name: (d.commander || d.name || '').toUpperCase() || null,
    color: d.color || '?',
    power: '?',
    bracket: d.bracket || '?',
    winRate: '—',
    archetype: '—',
    cardCount: d.card_count || 0,
    commanderCard: d.commander_card || d.commander || '',
    owner: d.owner,
    filePath: d.file_path,
  }))
}

function transformLiveStats(raw) {
  return {
    activeForges: 1,
    totalGames: raw.games_played || 0,
    gamesPerMin: raw.games_per_min || 0,
    userContrib: raw.games_played || 0,
    userRank: 'LOCAL',
    throughput: [],
  }
}

export function useProfile() {
  return useAsync(
    () => api.getProfile().then(transformProfile),
    MOCK_PROFILE,
  )
}

export function useDecks() {
  return useAsync(
    () => api.getDecks().then(transformDecks),
    MOCK_DECKS,
  )
}

export function useGames(limit = 20) {
  return useAsync(
    () => api.getGames(limit).then(transformGames),
    MOCK_GAMES,
  )
}

export function useLiveStats() {
  return useAsync(
    () => api.getLiveStats().then(transformLiveStats),
    MOCK_LIVE_STATS,
  )
}

export function useLiveELO() {
  return useAsync(
    () => api.getLiveELO(),
    [],
  )
}
