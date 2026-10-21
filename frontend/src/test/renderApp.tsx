import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Router } from 'wouter'
import { memoryLocation } from 'wouter/memory-location'
import App from '../App'

/**
 * Renders the whole app at `path` with an in-memory router, against the MSW
 * mock API. `location()` returns the current path, including the query.
 */
export function renderApp(path = '/') {
  const [pathname, search = ''] = path.split('?')
  const memory = memoryLocation({ path: pathname, searchPath: search, record: true })
  const user = userEvent.setup()
  const view = render(
    <Router hook={memory.hook} searchHook={memory.searchHook}>
      <App />
    </Router>,
  )
  return { ...view, user, location: () => memory.history.at(-1) }
}
