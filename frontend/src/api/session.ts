import { api, type Credentials, type User } from './client'
import { ApiRequestError } from './recipes'

/** Returns the logged-in user, or null when there's no session. */
export async function getMe(): Promise<User | null> {
  const { data, error, response } = await api.GET('/me')
  if (response.status === 401) return null
  if (!data) throw new ApiRequestError(error?.message || "Couldn't reach the server.")
  return data
}

export async function logIn(credentials: Credentials): Promise<User> {
  const { data, error, response } = await api.POST('/session', { body: credentials })
  if (data) return data
  if (response.status === 429) throw new ApiRequestError('Too many failed attempts. Wait a few minutes and try again.')
  throw new ApiRequestError(error?.message || "Couldn't log in.")
}

export async function logOut(): Promise<void> {
  await api.DELETE('/session')
}
