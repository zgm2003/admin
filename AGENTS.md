# Project Instructions

Before changing code, read `docs/superpowers/specs/2026-08-17-admin-foundation-design.md` and the active plan under `docs/superpowers/plans/`.

## Non-Negotiable Rules

- Keep every data flow explicit and linear. Normal database requests follow `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`.
- Handler must not access GORM. Service must not depend on Gin. Repository only accesses PostgreSQL and never calls Queue. Queue only wraps Asynq and never imports a business module.
- Use concrete technical names such as `database`, `redis`, `queue`, and `storage`. Do not introduce `platform`, `infra`, generic adapters, Manager, Factory, BaseService, BaseRepository, DI containers, runtime registries, or speculative interfaces.
- Do not silently recover from missing configuration, malformed requests, protocol violations, unavailable dependencies, or incomplete task payloads. Return explicit errors.
- The HTTP envelope is exactly `code`, `data`, and `message`. Do not accept or emit compatibility fields such as `msg`.
- JSON and TypeScript names use lower camel case. Go exported names use PascalCase and initialisms such as `UserID`, `HTTPServer`, and `APIClient`. PostgreSQL names use lower snake case. URLs use lowercase plural resources. Permission codes use colon-separated segments.
- Every PostgreSQL model explicitly declares `CreatedAt` and `UpdatedAt` as non-null `TIMESTAMPTZ` fields. Do not hide them in `gorm.Model` or a BaseModel.
- Tables with real deletion behavior use `gorm.DeletedAt`/`deleted_at` as the only deletion state. Do not add `is_del`, `is_deleted`, or another duplicate flag.
- Project Yes/No codes are `0 = No` and `1 = Yes`, defined only in backend `shared/yesno` and frontend `enums/yes-no`; database fields use `SMALLINT` with a `CHECK` constraint when introduced.
- Pass `context.Context` from HTTP or Asynq entry points through Service, Repository, GORM, Redis, and other I/O calls. Never pass `*gin.Context` below Handler or Middleware.
- Create only files, fields, tables, indexes, states, and abstractions used by the current feature.
- Do not use Docker or Docker Compose for this project.
- Follow the current hardening plan for Git setup. Do not automatically create commits, fetch, pull, or push; the repository owner controls Git history.

## Verification

- Backend: `cd server; go fmt ./...; go vet ./...; go test ./...; go build ./...`
- Frontend: `cd web; pnpm vitest run; pnpm build`
