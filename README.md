# Currency Rates Service

An asynchronous, high-throughput currency exchange and valuation engine built in Go. The application utilizes an internal memory queue (Go channels) backed by a worker pool pattern to handle incoming price updates asynchronously, persisting transaction states and price caches into PostgreSQL.

## Architecture Overview

The application splits heavy rate-fetching processing away from the client request-response lifecycle:

1. API Layer: Client requests a rate update via POST /rates/update. The request is validated, a stateful tracking record is stored as pending in the DB, and a task is quickly dispatched to an internal memory channel. The API immediately hands a 202 Accepted status and tracking UUID back to the client.
2. Worker Pool: A pool of concurrent background workers listens on the task channel. Workers consume tasks, fetch live exchange metrics from external valuation marketplaces, update the job tracking status to completed, and seed the final price into the cached latest_prices table.
3. Polling/Query Layer: Clients can query their specific job processing metrics via GET `/rates/job/{id}` or immediately query the highly optimized, finalized pricing cache via GET `/rates/latest/{pair}`.

## Directory Structure
```shell
.
├── Dockerfile                   # Multi-stage production Go compiler configuration
├── README.md                    # System documentation
├── api
│   └── swagger.yaml             # OpenAPI/Swagger contract definitions
├── docker-compose.yml           # Application, database, and orchestration layer profiles
├── endpoints                    # Core HTTP Handlers and Worker Logic Package
│   ├── Env.go                   # Central structural environment definitions and receivers
│   ├── channel
│   │   ├── ProcessCurrencyUpdate.go  # Core third-party fetch/update engine transactions
│   │   └── StartBackgroundWorker.go  # Concurrent worker channel listener loops
│   ├── handleGetJobStatus.go    # Handler for GET /rates/job/{id}
│   ├── handleGetLatestQuote.go  # Handler for GET /rates/latest/{pair}
│   └── handleUpdateQuote.go     # Handler for POST /rates/update
├── go.mod
├── go.sum
├── integration-tests            # Isolated E2E Integration Suite Workspace
│   ├── main_test.go             # Global suite controller (Native Docker lifecycle orchestration)
│   ├── job_status_test.go       # Lifecycle assertions for job processing states
│   ├── latest_quote_test.go     # Asynchronous worker verification loops via polling
│   └── update_quote_test.go     # Edge-case payload validation boundaries tests
├── main.go                      # Application bootstrap entryway
├── migrations
│   └── 000001_init_schema.up.sql # Raw PostgreSQL table structures
├── run-docker-app.sh            # Utility script to launch the app stack
├── run-tests.sh                 # Utility script to run the isolated integration tests
├── runMigrations.go             # Embedded migrations executor
├── structs                      # Shared Data Models/Domain API payloads
│   ├── ExchangeRateAPIResponse.go
│   ├── JobStatusResponse.go
│   ├── LatestQuoteResponse.go
│   ├── Task.go
│   ├── UpdateRequest.go
│   └── UpdateResponse.go
└── util
└── initDbClient.go          # Stateful pgxpool client initializer
```

## API Documentation

The complete API specifications are documented via OpenAPI. You can view the full schemas inside `api/swagger.yam`l or fetch it directly from the running instance at `/swagger.json`.

| Method | Endpoint | Description | Status Code |
| :--- | :--- | :--- | :--- |
| **POST** | `/rates/update` | Submits a currency pair update request to the async queue. | `202 Accepted` |
| **GET** | `/rates/job/{id}` | Inspects runtime processing metrics/status for a specified job. | `200 OK` / `404` |
| **GET** | `/rates/latest/{pair}` | Fetches the current, cached pricing for a URL-encoded pair (e.g., `EUR%2FUSD`). | `200 OK` / `404` |

## Getting Started

### Prerequisites

1. Docker and Docker Compose installed locally.

2. Go 1.22+ (only required if running components outside of Docker natively).

## Running the Application

To boot up the database, execute database migrations automatically, build the Go binaries, and start the background worker routines, run the provided utility script:

````sh
chmod +x run-docker-app.sh
./run-docker-app.sh
````

Alternatively, run it manually via Docker Compose:

```sh
docker compose up -d --build
```

The application container will start listening for API traffic on port `8080`.

## Testing Pipeline

The service utilizes an isolated integration testing matrix found under the `/integration-tests` workspace folder.

### Integration Test Lifecycle Mechanic

The integration engine does not use heavy external frameworks. Instead, `integration-tests/main_test.go` leverages native Go `os/exec` hooks to build and start your application and database containers cleanly on a dedicated docker network network layer.

1. `TestMain` fires up `docker compose up -d --build`.

2. A defensive health check loop blocks until the application layer is healthy.

3. Go automatically executes all test functions across `update_quote_test.go`, `job_status_test.go`, and `latest_quote_test.go` sequentially against real server ports.

4. An absolute defer block executes `docker compose down -v` to destroy test tables, data volumes, and bridge configurations securely.

### Running the Tests Quietly

To execute the full integration lifecycle (with verbose Go metrics while silencing standard Docker builder logs), run:

```shell
chmod +x run-tests.sh
./run-tests.sh
```

Or execute it directly from the project root directory context using standard toolchain flags:

```shell
go test -v -count=1 ./integration-tests/...
```
