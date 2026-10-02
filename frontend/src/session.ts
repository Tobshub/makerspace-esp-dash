import { createContext } from 'react'
import type { Project, Team, User } from './api/client'

export type SessionValue = {
  user: User | null
  loading: boolean
  teams: Team[]
  projects: Project[]
  projectId: string | null
  selectProject: (id: string) => void
  signIn: (token: string, user: User) => void
  signOut: () => void
}

export const SessionContext = createContext<SessionValue | null>(null)
