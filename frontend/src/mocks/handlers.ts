import { http, HttpResponse } from 'msw/http'
import type {
  ApiError,
  Cookbook,
  CookbookInput,
  CookbookList,
  Credentials,
  Health,
  Info,
  Member,
  MemberInput,
  MemberList,
  Recipe,
  RecipeInput,
  RecipeList,
  RoleInput,
  User,
} from '../api/client'

// Mock API used by unit tests (src/mocks/node.ts) and by `npm run dev:mock`
// (src/mocks/browser.ts). Response bodies are typed from the OpenAPI schema,
// so mocks stay in line with the contract the backend is tested against.
//
// Recipes live in memory and mimic the backend's behaviour: input is trimmed,
// tags are lowercased and de-duplicated, and a blank name is rejected.
//
// Sessions: the mock starts logged in as "dev". Any username logs in with
// mockPassword; everything but the public endpoints answers 401 when logged out.
//
// Cookbooks: whoever is logged in ("dev" by default) is admin of "Family"
// (with anna as editor) and an editor of "Friends" (run by anna). Unknown
// cookbooks are 404, recipes are only found in their own cookbook, editors get
// 403 for admin actions, and no change may leave a cookbook without an admin,
// as on the server. Accounts that exist: dev, anna, bob, carla.

const mockUser = 'dev'
export const mockPassword = 'cookbook password'
let sessionUser: string | null = mockUser

/** Logs the mock API in as `user`, or out with null. */
export function setMockSession(user: string | null) {
  sessionUser = user
}

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
  {
    id: 'escalivada',
    name: 'Escalivada',
    ingredients: ['2 aubergines', '2 red peppers', '1 onion', 'Olive oil'],
    steps: ['Roast the vegetables until soft.', 'Peel, slice and dress with oil.'],
    tags: ['catalan', 'vegan'],
    createdAt: '2026-01-03T10:00:00Z',
    createdBy: 'anna',
    updatedAt: '2026-01-03T10:00:00Z',
    updatedBy: 'anna',
  },
]

const seedCookbooks: Cookbook[] = [
  { id: 'family', name: 'Family', role: 'admin', createdAt: '2026-01-01T09:00:00Z', createdBy: mockUser },
  { id: 'friends', name: 'Friends', role: 'editor', createdAt: '2026-01-01T09:00:00Z', createdBy: 'anna' },
]
// Which cookbook each seed recipe is in.
const seedPlacement: Record<string, string> = { pancakes: 'family', 'pa-amb-tomaquet': 'family', escalivada: 'friends' }

const accounts = ['anna', 'bob', 'carla', mockUser]
const joined = '2026-01-01T09:00:00Z'

let cookbooks = new Map<string, Cookbook>()
// Members by cookbook. The mock assumes the logged-in user is "dev".
let members = new Map<string, Member[]>()
let recipes = new Map<string, { cookbookId: string; recipe: Recipe }>()
let nextId = 1

/** Restores the seed data. Called between tests. */
export function resetMockData() {
  cookbooks = new Map(seedCookbooks.map((c) => [c.id, structuredClone(c)]))
  members = new Map([
    [
      'family',
      [
        { username: 'anna', role: 'editor', joinedAt: joined },
        { username: mockUser, role: 'admin', joinedAt: joined },
      ],
    ],
    [
      'friends',
      [
        { username: 'anna', role: 'admin', joinedAt: joined },
        { username: mockUser, role: 'editor', joinedAt: joined },
      ],
    ],
  ])
  recipes = new Map(seed.map((r) => [r.id, { cookbookId: seedPlacement[r.id], recipe: structuredClone(r) }]))
  nextId = 1
  sessionUser = mockUser
}
resetMockData()

type CookbookParams = { cookbookId: string }
type RecipeParams = { cookbookId: string; recipeId: string }

const cookbookNotFound = () => HttpResponse.json<ApiError>({ message: 'cookbook not found' }, { status: 404 })
const recipeNotFound = () => HttpResponse.json<ApiError>({ message: 'recipe not found' }, { status: 404 })
const adminsOnly = () =>
  HttpResponse.json<ApiError>({ message: "only the cookbook's admins can do this" }, { status: 403 })

type MemberParams = { cookbookId: string; username: string }

const conflict = (message: string) => HttpResponse.json<ApiError>({ message }, { status: 409 })
const lastAdmin = 'a cookbook needs at least one admin: make someone else admin first'
const otherAdmins = (cookbookId: string, username: string) =>
  members.get(cookbookId)!.filter((m) => m.role === 'admin' && m.username !== username).length

const inCookbook = (cookbookId: string) =>
  [...recipes.values()].filter((r) => r.cookbookId === cookbookId).map((r) => r.recipe)

/** The recipe, if it's in the cookbook named in the URL. */
function findRecipe({ cookbookId, recipeId }: RecipeParams): Recipe | undefined {
  const entry = recipes.get(recipeId)
  return entry?.cookbookId === cookbookId ? entry.recipe : undefined
}

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

const publicPaths = ['/api/health', '/api/info', '/api/session']

export const handlers = [
  // Like the real server: endpoints need a session unless they're public.
  // Returning nothing passes the request on to the handlers below.
  http.all('*/api/*', ({ request }) => {
    if (sessionUser || publicPaths.includes(new URL(request.url).pathname)) return
    return HttpResponse.json<ApiError>({ message: 'not logged in' }, { status: 401 })
  }),

  http.post<never, Credentials, User | ApiError>('*/api/session', async ({ request }) => {
    const { username, password } = await request.json()
    if (password !== mockPassword) {
      return HttpResponse.json<ApiError>({ message: 'wrong username or password' }, { status: 401 })
    }
    sessionUser = username.trim().toLowerCase()
    return HttpResponse.json<User>({ username: sessionUser })
  }),
  http.delete('*/api/session', () => {
    sessionUser = null
    return new HttpResponse(null, { status: 204 })
  }),
  http.get('*/api/me', () => HttpResponse.json<User>({ username: sessionUser! })),

  http.get('*/api/health', () => HttpResponse.json<Health>({ status: 'ok' })),
  http.get('*/api/info', () => HttpResponse.json<Info>({ version: 'v0.0.0-mock', commit: 'mock' })),

  http.get('*/api/cookbooks', () => {
    const items = [...cookbooks.values()]
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      .map(({ id, name, role }) => ({ id, name, role, recipeCount: inCookbook(id).length }))
    return HttpResponse.json<CookbookList>({ items })
  }),

  http.post<never, CookbookInput, Cookbook | ApiError>('*/api/cookbooks', async ({ request }) => {
    const name = (await request.json()).name.trim()
    if (!name) return HttpResponse.json<ApiError>({ message: 'name must not be blank' }, { status: 400 })
    const cookbook: Cookbook = {
      id: `mock-${nextId++}`,
      name,
      role: 'admin',
      createdAt: new Date().toISOString(),
      createdBy: sessionUser!,
    }
    cookbooks.set(cookbook.id, cookbook)
    members.set(cookbook.id, [{ username: sessionUser!, role: 'admin', joinedAt: cookbook.createdAt }])
    return HttpResponse.json<Cookbook>(cookbook, {
      status: 201,
      headers: { Location: `/api/cookbooks/${cookbook.id}` },
    })
  }),

  http.get<CookbookParams, never, Cookbook | ApiError>('*/api/cookbooks/:cookbookId', ({ params }) => {
    const cookbook = cookbooks.get(params.cookbookId)
    return cookbook ? HttpResponse.json<Cookbook>(cookbook) : cookbookNotFound()
  }),

  http.put<CookbookParams, CookbookInput, Cookbook | ApiError>('*/api/cookbooks/:cookbookId', async ({ params, request }) => {
    const cookbook = cookbooks.get(params.cookbookId)
    if (!cookbook) return cookbookNotFound()
    if (cookbook.role !== 'admin') return adminsOnly()
    const name = (await request.json()).name.trim()
    if (!name) return HttpResponse.json<ApiError>({ message: 'name must not be blank' }, { status: 400 })
    cookbook.name = name
    return HttpResponse.json<Cookbook>(cookbook)
  }),

  http.delete<CookbookParams, never, ApiError | null>('*/api/cookbooks/:cookbookId', ({ params }) => {
    const cookbook = cookbooks.get(params.cookbookId)
    if (!cookbook) return cookbookNotFound()
    if (cookbook.role !== 'admin') return adminsOnly()
    cookbooks.delete(cookbook.id)
    for (const r of inCookbook(cookbook.id)) recipes.delete(r.id)
    return new HttpResponse(null, { status: 204 })
  }),

  http.get<CookbookParams, never, MemberList | ApiError>('*/api/cookbooks/:cookbookId/members', ({ params }) => {
    if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
    return HttpResponse.json<MemberList>({ items: members.get(params.cookbookId)! })
  }),

  http.post<CookbookParams, MemberInput, Member | ApiError>(
    '*/api/cookbooks/:cookbookId/members',
    async ({ params, request }) => {
      const cookbook = cookbooks.get(params.cookbookId)
      if (!cookbook) return cookbookNotFound()
      if (cookbook.role !== 'admin') return adminsOnly()
      const { username, role } = await request.json()
      const name = username.trim().toLowerCase()
      if (!accounts.includes(name)) {
        return HttpResponse.json<ApiError>({ message: `no account named "${username}"` }, { status: 400 })
      }
      const list = members.get(cookbook.id)!
      if (list.some((m) => m.username === name)) return conflict(`${name} is already a member`)
      const member: Member = { username: name, role, joinedAt: new Date().toISOString() }
      members.set(cookbook.id, [...list, member].sort((a, b) => a.username.localeCompare(b.username)))
      return HttpResponse.json<Member>(member, { status: 201 })
    },
  ),

  http.put<MemberParams, RoleInput, Member | ApiError>(
    '*/api/cookbooks/:cookbookId/members/:username',
    async ({ params, request }) => {
      const cookbook = cookbooks.get(params.cookbookId)
      if (!cookbook) return cookbookNotFound()
      if (cookbook.role !== 'admin') return adminsOnly()
      const member = members.get(cookbook.id)!.find((m) => m.username === params.username)
      if (!member) return HttpResponse.json<ApiError>({ message: 'member not found' }, { status: 404 })
      const { role } = await request.json()
      if (member.role === 'admin' && role !== 'admin' && otherAdmins(cookbook.id, member.username) === 0) {
        return conflict(lastAdmin)
      }
      member.role = role
      if (member.username === sessionUser) cookbook.role = role
      return HttpResponse.json<Member>(member)
    },
  ),

  http.delete<MemberParams, never, ApiError | null>('*/api/cookbooks/:cookbookId/members/:username', ({ params }) => {
    const cookbook = cookbooks.get(params.cookbookId)
    if (!cookbook) return cookbookNotFound()
    const leaving = params.username === sessionUser
    if (!leaving && cookbook.role !== 'admin') return adminsOnly()
    const list = members.get(cookbook.id)!
    const member = list.find((m) => m.username === params.username)
    if (!member) return HttpResponse.json<ApiError>({ message: 'member not found' }, { status: 404 })
    if (member.role === 'admin' && otherAdmins(cookbook.id, member.username) === 0) {
      return conflict(
        leaving
          ? "you're this cookbook's only admin: make someone else admin before leaving, or delete the cookbook"
          : lastAdmin,
      )
    }
    members.set(cookbook.id, list.filter((m) => m !== member))
    // Having left, the cookbook no longer exists for this user.
    if (leaving) cookbooks.delete(cookbook.id)
    return new HttpResponse(null, { status: 204 })
  }),

  http.get<CookbookParams, never, RecipeList | ApiError>('*/api/cookbooks/:cookbookId/recipes', ({ params }) => {
    if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
    const items = inCookbook(params.cookbookId)
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      .map(({ id, name, tags }) => ({ id, name, tags }))
    return HttpResponse.json<RecipeList>({ items })
  }),

  http.post<CookbookParams, RecipeInput, Recipe | ApiError>(
    '*/api/cookbooks/:cookbookId/recipes',
    async ({ params, request }) => {
      if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
      const content = normalize(await request.json())
      if ('message' in content) return HttpResponse.json<ApiError>(content, { status: 400 })
      const now = new Date().toISOString()
      const recipe: Recipe = {
        ...content,
        id: `mock-${nextId++}`,
        createdAt: now,
        createdBy: sessionUser!,
        updatedAt: now,
        updatedBy: sessionUser!,
      }
      recipes.set(recipe.id, { cookbookId: params.cookbookId, recipe })
      return HttpResponse.json<Recipe>(recipe, {
        status: 201,
        headers: { Location: `/api/cookbooks/${params.cookbookId}/recipes/${recipe.id}` },
      })
    },
  ),

  http.get<RecipeParams, never, Recipe | ApiError>('*/api/cookbooks/:cookbookId/recipes/:recipeId', ({ params }) => {
    if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
    const recipe = findRecipe(params)
    return recipe ? HttpResponse.json<Recipe>(recipe) : recipeNotFound()
  }),

  http.put<RecipeParams, RecipeInput, Recipe | ApiError>(
    '*/api/cookbooks/:cookbookId/recipes/:recipeId',
    async ({ params, request }) => {
      if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
      const existing = findRecipe(params)
      if (!existing) return recipeNotFound()
      const content = normalize(await request.json())
      if ('message' in content) return HttpResponse.json<ApiError>(content, { status: 400 })
      const { source: _, ...rest } = existing
      const recipe: Recipe = {
        ...rest,
        ...content,
        updatedAt: new Date().toISOString(),
        updatedBy: sessionUser!,
      }
      recipes.set(recipe.id, { cookbookId: params.cookbookId, recipe })
      return HttpResponse.json<Recipe>(recipe)
    },
  ),

  http.delete<RecipeParams, never, ApiError | null>('*/api/cookbooks/:cookbookId/recipes/:recipeId', ({ params }) => {
    if (!cookbooks.has(params.cookbookId)) return cookbookNotFound()
    if (!findRecipe(params)) return recipeNotFound()
    recipes.delete(params.recipeId)
    return new HttpResponse(null, { status: 204 })
  }),
]
