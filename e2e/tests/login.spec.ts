import { expect, test } from './fixtures'
import { account } from './account'

// These tests start logged out.
test.use({ storageState: { cookies: [], origins: [] } })

test('log in from a deep link, then log out', async ({ page, unique, cookbook, createRecipe }) => {
  const recipe = await createRecipe({ name: `Soup ${unique}` })

  await page.goto(`${cookbook.path}/recipes/${recipe.id}`)
  await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible()
  await page.getByLabel('Username').fill(account.username.toUpperCase()) // usernames ignore case
  await page.getByLabel('Password').fill(account.password)
  await page.getByRole('button', { name: 'Log in' }).click()

  // The address asked for opens after logging in.
  await expect(page.getByRole('heading', { level: 1, name: `Soup ${unique}` })).toBeVisible()
  await expect(page.getByRole('banner')).toContainText(`${account.username} · Log out`)

  // The session survives a reload, and logging out ends it.
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: `Soup ${unique}` })).toBeVisible()
  await page.getByRole('banner').getByRole('button', { name: 'Log out' }).click()
  await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible()
})

test('a wrong password is refused', async ({ page, unique }) => {
  await page.goto('/')
  // A made-up user: failures against the test account would lock it out
  // (5 per 15 minutes) for the other tests. The answer is the same either way.
  await page.getByLabel('Username').fill(`nobody-${unique}`.toLowerCase())
  await page.getByLabel('Password').fill('not the password')
  await page.getByRole('button', { name: 'Log in' }).click()

  await expect(page.getByRole('alert')).toHaveText('wrong username or password')
})
