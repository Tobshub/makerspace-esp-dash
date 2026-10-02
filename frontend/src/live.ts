import { useEffect } from 'react'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  getToken,
  type Device,
  type DeviceTelemetryLatest,
  type Overview,
  type ProjectTelemetryLatest,
  type TelemetryValue,
} from './api/client'

export type LiveEvent = {
  type: string
  deviceId: string
  timestamp: string
  data?: unknown
}

const fallbackMs = 30_000

export function liveInterval() {
  return fallbackMs
}

export function useProjectStream(projectId: string) {
  const queryClient = useQueryClient()
  useEffect(() => {
    if (!projectId) return
    let source: EventSource | null = null
    let timer = 0
    let stopped = false
    let opened = false

    const connect = () => {
      const token = getToken()
      if (!token || stopped) return
      source = new EventSource(`/api/v1/projects/${projectId}/stream?access_token=${encodeURIComponent(token)}`)
      source.onopen = () => {
        if (opened) refreshProject(queryClient, projectId)
        opened = true
      }
      source.onmessage = (message) => {
        try {
          applyLiveEvent(queryClient, projectId, JSON.parse(message.data) as LiveEvent)
        } catch {
          // A malformed event should not close the stream.
        }
      }
      source.onerror = () => {
        source?.close()
        source = null
        if (!stopped) timer = window.setTimeout(connect, 3000)
      }
    }

    connect()
    return () => {
      stopped = true
      window.clearTimeout(timer)
      source?.close()
    }
  }, [projectId, queryClient])
}

export function applyLiveEvent(queryClient: QueryClient, projectId: string, event: LiveEvent) {
  if (!event.deviceId || !event.type) return
  if (event.type === 'telemetry.received' && isRecord(event.data)) {
    const latest = queryClient.setQueryData<DeviceTelemetryLatest>(['telemetry-latest', event.deviceId], (old) => {
      if (!old) return old
      return { ...old, metrics: mergeMetrics(old.metrics, event.data as Record<string, unknown>, event.timestamp) }
    })
    if (!latest) void queryClient.invalidateQueries({ queryKey: ['telemetry-latest', event.deviceId] })

    let found = false
    const project = queryClient.setQueryData<ProjectTelemetryLatest>(['project-telemetry', projectId], (old) => {
      if (!old) return old
      found = old.devices.some((device) => device.deviceId === event.deviceId)
      if (!found) return old
      return {
        ...old,
        devices: old.devices.map((device) =>
          device.deviceId === event.deviceId
            ? { ...device, metrics: mergeMetrics(device.metrics, event.data as Record<string, unknown>, event.timestamp) }
            : device,
        ),
      }
    })
    if (!project || !found) void queryClient.invalidateQueries({ queryKey: ['project-telemetry', projectId] })

    const overview = queryClient.setQueryData<Overview>(['project-overview', projectId], (old) => {
      if (!old) return old
      return { ...old, messagesToday: old.messagesToday + 1, lastMessageAt: event.timestamp }
    })
    if (!overview) void queryClient.invalidateQueries({ queryKey: ['project-overview', projectId] })

    void queryClient.invalidateQueries({ queryKey: ['telemetry-series', event.deviceId] })
    void queryClient.invalidateQueries({ queryKey: ['project-events', projectId] })
    void queryClient.invalidateQueries({ queryKey: ['device-events', event.deviceId] })
    patchDevice(queryClient, projectId, event.deviceId, { status: 'online', lastSeenAt: event.timestamp })
  }

  if (event.type === 'device.online' || event.type === 'device.offline') {
    const status = event.type === 'device.online' ? 'online' : 'offline'
    const patch: Partial<Device> = { status }
    if (event.type === 'device.online') patch.lastSeenAt = event.timestamp
    patchDevice(queryClient, projectId, event.deviceId, patch)
    void queryClient.invalidateQueries({ queryKey: ['project-overview', projectId] })
    void queryClient.invalidateQueries({ queryKey: ['project-events', projectId] })
  }

  if (event.type === 'state.updated') {
    queryClient.setQueryData(['device-state', event.deviceId], {
      deviceId: event.deviceId,
      state: event.data ?? null,
      updatedAt: event.timestamp,
    })
    void queryClient.invalidateQueries({ queryKey: ['device', event.deviceId] })
    void queryClient.invalidateQueries({ queryKey: ['project-events', projectId] })
    void queryClient.invalidateQueries({ queryKey: ['device-events', event.deviceId] })
  }

  if (event.type === 'command.updated') {
    void queryClient.invalidateQueries({ queryKey: ['device-commands', event.deviceId] })
    void queryClient.invalidateQueries({ queryKey: ['device-events', event.deviceId] })
    void queryClient.invalidateQueries({ queryKey: ['project-events', projectId] })
  }
}

function refreshProject(queryClient: QueryClient, projectId: string) {
  void queryClient.invalidateQueries({ queryKey: ['devices', projectId] })
  void queryClient.invalidateQueries({ queryKey: ['project-overview', projectId] })
  void queryClient.invalidateQueries({ queryKey: ['project-telemetry', projectId] })
  void queryClient.invalidateQueries({ queryKey: ['project-events', projectId] })
  void queryClient.invalidateQueries({ queryKey: ['device'] })
  void queryClient.invalidateQueries({ queryKey: ['telemetry-latest'] })
  void queryClient.invalidateQueries({ queryKey: ['telemetry-series'] })
  void queryClient.invalidateQueries({ queryKey: ['device-state'] })
  void queryClient.invalidateQueries({ queryKey: ['device-events'] })
  void queryClient.invalidateQueries({ queryKey: ['device-commands'] })
}

function patchDevice(queryClient: QueryClient, projectId: string, deviceId: string, patch: Partial<Device>) {
  const current = queryClient.setQueryData<{ device: Device }>(['device', deviceId], (old) => {
    if (!old) return old
    return { device: { ...old.device, ...patch } }
  })
  if (!current) void queryClient.invalidateQueries({ queryKey: ['device', deviceId] })

  let seen = false
  const list = queryClient.setQueryData<{ devices: Device[] }>(['devices', projectId], (old) => {
    if (!old) return old
    seen = old.devices.some((device) => device.id === deviceId)
    if (!seen) return old
    return {
      devices: old.devices.map((device) => (device.id === deviceId ? { ...device, ...patch } : device)),
    }
  })
  if (!list || !seen) void queryClient.invalidateQueries({ queryKey: ['devices', projectId] })
}

function mergeMetrics(metrics: TelemetryValue[], data: Record<string, unknown>, timestamp: string) {
  const next = metrics.map((item) => ({ ...item }))
  for (const [metric, raw] of Object.entries(data)) {
    const value = metricValue(metric, raw, timestamp)
    if (!value) continue
    const index = next.findIndex((item) => item.metric === metric)
    if (index >= 0) next[index] = value
    else next.push(value)
  }
  next.sort((a, b) => a.metric.localeCompare(b.metric))
  return next
}

function metricValue(metric: string, raw: unknown, timestamp: string): TelemetryValue | null {
  const value: TelemetryValue = {
    metric,
    numericValue: null,
    booleanValue: null,
    stringValue: null,
    recordedAt: timestamp,
    receivedAt: timestamp,
  }
  if (typeof raw === 'number' && Number.isFinite(raw)) value.numericValue = raw
  else if (typeof raw === 'boolean') value.booleanValue = raw
  else if (typeof raw === 'string') value.stringValue = raw
  else return null
  return value
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}
