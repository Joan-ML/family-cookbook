import { useState } from 'react'
import { Link, useLocation } from 'wouter'
import type { Recipe } from '../api/client'
import { deleteRecipe, getRecipe } from '../api/recipes'
import { Loading, LoadError, TagList } from '../components'
import { useLoad } from '../useLoad'
import { NotFound } from './NotFound'

export function RecipePage({ id }: { id: string }) {
  const recipe = useLoad(() => getRecipe(id), id)

  if (recipe.status === 'loading') return <Loading />
  if (recipe.status === 'error') return <LoadError message={recipe.message} onRetry={recipe.reload} />
  if (!recipe.data) return <NotFound what="recipe" />
  return <RecipeView recipe={recipe.data} />
}

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

function RecipeView({ recipe }: { recipe: Recipe }) {
  const [, navigate] = useLocation()
  const [deleteError, setDeleteError] = useState('')

  async function onDelete() {
    if (!window.confirm(`Delete "${recipe.name}"? This can't be undone.`)) return
    try {
      await deleteRecipe(recipe.id)
      navigate('/', { replace: true })
    } catch (err) {
      setDeleteError((err as Error).message)
    }
  }

  return (
    <article className="recipe">
      <Link className="back" href="/">
        ← All recipes
      </Link>
      <h1>{recipe.name}</h1>
      <TagList tags={recipe.tags} />
      {recipe.source && (
        <p className="muted">
          Source: <Source source={recipe.source} />
        </p>
      )}

      <section>
        <h2>Ingredients</h2>
        {recipe.ingredients.length > 0 ? (
          <ul className="ingredients">
            {recipe.ingredients.map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ul>
        ) : (
          <p className="muted">No ingredients listed.</p>
        )}
      </section>

      <section>
        <h2>Steps</h2>
        {recipe.steps.length > 0 ? (
          <ol className="steps">
            {recipe.steps.map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
        ) : (
          <p className="muted">No steps listed.</p>
        )}
      </section>

      <p className="muted meta">
        Last edited by {recipe.updatedBy} on {dateFormat.format(new Date(recipe.updatedAt))}
      </p>

      {deleteError && (
        <p className="alert" role="alert">
          {deleteError}
        </p>
      )}
      <div className="actions">
        <Link className="button primary" href={`/recipes/${encodeURIComponent(recipe.id)}/edit`}>
          Edit
        </Link>
        <button type="button" className="button danger" onClick={onDelete}>
          Delete
        </button>
      </div>
    </article>
  )
}

/** Shows web addresses as links and anything else ("Grandma") as text. */
function Source({ source }: { source: string }) {
  if (/^https?:\/\//i.test(source)) {
    return (
      <a href={source} target="_blank" rel="noreferrer">
        {source.replace(/^https?:\/\/(www\.)?/i, '').replace(/\/$/, '')}
      </a>
    )
  }
  return <>{source}</>
}
