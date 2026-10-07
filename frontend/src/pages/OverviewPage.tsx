import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQueries, useQuery } from '@tanstack/react-query'
import {
  deviceTelemetry,
  errorText,
  formatTelemetry,
  projectEvents,
  projectMetrics,
  projectOverview,
  projectTelemetryLatest,
  api,
  type Device,
  type TelemetryValue,
} from '../api/client'
import { MetricWidgets } from '../components/MetricWidgets'
import { StatusPill } from '../components/StatusPill'
import { TelemetryChart } from '../components/TelemetryChart'
import { chartRanges, eventLabel, formatWhen, isNumericMetric, rangeMs } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { useSession } from '../useSession'

export function OverviewPage() {
  const { projectId = '' } = useParams()
  const now = useNow()
  const { projects, teams } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const team = teams.find((item) => item.id === project?.teamId)
  const [deviceId, setDeviceId] = useState('')
  const [metric, setMetric] = useState('')
  const [range, setRange] = useState<(typeof chartRanges)[number]['id']>('24h')

  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => api<{ devices: Device[] }>(`/api/v1/projects/${projectId}/devices`),
    refetchInterval: liveInterval(),
  })
  const overview = useQuery({
    queryKey: ['project-overview', projectId],
    queryFn: () => projectOverview(projectId),
    refetchInterval: liveInterval(),
  })
  const latest = useQuery({
    queryKey: ['project-telemetry', projectId],
    queryFn: () => projectTelemetryLatest(projectId),
    refetchInterval: liveInterval(),
  })
  const events = useQuery({
    queryKey: ['project-events', projectId],
    queryFn: () => projectEvents(projectId),
    refetchInterval: liveInterval(),
  })
  const definitions = useQuery({
    queryKey: ['metrics', projectId],
    queryFn: () => projectMetrics(projectId),
    refetchInterval: liveInterval(),
  })

  const allDefinitions = definitions.data?.metrics ?? []
  const visibleDefinitions = allDefinitions.filter((item) => !item.hidden)
  const definedKeys = new Set(allDefinitions.map((item) => item.key))
  const hiddenKeys = new Set(allDefinitions.filter((item) => item.hidden).map((item) => item.key))
  const cards = metricCards(latest.data?.devices ?? []).filter((card) => !definedKeys.has(card.metric))
  const widgetDevices = (latest.data?.devices ?? []).map((device) => ({
    deviceId: device.deviceId,
    deviceName: device.name,
    metrics: device.metrics,
  }))
  const sparkFrom = new Date(now - rangeMs('24h')).toISOString()
  const sparks = useQueries({
    queries: cards.slice(0, 4).map((card) => ({
      queryKey: ['telemetry-series', card.deviceId, card.metric, '24h'],
      queryFn: () =>
        deviceTelemetry(card.deviceId, {
          metric: card.metric,
          from: sparkFrom,
          limit: 80,
        }),
      refetchInterval: liveInterval(),
    })),
  })

  const chartDevices = (latest.data?.devices ?? [])
    .map((device) => ({ ...device, metrics: device.metrics.filter((item) => !hiddenKeys.has(item.metric)) }))
    .filter((device) => device.metrics.some(isNumericMetric))
  const selectedDevice = chartDevices.find((device) => device.deviceId === deviceId) ?? chartDevices[0]
  const numeric = selectedDevice?.metrics.filter(isNumericMetric) ?? []
  const selectedMetric = numeric.some((item) => item.metric === metric) ? metric : (numeric[0]?.metric ?? '')
  const historyFrom = new Date(now - rangeMs(range)).toISOString()
  const history = useQuery({
    queryKey: ['telemetry-series', selectedDevice?.deviceId ?? '', selectedMetric, range],
    queryFn: () =>
      deviceTelemetry(selectedDevice?.deviceId ?? '', {
        metric: selectedMetric,
        from: historyFrom,
      }),
    enabled: Boolean(selectedDevice && selectedMetric),
    refetchInterval: liveInterval(),
  })

  if (devices.isLoading || overview.isLoading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (devices.isError || overview.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(devices.error ?? overview.error)}</p>
      </section>
    )
  }

  const items = devices.data?.devices ?? []
  const counts = overview.data?.devices

  return (
    <section className="page">
      <p className="eyebrow">{team?.name ?? 'Project'}</p>
      <h1>{project?.name ?? 'Overview'}</h1>
      {items.length === 0 ? (
        <div className="card">
          <p>No devices yet.</p>
          <p>Connect an ESP32 to start sending telemetry.</p>
          <Link className="button" to={`/projects/${projectId}/devices/new`}>
            Add Device
          </Link>
        </div>
      ) : null}

      <div className="stat-grid">
        <div className="card stat">
          <p className="muted">Devices</p>
          <p className="stat-value">{counts?.total ?? 0}</p>
        </div>
        <div className="card stat">
          <p className="muted">Online</p>
          <p className="stat-value">{counts?.online ?? 0}</p>
        </div>
        <div className="card stat">
          <p className="muted">Offline</p>
          <p className="stat-value">{counts?.offline ?? 0}</p>
        </div>
        <div className="card stat">
          <p className="muted">Messages today</p>
          <p className="stat-value">{overview.data?.messagesToday ?? 0}</p>
          <p className="muted">Last {formatWhen(overview.data?.lastMessageAt, now)}</p>
        </div>
      </div>
      {(counts?.unknown ?? 0) > 0 ? (
        <p className="muted">{counts?.unknown} waiting for a first connection.</p>
      ) : null}
      <p className="muted">
        <Link to={`/projects/${projectId}/alerts`}>Active alerts: {overview.data?.activeAlerts ?? 0}</Link>
      </p>

      {definitions.isError ? <p className="error">{errorText(definitions.error)}</p> : null}
      <MetricWidgets definitions={visibleDefinitions} devices={widgetDevices} />

      {latest.isError ? <p className="error">{errorText(latest.error)}</p> : null}
      {latest.isLoading ? <p>Loading telemetry…</p> : null}
      {!latest.isLoading && cards.length === 0 && visibleDefinitions.length === 0 && items.length > 0 && !hasTelemetry(latest.data?.devices ?? []) ? (
        <p>Waiting for telemetry…</p>
      ) : null}
      {!latest.isLoading && hasTelemetry(latest.data?.devices ?? []) && cards.length === 0 && visibleDefinitions.length === 0 ? (
        <p className="muted">
          All current metrics are hidden. <Link to={`/projects/${projectId}/metrics`}>Show them again</Link> from Metrics.
        </p>
      ) : null}
      {cards.length > 0 ? (
        <p className="muted">
          <Link to={`/projects/${projectId}/metrics`}>Configure metrics</Link> to name these and choose a chart, gauge, or status.
        </p>
      ) : null}
      {cards.length > 0 ? (
        <div className="card-grid">
          {cards.map((card, index) => (
            <div className="card" key={`${card.deviceId}-${card.metric}`}>
              <p className="muted">
                {card.metric} · {card.deviceName}
              </p>
              <p className="stat-value">{formatTelemetry(card.value)}</p>
              <p className="muted">{formatWhen(card.value.recordedAt, now)}</p>
              {index < sparks.length && sparks[index]?.data ? (
                <TelemetryChart points={sparks[index].data?.points ?? []} height={56} compact />
              ) : null}
            </div>
          ))}
        </div>
      ) : null}

      <div className="card">
        <h2>History</h2>
        {chartDevices.length === 0 ? (
          <p className="muted">No numeric telemetry to chart yet.</p>
        ) : (
          <>
            <div className="filters">
              <label>
                Device
                <select value={selectedDevice?.deviceId ?? ''} onChange={(event) => setDeviceId(event.target.value)}>
                  {chartDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Metric
                <select value={selectedMetric} onChange={(event) => setMetric(event.target.value)}>
                  {numeric.map((item) => (
                    <option key={item.metric} value={item.metric}>
                      {visibleDefinitions.find((definition) => definition.key === item.metric)?.name ?? item.metric}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Range
                <select value={range} onChange={(event) => setRange(event.target.value as (typeof chartRanges)[number]['id'])}>
                  {chartRanges.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {history.isLoading ? <p>Loading chart…</p> : null}
            {history.isError ? <p className="error">{errorText(history.error)}</p> : null}
            {history.data ? <TelemetryChart points={history.data.points} /> : null}
            {history.data?.truncated ? <p className="muted">Showing the newest points in this window.</p> : null}
          </>
        )}
      </div>

      <h2>Recent events</h2>
      {events.isLoading ? <p>Loading…</p> : null}
      {events.isError ? <p className="error">{errorText(events.error)}</p> : null}
      {events.data && events.data.events.length === 0 ? <p className="muted">No events yet.</p> : null}
      {events.data && events.data.events.length > 0 ? (
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Device</th>
              <th>Event</th>
            </tr>
          </thead>
          <tbody>
            {events.data.events.map((event) => (
              <tr key={event.id}>
                <td>{formatWhen(event.createdAt, now)}</td>
                <td>
                  <Link to={`/projects/${projectId}/devices/${event.deviceId}`}>{event.deviceName}</Link>
                </td>
                <td>{eventLabel(event.eventType)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}

      <h2>Devices</h2>
      {items.length === 0 ? null : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Last seen</th>
            </tr>
          </thead>
          <tbody>
            {items.map((device) => (
              <tr key={device.id}>
                <td>
                  <Link to={`/projects/${projectId}/devices/${device.id}`}>{device.name}</Link>
                </td>
                <td>
                  <StatusPill status={device.status} />
                </td>
                <td>{formatWhen(device.lastSeenAt, now)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function hasTelemetry(devices: { metrics: TelemetryValue[] }[]) {
  return devices.some((device) => device.metrics.length > 0)
}

function metricCards(
  devices: { deviceId: string; name: string; metrics: TelemetryValue[] }[],
) {
  const cards: { deviceId: string; deviceName: string; metric: string; value: TelemetryValue }[] = []
  for (const device of devices) {
    for (const value of device.metrics) {
      cards.push({ deviceId: device.deviceId, deviceName: device.name, metric: value.metric, value })
    }
  }
  cards.sort((a, b) => Date.parse(b.value.recordedAt) - Date.parse(a.value.recordedAt))
  return cards.slice(0, 8)
}
