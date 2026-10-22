import { Link, Route, Switch } from 'wouter'
import { NotFound } from './pages/NotFound'
import { RecipeFormPage } from './pages/RecipeFormPage'
import { RecipeListPage } from './pages/RecipeListPage'
import { RecipePage } from './pages/RecipePage'

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
        <Switch>
          <Route path="/" component={RecipeListPage} />
          <Route path="/new">{() => <RecipeFormPage />}</Route>
          <Route path="/recipes/:id/edit">{(params) => <RecipeFormPage key={params.id} id={params.id} />}</Route>
          <Route path="/recipes/:id">{(params) => <RecipePage key={params.id} id={params.id} />}</Route>
          <Route>
            <NotFound />
          </Route>
        </Switch>
      </main>
      <footer className="site-footer">Family Cookbook · {import.meta.env.VITE_APP_VERSION}</footer>
    </>
  )
}

export default App
