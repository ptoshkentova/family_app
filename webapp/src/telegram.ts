// Minimal typings for the parts of window.Telegram.WebApp we use.
type HapticStyle = 'light' | 'medium' | 'heavy'

interface TelegramWebApp {
  initData: string
  colorScheme: 'light' | 'dark'
  ready(): void
  expand(): void
  showConfirm(message: string, callback: (ok: boolean) => void): void
  HapticFeedback?: {
    impactOccurred(style: HapticStyle): void
    notificationOccurred(type: 'success' | 'error' | 'warning'): void
  }
}

declare global {
  interface Window {
    Telegram?: { WebApp: TelegramWebApp }
  }
}

export const tg: TelegramWebApp | undefined = window.Telegram?.WebApp

/** Signed launch data; empty when the page is opened outside Telegram. */
export const initData = tg?.initData ?? ''

export function setupTelegram() {
  tg?.ready()
  tg?.expand()
}

export function haptic(type: 'success' | 'error' | 'tap' = 'tap') {
  const h = tg?.HapticFeedback
  if (!h) return
  if (type === 'tap') h.impactOccurred('light')
  else h.notificationOccurred(type)
}

export function confirmDialog(message: string): Promise<boolean> {
  if (tg?.initData) return new Promise((resolve) => tg.showConfirm(message, resolve))
  return Promise.resolve(window.confirm(message))
}
