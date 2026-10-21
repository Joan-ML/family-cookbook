import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { resetMockData } from './mocks/handlers'
import { server } from './mocks/node'

// Fail loudly if a component calls an endpoint that has no mock.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetMockData()
})
afterAll(() => server.close())
