import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

export type Health = components['schemas']['Health']
export type Info = components['schemas']['Info']
export type ApiError = components['schemas']['Error']
export type Recipe = components['schemas']['Recipe']
export type RecipeInput = components['schemas']['RecipeInput']
export type RecipeSummary = components['schemas']['RecipeSummary']
export type RecipeList = components['schemas']['RecipeList']

// Typed client for the API in api/openapi.yaml. Paths, params and response
// bodies are checked against the generated schema at compile time.
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api`,
  // Look up fetch on every call so MSW's interception in tests is picked up.
  fetch: (request) => globalThis.fetch(request),
})
