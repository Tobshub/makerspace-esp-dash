import { useQuery, useQueryClient } from '@tanstack/react-query'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useProjectStream, type AlertNotice } from '../live'
import { useSession } from '../useSession'

export function AppShell() {
  const { user, loading, teams, projects, projectId, selectProject, signOut } = useSession()
  const navigate = useNavigate()
  const location = useLocation()

  const projectLinks = projectId
    ? [
        { to: `/projects/${projectId}/overview`, label: 'Overview' },
        { to: `/projects/${projectId}/devices`, label: 'Devices' },
        { to: `/projects/${projectId}/metrics`, label: 'Metrics' },
        { to: `/projects/${projectId}/controls`, label: 'Controls' },
        { to: `/projects/${projectId}/alerts`, label: 'Alerts' },
        { to: `/projects/${projectId}/events`, label: 'Events' },
        { to: `/projects/${projectId}/settings`, label: 'Project Settings' },
      ]
    : []

  function chooseProject(id: string) {
    selectProject(id)
    const match = location.pathname.match(/^\/projects\/[^/]+\/(.+)$/)
    if (!match) {
      navigate(`/projects/${id}/overview`)
      return
    }
    const rest = match[1].startsWith('devices/') ? 'devices' : match[1]
    navigate(`/projects/${id}/${rest}`)
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <NavLink to="/" className="brand">
          Makerspace
        </NavLink>
        {user ? (
          <>
            <p className="who">{user.email}</p>
            {projects.length > 0 ? (
              <label>
                Project
                <select value={projectId ?? ''} onChange={(event) => chooseProject(event.target.value)}>
                  {teams.map((team) => (
                    <optgroup key={team.id} label={team.name}>
                      {projects
                        .filter((project) => project.teamId === team.id)
                        .map((project) => (
                          <option key={project.id} value={project.id}>
                            {project.name}
                          </option>
                        ))}
                    </optgroup>
                  ))}
                </select>
              </label>
            ) : null}
            <nav>
              <NavLink to="/dashboard" className={({ isActive }) => (isActive ? 'active' : undefined)}>
                Dashboard
              </NavLink>
              {projectLinks.map((link) => (
                <NavLink
                  key={link.to}
                  to={link.to}
                  className={({ isActive }) => (isActive ? 'active' : undefined)}
                >
                  {link.label}
                </NavLink>
              ))}
            </nav>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                signOut()
                navigate('/login')
              }}
            >
              Sign out
            </button>
          </>
        ) : (
          <nav>
            <NavLink to="/login" className={({ isActive }) => (isActive ? 'active' : undefined)}>
              {loading ? 'Loading…' : 'Sign in'}
            </NavLink>
          </nav>
        )}
      </aside>
      <main>
        {user && projectId ? <ProjectLive projectId={projectId} /> : null}
        {user && projectId ? <AlertNotices projectId={projectId} /> : null}
        <Outlet />
      </main>
    </div>
  )
}

function ProjectLive({ projectId }: { projectId: string }) {
  useProjectStream(projectId)
  return null
}

function AlertNotices({ projectId }: { projectId: string }) {
  const queryClient = useQueryClient()
  const notices = useQuery<AlertNotice[]>({
    queryKey: ['alert-notices', projectId],
    queryFn: async () => [],
    initialData: [],
    staleTime: Infinity,
  })
  if (!notices.data.length) return null
  return (
    <div className="stack">
      {notices.data.map((notice) => (
        <div className={notice.kind === 'alert.triggered' ? 'banner warn' : 'banner'} key={notice.id}>
          <span>{notice.message}</span>
          <button
            type="button"
            className="small secondary"
            onClick={() => {
              queryClient.setQueryData<AlertNotice[]>(['alert-notices', projectId], (old) =>
                (old ?? []).filter((item) => item.id !== notice.id),
              )
            }}
          >
            Dismiss
          </button>
        </div>
      ))}
    </div>
  )
}
