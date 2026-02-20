#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

DB_PATH="$TMPDIR/sessions.db"
EMPTY_DIR="$TMPDIR/empty"
mkdir -p "$EMPTY_DIR"

# Build and run fixture generator
echo "Building test fixture..."
CGO_ENABLED=1 go build -tags fts5 \
  -o "$TMPDIR/testfixture" "$ROOT/cmd/testfixture"
"$TMPDIR/testfixture" -out "$DB_PATH"

# Build the server with embedded frontend
echo "Building server..."
cd "$ROOT/frontend" && npm run build
rm -rf "$ROOT/internal/web/dist"
cp -r "$ROOT/frontend/dist" "$ROOT/internal/web/dist"
CGO_ENABLED=1 go build -tags fts5 \
  -o "$TMPDIR/agentsview" "$ROOT/cmd/agentsview"

# Run server with test DB, no sync dirs, fixed port
echo "Starting e2e server on :8090..."
AGENT_VIEWER_DATA_DIR="$TMPDIR" \
CLAUDE_PROJECTS_DIR="$EMPTY_DIR" \
CODEX_SESSIONS_DIR="$EMPTY_DIR" \
GEMINI_DIR="$EMPTY_DIR" \
exec "$TMPDIR/agentsview" \
  -port 8090 \
  -no-browser
