import { api, type Info, type Recipe, type RecipeInput, type RecipeSummary } from './client'

/** An API call failed; `message` is the server's explanation when it gave one. */
export class ApiRequestError extends Error {}

function fail(error: { message?: string } | undefined, fallback: string): never {
  throw new ApiRequestError(error?.message || fallback)
}

export async function getInfo(): Promise<Info> {
  const { data, error } = await api.GET('/info')
  if (!data) fail(error, "Couldn't load build information.")
  return data
}

export async function listRecipes(): Promise<RecipeSummary[]> {
  const { data, error } = await api.GET('/recipes')
  if (!data) fail(error, "Couldn't load recipes.")
  return data.items
}

/** Returns null when the recipe doesn't exist. */
export async function getRecipe(id: string): Promise<Recipe | null> {
  const { data, error, response } = await api.GET('/recipes/{recipeId}', {
    params: { path: { recipeId: id } },
  })
  if (response.status === 404) return null
  if (!data) fail(error, "Couldn't load the recipe.")
  return data
}

export async function createRecipe(input: RecipeInput): Promise<Recipe> {
  const { data, error } = await api.POST('/recipes', { body: input })
  if (!data) fail(error, "Couldn't save the recipe.")
  return data
}

export async function updateRecipe(id: string, input: RecipeInput): Promise<Recipe> {
  const { data, error } = await api.PUT('/recipes/{recipeId}', {
    params: { path: { recipeId: id } },
    body: input,
  })
  if (!data) fail(error, "Couldn't save the recipe.")
  return data
}

export async function deleteRecipe(id: string): Promise<void> {
  const { error, response } = await api.DELETE('/recipes/{recipeId}', {
    params: { path: { recipeId: id } },
  })
  if (!response.ok) fail(error, "Couldn't delete the recipe.")
}
