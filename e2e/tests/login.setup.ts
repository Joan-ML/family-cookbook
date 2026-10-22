import { expect, test as setup } from '@playwright/test'
import { account, friend } from './account'

for (const [who, credentials, file] of [
  ['the test account', account, '.auth/session.json'],
  ['the friend account', friend, '.auth/friend.json'],
] as const) {
  setup(`log in as ${who}`, async ({ playwright, baseURL }) => {
    const request = await playwright.request.newContext({ baseURL })
    const res = await request.post('/api/session', { data: credentials })
    expect(res.status(), `can't log in as "${credentials.username}"; does that account exist on the server?`).toBe(200)
    await request.storageState({ path: file })
    await request.dispose()
  })
}
