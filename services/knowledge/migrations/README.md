# Knowledge Service Migrations

PostgreSQL migrations for the Go Knowledge service live in this directory.

Knowledge owns knowledge-base metadata, knowledge document metadata, processing
jobs, chunks, and retrieval state. File service only stores base file-object
metadata. Knowledge document tables must not expose file service IDs through
public API responses.

Target model notes:

- `knowledge_documents` should store an internal `file_ref` for the original
  file object and keep `filename`, `content_type`, `size_bytes`, status, tags,
  parser state, chunks, and indexing state in knowledge-owned tables.
- `file_ref` is an opaque service-boundary reference. Current baseline code and
  early migrations may still use `file_id`; treat that as a compatibility
  column name and migrate toward `file_ref` when the repository contract is
  revised.
- Do not add `knowledge_base_id`, document status, tags, chunks, parser config,
  or Qdrant state to file service migrations.
