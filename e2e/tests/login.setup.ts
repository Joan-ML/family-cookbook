import { expect, test as setup } from '@playwright/test'
import { account } from './account'

setup('log in', async ({ request }) => {
  const res = await request.post('/api/session', { data: account })
  expect(res.status(), `can't log in as "${account.username}"; does that account exist on the server?`).toBe(200)
  await request.storageState({ path: '.auth/session.json' })
})
