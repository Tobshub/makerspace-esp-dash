import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, getToken, setToken, type Project, type Team, type User } from '../api/client'
import { SessionContext, type SessionValue } from '../session'

const PROJECT_KEY = 'makerspace.projectId'

export function SessionProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(() => !getToken())
  const [projectId, setProjectId] = useState<string | null>(() => localStorage.getItem(PROJECT_KEY))

  useEffect(() => {
    if (!getToken()) return
    let cancelled = false
    api<{ user: User }>('/api/v1/auth/me')
      .then((body) => {
        if (!cancelled) setUser(body.user)
      })
      .catch(() => {
        setToken(null)
      })
      .finally(() => {
        if (!cancelled) setReady(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const nav = useQuery({
    queryKey: ['nav', user?.id],
    enabled: Boolean(user),
    queryFn: async () => {
      const teamsBody = await api<{ teams: Team[] }>('/api/v1/teams')
      const lists = await Promise.all(
        teamsBody.teams.map((team) =>
          api<{ projects: Project[] }>(`/api/v1/teams/${team.id}/projects`),
        ),
      )
      return {
        teams: teamsBody.teams,
        projects: lists.flatMap((list) => list.projects),
      }
    },
  })

  const projects = useMemo(() => nav.data?.projects ?? [], [nav.data?.projects])

  const selectProject = useCallback((id: string) => {
    setProjectId(id)
    localStorage.setItem(PROJECT_KEY, id)
  }, [])

  const activeProjectId = projects.some((project) => project.id === projectId)
    ? projectId
    : (projects[0]?.id ?? null)

  const signIn = useCallback((token: string, next: User) => {
    setToken(token)
    setUser(next)
    setReady(true)
  }, [])

  const signOut = useCallback(() => {
    if (getToken()) {
      void api('/api/v1/auth/logout', { method: 'POST' }).catch(() => undefined)
    }
    setToken(null)
    setUser(null)
    queryClient.clear()
  }, [queryClient])

  const value = useMemo<SessionValue>(
    () => ({
      user,
      loading: !ready || Boolean(user && nav.isLoading),
      teams: nav.data?.teams ?? [],
      projects,
      projectId: activeProjectId,
      selectProject,
      signIn,
      signOut,
    }),
    [user, ready, nav.isLoading, nav.data?.teams, projects, activeProjectId, selectProject, signIn, signOut],
  )

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}
