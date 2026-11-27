import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { DailyBars, Donut } from '../components/Charts'
import { categoryColor, money, shortMoney, splitCategory } from '../format'
import { useBudget } from '../budget-context'

export function Dashboard() {
  const { budget, period, categories } = useBudget()
  const { data, isPending, isError, error } = useQuery({
    queryKey: ['summary', budget.id, period],
    queryFn: () => api.summary(budget.id, period),
  })

  if (isPending) return <div className="center muted">Yuklanmoqda…</div>
  if (isError) return <div className="center">⚠️ {error.message}</div>

  if (data.count === 0) {
    return (
      <div className="center">
        <div className="big-emoji">🪙</div>
        <p className="muted">{data.title}: hali hech narsa yozilmagan.</p>
      </div>
    )
  }

  const slices = data.byCategory.map((c) => ({
    label: c.category,
    value: c.amount,
    color: categoryColor(c.category, categories),
  }))

  return (
    <>
      <section className="cards">
        <div className="card stat">
          <div className="stat-label">💸 Xarajat</div>
          <div className="stat-value">{money(data.totalExpense)}</div>
        </div>
        <div className="card stat">
          <div className="stat-label">💰 Daromad</div>
          <div className="stat-value">{money(data.totalIncome)}</div>
        </div>
        {data.totalIncome > 0 && (
          <div className={`card stat wide ${data.balance < 0 ? 'negative' : 'positive'}`}>
            <div className="stat-label">Qoldiq</div>
            <div className="stat-value">
              {data.balance < 0 ? '−' : '+'}
              {money(Math.abs(data.balance))}
            </div>
          </div>
        )}
      </section>

      {slices.length > 0 && (
        <section className="card">
          <h3>Kategoriyalar</h3>
          <Donut slices={slices} center={shortMoney(data.totalExpense)} />
          <ul className="legend">
            {data.byCategory.map((c) => {
              const [icon, name] = splitCategory(c.category)
              return (
                <li key={c.category}>
                  <span className="dot" style={{ background: categoryColor(c.category, categories) }} />
                  <span className="legend-name">
                    {icon} {name}
                  </span>
                  <span className="legend-pct">{c.percent}%</span>
                  <span className="legend-amount">{money(c.amount)}</span>
                </li>
              )
            })}
          </ul>
        </section>
      )}

      {period !== 'day' && data.daily.length > 1 && (
        <section className="card">
          <h3>Kunlik xarajat</h3>
          <DailyBars days={data.daily} />
        </section>
      )}

      {data.byUser.length > 1 && (
        <section className="card">
          <h3>Kim qancha sarfladi</h3>
          <ul className="people">
            {data.byUser.map((u) => (
              <li key={u.name}>
                <span>👤 {u.name}</span>
                <div className="progress">
                  <div style={{ width: `${(u.amount / data.totalExpense) * 100}%` }} />
                </div>
                <span className="legend-amount">{money(u.amount)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  )
}
