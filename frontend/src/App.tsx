import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { HomePage } from './pages/HomePage'
import { PlaceholderPage } from './pages/PlaceholderPage'

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />
        <Route path="login" element={<PlaceholderPage title="Sign in" phase="Phase 2" />} />
        <Route path="dashboard" element={<PlaceholderPage title="Dashboard" phase="Phase 2" />} />
        <Route path="teams/:teamId" element={<PlaceholderPage title="Team" phase="Phase 2" />} />
        <Route path="teams/:teamId/projects" element={<PlaceholderPage title="Projects" phase="Phase 2" />} />
        <Route path="projects/:projectId" element={<Navigate to="overview" replace />} />
        <Route path="projects/:projectId/overview" element={<PlaceholderPage title="Overview" phase="Phase 8" />} />
        <Route path="projects/:projectId/devices" element={<PlaceholderPage title="Devices" phase="Phase 3" />} />
        <Route path="projects/:projectId/devices/:deviceId" element={<PlaceholderPage title="Device" phase="Phase 3" />} />
        <Route path="projects/:projectId/metrics" element={<PlaceholderPage title="Metrics" phase="Phase 9" />} />
        <Route path="projects/:projectId/controls" element={<PlaceholderPage title="Controls" phase="Phase 10" />} />
        <Route path="projects/:projectId/alerts" element={<PlaceholderPage title="Alerts" phase="Phase 12" />} />
        <Route path="projects/:projectId/events" element={<PlaceholderPage title="Events" phase="Phase 11" />} />
        <Route path="projects/:projectId/settings" element={<PlaceholderPage title="Project settings" phase="Phase 2" />} />
      </Route>
    </Routes>
  )
}
