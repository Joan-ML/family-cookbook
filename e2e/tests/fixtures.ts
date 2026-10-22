import { test as base, expect, type APIRequestContext } from '@playwright/test'

export { expect }

type RecipeInput = {
  name: string
  ingredients: string[]
  steps: string[]
  tags: string[]
  source?: string
}

type Recipe = RecipeInput & { id: string }

type Fixtures = {
  /**
   * API client logged in as the test account, for setting up and cleaning up
   * data, even in tests whose page starts logged out.
   */
  api: APIRequestContext
  /** A suffix unique to this test, to keep its data apart from other tests'. */
  unique: string
  /** Creates a recipe through the API; it's deleted when the test ends. */
  createRecipe: (input: Partial<RecipeInput> & { name: string }) => Promise<Recipe>
  /** Deletes, at the end of the test, recipes the test created through the UI. */
  cleanUpByName: (name: string) => void
}

async function deleteByName(request: APIRequestContext, name: string) {
  const res = await request.get('/api/recipes')
  const { items } = (await res.json()) as { items: { id: string; name: string }[] }
  for (const r of items.filter((r) => r.name === name)) {
    await request.delete(`/api/recipes/${r.id}`)
  }
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

  createRecipe: async ({ api: request }, use) => {
    const created: string[] = []
    await use(async (input) => {
      const res = await request.post('/api/recipes', {
        data: { ingredients: [], steps: [], tags: [], ...input },
      })
      expect(res.status()).toBe(201)
      const recipe = (await res.json()) as Recipe
      created.push(recipe.id)
      return recipe
    })
    for (const id of created) await request.delete(`/api/recipes/${id}`)
  },

  cleanUpByName: async ({ api: request }, use) => {
    const names: string[] = []
    await use((name) => names.push(name))
    for (const name of names) await deleteByName(request, name)
  },
})
