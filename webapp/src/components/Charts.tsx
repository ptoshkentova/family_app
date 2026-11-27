import { dayLabel, shortMoney } from '../format'

interface Slice {
  label: string
  value: number
  color: string
}

/** Donut chart drawn with stroke-dasharray on SVG circles. */
export function Donut({ slices, center }: { slices: Slice[]; center: string }) {
  const total = slices.reduce((s, x) => s + x.value, 0)
  const r = 40
  const c = 2 * Math.PI * r
  let offset = 0
  return (
    <svg viewBox="0 0 100 100" className="donut" role="img" aria-label="Xarajatlar taqsimoti">
      <circle cx="50" cy="50" r={r} fill="none" stroke="var(--track)" strokeWidth="14" />
      {total > 0 &&
        slices.map((s) => {
          const len = (s.value / total) * c
          const el = (
            <circle
              key={s.label}
              cx="50"
              cy="50"
              r={r}
              fill="none"
              stroke={s.color}
              strokeWidth="14"
              strokeDasharray={`${Math.max(len - 0.6, 0)} ${c}`}
              strokeDashoffset={-offset}
              transform="rotate(-90 50 50)"
            >
              <title>{s.label}</title>
            </circle>
          )
          offset += len
          return el
        })}
      <text x="50" y="47" textAnchor="middle" className="donut-label">
        Jami
      </text>
      <text x="50" y="60" textAnchor="middle" className="donut-value">
        {center}
      </text>
    </svg>
  )
}

/** Daily expense bars. Shows a label under every bar for short ranges, every few bars otherwise. */
export function DailyBars({ days }: { days: { date: string; expense: number }[] }) {
  const max = Math.max(...days.map((d) => d.expense), 1)
  const labelEvery = days.length > 10 ? Math.ceil(days.length / 6) : 1
  return (
    <div className="bars">
      {days.map((d, i) => (
        <div key={d.date} className="bar-col" title={`${dayLabel(d.date)}: ${shortMoney(d.expense)}`}>
          <div className="bar-track">
            <div className="bar" style={{ height: `${(d.expense / max) * 100}%` }} />
          </div>
          <div className="bar-label">{i % labelEvery === 0 ? dayLabel(d.date).split('-')[0] : ''}</div>
        </div>
      ))}
    </div>
  )
}
