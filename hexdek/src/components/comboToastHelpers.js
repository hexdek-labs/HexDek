// Pure helpers for the combo-fired SSE → toast bridge. Lives in a
// .js file so node --test can exercise them without a JSX
// transform.

// formatComboMessage takes the Data payload from a `game.combo_fired`
// EventBus event and produces the human-readable toast string.
//
// Expected shape (from internal/hexapi/showmatch.go publishNewCombos):
//   {
//     game_id, turn, seat,
//     controller_commander,
//     cycle_length,
//     participating_cards: ["Heliod, Sun-Crowned", "Walking Ballista", ...],
//     detected_by,
//   }
//
// Output examples:
//   "🎯 Combo fired: Heliod, Sun-Crowned + Walking Ballista"
//   "🎯 Combo fired: 5-card loop (Atraxa + 4 more)"
//   "🎯 Combo fired (Krenko)" — fallback when no participating cards
//
// Defensive against partial payloads: anything missing degrades to a
// shorter message rather than throwing.
export function formatComboMessage(data) {
  if (!data || typeof data !== 'object') return '🎯 Combo fired'

  const cards = Array.isArray(data.participating_cards)
    ? data.participating_cards.filter(c => typeof c === 'string' && c.length > 0)
    : []
  const controller = typeof data.controller_commander === 'string' && data.controller_commander
    ? data.controller_commander
    : null

  // 0 cards → fall back to controller name.
  if (cards.length === 0) {
    return controller ? `🎯 Combo fired (${controller})` : '🎯 Combo fired'
  }
  // 1-2 cards → list inline.
  if (cards.length <= 2) {
    return `🎯 Combo fired: ${cards.join(' + ')}`
  }
  // 3+ cards → cycle-length framing ("5-card loop") with the lead
  // card surfaced so spectators have at least one anchor name.
  const lead = cards[0]
  const remaining = cards.length - 1
  const cycleLen = typeof data.cycle_length === 'number' && data.cycle_length > 0
    ? data.cycle_length
    : cards.length
  return `🎯 Combo fired: ${cycleLen}-card loop (${lead} + ${remaining} more)`
}

// parseSSEEventData unboxes the SSE wrapper produced by the
// /api/events/{game_id}/stream handler. The handler wraps the
// EventBus event in {event, data, timestamp}; this helper strips
// the wrapper and surfaces just the `data` payload that
// formatComboMessage consumes.
//
// Two shapes are tolerated:
//   1. The full SSE wrapper: {event, data, timestamp}
//   2. The raw bus event: {Type, Data, Timestamp} (CamelCase from
//      WebSocket bridge — different transport, same content)
// Returns null when neither shape applies.
export function parseSSEEventData(payload) {
  if (!payload || typeof payload !== 'object') return null
  // SSE wrapper (snake_case from /api/events/.../stream).
  if (payload.event != null && 'data' in payload) {
    return payload.data
  }
  // WS wrapper (CamelCase from /ws/events).
  if (payload.Type != null && 'Data' in payload) {
    return payload.Data
  }
  return null
}

// isComboFiredEvent returns true when a raw SSE / WS payload
// represents a `game.combo_fired` event. Tolerant of both
// transport shapes (see parseSSEEventData).
export function isComboFiredEvent(payload) {
  if (!payload || typeof payload !== 'object') return false
  const type = payload.event ?? payload.Type
  return type === 'game.combo_fired'
}
