# Parser Service Orchestration

Parser orchestration and normalization logic lives here.

This layer decodes base64 payloads, enforces size limits, validates `sizeBytes`,
serializes backend access with a bounded semaphore, applies parse timeouts, and
normalizes backend output before HTTP handlers write the response envelope.
Knowledge remains responsible for chunking, embedding, vector indexing,
retrieval, and ingestion job state.
