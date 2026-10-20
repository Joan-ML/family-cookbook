import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// In `npm run dev:mock`, serve the API from MSW in the browser so the UI
// can be developed without a running backend. Dropped from production builds.
async function enableMocking() {
  if (import.meta.env.MODE !== 'mock') return
  const { worker } = await import('./mocks/browser')
  await worker.start({ onUnhandledFrame: 'bypass' })
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
})
