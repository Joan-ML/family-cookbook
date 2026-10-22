import { screen, waitFor, within } from '@testing-library/react'
import { getResponse } from 'msw'
import { http, HttpResponse } from 'msw/http'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { handlers, mockPassword, setMockSession } from './mocks/handlers'
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
    renderApp('/cookbooks/family')

    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
    expect(screen.getAllByRole('link', { name: 'vegan' })).toHaveLength(1)
  })

  test('search matches names and tags, ignoring accents and case', async () => {
    const { user } = renderApp('/cookbooks/family')
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
    const { user } = renderApp('/cookbooks/family?tag=quick')

    expect(await recipeNames()).toEqual(['Pa amb tomàquet'])

    await user.click(screen.getByRole('button', { name: 'Show all' }))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
  })

  test('shows an empty state when there are no recipes', async () => {
    server.use(http.get('*/api/cookbooks/:cookbookId/recipes', () => HttpResponse.json({ items: [] })))

    renderApp('/cookbooks/family')

    expect(await screen.findByText(/No recipes yet/)).toBeInTheDocument()
  })

  test('shows an error with a retry button when loading fails', async () => {
    // Fails once, then falls back to the default mock.
    server.use(
      http.get('*/api/cookbooks/:cookbookId/recipes', () => HttpResponse.json({ message: 'Database is down' }, { status: 500 }), {
        once: true,
      }),
    )
    const { user } = renderApp('/cookbooks/family')

    expect(await screen.findByRole('alert')).toHaveTextContent('Database is down')

    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet', 'Pancakes'])
  })
})

describe('recipe page', () => {
  test('shows ingredients, numbered steps, source and who edited it', async () => {
    const { user, location } = renderApp('/cookbooks/family')

    await user.click(await screen.findByRole('link', { name: 'Pancakes' }))

    expect(location()).toBe('/cookbooks/family/recipes/pancakes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Pancakes' })).toBeInTheDocument()
    const ingredients = screen.getByRole('heading', { name: 'Ingredients' }).nextElementSibling!
    expect(within(ingredients as HTMLElement).getAllByRole('listitem')).toHaveLength(4)
    const steps = screen.getByRole('heading', { name: 'Steps' }).nextElementSibling!
    expect(steps.tagName).toBe('OL')
    expect(screen.getByText(/Source:/)).toHaveTextContent('Source: Grandma')
    expect(screen.getByText(/Last edited by dev/)).toBeInTheDocument()
  })

  test('links a tag back to the filtered list', async () => {
    const { user, location } = renderApp('/cookbooks/family/recipes/pancakes')

    await user.click(await screen.findByRole('link', { name: 'breakfast' }))

    expect(location()).toBe('/cookbooks/family?tag=breakfast')
    expect(await recipeNames()).toEqual(['Pancakes'])
  })

  test('shows a web source as a link', async () => {
    server.use(
      http.get('*/api/cookbooks/family/recipes/pancakes', async ({ request }) => {
        const recipe = await (await getResponse(handlers, request))!.json()
        return HttpResponse.json({ ...recipe, source: 'https://www.example.com/pancakes/' })
      }),
    )

    renderApp('/cookbooks/family/recipes/pancakes')

    const link = await screen.findByRole('link', { name: 'example.com/pancakes' })
    expect(link).toHaveAttribute('href', 'https://www.example.com/pancakes/')
  })

  test('shows not found for a missing recipe', async () => {
    renderApp('/cookbooks/family/recipes/missing')

    expect(await screen.findByRole('heading', { name: 'Not found' })).toBeInTheDocument()
  })

  test('deletes a recipe after confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { user, location } = renderApp('/cookbooks/family/recipes/pancakes')

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(location()).toBe('/cookbooks/family'))
    expect(await recipeNames()).toEqual(['Pa amb tomàquet'])
  })

  test('keeps the recipe when deletion is cancelled', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { user, location } = renderApp('/cookbooks/family/recipes/pancakes')

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    expect(location()).toBe('/cookbooks/family/recipes/pancakes')
  })
})

describe('recipe form', () => {
  test('creates a recipe and shows it', async () => {
    const { user, location } = renderApp('/cookbooks/family')
    await user.click(await screen.findByRole('link', { name: 'New recipe' }))

    await user.type(await screen.findByLabelText('Name'), 'Gazpacho')
    await user.type(screen.getByLabelText(/Ingredients/), 'Tomatoes{Enter}Cucumber{Enter}Olive oil')
    await user.type(screen.getByLabelText('Step 1'), 'Blend everything.')
    await user.click(screen.getByRole('button', { name: 'Add step' }))
    await user.type(screen.getByLabelText('Step 2'), 'Chill.')
    await user.type(screen.getByLabelText(/Tags/), 'Vegan, summer,')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Gazpacho' })).toBeInTheDocument()
    expect(location()).toMatch(/^\/cookbooks\/family\/recipes\/mock-\d+$/)
    expect(screen.getByRole('link', { name: 'vegan' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'summer' })).toBeInTheDocument()
    expect(screen.getByText('Chill.')).toBeInTheDocument()
    expect(screen.queryByText(/Source:/)).not.toBeInTheDocument()
  })

  test('edits an existing recipe, starting from its current content', async () => {
    const { user } = renderApp('/cookbooks/family/recipes/pancakes')
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
    const { user, location } = renderApp('/cookbooks/family/new')

    await user.type(await screen.findByLabelText('Name'), '   ')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('name must not be blank')
    expect(location()).toBe('/cookbooks/family/new')
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  test('pasting several steps splits them into separate steps', async () => {
    const { user } = renderApp('/cookbooks/family/new')
    const step1 = await screen.findByLabelText('Step 1')
    await user.type(step1, 'Before: ')

    await user.click(step1)
    await user.paste('1. Chop the onions.\n2. Fry them.\n3. Season.')

    expect(screen.getByLabelText('Step 1')).toHaveValue('Before: Chop the onions.')
    expect(screen.getByLabelText('Step 2')).toHaveValue('Fry them.')
    expect(screen.getByLabelText('Step 3')).toHaveValue('Season.')
  })

  test('pasting a single step pastes normally', async () => {
    const { user } = renderApp('/cookbooks/family/new')
    const step1 = await screen.findByLabelText('Step 1')

    await user.click(step1)
    await user.paste('Whisk everything\ntogether.')

    expect(step1).toHaveValue('Whisk everything\ntogether.')
    expect(screen.queryByLabelText('Step 2')).not.toBeInTheDocument()
  })

  test('each step has a reorder handle', async () => {
    renderApp('/cookbooks/family/recipes/pancakes/edit')

    expect(await screen.findByRole('button', { name: 'Reorder step 1' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reorder step 2' })).toBeInTheDocument()
  })

  test('sends no empty ingredients, steps or tags, which the API rejects', async () => {
    let body: unknown
    server.use(
      http.post('*/api/cookbooks/:cookbookId/recipes', async ({ request }) => {
        body = await request.clone().json()
        return getResponse(handlers, request)
      }),
    )
    const { user } = renderApp('/cookbooks/family/new')

    await user.type(await screen.findByLabelText('Name'), 'Toast')
    await user.type(screen.getByLabelText(/Tags/), 'quick, , ')
    await user.click(screen.getByRole('button', { name: 'Add step' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await screen.findByRole('heading', { level: 1, name: 'Toast' })
    expect(body).toEqual({ name: 'Toast', ingredients: [], steps: [], tags: ['quick'] })
  })

  test('cancel goes back without saving', async () => {
    const { user, location } = renderApp('/cookbooks/family/recipes/pancakes/edit')

    await user.type(await screen.findByLabelText('Name'), ' changed')
    await user.click(screen.getByRole('link', { name: 'Cancel' }))

    expect(location()).toBe('/cookbooks/family/recipes/pancakes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Pancakes' })).toBeInTheDocument()
  })
})

describe('footer', () => {
  test('shows the release version and commit', async () => {
    renderApp('/')

    const footer = await screen.findByRole('contentinfo')
    await waitFor(() => expect(footer).toHaveTextContent('Family Cookbook · v0.0.0-mock (mock)'))
  })

  test('shows just the commit for an unreleased build', async () => {
    server.use(http.get('*/api/info', () => HttpResponse.json({ commit: 'abc1234' })))

    renderApp('/')

    const footer = await screen.findByRole('contentinfo')
    await waitFor(() => expect(footer).toHaveTextContent(/^Family Cookbook · abc1234$/))
  })

  test('still shows the name when build information is unavailable', async () => {
    server.use(http.get('*/api/info', () => HttpResponse.error()))

    renderApp('/cookbooks/family')

    await screen.findByText('Pancakes') // the page has otherwise loaded
    expect(screen.getByRole('contentinfo')).toHaveTextContent(/^Family Cookbook$/)
  })
})

test('shows not found for an unknown page', async () => {
  renderApp('/nope')

  expect(await screen.findByRole('heading', { name: 'Not found' })).toBeInTheDocument()
})

describe('login', () => {
  async function logInAs(user: ReturnType<typeof renderApp>['user'], username: string, password: string) {
    await user.type(await screen.findByLabelText('Username'), username)
    await user.type(screen.getByLabelText('Password'), password)
    await user.click(screen.getByRole('button', { name: 'Log in' }))
  }

  test('logged out, any address shows the login page, then what was asked for', async () => {
    setMockSession(null)
    const { user, location } = renderApp('/cookbooks/family/recipes/pancakes')

    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
    expect(screen.queryByText(/session ended/)).not.toBeInTheDocument()
    await logInAs(user, 'anna', mockPassword)

    expect(await screen.findByRole('heading', { level: 1, name: 'Pancakes' })).toBeInTheDocument()
    expect(location()).toBe('/cookbooks/family/recipes/pancakes')
    expect(screen.getByRole('banner')).toHaveTextContent('anna · Log out')
  })

  test('a wrong password shows the server message and keeps the form', async () => {
    setMockSession(null)
    const { user } = renderApp('/')

    await logInAs(user, 'anna', 'wrong password')

    expect(await screen.findByRole('alert')).toHaveTextContent('wrong username or password')
    expect(screen.getByLabelText('Username')).toHaveValue('anna')
    expect(screen.getByRole('button', { name: 'Log in' })).toBeEnabled()
  })

  test('too many attempts are explained', async () => {
    setMockSession(null)
    server.use(
      http.post('*/api/session', () =>
        HttpResponse.json({ message: 'too many' }, { status: 429, headers: { 'Retry-After': '600' } }),
      ),
    )
    const { user } = renderApp('/')

    await logInAs(user, 'anna', mockPassword)

    expect(await screen.findByRole('alert')).toHaveTextContent('Too many failed attempts')
  })

  test('logging out returns to the login page', async () => {
    const { user } = renderApp('/cookbooks/family')
    await screen.findByText('Pancakes')

    await user.click(within(screen.getByRole('banner')).getByRole('button', { name: 'Log out' }))

    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
    expect(screen.queryByText('Pancakes')).not.toBeInTheDocument()
  })

  test('when the session ends while using the app, it asks to log in again', async () => {
    const { user } = renderApp('/cookbooks/family')
    await screen.findByText('Pancakes')

    setMockSession(null) // e.g. the password was reset from the command line
    await user.click(screen.getByRole('link', { name: 'Pancakes' }))

    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
    expect(screen.getByText(/Your session ended/)).toBeInTheDocument()
  })
})

describe('cookbooks', () => {
  test('home lists your cookbooks with recipe counts and roles, and opens them', async () => {
    const { user, location } = renderApp('/')

    const list = await screen.findByRole('list', { name: 'Your cookbooks' })
    expect(within(list).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'Family2 recipes · Admin',
      'Friends1 recipe · Editor',
    ])

    await user.click(within(list).getByRole('link', { name: 'Friends' }))
    expect(location()).toBe('/cookbooks/friends')
    expect(await screen.findByRole('heading', { level: 1, name: 'Friends' })).toBeInTheDocument()
    expect(await recipeNames()).toEqual(['Escalivada'])
  })

  test('creating a cookbook opens it, empty', async () => {
    const { user, location } = renderApp('/')

    await user.type(await screen.findByLabelText('New cookbook name'), 'Weeknight dinners')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Weeknight dinners' })).toBeInTheDocument()
    expect(location()).toMatch(/^\/cookbooks\/mock-\d+$/)
    expect(screen.getByText(/No recipes yet/)).toBeInTheDocument()
  })

  test('only admins see the settings link', async () => {
    renderApp('/cookbooks/family')
    expect(await screen.findByRole('link', { name: 'Cookbook settings' })).toBeInTheDocument()
  })

  test('editors see no settings link', async () => {
    renderApp('/cookbooks/friends')
    await screen.findByRole('heading', { level: 1, name: 'Friends' })
    expect(screen.queryByRole('link', { name: 'Cookbook settings' })).not.toBeInTheDocument()
  })

  test('for editors, the settings page explains that only admins can change settings', async () => {
    renderApp('/cookbooks/friends/settings')

    expect(await screen.findByText("Only the cookbook's admins can change its settings.")).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete cookbook' })).not.toBeInTheDocument()
  })

  test('admins rename a cookbook', async () => {
    const { user } = renderApp('/cookbooks/family/settings')

    const name = await screen.findByLabelText('Name')
    await user.clear(name)
    await user.type(name, 'Home cooking')
    await user.click(screen.getByRole('button', { name: 'Rename' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Renamed.')
    // Everything on the page uses the new name, including the delete confirmation.
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    await user.click(screen.getByRole('button', { name: 'Delete cookbook' }))
    expect(confirm).toHaveBeenCalledWith(`Delete "Home cooking" and its 2 recipes? This can't be undone.`)
    await user.click(screen.getByRole('link', { name: '← Home cooking' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Home cooking' })).toBeInTheDocument()
  })

  test('deleting a cookbook says how many recipes go with it', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { user, location } = renderApp('/cookbooks/family/settings')

    await user.click(await screen.findByRole('button', { name: 'Delete cookbook' }))

    expect(confirm).toHaveBeenCalledWith(`Delete "Family" and its 2 recipes? This can't be undone.`)
    await waitFor(() => expect(location()).toBe('/'))
    const list = await screen.findByRole('list', { name: 'Your cookbooks' })
    expect(within(list).queryByRole('link', { name: 'Family' })).not.toBeInTheDocument()
  })

  test('an unknown cookbook, or one you are not in, shows not found', async () => {
    renderApp('/cookbooks/someone-elses')

    expect(await screen.findByRole('heading', { name: 'Not found' })).toBeInTheDocument()
    expect(screen.getByText(/This cookbook doesn't exist/)).toBeInTheDocument()
  })
})
