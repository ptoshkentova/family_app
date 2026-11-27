import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import type { Period } from './api'
import { Layout } from './Layout'
import { Dashboard } from './pages/Dashboard'
import { Transactions } from './pages/Transactions'

export interface AppSearch {
  budget?: number
  period: Period
}

const rootRoute = createRootRoute({
  component: Layout,
  validateSearch: (search: Record<string, unknown>): AppSearch => {
    const period = ['day', 'week', 'month'].includes(search.period as string)
      ? (search.period as Period)
      : 'month'
    const budget = Number(search.budget)
    return { period, budget: Number.isFinite(budget) && budget !== 0 ? budget : undefined }
  },
})

const dashboardRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: Dashboard })
const transactionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/transactions',
  component: Transactions,
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([dashboardRoute, transactionsRoute]),
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
