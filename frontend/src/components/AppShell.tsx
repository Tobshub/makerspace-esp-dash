import { NavLink, Outlet } from 'react-router-dom'

const links = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/projects/demo/overview', label: 'Overview' },
  { to: '/projects/demo/devices', label: 'Devices' },
  { to: '/projects/demo/metrics', label: 'Metrics' },
  { to: '/projects/demo/controls', label: 'Controls' },
  { to: '/projects/demo/alerts', label: 'Alerts' },
  { to: '/projects/demo/events', label: 'Events' },
  { to: '/projects/demo/settings', label: 'Project Settings' },
]

export function AppShell() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <NavLink to="/" className="brand">
          Makerspace
        </NavLink>
        <nav>
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              className={({ isActive }) => (isActive ? 'active' : undefined)}
            >
              {link.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main>
        <Outlet />
      </main>
    </div>
  )
}
