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
│   └── internal/
│       ├── api/       # handlers, generated code, contract tests
│       ├── store/     # storage interface; sqlite/ implements it
│       └── web/       # serves the embedded frontend in the image
├── frontend/          # React single-page app
│   └── src/
│       ├── api/       # typed client + types generated from the contract
│       └── mocks/     # MSW handlers for tests and mock dev mode
├── e2e/               # Playwright tests against the running app
├── .github/workflows/ # CI: one workflow per package, plus the image
├── Dockerfile         # the single deployable image
├── compose.yaml       # run the image locally
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

### Running locally

Requirements: Go 1.26+ and Node.js 22+. There is no recipe UI yet: the web page only shows whether it can reach the API, so try recipes through the API directly.

1. **Start the backend** (terminal 1). It creates `backend/data/cookbook.db` on first run.

   ```sh
   cd backend
   go run ./cmd/server
   ```

2. **Start the frontend** (terminal 2), then open <http://localhost:5173>. It should show `API: ok`.

   ```sh
   cd frontend
   npm install   # first time only
   npm run dev
   ```

3. **Try the API** with curl (below), or import `api/openapi.yaml` into Bruno, Postman or a similar client, using `http://localhost:8080/api` as the base URL. Every request runs as the user `dev` until login exists.

   ```sh
   API=http://localhost:8080/api

   curl -i -X POST $API/recipes -H 'Content-Type: application/json' -d '{
     "name": "Pancakes",
     "ingredients": ["200 g flour", "2 eggs", "300 ml milk"],
     "steps": ["Whisk everything together.", "Fry in a hot pan."],
     "tags": ["Breakfast", "vegetarian"],
     "source": "Grandma"
   }'                                   # 201; the Location header has the new recipe's URL

   curl $API/recipes                    # list
   curl $API/recipes/<id>               # get one
   curl -X PUT $API/recipes/<id> -H 'Content-Type: application/json' -d '{...}'  # replace
   curl -X DELETE $API/recipes/<id>     # delete
   ```

To start over with an empty database, stop the backend and delete `backend/data/`.

To work on the frontend without the backend, run `npm run dev:mock` instead of step 1 and 2. The API is then mocked in the browser, with two sample recipes that reset on page reload.

### Development

Each package is self-contained and uses its own tools.

**Backend** (Go 1.26+), from `backend/`:

```sh
go run ./cmd/server   # API on :8080 (env: ADDR, DATA_DIR, default "data")
go test ./...         # tests, including contract validation
go generate ./...     # regenerate code after editing api/openapi.yaml
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

**End-to-end tests** (Node.js 22+), from `e2e/`. They drive the real app in phone-sized Chromium (Android) and WebKit (iPhone), through the real server and database. CI runs them against every image before publishing it. Each test creates uniquely named recipes and deletes them afterwards, so any running instance works, but prefer a scratch one:

```sh
npm install                              # once
npx playwright install chromium webkit   # once; downloads browsers to ~/.cache/ms-playwright
BASE_URL=http://localhost:8080 npx playwright test          # against the image or `go run`
BASE_URL=http://localhost:5173 npx playwright test          # or through Vite
npx playwright test --project=android    # one browser only
npm run report                           # HTML report of the last run
```

Playwright's WebKit is built for Ubuntu and needs library versions that other distributions (e.g. Fedora) don't ship, so it may not start locally; use `--project=android` there and let CI run both.

Local data (the SQLite file and photos) lives in `data/`, which git ignores.

### Deployment

The whole app ships as **one Docker image**: a multi-stage build compiles the frontend, embeds it in the Go binary, and produces a small runtime image. A mounted volume holds the SQLite file and the photos.

Why one container rather than separate backend and nginx/frontend containers:

- **The frontend is just static files**, which Go can serve. nginx would be a second process with nothing else to do.
- **Same origin:** UI and API share a host and port, so there's no CORS and session cookies work simply.
- **Frontend and backend can't get out of sync:** they are built, versioned and deployed together.
- **Less to run:** one container, one volume, one health check, one log stream.

HTTPS, compression and similar concerns are handled by a reverse proxy in front of the container (e.g. Caddy), not by the app.

The image runs as an unprivileged user on a distroless base (no shell). It serves the UI and the API on port 8080 and keeps its data in `/data`.

**Running a release.** On the server, pull a published image and give it its version through one variable, so the image tag and the reported version can't disagree:

```yaml
# compose.yaml on the server
services:
  cookbook:
    image: ghcr.io/<owner>/<repo>:${VERSION}
    environment:
      APP_VERSION: v${VERSION}
    ports:
      - "8080:8080"
    volumes:
      - cookbook-data:/data
    restart: unless-stopped

volumes:
  cookbook-data:
```

```sh
VERSION=0.2.0 docker compose up -d
```

**Building from source.** From the repository root, with Docker or Podman (`podman` takes the same arguments):

```sh
COMMIT=$(git rev-parse --short HEAD) docker compose up --build   # open http://localhost:8080

# or by hand:
docker build --build-arg COMMIT=$(git rev-parse --short HEAD) -t family-cookbook .
docker run -d --name cookbook -p 8080:8080 -v cookbook-data:/data family-cookbook
```

Use a named volume as above. To use a host directory instead, it must be writable by the container's user (UID 65532); with rootless Podman, add `:U` to the mount (`-v ./data:/data:U`).

To back up, copy `cookbook.db` from the volume while the container is stopped. The `.db-wal` and `.db-shm` files next to it are only present while it runs.

Until login exists, the image is for local use only: don't expose it to the internet.

### Versioning and releases

Images are **built once and promoted**, never rebuilt for a release:

1. Every commit on `main` is built, smoke-tested and published as `sha-<commit>` (and `main`).
2. Tagging a commit `vX.Y.Z` copies that commit's image to the release tags. The digest is unchanged, so a release is byte-for-byte the image that was tested. Only commits built on `main` can be released.

Because an image exists before it has a release number, it only knows its commit. The release version is given when deploying, as `APP_VERSION` (see above). The page footer and `GET /api/info` show both, e.g. `v0.2.0 (abc1234)`, or just the commit when `APP_VERSION` isn't set.

Versions follow SemVer, with the numbers chosen for whoever runs the server:

- **Major:** upgrading needs manual action or can't be undone, e.g. a database migration worth a backup first.
- **Minor:** new features.
- **Patch:** fixes.

Versions stay at `0.x` until login exists; `v1.0.0` will mean "safe to deploy".

To release, tag a commit that is on `main` and push the tag:

```sh
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

Published images (GitHub Container Registry):

| Image tag | Contains |
|---|---|
| `1.2.3` | that release |
| `1.2` | the latest `1.2.x` release |
| `latest` | the latest release |
| `main` | the latest commit on `main`, unreleased |
| `sha-<commit>` | the build of that commit |
