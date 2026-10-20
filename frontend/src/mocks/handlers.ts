import { http, HttpResponse } from 'msw/http'
import type { Health } from '../api/client'

// Mock API used by unit tests (src/mocks/node.ts) and by `npm run dev:mock`
// (src/mocks/browser.ts). Response bodies are typed from the OpenAPI schema,
// so mocks stay in line with the contract the backend is tested against.
export const handlers = [
  http.get('*/api/health', () => HttpResponse.json<Health>({ status: 'ok' })),
]
