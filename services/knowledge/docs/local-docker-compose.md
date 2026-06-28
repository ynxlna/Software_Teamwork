# Knowledge Service Local Docker Compose Setup

This document explains how to start the local Knowledge Service stack from `services/knowledge/`.

## What Starts

| Service | URL / Port | Purpose |
|---|---:|---|
| `knowledge-api` | <http://localhost:8000> | FastAPI Knowledge Service and Swagger |
| `knowledge-worker` | no public port | Placeholder background worker |
| `postgres` | `localhost:5432` | Metadata database |
| `redis` | `localhost:6379` | Queue and event backend |
| `qdrant` | <http://localhost:6333/dashboard> | Vector database and dashboard |
| `minio` | <http://localhost:9001> | Object storage console |
| `adminer` | <http://localhost:8080> | PostgreSQL web UI |
| `redis-commander` | <http://localhost:8081> | Redis web UI |

## First Run

Enter the Knowledge Service directory:

```bash
cd services/knowledge
```

Copy local environment defaults:

```bash
cp .env.example .env
```

The default `PIP_INDEX_URL` in `.env.example` uses the Tsinghua PyPI mirror for mainland China. Docker image acceleration still depends on your local Docker daemon mirror configuration.

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

Open Swagger:

```text
http://localhost:8000/docs
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
docker compose logs -f knowledge-api knowledge-worker
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

## Folder Ingest

Scan a local folder for files that can enter the basic ingest pipeline:

```bash
scripts/ingest_folder.sh \
  --dir /home/bao/Obsidian/Computer/软件工程合作/要求 \
  --recursive \
  --output /tmp/knowledge_candidates.txt \
  --show-excluded
```

Upload a controlled subset into a local knowledge base:

```bash
scripts/ingest_folder.sh \
  --dir /home/bao/projects/linux \
  --recursive \
  --upload \
  --kb-id kb_linux \
  --kb-name "Linux Knowledge Base" \
  --tags '["linux","local-test"]' \
  --max-files 20 \
  --show-excluded
```

Then inspect the service:

```bash
curl http://localhost:8000/api/v1/knowledge-bases/kb_linux
curl http://localhost:8000/api/v1/knowledge-bases/kb_linux/documents
curl http://localhost:8000/api/v1/admin-overview
curl -X POST http://localhost:8000/api/v1/knowledge-queries \
  -H "Content-Type: application/json" \
  -d '{"query":"linux scheduler","knowledgeBaseIds":["kb_linux"],"topK":5}'
```

## Embedding

The default embedding provider is `local_hashing`, which runs offline and verifies the full PostgreSQL/Qdrant pipeline. To use a real semantic embedding model, update `.env` before ingestion:

```text
EMBEDDING_PROVIDER=siliconflow
EMBEDDING_DIMENSION=1024
EMBEDDING_API_BASE=https://api.siliconflow.cn/v1
EMBEDDING_API_KEY=your-key
EMBEDDING_MODEL=BAAI/bge-m3
```

If `EMBEDDING_DIMENSION` changes, recreate `knowledge_chunks` in Qdrant and re-ingest documents.

Pull images manually if your network needs it:

```bash
docker pull postgres:16-alpine
docker pull redis:7-alpine
docker pull qdrant/qdrant:latest
docker pull minio/minio:latest
docker pull minio/mc:latest
docker pull adminer:latest
docker pull rediscommander/redis-commander:latest
```

## Data Directory

Local persistent data is stored under:

```text
data/postgres
data/redis
data/qdrant
data/minio
```

These directories are for local development only and should not be committed.

## Current Limitations

- The local ingest path runs inline in `knowledge-api`; `knowledge-worker` is reserved for async queue work.
- OCR is not implemented yet. Parser boundaries are kept in `app/parsers.py` so OCR can be added later without changing the folder script.
- Upstream gateway now contains active knowledge-owned frontend contracts for knowledge bases, document processing details, chunks, and knowledge queries. This local stack follows those resource paths and keeps jobs/admin overview as local-only until gateway exposes them.
