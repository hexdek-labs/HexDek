// Renders a translated sentence whose {token} placeholders are React nodes.
// Interpolating with JSX concatenation would pin the node's position to
// English word order; splitting the translated string lets the token sit
// wherever the target language puts it.
export function fill(str, nodes) {
  return String(str).split(/(\{\w+\})/g).map((part, i) => {
    const m = /^\{(\w+)\}$/.exec(part)
    return m && nodes[m[1]] !== undefined ? <span key={i}>{nodes[m[1]]}</span> : part
  })
}
