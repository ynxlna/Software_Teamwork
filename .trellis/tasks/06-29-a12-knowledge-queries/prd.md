# A-12 Knowledge queries retrieval and rerank integration

## Goal

Complete the existing Go Knowledge Service retrieval vertical slice for `POST /internal/v1/knowledge-queries`, preserving the gateway contract and safely supporting optional reranking without requiring a live external model.

## Requirements

* Accept and validate all fields required by issue #84.
* Embed, vector-search, repository-hydrate, and enforce ownership, ready status, deletion, score, tag, and metadata filters.
* Return only documented result fields and a safe trace summary.
* Add an injectable reranker boundary. When S-04 is unavailable, preserve vector order as a deterministic fallback.
* Keep the internal response aligned with the public gateway schema.

## Acceptance Criteria

* [ ] Existing interfaces and tests remain compatible.
* [ ] Tests cover `topK`, threshold, tags, metadata, rerank, and `rerankTopN`.
* [ ] Empty, low-score, non-ready, deleted, and unauthorized results are handled safely.
* [ ] Reranking uses a fake and no network credentials.
* [ ] `gofmt` and `go test ./...` pass.

## Technical Approach

Extend the existing flow. Add a provider-neutral `Reranker` port and service option; invoke it only after authorization/status hydration, normalize output by chunk ID, and cap it to `rerankTopN`.

## Decision (ADR-lite)

The AI Gateway contract exists but S-04 runtime code does not. Define the boundary and no-op fallback now; defer the HTTP adapter and credentials/config wiring to S-04.

## Out of Scope

* Rewriting Knowledge, implementing AI Gateway, real-model tests, gateway routing, or frontend work.
* Returning full content, vectors, payloads, object keys, or provider errors.

## Technical Notes

Baseline is `develop`; `main` contains the retired Python prototype. Existing vector adapters already implement tag AND and exact metadata filtering.
