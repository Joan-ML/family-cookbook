import { useEffect, useState, type ReactNode } from 'react'
import { onUnauthorized } from './api/client'
import * as session from './api/session'
import { AuthContext, type AuthState } from './auth'

/** Knows who's logged in, and offers logging in and out. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'checking' })

  useEffect(() => {
    let current = true
    session
      .getMe()
      .then((user) => current && setState(user ? { status: 'user', user } : { status: 'anonymous', ended: false }))
      // If the server can't be reached, show the login page, which reports it.
      .catch(() => current && setState({ status: 'anonymous', ended: false }))
    const stop = onUnauthorized(() => setState({ status: 'anonymous', ended: true }))
    return () => {
      current = false
      stop()
    }
  }, [])

  const auth = {
    state,
    logIn: async (username: string, password: string) => {
      const user = await session.logIn({ username, password })
      setState({ status: 'user', user })
    },
    logOut: async () => {
      await session.logOut()
      setState({ status: 'anonymous', ended: false })
    },
    setUpPassword: async (token: string, password: string) => {
      const user = await session.setUpPassword(token, password)
      setState({ status: 'user', user })
    },
  }
  return <AuthContext value={auth}>{children}</AuthContext>
}
