# Contributing to free-api-hunter

## Commit Convention
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`, `ci:`, `build:`
- Write in English
- Subject line: imperative mood, no period, max 72 chars
- Body: explain WHY, not WHAT

## Before Committing
- Run tests: check project README for test command
- No files larger than 10MB
- No .env, credentials, secrets, node_modules, build artifacts
- Always use `bin/lab-commit.sh <agent> -m "message"` for commits

## Git Rules
- Branch from `main`, prefix with `feat/`, `fix/`, `chore/`
- Squash before merge to main
- No direct commits to main (use PR or lab-commit.sh)

## Security
- Never commit secrets, tokens, passwords, keys
- Use environment variables or vault for secrets
