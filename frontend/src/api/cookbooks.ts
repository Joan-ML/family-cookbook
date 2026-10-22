import { api, type Cookbook, type CookbookSummary } from './client'
import { fail } from './recipes'

const path = (cookbookId: string) => ({ params: { path: { cookbookId } } })

export async function listCookbooks(): Promise<CookbookSummary[]> {
  const { data, error } = await api.GET('/cookbooks')
  if (!data) fail(error, "Couldn't load your cookbooks.")
  return data.items
}

/** Returns null when the cookbook doesn't exist or you're not a member. */
export async function getCookbook(id: string): Promise<Cookbook | null> {
  const { data, error, response } = await api.GET('/cookbooks/{cookbookId}', path(id))
  if (response.status === 404) return null
  if (!data) fail(error, "Couldn't load the cookbook.")
  return data
}

export async function createCookbook(name: string): Promise<Cookbook> {
  const { data, error } = await api.POST('/cookbooks', { body: { name } })
  if (!data) fail(error, "Couldn't create the cookbook.")
  return data
}

export async function renameCookbook(id: string, name: string): Promise<Cookbook> {
  const { data, error } = await api.PUT('/cookbooks/{cookbookId}', { ...path(id), body: { name } })
  if (!data) fail(error, "Couldn't rename the cookbook.")
  return data
}

export async function deleteCookbook(id: string): Promise<void> {
  const { error, response } = await api.DELETE('/cookbooks/{cookbookId}', path(id))
  if (!response.ok) fail(error, "Couldn't delete the cookbook.")
}
