import { Navigate, Outlet } from 'react-router-dom'
import { useSession } from '../useSession'

export function RequireAuth() {
  const { user, loading } = useSession()
  if (loading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}
