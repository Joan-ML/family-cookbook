import { test as base, expect, type APIRequestContext, type Page } from '@playwright/test'

export { expect }

type RecipeInput = {
  name: string
  ingredients: string[]
  steps: string[]
  tags: string[]
  source?: string
}

type Recipe = RecipeInput & { id: string }

type Cookbook = {
  id: string
  name: string
  /** The cookbook's page in the app, e.g. "/cookbooks/abc". */
  path: string
}

type Fixtures = {
  /**
   * API client logged in as the test account, for setting up and cleaning up
   * data, even in tests whose page starts logged out.
   */
  api: APIRequestContext
  /** A suffix unique to this test, to keep its data apart from other tests'. */
  unique: string
  /**
   * A fresh cookbook for this test, which the test account is admin of. It's
   * deleted, with its recipes, when the test ends.
   */
  cookbook: Cookbook
  /** Creates a recipe in the test's cookbook through the API. */
  createRecipe: (input: Partial<RecipeInput> & { name: string }) => Promise<Recipe>
  /** Deletes, at the end of the test, cookbooks the test created through the UI. */
  cleanUpCookbook: (name: string) => void
  /** A page in a separate browser session, logged in as the friend account. */
  friendPage: Page
}

export const test = base.extend<Fixtures>({
  api: async ({ playwright, baseURL }, use) => {
    const api = await playwright.request.newContext({ baseURL, storageState: '.auth/session.json' })
    await use(api)
    await api.dispose()
  },

  unique: async ({}, use, testInfo) => {
    await use(`${testInfo.project.name}-${testInfo.testId.slice(0, 6)}-${Date.now().toString(36)}`)
  },

  cookbook: async ({ api, unique }, use) => {
    const res = await api.post('/api/cookbooks', { data: { name: `Test kitchen ${unique}` } })
    expect(res.status()).toBe(201)
    const { id, name } = (await res.json()) as { id: string; name: string }
    await use({ id, name, path: `/cookbooks/${id}` })
    await api.delete(`/api/cookbooks/${id}`)
  },

  createRecipe: async ({ api, cookbook }, use) => {
    await use(async (input) => {
      const res = await api.post(`/api${cookbook.path}/recipes`, {
        data: { ingredients: [], steps: [], tags: [], ...input },
      })
      expect(res.status()).toBe(201)
      return (await res.json()) as Recipe
    })
  },

  friendPage: async ({ browser }, use, testInfo) => {
    // Same phone as the test's project, but a different person.
    const { viewport, userAgent, deviceScaleFactor, isMobile, hasTouch } = testInfo.project.use
    const context = await browser.newContext({
      viewport,
      userAgent,
      deviceScaleFactor,
      isMobile,
      hasTouch,
      baseURL: testInfo.project.use.baseURL,
      storageState: '.auth/friend.json',
    })
    await use(await context.newPage())
    await context.close()
  },

  cleanUpCookbook: async ({ api }, use) => {
    const names: string[] = []
    await use((name) => names.push(name))
    const res = await api.get('/api/cookbooks')
    const { items } = (await res.json()) as { items: { id: string; name: string }[] }
    for (const c of items.filter((c) => names.includes(c.name))) {
      await api.delete(`/api/cookbooks/${c.id}`)
    }
  },
})
