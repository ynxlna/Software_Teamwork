# Parser Service Orchestration

Parser orchestration and normalization logic belongs here.

This layer should coordinate backend adapters such as PaddleOCR and return a
stable parsed-document structure to HTTP handlers. Knowledge remains responsible
for chunking, embedding, vector indexing, retrieval, and ingestion job state.
