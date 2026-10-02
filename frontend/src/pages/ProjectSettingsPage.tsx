import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText, type BrokerInfo, type Project, type Team } from '../api/client'

export function ProjectSettingsPage() {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api<{ project: Project }>(`/api/v1/projects/${projectId}`),
  })
  const broker = useQuery({
    queryKey: ['broker'],
    queryFn: () => api<BrokerInfo>('/api/v1/broker'),
  })
  const teamId = project.data?.project.teamId
  const team = useQuery({
    queryKey: ['team', teamId],
    enabled: Boolean(teamId),
    queryFn: () => api<{ team: Team }>(`/api/v1/teams/${teamId}`),
  })
  const [draft, setDraft] = useState<{ name: string; description: string } | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const name = draft?.name ?? project.data?.project.name ?? ''
  const description = draft?.description ?? project.data?.project.description ?? ''

  if (project.isLoading || broker.isLoading) return <section className="page"><p>Loading…</p></section>
  if (project.isError || broker.isError) {
    return <section className="page"><p className="error">{errorText(project.error ?? broker.error)}</p></section>
  }

  const current = project.data?.project
  const info = broker.data
  const canWrite = team.data && team.data.team.role !== 'viewer'
  const canDelete = team.data?.team.role === 'owner' || team.data?.team.role === 'admin'

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      await api(`/api/v1/projects/${projectId}`, {
        method: 'PATCH',
        body: JSON.stringify({ name, description }),
      })
      await queryClient.invalidateQueries({ queryKey: ['project', projectId] })
      await queryClient.invalidateQueries({ queryKey: ['nav'] })
      setDraft(null)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  async function destroy() {
    if (!current || !window.confirm(`Delete ${current.name}? Devices in this project are deleted too.`)) return
    try {
      await api(`/api/v1/projects/${projectId}`, { method: 'DELETE' })
      await queryClient.invalidateQueries({ queryKey: ['nav'] })
      navigate(`/teams/${current.teamId}/projects`)
    } catch (err) {
      setError(err)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">Project settings</p>
      <h1>{current?.name}</h1>
      {canWrite ? (
        <form className="form" onSubmit={onSubmit}>
          {error ? <p className="error">{errorText(error)}</p> : null}
          <label>
            Project name
            <input
              value={name}
              onChange={(event) => setDraft({ name: event.target.value, description })}
              required
            />
          </label>
          <label>
            Description
            <textarea
              value={description}
              onChange={(event) => setDraft({ name, description: event.target.value })}
            />
          </label>
          <button type="submit" disabled={pending}>
            Save
          </button>
        </form>
      ) : (
        <p>{current?.description}</p>
      )}
      {info ? (
        <div className="card">
          <h2>MQTT connection</h2>
          <p>Host {info.host}</p>
          <p>Port {info.port}</p>
          <p>TLS {info.tls ? 'on' : 'off'}</p>
          <p>Device offline timeout {info.offlineTimeoutSeconds} seconds</p>
          <p className="muted">
            Each device uses its device key as the MQTT username. Broker administrator passwords are not shown here.
            {info.anonymousLocal
              ? ' This local broker still allows anonymous connections. Per-device topic limits arrive with production hardening.'
              : ''}
          </p>
          <p className="muted">Telemetry topic pattern: {info.topics.telemetry}</p>
        </div>
      ) : null}
      {canDelete && current ? (
        <div className="card">
          <h2>Danger zone</h2>
          <button type="button" className="danger" onClick={() => void destroy()}>
            Delete project
          </button>
        </div>
      ) : null}
      <p>
        <Link to={`/projects/${projectId}/devices`}>Devices</Link>
      </p>
    </section>
  )
}
