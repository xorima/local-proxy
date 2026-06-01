# Rules for opencode agents

## Before committing any code changes
1. Run `golangci-lint run ./...` and fix all issues
2. Run `govulncheck ./...` and fix any vulnerabilities
3. Run `go vet ./...` and fix all issues
4. Run `go test ./internal/...` and ensure all tests pass

## Code style
- Follow existing patterns in the codebase
- Do NOT add comments to code unless the logic is genuinely non-obvious
- Use `_ =` prefix for unchecked error returns (lint-mandated in this repo)
- Use BDD-style test naming: `it should do X when Y`
