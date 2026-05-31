// StackPanel — compact mini-display of the engine's resolution
// stack. Consumes GameSnapshot.Stack (added in the R60 spectator UX
// batch — see internal/hexapi/showmatch.go buildStackSnapshot).
//
// Sits above the seats area when the stack is non-empty; renders
// nothing when the stack is empty so the layout doesn't reserve
// space during the no-spell-pending steady state.
//
// Spectator UX rationale: pre-this, spectators had no view of
// what's pending resolution — a Counterspell vs Lightning Bolt
// exchange was only inferrable from the event log. The mini-panel
// surfaces the top items directly: "TOP: Counterspell ← Lightning
// Bolt" so the resolution order is obvious at a glance.
//
// Pure helpers (formatStackItemLabel, stackItemKindGlyph,
// stackOrderTopFirst) live in stackPanelHelpers.js so node --test
// can exercise them directly.

import {
  formatStackItemLabel,
  stackItemKindGlyph,
  stackOrderTopFirst,
} from './stackPanelHelpers.js'

export { formatStackItemLabel, stackItemKindGlyph, stackOrderTopFirst }

// Max items rendered before the "+N more" overflow. The engine can
// occasionally produce deep stacks (storm chains, copy effects),
// but realistically more than 4 top-of-stack items is hard to read
// in the spectator's peripheral vision.
const MAX_VISIBLE = 4

export default function StackPanel({ stack, seats }) {
  if (!Array.isArray(stack) || stack.length === 0) return null
  const ordered = stackOrderTopFirst(stack)
  const visible = ordered.slice(0, MAX_VISIBLE)
  const hidden = ordered.length - visible.length

  return (
    <div
      className="stack-panel"
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
        padding: '6px 10px',
        background: 'var(--bg-2, transparent)',
        borderTop: '1px solid var(--rule-2)',
        borderBottom: '1px solid var(--rule-2)',
        fontSize: 10,
        letterSpacing: '0.06em',
      }}
      aria-label={`Stack of ${stack.length} ${stack.length === 1 ? 'item' : 'items'}`}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          color: 'var(--ink-2)',
          fontSize: 9,
          fontWeight: 600,
          letterSpacing: '0.1em',
        }}
      >
        <span>STACK / {stack.length} {stack.length === 1 ? 'ITEM' : 'ITEMS'}</span>
        {hidden > 0 && (
          <span>+{hidden} BELOW</span>
        )}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {visible.map((item, idx) => {
          const isTop = idx === 0
          const glyph = stackItemKindGlyph(item.kind)
          const label = formatStackItemLabel(item)
          const cmdr = seats?.[item.controller]?.commander
          return (
            <div
              key={idx}
              style={{
                display: 'flex',
                gap: 8,
                alignItems: 'center',
                padding: '3px 6px',
                background: isTop ? 'var(--ok)' : 'var(--rule-2)',
                color: isTop ? 'var(--bg-2, #000)' : 'var(--ink)',
                opacity: item.countered ? 0.5 : 1,
                fontWeight: isTop ? 700 : 500,
              }}
            >
              <span style={{ fontSize: 9, opacity: 0.7, minWidth: 28 }}>
                {isTop ? 'TOP' : `#${idx + 1}`}
              </span>
              {glyph && (
                <span
                  style={{
                    fontSize: 8,
                    padding: '1px 4px',
                    background: 'rgba(0, 0, 0, 0.15)',
                    borderRadius: 2,
                  }}
                >
                  {glyph}
                </span>
              )}
              <span style={{ flex: 1 }}>{label.toUpperCase()}</span>
              {cmdr && (
                <span style={{ fontSize: 9, opacity: 0.8 }}>
                  &lsaquo; {cmdr.toUpperCase()}
                </span>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
