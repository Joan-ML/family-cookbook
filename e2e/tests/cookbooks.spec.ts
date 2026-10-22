import { expect, test } from './fixtures'

test('create a cookbook, fill it, rename it and delete it', async ({ page, unique, cleanUpCookbook }) => {
  const name = `Weeknight ${unique}`
  const renamed = `Weekend ${unique}`
  cleanUpCookbook(name)
  cleanUpCookbook(renamed)
  const yours = page.getByRole('list', { name: 'Your cookbooks' })

  // Create it from the home page; it opens empty.
  await page.goto('/')
  await page.getByLabel('New cookbook name').fill(name)
  await page.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(page.getByText(/No recipes yet/)).toBeVisible()

  // Add a recipe; home then counts it.
  await page.getByRole('link', { name: 'New recipe' }).click()
  await page.getByLabel('Name', { exact: true }).fill(`Omelette ${unique}`)
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { level: 1, name: `Omelette ${unique}` })).toBeVisible()
  await page.getByRole('link', { name: '← All recipes' }).click()
  await page.getByRole('link', { name: '← All cookbooks' }).click()
  await expect(yours.getByRole('listitem').filter({ hasText: name })).toHaveText(`${name}1 recipe · Admin`)

  // Rename it in its settings.
  await yours.getByRole('link', { name }).click()
  await page.getByRole('link', { name: 'Cookbook settings' }).click()
  await page.getByLabel('Name').fill(renamed)
  await page.getByRole('button', { name: 'Rename' }).click()
  await expect(page.getByRole('status')).toHaveText('Renamed.')

  // Delete it, after a confirmation that mentions its recipe.
  let question = ''
  page.once('dialog', (dialog) => {
    question = dialog.message()
    return dialog.accept()
  })
  await page.getByRole('button', { name: 'Delete cookbook' }).click()
  await expect(page).toHaveURL('/')
  expect(question).toBe(`Delete "${renamed}" and its recipe? This can't be undone.`)
  await expect(page.getByRole('heading', { level: 1, name: 'Cookbooks' })).toBeVisible()
  await expect(yours.getByRole('link', { name: renamed })).toHaveCount(0)
})

test("a cookbook that doesn't exist, or that you're not in, shows not found", async ({ page }) => {
  await page.goto('/cookbooks/not-a-cookbook-of-yours')

  await expect(page.getByRole('heading', { name: 'Not found' })).toBeVisible()
})
