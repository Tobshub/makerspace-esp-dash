import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText, type Project, type Team } from '../api/client'
import { useSession } from '../useSession'

export function TeamProjectsPage() {
  const { teamId = '' } = useParams()
  const { selectProject } = useSession()
  const queryClient = useQueryClient()
  const team = useQuery({
    queryKey: ['team', teamId],
    queryFn: () => api<{ team: Team }>(`/api/v1/teams/${teamId}`),
  })
  const projects = useQuery({
    queryKey: ['projects', teamId],
    queryFn: () => api<{ projects: Project[] }>(`/api/v1/teams/${teamId}/projects`),
  })
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const canWrite = team.data && team.data.team.role !== 'viewer'

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      await api(`/api/v1/teams/${teamId}/projects`, {
        method: 'POST',
        body: JSON.stringify({ name, description }),
      })
      setName('')
      setDescription('')
      await queryClient.invalidateQueries({ queryKey: ['projects', teamId] })
      await queryClient.invalidateQueries({ queryKey: ['nav'] })
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  if (team.isLoading || projects.isLoading) return <section className="page"><p>Loading…</p></section>
  if (team.isError || projects.isError) {
    return <section className="page"><p className="error">{errorText(team.error ?? projects.error)}</p></section>
  }

  return (
    <section className="page">
      <p className="eyebrow">{team.data?.team.name}</p>
      <h1>Projects</h1>
      {(projects.data?.projects.length ?? 0) === 0 ? (
        <p className="lede">No projects yet. Create one for a product, then register its ESP32.</p>
      ) : (
        <ul className="plain-list">
          {projects.data?.projects.map((project) => (
            <li key={project.id}>
              <Link
                to={`/projects/${project.id}/devices`}
                onClick={() => selectProject(project.id)}
              >
                {project.name}
              </Link>
              {project.description ? <span className="muted"> — {project.description}</span> : null}
            </li>
          ))}
        </ul>
      )}
      {canWrite ? (
        <form className="form" onSubmit={onSubmit}>
          <h2>New project</h2>
          {error ? <p className="error">{errorText(error)}</p> : null}
          <label>
            Name
            <input value={name} onChange={(event) => setName(event.target.value)} required />
          </label>
          <label>
            Description
            <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
          </label>
          <button type="submit" disabled={pending}>
            Create project
          </button>
        </form>
      ) : null}
    </section>
  )
}
