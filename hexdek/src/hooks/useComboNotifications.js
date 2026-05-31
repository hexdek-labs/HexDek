// useComboNotifications — subscribes to the per-game SSE event
// stream (/api/events/{game_id}/stream) and surfaces
// `game.combo_fired` events as toasts on the spectator screen.
//
// The hook returns nothing. Side effects: opens an EventSource
// when gameId is set and emits `toast.combo(...)` per event.
// Closes / reopens whenever gameId changes.
//
// Why this pattern: the existing LiveProvider drives the WS-based
// /ws/live game-snapshot stream. We deliberately use a SEPARATE
// SSE stream for events so the spectator doesn't have to retrofit
// the LiveProvider for event-bus messages. SSE also reconnects
// automatically and works through proxies that block WS upgrades.

import { useEffect } from 'react'
import { toast } from '../components/Toast'
import { API_BASE } from '../services/api'
import {
  formatComboMessage,
  isComboFiredEvent,
  parseSSEEventData,
} from '../components/comboToastHelpers.js'

export default function useComboNotifications(gameId) {
  useEffect(() => {
    if (!gameId || gameId <= 0) return
    if (typeof EventSource === 'undefined') return // SSR / test guard

    const url = `${API_BASE}/api/events/${gameId}/stream`
    const es = new EventSource(url)

    // The SSE handler frames each event as { event, data, timestamp }
    // and emits with the event's `event` field as the SSE event-name.
    // We listen on `game.combo_fired` specifically.
    const handler = (e) => {
      try {
        const payload = JSON.parse(e.data)
        if (!isComboFiredEvent(payload)) return
        const data = parseSSEEventData(payload)
        toast.combo(formatComboMessage(data))
      } catch {
        // Bad payload — silently ignore. The next event will still
        // try to parse.
      }
    }

    es.addEventListener('game.combo_fired', handler)

    // Defensive: some SSE servers emit on the default `message`
    // channel instead of the typed channel. Listen there too.
    const messageHandler = (e) => {
      try {
        const payload = JSON.parse(e.data)
        if (!isComboFiredEvent(payload)) return
        toast.combo(formatComboMessage(parseSSEEventData(payload)))
      } catch {}
    }
    es.addEventListener('message', messageHandler)

    return () => {
      es.removeEventListener('game.combo_fired', handler)
      es.removeEventListener('message', messageHandler)
      es.close()
    }
  }, [gameId])
}
