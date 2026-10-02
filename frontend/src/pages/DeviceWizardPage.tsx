import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, errorText, fieldErrors, type BrokerInfo, type Device, type IssuedDevice } from '../api/client'
import { CopyButton } from '../components/CopyButton'

export function DeviceWizardPage() {
  const { projectId = '' } = useParams()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const [issued, setIssued] = useState<IssuedDevice | null>(null)
  const broker = useQuery({
    queryKey: ['broker'],
    queryFn: () => api<BrokerInfo>('/api/v1/broker'),
  })
  const deviceId = issued?.device.id
  const live = useQuery({
    queryKey: ['device', deviceId],
    enabled: Boolean(deviceId),
    queryFn: () => api<{ device: Device }>(`/api/v1/devices/${deviceId}`),
    refetchInterval: (query) => {
      const current = query.state.data?.device
      if (!current || current.lastSeenAt || current.status === 'online' || current.status === 'disabled') {
        return false
      }
      return 3000
    },
  })

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      const body = await api<IssuedDevice>(`/api/v1/projects/${projectId}/devices`, {
        method: 'POST',
        body: JSON.stringify({ name, description }),
      })
      setIssued(body)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  const connected = Boolean(live.data?.device.lastSeenAt || live.data?.device.status === 'online')
  const fields = fieldErrors(error)

  return (
    <section className="page">
      <p className="eyebrow">Add device</p>
      <h1>{issued ? issued.device.name : 'Add Device'}</h1>
      <p>
        <Link to={`/projects/${projectId}/devices`}>All devices</Link>
      </p>
      {!issued ? (
        <form className="form" onSubmit={onSubmit}>
          {error ? <p className="error">{errorText(error)}</p> : null}
          <label>
            Device name
            <input value={name} onChange={(event) => setName(event.target.value)} required />
            {fields.name ? <span className="field-error">{fields.name}</span> : null}
          </label>
          <label>
            Description
            <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
          </label>
          <button type="submit" disabled={pending}>
            Create credentials
          </button>
        </form>
      ) : (
        <div className="stack">
          <div className="banner">
            Save the secret now. It is shown once and is not stored in plaintext.
          </div>
          <Credential label="Device key" value={issued.device.deviceKey} />
          <Credential label="Device secret" value={issued.credentials.secret} />
          <Credential label="MQTT host" value={issued.connection.host} />
          <Credential label="MQTT port" value={String(issued.connection.port)} />
          <Credential label="MQTT username" value={issued.connection.username} />
          <Credential label="MQTT password" value={issued.connection.password} />
          <Credential label="Telemetry topic" value={issued.connection.telemetryTopic} />
          <Credential label="Presence topic" value={issued.connection.statusTopic} />
          <Credential label="Command topic" value={issued.connection.commandTopic} />
          {broker.data?.anonymousLocal ? (
            <p className="muted">
              This local broker allows anonymous connections. Keep the password anyway. Production uses TLS and
              limits each device to its own topics.
            </p>
          ) : null}
          <h2>Firmware snippet</h2>
          <pre className="snippet">{issued.firmwareSnippet}</pre>
          <CopyButton value={issued.firmwareSnippet} />
          <div className="card">
            {connected ? (
              <p>Device connected {live.data?.device.lastSeenAt ? new Date(live.data.device.lastSeenAt).toLocaleString() : ''}.</p>
            ) : (
              <p>Waiting for device…</p>
            )}
            {live.isError ? <p className="error">{errorText(live.error)}</p> : null}
          </div>
          <Link to={`/projects/${projectId}/devices/${issued.device.id}`}>Open device</Link>
        </div>
      )}
    </section>
  )
}

function Credential({ label, value }: { label: string; value: string }) {
  return (
    <div className="card">
      <p className="eyebrow">{label}</p>
      <p className="secret-value">{value}</p>
      <CopyButton value={value} />
    </div>
  )
}
