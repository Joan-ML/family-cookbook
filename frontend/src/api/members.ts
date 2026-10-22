import { api, type Member, type Role } from './client'
import { fail } from './recipes'

const inCookbook = (cookbookId: string) => ({ params: { path: { cookbookId } } })
const member = (cookbookId: string, username: string) => ({ params: { path: { cookbookId, username } } })

export async function listMembers(cookbookId: string): Promise<Member[]> {
  const { data, error } = await api.GET('/cookbooks/{cookbookId}/members', inCookbook(cookbookId))
  if (!data) fail(error, "Couldn't load the members.")
  return data.items
}

export async function addMember(cookbookId: string, username: string, role: Role): Promise<Member> {
  const { data, error } = await api.POST('/cookbooks/{cookbookId}/members', {
    ...inCookbook(cookbookId),
    body: { username, role },
  })
  if (!data) fail(error, "Couldn't add the member.")
  return data
}

export async function changeRole(cookbookId: string, username: string, role: Role): Promise<Member> {
  const { data, error } = await api.PUT('/cookbooks/{cookbookId}/members/{username}', {
    ...member(cookbookId, username),
    body: { role },
  })
  if (!data) fail(error, "Couldn't change the role.")
  return data
}

/** Removes a member; removing yourself is leaving the cookbook. */
export async function removeMember(cookbookId: string, username: string): Promise<void> {
  const { error, response } = await api.DELETE('/cookbooks/{cookbookId}/members/{username}', member(cookbookId, username))
  if (!response.ok) fail(error, "Couldn't remove the member.")
}
