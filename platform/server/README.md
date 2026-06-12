# Platform Server

CI/CD platform v1 backend for the existing `forum-api` Jenkins + ArgoCD delivery path.

The server intentionally keeps a narrow boundary:

- API accepts and validates release requests, writes PostgreSQL state, publishes RabbitMQ messages, and serves status queries.
- Worker consumes RabbitMQ messages and advances the asynchronous release state machine.
- PostgreSQL is the source of truth.
- RabbitMQ only distributes work.
- Jenkins remains the existing `forum-api-pipeline`.
- ArgoCD remains the deployment executor.
- The platform does not edit Helm values or Kubernetes Deployments directly.

## Build

```bash
cd platform/server
go test ./...
go build -o platform-server ./cmd/server
```

## Configuration

Required environment variables:

```bash
export DATABASE_URL='postgres://platform:platform@localhost:5432/platform?sslmode=disable'
export RABBITMQ_URL='amqp://guest:guest@localhost:5672/'
export JENKINS_BASE_URL='http://jenkins.cicd.svc.cluster.local:8080'
export JENKINS_USERNAME='admin'
export JENKINS_TOKEN='...'
```

Optional environment variables:

```bash
export HTTP_ADDR=':8080'
export SERVICE_CATALOG_PATH='configs/service-catalog.yaml'
export KUBECONFIG="$HOME/.kube/config"
export JENKINS_POLL_INTERVAL='10s'
export JENKINS_TIMEOUT='45m'
export ARGO_POLL_INTERVAL='10s'
export ARGO_TIMEOUT='10m'
export ROLLOUT_POLL_INTERVAL='5s'
export ROLLOUT_TIMEOUT='10m'
export RELEASE_LOCK_TTL='4h'
```

RabbitMQ defaults:

```text
exchange: platform.release.exchange
queue: platform.release.requested.queue
dlq: platform.release.dlq
routing key: release.requested
```

## Database

PostgreSQL access is implemented with GORM. Schema ownership stays in SQL migrations; the server does not call `AutoMigrate`.

Apply the migration:

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
```

Only these tables are created:

- `releases`
- `release_events`
- `release_locks`

## Run

The same binary supports API and worker modes:

```bash
./platform-server api
./platform-server worker
```

Container usage:

```bash
docker build -t platform-server:dev .
docker run --rm --env-file .env -p 8080:8080 platform-server:dev api
```

## API Examples

Health checks:

```bash
curl -s http://localhost:8080/healthz | jq
curl -s http://localhost:8080/readyz | jq
```

List services:

```bash
curl -s http://localhost:8080/api/services | jq
```

Get `forum-api` catalog details:

```bash
curl -s http://localhost:8080/api/services/forum-api | jq
```

Get service delivery status:

```bash
curl -s http://localhost:8080/api/services/forum-api/status | jq
```

Create a release:

```bash
curl -s -X POST http://localhost:8080/api/releases \
  -H 'Content-Type: application/json' \
  -d '{
    "service": "forum-api",
    "environment": "dev",
    "branch": "main",
    "operator": "worryyy"
  }' | jq
```

The API returns immediately:

```json
{
  "code": 202,
  "message": "",
  "data": {
    "release_id": "rel-20260612-001",
    "status": "Queued"
  }
}
```

Query release state:

```bash
RELEASE_ID='rel-20260612-001'
curl -s "http://localhost:8080/api/releases/${RELEASE_ID}" | jq
curl -s "http://localhost:8080/api/releases/${RELEASE_ID}/events" | jq
curl -s http://localhost:8080/api/services/forum-api/releases | jq
```

Duplicate release conflict check:

```bash
curl -i -X POST http://localhost:8080/api/releases \
  -H 'Content-Type: application/json' \
  -d '{
    "service": "forum-api",
    "environment": "dev",
    "branch": "main",
    "operator": "worryyy"
  }'
```

When a `forum-api/dev` release lock is still held, the API returns `409 Conflict`:

```json
{
  "code": 409,
  "message": "release already running for service and environment",
  "data": null
}
```

## State Machine

Implemented states:

```text
Requested
Validated
Queued
JenkinsTriggered
JenkinsRunning
GitOpsUpdated
ArgoSyncing
RolloutChecking
Succeeded
Failed
Timeout
Canceled
```

Every status transition writes a `release_events` row.

## Verification Checklist

After PostgreSQL, RabbitMQ, Jenkins, K3s, and ArgoCD are reachable:

1. `GET /api/services` returns `forum-api`.
2. `GET /api/services/forum-api` returns the catalog entry.
3. `POST /api/releases` creates a release and returns a `release_id`.
4. `select * from releases order by created_at desc limit 1;` shows the release row.
5. `select status,message from release_events where release_id = '<release_id>' order by id;` includes `Requested`, `Validated`, and `Queued`.
6. RabbitMQ queue `platform.release.requested.queue` receives the message.
7. Worker consumes the message.
8. Worker triggers Jenkins job `forum-api-pipeline`.
9. Worker observes Jenkins `SUCCESS`, `FAILURE`, or `ABORTED`.
10. Worker reads ArgoCD `applications.argoproj.io/forum-api` through Kubernetes API and waits for `Synced / Healthy`.
11. Worker reads Kubernetes Deployment `app/forum-api` rollout state.
12. Successful rollout sets release status to `Succeeded`.
13. Jenkins failure sets release status to `Failed` and records the failure detail.
14. Concurrent `forum-api/dev` release requests return `409 Conflict`.
