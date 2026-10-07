import './Parttag.css'

const PART_TAG_LABELS = {
  ITC: 'IT Controller',
  CV: 'Control Valve',
  SM: 'Swing Motor',
  MP: 'Motor Propel',
  PH: 'Pump Assy HYD',
  EN: 'Engine',
  CW: 'Counter Weight',
  MC: 'Machine',
}

export function partTagLabel(code) {
  const key = String(code || '').toUpperCase()
  return PART_TAG_LABELS[key] || ''
}

export default function PartTag({ code, label, className = '', showCode = true }) {
  const key = String(code || '').toUpperCase()
  const text = label || partTagLabel(key)
  if (!text) return null

  const known = Boolean(PART_TAG_LABELS[key])
  const cls = ['part-tag', 'part-tag-' + (known ? key.toLowerCase() : 'mc'), className]
    .filter(Boolean)
    .join(' ')

  return (
    <span className={cls} title={text}>
      {showCode && known && <span className="part-tag-code">{key}</span>}
      <span className="part-tag-name">{text}</span>
    </span>
  )
}
