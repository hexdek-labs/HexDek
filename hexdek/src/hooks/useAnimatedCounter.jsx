import { useRef, useState, useEffect, memo } from 'react'

export const AnimatedCounter = memo(function AnimatedCounter({ target, rate, className, style }) {
  const [display, setDisplay] = useState(0)
  const s = useRef({ base: 0, baseTime: 0, rate: 0 })

  useEffect(() => {
    if (!target || !rate) return
    const now = Date.now()
    const c = s.current
    if (c.base === 0) {
      c.base = target
    } else {
      c.base += c.rate * ((now - c.baseTime) / 1000)
    }
    c.baseTime = now
    c.rate = rate / 60
    setDisplay(Math.round(c.base))
  }, [target, rate])

  useEffect(() => {
    const id = setInterval(() => {
      const c = s.current
      if (c.rate <= 0) return
      const elapsed = (Date.now() - c.baseTime) / 1000
      setDisplay(Math.round(c.base + c.rate * elapsed))
    }, 50)
    return () => clearInterval(id)
  }, [])

  return <span className={className} style={style}>{display ? display.toLocaleString() : '—'}</span>
})
