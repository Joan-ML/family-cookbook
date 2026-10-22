import { createContext, useContext } from 'react'
import type { User } from './api/client'

export type AuthState =
  | { status: 'checking' }
  // `ended` is true when a session ended while the app was open.
  | { status: 'anonymous'; ended: boolean }
  | { status: 'user'; user: User }

export type Auth = {
  state: AuthState
  logIn: (username: string, password: string) => Promise<void>
  logOut: () => Promise<void>
}

export const AuthContext = createContext<Auth | null>(null)

export function useAuth(): Auth {
  const auth = useContext(AuthContext)
  if (!auth) throw new Error('useAuth must be used inside <AuthProvider>')
  return auth
}
