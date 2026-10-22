import { useState, type FormEvent } from 'react'
import { Link } from 'wouter'
import { changePassword } from '../api/session'
import { useAuth } from '../auth'
import { paths } from '../paths'

/** Your account: change your password. */
export function AccountPage() {
  const { state } = useAuth()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  const [message, setMessage] = useState<{ error: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (next !== repeat) {
      setMessage({ error: true, text: "The new passwords don't match." })
      return
    }
    setBusy(true)
    setMessage(null)
    try {
      await changePassword(current, next)
      setCurrent('')
      setNext('')
      setRepeat('')
      setMessage({ error: false, text: 'Password changed. Other devices have been logged out.' })
    } catch (err) {
      setMessage({ error: true, text: (err as Error).message })
    }
    setBusy(false)
  }

  return (
    <>
      <Link className="back" href={paths.home}>
        ← All cookbooks
      </Link>
      <h1>Your account</h1>
      {state.status === 'user' && <p className="muted">Logged in as {state.user.username}.</p>}
      <form className="login-form account-form" onSubmit={onSubmit}>
        <h2>Change password</h2>
        <label className="field">
          <span>Current password</span>
          <input
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>New password</span>
          <small className="muted">At least 10 characters</small>
          <input
            type="password"
            autoComplete="new-password"
            value={next}
            onChange={(e) => setNext(e.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>Repeat the new password</span>
          <input
            type="password"
            autoComplete="new-password"
            value={repeat}
            onChange={(e) => setRepeat(e.target.value)}
            required
          />
        </label>
        {message && (
          <p className={message.error ? 'alert' : 'muted'} role={message.error ? 'alert' : 'status'}>
            {message.text}
          </p>
        )}
        <button type="submit" className="button primary" disabled={busy}>
          Change password
        </button>
      </form>
    </>
  )
}
