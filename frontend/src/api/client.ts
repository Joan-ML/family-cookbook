import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

export type Health = components['schemas']['Health']
export type Info = components['schemas']['Info']
export type User = components['schemas']['User']
export type Credentials = components['schemas']['Credentials']
export type ApiError = components['schemas']['Error']
export type Recipe = components['schemas']['Recipe']
export type RecipeInput = components['schemas']['RecipeInput']
export type RecipeSummary = components['schemas']['RecipeSummary']
export type RecipeList = components['schemas']['RecipeList']
export type Cookbook = components['schemas']['Cookbook']
export type CookbookSummary = components['schemas']['CookbookSummary']
export type CookbookList = components['schemas']['CookbookList']
export type Role = components['schemas']['Role']
export type CookbookInput = components['schemas']['CookbookInput']
export type Member = components['schemas']['Member']
export type MemberList = components['schemas']['MemberList']
export type MemberInput = components['schemas']['MemberInput']
export type RoleInput = components['schemas']['RoleInput']

// Typed client for the API in api/openapi.yaml. Paths, params and response
// bodies are checked against the generated schema at compile time.
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api`,
  // Look up fetch on every call so MSW's interception in tests is picked up.
  fetch: (request) => globalThis.fetch(request),
})

// Requests that get 401 mean the session has ended (expired, logged out
// elsewhere, password reset). Listeners send the user back to the login page.
// /session and /me answer 401 as part of logging in, so they don't count.
const unauthorizedListeners = new Set<() => void>()

export function onUnauthorized(listener: () => void): () => void {
  unauthorizedListeners.add(listener)
  return () => unauthorizedListeners.delete(listener)
}

api.use({
  onResponse({ request, response }) {
    const path = new URL(request.url).pathname
    if (response.status === 401 && path !== '/api/session' && path !== '/api/me') {
      unauthorizedListeners.forEach((listener) => listener())
    }
  },
})
