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
├── .github/workflows/ # CI, one workflow per package
└── README.md
```

### Contract-first development

Shipping as one image is a deployment choice. In development, the backend and frontend can move forward independently, and `api/openapi.yaml` is the contract between them:

1. **Change the contract first.** Every new or changed endpoint starts as an edit to `api/openapi.yaml`.
2. **The backend is generated from it and tested against it.** Go types and the server interface are generated from the spec with [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen), so a handler that returns the wrong shape fails to compile. API tests also send real requests through the handlers and validate both request and response against the spec (via `kin-openapi`), catching what types can't, such as status codes, enum values and required fields. A request to an undocumented route also fails.
3. **The frontend is built against it.** TypeScript types are generated from the spec (`openapi-typescript`) and used by a typed client (`openapi-fetch`). [MSW](https://mswjs.io) mocks the API, with mock bodies typed from the same schema, both in unit tests and in the browser (`npm run dev:mock`). Frontend work never needs a running backend.
4. **CI keeps them in sync.** Each package has its own GitHub Actions workflow that runs only when that package or `api/` changes, so a contract change must pass on both sides. The frontend workflow also fails if the generated types are stale (`npm run check:api`).

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

Recipes are built first and authentication second, but the app is designed for authentication from the start, so adding it later is a contained change:

- **Contract:** the spec declares a cookie-based security scheme that applies to every endpoint by default, and each endpoint documents a `401` response.
- **Backend:** every route goes through authentication middleware. Until login exists, the middleware attaches a fixed development user, and handlers always read the current user from the request.
- **Data:** recipes record `createdBy` and `updatedBy` from the start.
- **No deployment until authentication exists.**

There is no public sign-up. Accounts are managed with subcommands of the same binary, which run against the same database as the server (in Docker: `docker exec -it <container> cookbook user add <name>`):

```sh
cookbook serve                  # run the web app (default)
cookbook user add <username>    # passwords are always prompted, never passed as arguments
cookbook user passwd <username> # reset a password
cookbook user list
cookbook user remove <username>
```

### Development

Each package is self-contained and uses its own tools.

**Backend** (Go 1.26+), from `backend/`:

```sh
go run ./cmd/server   # API on :8080
go test ./...         # tests, including contract validation
```

**Frontend** (Node.js 22+), from `frontend/`:

```sh
npm install           # once
npm run dev           # Vite on :5173, passes /api requests through to :8080
npm run dev:mock      # API mocked by MSW; no backend needed
npm test              # unit tests
npm run lint
npm run gen:api       # regenerate types after editing api/openapi.yaml
npm run check:api     # fail if generated types are stale
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
