import { expect, test } from '@playwright/test'

// Setup links printed by `cookbook user add e2e-newbie-<project>`, e.g.
// "/set-password#K3x9...", one per phone project in E2E_SETUP_LINK_ANDROID
// and E2E_SETUP_LINK_IPHONE. Links work once, so each run needs fresh ones;
// without one, the test is skipped.

// Starts logged out, like someone opening the link they were sent.
test.use({ storageState: { cookies: [], origins: [] } })

test('a setup link lets someone choose a password, then change it', async ({ page }, testInfo) => {
  const project = testInfo.project.name
  const link = process.env[`E2E_SETUP_LINK_${project.toUpperCase()}`]
  const username = `e2e-newbie-${project}`
  test.skip(!link, `set E2E_SETUP_LINK_${project.toUpperCase()} to a fresh link from \`cookbook user add ${username}\``)
  const path = link!.slice(link!.indexOf('/set-password'))

  // Open the link: choose a password, and you're in.
  await page.goto(path)
  await expect(page.getByRole('heading', { name: 'Choose your password' })).toBeVisible()
  expect(new URL(page.url()).hash).toBe('') // the token left the address bar
  await page.getByLabel('New password').fill('first chosen password')
  await page.getByLabel('Repeat it').fill('first chosen password')
  await page.getByRole('button', { name: 'Save and log in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Cookbooks' })).toBeVisible()
  await expect(page.getByRole('banner')).toContainText(`${username} · Log out`)

  // Change it on the account page.
  await page.getByRole('banner').getByRole('link', { name: username }).click()
  await page.getByLabel('Current password').fill('first chosen password')
  await page.getByLabel(/^New password/).fill('second chosen password')
  await page.getByLabel('Repeat the new password').fill('second chosen password')
  await page.getByRole('button', { name: 'Change password' }).click()
  await expect(page.getByRole('status')).toContainText('Password changed')

  // The link is used up, and the new password is the one that works.
  await page.getByRole('banner').getByRole('button', { name: 'Log out' }).click()
  await page.goto(path)
  await page.getByLabel('New password').fill('a third password here')
  await page.getByLabel('Repeat it').fill('a third password here')
  await page.getByRole('button', { name: 'Save and log in' }).click()
  await expect(page.getByRole('alert')).toContainText('already used')

  await page.goto('/')
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill('second chosen password')
  await page.getByRole('button', { name: 'Log in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Cookbooks' })).toBeVisible()
})
