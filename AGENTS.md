# Repository Guidelines

## Plans
Refer to .plans/stories/ when looking for implementation details for stories

## Project Structure & Module Organization (Go)
- Keep production code in `src/` using standard Go package layout.
- Controllers live under `src/controllers/<function>/controller.go` (one package per function area).
- Core/shared primitives in `src/core/` (enums, constants, helpers).
- External integrations/adapters in `src/adapters/` (Kubernetes clients, webhooks, metrics, events).
- API structures live under `src/` in their usual packages (e.g., `src/api/...` or `src/<domain>/...`) consistent with Go module/package norms.
- Keep configuration defaults/manifests in `configs/`, long‑form docs in `docs/`, and reusable prompt assets in `assets/`.
- Tests mirror package layout using `_test.go` files colocated with code, with additional fixtures in `tests/fixtures/` when appropriate.

## Build, Test, and Development Commands (Go)
- `go mod tidy` — ensure module deps are up to date.
- `go build ./...` — build all packages.
- `go test ./...` — run unit tests; add `-run Name` to filter.
- `golangci-lint run` — lint the codebase (configure in `.golangci.yml`).
- `gofmt -s -w .` or `go fmt ./...` — format code before committing.
- If using code generation (CRDs, deep copy, etc.), document commands in `docs/development.md` and wire via `make generate`.
- Tests should be kept with the structures they are testing
- Ensure there are tests for everything to aim at 100% code coverage

## Coding Style & Naming Conventions (Go)
- Target Go 1.21+ and follow Effective Go and Go Code Review Comments.
- Package names are short, lowercased, and meaningful (`core`, `adapters`, `controllers`, `api`).
- Filenames use lowercase with underscores only when helpful; prefer concise names (`controller.go`, `validation.go`).
- Exported identifiers use CamelCase; unexported identifiers use lowerCamelCase. Constants use PascalCase when exported.
- Keep comments focused on intent; add inline comments for non‑obvious concurrency or side effects. Use GoDoc style for exported types and functions.

## Testing Guidelines (Go)
- Place unit tests in `_test.go` files alongside the code; use table‑driven tests where appropriate.
- Isolate external systems behind interfaces and fakes; keep integration tests separate and flagged.
- Maintain statement coverage ≥85% (`go test ./... -cover`); document any gaps in the PR.
- Store reusable fixtures under `tests/fixtures/` if not colocated; avoid brittle golden files unless stable.

## Commit & Pull Request Guidelines
- Use Conventional Commits (`feat(controller): add propagation reconcile`) so automation can assemble release notes.
- Rebase before opening a PR; describe intent, validation steps, new configuration, and follow‑up work in the template.
- Attach logs, transcripts, or screenshots when UX shifts; update docs (`AGENTS.md`, `docs/changelog.md`) alongside code.
- Ensure `go build`, `go test`, `golangci-lint`, and formatting pass locally; list the executed commands in the PR body checklist.

## Security & Configuration Tips
- Keep secrets out of version control; load via environment variables documented in `configs/README.md`.
- Flag tests that require network access and disable them in CI by default (use build tags or separate integration jobs).
- Record new dependencies and licenses in `docs/dependencies.md`; scrub user‑identifiable data from samples before committing.

## Controllers & API Layout
- Controllers: `src/controllers/<function>/controller.go` contains the Reconcile logic and related helpers for that domain.
- Shared controller utilities (queues, backoff, predicates) can live in `src/controllers/internal/` or `src/core/` depending on reuse scope.
- API structures (Spec/Status types, constants, validation helpers) live under `src/` in domain‑appropriate packages (e.g., `src/api/configpropagation`, or `src/core` for shared constants).
- Manifests (CRDs, RBAC, deployment, webhooks) are stored in `configs/`.
