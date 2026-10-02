import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  api,
  deviceEvents,
  deviceState,
  deviceTelemetry,
  deviceTelemetryLatest,
  errorText,
  formatTelemetry,
  type Device,
  type IssuedDevice,
} from '../api/client'
import { CopyButton } from '../components/CopyButton'
import { StatusPill } from '../components/StatusPill'
import { TelemetryChart } from '../components/TelemetryChart'
import { chartRanges, eventLabel, formatWhen, isNumericMetric, rangeMs, wifiRssi } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'

export function DevicePage() {
  const { projectId = '', deviceId = '' } = useParams()
  const navigate = useNavigate()
  const now = useNow()
  const queryClient = useQueryClient()
  const [metricChoice, setMetricChoice] = useState('')
  const [range, setRange] = useState<(typeof chartRanges)[number]['id']>('24h')
  const device = useQuery({
    queryKey: ['device', deviceId],
    queryFn: () => api<{ device: Device }>(`/api/v1/devices/${deviceId}`),
    refetchInterval: (query) => {
      const current = query.state.data?.device
      if (!current || current.status === 'disabled') return false
      return liveInterval()
    },
  })
  const latest = useQuery({
    queryKey: ['telemetry-latest', deviceId],
    queryFn: () => deviceTelemetryLatest(deviceId),
    enabled: deviceId !== '',
    refetchInterval: liveInterval(),
  })
  const state = useQuery({
    queryKey: ['device-state', deviceId],
    queryFn: () => deviceState(deviceId),
    enabled: deviceId !== '',
    refetchInterval: liveInterval(),
  })
  const events = useQuery({
    queryKey: ['device-events', deviceId],
    queryFn: () => deviceEvents(deviceId, { limit: 20 }),
    enabled: deviceId !== '',
    refetchInterval: liveInterval(),
  })
  const commands = useQuery({
    queryKey: ['device-commands', deviceId],
    queryFn: () => deviceEvents(deviceId, { limit: 20, eventType: 'command_ack' }),
    enabled: deviceId !== '',
    refetchInterval: liveInterval(),
  })
  const numeric = latest.data?.metrics.filter(isNumericMetric) ?? []
  const selectedMetric = numeric.some((item) => item.metric === metricChoice) ? metricChoice : (numeric[0]?.metric ?? '')
  const historyFrom = new Date(now - rangeMs(range)).toISOString()
  const history = useQuery({
    queryKey: ['telemetry-series', deviceId, selectedMetric, range],
    queryFn: () =>
      deviceTelemetry(deviceId, {
        metric: selectedMetric,
        from: historyFrom,
      }),
    enabled: deviceId !== '' && selectedMetric !== '',
    refetchInterval: liveInterval(),
  })
  const [draft, setDraft] = useState<{ name: string; description: string } | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [issued, setIssued] = useState<IssuedDevice | null>(null)
  const name = draft?.name ?? device.data?.device.name ?? ''
  const description = draft?.description ?? device.data?.device.description ?? ''

  if (device.isLoading) return <section className="page"><p>Loading…</p></section>
  if (device.isError || !device.data) {
    return <section className="page"><p className="error">{errorText(device.error)}</p></section>
  }

  const current = device.data.device
  if (current.projectId !== projectId) {
    return <section className="page"><p className="error">Not found</p></section>
  }

  const waiting = !current.lastSeenAt && current.status !== 'online' && current.status !== 'disabled'
  const rssi = wifiRssi(state.data?.state)

  async function save(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await api(`/api/v1/devices/${deviceId}`, {
        method: 'PATCH',
        body: JSON.stringify({ name, description }),
      })
      await queryClient.invalidateQueries({ queryKey: ['device', deviceId] })
      await queryClient.invalidateQueries({ queryKey: ['devices', projectId] })
      setDraft(null)
    } catch (err) {
      setError(err)
    }
  }

  async function rotate() {
    if (!window.confirm('Rotate the secret? The current password will stop matching.')) return
    setError(null)
    try {
      const body = await api<IssuedDevice>(`/api/v1/devices/${deviceId}/rotate-secret`, { method: 'POST' })
      setIssued(body)
    } catch (err) {
      setError(err)
    }
  }

  async function disable() {
    setError(null)
    try {
      await api(`/api/v1/devices/${deviceId}/disable`, { method: 'POST' })
      await queryClient.invalidateQueries({ queryKey: ['device', deviceId] })
    } catch (err) {
      setError(err)
    }
  }

  async function remove() {
    if (!window.confirm('Delete this device?')) return
    try {
      await api(`/api/v1/devices/${deviceId}`, { method: 'DELETE' })
      await queryClient.invalidateQueries({ queryKey: ['devices', projectId] })
      navigate(`/projects/${projectId}/devices`)
    } catch (err) {
      setError(err)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">{current.deviceKey}</p>
      <h1>{current.name}</h1>
      <p>
        <Link to={`/projects/${projectId}/devices`}>All devices</Link>
      </p>
      {error ? <p className="error">{errorText(error)}</p> : null}
      <div className="card">
        <h2>Summary</h2>
        <p>
          Status: <StatusPill status={current.status} />
        </p>
        <p>Firmware: {current.firmwareVersion || 'Unknown until the device connects.'}</p>
        <p>Last seen: {current.lastSeenAt ? formatWhen(current.lastSeenAt, now) : 'Never'}</p>
        <p>Wi-Fi RSSI: {rssi == null ? '—' : `${rssi} dBm`}</p>
        <p>Created: {new Date(current.createdAt).toLocaleString()}</p>
        {waiting ? <p>Waiting for device…</p> : null}
        {current.status === 'disabled' ? <p>This device is disabled and should not connect.</p> : null}
      </div>
      <div className="card">
        <h2>Latest telemetry</h2>
        {latest.isLoading ? <p>Loading…</p> : null}
        {latest.isError ? <p className="error">{errorText(latest.error)}</p> : null}
        {latest.data && latest.data.metrics.length === 0 ? <p>No telemetry yet.</p> : null}
        {latest.data && latest.data.metrics.length > 0 ? (
          <ul className="metric-list">
            {latest.data.metrics.map((metric) => (
              <li key={metric.metric}>
                <span>{metric.metric}</span>
                <span>
                  {formatTelemetry(metric)}{' '}
                  <span className="muted">{formatWhen(metric.recordedAt, now)}</span>
                </span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <div className="card">
        <h2>Chart</h2>
        {numeric.length === 0 ? (
          <p className="muted">No numeric telemetry to chart yet.</p>
        ) : (
          <>
            <div className="filters">
              <label>
                Metric
                <select value={selectedMetric} onChange={(event) => setMetricChoice(event.target.value)}>
                  {numeric.map((item) => (
                    <option key={item.metric} value={item.metric}>
                      {item.metric}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Range
                <select
                  value={range}
                  onChange={(event) => setRange(event.target.value as (typeof chartRanges)[number]['id'])}
                >
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
      <div className="card">
        <h2>State</h2>
        {state.isLoading ? <p>Loading…</p> : null}
        {state.isError ? <p className="error">{errorText(state.error)}</p> : null}
        {state.data && state.data.state == null ? <p className="muted">No state reported yet.</p> : null}
        {state.data?.state != null ? (
          <>
            <p className="muted">{state.data.updatedAt ? formatWhen(state.data.updatedAt, now) : null}</p>
            <pre className="snippet">{JSON.stringify(state.data.state, null, 2)}</pre>
          </>
        ) : null}
      </div>
      <div className="card">
        <h2>Commands</h2>
        {commands.isLoading ? <p>Loading…</p> : null}
        {commands.isError ? <p className="error">{errorText(commands.error)}</p> : null}
        {commands.data && commands.data.events.length === 0 ? (
          <p className="muted">No commands yet. Acknowledgements show up here when a device answers one.</p>
        ) : null}
        {commands.data && commands.data.events.length > 0 ? (
          <ul className="metric-list">
            {commands.data.events.map((event) => (
              <li key={event.id}>
                <span>Acknowledged</span>
                <span className="muted">{formatWhen(event.createdAt, now)}</span>
              </li>
            ))}
          </ul>
        ) : null}
        {commands.data?.events.map((event) => (
          <pre className="snippet" key={`${event.id}-payload`}>
            {JSON.stringify(event.payload, null, 2)}
          </pre>
        ))}
      </div>
      <div className="card">
        <h2>Events</h2>
        {events.isLoading ? <p>Loading…</p> : null}
        {events.isError ? <p className="error">{errorText(events.error)}</p> : null}
        {events.data && events.data.events.length === 0 ? <p className="muted">No events yet.</p> : null}
        {events.data && events.data.events.length > 0 ? (
          <ul className="metric-list">
            {events.data.events.map((event) => (
              <li key={event.id}>
                <span>{eventLabel(event.eventType)}</span>
                <span className="muted">{formatWhen(event.createdAt, now)}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      {issued ? (
        <div className="stack">
          <div className="banner">New secret, shown once. It replaces the previous password.</div>
          <p className="secret-value">{issued.credentials.secret}</p>
          <CopyButton value={issued.credentials.secret} />
          <pre className="snippet">{issued.firmwareSnippet}</pre>
        </div>
      ) : (
        <p className="muted">The device secret is not stored and cannot be shown again. Rotate it if you need a new one.</p>
      )}
      <form className="form" id="details" onSubmit={(event) => void save(event)}>
        <h2>Details</h2>
        <label>
          Name
          <input value={name} onChange={(event) => setDraft({ name: event.target.value, description })} required />
        </label>
        <label>
          Description
          <textarea value={description} onChange={(event) => setDraft({ name, description: event.target.value })} />
        </label>
        <button type="submit">Save</button>
      </form>
      <div className="row">
        <button type="button" className="secondary" onClick={() => void rotate()}>
          Rotate secret
        </button>
        {current.status !== 'disabled' ? (
          <button type="button" className="secondary" onClick={() => void disable()}>
            Disable
          </button>
        ) : null}
        <button type="button" className="danger" onClick={() => void remove()}>
          Delete
        </button>
      </div>
    </section>
  )
}
