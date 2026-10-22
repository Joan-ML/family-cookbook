# One image for the whole app: the Go binary with the built frontend embedded.
# Build from the repository root:
#   docker build --build-arg VERSION=$(git describe --tags --always --dirty) -t family-cookbook .

# Shown in the UI footer, /api/health and the logs. CI sets it from the git tag.
ARG VERSION=dev

# --- Frontend ----------------------------------------------------------------
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ARG VERSION
RUN VITE_APP_VERSION=$VERSION npm run build \
    # The MSW worker is only for `npm run dev:mock`.
    && rm -f dist/mockServiceWorker.js

# --- Backend -----------------------------------------------------------------
FROM golang:1.26-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/dist ./internal/web/dist
ARG VERSION
RUN CGO_ENABLED=0 go build -tags embedui -trimpath \
        -ldflags="-s -w -X family-cookbook/backend/internal/version.Version=$VERSION" \
        -o /out/cookbook ./cmd/server \
    && mkdir /out/data

# --- Runtime -----------------------------------------------------------------
# Distroless: no shell or package manager, runs as an unprivileged user.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=backend /out/cookbook /usr/local/bin/cookbook
COPY --from=backend --chown=nonroot:nonroot /out/data /data
ENV ADDR=:8080 \
    DATA_DIR=/data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["cookbook"]
