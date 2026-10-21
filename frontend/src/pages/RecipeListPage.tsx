import { Link, useSearchParams } from 'wouter'
import type { RecipeSummary } from '../api/client'
import { listRecipes } from '../api/recipes'
import { Loading, LoadError, TagList } from '../components'
import { useLoad } from '../useLoad'

/** Lowercases and strips accents, so "tomaquet" finds "Tomàquet". */
function fold(s: string) {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

function matches(recipe: RecipeSummary, query: string, tag: string) {
  if (tag && !recipe.tags.includes(tag)) return false
  const q = fold(query.trim())
  return !q || fold(recipe.name).includes(q) || recipe.tags.some((t) => fold(t).includes(q))
}

export function RecipeListPage() {
  const recipes = useLoad(listRecipes, 'recipes')
  // Filters live in the URL, so going back from a recipe restores them.
  const [params, setParams] = useSearchParams()
  const query = params.get('q') ?? ''
  const tag = params.get('tag') ?? ''

  function setFilter(name: 'q' | 'tag', value: string) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(name, value)
        else next.delete(name)
        return next
      },
      { replace: true },
    )
  }

  return (
    <>
      <div className="page-title">
        <h1>Recipes</h1>
        <Link className="button primary" href="/new">
          New recipe
        </Link>
      </div>

      <input
        type="search"
        className="search"
        placeholder="Search recipes or tags"
        aria-label="Search recipes"
        value={query}
        onChange={(e) => setFilter('q', e.target.value)}
      />
      {tag && (
        <p className="active-filter">
          Tagged <span className="tag">{tag}</span>
          <button type="button" className="link-button" onClick={() => setFilter('tag', '')}>
            Show all
          </button>
        </p>
      )}

      {recipes.status === 'loading' && <Loading />}
      {recipes.status === 'error' && <LoadError message={recipes.message} onRetry={recipes.reload} />}
      {recipes.status === 'done' && (
        <RecipeList recipes={recipes.data.filter((r) => matches(r, query, tag))} total={recipes.data.length} />
      )}
    </>
  )
}

function RecipeList({ recipes, total }: { recipes: RecipeSummary[]; total: number }) {
  if (total === 0) {
    return (
      <p className="empty">
        No recipes yet. <Link href="/new">Add the first one</Link>.
      </p>
    )
  }
  if (recipes.length === 0) {
    return <p className="empty">No recipes match.</p>
  }
  return (
    <ul className="recipe-list" aria-label="Recipes">
      {recipes.map((r) => (
        <li key={r.id} className="card">
          <Link className="card-link" href={`/recipes/${encodeURIComponent(r.id)}`}>
            {r.name}
          </Link>
          <TagList tags={r.tags} />
        </li>
      ))}
    </ul>
  )
}
