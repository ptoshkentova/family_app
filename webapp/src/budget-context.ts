import { createContext, useContext } from 'react'
import type { Budget, Period } from './api'

export interface BudgetCtx {
  budget: Budget
  period: Period
  categories: string[]
}

export const BudgetContext = createContext<BudgetCtx | null>(null)

export function useBudget(): BudgetCtx {
  const v = useContext(BudgetContext)
  if (!v) throw new Error('useBudget outside Layout')
  return v
}
