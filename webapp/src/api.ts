import { initData } from './telegram'

export type Period = 'day' | 'week' | 'month'

export interface Budget {
  id: number
  title: string
  isGroup: boolean
}

export interface Me {
  user: { id: number; first_name: string }
  budgets: Budget[]
}

export interface Summary {
  title: string
  totalExpense: number
  totalIncome: number
  balance: number
  count: number
  byCategory: { category: string; amount: number; percent: number }[]
  byUser: { name: string; amount: number }[]
  daily: { date: string; expense: number }[]
}

export interface Transaction {
  id: number
  budgetId: number
  userId: number
  userName: string
  amount: number
  category: string
  description: string
  isIncome: boolean
  createdAt: string
}

export type TransactionPatch = Partial<Pick<Transaction, 'amount' | 'category' | 'description'>>

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (initData) headers.set('Authorization', `tma ${initData}`)
  if (init.body) headers.set('Content-Type', 'application/json')
  const res = await fetch(path, { ...init, headers })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }
  return res.status === 204 ? (undefined as T) : res.json()
}

export const api = {
  me: () => request<Me>('/api/me'),
  categories: () => request<string[]>('/api/categories'),
  summary: (budget: number, period: Period) =>
    request<Summary>(`/api/budgets/${budget}/summary?period=${period}`),
  transactions: (budget: number, period: Period) =>
    request<Transaction[]>(`/api/budgets/${budget}/transactions?period=${period}`),
  updateTx: (id: number, patch: TransactionPatch) =>
    request<Transaction>(`/api/transactions/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  deleteTx: (id: number) => request<void>(`/api/transactions/${id}`, { method: 'DELETE' }),
}
