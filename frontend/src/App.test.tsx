import { render, screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw/http'
import { expect, test } from 'vitest'
import App from './App'
import { server } from './mocks/node'

test('shows the API as ok when the health check succeeds', async () => {
  render(<App />)

  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('API: ok'))
})

test('shows the API as unreachable when the health check fails', async () => {
  server.use(http.get('*/api/health', () => new HttpResponse(null, { status: 500 })))

  render(<App />)

  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('API: unreachable'))
})
