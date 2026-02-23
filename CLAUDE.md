# Beads

Beads is an event-driven work-tracking system. It manages hierarchical work items ("beads") with dependencies, labels, comments, and event history. The system exposes both gRPC and REST APIs and ships a Cobra-based CLI client (`kd`).

## Key concepts

- **Bead** — the core work item. Has a kind (`issue`, `data`, `config`), a type (`epic`, `task`, `feature`, `chore`, `bug`, or custom), a status (`open`, `in_progress`, `blocked`, `deferred`, `closed`), and optional metadata, labels, dependencies, and comments.
- **Store** — persistence interface (`internal/store/store.go`) with a PostgreSQL implementation. All mutations are wrapped in transactions and recorded as events.
- **Events** — every mutation records an event row in Postgres and publishes to an event bus via the `Publisher` interface (`internal/events`). Publishing is optional; a no-op publisher is used when no bus is configured.
- **IDs** — nanoid format, prefixed `kd-` (see `internal/idgen`).
- **Type configuration** — bead types are extensible. Five issue types are built in; custom types are registered via `SetConfig` with key `type:<name>`. Config is defined by `TypeConfig` / `FieldDef` in `internal/model/type_config.go`.

## Directory structure

```
kbeads/
├── cmd/kd/              # CLI client (Cobra); one file per command
├── internal/
│   ├── config/          # Env-var configuration (BEADS_DATABASE_URL, etc.)
│   ├── events/          # Publisher interface and implementations
│   ├── idgen/           # Nanoid-based ID generation
│   ├── model/           # Domain types: Bead, Dependency, Comment, Event, Filter
│   ├── server/          # gRPC + HTTP server, proto ↔ model conversion, interceptors
│   └── store/           # Store interface + postgres/ implementation with migrations
├── proto/beads/v1/      # Protobuf service and message definitions
├── gen/beads/v1/        # Generated Go code from proto (do not edit)
├── Dockerfile           # Multi-stage build → /usr/local/bin/kd
├── quench.toml          # Quench quality-check config
└── go.mod
```

## Build & test

```sh
go build ./cmd/kd                # build the CLI
go test ./...                    # run all tests (uses sqlmock; no external deps)
```

Database tests use `go-sqlmock`; no running Postgres instance is needed for `go test`.

## Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `BEADS_DATABASE_URL` | *(required)* | Postgres connection string |
| `BEADS_GRPC_ADDR` | `:9090` | gRPC listen address |
| `BEADS_HTTP_ADDR` | `:8080` | HTTP listen address |
| `BEADS_NATS_URL` | *(optional)* | Event bus URL |
| `BEADS_SERVER` | `localhost:9090` | CLI client target address |

## Commits

Use short, imperative subject lines. Scope in parentheses when it helps: `fix(store): handle nil metadata on update`.

## Landing the plane

When finishing work on this codebase:

1. **Run tests** — `go test ./...` must pass.
2. **Run quench** — `quench check` must pass (cloc, docs, agents checks are configured).
3. **Keep generated code in sync** — if you modify `.proto` files under `proto/`, regenerate `gen/` and commit both.
4. **Follow existing patterns** — the server layer converts between proto and model types via `internal/server/convert.go`; new RPCs should do the same. New CLI commands go in their own file under `cmd/kd/`.
5. **Record events** — any new mutation must call both `store.RecordEvent` and `publisher.Publish`.
6. **Migrations** — schema changes need a new numbered migration pair in `internal/store/postgres/migrations/`.
