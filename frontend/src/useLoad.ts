import { useCallback, useEffect, useState } from 'react'

export type LoadState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'done'; data: T }

/**
 * Runs `load` when the component mounts or `key` changes, and tracks the
 * result. `reload` runs it again, e.g. from a "Try again" button.
 */
export function useLoad<T>(load: () => Promise<T>, key: string): LoadState<T> & { reload: () => void } {
  const [state, setState] = useState<LoadState<T>>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let current = true
    setState({ status: 'loading' })
    load()
      .then((data) => current && setState({ status: 'done', data }))
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : String(err)
        if (current) setState({ status: 'error', message })
      })
    return () => {
      current = false
    }
    // `load` is usually an inline closure; `key` says when it really changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, attempt])

  const reload = useCallback(() => setAttempt((n) => n + 1), [])
  return { ...state, reload }
}
