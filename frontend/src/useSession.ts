import { useContext } from 'react'
import { SessionContext } from './session'

export function useSession() {
  const value = useContext(SessionContext)
  if (!value) throw new Error('Session missing')
  return value
}
