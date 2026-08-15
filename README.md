# Cervi

Cervi is a Wails 3 application targeting server, desktop, and mobile runtimes.
The server build uses PostgreSQL through Bun and applies embedded Goose
migrations during startup. Desktop and mobile storage will be added separately.

## Local server development

Requirements: Go, Wails 3, Node.js/npm, Docker, and Docker Compose.

Start PostgreSQL:

```sh
wails3 task db:up
```

Build and run the Wails server. The local task supplies the development
`DATABASE_URL` used by `docker-compose.yml` unless it is overridden:

```sh
wails3 task run:server
```

The application is available at <http://localhost:8080>. Server mode is built
with Wails' `server` build tag and does not create a native window.

To override the database or pool settings:

```sh
DATABASE_URL='postgres://user:password@host:5432/database?sslmode=require' \
POSTGRES_MAX_OPEN_CONNS=25 \
wails3 task run:server
```

Supported PostgreSQL environment variables:

| Variable | Default |
| --- | --- |
| `DATABASE_URL` | Required in server builds |
| `POSTGRES_MAX_OPEN_CONNS` | `25` |
| `POSTGRES_MAX_IDLE_CONNS` | `5` |
| `POSTGRES_CONN_MAX_LIFETIME` | `30m` |
| `POSTGRES_CONN_MAX_IDLE_TIME` | `5m` |
| `POSTGRES_STARTUP_TIMEOUT` | `1m` |

The process fails fast if PostgreSQL cannot be reached or a migration fails.
Migration SQL is embedded in the binary from
`internal/storage/postgres/migrations`.

Create the next sequential Goose migration with:

```sh
wails3 task db:migration:create NAME=create_users
```

Stop the local database without deleting its named volume:

```sh
wails3 task db:down
```

## Containers

Run PostgreSQL only (the default):

```sh
docker compose up -d --wait postgres
```

Or build and run the complete server stack:

```sh
docker compose --profile app up --build
```

The credentials in `docker-compose.yml` are for local development only. A
deployed server must receive `DATABASE_URL` from its secret/configuration
system.

## Desktop development

Desktop startup remains unchanged and does not require PostgreSQL:

```sh
wails3 dev
```
