import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorText, fieldErrors, type Member, type Team } from '../api/client'

const roles = ['owner', 'admin', 'member', 'viewer']

export function TeamPage() {
  const { teamId = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const team = useQuery({
    queryKey: ['team', teamId],
    queryFn: () => api<{ team: Team }>(`/api/v1/teams/${teamId}`),
  })
  const members = useQuery({
    queryKey: ['members', teamId],
    queryFn: () => api<{ members: Member[] }>(`/api/v1/teams/${teamId}/members`),
  })
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('member')
  const [error, setError] = useState<unknown>(null)

  const canManage = team.data?.team.role === 'owner' || team.data?.team.role === 'admin'

  const rename = useMutation({
    mutationFn: () =>
      api(`/api/v1/teams/${teamId}`, {
        method: 'PATCH',
        body: JSON.stringify({ name }),
      }),
    onSuccess: async () => {
      setName('')
      await queryClient.invalidateQueries({ queryKey: ['team', teamId] })
      await queryClient.invalidateQueries({ queryKey: ['nav'] })
    },
  })

  const add = useMutation({
    mutationFn: () =>
      api(`/api/v1/teams/${teamId}/members`, {
        method: 'POST',
        body: JSON.stringify({ email, role }),
      }),
    onSuccess: async () => {
      setEmail('')
      await queryClient.invalidateQueries({ queryKey: ['members', teamId] })
    },
  })

  async function remove(memberId: string) {
    setError(null)
    try {
      await api(`/api/v1/teams/${teamId}/members/${memberId}`, { method: 'DELETE' })
      await queryClient.invalidateQueries({ queryKey: ['members', teamId] })
    } catch (err) {
      setError(err)
    }
  }

  async function destroy(event: FormEvent) {
    event.preventDefault()
    if (!window.confirm('Delete this team and its projects?')) return
    await api(`/api/v1/teams/${teamId}`, { method: 'DELETE' })
    await queryClient.invalidateQueries({ queryKey: ['nav'] })
    navigate('/dashboard')
  }

  if (team.isLoading) return <section className="page"><p>Loading…</p></section>
  if (team.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(team.error)}</p>
      </section>
    )
  }

  const current = team.data?.team
  const memberError = add.error ?? error

  return (
    <section className="page">
      <p className="eyebrow">Team</p>
      <h1>{current?.name}</h1>
      <p>
        <Link to={`/teams/${teamId}/projects`}>Projects</Link>
      </p>
      <h2>Members</h2>
      {members.isError ? <p className="error">{errorText(members.error)}</p> : null}
      <ul className="plain-list">
        {(members.data?.members ?? []).map((member) => (
          <li key={member.id}>
            {member.name} <span className="muted">{member.email}</span> · {member.role}
            {canManage ? (
              <button type="button" className="secondary" onClick={() => void remove(member.id)}>
                Remove
              </button>
            ) : null}
          </li>
        ))}
      </ul>
      {canManage ? (
        <form
          className="form"
          onSubmit={(event) => {
            event.preventDefault()
            add.mutate()
          }}
        >
          <h2>Add someone</h2>
          {memberError ? <p className="error">{errorText(memberError)}</p> : null}
          {fieldErrors(memberError).email ? (
            <p className="field-error">{fieldErrors(memberError).email}</p>
          ) : null}
          <label>
            Email
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
          </label>
          <label>
            Role
            <select value={role} onChange={(event) => setRole(event.target.value)}>
              {roles.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          <button type="submit" disabled={add.isPending}>
            Add member
          </button>
        </form>
      ) : null}
      {canManage ? (
        <form
          className="form"
          onSubmit={(event) => {
            event.preventDefault()
            rename.mutate()
          }}
        >
          <h2>Rename team</h2>
          {rename.isError ? <p className="error">{errorText(rename.error)}</p> : null}
          <label>
            Name
            <input value={name} onChange={(event) => setName(event.target.value)} placeholder={current?.name} required />
          </label>
          <button type="submit" disabled={rename.isPending}>
            Save
          </button>
        </form>
      ) : null}
      {current?.role === 'owner' ? (
        <form className="form" onSubmit={(event) => void destroy(event)}>
          <h2>Danger zone</h2>
          <button type="submit" className="danger">
            Delete team
          </button>
        </form>
      ) : null}
    </section>
  )
}
