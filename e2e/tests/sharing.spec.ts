import { friend } from './account'
import { expect, test } from './fixtures'

test('share a cookbook with someone, who can then cook from it, and leave it', async ({
  page,
  friendPage,
  cookbook,
  createRecipe,
  unique,
}) => {
  await createRecipe({ name: `Shared soup ${unique}` })

  // Before being added, the friend can't see the cookbook at all.
  await friendPage.goto(cookbook.path)
  await expect(friendPage.getByRole('heading', { name: 'Not found' })).toBeVisible()

  // The admin adds them as an editor.
  await page.goto(`${cookbook.path}/settings`)
  await page.getByLabel('Username to add').fill(friend.username)
  await page.getByRole('button', { name: 'Add' }).click()
  await expect(page.getByLabel(`Role of ${friend.username}`)).toHaveValue('editor')

  // Now the friend sees it among their cookbooks, with its recipe, and can edit.
  await friendPage.goto('/')
  await friendPage.getByRole('list', { name: 'Your cookbooks' }).getByRole('link', { name: cookbook.name }).click()
  await friendPage.getByRole('link', { name: `Shared soup ${unique}` }).click()
  await friendPage.getByRole('link', { name: 'Edit' }).click()
  await friendPage.getByLabel('Name', { exact: true }).fill(`Shared soup, improved ${unique}`)
  await friendPage.getByRole('button', { name: 'Save' }).click()
  await expect(friendPage.getByText(`Last edited by ${friend.username}`)).toBeVisible()

  // The admin sees the friend's change.
  await page.goto(cookbook.path)
  await expect(page.getByRole('link', { name: `Shared soup, improved ${unique}` })).toBeVisible()

  // As an editor, the friend sees the members but can't manage them; they leave.
  await friendPage.goto(`${cookbook.path}/settings`)
  await expect(friendPage.getByText("Only the cookbook's admins can add or remove members.")).toBeVisible()
  friendPage.once('dialog', (dialog) => dialog.accept())
  await friendPage.getByRole('button', { name: 'Leave cookbook' }).click()
  await expect(friendPage).toHaveURL('/')
  await expect(friendPage.getByRole('link', { name: cookbook.name })).toHaveCount(0)

  // The admin's member list no longer has them.
  await page.goto(`${cookbook.path}/settings`)
  await expect(page.getByRole('list', { name: 'Members' }).getByRole('listitem')).toHaveCount(1)
})

test('the only admin is told to hand over before leaving', async ({ page, cookbook }) => {
  await page.goto(`${cookbook.path}/settings`)

  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: 'Leave cookbook' }).click()

  await expect(page.getByRole('alert')).toContainText('make someone else admin before leaving')
  await expect(page).toHaveURL(`${cookbook.path}/settings`)
})
