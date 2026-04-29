import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import {
  MOCK_PROFILE,
  MOCK_DECKS,
  MOCK_GAMES,
  MOCK_MATCHUPS,
  MOCK_LIVE_STATS,
  MOCK_DECK_ANALYSIS,
} from '../services/mock'

function useAsync(fetcher, fallback) {
  const [data, setData] = useState(fallback)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const refetch = useCallback(() => {
    setLoading(true)
    setError(null)
    fetcher()
      .then(setData)
      .catch((err) => {
        console.warn('API unavailable, using mock data:', err.message)
        setData(fallback)
        setError(err)
      })
      .finally(() => setLoading(false))
  }, [fetcher, fallback])

  useEffect(() => { refetch() }, [refetch])

  return { data, loading, error, refetch }
}

function timeAgo(dateStr) {
  const d = new Date(dateStr)
  const now = Date.now()
  const sec = Math.floor((now - d.getTime()) / 1000)
  if (sec < 60) return 'JUST NOW'
  if (sec < 3600) return `${Math.floor(sec / 60)}M AGO`
  if (sec < 86400) return `${Math.floor(sec / 3600)}H AGO`
  return `${Math.floor(sec / 86400)}D AGO`
}

function transformProfile(raw) {
  return {
    username: raw.username || 'OPERATOR',
    userId: raw.userId || 'USR.0001',
    joined: raw.joined || '04.2026',
    elo: raw.elo || 1500,
    eloChange: raw.eloChange || 0,
    tier: raw.tier || 'UNRANKED',
    streak: raw.streak || '—',
    primaryColor: raw.primaryColor || '—',
    archetype: raw.archetype || 'OBSERVER',
    percentile: raw.percentile || '—',
    gamesPlayed: raw.gamesPlayed || 0,
    winRate: typeof raw.winRate === 'number' ? Math.round(raw.winRate * 10) / 10 : 0,
    avgWinTurn: raw.avgWinTurn || 0,
    skills: raw.skills || [
      { name: 'DATA PENDING', value: 0 },
    ],
    achievements: raw.achievements || [
      { icon: '◇', name: 'FIRST BLOOD', unlocked: false },
      { icon: '◇', name: 'OBSERVER · WATCH 10 GAMES', unlocked: false },
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
      deck: winner?.split(',')[0]?.toUpperCase() || 'UNKNOWN',
      opponent: `VS ${losers.length}× AI`,
      result: g.winner >= 0 ? 'WIN' : 'DRAW',
      detail: `T${g.turns} / ${(g.end_reason || '').replace(/_/g, ' ').toUpperCase()}`,
      time: timeAgo(g.finished_at),
      kind: g.winner >= 0 ? 'ok' : 'warn',
    }
  })
}

function transformDecks(raw) {
  if (!Array.isArray(raw)) return MOCK_DECKS
  return raw.map((d, i) => ({
    id: d.id || d.file_path,
    slot: String(i + 1).padStart(2, '0'),
    name: (d.commander || d.name || 'UNKNOWN').toUpperCase(),
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

export function useMatchups() {
  return useAsync(
    api.getTournamentStats,
    MOCK_MATCHUPS,
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

export function useDeckAnalysis(deckId) {
  const fallback = MOCK_DECK_ANALYSIS[deckId] || MOCK_DECK_ANALYSIS.tinybones
  return useAsync(
    () => api.getDeckAnalysis(deckId),
    fallback,
  )
}

export function useDeckDetail(owner, id) {
  const fallback = null
  return useAsync(
    () => api.getDeck(`${owner}/${id}`),
    fallback,
  )
}
