# Family Cookbook

A lightweight web application for keeping a household's recipes in one shared place.

This application's main purpose is for me to finally try vibe coding. Its secondary purpose is to fulfill the goals stated below.

## Goals

- **Shared vault.** Every recipe is visible to every user, and any user can edit any recipe. Everyone sees the same changes.
- **Simple recipes.** Each recipe has:
  - a name
  - photos (optional)
  - a list of ingredients
  - cooking instructions
  - tags you can type freely (e.g. `vegetarian`, `dessert`, `quick`)
  - a source (optional, e.g. a URL, a book, "Grandma")
- **Easy to find.** Browse recipes, search them by name and filter them by tag.
- **Lightweight and easy to use.** Few screens, fast to load, no clutter.
- **Mobile-first.** Works well on a phone in the kitchen, and on a desktop too.

## Non-goals

- Exporting to other formats (PDF, print layouts, etc.)
- Sharing recipes with people outside the household, or any social features
- Private or per-user recipes and fine-grained permissions
- Nutrition data, calorie counts, or scaling servings
- Meal planning and shopping lists

## Technical overview

### Repository layout (monorepo)

```
family-cookbook/
├── api/
│   └── openapi.yaml   # API contract shared by backend and frontend
├── backend/           # Go HTTP API
│   ├── cmd/server/    # entry point
│   └── internal/api/  # handlers and contract tests
├── frontend/          # React single-page app
│   └── src/
│       ├── api/       # typed client + types generated from the contract
│       └── mocks/     # MSW handlers for tests and mock dev mode
├── Makefile
└── README.md
```

### Contract-first development

Shipping as one image is a deployment choice. In development, the backend and frontend can move forward independently, and `api/openapi.yaml` is the contract between them:

1. **Change the contract first.** Every new or changed endpoint starts as an edit to `api/openapi.yaml`.
2. **The backend is tested against it.** API tests send real requests through the handlers and validate both request and response against the spec (via `kin-openapi`). A response that doesn't match the contract, or a request to an undocumented route, fails the test.
3. **The frontend is built against it.** TypeScript types are generated from the spec (`openapi-typescript`) and used by a typed client (`openapi-fetch`). [MSW](https://mswjs.io) mocks the API, with mock bodies typed from the same schema, both in unit tests and in the browser (`npm run dev:mock`). Frontend work never needs a running backend.
4. **CI keeps them in sync.** `make check-api` fails if the generated types are stale.

### Backend: Go

- A REST/JSON API built on the standard library's `net/http` (method-aware routing in Go 1.22+), with as few dependencies as possible.
- Resources: `recipes` (create, read, update, delete, search by name and tag), `tags` (list existing tags for autocomplete), `photos` (upload and serve).
- Photos are stored on disk and shrunk when uploaded. The database stores only references to them.
- In production the backend also serves the built frontend, which is embedded in the binary with `embed`.

### Frontend: React

- React + TypeScript, built with Vite.
- Mobile-first, responsive layout with a small set of views: recipe list (with search and tag filter), recipe detail, recipe edit/create.
- Plain CSS or a minimal utility layer. No heavy UI framework.

### Database: SQLite

We chose **SQLite** over MongoDB:

- **No extra service to run.** The database is a single file inside the backend process, which keeps hosting and backups trivial (copy one file plus the photos folder).
- **The data is small and simple.** A household cookbook holds hundreds of recipes, not millions. Ingredients, instructions and tags fit naturally in a few tables, or in JSON columns where that is simpler.
- **Search is built in.** SQLite's FTS5 extension covers searching by name.
- **Go support is good.** For example, `modernc.org/sqlite` is pure Go, so no CGO is needed.

The storage layer sits behind a small Go interface, so switching to another database later (e.g. PostgreSQL or MongoDB) won't affect the API or the frontend.

### Users and access

Since everything is shared, accounts exist only to keep strangers out and to record who last edited a recipe. Users log in with a username and password and stay logged in through session cookies. There are no roles.

### Development

Requirements: Go 1.26+, Node.js 22+.

```sh
cd frontend && npm install   # once

make dev-backend    # Go API on :8080
make dev-frontend   # Vite on :5173, passes /api requests through to :8080
make dev-mock       # Vite with the API mocked by MSW; no backend needed

make test           # backend tests, frontend lint + tests, contract check
make gen-api        # regenerate frontend types after editing api/openapi.yaml
```

Local data (the SQLite file and photos) lives in `data/`, which git ignores.

### Deployment

The whole app ships as **one Docker image**: a multi-stage build compiles the frontend, embeds it in the Go binary, and produces a small runtime image. A mounted volume holds the SQLite file and the photos.

Why one container rather than separate backend and nginx/frontend containers:

- **The frontend is just static files**, which Go can serve. nginx would be a second process with nothing else to do.
- **Same origin:** UI and API share a host and port, so there's no CORS and session cookies work simply.
- **Frontend and backend can't get out of sync:** they are built, versioned and deployed together.
- **Less to run:** one container, one volume, one health check, one log stream.

HTTPS, compression and similar concerns are handled by a reverse proxy in front of the container (e.g. Caddy), not by the app.
