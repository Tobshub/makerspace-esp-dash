import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { errorText, projectDevices, projectEvents } from '../api/client'
import { eventLabel, eventSummary, formatWhen } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { useSession } from '../useSession'

const eventTypes = ['', 'connected', 'disconnected', 'telemetry', 'state', 'command', 'command_ack', 'error']

export function EventsPage() {
  const { projectId = '' } = useParams()
  const now = useNow()
  const { projects } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const [deviceId, setDeviceId] = useState('')
  const [eventType, setEventType] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => projectDevices(projectId),
  })
  const events = useQuery({
    queryKey: ['project-events', projectId, deviceId, eventType, from, to],
    queryFn: () =>
      projectEvents(projectId, {
        limit: 100,
        deviceId,
        eventType,
        from: toISO(from),
        to: toISO(to),
      }),
    refetchInterval: liveInterval(),
  })

  return (
    <section className="page">
      <p className="eyebrow">{project?.name ?? 'Project'}</p>
      <h1>Events</h1>
      <p className="lede">Connection, telemetry, commands, and acknowledgements as they arrive.</p>
      <div className="filters">
        <label>
          Device
          <select value={deviceId} onChange={(event) => setDeviceId(event.target.value)}>
            <option value="">All devices</option>
            {devices.data?.devices.map((device) => (
              <option key={device.id} value={device.id}>
                {device.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Event
          <select value={eventType} onChange={(event) => setEventType(event.target.value)}>
            {eventTypes.map((item) => (
              <option key={item || 'all'} value={item}>
                {item ? eventLabel(item) : 'All types'}
              </option>
            ))}
          </select>
        </label>
        <label>
          From
          <input type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} />
        </label>
        <label>
          To
          <input type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} />
        </label>
      </div>
      {events.isLoading ? <p>Loading…</p> : null}
      {events.isError ? <p className="error">{errorText(events.error)}</p> : null}
      {events.data && events.data.events.length === 0 ? <p className="muted">No events match these filters.</p> : null}
      {events.data && events.data.events.length > 0 ? (
        <ul className="event-log">
          {events.data.events.map((event) => {
            const summary = eventSummary(event.eventType, event.payload)
            return (
              <li key={event.id}>
                <details>
                  <summary>
                    <span>{new Date(event.createdAt).toLocaleTimeString()}</span>
                    <strong>{eventLabel(event.eventType)}</strong>
                    <span>{event.deviceName}</span>
                    <span className="muted">{summary || formatWhen(event.createdAt, now)}</span>
                  </summary>
                  <pre className="snippet">{JSON.stringify(event.payload, null, 2)}</pre>
                </details>
              </li>
            )
          })}
        </ul>
      ) : null}
    </section>
  )
}

function toISO(value: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString()
}
