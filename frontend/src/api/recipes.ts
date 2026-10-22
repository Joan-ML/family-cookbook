import { api, type Info, type Recipe, type RecipeInput, type RecipeSummary } from './client'

/** An API call failed; `message` is the server's explanation when it gave one. */
export class ApiRequestError extends Error {}

export function fail(error: { message?: string } | undefined, fallback: string): never {
  throw new ApiRequestError(error?.message || fallback)
}

export async function getInfo(): Promise<Info> {
  const { data, error } = await api.GET('/info')
  if (!data) fail(error, "Couldn't load build information.")
  return data
}

const inCookbook = (cookbookId: string) => ({ params: { path: { cookbookId } } })
const recipeIn = (cookbookId: string, recipeId: string) => ({ params: { path: { cookbookId, recipeId } } })

export async function listRecipes(cookbookId: string): Promise<RecipeSummary[]> {
  const { data, error } = await api.GET('/cookbooks/{cookbookId}/recipes', inCookbook(cookbookId))
  if (!data) fail(error, "Couldn't load recipes.")
  return data.items
}

/** Returns null when the recipe doesn't exist (or isn't in a cookbook you can see). */
export async function getRecipe(cookbookId: string, id: string): Promise<Recipe | null> {
  const { data, error, response } = await api.GET('/cookbooks/{cookbookId}/recipes/{recipeId}', recipeIn(cookbookId, id))
  if (response.status === 404) return null
  if (!data) fail(error, "Couldn't load the recipe.")
  return data
}

export async function createRecipe(cookbookId: string, input: RecipeInput): Promise<Recipe> {
  const { data, error } = await api.POST('/cookbooks/{cookbookId}/recipes', { ...inCookbook(cookbookId), body: input })
  if (!data) fail(error, "Couldn't save the recipe.")
  return data
}

export async function updateRecipe(cookbookId: string, id: string, input: RecipeInput): Promise<Recipe> {
  const { data, error } = await api.PUT('/cookbooks/{cookbookId}/recipes/{recipeId}', {
    ...recipeIn(cookbookId, id),
    body: input,
  })
  if (!data) fail(error, "Couldn't save the recipe.")
  return data
}

export async function deleteRecipe(cookbookId: string, id: string): Promise<void> {
  const { error, response } = await api.DELETE('/cookbooks/{cookbookId}/recipes/{recipeId}', recipeIn(cookbookId, id))
  if (!response.ok) fail(error, "Couldn't delete the recipe.")
}
