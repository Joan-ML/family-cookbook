import { useState, type FormEvent } from 'react'
import { useLocation } from 'wouter'
import { useAuth } from '../auth'
import { paths } from '../paths'

/** Reads the token from the link (/set-password#<token>) and removes it from
 *  the address bar, so it doesn't linger in the browser's history. */
function takeTokenFromLink(): string {
  const token = decodeURIComponent(window.location.hash.slice(1))
  if (token) window.history.replaceState(null, '', window.location.pathname + window.location.search)
  return token
}

/** Where setup links land: choose your password, and you're logged in. */
export function SetPasswordPage() {
  const { setUpPassword } = useAuth()
  const [, navigate] = useLocation()
  const [token] = useState(takeTokenFromLink)
  const [password, setPassword] = useState('')
  const [repeat, setRepeat] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (!token) {
    return (
      <div className="empty">
        <h1>Incomplete link</h1>
        <p>This link is missing its code. Open the full link you were sent, or ask for a new one.</p>
      </div>
    )
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (password !== repeat) {
      setError("The passwords don't match.")
      return
    }
    setBusy(true)
    setError('')
    try {
      await setUpPassword(token, password)
      navigate(paths.home, { replace: true })
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <form className="login-form" onSubmit={onSubmit}>
      <h1>Choose your password</h1>
      <p className="muted">At least 10 characters. A few words in a row work well.</p>
      <label className="field">
        <span>New password</span>
        <input
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          autoFocus
        />
      </label>
      <label className="field">
        <span>Repeat it</span>
        <input
          type="password"
          autoComplete="new-password"
          value={repeat}
          onChange={(e) => setRepeat(e.target.value)}
          required
        />
      </label>
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="button primary" disabled={busy}>
        {busy ? 'Saving…' : 'Save and log in'}
      </button>
    </form>
  )
}
