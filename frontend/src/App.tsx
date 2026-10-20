import { useEffect, useState } from 'react'
import { api } from './api/client'

type Status = 'checking' | 'ok' | 'unreachable'

function App() {
  const [status, setStatus] = useState<Status>('checking')

  useEffect(() => {
    api
      .GET('/health')
      .then(({ data }) => setStatus(data?.status === 'ok' ? 'ok' : 'unreachable'))
      .catch(() => setStatus('unreachable'))
  }, [])

  return (
    <main>
      <h1>Family Cookbook</h1>
      <p role="status">API: {status}</p>
    </main>
  )
}

export default App
