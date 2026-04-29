import { createContext, useContext, useEffect, useRef, useState, useCallback } from 'react'

const API_BASE = import.meta.env.VITE_API_URL ?? ''
const WS_URL = API_BASE.replace(/^http/, 'ws') + '/ws/live'
const CACHE_KEY = 'hexdek_live_cache'

function loadCached() {
  try {
    const raw = sessionStorage.getItem(CACHE_KEY)
    if (!raw) return null
    const c = JSON.parse(raw)
    if (c.stats && c.ts && c.stats.games_per_min > 0) {
      const elapsed = (Date.now() - c.ts) / 1000
      if (elapsed < 3600) {
        c.stats = { ...c.stats, games_played: Math.round(c.stats.games_played + (c.stats.games_per_min / 60) * elapsed) }
      }
    }
    return c
  } catch { return null }
}

function saveCache(patch) {
  try {
    const prev = JSON.parse(sessionStorage.getItem(CACHE_KEY) || '{}')
    sessionStorage.setItem(CACHE_KEY, JSON.stringify({ ...prev, ...patch, ts: Date.now() }))
  } catch {}
}

const LiveCtx = createContext(null)

// status: 'disconnected' | 'contacting' | 'initializing' | 'live'
export function LiveProvider({ children }) {
  const cached = useRef(loadCached()).current
  const [game, setGame] = useState(null)
  const [elo, setElo] = useState(cached?.elo || [])
  const [stats, setStats] = useState(cached?.stats || null)
  const [history, setHistory] = useState([])
  const [speed, setSpeed] = useState(1)
  const [status, setStatus] = useState('disconnected')
  const wsRef = useRef(null)
  const reconnectRef = useRef(null)
  const gotFirstStats = useRef(!!cached?.stats)

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    setStatus('contacting')
    const ws = new WebSocket(WS_URL)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('initializing')
      if (reconnectRef.current) {
        clearTimeout(reconnectRef.current)
        reconnectRef.current = null
      }
    }

    ws.onmessage = (evt) => {
      try {
        const { type, payload } = JSON.parse(evt.data)
        switch (type) {
          case 'game': setGame(payload); break
          case 'elo': setElo(payload || []); saveCache({ elo: payload }); break
          case 'stats':
            setStats(payload)
            saveCache({ stats: payload })
            if (!gotFirstStats.current) {
              gotFirstStats.current = true
              setStatus('live')
            }
            break
          case 'history': setHistory(payload || []); break
          case 'speed': setSpeed(payload?.multiplier ?? 1); break
          case 'pong': break
        }
        if (gotFirstStats.current && type !== 'pong') {
          setStatus('live')
        }
      } catch {}
    }

    ws.onclose = () => {
      setStatus('disconnected')
      wsRef.current = null
      gotFirstStats.current = false
      reconnectRef.current = setTimeout(connect, 2000)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [])

  useEffect(() => {
    connect()

    const ping = setInterval(() => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'ping' }))
      }
    }, 30000)

    return () => {
      clearInterval(ping)
      if (reconnectRef.current) clearTimeout(reconnectRef.current)
      if (wsRef.current) wsRef.current.close()
    }
  }, [connect])

  return (
    <LiveCtx.Provider value={{ game, elo, stats, history, speed, status }}>
      {children}
    </LiveCtx.Provider>
  )
}

export function useLiveSocket() {
  const ctx = useContext(LiveCtx)
  if (!ctx) throw new Error('useLiveSocket must be inside LiveProvider')
  return ctx
}
