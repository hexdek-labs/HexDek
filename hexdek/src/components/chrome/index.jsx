import { Fragment } from 'react'

export const Crops = () => (
  <>
    <span className="crop crop--tl" />
    <span className="crop crop--tr" />
    <span className="crop crop--bl" />
    <span className="crop crop--br" />
  </>
)

export const Panel = ({ title, code, right, children, solid, style, className = '' }) => (
  <div className={`panel ${solid ? 'panel--solid' : ''} ${className}`} style={style}>
    {(title || right) && (
      <div className="panel-hd">
        <span>{code && <span className="muted-2" style={{ marginRight: 8 }}>{code}</span>}{title}</span>
        <span>{right}</span>
      </div>
    )}
    <div className="panel-bd">{children}</div>
  </div>
)

export const KV = ({ rows }) => (
  <div className="kv">
    {rows.map((r, i) => (
      <Fragment key={i}>
        <span className="k">{r[0]}</span>
        <span className="dots">{'.'.repeat(60)}</span>
        <span className="v">{r[1]}</span>
      </Fragment>
    ))}
  </div>
)

export const Bar = ({ value, max = 100, lg }) => (
  <div className={`bar ${lg ? 'bar--lg' : ''}`}>
    <i style={{ width: `${(value / max) * 100}%`, transition: 'width 0.3s ease' }} />
  </div>
)

export const Tag = ({ children, kind, solid, onClick, style }) => (
  <span className={`tag ${kind ? `tag--${kind}` : ''} ${solid ? 'tag--solid' : ''}`} onClick={onClick} style={style}>
    {children}
  </span>
)

export const Btn = ({ children, solid, sm, ghost, arrow = '↗', onClick }) => (
  <button
    className={`btn ${solid ? 'btn--solid' : ''} ${sm ? 'btn--sm' : ''} ${ghost ? 'btn--ghost' : ''}`}
    onClick={onClick}
  >
    <span>{children}</span>
    {arrow && <span className="arr">{arrow}</span>}
  </button>
)

export const Tape = ({ left, mid, right }) => (
  <div
    className="flex items-center justify-between"
    style={{
      borderTop: '1px solid var(--rule-2)',
      borderBottom: '1px solid var(--rule-2)',
      padding: '4px 10px',
      fontSize: 9,
      letterSpacing: '0.1em',
      textTransform: 'uppercase',
      color: 'var(--ink-2)',
    }}
  >
    <span>{left}</span>
    {mid && <span className="muted-2">{mid}</span>}
    <span>{right}</span>
  </div>
)

export const Stripes = ({ height = 18, w = '100%' }) => (
  <div className="stripes" style={{ height, width: w }} />
)

export const MiniBars = ({ data }) => (
  <div className="minibars">
    {data.map((v, i) => <i key={i} style={{ height: `${v}%` }} />)}
  </div>
)
