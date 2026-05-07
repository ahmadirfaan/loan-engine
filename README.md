# Mini P2P Lending Engine

A hands-on Go project implementing a mini peer-to-peer lending engine with Clean Architecture, concurrent programming, raw SQL (no ORM), and Event-Driven Architecture via the Transactional Outbox Pattern.

**Loan state machine:** `PROPOSED → APPROVED → INVESTED → DISBURSED`

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| Web Framework | Gin |
| Database | PostgreSQL (raw `database/sql` + `pgx` driver — no ORM) |
| Messaging | RabbitMQ (amqp091-go) |
| Containerization | Docker / Podman |

---

## Project Structure

```
loan-engine/
├── cmd/server/          # Application entry point (main.go)
├── internal/
│   ├── config/          # Environment config loader
│   ├── constants/       # Shared constants
│   ├── domain/          # Domain entities and DTOs
│   ├── handler/         # HTTP handlers (Gin) + router
│   ├── mocks/           # Test mocks for all repository interfaces
│   ├── repository/      # Repository interfaces + PostgreSQL implementations
│   ├── service/         # Business logic (LoanService, ProductService)
│   └── worker/          # Outbox relay goroutine worker
├── pkg/
│   ├── database/        # PostgreSQL connection helper
│   └── rabbitmq/        # RabbitMQ client + Publisher interface
├── Dockerfile           # Multi-stage build (golang:alpine → alpine)
├── docker-compose.yml   # PostgreSQL + RabbitMQ + app services
├── init.sql             # Database schema + seed data
└── test_flow.sh         # End-to-end test script
```

---

## Prerequisites

### Running Unit Tests

- **Go 1.21+** installed and on your `PATH`
- No external services required

```bash
go test ./internal/... -cover
```

Expected: 100% coverage on `handler`, `service`, and `worker` packages.

---

### Running `test_flow.sh` (End-to-End)

The script boots the full stack from scratch, applies the database schema, and exercises the entire loan lifecycle via HTTP.

#### Required Tools

| Tool | Purpose | Install |
|------|---------|---------|
| **Docker** or **Podman** | Container runtime | See below |
| **curl** | HTTP requests | Pre-installed on macOS/Linux |
| **bash** | Script runner | Pre-installed |

#### Container Runtime Setup

The script is pre-configured for **Podman** (`COMPOSE_CMD="podman compose -p ..."`). To switch to Docker, open `test_flow.sh` and change the `COMPOSE_CMD` line near the top:

```bash
# Podman (default)
COMPOSE_CMD="podman compose -p ${COMPOSE_PROJECT}"

# Docker — change to this:
COMPOSE_CMD="docker compose -p ${COMPOSE_PROJECT}"
```

**Docker setup (first time):**
```bash
# Install Docker Desktop from https://www.docker.com/products/docker-desktop/
# Then start it — no further setup needed
docker compose version   # verify
```

**Podman setup (first time on macOS):**
```bash
brew install podman podman-compose
podman machine init
podman machine start
podman compose version   # verify
```

#### Running the Script

```bash
chmod +x test_flow.sh
bash test_flow.sh
```

The script will:

1. Tear down any existing containers and volumes (clean slate)
2. Build the Go app image and start PostgreSQL, RabbitMQ, and the app
3. Wait for the API to be ready at `http://localhost:8080`
4. Run the following test scenarios:

| # | Scenario | Expected Result |
|---|----------|----------------|
| 1 | Many investors fund a loan to 100% | `PROPOSED → APPROVED → INVESTED → DISBURSED` |
| 2 | Single investor funds a loan to 100% | `PROPOSED → APPROVED → INVESTED → DISBURSED` |
| 3 | Partial investment | Loan stays in `APPROVED` |
| NEG-1 | Re-approve a disbursed loan | HTTP 422 |
| NEG-2 | Invest in a disbursed loan | HTTP 422 |
| NEG-3 | Non-staff user attempts approval | HTTP 422 |

#### Ports Used

| Service | Port |
|---------|------|
| App (Gin) | `8080` |
| PostgreSQL | `5432` |
| RabbitMQ AMQP | `5672` |
| RabbitMQ Management UI | `15672` (guest/guest) |

---

## API Reference

All endpoints are prefixed with `/api/v1`.

### Products

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/products` | Create a loan product |

### Loans

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/loans` | Create a loan (status: `PROPOSED`) |
| `GET` | `/loans?state={state}` | List loans by state (`proposed`, `approved`, `invested`, `disbursed`) |
| `POST` | `/loans/:id/approve` | Approve a loan (staff only, multipart: `staff_id` + `picture`) |
| `POST` | `/loans/:id/invest` | Record an investment (JSON: `investor_id`, `amount`) |
| `POST` | `/loans/:id/disburse` | Disburse a loan (staff only, multipart: `staff_id` + `agreement`) |

### Loan Response Shape

```json
{
  "id": 1,
  "borrower": { "name": "Alice Borrower", "email": "alice@example.com" },
  "product": {
    "product_name": "Personal Loan Gold",
    "tenor_length": 12,
    "interest_rate": 10.5,
    "roi_rate": 8
  },
  "principal_amount": 10000,
  "remainder_amount": 3000,
  "status": "APPROVED",
  "have_invested": [
    { "amount": 3000, "investment_date": "2026-05-07T15:26:16.09Z" },
    { "amount": 4000, "investment_date": "2026-05-07T15:26:16.15Z" }
  ],
  "date_approval": "2026-05-07T15:26:16.02Z",
  "date_disbursed": null
}
```

---

## Key Design Decisions

### Transactional Outbox Pattern
When a loan becomes fully invested, an `order_event` record is inserted in the **same SQL transaction** as the loan status update. After commit, the service attempts an immediate RabbitMQ publish. If RabbitMQ is unavailable, the event stays `PENDING` and the outbox relay worker retries every minute.

### Optimistic Locking
The invest endpoint uses a `version` field: `UPDATE loan SET ... WHERE id = $1 AND version = $2`. Concurrent investments on the same loan are safe — only one writer wins per version increment.

### No ORM
All database access uses raw `database/sql` with hand-written SQL. The `pgx` driver is used for its PostgreSQL-specific features (e.g., `pgx/stdlib` adapter).

### Staff Role Validation
`/approve` and `/disburse` endpoints validate that the provided `staff_id` belongs to a user with `role = STAFF`. Non-staff requests are rejected with HTTP 422.
