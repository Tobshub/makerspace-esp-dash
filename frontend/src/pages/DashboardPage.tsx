import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, errorText } from '../api/client'
import { useSession } from '../useSession'

export function DashboardPage() {
  const { teams, projects, selectProject } = useSession()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      await api('/api/v1/teams', {
        method: 'POST',
        body: JSON.stringify({ name }),
      })
      setName('')
      await queryClient.invalidateQueries({ queryKey: ['nav'] })
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">Home</p>
      <h1>Your teams</h1>
      {teams.length === 0 ? (
        <p className="lede">Create a team, then open a project for the devices you want to connect.</p>
      ) : (
        <ul className="plain-list">
          {teams.map((team) => (
            <li key={team.id}>
              <Link to={`/teams/${team.id}`}>{team.name}</Link>
              <span className="muted"> {team.role}</span>
            </li>
          ))}
        </ul>
      )}

      <h2>Projects</h2>
      {projects.length === 0 ? (
        <p className="muted">No projects yet.</p>
      ) : (
        <ul className="plain-list">
          {projects.map((project) => (
            <li key={project.id}>
              <Link to={`/projects/${project.id}/devices`} onClick={() => selectProject(project.id)}>
                {project.name}
              </Link>
            </li>
          ))}
        </ul>
      )}

      <form className="form" onSubmit={onSubmit}>
        <h2>New team</h2>
        {error ? <p className="error">{errorText(error)}</p> : null}
        <label>
          Team name
          <input value={name} onChange={(event) => setName(event.target.value)} required />
        </label>
        <button type="submit" disabled={pending}>
          Create team
        </button>
      </form>
    </section>
  )
}
