// Replay video export.
//
// Exports a completed game as a watchable MP4 / WebM file. Two
// layers:
//
//   1. Pure helpers (this module): build the frame plan, extract a
//      per-turn render-ready summary, lay out the static text/rects
//      we'll paint, pick the best browser-supported video MIME, and
//      build the download filename.
//
//   2. Canvas-driven recorder (recordReplayVideo, also here): owns
//      the actual MediaRecorder dance — captureStream off a regular
//      <canvas>, draw each frame with raf-paced timing, stop, and
//      return the resulting Blob. Browser-only; the React component
//      hands it the canvas it already keeps onscreen for previewing.
//
// Why no server round-trip: ffmpeg server-side is the "right" answer
// for production-grade video, but it's a big chunk of infra and one
// PR's worth of work doesn't justify it. Browser MediaRecorder
// produces decent webm everywhere and mp4 on recent Safari — good
// enough for "let me share this game with a friend."

// Default cadence. 2.5 s per turn at 30 fps gives ~75 frames per
// turn — enough to read the board state without making a 40-turn
// game stretch into a 5-minute video. fps=30 keeps file sizes
// manageable.
export const DEFAULT_SECONDS_PER_TURN = 2.5
export const DEFAULT_FPS = 30
export const DEFAULT_VIDEO_WIDTH = 1280
export const DEFAULT_VIDEO_HEIGHT = 720

// buildFramePlan converts a timeline into a list of "render this
// snapshot from startMs for durationMs" entries. The recorder walks
// the plan, paints each frame at its scheduled moment, and stops at
// totalMs.
//
// Returns { totalMs, fps, frames: [...] }. `frames` is one entry per
// timeline snapshot (NOT one entry per video frame) — the recorder
// holds each snapshot's frame for `frame.durationMs` milliseconds.
export function buildFramePlan(timeline, options = {}) {
  const secondsPerTurn = Number.isFinite(options.secondsPerTurn) ? options.secondsPerTurn : DEFAULT_SECONDS_PER_TURN
  const fps = Number.isFinite(options.fps) ? options.fps : DEFAULT_FPS
  if (!Array.isArray(timeline) || timeline.length === 0) {
    return { totalMs: 0, fps, frames: [] }
  }
  const turnMs = Math.round(secondsPerTurn * 1000)
  const frames = []
  let cursor = 0
  for (let i = 0; i < timeline.length; i++) {
    frames.push({
      idx: i,
      turn: timeline[i]?.turn ?? (i + 1),
      startMs: cursor,
      durationMs: turnMs,
      snapshot: timeline[i],
    })
    cursor += turnMs
  }
  return { totalMs: cursor, fps, frames }
}

// frameSummary extracts the render-ready data for one snapshot.
// Pulled out so the layout logic can be tested without needing to
// shape a full timeline snapshot in every test.
export function frameSummary(snapshot, commanders = []) {
  const seats = snapshot?.seats || []
  return {
    turn: snapshot?.turn ?? null,
    activeSeat: snapshot?.active_seat ?? -1,
    seats: seats.map((s, i) => ({
      seat: i,
      commander: commanderShort(commanders[i]),
      life:       intOr(s.life, 0),
      hand:       intOr(s.hand_size, 0),
      library:    intOr(s.library_size, 0),
      graveyard:  intOr(s.gy_size, 0),
      battlefield: (s.battlefield || []).length,
      lost: !!s.lost,
    })),
    eventCount: (snapshot?.events || []).length,
  }
}

function commanderShort(name) {
  if (typeof name !== 'string' || name.length === 0) return 'UNKNOWN'
  return name.split('//')[0].trim().split(',')[0].toUpperCase()
}

function intOr(v, fallback) {
  return Number.isFinite(v) ? Math.trunc(v) : fallback
}

// composeFrameLayout takes a frame summary plus canvas dimensions
// and returns a list of positioned items the recorder will paint.
// Pure layout math — no Canvas2D calls. Each item is one of:
//
//   { type: 'rect', x, y, w, h, fill, stroke }
//   { type: 'text', x, y, text, font, fill, align, baseline }
//
// Tests assert the right boxes land in the right quadrants and the
// header carries the right turn number.
export function composeFrameLayout({ summary, width = DEFAULT_VIDEO_WIDTH, height = DEFAULT_VIDEO_HEIGHT, gameId } = {}) {
  if (!summary) return []
  const items = []
  // Background.
  items.push({ type: 'rect', x: 0, y: 0, w: width, h: height, fill: '#0c0d0a' })
  // Header band.
  const headerH = 80
  items.push({ type: 'rect', x: 0, y: 0, w: width, h: headerH, fill: '#1a1c17' })
  items.push({
    type: 'text', x: 32, y: headerH / 2, text: `HEXDEK REPLAY · GAME #${gameId ?? '—'}`,
    font: 'bold 26px sans-serif', fill: '#fff', align: 'left', baseline: 'middle',
  })
  items.push({
    type: 'text', x: width - 32, y: headerH / 2,
    text: `TURN ${summary.turn ?? '—'}${summary.eventCount > 0 ? ` · ${summary.eventCount} EVT` : ''}`,
    font: 'bold 22px monospace', fill: '#9aa19a', align: 'right', baseline: 'middle',
  })
  // 2×2 seat grid below the header.
  const seats = summary.seats.slice(0, 4)
  const gridY = headerH + 16
  const gridH = height - gridY - 32
  const cellW = (width - 48) / 2
  const cellH = (gridH - 16) / 2
  const positions = [
    { x: 16,                 y: gridY },
    { x: 16 + cellW + 16,    y: gridY },
    { x: 16,                 y: gridY + cellH + 16 },
    { x: 16 + cellW + 16,    y: gridY + cellH + 16 },
  ]
  for (let i = 0; i < seats.length; i++) {
    const s = seats[i]
    const p = positions[i]
    const stroke = s.lost ? '#c54040' : (i === summary.activeSeat ? '#67c5a4' : '#3a3b35')
    items.push({ type: 'rect', x: p.x, y: p.y, w: cellW, h: cellH, fill: '#15161c', stroke })
    // Seat label + status.
    items.push({
      type: 'text', x: p.x + 14, y: p.y + 24,
      text: `SEAT.${String(i + 1).padStart(2, '0')}${s.lost ? '  ✗ ELIMINATED' : i === summary.activeSeat ? '  ● ACTIVE' : ''}`,
      font: 'bold 14px monospace', fill: '#9aa19a', align: 'left', baseline: 'middle',
    })
    // Commander.
    items.push({
      type: 'text', x: p.x + 14, y: p.y + 52,
      text: s.commander, font: 'bold 22px sans-serif', fill: '#fff',
      align: 'left', baseline: 'middle',
    })
    // Life bar.
    const barX = p.x + 14
    const barY = p.y + 78
    const barW = cellW - 28
    items.push({ type: 'rect', x: barX, y: barY, w: barW, h: 10, fill: '#222421' })
    const lifePct = Math.max(0, Math.min(1, s.life / 40))
    items.push({
      type: 'rect', x: barX, y: barY, w: barW * lifePct, h: 10,
      fill: s.lost ? '#c54040' : '#67c5a4',
    })
    items.push({
      type: 'text', x: barX, y: barY + 28, text: `${s.life} / 40 LIFE`,
      font: 'bold 16px monospace', fill: s.lost ? '#c54040' : '#fff',
      align: 'left', baseline: 'middle',
    })
    // Zone counts.
    const stats = [
      ['HAND',        s.hand],
      ['LIBRARY',     s.library],
      ['GRAVEYARD',   s.graveyard],
      ['BATTLEFIELD', s.battlefield],
    ]
    const statsY = p.y + cellH - 28
    const colW = (cellW - 28) / stats.length
    for (let c = 0; c < stats.length; c++) {
      const [k, v] = stats[c]
      const cx = p.x + 14 + colW * c + colW / 2
      items.push({
        type: 'text', x: cx, y: statsY - 14, text: k,
        font: '11px monospace', fill: '#9aa19a', align: 'center', baseline: 'middle',
      })
      items.push({
        type: 'text', x: cx, y: statsY + 8, text: String(v),
        font: 'bold 18px monospace', fill: '#fff', align: 'center', baseline: 'middle',
      })
    }
  }
  return items
}

// pickMimeType walks a preference list of MIME strings and returns
// the first one the browser's MediaRecorder reports as supported.
// Returns null when none match (caller should disable export).
//
// `globals` parameter accepts a stub in tests; pass `window` (or
// `{ MediaRecorder: window.MediaRecorder }`) at runtime.
export function pickMimeType(globals) {
  const MR = globals?.MediaRecorder
  if (!MR || typeof MR.isTypeSupported !== 'function') return null
  for (const mime of VIDEO_MIME_PREFERENCES) {
    if (MR.isTypeSupported(mime)) return mime
  }
  return null
}

// Preferences ordered by user-friendliness: mp4 plays in iMessage /
// QuickTime / iOS without re-encoding; webm is the universal browser
// fallback and is what Chromium produces by default.
export const VIDEO_MIME_PREFERENCES = [
  'video/mp4;codecs=avc1.42E01E',
  'video/mp4',
  'video/webm;codecs=vp9',
  'video/webm;codecs=vp8',
  'video/webm',
]

// isVideoExportSupported is the higher-level "should we even show
// the button" check: MediaRecorder exists AND at least one MIME
// is supported AND HTMLCanvasElement.captureStream is available.
// Returns false in node tests, true in any reasonable browser
// from the last ~5 years.
export function isVideoExportSupported(globals) {
  if (!globals?.MediaRecorder) return false
  if (!pickMimeType(globals)) return false
  const proto = globals?.HTMLCanvasElement?.prototype
  if (!proto || typeof proto.captureStream !== 'function') return false
  return true
}

// videoFileName builds the download filename. Slashes / dots aren't
// safe in some filesystems, so we sanitize gameId. The extension
// follows the chosen mime so QuickTime / Finder pick the right
// preview generator.
export function videoFileName(gameId, mimeType) {
  const id = String(gameId ?? 'replay').replace(/[^a-z0-9_-]/gi, '-').slice(0, 32)
  const ext = mimeType?.includes('mp4') ? 'mp4' : 'webm'
  return `hexdek-replay-${id}.${ext}`
}

// formatVideoDuration renders ms as M:SS — "0:45" / "2:17". Used in
// the export button's progress text so the user can tell how long
// the recording will take.
export function formatVideoDuration(ms) {
  if (!Number.isFinite(ms) || ms < 0) return '0:00'
  const totalSec = Math.round(ms / 1000)
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

// drawLayout paints a layout list onto a Canvas2D context. Untested
// in node (would need a real Canvas API or jsdom-canvas); kept
// trivial — every branch above happens on data shape, not on canvas
// drawing.
export function drawLayout(ctx, layout) {
  if (!ctx || !Array.isArray(layout)) return
  for (const item of layout) {
    if (item.type === 'rect') {
      if (item.fill) {
        ctx.fillStyle = item.fill
        ctx.fillRect(item.x, item.y, item.w, item.h)
      }
      if (item.stroke) {
        ctx.strokeStyle = item.stroke
        ctx.lineWidth = 2
        ctx.strokeRect(item.x, item.y, item.w, item.h)
      }
    } else if (item.type === 'text') {
      ctx.font = item.font
      ctx.fillStyle = item.fill
      ctx.textAlign = item.align || 'left'
      ctx.textBaseline = item.baseline || 'alphabetic'
      ctx.fillText(item.text, item.x, item.y)
    }
  }
}

// recordReplayVideo orchestrates the recording. Browser-only;
// returns a Promise<Blob> the caller can save with createObjectURL +
// <a download>. The caller passes in:
//
//   { canvas, timeline, commanders, gameId, onProgress, options }
//
// onProgress receives { idx, total, elapsedMs, totalMs }.
//
// Cancellation: if `signal?.aborted` ever becomes true the recorder
// stops, the Blob is still returned (partial), and onProgress fires
// one last time with the final idx. AbortSignal is the standard
// shape — keeps the React caller boilerplate light.
export function recordReplayVideo({
  canvas,
  timeline,
  commanders = [],
  gameId,
  onProgress,
  signal,
  options = {},
} = {}) {
  return new Promise((resolve, reject) => {
    if (!canvas || typeof canvas.getContext !== 'function') {
      reject(new Error('recordReplayVideo: canvas required'))
      return
    }
    const globals = options.globals ?? (typeof window !== 'undefined' ? window : null)
    if (!isVideoExportSupported(globals)) {
      reject(new Error('recordReplayVideo: browser does not support MediaRecorder + canvas capture'))
      return
    }
    const plan = buildFramePlan(timeline, options)
    if (plan.frames.length === 0) {
      reject(new Error('recordReplayVideo: empty timeline'))
      return
    }
    const fps = plan.fps
    const stream = canvas.captureStream(fps)
    const mimeType = pickMimeType(globals)
    const recorder = new globals.MediaRecorder(stream, { mimeType })
    const chunks = []
    recorder.ondataavailable = (e) => { if (e.data && e.data.size > 0) chunks.push(e.data) }
    recorder.onerror = (e) => reject(e.error || new Error('MediaRecorder error'))
    recorder.onstop = () => resolve(new Blob(chunks, { type: mimeType }))
    recorder.start()

    const ctx = canvas.getContext('2d')
    const width = canvas.width
    const height = canvas.height
    const startedAt = (globals?.performance?.now?.() ?? Date.now())
    let i = 0
    const tick = () => {
      if (signal?.aborted) {
        try { recorder.stop() } catch { /* ignore */ }
        return
      }
      const elapsed = (globals?.performance?.now?.() ?? Date.now()) - startedAt
      // Advance i to the frame whose [start, start+duration) covers
      // the elapsed time. Avoids drift on slow systems where raf
      // skips ahead.
      while (i < plan.frames.length - 1 && elapsed >= plan.frames[i + 1].startMs) i++
      const frame = plan.frames[i]
      const summary = frameSummary(frame.snapshot, commanders)
      const layout = composeFrameLayout({ summary, width, height, gameId })
      drawLayout(ctx, layout)
      onProgress?.({ idx: i, total: plan.frames.length, elapsedMs: elapsed, totalMs: plan.totalMs })
      if (elapsed >= plan.totalMs) {
        try { recorder.stop() } catch { /* ignore */ }
        return
      }
      globals.requestAnimationFrame(tick)
    }
    globals.requestAnimationFrame(tick)
  })
}
