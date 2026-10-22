import { useState, type FormEvent } from 'react'
import { Link, useLocation } from 'wouter'
import type { Cookbook, Member } from '../api/client'
import { deleteCookbook, getCookbook, renameCookbook } from '../api/cookbooks'
import { listMembers } from '../api/members'
import { listRecipes } from '../api/recipes'
import { useAuth } from '../auth'
import { Loading, LoadError } from '../components'
import { paths } from '../paths'
import { useLoad } from '../useLoad'
import { MembersSection } from './MembersSection'
import { NotFound } from './NotFound'

/**
 * Members & settings: every member sees who's in the cookbook and can leave;
 * admins also manage members, rename the cookbook and delete it.
 */
export function CookbookSettingsPage({ cookbookId }: { cookbookId: string }) {
  const { state } = useAuth()
  const page = useLoad(async () => {
    const cookbook = await getCookbook(cookbookId)
    if (!cookbook) return null
    const [members, recipes] = await Promise.all([listMembers(cookbookId), listRecipes(cookbookId)])
    return { cookbook, members, recipeCount: recipes.length }
  }, cookbookId)

  if (page.status === 'loading') return <Loading />
  if (page.status === 'error') return <LoadError message={page.message} onRetry={page.reload} />
  if (!page.data) return <NotFound what="cookbook" />
  const me = state.status === 'user' ? state.user.username : ''
  return <Settings loaded={page.data.cookbook} members={page.data.members} recipeCount={page.data.recipeCount} me={me} />
}

function Settings(props: { loaded: Cookbook; members: Member[]; recipeCount: number; me: string }) {
  // Renaming updates this, so the back link and the confirmations use the
  // current name.
  const [cookbook, setCookbook] = useState(props.loaded)

  return (
    <>
      <Link className="back" href={paths.cookbook(cookbook.id)}>
        ← {cookbook.name}
      </Link>
      <h1>Members & settings</h1>
      <MembersSection cookbook={cookbook} me={props.me} loaded={props.members} />
      {cookbook.role === 'admin' && (
        <>
          <Rename cookbook={cookbook} onRenamed={setCookbook} />
          <Delete cookbook={cookbook} recipeCount={props.recipeCount} />
        </>
      )}
    </>
  )
}

function Rename({ cookbook, onRenamed }: { cookbook: Cookbook; onRenamed: (c: Cookbook) => void }) {
  const [name, setName] = useState(cookbook.name)
  const [message, setMessage] = useState<{ error: boolean; text: string } | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    try {
      const renamed = await renameCookbook(cookbook.id, name)
      setName(renamed.name)
      onRenamed(renamed)
      setMessage({ error: false, text: 'Renamed.' })
    } catch (err) {
      setMessage({ error: true, text: (err as Error).message })
    }
  }

  return (
    <form className="settings-section" onSubmit={onSubmit}>
      <h2>Name</h2>
      <label className="field">
        <span>Cookbook name</span>
        <div className="inline-field">
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
          <button type="submit" className="button primary" disabled={name.trim() === cookbook.name}>
            Rename
          </button>
        </div>
      </label>
      {message && (
        <p className={message.error ? 'alert' : 'muted'} role={message.error ? 'alert' : 'status'}>
          {message.text}
        </p>
      )}
    </form>
  )
}

function Delete({ cookbook, recipeCount }: { cookbook: Cookbook; recipeCount: number }) {
  const [, navigate] = useLocation()
  const [error, setError] = useState('')
  const what = recipeCount === 1 ? 'its recipe' : `its ${recipeCount} recipes`

  async function onDelete() {
    const question =
      recipeCount === 0
        ? `Delete "${cookbook.name}"? This can't be undone.`
        : `Delete "${cookbook.name}" and ${what}? This can't be undone.`
    if (!window.confirm(question)) return
    try {
      await deleteCookbook(cookbook.id)
      navigate(paths.home, { replace: true })
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <section className="settings-section danger-zone">
      <h2>Delete cookbook</h2>
      <p className="muted">
        Deletes the cookbook {recipeCount > 0 && `and ${what} `}for everyone who uses it.
      </p>
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <button type="button" className="button danger" onClick={onDelete}>
        Delete cookbook
      </button>
    </section>
  )
}
