import { http, HttpResponse } from 'msw/http'
import type { ApiError, Health, Recipe, RecipeInput, RecipeList } from '../api/client'

// Mock API used by unit tests (src/mocks/node.ts) and by `npm run dev:mock`
// (src/mocks/browser.ts). Response bodies are typed from the OpenAPI schema,
// so mocks stay in line with the contract the backend is tested against.
//
// Recipes live in memory and mimic the backend's behaviour: input is trimmed,
// tags are lowercased and de-duplicated, and a blank name is rejected.

const mockUser = 'dev'

const seed: Recipe[] = [
  {
    id: 'pancakes',
    name: 'Pancakes',
    ingredients: ['200 g flour', '2 eggs', '300 ml milk', '1 pinch of salt'],
    steps: ['Whisk everything into a smooth batter.', 'Fry ladlefuls in a hot, buttered pan.'],
    tags: ['breakfast', 'vegetarian'],
    source: 'Grandma',
    createdAt: '2026-01-01T10:00:00Z',
    createdBy: mockUser,
    updatedAt: '2026-01-01T10:00:00Z',
    updatedBy: mockUser,
  },
  {
    id: 'pa-amb-tomaquet',
    name: 'Pa amb tomàquet',
    ingredients: ['Country bread', 'Ripe tomatoes', 'Olive oil', 'Salt'],
    steps: ['Toast the bread.', 'Rub with halved tomatoes.', 'Drizzle with oil and season.'],
    tags: ['catalan', 'quick', 'vegan'],
    createdAt: '2026-01-02T10:00:00Z',
    createdBy: mockUser,
    updatedAt: '2026-01-02T10:00:00Z',
    updatedBy: mockUser,
  },
]

let recipes = new Map<string, Recipe>()
let nextId = 1

/** Restores the seed data. Called between tests. */
export function resetMockData() {
  recipes = new Map(seed.map((r) => [r.id, structuredClone(r)]))
  nextId = 1
}
resetMockData()

type RecipeParams = { recipeId: string }

const notFound = () => HttpResponse.json<ApiError>({ message: 'recipe not found' }, { status: 404 })

function normalize(input: RecipeInput): RecipeInput | ApiError {
  const lines = (xs: string[]) => xs.map((x) => x.trim()).filter(Boolean)
  const name = input.name.trim()
  if (!name) return { message: 'name must not be blank' }
  const source = input.source?.trim()
  return {
    name,
    ingredients: lines(input.ingredients),
    steps: lines(input.steps),
    tags: [...new Set(lines(input.tags).map((t) => t.toLowerCase()))],
    ...(source ? { source } : {}),
  }
}

export const handlers = [
  http.get('*/api/health', () => HttpResponse.json<Health>({ status: 'ok' })),

  http.get('*/api/recipes', () => {
    const items = [...recipes.values()]
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      .map(({ id, name, tags }) => ({ id, name, tags }))
    return HttpResponse.json<RecipeList>({ items })
  }),

  http.post<never, RecipeInput, Recipe | ApiError>('*/api/recipes', async ({ request }) => {
    const content = normalize(await request.json())
    if ('message' in content) return HttpResponse.json<ApiError>(content, { status: 400 })
    const now = new Date().toISOString()
    const recipe: Recipe = {
      ...content,
      id: `mock-${nextId++}`,
      createdAt: now,
      createdBy: mockUser,
      updatedAt: now,
      updatedBy: mockUser,
    }
    recipes.set(recipe.id, recipe)
    return HttpResponse.json<Recipe>(recipe, {
      status: 201,
      headers: { Location: `/api/recipes/${recipe.id}` },
    })
  }),

  http.get<RecipeParams, never, Recipe | ApiError>('*/api/recipes/:recipeId', ({ params }) => {
    const recipe = recipes.get(params.recipeId)
    return recipe ? HttpResponse.json<Recipe>(recipe) : notFound()
  }),

  http.put<RecipeParams, RecipeInput, Recipe | ApiError>('*/api/recipes/:recipeId', async ({ params, request }) => {
    const existing = recipes.get(params.recipeId)
    if (!existing) return notFound()
    const content = normalize(await request.json())
    if ('message' in content) return HttpResponse.json<ApiError>(content, { status: 400 })
    const { source: _, ...rest } = existing
    const recipe: Recipe = {
      ...rest,
      ...content,
      updatedAt: new Date().toISOString(),
      updatedBy: mockUser,
    }
    recipes.set(recipe.id, recipe)
    return HttpResponse.json<Recipe>(recipe)
  }),

  http.delete<RecipeParams, never, ApiError | null>('*/api/recipes/:recipeId', ({ params }) => {
    if (!recipes.delete(params.recipeId)) return notFound()
    return new HttpResponse(null, { status: 204 })
  }),
]
