#!/usr/bin/env bash
# Installs the project's dependencies once, when the devcontainer is created.
set -euo pipefail
cd "$(dirname "$0")/.."

(cd backend && go mod download)
(cd frontend && npm ci)
# The image already has the browsers; this only downloads them if
# e2e/package.json has moved to a newer Playwright than the image.
(cd e2e && npm ci && npx playwright install chromium webkit)
