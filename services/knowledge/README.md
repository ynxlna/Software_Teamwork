# Knowledge Service

This directory contains the knowledge ingestion, chunking, embedding, and retrieval service.
It lives under `services/knowledge/` to match the upstream service boundary docs.

## Local Startup

```bash
cd services/knowledge
cp .env.example .env
docker compose up -d --build
curl http://localhost:8000/healthz
```

Swagger is available at:

```text
http://localhost:8000/docs
```

Detailed local setup notes are in [docs/local-docker-compose.md](docs/local-docker-compose.md).

Most JSON responses use the gateway-style envelope:

```json
{
  "data": {},
  "requestId": "req_123"
}
```

Knowledge debugging responses include `_fieldDescriptions`, a Chinese field explanation map for backend inspection.

## Local API Surface

The local service uses resource-style paths that match the active gateway knowledge contract where possible:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness check |
| `GET` | `/readyz` | Readiness and model/vector configuration check |
| `POST` | `/api/v1/knowledge-bases` | Create or update a local knowledge base |
| `GET` | `/api/v1/knowledge-bases` | List knowledge bases |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | Get knowledge base details |
| `PATCH` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | Update knowledge base metadata or strategies |
| `DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | Delete knowledge base metadata, chunks, and Qdrant points |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | Upload a document and run local ingestion |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | List documents in a knowledge base |
| `GET` | `/api/v1/documents/{documentId}` | Get document processing details |
| `PATCH` | `/api/v1/documents/{documentId}` | Update document tags |
| `DELETE` | `/api/v1/documents/{documentId}` | Delete document chunks and Qdrant points |
| `GET` | `/api/v1/documents/{documentId}/chunks` | List semantic chunks |
| `GET` | `/api/v1/jobs/{jobId}` | Get ingest job status |
| `POST` | `/api/v1/knowledge-queries` | Run vector retrieval |
| `GET` | `/api/v1/admin-overview` | Local overview statistics |

Note: upstream `docs/api/gateway.openapi.yaml` now contains active knowledge-owned paths for knowledge bases, document processing details, chunks, and knowledge queries. This local service is used to verify those capabilities before gateway proxy implementation; local-only endpoints such as jobs and admin overview are not frontend-stable unless they are added to the gateway OpenAPI.

## Folder Ingest

Scan a folder and output candidate files:

```bash
cd services/knowledge
scripts/ingest_folder.sh \
  --dir /home/bao/Obsidian/Computer/软件工程合作/要求 \
  --recursive \
  --output /tmp/knowledge_candidates.txt \
  --show-excluded
```

Upload candidates into a local knowledge base and run service-side chunking/vectorization:

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

The shell script only scans and calls the API. Parsing, semantic chunking, embedding, PostgreSQL writes, and Qdrant writes happen inside `knowledge-api`.

For metadata-style local filtering, `--tags '{"数据集":"linux","来源":"本地测试"}'` is still accepted by the service, but the upstream public upload contract uses `string[]` tags.

## Embedding

The default embedding provider is `local_hashing`, which is offline and only intended to verify the pipeline. For real semantic retrieval, configure an OpenAI-compatible embedding API in `.env`:

```text
EMBEDDING_PROVIDER=siliconflow
EMBEDDING_DIMENSION=1024
EMBEDDING_API_BASE=https://api.siliconflow.cn/v1
EMBEDDING_API_KEY=your-key
EMBEDDING_MODEL=BAAI/bge-m3
```

When changing `EMBEDDING_DIMENSION`, clear or recreate the Qdrant collection before re-ingesting.

## Current Scope

- `knowledge-api`: FastAPI app with knowledge base, document upload, chunk listing, job lookup, and retrieval endpoints.
- `knowledge-worker`: placeholder worker process for the future async ingest queue.
- PostgreSQL, Redis, Qdrant, MinIO, Adminer, and Redis Commander via service-local Docker Compose.

The current ingestion path runs inline inside the API process for local development. The service boundaries are structured so it can move to the worker later without changing the folder script contract.
