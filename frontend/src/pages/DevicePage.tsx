import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  api,
  deviceTelemetryLatest,
  errorText,
  formatTelemetry,
  type Device,
  type IssuedDevice,
} from '../api/client'
import { CopyButton } from '../components/CopyButton'

export function DevicePage() {
  const { projectId = '', deviceId = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const device = useQuery({
    queryKey: ['device', deviceId],
    queryFn: () => api<{ device: Device }>(`/api/v1/devices/${deviceId}`),
    refetchInterval: (query) => {
      const current = query.state.data?.device
      if (!current || current.status === 'disabled') return false
      return 5000
    },
  })
  const latest = useQuery({
    queryKey: ['telemetry-latest', deviceId],
    queryFn: () => deviceTelemetryLatest(deviceId),
    enabled: deviceId !== '',
    refetchInterval: 5000,
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
        <p>Status: {current.status}</p>
        <p>Firmware: {current.firmwareVersion || 'Unknown until the device connects.'}</p>
        <p>Last seen: {current.lastSeenAt ? new Date(current.lastSeenAt).toLocaleString() : 'Never'}</p>
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
                  <span className="muted">{new Date(metric.recordedAt).toLocaleString()}</span>
                </span>
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
      <form className="form" onSubmit={(event) => void save(event)}>
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
