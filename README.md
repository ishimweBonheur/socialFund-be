# Social Fund backend

Social Fund is a Go and PostgreSQL backend with versioned SQL migrations, transaction-safe financial workflows, and a durable notification queue. Application startup never creates or changes the schema; migrations are the sole schema authority.

## Quick Start

Create the local environment file:

```sh
cp .env.example .env
```

PowerShell equivalent:

```powershell
Copy-Item .env.example .env
```

Start PostgreSQL, apply pending migrations, and start the API:

```sh
docker compose up -d
docker compose ps
```

The API is available at <http://localhost:8080> and database-aware health is available at <http://localhost:8080/healthz>.

Development tools:

- Swagger UI: <http://localhost:8080/swagger/index.html>
- Mailpit email inbox: <http://localhost:8025>

Compose starts services in this order:

```text
postgres healthy -> migrate exits 0 -> api starts
```

The `migrate` container is intentionally a one-shot job. If a migration fails, it exits nonzero and Compose does not start the API.

## Configuration

`DATABASE_URL` is the preferred database configuration. The example Docker value is:

```env
DATABASE_URL=postgres://app:password@postgres:5432/socialfund?sslmode=disable
```

Inside Compose, `postgres` is the database service's Docker DNS hostname. When running the Go API or migration CLI directly on the host while PostgreSQL remains in Docker, use the published host port instead:

```env
DATABASE_URL=postgres://app:password@localhost:5432/socialfund?sslmode=disable
```

Legacy `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASSWORD` variables remain supported only when all are supplied. Credentials belong in the ignored `.env` file and must not be committed.

Authentication also requires:

```env
FRONTEND_URL=http://localhost:3000
JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRATION=24h
GOOGLE_CLIENT_ID=your-google-web-client-id.apps.googleusercontent.com
```

SMTP delivery is configured with `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, and `SMTP_FROM`. Compose uses Mailpit for local email capture.

## Member Onboarding

An authenticated admin creates a member with:

```text
POST /api/v1/admin/users
```

The API transaction creates an `INACTIVE` member, active contribution plan, pending welcome notification, and `USER_CREATED` audit entry. Any failed insert rolls back the entire operation. The welcome link opens `${FRONTEND_URL}/login`; it never activates the account.

The frontend sends the Google-issued credential to:

```text
POST /api/v1/auth/google
```

The backend validates the Google signature, audience, issuer, and expiration using `GOOGLE_CLIENT_ID`. It uses only the verified `sub`, `email`, and `email_verified` claims. A matching inactive account is activated and bound to the Google subject inside a transaction, activation/login audits are written, and the Social Fund JWT is issued only after commit.

For system testing, migrations seed `ishimwebonheur078@gmail.com` as an administrator. Sign in with that verified Google account; its first successful login activates the account.

Protected requests use:

```http
Authorization: Bearer <social-fund-jwt>
```

All API errors use:

```json
{"error":{"code":"STRING_CODE","message":"Human-readable message"}}
```

## Operations

Inspect or follow logs:

```sh
docker compose logs postgres
docker compose logs migrate
docker compose logs -f api
```

Run migrations manually using the same image and CLI as startup:

```sh
docker compose run --rm migrate -direction up
docker compose run --rm migrate -direction down -steps 1
docker compose run --rm migrate -direction version
```

Omitting `-steps` from `-direction down` rolls back all migrations. No command automatically forces dirty migrations or resets data.

Stop containers while preserving PostgreSQL data:

```sh
docker compose down
```

Start them again with the existing database:

```sh
docker compose up -d
```

Delete all local PostgreSQL data only when an intentional reset is required:

```sh
docker compose down -v
```

Warning: `docker compose down -v` permanently deletes the local named database volume.

## Development Checks

Unit and compile checks run without a database. Financial integration tests run when `TEST_DATABASE_URL` is set to an already migrated disposable database:

```sh
go test ./...
go vet ./...
```

PowerShell example against the Compose PostgreSQL port:

```powershell
$env:TEST_DATABASE_URL = "postgres://app:password@localhost:5432/socialfund?sslmode=disable"
go test ./internal/service -count=1
```

The integration suite covers successful contribution approval, rollback after a late transaction failure, concurrent double approval, assistance rollback, and duplicate disbursement.

## Application Structure

Business code is grouped by feature under `internal/`:

```text
user/                 users and membership
contributionplan/     member contribution plans
contribution/         contribution lifecycle and approval
assistance/           assistance requests and disbursement
notification/         durable queue and delivery worker
fund/                 financial ledger writes
audit/                append-only audit writes
auth/                 verified identity and authorization boundaries
database/             pool and transaction infrastructure
```

The HTTP layer uses `chi`. Current route groups are:

```text
GET  /healthz
GET  /api/v1/users/{id}
POST /api/v1/users
GET  /api/v1/contribution-plans/users/{userID}/active
POST /api/v1/contribution-plans
POST /api/v1/contributions/{id}/approve
POST /api/v1/contributions/{id}/reject
POST /api/v1/assistance-requests/{id}/pay
```

Handlers perform HTTP decoding and response mapping only. Financial coordination remains in the contribution and assistance services.

## Index Rationale

- `contributions_user_due_date_idx` serves member history in due-date order.
- `contributions_status_due_date_idx` serves overdue scheduling; the partial pending index serves approval queues.
- `contribution_plans_one_active_per_user_idx` enforces and locates the sole active plan.
- Partial notification indexes keep pending and retry worker indexes small; `FOR UPDATE SKIP LOCKED` permits concurrent workers.
- `fund_transactions_direction_created_idx` serves direction/date reporting, while the created-at index serves unrestricted date ranges.
- Partial unique ledger indexes make contribution approval and assistance payout idempotent at the database boundary.
