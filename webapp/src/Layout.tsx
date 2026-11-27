import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect } from 'react'
import { api, type Period } from './api'
import { BudgetContext } from './budget-context'
import { haptic } from './telegram'

const PERIODS: { value: Period; label: string }[] = [
  { value: 'day', label: 'Bugun' },
  { value: 'week', label: 'Hafta' },
  { value: 'month', label: 'Oy' },
]

export function Layout() {
  const search = useSearch({ from: '__root__' })
  const navigate = useNavigate()
  const me = useQuery({ queryKey: ['me'], queryFn: api.me })
  const categories = useQuery({ queryKey: ['categories'], queryFn: api.categories, staleTime: Infinity })

  const budgets = me.data?.budgets ?? []
  const budget = budgets.find((b) => b.id === search.budget) ?? budgets[0]

  // Keep the selected budget in the URL so reloads and links keep it.
  useEffect(() => {
    if (budget && search.budget !== budget.id) {
      navigate({ to: '.', search: (prev) => ({ ...prev, budget: budget.id }), replace: true })
    }
  }, [budget, search.budget, navigate])

  if (me.isPending) return <div className="center muted">Yuklanmoqda…</div>
  if (me.isError) {
    return (
      <div className="center">
        <p>⚠️ {me.error.message}</p>
        <p className="muted">Sahifani Telegram botidagi “📊 Hisobot” tugmasi orqali oching.</p>
      </div>
    )
  }
  if (!budget) {
    return (
      <div className="center">
        <div className="big-emoji">👋</div>
        <h2>Xush kelibsiz!</h2>
        <p className="muted">Botga birinchi xarajatingizni yozing, masalan: <code>non 5 ming</code></p>
      </div>
    )
  }

  const setPeriod = (period: Period) => {
    haptic()
    navigate({ to: '.', search: (prev) => ({ ...prev, period }) })
  }

  return (
    <BudgetContext.Provider value={{ budget, period: search.period, categories: categories.data ?? [] }}>
      <div className="app">
        <header className="header">
          {budgets.length > 1 ? (
            <select
              className="budget-select"
              value={budget.id}
              onChange={(e) => navigate({ to: '.', search: (prev) => ({ ...prev, budget: Number(e.target.value) }) })}
            >
              {budgets.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.isGroup ? '👨‍👩‍👧' : '👤'} {b.title}
                </option>
              ))}
            </select>
          ) : (
            <h1 className="budget-title">
              {budget.isGroup ? '👨‍👩‍👧' : '👤'} {budget.title}
            </h1>
          )}
          <div className="segmented">
            {PERIODS.map((p) => (
              <button
                key={p.value}
                className={p.value === search.period ? 'active' : ''}
                onClick={() => setPeriod(p.value)}
              >
                {p.label}
              </button>
            ))}
          </div>
        </header>

        <main className="content">
          <Outlet />
        </main>

        <nav className="tabbar">
          <Link to="/" search={(prev) => ({ ...prev, period: prev.period ?? 'month' })} activeOptions={{ exact: true, includeSearch: false }} onClick={() => haptic()}>
            <span>📊</span>Hisobot
          </Link>
          <Link to="/transactions" search={(prev) => ({ ...prev, period: prev.period ?? 'month' })} activeOptions={{ includeSearch: false }} onClick={() => haptic()}>
            <span>🧾</span>Yozuvlar
          </Link>
        </nav>
      </div>
    </BudgetContext.Provider>
  )
}
