import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { RequireAuth } from './components/RequireAuth'
import { DashboardPage } from './pages/DashboardPage'
import { DevicePage } from './pages/DevicePage'
import { DeviceWizardPage } from './pages/DeviceWizardPage'
import { DevicesPage } from './pages/DevicesPage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { OverviewPage } from './pages/OverviewPage'
import { PlaceholderPage } from './pages/PlaceholderPage'
import { ProjectSettingsPage } from './pages/ProjectSettingsPage'
import { TeamPage } from './pages/TeamPage'
import { TeamProjectsPage } from './pages/TeamProjectsPage'

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />
        <Route path="login" element={<LoginPage />} />
        <Route element={<RequireAuth />}>
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="teams/:teamId" element={<TeamPage />} />
          <Route path="teams/:teamId/projects" element={<TeamProjectsPage />} />
          <Route path="projects/:projectId" element={<Navigate to="overview" replace />} />
          <Route path="projects/:projectId/overview" element={<OverviewPage />} />
          <Route path="projects/:projectId/devices" element={<DevicesPage />} />
          <Route path="projects/:projectId/devices/new" element={<DeviceWizardPage />} />
          <Route path="projects/:projectId/devices/:deviceId" element={<DevicePage />} />
          <Route path="projects/:projectId/metrics" element={<PlaceholderPage title="Metrics" phase="Phase 9" />} />
          <Route path="projects/:projectId/controls" element={<PlaceholderPage title="Controls" phase="Phase 10" />} />
          <Route path="projects/:projectId/alerts" element={<PlaceholderPage title="Alerts" phase="Phase 12" />} />
          <Route path="projects/:projectId/events" element={<PlaceholderPage title="Events" phase="Phase 11" />} />
          <Route path="projects/:projectId/settings" element={<ProjectSettingsPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
