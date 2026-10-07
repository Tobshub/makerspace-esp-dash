import { useQuery, useQueryClient } from '@tanstack/react-query'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useProjectStream, type AlertNotice } from '../live'
import { useTheme } from '../theme'
import { useSession } from '../useSession'

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5L19 19M19 5l-1.5 1.5M6.5 17.5L5 19" />
    </svg>
  )
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </svg>
  )
}

export function AppShell() {
  const { user, loading, teams, projects, projectId, selectProject, signOut } = useSession()
  const { theme, toggleTheme } = useTheme()
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
        <div className="sidebar__top">
          <NavLink to="/" className="brand" aria-label="SST Makerspace">
            <img src="/logo.png" alt="SST Makerspace" />
          </NavLink>
          <button type="button" className="icon-btn" onClick={toggleTheme} aria-label="Toggle theme">
            {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
          </button>
        </div>
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
