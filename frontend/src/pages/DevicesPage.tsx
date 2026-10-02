import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, canWrite, errorText, type Device, type IssuedDevice } from '../api/client'
import { CopyButton } from '../components/CopyButton'
import { StatusPill } from '../components/StatusPill'
import { formatWhen } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { useSession } from '../useSession'

export function DevicesPage() {
  const { projectId = '' } = useParams()
  const now = useNow()
  const { teams, projects } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const role = teams.find((team) => team.id === project?.teamId)?.role
  const write = canWrite(role)
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const [issued, setIssued] = useState<IssuedDevice | null>(null)
  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => api<{ devices: Device[] }>(`/api/v1/projects/${projectId}/devices`),
    refetchInterval: liveInterval(),
  })

  if (devices.isLoading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (devices.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(devices.error)}</p>
      </section>
    )
  }

  const items = devices.data?.devices ?? []

  async function refresh() {
    await queryClient.invalidateQueries({ queryKey: ['devices', projectId] })
    await queryClient.invalidateQueries({ queryKey: ['project-overview', projectId] })
  }

  async function rotate(deviceId: string) {
    if (!window.confirm('Rotate the secret? The current password will stop matching.')) return
    setError(null)
    try {
      const body = await api<IssuedDevice>(`/api/v1/devices/${deviceId}/rotate-secret`, { method: 'POST' })
      setIssued(body)
    } catch (err) {
      setError(err)
    }
  }

  async function disable(deviceId: string) {
    setError(null)
    try {
      await api(`/api/v1/devices/${deviceId}/disable`, { method: 'POST' })
      await refresh()
    } catch (err) {
      setError(err)
    }
  }

  async function remove(deviceId: string) {
    if (!window.confirm('Delete this device?')) return
    setError(null)
    try {
      await api(`/api/v1/devices/${deviceId}`, { method: 'DELETE' })
      await refresh()
    } catch (err) {
      setError(err)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">Devices</p>
      <h1>Devices</h1>
      {error ? <p className="error">{errorText(error)}</p> : null}
      {issued ? (
        <div className="stack">
          <div className="banner">New secret for {issued.device.name}, shown once.</div>
          <p className="secret-value">{issued.credentials.secret}</p>
          <CopyButton value={issued.credentials.secret} />
        </div>
      ) : null}
      {items.length === 0 ? (
        <div className="card">
          <p>No devices yet.</p>
          <p>Connect an ESP32 to start sending telemetry.</p>
          {write ? (
            <Link className="button" to={`/projects/${projectId}/devices/new`}>
              Add Device
            </Link>
          ) : null}
        </div>
      ) : (
        <>
          {write ? (
            <p>
              <Link className="button" to={`/projects/${projectId}/devices/new`}>
                Add Device
              </Link>
            </p>
          ) : null}
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Device key</th>
                <th>Status</th>
                <th>Last seen</th>
                <th>Firmware</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((device) => (
                <tr key={device.id}>
                  <td>
                    <Link to={`/projects/${projectId}/devices/${device.id}`}>{device.name}</Link>
                  </td>
                  <td>{device.deviceKey}</td>
                  <td>
                    <StatusPill status={device.status} />
                  </td>
                  <td>{formatWhen(device.lastSeenAt, now)}</td>
                  <td>{device.firmwareVersion || '—'}</td>
                  <td>
                    <div className="actions">
                      <Link className="button small secondary" to={`/projects/${projectId}/devices/${device.id}`}>
                        Open
                      </Link>
                      <Link className="button small secondary" to={`/projects/${projectId}/devices/${device.id}#details`}>
                        Edit
                      </Link>
                      {write ? (
                        <>
                          <button type="button" className="secondary small" onClick={() => void rotate(device.id)}>
                            Rotate secret
                          </button>
                          {device.status !== 'disabled' ? (
                            <button type="button" className="secondary small" onClick={() => void disable(device.id)}>
                              Disable
                            </button>
                          ) : null}
                          <button type="button" className="danger small" onClick={() => void remove(device.id)}>
                            Delete
                          </button>
                        </>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </section>
  )
}
