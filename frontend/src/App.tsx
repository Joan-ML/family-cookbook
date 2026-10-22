import { lazy, Suspense } from 'react'
import { Link, Route, Switch } from 'wouter'
import { useAuth } from './auth'
import { AuthProvider } from './AuthProvider'
import { Footer, Loading } from './components'
import { LoginPage } from './pages/LoginPage'
import { NotFound } from './pages/NotFound'
import { RecipeListPage } from './pages/RecipeListPage'
import { RecipePage } from './pages/RecipePage'

// The form (and its drag-and-drop library) loads only when someone edits.
const RecipeFormPage = lazy(() => import('./pages/RecipeFormPage').then((m) => ({ default: m.RecipeFormPage })))

// Routes follow the browser's location by default; tests wrap App in a
// <Router> with an in-memory location instead.
function App() {
  return (
    <AuthProvider>
      <Header />
      <main>
        <Content />
      </main>
      <Footer />
    </AuthProvider>
  )
}

function Header() {
  const { state, logOut } = useAuth()
  return (
    <header className="site-header">
      <Link href="/" className="site-title">
        Family Cookbook
      </Link>
      <p className="site-tagline">Our recipes, all in one place</p>
      {state.status === 'user' && (
        <p className="session-bar">
          {state.user.username} ·{' '}
          <button type="button" className="link-button" onClick={logOut}>
            Log out
          </button>
        </p>
      )}
    </header>
  )
}

// Logged out, every address shows the login page; after logging in, the
// same address shows what was asked for, so links survive a login.
function Content() {
  const { state } = useAuth()
  if (state.status === 'checking') return <Loading />
  if (state.status === 'anonymous') return <LoginPage sessionEnded={state.ended} />
  return (
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
  )
}

export default App
