import { screen, waitFor, within } from '@testing-library/react'
import { getResponse } from 'msw'
import { http, HttpResponse } from 'msw/http'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { handlers } from './mocks/handlers'
import { server } from './mocks/node'
import { renderApp } from './test/renderApp'

// These tests run the whole app against the MSW mock API, which starts every
// test with two recipes: "Pancakes" and "Pa amb tomàquet".

afterEach(() => {
  vi.restoreAllMocks()
})

async function recipeNames() {
  const list = await screen.findByRole('list', { name: 'Recipes' })
  return within(list)
    .getAllByRole('link')
    .filter((a) => a.classList.contains('card-link'))
    .map((a) => a.textContent)
}

describe('recipe list', () => {
  test('shows all recipes sorted by name, with their tags', async () => {
    renderApp('/')

    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
    expect(screen.getAllByRole('link', { name: 'vegan' })).toHaveLength(1)
  })

  test('search matches names and tags, ignoring accents and case', async () => {
    const { user } = renderApp('/')
    await screen.findByText('Pancakes')

    await user.type(screen.getByRole('searchbox'), 'TOMAQUET')
    expect(await recipeNames()).toEqual(['Pa amb tomàquet'])

    await user.clear(screen.getByRole('searchbox'))
    await user.type(screen.getByRole('searchbox'), 'breakfast')
    expect(await recipeNames()).toEqual(['Pancakes'])

    await user.clear(screen.getByRole('searchbox'))
    await user.type(screen.getByRole('searchbox'), 'nothing like this')
    expect(await screen.findByText('No recipes match.')).toBeInTheDocument()
  })

  test('filters by tag from the URL, and can clear the filter', async () => {
    const { user } = renderApp('/?tag=quick')

    expect(await recipeNames()).toEqual(['Pa amb tomàquet'])

    await user.click(screen.getByRole('button', { name: 'Show all' }))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
  })

  test('shows an empty state when there are no recipes', async () => {
    server.use(http.get('*/api/recipes', () => HttpResponse.json({ items: [] })))

    renderApp('/')

    expect(await screen.findByText(/No recipes yet/)).toBeInTheDocument()
  })

  test('shows an error with a retry button when loading fails', async () => {
    // Fails once, then falls back to the default mock.
    server.use(
      http.get('*/api/recipes', () => HttpResponse.json({ message: 'Database is down' }, { status: 500 }), {
        once: true,
      }),
    )
    const { user } = renderApp('/')

    expect(await screen.findByRole('alert')).toHaveTextContent('Database is down')

    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
  })
})

describe('recipe page', () => {
  test('shows ingredients, numbered steps, source and who edited it', async () => {
    const { user, location } = renderApp('/')

    await user.click(await screen.findByRole('link', { name: 'Pancakes' }))

    expect(location()).toBe('/recipes/pancakes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Pancakes' })).toBeInTheDocument()
    const ingredients = screen.getByRole('heading', { name: 'Ingredients' }).nextElementSibling!
    expect(within(ingredients as HTMLElement).getAllByRole('listitem')).toHaveLength(4)
    const steps = screen.getByRole('heading', { name: 'Steps' }).nextElementSibling!
    expect(steps.tagName).toBe('OL')
    expect(screen.getByText(/Source:/)).toHaveTextContent('Source: Grandma')
    expect(screen.getByText(/Last edited by dev/)).toBeInTheDocument()
  })

  test('links a tag back to the filtered list', async () => {
    const { user, location } = renderApp('/recipes/pancakes')

    await user.click(await screen.findByRole('link', { name: 'breakfast' }))

    expect(location()).toBe('/?tag=breakfast')
    expect(await recipeNames()).toEqual(['Pancakes'])
  })

  test('shows a web source as a link', async () => {
    server.use(
      http.get('*/api/recipes/pancakes', async ({ request }) => {
        const recipe = await (await getResponse(handlers, request))!.json()
        return HttpResponse.json({ ...recipe, source: 'https://www.example.com/pancakes/' })
      }),
    )

    renderApp('/recipes/pancakes')

    const link = await screen.findByRole('link', { name: 'example.com/pancakes' })
    expect(link).toHaveAttribute('href', 'https://www.example.com/pancakes/')
  })

  test('shows not found for a missing recipe', async () => {
    renderApp('/recipes/missing')

    expect(await screen.findByRole('heading', { name: 'Not found' })).toBeInTheDocument()
  })

  test('deletes a recipe after confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { user, location } = renderApp('/recipes/pancakes')

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(location()).toBe('/'))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet'])
  })

  test('keeps the recipe when deletion is cancelled', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { user, location } = renderApp('/recipes/pancakes')

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    expect(location()).toBe('/recipes/pancakes')
  })
})

describe('recipe form', () => {
  test('creates a recipe and shows it', async () => {
    const { user, location } = renderApp('/')
    await user.click(await screen.findByRole('link', { name: 'New recipe' }))

    await user.type(screen.getByLabelText('Name'), 'Gazpacho')
    await user.type(screen.getByLabelText(/Ingredients/), 'Tomatoes{Enter}Cucumber{Enter}Olive oil')
    await user.type(screen.getByLabelText('Step 1'), 'Blend everything.')
    await user.click(screen.getByRole('button', { name: 'Add step' }))
    await user.type(screen.getByLabelText('Step 2'), 'Chill.')
    await user.type(screen.getByLabelText(/Tags/), 'Vegan, summer,')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Gazpacho' })).toBeInTheDocument()
    expect(location()).toMatch(/^\/recipes\/mock-\d+$/)
    expect(screen.getByRole('link', { name: 'vegan' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'summer' })).toBeInTheDocument()
    expect(screen.getByText('Chill.')).toBeInTheDocument()
    expect(screen.queryByText(/Source:/)).not.toBeInTheDocument()
  })

  test('edits an existing recipe, starting from its current content', async () => {
    const { user } = renderApp('/recipes/pancakes')
    await user.click(await screen.findByRole('link', { name: 'Edit' }))

    const name = await screen.findByLabelText('Name')
    expect(name).toHaveValue('Pancakes')
    expect(screen.getByLabelText(/Tags/)).toHaveValue('breakfast, vegetarian')
    await user.clear(name)
    await user.type(name, 'Fluffy pancakes')
    await user.click(screen.getByRole('button', { name: 'Remove step 2' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Fluffy pancakes' })).toBeInTheDocument()
    const steps = screen.getByRole('heading', { name: 'Steps' }).nextElementSibling as HTMLElement
    expect(within(steps).getAllByRole('listitem')).toHaveLength(1)
  })

  test('shows the server error and keeps the input when saving fails', async () => {
    const { user, location } = renderApp('/new')

    await user.type(await screen.findByLabelText('Name'), '   ')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('name must not be blank')
    expect(location()).toBe('/new')
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  test('cancel goes back without saving', async () => {
    const { user, location } = renderApp('/recipes/pancakes/edit')

    await user.type(await screen.findByLabelText('Name'), ' changed')
    await user.click(screen.getByRole('link', { name: 'Cancel' }))

    expect(location()).toBe('/recipes/pancakes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Pancakes' })).toBeInTheDocument()
  })
})

test('shows not found for an unknown page', async () => {
  renderApp('/nope')

  expect(await screen.findByRole('heading', { name: 'Not found' })).toBeInTheDocument()
})
