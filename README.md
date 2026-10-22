# Family Cookbook

A lightweight, self-hosted web application where family and friends keep their recipes in shared cookbooks.

This application's main purpose is for me to finally try vibe coding. Its secondary purpose is to fulfill the goals stated below.

## Goals

- **Shared cookbooks.** Users create cookbooks and invite other users to them. All members of a cookbook see and edit its recipes, and everyone sees the same changes. Recipes are only visible to the members of their cookbook.
- **Simple recipes.** Each recipe has:
  - a name
  - how many people it serves (optional)
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
- Public sign-up, or sharing recipes with people who have no account (e.g. public links)
- Social features: following, comments, ratings
- Fine-grained permissions beyond cookbook admins and editors
- Moving or copying recipes between cookbooks
- Nutrition data, calorie counts, or scaling recipes to a different number of servings
- Meal planning and shopping lists

## Technical overview

### Repository layout (monorepo)

```
family-cookbook/
├── api/
│   └── openapi.yaml   # API contract shared by backend and frontend
├── backend/           # Go HTTP API
│   ├── cmd/cookbook/  # the binary: serve, backup
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
- Mobile-first, responsive layout with a small set of views: your cookbooks, a cookbook's recipes (with search and tag filter), recipe detail, recipe edit/create, cookbook settings, and the login page.
- Plain CSS or a minimal utility layer. No heavy UI framework.

### Database: SQLite

We chose **SQLite** over MongoDB:

- **No extra service to run.** The database is a single file inside the backend process, which keeps hosting and backups trivial (copy one file plus the photos folder).
- **The data is small and simple.** A household cookbook holds hundreds of recipes, not millions. Ingredients, instructions and tags fit naturally in a few tables, or in JSON columns where that is simpler.
- **Search is built in.** SQLite's FTS5 extension covers searching by name.
- **Go support is good.** For example, `modernc.org/sqlite` is pure Go, so no CGO is needed.

The storage layer sits behind a small Go interface, so switching to another database later (e.g. PostgreSQL or MongoDB) won't affect the API or the frontend.

### Users, cookbooks and access

The app is self-hosted for family and friends, and may be reachable from the internet.

- **Accounts are created by the server admin,** the person running the server, from the command line (below). There is no public sign-up, and the server admin isn't a role inside the app.
- **Users log in** with a username and password and stay logged in through a session cookie. Passwords are hashed with argon2id; sessions are stored on the server, so logging out or removing a user ends them immediately; repeated failed logins are slowed down.
- **Cookbooks have two roles.** Whoever creates a cookbook becomes its **admin**. Admins invite existing users by username, remove members, change roles and delete the cookbook; **editors** create, edit and delete its recipes. A cookbook can have several admins, but always at least one: the last admin can't leave or be demoted. Anyone else can leave a cookbook.
- **Cookbooks are private to their members.** For anyone else, a cookbook and its recipes don't exist: the API answers `404`, not `403`, so cookbooks can't be discovered.
- **Removing an account never leaves a cookbook without an admin.** Cookbooks the user was the only member of are deleted with their recipes; where they were the last admin, the longest-standing remaining member becomes admin. `cookbook user remove` lists what it changed. Recipes keep showing who created and last edited them, as history, even after that account is gone.

- **Members & settings.** Every member can see who's in a cookbook and leave it. Admins add existing accounts by username (they're added straight away, with no invitation to accept), change roles and remove members. Changes that would leave a cookbook without an admin are refused: the last admin must first make someone else admin, or delete the cookbook.

The work was split into increments, each starting with its API contract: 1. authentication, 2. cookbooks and membership, 3. member management. All three are done.

Accounts are managed with subcommands of the same binary, which run against the same database as the server (in Docker: `docker exec -it <container> cookbook user add <name>`):

```sh
cookbook serve                  # run the web app (default)
cookbook user add <username>    # asks for the password twice, without echo
cookbook user passwd <username> # set a new password; logs the user out everywhere
cookbook user list
cookbook user remove <username> # also logs the user out everywhere
```

Passwords are never passed as arguments, where they'd end up in shell history. On a terminal they're typed (twice, without echo); otherwise one line is read from standard input, for scripts: `docker exec -i <container> cookbook user add <name> < password.txt`. Passwords need at least 10 characters. Usernames are 1–64 characters out of letters, digits, `.`, `_` and `-`, and ignore case.

### Running locally

Requirements: Go 1.26+ and Node.js 22+ (or the devcontainer, below).

1. **Create an account** (once). It creates `backend/data/cookbook.db` if needed.

   ```sh
   cd backend
   go run ./cmd/cookbook user add <your name>
   ```

2. **Start the backend** (terminal 1):

   ```sh
   cd backend
   go run ./cmd/cookbook
   ```

3. **Start the frontend** (terminal 2), then open <http://localhost:5173>, log in and create a cookbook:

   ```sh
   cd frontend
   npm install   # first time only
   npm run dev
   ```

To try the API directly, log in once and keep the session cookie, with curl or a client such as Bruno or Postman (import `api/openapi.yaml`, base URL `http://localhost:8080/api`):

```sh
API=http://localhost:8080/api
curl -c session.txt -X POST $API/session -H 'Content-Type: application/json' \
  -d '{"username": "<your name>", "password": "<your password>"}'

curl -b session.txt $API/cookbooks                          # your cookbooks
curl -b session.txt -X POST $API/cookbooks -H 'Content-Type: application/json' \
  -d '{"name": "Family"}'                                    # create one; note its "id"
curl -b session.txt $API/cookbooks/<id>/recipes             # its recipes
curl -b session.txt -X POST $API/cookbooks/<id>/recipes -H 'Content-Type: application/json' -d '{
  "name": "Pancakes", "ingredients": ["200 g flour"], "steps": ["Fry."], "tags": ["breakfast"]
}'                                                           # add a recipe
curl -b session.txt -X DELETE $API/session                   # log out
```

To start over with an empty database, stop the backend and delete `backend/data/`.

To work on the frontend without the backend, run `npm run dev:mock` instead of steps 1 and 2. The API is then mocked in the browser, with sample data that resets on page reload: you're admin of a "Family" cookbook and an editor of "Friends". Log in with any username and the password `cookbook password`.

### Development

**Devcontainer (recommended).** `.devcontainer/` describes the whole environment: Ubuntu 24.04 (as in CI) with pinned Go and Node, and Playwright's Chromium and WebKit with their system libraries baked into the image. It works with rootless Podman: your user ID is kept inside the container, so files stay yours.

- VS Code: "Dev Containers: Reopen in Container" (with Podman, set `dev.containers.dockerPath` to `podman`).
- CLI: `npx @devcontainers/cli up --workspace-folder .`, then `npx @devcontainers/cli exec --workspace-folder . <command>`.

The first build takes a few minutes, mostly the browsers; creating a container from the cached image then takes under a minute.

When driving the host's Podman from inside a toolbox (e.g. through a `flatpak-spawn --host podman` wrapper), creating a new container can stall right after "Container started": `podman events` output doesn't make it through `flatpak-spawn`, and the CLI waits for it. Stop it and run `up` again; the existing container is picked up and setup continues.

Each package is self-contained and uses its own tools.

**Backend** (Go 1.26+), from `backend/`:

```sh
go run ./cmd/cookbook # API on :8080 (env: ADDR, DATA_DIR, default "data")
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

**End-to-end tests** (Node.js 22+), from `e2e/`. They drive the real app in phone-sized Chromium (Android) and WebKit (iPhone), through the real server and database. CI runs them against every image before publishing it. Each test creates uniquely named recipes and deletes them afterwards, so any running instance works, but prefer a scratch one. The tests log in as two accounts, which must exist on that server: `E2E_USERNAME` / `E2E_PASSWORD` (default `e2e` / `e2e test password`), and a second person for the sharing tests, `E2E_FRIEND_USERNAME` / `E2E_FRIEND_PASSWORD` (default `e2e-friend` / `e2e friend password`):

```sh
npm install                              # once
npx playwright install chromium webkit   # once; downloads browsers to ~/.cache/ms-playwright
(cd ../backend && echo 'e2e test password' | go run ./cmd/cookbook user add e2e)            # once per database
(cd ../backend && echo 'e2e friend password' | go run ./cmd/cookbook user add e2e-friend)
BASE_URL=http://localhost:8080 npx playwright test          # against the image or `go run`
BASE_URL=http://localhost:5173 npx playwright test          # or through Vite
npx playwright test --project=android    # one browser only
npm run report                           # HTML report of the last run
```

Playwright's WebKit is built for Ubuntu and needs library versions that other distributions (e.g. Fedora) don't ship, so outside the devcontainer it may not start; use the devcontainer, or `--project=android` and let CI run both.

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

**First account.** After the first start, create your account inside the container: `docker exec -it cookbook cookbook user add <your name>`.

**HTTPS.** Put the app behind a reverse proxy that provides HTTPS (e.g. Caddy) before letting people log in from elsewhere. The session cookie is marked `Secure`, so browsers only send it over HTTPS, except when the app is reached as `localhost`. If you must serve plain HTTP under another name (e.g. a LAN address), set `INSECURE_COOKIES=true`, knowing that passwords and sessions then travel unencrypted.

### Backups

Don't copy `cookbook.db` while the app runs: recent changes live in `cookbook.db-wal` until SQLite folds them in, so a plain copy can be stale or corrupt. Instead, `cookbook backup` writes a consistent snapshot while the app keeps serving, checks it, and streams it out of the container:

```sh
docker exec cookbook cookbook backup - > cookbook-$(date +%F).db
```

The result is a single, self-contained SQLite file (no `-wal`/`-shm` companions).

**Nightly, off the machine.** On the server, a systemd timer can take a snapshot and send it elsewhere with [restic](https://restic.net) (encrypted, deduplicated, with retention rules). This assumes a restic repository already initialised (`restic init`), with its location and password in `/etc/cookbook-backup.env` (`RESTIC_REPOSITORY=...`, `RESTIC_PASSWORD=...`):

```sh
#!/bin/sh
# /usr/local/bin/cookbook-backup
set -eu
dir=/var/backups/cookbook
file="$dir/cookbook-$(date +%F).db"
trap 'rm -f "$file.tmp"' EXIT
mkdir -p "$dir"
docker exec cookbook cookbook backup - > "$file.tmp"   # only a complete backup
mv "$file.tmp" "$file"                                  # gets the final name
find "$dir" -name 'cookbook-*.db' -mtime +14 -delete    # local copies: two weeks
restic backup "$dir"
restic forget --prune --keep-daily 7 --keep-weekly 4 --keep-monthly 12
```

```ini
# /etc/systemd/system/cookbook-backup.service
[Unit]
Description=Back up the Family Cookbook database

[Service]
Type=oneshot
EnvironmentFile=/etc/cookbook-backup.env
ExecStart=/usr/local/bin/cookbook-backup
```

```ini
# /etc/systemd/system/cookbook-backup.timer
[Unit]
Description=Back up the Family Cookbook database daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Enable it with `systemctl enable --now cookbook-backup.timer`. Keep at least one copy off the server (another machine, cloud storage), and back up `/data/photos` the same way once photos exist.

**Restoring.** Stop the app, put the backup in place, and remove the old `-wal`/`-shm` files, which would otherwise be replayed on top of it:

```sh
docker stop cookbook
docker run --rm -v cookbook-data:/data -v "$PWD":/backup:ro alpine sh -c \
  'rm -f /data/cookbook.db-wal /data/cookbook.db-shm &&
   cp /backup/cookbook-2026-10-22.db /data/cookbook.db &&
   chown 65532:65532 /data/cookbook.db'
docker start cookbook
```

On SELinux hosts (e.g. Fedora), mount the backup folder with `:ro,Z`. After a restart, wait for `/api/health` to answer before checking the data. Try a restore now and then, e.g. into a scratch volume: a backup you've never restored is a guess.

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
