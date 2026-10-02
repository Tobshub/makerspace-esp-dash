import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { RequireAuth } from './components/RequireAuth'
import { DashboardPage } from './pages/DashboardPage'
import { DevicePage } from './pages/DevicePage'
import { DeviceWizardPage } from './pages/DeviceWizardPage'
import { DevicesPage } from './pages/DevicesPage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { AlertsPage } from './pages/AlertsPage'
import { ControlsPage } from './pages/ControlsPage'
import { EventsPage } from './pages/EventsPage'
import { MetricsPage } from './pages/MetricsPage'
import { OverviewPage } from './pages/OverviewPage'
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
          <Route path="projects/:projectId/metrics" element={<MetricsPage />} />
          <Route path="projects/:projectId/controls" element={<ControlsPage />} />
          <Route path="projects/:projectId/alerts" element={<AlertsPage />} />
          <Route path="projects/:projectId/events" element={<EventsPage />} />
          <Route path="projects/:projectId/settings" element={<ProjectSettingsPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
