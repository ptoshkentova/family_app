import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type Transaction, type TransactionPatch } from '../api'
import { categoryColor, dayLabel, money, splitCategory, tashkentDate, timeLabel } from '../format'
import { useBudget } from '../Layout'
import { confirmDialog, haptic } from '../telegram'

export function Transactions() {
  const { budget, period, categories } = useBudget()
  const [editing, setEditing] = useState<Transaction | null>(null)
  const { data, isPending, isError, error } = useQuery({
    queryKey: ['transactions', budget.id, period],
    queryFn: () => api.transactions(budget.id, period),
  })

  if (isPending) return <div className="center muted">Yuklanmoqda…</div>
  if (isError) return <div className="center">⚠️ {error.message}</div>
  if (data.length === 0) return <div className="center muted">Bu davrda yozuvlar yo'q.</div>

  // Group by Tashkent calendar day, newest first (API already sorts).
  const groups = new Map<string, Transaction[]>()
  for (const t of data) {
    const day = tashkentDate(t.createdAt)
    groups.set(day, [...(groups.get(day) ?? []), t])
  }

  return (
    <>
      {[...groups].map(([day, txs]) => {
        const spent = txs.filter((t) => !t.isIncome).reduce((s, t) => s + t.amount, 0)
        return (
          <section key={day} className="card list">
            <div className="list-head">
              <span>{dayLabel(day)}</span>
              <span className="muted">−{money(spent)}</span>
            </div>
            {txs.map((t) => {
              const [icon, name] = splitCategory(t.category)
              return (
                <button key={t.id} className="row" onClick={() => (haptic(), setEditing(t))}>
                  <span className="row-icon" style={{ background: categoryColor(t.category, categories) + '22' }}>
                    {icon}
                  </span>
                  <span className="row-main">
                    <span className="row-title">{t.description}</span>
                    <span className="row-sub">
                      {name} · {t.userName} · {timeLabel(t.createdAt)}
                    </span>
                  </span>
                  <span className={`row-amount ${t.isIncome ? 'income' : ''}`}>
                    {t.isIncome ? '+' : '−'}
                    {money(t.amount)}
                  </span>
                </button>
              )
            })}
          </section>
        )
      })}
      {editing && <EditSheet tx={editing} categories={categories} onClose={() => setEditing(null)} />}
    </>
  )
}

function EditSheet({ tx, categories, onClose }: { tx: Transaction; categories: string[]; onClose: () => void }) {
  const qc = useQueryClient()
  const [amount, setAmount] = useState(String(tx.amount))
  const [description, setDescription] = useState(tx.description)
  const [category, setCategory] = useState(tx.category)

  const done = () => {
    qc.invalidateQueries({ queryKey: ['transactions'] })
    qc.invalidateQueries({ queryKey: ['summary'] })
    onClose()
  }
  const save = useMutation({
    mutationFn: (patch: TransactionPatch) => api.updateTx(tx.id, patch),
    onSuccess: () => (haptic('success'), done()),
    onError: () => haptic('error'),
  })
  const remove = useMutation({
    mutationFn: () => api.deleteTx(tx.id),
    onSuccess: () => (haptic('success'), done()),
    onError: () => haptic('error'),
  })

  const parsedAmount = Number(amount.replace(/\s/g, ''))
  const valid = Number.isInteger(parsedAmount) && parsedAmount > 0 && description.trim().length > 0

  const onSave = () => {
    const patch: TransactionPatch = {}
    if (parsedAmount !== tx.amount) patch.amount = parsedAmount
    if (description.trim() !== tx.description) patch.description = description.trim()
    if (category !== tx.category) patch.category = category
    if (Object.keys(patch).length === 0) return onClose()
    save.mutate(patch)
  }

  const onDelete = async () => {
    if (await confirmDialog(`"${tx.description}" yozuvini o'chirasizmi?`)) remove.mutate()
  }

  const error = save.error ?? remove.error
  const busy = save.isPending || remove.isPending

  return (
    <div className="sheet-backdrop" onClick={onClose}>
      <div className="sheet" onClick={(e) => e.stopPropagation()}>
        <div className="sheet-handle" />
        <h3>Yozuvni tahrirlash</h3>
        <label>
          Summa (so'm)
          <input inputMode="numeric" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </label>
        <label>
          Izoh
          <input value={description} maxLength={200} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <label>
          Kategoriya
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            {(categories.includes(category) ? categories : [category, ...categories]).map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </label>
        {error && <p className="error">⚠️ {error.message}</p>}
        <div className="sheet-actions">
          <button className="btn danger" disabled={busy} onClick={onDelete}>
            🗑 O'chirish
          </button>
          <button className="btn primary" disabled={busy || !valid} onClick={onSave}>
            {save.isPending ? 'Saqlanmoqda…' : 'Saqlash'}
          </button>
        </div>
      </div>
    </div>
  )
}
