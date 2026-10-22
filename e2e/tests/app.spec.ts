import { expect, test } from './fixtures'

test('the footer shows which build is running', async ({ page, request }) => {
  const info = (await (await request.get('/api/info')).json()) as { version?: string; commit: string }
  const label = info.version ? `${info.version} (${info.commit})` : info.commit

  await page.goto('/')

  await expect(page.getByRole('contentinfo')).toHaveText(`Family Cookbook · ${label}`)
})

test('an unknown address shows not found', async ({ page }) => {
  await page.goto('/no/such/page')

  await expect(page.getByRole('heading', { name: 'Not found' })).toBeVisible()
})

// Pages must fit the phone's width: sideways scrolling is the most common way
// a mobile layout breaks.
for (const [name, path] of [
  ['home', () => '/'],
  ['a cookbook', (c: { path: string }) => c.path],
  ['the new recipe form', (c: { path: string }) => `${c.path}/new`],
  ['cookbook settings', (c: { path: string }) => `${c.path}/settings`],
] as const) {
  test(`${name} fits the screen width`, async ({ page, cookbook }) => {
    await page.goto(path(cookbook))
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    )
    expect(overflow).toBe(0)
  })
}

test('a recipe page with long content fits the screen width', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({
    name: `A recipe with a rather long name that has to wrap nicely ${unique}`,
    ingredients: ['https://example.com/a/very/long/link/without/spaces/that/could/push/the/layout/sideways'],
    steps: ['Supercalifragilisticexpialidocious-and-then-some-more-without-any-spaces-at-all'],
    tags: ['vegetarian', 'quick', 'summer', 'catalan', 'grandma', 'weekend'],
    source: 'https://www.example.com/recipes/a-very-long-path/that-goes-on-and-on/',
  })

  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible()

  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  )
  expect(overflow).toBe(0)
})
