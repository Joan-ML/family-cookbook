import { lazy, Suspense } from 'react'
import { Link, Route, Switch } from 'wouter'
import { Footer, Loading } from './components'
import { NotFound } from './pages/NotFound'
import { RecipeListPage } from './pages/RecipeListPage'
import { RecipePage } from './pages/RecipePage'

// The form (and its drag-and-drop library) loads only when someone edits.
const RecipeFormPage = lazy(() => import('./pages/RecipeFormPage').then((m) => ({ default: m.RecipeFormPage })))

// Routes follow the browser's location by default; tests wrap App in a
// <Router> with an in-memory location instead.
function App() {
  return (
    <>
      <header className="site-header">
        <Link href="/" className="site-title">
          Family Cookbook
        </Link>
        <p className="site-tagline">Our recipes, all in one place</p>
      </header>
      <main>
        <Suspense fallback={<Loading />}>
          <Switch>
            <Route path="/" component={RecipeListPage} />
            <Route path="/new">{() => <RecipeFormPage />}</Route>
            <Route path="/recipes/:id/edit">{(params) => <RecipeFormPage key={params.id} id={params.id} />}</Route>
            <Route path="/recipes/:id">{(params) => <RecipePage key={params.id} id={params.id} />}</Route>
            <Route>
              <NotFound />
            </Route>
          </Switch>
        </Suspense>
      </main>
      <Footer />
    </>
  )
}

export default App
