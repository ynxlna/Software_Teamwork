# Knowledge Service Local Docker Compose Setup

This document explains how to start the local Go-based Knowledge Service stack
from `services/knowledge/`.

## What Starts

| Service | URL / Port | Purpose |
|---|---:|---|
| `knowledge-api` | <http://localhost:8000> | Go Knowledge Service baseline |
| `postgres` | `localhost:5432` | Future metadata database |
| `redis` | `localhost:6379` | Future queue and event backend |
| `qdrant` | <http://localhost:6333/dashboard> | Future vector database and dashboard |
| `minio` | <http://localhost:9001> | Future object storage console |
| `adminer` | <http://localhost:8080> | PostgreSQL web UI |
| `redis-commander` | <http://localhost:8081> | Redis web UI |

The current Go baseline exposes only `/healthz` and `/readyz`. The previous
Python/FastAPI ingest API has been removed from this service directory.

## First Run

Enter the Knowledge Service directory:

```bash
cd services/knowledge
```

Copy local environment defaults:

```bash
cp .env.example .env
```

Build and start the stack:

```bash
docker compose up -d --build
```

Check containers:

```bash
docker compose ps
```

Check the API:

```bash
curl http://localhost:8000/healthz
curl http://localhost:8000/readyz
```

## Local Credentials

PostgreSQL:

```text
server: localhost
port: 5432
database: knowledge
user: knowledge
password: knowledge
```

Adminer:

```text
URL: http://localhost:8080
System: PostgreSQL
Server: postgres
Username: knowledge
Password: knowledge
Database: knowledge
```

MinIO:

```text
URL: http://localhost:9001
Username: minio
Password: minio123
Bucket: knowledge-documents
```

Redis Commander:

```text
URL: http://localhost:8081
```

Qdrant:

```text
Dashboard: http://localhost:6333/dashboard
REST API: http://localhost:6333
gRPC: localhost:6334
Collection: knowledge_chunks
```

## Useful Commands

View logs:

```bash
docker compose logs -f knowledge-api
```

Restart the API:

```bash
docker compose restart knowledge-api
```

Stop services but keep data:

```bash
docker compose down
```

Stop services and delete local data:

```bash
docker compose down -v
rm -rf data/
```

Validate Compose syntax:

```bash
docker compose config
```

## Removed Prototype

Historical folder-ingest commands and FastAPI Swagger examples from the Python
prototype are no longer part of this service. Rebuild ingestion, parser,
embedding, and retrieval behavior as Go vertical slices under the service-local
`internal/` packages.
