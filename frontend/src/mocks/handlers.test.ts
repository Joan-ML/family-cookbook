import { expect, test } from 'vitest'
import { api, type RecipeInput } from '../api/client'

// These tests exercise the mock API through the typed client, so the mocks
// can be relied on when building the UI.

const soup: RecipeInput = {
  name: '  Soup ',
  ingredients: ['water', ' ', 'salt'],
  steps: ['Boil.'],
  tags: ['Vegan', 'vegan ', 'QUICK'],
}

const family = { params: { path: { cookbookId: 'family' } } }
const recipe = (recipeId: string, cookbookId = 'family') => ({ params: { path: { cookbookId, recipeId } } })

test('lists the cookbooks with roles and recipe counts', async () => {
  const { data } = await api.GET('/cookbooks')

  expect(data?.items).toEqual([
    { id: 'family', name: 'Family', role: 'admin', recipeCount: 2 },
    { id: 'friends', name: 'Friends', role: 'editor', recipeCount: 1 },
  ])
})

test("lists a cookbook's recipes sorted by name", async () => {
  const { data } = await api.GET('/cookbooks/{cookbookId}/recipes', family)

  expect(data?.items.map((r) => r.name)).toEqual(['Pa amb tomàquet', 'Pancakes'])
})

test('creates, reads, updates and deletes a recipe', async () => {
  const created = await api.POST('/cookbooks/{cookbookId}/recipes', { ...family, body: soup })
  expect(created.response.status).toBe(201)
  expect(created.data).toMatchObject({
    name: 'Soup',
    ingredients: ['water', 'salt'],
    tags: ['vegan', 'quick'],
    createdBy: 'dev',
  })
  const id = created.data!.id
  expect(created.response.headers.get('Location')).toBe(`/api/cookbooks/family/recipes/${id}`)

  const fetched = await api.GET('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe(id))
  expect(fetched.data).toEqual(created.data)

  const updated = await api.PUT('/cookbooks/{cookbookId}/recipes/{recipeId}', {
    ...recipe(id),
    body: { ...soup, name: 'Hot soup', source: 'Me' },
  })
  expect(updated.data).toMatchObject({ id, name: 'Hot soup', source: 'Me' })

  const deleted = await api.DELETE('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe(id))
  expect(deleted.response.status).toBe(204)

  const gone = await api.GET('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe(id))
  expect(gone.response.status).toBe(404)
  expect(gone.error?.message).toBeTruthy()
})

test('rejects a blank name', async () => {
  const { response, error } = await api.POST('/cookbooks/{cookbookId}/recipes', {
    ...family,
    body: { ...soup, name: '   ' },
  })

  expect(response.status).toBe(400)
  expect(error?.message).toBe('name must not be blank')
})

test('recipes are only found in their own cookbook, and unknown cookbooks are 404', async () => {
  const wrongCookbook = await api.GET('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe('pancakes', 'friends'))
  const unknown = await api.GET('/cookbooks/{cookbookId}', { params: { path: { cookbookId: 'nope' } } })

  expect(wrongCookbook.response.status).toBe(404)
  expect(unknown.response.status).toBe(404)
})

test('editors cannot rename or delete a cookbook; admins can', async () => {
  const friends = { params: { path: { cookbookId: 'friends' } } }

  expect((await api.PUT('/cookbooks/{cookbookId}', { ...friends, body: { name: 'Mine' } })).response.status).toBe(403)
  expect((await api.DELETE('/cookbooks/{cookbookId}', friends)).response.status).toBe(403)

  const renamed = await api.PUT('/cookbooks/{cookbookId}', { ...family, body: { name: ' Home ' } })
  expect(renamed.data?.name).toBe('Home')
  expect((await api.DELETE('/cookbooks/{cookbookId}', family)).response.status).toBe(204)
  expect((await api.GET('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe('pancakes'))).response.status).toBe(404)
})

test('starts every test from the seed data', async () => {
  await api.DELETE('/cookbooks/{cookbookId}/recipes/{recipeId}', recipe('pancakes'))
  const { data } = await api.GET('/cookbooks/{cookbookId}/recipes', family)
  expect(data?.items).toHaveLength(1)
})

test('...including after another test changed it', async () => {
  const { data } = await api.GET('/cookbooks/{cookbookId}/recipes', family)
  expect(data?.items).toHaveLength(2)
})
