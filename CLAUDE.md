# Claude Code Instructions

## Git Workflow

- **Never amend commits** - always create new commits for fixes
- **Let the user handle branch management** - don't create, switch, or manage branches
- Use conventional commit messages
- Run tests before committing when applicable
- **Commit after every turn** - always commit your work at the end of each turn

## Project Structure

- `cmd/agentsv/` - Go server entrypoint
- `cmd/testfixture/` - Test data generator
- `internal/` - Go packages (config, db, parser, server, sync, web)
- `frontend/` - Svelte 5 SPA (Vite, TypeScript)

## Development

```bash
make build      # Build binary with embedded frontend
make dev        # Run Go server in dev mode
make frontend   # Build frontend SPA only
```

## Testing

**All new features and bug fixes must include unit tests.** Run tests before committing:

```bash
make test       # Go tests
make e2e        # Playwright E2E tests
make lint       # golangci-lint
```

### Test Guidelines

- Table-driven tests for Go code
- Use `testDB(t)` helper for database tests
- Frontend: colocated `*.test.ts` files, Playwright specs in `frontend/e2e/`
