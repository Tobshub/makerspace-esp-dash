import type { TelemetryValue } from './api/client'

export function formatWhen(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return '—'
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return '—'
  const delta = now - then
  if (delta < 0) return new Date(iso).toLocaleString()
  if (delta < 10_000) return 'just now'
  if (delta < 60_000) return `${Math.round(delta / 1000)} seconds ago`
  const minutes = Math.round(delta / 60_000)
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`
  const hours = Math.round(delta / 3_600_000)
  if (hours < 36) return `${hours} hour${hours === 1 ? '' : 's'} ago`
  return new Date(iso).toLocaleString()
}

export function eventLabel(eventType: string): string {
  switch (eventType) {
    case 'telemetry':
      return 'Telemetry'
    case 'state':
      return 'State'
    case 'connected':
      return 'Online'
    case 'disconnected':
      return 'Offline'
    case 'command_ack':
      return 'Command acknowledged'
    case 'error':
      return 'Error'
    default:
      return eventType
  }
}

export function wifiRssi(state: unknown): number | null {
  if (!state || typeof state !== 'object') return null
  const record = state as Record<string, unknown>
  for (const key of ['wifi_rssi', 'wifiRssi', 'rssi']) {
    const value = record[key]
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return null
}

export function isNumericMetric(value: Pick<TelemetryValue, 'numericValue'>) {
  return value.numericValue != null
}

export const chartRanges = [
  { id: '1h', label: 'Last hour', ms: 60 * 60 * 1000 },
  { id: '24h', label: 'Last 24 hours', ms: 24 * 60 * 60 * 1000 },
  { id: '7d', label: 'Last 7 days', ms: 7 * 24 * 60 * 60 * 1000 },
] as const

export function rangeMs(id: string) {
  return chartRanges.find((range) => range.id === id)?.ms ?? chartRanges[1].ms
}
