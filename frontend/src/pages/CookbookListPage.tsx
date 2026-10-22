import { useState, type FormEvent } from 'react'
import { Link, useLocation } from 'wouter'
import type { CookbookSummary } from '../api/client'
import { createCookbook, listCookbooks } from '../api/cookbooks'
import { Loading, LoadError } from '../components'
import { paths } from '../paths'
import { useLoad } from '../useLoad'

/** Home: the cookbooks you're a member of, and a way to start a new one. */
export function CookbookListPage() {
  const cookbooks = useLoad(listCookbooks, 'cookbooks')

  return (
    <>
      <h1>Cookbooks</h1>
      {cookbooks.status === 'loading' && <Loading />}
      {cookbooks.status === 'error' && <LoadError message={cookbooks.message} onRetry={cookbooks.reload} />}
      {cookbooks.status === 'done' && <CookbookList cookbooks={cookbooks.data} />}
      <NewCookbook />
    </>
  )
}

function CookbookList({ cookbooks }: { cookbooks: CookbookSummary[] }) {
  if (cookbooks.length === 0) {
    return <p className="empty">No cookbooks yet. Create one below to start collecting recipes.</p>
  }
  return (
    <ul className="recipe-list" aria-label="Your cookbooks">
      {cookbooks.map((c) => (
        <li key={c.id} className="card">
          <Link className="card-link" href={paths.cookbook(c.id)}>
            {c.name}
          </Link>
          <p className="muted card-detail">
            {c.recipeCount === 1 ? '1 recipe' : `${c.recipeCount} recipes`} · {c.role === 'admin' ? 'Admin' : 'Editor'}
          </p>
        </li>
      ))}
    </ul>
  )
}

function NewCookbook() {
  const [, navigate] = useLocation()
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const created = await createCookbook(name)
      navigate(paths.cookbook(created.id))
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <form className="new-cookbook" onSubmit={onSubmit}>
      <h2>New cookbook</h2>
      <div className="inline-field">
        <input
          aria-label="New cookbook name"
          placeholder="e.g. Family, Weeknight dinners"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={100}
          required
        />
        <button type="submit" className="button primary" disabled={busy}>
          Create
        </button>
      </div>
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
    </form>
  )
}
