import { expect, test } from './fixtures'

// Core flows through the real UI, server and database, on phone-sized screens.

test('create a recipe and find it again after a reload', async ({ page, unique, cookbook }) => {
  const name = `Gazpacho ${unique}`

  await page.goto(cookbook.path)
  await page.getByRole('link', { name: 'New recipe' }).click()
  await page.getByLabel('Name', { exact: true }).fill(name)
  await page.getByLabel(/Ingredients/).fill('1 kg tomatoes\n1 cucumber\nOlive oil')
  await page.getByLabel('Step 1', { exact: true }).fill('Blend everything.')
  await page.getByRole('button', { name: 'Add step' }).click()
  await page.getByLabel('Step 2', { exact: true }).fill('Chill for an hour.')
  await page.getByLabel(/Tags/).fill('Summer, Vegan')
  await page.getByRole('button', { name: 'Save' }).click()

  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(page).toHaveURL(new RegExp(`^.*${cookbook.path}/recipes/[^/]+$`))
  await expect(page.getByRole('link', { name: 'vegan', exact: true })).toBeVisible()

  // A deep link survives a full reload: the server hands client routes to the app.
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(page.getByRole('listitem').filter({ hasText: 'Chill for an hour.' })).toBeVisible()

  await page.getByRole('link', { name: '← All recipes' }).click()
  await expect(page.getByRole('list', { name: 'Recipes' }).getByRole('link', { name })).toBeVisible()
})

test('edit a recipe; the change is saved', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({
    name: `Pancakes ${unique}`,
    ingredients: ['200 g flour'],
    steps: ['Whisk.', 'Fry.'],
    tags: ['breakfast'],
  })

  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)
  await page.getByRole('link', { name: 'Edit' }).click()
  await page.getByLabel('Name', { exact: true }).fill(`Fluffy pancakes ${unique}`)
  await page.getByRole('button', { name: 'Remove step 2' }).click()
  await page.getByLabel(/^Source/).fill('Grandma')
  await page.getByRole('button', { name: 'Save' }).click()

  await expect(page.getByRole('heading', { level: 1, name: `Fluffy pancakes ${unique}` })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: `Fluffy pancakes ${unique}` })).toBeVisible()
  await expect(page.getByText('Source: Grandma')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Steps' }).locator('+ ol > li')).toHaveCount(1)
})

test('delete asks for confirmation', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({ name: `Soup ${unique}` })
  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)

  // Cancelling keeps the recipe.
  page.once('dialog', (dialog) => dialog.dismiss())
  await page.getByRole('button', { name: 'Delete' }).click()
  await expect(page).toHaveURL(`${cookbook.path}/recipes/${recipe.id}`)

  // Confirming deletes it.
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: 'Delete' }).click()
  await expect(page).toHaveURL(cookbook.path)
  await expect(page.getByRole('link', { name: `Soup ${unique}` })).toHaveCount(0)

  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)
  await expect(page.getByRole('heading', { name: 'Not found' })).toBeVisible()
})

test('search and tag filters survive navigating back', async ({ page, unique, cookbook, createRecipe }) => {
  const tag = `tag-${unique}`.toLowerCase()
  await createRecipe({ name: `Escalivada ${unique}`, tags: [tag, 'vegan'] })
  await createRecipe({ name: `Pa amb tomàquet ${unique}`, tags: [tag] })
  await createRecipe({ name: `Lentil soup ${unique}`, tags: ['soup'] })
  const list = page.getByRole('list', { name: 'Recipes' })

  // Search ignores accents and case.
  await page.goto(cookbook.path)
  await page.getByRole('searchbox').fill(`TOMAQUET ${unique}`)
  await expect(list.getByRole('link', { name: `Pa amb tomàquet ${unique}` })).toBeVisible()
  await expect(list.locator('.card')).toHaveCount(1)

  // Filter by tag from a recipe, open another recipe, come back: still filtered.
  await page.getByRole('searchbox').fill('')
  await list.getByRole('link', { name: `Escalivada ${unique}` }).click()
  await page.getByRole('link', { name: tag, exact: true }).click()
  await expect(page).toHaveURL(`${cookbook.path}?tag=${encodeURIComponent(tag)}`)
  await expect(list.locator('.card')).toHaveCount(2)
  await list.getByRole('link', { name: `Pa amb tomàquet ${unique}` }).click()
  await page.goBack()
  await expect(page).toHaveURL(`${cookbook.path}?tag=${encodeURIComponent(tag)}`)
  await expect(list.locator('.card')).toHaveCount(2)
  await expect(list.getByRole('link', { name: `Lentil soup ${unique}` })).toHaveCount(0)
})

test('reorder steps by dragging their handle', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({ name: `Bread ${unique}`, steps: ['Mix.', 'Knead.', 'Bake.'] })
  await page.goto(`${cookbook.path}/recipes/${recipe.id}/edit`)
  const step = (n: number) => page.getByRole('textbox', { name: `Step ${n}`, exact: true })
  await expect(step(3)).toHaveValue('Bake.')

  // Drag step 3's handle above step 1. Both must be on screen: the mouse works
  // in viewport coordinates, and phone screens are short.
  await step(1).evaluate((el) => el.scrollIntoView({ block: 'start' }))
  const from = (await page.getByRole('button', { name: 'Reorder step 3' }).boundingBox())!
  const to = (await page.getByRole('button', { name: 'Reorder step 1' }).boundingBox())!
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2)
  await page.mouse.down()
  await page.mouse.move(to.x + to.width / 2, to.y + 2, { steps: 12 })
  await page.mouse.up()
  await expect(step(1)).toHaveValue('Bake.')
  await expect(step(2)).toHaveValue('Mix.')
  await expect(step(3)).toHaveValue('Knead.')

  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { level: 1, name: `Bread ${unique}` })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Steps' }).locator('+ ol > li')).toHaveText([
    'Bake.',
    'Mix.',
    'Knead.',
  ])
})

test('reorder steps with the keyboard', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({ name: `Bread ${unique}`, steps: ['Mix.', 'Knead.', 'Bake.'] })
  await page.goto(`${cookbook.path}/recipes/${recipe.id}/edit`)
  const step = (n: number) => page.getByRole('textbox', { name: `Step ${n}`, exact: true })

  // Pick up step 3, move it up one, drop it. Moves are announced to screen
  // readers; waiting for the announcements keeps the keys in step with the drag.
  await page.getByRole('button', { name: 'Reorder step 3' }).focus()
  await page.keyboard.press('Space')
  await expect(page.locator('li.dragging')).toHaveCount(1)
  await page.keyboard.press('ArrowUp')
  await expect(page.getByText('Step 3 is over position 2.')).toBeAttached()
  await page.keyboard.press('Space')
  await expect(page.getByText('Step 3 moved to position 2.')).toBeAttached()

  await expect(step(1)).toHaveValue('Mix.')
  await expect(step(2)).toHaveValue('Bake.')
  await expect(step(3)).toHaveValue('Knead.')
})

test('line breaks within a step are shown', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({ name: `Pasta ${unique}`, steps: ['Make the sauce:\nstir it well.'] })

  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)

  const firstStep = page.getByRole('heading', { name: 'Steps' }).locator('+ ol > li').first()
  // innerText reflects how text is rendered, so collapsed line breaks would show as a space.
  expect(await firstStep.evaluate((li) => (li as HTMLElement).innerText)).toBe('Make the sauce:\nstir it well.')
})
