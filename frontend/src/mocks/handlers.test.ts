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

test('lists the seed recipes sorted by name', async () => {
  const { data } = await api.GET('/recipes')

  expect(data?.items.map((r) => r.name)).toEqual(['Pa amb tomàquet', 'Pancakes'])
})

test('creates, reads, updates and deletes a recipe', async () => {
  const created = await api.POST('/recipes', { body: soup })
  expect(created.response.status).toBe(201)
  expect(created.data).toMatchObject({
    name: 'Soup',
    ingredients: ['water', 'salt'],
    tags: ['vegan', 'quick'],
    createdBy: 'dev',
  })
  const id = created.data!.id
  expect(created.response.headers.get('Location')).toBe(`/api/recipes/${id}`)

  const fetched = await api.GET('/recipes/{recipeId}', { params: { path: { recipeId: id } } })
  expect(fetched.data).toEqual(created.data)

  const updated = await api.PUT('/recipes/{recipeId}', {
    params: { path: { recipeId: id } },
    body: { ...soup, name: 'Hot soup', source: 'Me' },
  })
  expect(updated.data).toMatchObject({ id, name: 'Hot soup', source: 'Me' })

  const deleted = await api.DELETE('/recipes/{recipeId}', { params: { path: { recipeId: id } } })
  expect(deleted.response.status).toBe(204)

  const gone = await api.GET('/recipes/{recipeId}', { params: { path: { recipeId: id } } })
  expect(gone.response.status).toBe(404)
  expect(gone.error?.message).toBeTruthy()
})

test('rejects a blank name', async () => {
  const { response, error } = await api.POST('/recipes', { body: { ...soup, name: '   ' } })

  expect(response.status).toBe(400)
  expect(error?.message).toBe('name must not be blank')
})

test('starts every test from the seed data', async () => {
  await api.DELETE('/recipes/{recipeId}', { params: { path: { recipeId: 'pancakes' } } })
  const { data } = await api.GET('/recipes')
  expect(data?.items).toHaveLength(1)
})

test('...including after another test changed it', async () => {
  const { data } = await api.GET('/recipes')
  expect(data?.items).toHaveLength(2)
})
