import json
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from typing import Any

from fastapi import FastAPI, File, Form, HTTPException, Query, Request, Response, UploadFile
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, ConfigDict, Field

from app.config import EMBEDDING_DIMENSION, EMBEDDING_MODEL, EMBEDDING_PROVIDER, QDRANT_COLLECTION
from app.database import init_db
from app.embedding import embed_text
from app.ids import new_id
from app.ingest import process_uploaded_document
from app.parsers import is_supported_filename
from app.qdrant_store import delete_points, search_points, set_payload_for_points
from app.repository import execute_one, fetch_all, json_dumps


@asynccontextmanager
async def lifespan(_app: FastAPI):
    init_db()
    yield


app = FastAPI(
    title="Knowledge Service",
    description="Knowledge ingestion, chunking, embedding, and retrieval service.",
    version="0.2.0",
    lifespan=lifespan,
)


class KnowledgeBaseCreate(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    id: str | None = None
    name: str
    description: str = ""
    doc_type: str = Field(default="GENERAL", alias="docType")
    chunk_strategy: dict[str, Any] = Field(default_factory=dict, alias="chunkStrategy")
    retrieval_strategy: dict[str, Any] = Field(default_factory=dict, alias="retrievalStrategy")


class KnowledgeBaseUpdate(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    name: str | None = None
    description: str | None = None
    doc_type: str | None = Field(default=None, alias="docType")
    chunk_strategy: dict[str, Any] | None = Field(default=None, alias="chunkStrategy")
    retrieval_strategy: dict[str, Any] | None = Field(default=None, alias="retrievalStrategy")


class DocumentUpdate(BaseModel):
    tags: list[str] = Field(default_factory=list)


class KnowledgeQueryCreate(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    query: str
    knowledge_base_ids: list[str] = Field(default_factory=list, alias="knowledgeBaseIds")
    top_k: int = Field(default=10, ge=1, le=100, alias="topK")
    score_threshold: float = Field(default=0.0, ge=0.0, alias="scoreThreshold")
    tags: list[str] = Field(default_factory=list)
    metadata_filter: dict[str, str] = Field(default_factory=dict, alias="metadataFilter")
    rerank: bool = False
    rerank_top_n: int | None = Field(default=None, alias="rerankTopN")


KB_FIELD_DESCRIPTIONS = {
    "id": "知识库唯一标识，后续上传、检索都用这个 ID 指定知识库。",
    "name": "知识库名称，用于后台和接口展示。",
    "description": "知识库描述，说明该知识库的内容范围。",
    "docType": "知识库文档类型，例如 GENERAL、REGULATION、TECH_REPORT。",
    "chunkStrategy": "切片策略配置，描述文档如何被切成多个文本片段。",
    "chunkStrategy.type": "切片策略类型；当前本地链路使用 SEMANTIC_TEXT。",
    "chunkStrategy.chunkSize": "目标切片长度，单位是字符数。",
    "chunkStrategy.overlap": "相邻切片重叠长度，单位是字符数。",
    "retrievalStrategy": "检索策略配置，描述向量召回、Top K、阈值等参数。",
    "retrievalStrategy.mode": "检索模式；当前本地链路使用 VECTOR。",
    "retrievalStrategy.topK": "默认返回的最大检索结果数。",
    "retrievalStrategy.scoreThreshold": "相似度分数过滤阈值，低于该值的结果会被过滤。",
    "documentCount": "当前知识库下的文档数量统计。",
    "chunkCount": "当前知识库下的切片数量统计，也对应已写入 Qdrant 的向量点数量。",
    "createdBy": "创建人；本地调试默认是 local。",
    "createdAt": "知识库创建时间，UTC 时间。",
    "updatedAt": "知识库最后更新时间，UTC 时间。",
}

DOCUMENT_FIELD_DESCRIPTIONS = {
    "id": "文档唯一标识。",
    "knowledgeBaseId": "所属知识库 ID。",
    "name": "展示用原始文件名或规范化文件名。",
    "contentType": "上传文件的 MIME 类型。",
    "sizeBytes": "文件大小，单位字节。",
    "status": "文档处理状态，按上游契约使用 uploaded、parsing、chunking、embedding、ready、failed。",
    "errorCode": "失败错误码；成功时为空。",
    "errorMessage": "失败原因；成功时为空。",
    "chunkCount": "该文档生成的切片数量。",
    "tags": "文档标签。公开契约使用 string[]，本地兼容旧 JSON object 输入并会转为展示标签。",
    "parserBackend": "实际使用的解析器后端，例如 text、pypdf、python-docx、openpyxl。",
    "createdBy": "创建人；本地调试默认是 local。",
    "createdAt": "文档创建时间，UTC 时间。",
    "updatedAt": "文档最后更新时间，UTC 时间。",
    "jobId": "本次上传触发的入库任务 ID，仅上传响应中返回，方便本地调试。",
}

CHUNK_FIELD_DESCRIPTIONS = {
    "id": "切片唯一标识，业务层引用这个 chunk id。",
    "knowledgeBaseId": "所属知识库 ID。",
    "documentId": "所属文档 ID。",
    "chunkIndex": "该切片在文档内的顺序编号，从 0 开始。",
    "sectionPath": "切片所在章节路径或标题；无法识别标题时为 root。",
    "content": "切片文本内容。",
    "tokenCount": "切片的粗略 token/词数统计，用于观察切片大小。",
    "chunkType": "切片类型；当前基础链路主要是 text，后续 OCR 可扩展为 image_ocr。",
    "qdrantPointId": "Qdrant 向量点 ID，必须是 UUID 或无符号整数。",
    "embeddingProvider": "生成该切片向量时使用的 embedding provider。",
    "embeddingDimension": "向量维度。",
    "embeddingPreview": "向量前几个维度的预览，方便确认已经完成向量化；不是完整向量。",
    "metadata": "解析和切片过程中的扩展元数据，例如 parser 和源文件后缀。",
    "createdAt": "切片创建时间，UTC 时间。",
}

JOB_FIELD_DESCRIPTIONS = {
    "id": "入库任务唯一标识。",
    "documentId": "该任务处理的文档 ID。",
    "jobType": "任务类型，例如 ingest、reprocess、delete。",
    "status": "任务状态，例如 running、succeeded、failed。",
    "currentStage": "当前处理阶段，例如 parsing、chunking、embedding、indexing、done。",
    "progress": "任务进度，0-100。",
    "message": "当前阶段说明。",
    "errorCode": "失败错误码；成功时为空。",
    "errorMessage": "失败原因；成功时为空。",
    "startedAt": "任务开始时间，UTC 时间。",
    "finishedAt": "任务结束时间，UTC 时间。",
    "createdAt": "任务记录创建时间，UTC 时间。",
    "updatedAt": "任务记录最后更新时间，UTC 时间。",
}

RETRIEVAL_FIELD_DESCRIPTIONS = {
    "id": "本次知识检索请求 ID，便于日志和链路追踪。",
    "query": "本次检索的原始问题。",
    "results": "检索结果列表，按相似度分数排序。",
    "results[].score": "Qdrant 返回的向量相似度分数，越高越相关。",
    "results[].pointId": "Qdrant 向量点 ID。",
    "results[].knowledgeBaseId": "命中的知识库 ID。",
    "results[].documentId": "命中的文档 ID。",
    "results[].chunkId": "命中的切片 ID。",
    "results[].documentName": "命中文档的文件名。",
    "results[].sectionPath": "命中切片所在章节路径。",
    "results[].chunkIndex": "命中切片在文档中的顺序编号。",
    "results[].contentPreview": "命中切片的内容预览，用于后台快速判断召回是否合理。",
    "results[].tags": "命中文档的标签。",
    "trace": "本次检索的处理链路摘要。",
    "trace.embeddingProvider": "本次查询向量使用的 embedding provider。",
    "trace.embeddingModel": "本次查询向量使用的 embedding 模型名。",
    "trace.embeddingDimension": "本次查询向量维度。",
    "trace.qdrantCollection": "检索使用的 Qdrant collection。",
    "trace.searchTopK": "本次向量召回请求的 Top K。",
    "trace.hitCount": "过滤后返回的命中数量。",
    "trace.rerank": "是否请求重排序；当前本地链路只保留开关和 trace，不实际调用 rerank 服务。",
}

STATS_FIELD_DESCRIPTIONS = {
    "knowledgeBaseCount": "知识库总数。",
    "documentCount": "文档总数。",
    "chunkCount": "切片总数。",
    "readyDocumentCount": "处理完成、可检索的文档数量。",
    "failedDocumentCount": "处理失败的文档数量。",
}

DOCUMENT_STATUSES = {"uploaded", "parsing", "chunking", "embedding", "ready", "failed"}


def request_id(request: Request) -> str:
    state_value = getattr(request.state, "request_id", None)
    if state_value:
        return str(state_value)
    header_value = request.headers.get("X-Request-Id")
    return header_value or new_id("req")


def api_error(
    status_code: int,
    code: str,
    message: str,
    fields: dict[str, str] | None = None,
) -> HTTPException:
    detail: dict[str, Any] = {"code": code, "message": message}
    if fields:
        detail["fields"] = fields
    return HTTPException(status_code=status_code, detail=detail)


def envelope(request: Request, data: Any, page: dict[str, int] | None = None) -> dict[str, Any]:
    body: dict[str, Any] = {"data": data, "requestId": request_id(request)}
    if page is not None:
        body["page"] = page
    return body


def with_field_descriptions(data: dict[str, Any], descriptions: dict[str, str]) -> dict[str, Any]:
    annotated = dict(data)
    annotated["_fieldDescriptions"] = descriptions
    return annotated


def lower_value(value: Any) -> str | None:
    if value is None:
        return None
    return str(value).lower()


def public_tags(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, list):
        return [str(item) for item in value]
    if isinstance(value, dict):
        return [f"{key}:{item}" for key, item in value.items()]
    return [str(value)]


def parse_multipart_tags(raw_tags: list[str] | None) -> Any:
    if not raw_tags:
        return []

    values = [item.strip() for item in raw_tags if item and item.strip()]
    if not values:
        return []

    if len(values) == 1:
        raw = values[0]
        try:
            decoded = json.loads(raw)
        except json.JSONDecodeError:
            if "," in raw:
                return [item.strip() for item in raw.split(",") if item.strip()]
            return [raw]

        if isinstance(decoded, list):
            return [str(item) for item in decoded]
        if isinstance(decoded, dict):
            return {str(key): str(value) for key, value in decoded.items()}
        if isinstance(decoded, str):
            return [decoded]
        raise api_error(400, "validation_error", "tags must be a string array or JSON object")

    return values


def row_or_404(row: dict[str, Any] | None, resource: str = "resource") -> dict[str, Any]:
    if row is None:
        raise api_error(404, "not_found", f"{resource} not found")
    return dict(row)


def knowledge_base_data(row: dict[str, Any]) -> dict[str, Any]:
    return with_field_descriptions(
        {
            "id": row["id"],
            "name": row["name"],
            "description": row.get("description", ""),
            "docType": row.get("doc_type", "GENERAL"),
            "chunkStrategy": row.get("chunk_strategy") or {},
            "retrievalStrategy": row.get("retrieval_strategy") or {},
            "documentCount": row.get("document_count", 0),
            "chunkCount": row.get("chunk_count", 0),
            "createdBy": row.get("created_by"),
            "createdAt": row.get("created_at"),
            "updatedAt": row.get("updated_at"),
        },
        KB_FIELD_DESCRIPTIONS,
    )


def document_data(row: dict[str, Any], job_id: str | None = None) -> dict[str, Any]:
    data = {
        "id": row["id"],
        "knowledgeBaseId": row["kb_id"],
        "name": row["filename"],
        "contentType": row.get("content_type"),
        "sizeBytes": row.get("size_bytes", 0),
        "status": lower_value(row.get("status")) or "uploaded",
        "errorCode": row.get("error_code"),
        "errorMessage": row.get("error_message"),
        "chunkCount": row.get("chunk_count", 0),
        "tags": public_tags(row.get("tags")),
        "parserBackend": row.get("parser_backend"),
        "createdBy": row.get("created_by"),
        "createdAt": row.get("created_at"),
        "updatedAt": row.get("updated_at"),
    }
    if job_id is not None:
        data["jobId"] = job_id
    return with_field_descriptions(data, DOCUMENT_FIELD_DESCRIPTIONS)


def chunk_data(row: dict[str, Any]) -> dict[str, Any]:
    return with_field_descriptions(
        {
            "id": row["id"],
            "knowledgeBaseId": row["kb_id"],
            "documentId": row["document_id"],
            "chunkIndex": row["chunk_index"],
            "sectionPath": row.get("section_path"),
            "content": row["content"],
            "tokenCount": row.get("token_count", 0),
            "chunkType": row.get("chunk_type"),
            "qdrantPointId": row.get("qdrant_point_id"),
            "embeddingProvider": row.get("embedding_provider"),
            "embeddingDimension": row.get("embedding_dimension"),
            "embeddingPreview": row.get("embedding_preview"),
            "metadata": row.get("metadata") or {},
            "createdAt": row.get("created_at"),
        },
        CHUNK_FIELD_DESCRIPTIONS,
    )


def job_data(row: dict[str, Any]) -> dict[str, Any]:
    return with_field_descriptions(
        {
            "id": row["id"],
            "documentId": row["document_id"],
            "jobType": lower_value(row.get("job_type")),
            "status": lower_value(row.get("status")),
            "currentStage": lower_value(row.get("current_stage")),
            "progress": row.get("progress", 0),
            "message": row.get("message", ""),
            "errorCode": row.get("error_code"),
            "errorMessage": row.get("error_message"),
            "startedAt": row.get("started_at"),
            "finishedAt": row.get("finished_at"),
            "createdAt": row.get("created_at"),
            "updatedAt": row.get("updated_at"),
        },
        JOB_FIELD_DESCRIPTIONS,
    )


def update_knowledge_base_counts(knowledge_base_id: str) -> None:
    execute_one(
        """
        UPDATE knowledge_bases
        SET document_count = (
                SELECT count(*) FROM documents
                WHERE kb_id = %s
            ),
            chunk_count = (
                SELECT count(*) FROM document_chunks
                WHERE kb_id = %s
            ),
            updated_at = now()
        WHERE id = %s
        RETURNING id
        """,
        (knowledge_base_id, knowledge_base_id, knowledge_base_id),
    )


@app.middleware("http")
async def request_id_middleware(request: Request, call_next):
    request.state.request_id = request.headers.get("X-Request-Id") or new_id("req")
    response = await call_next(request)
    response.headers["X-Request-Id"] = request_id(request)
    return response


@app.exception_handler(HTTPException)
async def http_exception_handler(request: Request, exc: HTTPException):
    detail = exc.detail
    if isinstance(detail, dict) and "code" in detail and "message" in detail:
        code = str(detail["code"])
        message = str(detail["message"])
        fields = detail.get("fields")
    else:
        code = "validation_error" if exc.status_code == 400 else "internal_error"
        message = str(detail)
        fields = None

    body: dict[str, Any] = {
        "error": {
            "code": code,
            "message": message,
            "requestId": request_id(request),
        }
    }
    if isinstance(fields, dict):
        body["error"]["fields"] = fields
    return JSONResponse(status_code=exc.status_code, content=body)


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    fields: dict[str, str] = {}
    for error in exc.errors():
        location = ".".join(str(part) for part in error.get("loc", []) if part != "body")
        fields[location or "request"] = str(error.get("msg", "invalid value"))
    body = {
        "error": {
            "code": "validation_error",
            "message": "request validation failed",
            "requestId": request_id(request),
            "fields": fields,
        }
    }
    return JSONResponse(status_code=400, content=body)


@app.exception_handler(RuntimeError)
async def runtime_exception_handler(request: Request, _exc: RuntimeError):
    return JSONResponse(
        status_code=502,
        content={
            "error": {
                "code": "dependency_error",
                "message": "dependency request failed",
                "requestId": request_id(request),
            }
        },
    )


@app.exception_handler(Exception)
async def unhandled_exception_handler(request: Request, _exc: Exception):
    return JSONResponse(
        status_code=500,
        content={
            "error": {
                "code": "internal_error",
                "message": "internal server error",
                "requestId": request_id(request),
            }
        },
    )


@app.get("/")
def read_root(request: Request) -> dict[str, Any]:
    return envelope(
        request,
        {
            "service": "knowledge",
            "message": "Knowledge Service is running.",
            "docsUrl": "/docs",
            "healthUrl": "/healthz",
        },
    )


@app.get("/healthz")
def read_healthz(request: Request) -> dict[str, Any]:
    return envelope(request, {"status": "ok"})


@app.get("/readyz")
def read_readyz(request: Request) -> dict[str, Any]:
    return envelope(
        request,
        {
            "status": "ok",
            "service": "knowledge",
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "embeddingProvider": EMBEDDING_PROVIDER,
            "embeddingModel": EMBEDDING_MODEL,
            "embeddingDimension": EMBEDDING_DIMENSION,
            "qdrantCollection": QDRANT_COLLECTION,
        },
    )


@app.post("/api/v1/knowledge-bases", status_code=201)
def create_knowledge_base(request: Request, payload: KnowledgeBaseCreate) -> dict[str, Any]:
    knowledge_base_id = payload.id or new_id("kb")
    row = execute_one(
        """
        INSERT INTO knowledge_bases (
            id, name, description, doc_type, chunk_strategy, retrieval_strategy
        )
        VALUES (%s, %s, %s, %s, %s::jsonb, %s::jsonb)
        ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            doc_type = EXCLUDED.doc_type,
            chunk_strategy = EXCLUDED.chunk_strategy,
            retrieval_strategy = EXCLUDED.retrieval_strategy,
            updated_at = now()
        RETURNING *
        """,
        (
            knowledge_base_id,
            payload.name,
            payload.description,
            payload.doc_type,
            json_dumps(payload.chunk_strategy),
            json_dumps(payload.retrieval_strategy),
        ),
    )
    return envelope(request, knowledge_base_data(row_or_404(row, "knowledge base")))


@app.get("/api/v1/knowledge-bases")
def list_knowledge_bases(
    request: Request,
    page: int = Query(default=1, ge=1),
    pageSize: int = Query(default=20, ge=1, le=200),
) -> dict[str, Any]:
    offset = (page - 1) * pageSize
    rows = fetch_all(
        "SELECT * FROM knowledge_bases ORDER BY created_at DESC LIMIT %s OFFSET %s",
        (pageSize, offset),
    )
    total_row = execute_one("SELECT count(*) AS total FROM knowledge_bases", ())
    total = int(total_row["total"]) if total_row else 0
    return envelope(
        request,
        [knowledge_base_data(row) for row in rows],
        {"page": page, "pageSize": pageSize, "total": total},
    )


@app.get("/api/v1/knowledge-bases/{knowledgeBaseId}")
def get_knowledge_base(request: Request, knowledgeBaseId: str) -> dict[str, Any]:
    row = execute_one("SELECT * FROM knowledge_bases WHERE id = %s", (knowledgeBaseId,))
    return envelope(request, knowledge_base_data(row_or_404(row, "knowledge base")))


@app.patch("/api/v1/knowledge-bases/{knowledgeBaseId}")
def update_knowledge_base(
    request: Request,
    knowledgeBaseId: str,
    payload: KnowledgeBaseUpdate,
) -> dict[str, Any]:
    current = row_or_404(
        execute_one("SELECT * FROM knowledge_bases WHERE id = %s", (knowledgeBaseId,)),
        "knowledge base",
    )
    row = execute_one(
        """
        UPDATE knowledge_bases
        SET name = %s,
            description = %s,
            doc_type = %s,
            chunk_strategy = %s::jsonb,
            retrieval_strategy = %s::jsonb,
            updated_at = now()
        WHERE id = %s
        RETURNING *
        """,
        (
            payload.name if payload.name is not None else current["name"],
            payload.description if payload.description is not None else current["description"],
            payload.doc_type if payload.doc_type is not None else current["doc_type"],
            json_dumps(
                payload.chunk_strategy
                if payload.chunk_strategy is not None
                else current["chunk_strategy"]
            ),
            json_dumps(
                payload.retrieval_strategy
                if payload.retrieval_strategy is not None
                else current["retrieval_strategy"]
            ),
            knowledgeBaseId,
        ),
    )
    return envelope(request, knowledge_base_data(row_or_404(row, "knowledge base")))


@app.delete("/api/v1/knowledge-bases/{knowledgeBaseId}", status_code=204)
def delete_knowledge_base(knowledgeBaseId: str) -> Response:
    row_or_404(
        execute_one("SELECT id FROM knowledge_bases WHERE id = %s", (knowledgeBaseId,)),
        "knowledge base",
    )
    point_rows = fetch_all(
        "SELECT qdrant_point_id FROM document_chunks WHERE kb_id = %s",
        (knowledgeBaseId,),
    )
    delete_points([row["qdrant_point_id"] for row in point_rows if row.get("qdrant_point_id")])
    execute_one("DELETE FROM document_chunks WHERE kb_id = %s RETURNING id", (knowledgeBaseId,))
    execute_one(
        """
        DELETE FROM ingest_jobs
        WHERE document_id IN (SELECT id FROM documents WHERE kb_id = %s)
        RETURNING id
        """,
        (knowledgeBaseId,),
    )
    execute_one("DELETE FROM documents WHERE kb_id = %s RETURNING id", (knowledgeBaseId,))
    execute_one("DELETE FROM knowledge_bases WHERE id = %s RETURNING id", (knowledgeBaseId,))
    return Response(status_code=204)


@app.get("/api/v1/knowledge-bases/{knowledgeBaseId}/documents")
def list_documents(
    request: Request,
    knowledgeBaseId: str,
    page: int = Query(default=1, ge=1),
    pageSize: int = Query(default=20, ge=1, le=200),
    status: str | None = Query(default=None),
) -> dict[str, Any]:
    row_or_404(
        execute_one("SELECT id FROM knowledge_bases WHERE id = %s", (knowledgeBaseId,)),
        "knowledge base",
    )
    if status is not None and status.lower() not in DOCUMENT_STATUSES:
        raise api_error(
            400,
            "validation_error",
            "invalid document status",
            {"status": "must be one of uploaded, parsing, chunking, embedding, ready, failed"},
        )
    offset = (page - 1) * pageSize
    if status is None:
        rows = fetch_all(
            """
            SELECT * FROM documents
            WHERE kb_id = %s
            ORDER BY created_at DESC
            LIMIT %s OFFSET %s
            """,
            (knowledgeBaseId, pageSize, offset),
        )
        total_row = execute_one(
            "SELECT count(*) AS total FROM documents WHERE kb_id = %s",
            (knowledgeBaseId,),
        )
    else:
        rows = fetch_all(
            """
            SELECT * FROM documents
            WHERE kb_id = %s AND lower(status) = lower(%s)
            ORDER BY created_at DESC
            LIMIT %s OFFSET %s
            """,
            (knowledgeBaseId, status, pageSize, offset),
        )
        total_row = execute_one(
            """
            SELECT count(*) AS total FROM documents
            WHERE kb_id = %s AND lower(status) = lower(%s)
            """,
            (knowledgeBaseId, status),
        )
    total = int(total_row["total"]) if total_row else 0
    return envelope(
        request,
        [document_data(row) for row in rows],
        {"page": page, "pageSize": pageSize, "total": total},
    )


@app.post("/api/v1/knowledge-bases/{knowledgeBaseId}/documents", status_code=201)
async def upload_document(
    request: Request,
    knowledgeBaseId: str,
    file: UploadFile = File(...),
    tags: list[str] | None = Form(default=None),
) -> dict[str, Any]:
    row_or_404(
        execute_one("SELECT id FROM knowledge_bases WHERE id = %s", (knowledgeBaseId,)),
        "knowledge base",
    )
    if not is_supported_filename(file.filename or ""):
        raise api_error(
            400,
            "validation_error",
            "unsupported file type",
            {"file": f"unsupported file type: {file.filename}"},
        )

    content = await file.read()
    if not content:
        raise api_error(400, "validation_error", "uploaded file is empty", {"file": "is empty"})

    document_id = new_id("doc")
    job_id = new_id("job")
    parsed_tags = parse_multipart_tags(tags)

    execute_one(
        """
        INSERT INTO documents (
            id, kb_id, filename, content_type, size_bytes, status, tags
        )
        VALUES (%s, %s, %s, %s, %s, 'uploaded', %s::jsonb)
        RETURNING id
        """,
        (
            document_id,
            knowledgeBaseId,
            file.filename or "uploaded-file",
            file.content_type or "application/octet-stream",
            len(content),
            json_dumps(parsed_tags),
        ),
    )
    execute_one(
        """
        INSERT INTO ingest_jobs (
            id, document_id, status, current_stage, progress, message, started_at
        )
        VALUES (%s, %s, 'running', 'upload', 5, 'upload received', now())
        RETURNING id
        """,
        (job_id, document_id),
    )

    try:
        process_uploaded_document(
            kb_id=knowledgeBaseId,
            document_id=document_id,
            job_id=job_id,
            filename=file.filename or "uploaded-file",
            content=content,
            tags=parsed_tags,
        )
    except Exception as exc:
        raise api_error(500, "internal_error", "document ingest failed") from exc

    row = row_or_404(
        execute_one("SELECT * FROM documents WHERE id = %s", (document_id,)),
        "document",
    )
    return envelope(request, document_data(row, job_id=job_id))


@app.get("/api/v1/documents/{documentId}")
def get_document(request: Request, documentId: str) -> dict[str, Any]:
    row = execute_one("SELECT * FROM documents WHERE id = %s", (documentId,))
    return envelope(request, document_data(row_or_404(row, "document")))


@app.patch("/api/v1/documents/{documentId}")
def update_document(
    request: Request,
    documentId: str,
    payload: DocumentUpdate,
) -> dict[str, Any]:
    row_or_404(execute_one("SELECT id FROM documents WHERE id = %s", (documentId,)), "document")
    row = execute_one(
        """
        UPDATE documents
        SET tags = %s::jsonb,
            updated_at = now()
        WHERE id = %s
        RETURNING *
        """,
        (json_dumps(payload.tags), documentId),
    )
    point_rows = fetch_all(
        "SELECT qdrant_point_id FROM document_chunks WHERE document_id = %s",
        (documentId,),
    )
    set_payload_for_points(
        [item["qdrant_point_id"] for item in point_rows if item.get("qdrant_point_id")],
        {"tags": payload.tags},
    )
    return envelope(request, document_data(row_or_404(row, "document")))


@app.delete("/api/v1/documents/{documentId}", status_code=204)
def delete_document(documentId: str) -> Response:
    document = row_or_404(
        execute_one("SELECT id, kb_id FROM documents WHERE id = %s", (documentId,)),
        "document",
    )
    point_rows = fetch_all(
        "SELECT qdrant_point_id FROM document_chunks WHERE document_id = %s",
        (documentId,),
    )
    delete_points([row["qdrant_point_id"] for row in point_rows if row.get("qdrant_point_id")])
    execute_one("DELETE FROM document_chunks WHERE document_id = %s RETURNING id", (documentId,))
    execute_one("DELETE FROM ingest_jobs WHERE document_id = %s RETURNING id", (documentId,))
    execute_one("DELETE FROM documents WHERE id = %s RETURNING id", (documentId,))
    update_knowledge_base_counts(document["kb_id"])
    return Response(status_code=204)


@app.get("/api/v1/documents/{documentId}/chunks")
def list_document_chunks(
    request: Request,
    documentId: str,
    page: int = Query(default=1, ge=1),
    pageSize: int = Query(default=50, ge=1, le=500),
) -> dict[str, Any]:
    row_or_404(execute_one("SELECT id FROM documents WHERE id = %s", (documentId,)), "document")
    offset = (page - 1) * pageSize
    rows = fetch_all(
        """
        SELECT id, kb_id, document_id, chunk_index, section_path, content,
               token_count, chunk_type, qdrant_point_id, embedding_provider,
               embedding_dimension, embedding_preview, metadata, created_at
        FROM document_chunks
        WHERE document_id = %s
        ORDER BY chunk_index
        LIMIT %s OFFSET %s
        """,
        (documentId, pageSize, offset),
    )
    total_row = execute_one(
        "SELECT count(*) AS total FROM document_chunks WHERE document_id = %s",
        (documentId,),
    )
    total = int(total_row["total"]) if total_row else 0
    return envelope(
        request,
        [chunk_data(row) for row in rows],
        {"page": page, "pageSize": pageSize, "total": total},
    )


@app.get("/api/v1/jobs/{jobId}")
def get_job(request: Request, jobId: str) -> dict[str, Any]:
    row = execute_one("SELECT * FROM ingest_jobs WHERE id = %s", (jobId,))
    return envelope(request, job_data(row_or_404(row, "job")))


def build_qdrant_filter(payload: KnowledgeQueryCreate) -> dict[str, Any] | None:
    must: list[dict[str, Any]] = []
    if payload.knowledge_base_ids:
        if len(payload.knowledge_base_ids) == 1:
            must.append({"key": "kb_id", "match": {"value": payload.knowledge_base_ids[0]}})
        else:
            must.append({"key": "kb_id", "match": {"any": payload.knowledge_base_ids}})
    if payload.tags:
        must.append({"key": "tags", "match": {"any": payload.tags}})
    for key, value in payload.metadata_filter.items():
        must.append({"key": f"tags.{key}", "match": {"value": value}})
    if not must:
        return None
    return {"must": must}


@app.post("/api/v1/knowledge-queries", status_code=201)
def create_knowledge_query(request: Request, payload: KnowledgeQueryCreate) -> dict[str, Any]:
    vector = embed_text(payload.query)
    raw_results = search_points(
        vector,
        payload.top_k,
        build_qdrant_filter(payload),
    )
    results = []
    for item in raw_results:
        score = float(item.get("score", 0.0))
        if score < payload.score_threshold:
            continue
        point_payload = item.get("payload") or {}
        chunk_row = execute_one(
            "SELECT content FROM document_chunks WHERE id = %s",
            (point_payload.get("chunk_id"),),
        )
        content = chunk_row["content"] if chunk_row else ""
        results.append(
            {
                "score": score,
                "pointId": item.get("id"),
                "knowledgeBaseId": point_payload.get("kb_id"),
                "documentId": point_payload.get("document_id"),
                "chunkId": point_payload.get("chunk_id"),
                "documentName": point_payload.get("filename"),
                "sectionPath": point_payload.get("section_path"),
                "chunkIndex": point_payload.get("chunk_index"),
                "contentPreview": content[:240],
                "tags": public_tags(point_payload.get("tags")),
            }
        )

    data = with_field_descriptions(
        {
            "id": new_id("kq"),
            "query": payload.query,
            "results": results,
            "trace": {
                "embeddingProvider": EMBEDDING_PROVIDER,
                "embeddingModel": EMBEDDING_MODEL,
                "embeddingDimension": EMBEDDING_DIMENSION,
                "qdrantCollection": QDRANT_COLLECTION,
                "searchTopK": payload.top_k,
                "scoreThreshold": payload.score_threshold,
                "hitCount": len(results),
                "rerank": payload.rerank,
                "rerankTopN": payload.rerank_top_n,
            },
        },
        RETRIEVAL_FIELD_DESCRIPTIONS,
    )
    return envelope(request, data)


@app.get("/api/v1/admin-overview")
def admin_overview(request: Request) -> dict[str, Any]:
    row = execute_one(
        """
        SELECT
            (SELECT count(*) FROM knowledge_bases) AS knowledge_base_count,
            (SELECT count(*) FROM documents) AS document_count,
            (SELECT count(*) FROM document_chunks) AS chunk_count,
            (SELECT count(*) FROM documents WHERE lower(status) = 'ready') AS ready_document_count,
            (SELECT count(*) FROM documents WHERE lower(status) = 'failed') AS failed_document_count
        """,
        (),
    )
    stats = row_or_404(row, "admin overview")
    data = with_field_descriptions(
        {
            "knowledgeBaseCount": stats["knowledge_base_count"],
            "documentCount": stats["document_count"],
            "chunkCount": stats["chunk_count"],
            "readyDocumentCount": stats["ready_document_count"],
            "failedDocumentCount": stats["failed_document_count"],
        },
        STATS_FIELD_DESCRIPTIONS,
    )
    return envelope(request, data)
