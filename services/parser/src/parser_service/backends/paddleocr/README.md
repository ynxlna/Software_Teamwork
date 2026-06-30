# PaddleOCR Backend

PaddleOCR-specific Python runtime code belongs in this package area.

This scaffold reserves the boundary only. A follow-up implementation should add
the actual PaddleOCR dependency, model loading, page/image preprocessing,
concurrency limits, and normalized block extraction behind the parser service
HTTP contract.
