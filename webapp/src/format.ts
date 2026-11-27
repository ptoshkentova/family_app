export function money(amount: number): string {
  return amount.toLocaleString('ru-RU').replace(/,/g, ' ') + " so'm"
}

/** Short form for chart labels: 1.2M, 350k. */
export function shortMoney(amount: number): string {
  if (amount >= 1_000_000) return `${+(amount / 1_000_000).toFixed(1)}M`
  if (amount >= 1_000) return `${Math.round(amount / 1_000)}k`
  return String(amount)
}

const MONTHS = ['yan', 'fev', 'mar', 'apr', 'may', 'iyn', 'iyl', 'avg', 'sen', 'okt', 'noy', 'dek']

export function dayLabel(isoDate: string): string {
  const [, m, d] = isoDate.split('-').map(Number)
  return `${d}-${MONTHS[m - 1]}`
}

export function timeLabel(iso: string): string {
  return new Date(iso).toLocaleTimeString('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Tashkent',
  })
}

export function tashkentDate(iso: string): string {
  // en-CA formats as YYYY-MM-DD
  return new Date(iso).toLocaleDateString('en-CA', { timeZone: 'Asia/Tashkent' })
}

/** Splits "🍞 Oziq-ovqat" into icon and name. */
export function splitCategory(category: string): [string, string] {
  const i = category.indexOf(' ')
  return i < 0 ? ['', category] : [category.slice(0, i), category.slice(i + 1)]
}

const PALETTE = ['#4f8cff', '#ff8a3d', '#2fbf71', '#e5484d', '#a46bff', '#f5c518', '#14b8c4', '#ff6fb1',
  '#8d6e63', '#7c8b9c', '#5b6cff', '#3fa34d']

export function categoryColor(category: string, all: string[]): string {
  const i = all.indexOf(category)
  return PALETTE[(i < 0 ? all.length : i) % PALETTE.length]
}
