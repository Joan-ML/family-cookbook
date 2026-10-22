import { Link, useSearchParams } from 'wouter'
import type { RecipeSummary } from '../api/client'
import { getCookbook } from '../api/cookbooks'
import { listRecipes } from '../api/recipes'
import { Loading, LoadError, TagList } from '../components'
import { paths } from '../paths'
import { useLoad } from '../useLoad'
import { NotFound } from './NotFound'

/** Lowercases and strips accents, so "tomaquet" finds "Tomàquet". */
function fold(s: string) {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

function matches(recipe: RecipeSummary, query: string, tag: string) {
  if (tag && !recipe.tags.includes(tag)) return false
  const q = fold(query.trim())
  return !q || fold(recipe.name).includes(q) || recipe.tags.some((t) => fold(t).includes(q))
}

/** A cookbook's page: its recipes, with search and tag filters. */
export function RecipeListPage({ cookbookId }: { cookbookId: string }) {
  const page = useLoad(async () => {
    const cookbook = await getCookbook(cookbookId)
    return cookbook && { cookbook, recipes: await listRecipes(cookbookId) }
  }, cookbookId)
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

  if (page.status === 'loading') return <Loading />
  if (page.status === 'error') return <LoadError message={page.message} onRetry={page.reload} />
  if (!page.data) return <NotFound what="cookbook" />
  const { cookbook, recipes } = page.data

  return (
    <>
      <Link className="back" href={paths.home}>
        ← All cookbooks
      </Link>
      <div className="page-title">
        <h1>{cookbook.name}</h1>
        <Link className="button primary" href={paths.newRecipe(cookbook.id)}>
          New recipe
        </Link>
      </div>
      {cookbook.role === 'admin' && (
        <p className="cookbook-meta">
          <Link href={paths.cookbookSettings(cookbook.id)}>Cookbook settings</Link>
        </p>
      )}

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

      <RecipeList
        cookbookId={cookbook.id}
        recipes={recipes.filter((r) => matches(r, query, tag))}
        total={recipes.length}
      />
    </>
  )
}

function RecipeList({ cookbookId, recipes, total }: { cookbookId: string; recipes: RecipeSummary[]; total: number }) {
  if (total === 0) {
    return (
      <p className="empty">
        No recipes yet. <Link href={paths.newRecipe(cookbookId)}>Add the first one</Link>.
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
          <Link className="card-link" href={paths.recipe(cookbookId, r.id)}>
            {r.name}
          </Link>
          <TagList tags={r.tags} cookbookId={cookbookId} />
        </li>
      ))}
    </ul>
  )
}
