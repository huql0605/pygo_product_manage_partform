# Copilot instructions

## Repository layout and architecture

- This repository contains two services, not a single application module:
  - `go_api/` is a Go module. `cmd/server/main.go` loads `configs/configs.yaml` relative to the current working directory, opens PostgreSQL, runs GORM migrations for the product and sales models, and starts the Gin server.
  - `python_code/` is a Python project. `app/main.py` creates the FastAPI application and mounts the analytics router; run it with `python_code` as the working directory so `app` imports resolve.
- The Go API owns product endpoints and proxies analytics endpoints to the Python service. Its router wires Gin handlers to product service/repository layers backed by GORM, and to an analytics HTTP client configured with the Python base URL.
- The Python analytics router delegates to `app/modules/analytics/service.py`, where Pandas reads SQL query results through a shared SQLAlchemy engine. Both services use PostgreSQL; the Go `Sales` model and the Python analytics queries must stay consistent with the `sales` table.
- Request IDs span both services: Go middleware creates or accepts `X-Request-ID`, stores it in the request context, and the analytics client forwards it. Python middleware stores the header in a `ContextVar` for logging and returns it in the response. Keep Go middleware order intact: request-ID middleware must run before the logger.
- The root CSV files are inputs for the Python import scripts. Import scripts are separate from either server's startup path.

## Build, test, and run

Run Go commands from `go_api/`:

```sh
go build ./...
go test ./...
go test ./internal/product
go test ./internal/product -run '^TestName$' -count=1
go vet ./...
gofmt -w path/to/changed.go
go run ./cmd/server
```

Replace `TestName` with the exact Go test function name. There are currently no test files in the repository. The Go server expects PostgreSQL and the Python analytics service configuration to be available when run.

Run Python commands from `python_code/`:

```sh
python -m pip install -r requirements.txt
python -m uvicorn app.main:app --reload
```

There are currently no Python test or lint configuration files or test files. The API needs a reachable PostgreSQL database for analytics queries.

## 凭据

- 任何被 git 跟踪的文件（文档、示例配置、脚本、测试、注释）都不得出现真实的数据库用户名或密码。
- 示例与文档里只写占位符：`change_me`、`<db_user>`、`$DB_USER`。
- 真实值只放在 `.env`（已被 `.gitignore` 忽略）或部署环境变量中。
- `configs/configs.yaml` 的 `database.user` / `database.password` 只是兜底默认值，运行时必须由 `DB_USER` / `DB_PASSWORD` 覆盖。
- `.mcp.json` 的 `POSTGRES_CONNECTION_STRING` 同样只从环境变量读取。

## MCP

- `.mcp.json` configures the PostgreSQL MCP server in restricted (read-only) mode. It requires `uvx` and the `POSTGRES_CONNECTION_STRING` environment variable; keep the connection string out of tracked files.

## Codebase conventions

- In Go, keep HTTP concerns in Gin handlers, application logic in the service, and database operations in repositories. Route registration and dependency wiring belong in `internal/router`.
- Use the shared response helpers in `internal/common/response` for product API responses, and the common error codes in `internal/common/errors` rather than introducing a competing envelope.
- Product request payloads are defined separately as DTOs with JSON and Gin binding tags; database models carry JSON and GORM tags. Keep validation at the request/service boundary and persistence details in the repository.
- For downstream analytics calls, use request contexts and `url.Values` query encoding through the existing analytics client. Preserve request-ID propagation through Go's standard `context.Context`.
- In Python, keep endpoint parameter validation and routing in `app/api`, and database-backed analytics calculations in `app/modules/analytics/service.py`. Use the shared engine from `app/db.py`.
- Python settings are centralized in the `settings` instance from `app/config.py` and read from `.env` plus defaults. Go settings are loaded from the YAML config through `internal/config`; the relative config path means the server must be started from `go_api/`.
- Analytics SQL currently uses named bound parameters for request values. Continue binding values rather than interpolating request input into SQL; only construct SQL fragments for fixed, code-controlled query structure.
