.PHONY: test test-backend test-frontend gen-api check-api dev-backend dev-frontend dev-mock

test: test-backend test-frontend check-api

test-backend:
	cd backend && go vet ./... && go test ./...

test-frontend:
	cd frontend && npm run lint && npm test

# Regenerate frontend types from api/openapi.yaml.
gen-api:
	cd frontend && npm run gen:api

# Fail if the generated frontend types are out of date with the spec.
check-api: gen-api
	git diff --exit-code -- frontend/src/api/schema.d.ts

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

dev-mock:
	cd frontend && npm run dev:mock
