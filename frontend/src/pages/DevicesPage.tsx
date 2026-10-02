import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, errorText, type Device } from '../api/client'

export function DevicesPage() {
  const { projectId = '' } = useParams()
  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => api<{ devices: Device[] }>(`/api/v1/projects/${projectId}/devices`),
    refetchInterval: 5000,
  })

  if (devices.isLoading) return <section className="page"><p>Loading…</p></section>
  if (devices.isError) {
    return <section className="page"><p className="error">{errorText(devices.error)}</p></section>
  }

  const items = devices.data?.devices ?? []

  return (
    <section className="page">
      <p className="eyebrow">Devices</p>
      <h1>Devices</h1>
      {items.length === 0 ? (
        <div className="card">
          <p>No devices yet.</p>
          <p>Connect an ESP32 to start sending telemetry.</p>
          <Link className="button" to={`/projects/${projectId}/devices/new`}>
            Add Device
          </Link>
        </div>
      ) : (
        <>
          <p>
            <Link className="button" to={`/projects/${projectId}/devices/new`}>
              Add Device
            </Link>
          </p>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Key</th>
                <th>Status</th>
                <th>Firmware</th>
                <th>Last seen</th>
              </tr>
            </thead>
            <tbody>
              {items.map((device) => (
                <tr key={device.id}>
                  <td>
                    <Link to={`/projects/${projectId}/devices/${device.id}`}>{device.name}</Link>
                  </td>
                  <td>{device.deviceKey}</td>
                  <td>{device.status}</td>
                  <td>{device.firmwareVersion || '—'}</td>
                  <td>{device.lastSeenAt ? new Date(device.lastSeenAt).toLocaleString() : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </section>
  )
}
