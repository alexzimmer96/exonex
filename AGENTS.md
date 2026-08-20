# Exonex

Exonex is a Go backend service exposing ConnectRPC (protobuf) APIs backed by PostgreSQL. It uses the "cortex" domain model (documents, artifacts, analysis; publishers is currently a DB-only reference table) and an RBAC-style auth layer implemented as Connect interceptors.

## Repository layout

- `api/` — Protobuf source files (buf module, path `api`). Two packages: `exonex.cortex.v1alpha1` (services) and `exonex.api.v1` (annotations). Hand-maintained; change these to change the API.
- `pkg/api/` — **Generated** Connect/protobuf Go code (committed). Do not edit; regenerate via buf.
- `internal/cortex/` — Application code, layered as `handler/` (Connect handlers) → `domain/` (services) → `repository/` (pgx + jet against Postgres). `server.go` wires everything into an `http.ServeMux`.
- `internal/auth/` — Auth context, roles, permissions, and Connect interceptors.
- `internal/dbschema/` — **Generated** by jet from the live DB into `internal/dbschema/exonex/cortex/` (model + table packages). Do not edit; regenerate after migrations change the schema.
- `cmd/main.go` — Binary entrypoint. Opens a pgx pool and starts the Cortex server on `:8080`.
- `database/migrations/` — Goose SQL migrations, embedded via `database/migrations.go`; helper runner in `pkg/migrations.go`.
- `pkg/` — Shared utilities: `grpc/` (field-mask + protovalidate interceptors), `sql/` (CEL→SQL filter builder, transactions), `jsonschemas/` (annotation JSON schemas), plus schema/mime/mapping/http helpers.
- `dist/images/` — Dockerfiles (custom Postgres 18 image with pgvector, pg_partman, pg_jsonschema).
- `web/admin/` — Placeholder for an admin frontend (untracked in git; won't exist in a fresh clone).
- `docs/` — **Separate Go/npm module** (Hugo + Docsy site, own `go.mod` and `package.json`). User-facing docs in `docs/content/en/`. `docs/data/api.json` is **generated** from the protos via protoc-gen-doc; do not edit.

## Prerequisites

- Go 1.26+ (required `go tool` directive is used for buf, goose, jet, sqlc, protoc-gen-doc — no global installs needed)
- Docker Compose (runs Postgres + RustFS)
- `go task` (Taskfile) for convenience; every task is a thin wrapper around `go tool` commands (plus `git add .` in the codegen tasks), so raw commands work too
- `pg_format` (optional, for `format:sql`)

## Setup

```sh
docker compose up -d          # postgres (5432, user/pass: exonex) + rustfs S3 (9000/9001)
task db:reset                 # or: task db:up  (runs goose migrations)
go run ./cmd                  # serves on :8080
```

Dev DSN: `postgres://exonex:exonex@localhost:5432/exonex`

Note: `cmd/main.go` does **not** run migrations on startup — apply them with `task db:up` first (or call `pkg.Migrate` with the embedded `database.Migrations`).

## Code generation

| What | Command | Output |
|---|---|---|
| Protos → Go (committed) + `docs/data/api.json` | `task proto:gen` | `rm -rf pkg/api`; regenerates `pkg/api` (both packages) and `docs/data/api.json` |
| Live DB → jet schema | `task db:gen` | regenerates `internal/dbschema` |
| New migration | `task db:migration:create <name>` | new goose file in `database/migrations` |

- `proto:gen` runs `rm -rf pkg/api`, `go tool buf dep update`, `go tool buf generate`, and **auto-commits with `git add .`** — run it only when you intend to commit the generated changes.
- Regenerate jet (`db:gen`) after any migration alters the `cortex` schema, and before building code that references changed columns.

## Conventions

- **Never hand-edit generated code**: `pkg/api/**`, `internal/dbschema/**`, `docs/data/api.json`. Change sources (`api/**`, migrations) and regenerate.
- Layering: Connect handlers (`internal/cortex/handler`) talk to domain services (`internal/cortex/domain`), which talk to repositories (`internal/cortex/repository`). Repositories use pgx v5 and jet-generated tables; keep SQL in repositories, business rules in domain.
- Query filtering uses the CEL→SQL `pkg/sql.FilterBuilder`; validation uses protovalidate via `pkg/grpc` interceptors. Auth/authorization goes through `internal/auth` interceptors, not per-handler checks.
- New domain entities: add proto in `api/exonex/cortex/v1alpha1/`, migration in `database/migrations/`, then regenerate, then add table usage in `repository/`, `domain/`, `handler/` and register in `internal/cortex/server.go`.
- Logging: `log/slog` with context (`slog.ErrorContext`/`InfoContext`).
- Migrations: goose, one file per change, always `BEGIN;`/`COMMIT;`, with a `-- +goose Down` section. Use the `cortex.` schema prefix.

## SQL

Run `task format:sql` (or `format:sql:file <file>`) after editing `.sql` files; it applies the standard `pg_format` flags (uppercase keywords/types/functions, 2-space indent).

## Testing

No test suite exists yet. Use plain `go test ./...` (standard library + testify is available in go.mod). The `docs/` workspace is a separate module — run go/npm commands from inside `docs/`.

## Gotchas

- The Postgres data lives in the named volume `cortex-data` (docker-compose); `task db:down`/`db:reset` use goose reset against it.
- `docs/` has its own `go.mod`, `package.json`, and `node_modules`; don't treat it as part of the root Go module.
- Empty directories (`web/admin`, `internal/cortex/controller/`, `adapter/`, `app/`) are not tracked by git and won't appear in a fresh clone; `web/admin` is the placeholder for the future admin UI.
