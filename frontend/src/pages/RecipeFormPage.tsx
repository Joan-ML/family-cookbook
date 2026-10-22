import { useState, type FormEvent } from 'react'
import { Link, useLocation } from 'wouter'
import type { Recipe, RecipeInput } from '../api/client'
import { createRecipe, getRecipe, updateRecipe } from '../api/recipes'
import { Loading, LoadError } from '../components'
import { newStep, type Step } from '../steps'
import { paths } from '../paths'
import { useLoad } from '../useLoad'
import { NotFound } from './NotFound'
import { StepsField } from './StepsField'

/** Create a recipe in a cookbook, or edit one when `id` is given. */
export function RecipeFormPage({ cookbookId, id }: { cookbookId: string; id?: string }) {
  if (!id) return <RecipeForm cookbookId={cookbookId} />
  return <EditRecipe cookbookId={cookbookId} id={id} />
}

function EditRecipe({ cookbookId, id }: { cookbookId: string; id: string }) {
  const recipe = useLoad(() => getRecipe(cookbookId, id), `${cookbookId}/${id}`)

  if (recipe.status === 'loading') return <Loading />
  if (recipe.status === 'error') return <LoadError message={recipe.message} onRetry={recipe.reload} />
  if (!recipe.data) return <NotFound what="recipe" />
  return <RecipeForm cookbookId={cookbookId} recipe={recipe.data} />
}

// The form edits plain strings; ingredients are one per line and tags are
// comma-separated.
type Fields = {
  name: string
  ingredients: string
  steps: Step[]
  tags: string
  source: string
}

function toFields(r?: Recipe): Fields {
  return {
    name: r?.name ?? '',
    ingredients: r?.ingredients.join('\n') ?? '',
    steps: r?.steps.length ? r.steps.map((text) => newStep(text)) : [newStep()],
    tags: r?.tags.join(', ') ?? '',
    source: r?.source ?? '',
  }
}

// The API rejects empty entries, so blank lines, steps and tags are dropped.
const nonBlank = (items: string[]) => items.map((s) => s.trim()).filter(Boolean)

function toInput(f: Fields): RecipeInput {
  return {
    name: f.name,
    ingredients: nonBlank(f.ingredients.split('\n')),
    steps: nonBlank(f.steps.map((s) => s.text)),
    tags: nonBlank(f.tags.split(',')),
    ...(f.source.trim() ? { source: f.source } : {}),
  }
}

function RecipeForm({ cookbookId, recipe }: { cookbookId: string; recipe?: Recipe }) {
  const [, navigate] = useLocation()
  const [fields, setFields] = useState(() => toFields(recipe))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const set = (patch: Partial<Fields>) => setFields((f) => ({ ...f, ...patch }))

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      const saved = recipe
        ? await updateRecipe(cookbookId, recipe.id, toInput(fields))
        : await createRecipe(cookbookId, toInput(fields))
      navigate(paths.recipe(cookbookId, saved.id), { replace: true })
    } catch (err) {
      setError((err as Error).message)
      setSaving(false)
    }
  }

  const cancelHref = recipe ? paths.recipe(cookbookId, recipe.id) : paths.cookbook(cookbookId)

  return (
    <form className="recipe-form" onSubmit={onSubmit}>
      <h1>{recipe ? 'Edit recipe' : 'New recipe'}</h1>

      <label className="field">
        <span>Name</span>
        <input
          value={fields.name}
          onChange={(e) => set({ name: e.target.value })}
          required
          maxLength={200}
          autoFocus={!recipe}
        />
      </label>

      <label className="field">
        <span>Ingredients</span>
        <small className="muted">One per line, e.g. "200 g flour"</small>
        <textarea rows={6} value={fields.ingredients} onChange={(e) => set({ ingredients: e.target.value })} />
      </label>

      <fieldset className="field">
        <legend>Steps</legend>
        <small className="muted">Drag ⠿ to reorder. Pasting several steps splits them up.</small>
        <StepsField steps={fields.steps} onChange={(steps) => set({ steps })} />
        <button type="button" className="button" onClick={() => set({ steps: [...fields.steps, newStep()] })}>
          Add step
        </button>
      </fieldset>

      <label className="field">
        <span>Tags</span>
        <small className="muted">Separated by commas, e.g. "vegetarian, quick"</small>
        <input value={fields.tags} onChange={(e) => set({ tags: e.target.value })} />
      </label>

      <label className="field">
        <span>Source</span>
        <small className="muted">Optional: a web address, a book, "Grandma"…</small>
        <input value={fields.source} onChange={(e) => set({ source: e.target.value })} maxLength={500} />
      </label>

      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <div className="actions">
        <button type="submit" className="button primary" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
        <Link className="button" href={cancelHref}>
          Cancel
        </Link>
      </div>
    </form>
  )
}
