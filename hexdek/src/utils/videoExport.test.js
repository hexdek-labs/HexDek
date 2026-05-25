// Run with: node --test hexdek/src/utils/videoExport.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  DEFAULT_SECONDS_PER_TURN,
  DEFAULT_FPS,
  VIDEO_MIME_PREFERENCES,
  buildFramePlan,
  frameSummary,
  composeFrameLayout,
  pickMimeType,
  isVideoExportSupported,
  videoFileName,
  formatVideoDuration,
  drawLayout,
} from './videoExport.js'

const SNAPSHOT = {
  turn: 5,
  active_seat: 1,
  events: [{ kind: 'cast' }, { kind: 'combat' }],
  seats: [
    { life: 30, hand_size: 5, library_size: 80, gy_size: 4, battlefield: [{}, {}, {}] },
    { life: 22, hand_size: 3, library_size: 78, gy_size: 7, battlefield: [{}, {}] },
    { life: 0,  hand_size: 0, library_size: 0,  gy_size: 12, battlefield: [], lost: true },
    { life: 40, hand_size: 7, library_size: 90, gy_size: 1, battlefield: [{}] },
  ],
}
const COMMANDERS = [
  'Atraxa, Praetors\' Voice // Phyrexia',
  'Krenko, Mob Boss',
  'Sheoldred, the Apocalypse',
  'Niv-Mizzet, Parun',
]

// -----------------------------------------------------------------------------
// buildFramePlan
// -----------------------------------------------------------------------------

test('buildFramePlan defaults to 2.5s/turn at 30fps', () => {
  const plan = buildFramePlan([SNAPSHOT, SNAPSHOT, SNAPSHOT])
  assert.equal(plan.fps, DEFAULT_FPS)
  assert.equal(plan.totalMs, Math.round(DEFAULT_SECONDS_PER_TURN * 1000) * 3)
  assert.equal(plan.frames.length, 3)
})

test('buildFramePlan stamps consecutive startMs values', () => {
  const plan = buildFramePlan([SNAPSHOT, SNAPSHOT, SNAPSHOT], { secondsPerTurn: 2, fps: 30 })
  assert.equal(plan.frames[0].startMs, 0)
  assert.equal(plan.frames[1].startMs, 2000)
  assert.equal(plan.frames[2].startMs, 4000)
  // each holds for the full per-turn duration
  for (const f of plan.frames) assert.equal(f.durationMs, 2000)
  assert.equal(plan.totalMs, 6000)
})

test('buildFramePlan honors custom secondsPerTurn / fps', () => {
  const plan = buildFramePlan([SNAPSHOT, SNAPSHOT], { secondsPerTurn: 1.5, fps: 60 })
  assert.equal(plan.fps, 60)
  assert.equal(plan.totalMs, 3000)
})

test('buildFramePlan handles empty timeline', () => {
  const plan = buildFramePlan([])
  assert.equal(plan.totalMs, 0)
  assert.equal(plan.frames.length, 0)
})

test('buildFramePlan handles bad input', () => {
  assert.equal(buildFramePlan(null).frames.length, 0)
  assert.equal(buildFramePlan(undefined).frames.length, 0)
})

test('buildFramePlan carries the snapshot through', () => {
  const plan = buildFramePlan([SNAPSHOT])
  assert.strictEqual(plan.frames[0].snapshot, SNAPSHOT)
})

// -----------------------------------------------------------------------------
// frameSummary
// -----------------------------------------------------------------------------

test('frameSummary extracts per-seat fields', () => {
  const s = frameSummary(SNAPSHOT, COMMANDERS)
  assert.equal(s.turn, 5)
  assert.equal(s.activeSeat, 1)
  assert.equal(s.eventCount, 2)
  assert.equal(s.seats.length, 4)
})

test('frameSummary shortens commander names (drop side B + comma suffix)', () => {
  const s = frameSummary(SNAPSHOT, COMMANDERS)
  assert.equal(s.seats[0].commander, 'ATRAXA',     'strips MDFC partner + appositive')
  assert.equal(s.seats[1].commander, 'KRENKO',     'strips the comma-clause appositive')
  assert.equal(s.seats[2].commander, 'SHEOLDRED')
})

test('frameSummary maps the lost flag and battlefield count', () => {
  const s = frameSummary(SNAPSHOT, COMMANDERS)
  assert.equal(s.seats[2].lost, true)
  assert.equal(s.seats[2].battlefield, 0)
  assert.equal(s.seats[0].battlefield, 3)
})

test('frameSummary fills missing fields with 0', () => {
  const s = frameSummary({ turn: 1, seats: [{}] }, [])
  assert.equal(s.seats[0].life, 0)
  assert.equal(s.seats[0].hand, 0)
  assert.equal(s.seats[0].commander, 'UNKNOWN')
})

test('frameSummary handles null snapshot', () => {
  const s = frameSummary(null, COMMANDERS)
  assert.equal(s.turn, null)
  assert.equal(s.seats.length, 0)
})

// -----------------------------------------------------------------------------
// composeFrameLayout
// -----------------------------------------------------------------------------

test('composeFrameLayout draws a background + header band', () => {
  const summary = frameSummary(SNAPSHOT, COMMANDERS)
  const layout = composeFrameLayout({ summary, width: 1280, height: 720, gameId: 42 })
  const bgs = layout.filter(it => it.type === 'rect' && it.x === 0 && it.y === 0 && it.w === 1280)
  assert.ok(bgs.length >= 2, 'background rect + header band rect')
  const header = layout.find(it => it.type === 'text' && /HEXDEK REPLAY/.test(it.text))
  assert.ok(header, 'header text present')
  assert.match(header.text, /GAME #42/)
})

test('composeFrameLayout includes the current turn + event count', () => {
  const summary = frameSummary(SNAPSHOT, COMMANDERS)
  const layout = composeFrameLayout({ summary, gameId: 42 })
  const turnHeader = layout.find(it => it.type === 'text' && /^TURN /.test(it.text))
  assert.ok(turnHeader)
  assert.match(turnHeader.text, /TURN 5/)
  assert.match(turnHeader.text, /2 EVT/)
})

test('composeFrameLayout places one cell per seat (max 4)', () => {
  const summary = frameSummary(SNAPSHOT, COMMANDERS)
  const layout = composeFrameLayout({ summary, gameId: 42 })
  const commanderTexts = layout.filter(it => it.type === 'text' && (
    it.text === 'ATRAXA' || it.text === 'KRENKO' || it.text === 'SHEOLDRED' || it.text === 'NIV-MIZZET'
  ))
  assert.equal(commanderTexts.length, 4)
})

test('composeFrameLayout marks eliminated seats with the danger stroke', () => {
  const summary = frameSummary(SNAPSHOT, COMMANDERS)
  const layout = composeFrameLayout({ summary })
  // The eliminated seat (idx 2) cell rect carries the danger stroke.
  const dangerStrokes = layout.filter(it => it.type === 'rect' && it.stroke === '#c54040')
  assert.ok(dangerStrokes.length >= 1, 'eliminated seat carries danger stroke')
})

test('composeFrameLayout marks the active seat with the ok stroke', () => {
  const summary = frameSummary(SNAPSHOT, COMMANDERS)
  const layout = composeFrameLayout({ summary })
  const okStrokes = layout.filter(it => it.type === 'rect' && it.stroke === '#67c5a4')
  assert.equal(okStrokes.length, 1, 'exactly one active seat (the one whose idx === activeSeat)')
})

test('composeFrameLayout handles 1-seat snapshots without crashing', () => {
  const summary = frameSummary({ turn: 3, active_seat: 0, seats: [{ life: 40, battlefield: [] }] }, ['Solo'])
  const layout = composeFrameLayout({ summary })
  assert.ok(layout.length > 0)
})

test('composeFrameLayout returns empty for missing summary', () => {
  assert.deepEqual(composeFrameLayout({ summary: null }), [])
  assert.deepEqual(composeFrameLayout({}), [])
})

// -----------------------------------------------------------------------------
// pickMimeType + isVideoExportSupported
// -----------------------------------------------------------------------------

function stubGlobals({ supported = [], hasCaptureStream = true } = {}) {
  const MR = function () {}
  MR.isTypeSupported = (mime) => supported.includes(mime)
  const proto = hasCaptureStream ? { captureStream() {} } : {}
  return {
    MediaRecorder: MR,
    HTMLCanvasElement: { prototype: proto },
  }
}

test('pickMimeType prefers mp4 over webm when both are supported', () => {
  const globals = stubGlobals({ supported: ['video/mp4', 'video/webm'] })
  assert.equal(pickMimeType(globals), 'video/mp4')
})

test('pickMimeType falls back to webm when no mp4 variant is supported', () => {
  const globals = stubGlobals({ supported: ['video/webm;codecs=vp8'] })
  assert.equal(pickMimeType(globals), 'video/webm;codecs=vp8')
})

test('pickMimeType returns null when nothing matches', () => {
  const globals = stubGlobals({ supported: [] })
  assert.equal(pickMimeType(globals), null)
})

test('pickMimeType returns null when MediaRecorder is missing', () => {
  assert.equal(pickMimeType({}), null)
  assert.equal(pickMimeType(null), null)
})

test('isVideoExportSupported needs MediaRecorder + captureStream + a supported mime', () => {
  assert.equal(isVideoExportSupported(stubGlobals({ supported: ['video/mp4'] })), true)
  assert.equal(isVideoExportSupported(stubGlobals({ supported: [] })), false)
  assert.equal(isVideoExportSupported(stubGlobals({ supported: ['video/mp4'], hasCaptureStream: false })), false)
  assert.equal(isVideoExportSupported(null), false)
})

test('VIDEO_MIME_PREFERENCES is ordered mp4-first', () => {
  assert.match(VIDEO_MIME_PREFERENCES[0], /mp4/)
})

// -----------------------------------------------------------------------------
// videoFileName + formatVideoDuration + drawLayout sanity
// -----------------------------------------------------------------------------

test('videoFileName picks extension from mime', () => {
  assert.equal(videoFileName(42, 'video/mp4'), 'hexdek-replay-42.mp4')
  assert.equal(videoFileName(42, 'video/webm;codecs=vp9'), 'hexdek-replay-42.webm')
})

test('videoFileName sanitizes weird game ids', () => {
  assert.equal(videoFileName('a/b.c', 'video/mp4'), 'hexdek-replay-a-b-c.mp4')
  assert.equal(videoFileName(null, 'video/webm'), 'hexdek-replay-replay.webm')
})

test('formatVideoDuration renders M:SS', () => {
  assert.equal(formatVideoDuration(0),       '0:00')
  assert.equal(formatVideoDuration(45_000),  '0:45')
  assert.equal(formatVideoDuration(137_500), '2:18') // rounds
})

test('formatVideoDuration is defensive on bad input', () => {
  assert.equal(formatVideoDuration(NaN), '0:00')
  assert.equal(formatVideoDuration(-5),  '0:00')
})

test('drawLayout dispatches rect / text via the Canvas2D surface', () => {
  // Stub the Canvas2D context — just record method calls.
  const calls = []
  const ctx = {
    fillRect:   (...a) => calls.push(['fillRect',   ...a]),
    strokeRect: (...a) => calls.push(['strokeRect', ...a]),
    fillText:   (...a) => calls.push(['fillText',   ...a]),
    set fillStyle(v)   { calls.push(['fillStyle',   v]) },
    set strokeStyle(v) { calls.push(['strokeStyle', v]) },
    set lineWidth(v)   { calls.push(['lineWidth',   v]) },
    set font(v)        { calls.push(['font',        v]) },
    set textAlign(v)   { calls.push(['textAlign',   v]) },
    set textBaseline(v){ calls.push(['textBaseline',v]) },
  }
  drawLayout(ctx, [
    { type: 'rect', x: 0, y: 0, w: 10, h: 10, fill: '#000', stroke: '#fff' },
    { type: 'text', x: 5, y: 5, text: 'hi', font: '12px sans', fill: '#fff', align: 'left', baseline: 'middle' },
  ])
  const types = calls.map(c => c[0])
  assert.ok(types.includes('fillRect'),   'rect fill dispatched')
  assert.ok(types.includes('strokeRect'), 'rect stroke dispatched')
  assert.ok(types.includes('fillText'),   'text dispatched')
})

test('drawLayout is a no-op on null ctx / non-array layout', () => {
  // Just checking it doesn't throw.
  drawLayout(null, [])
  drawLayout({ fillRect() {} }, null)
})
