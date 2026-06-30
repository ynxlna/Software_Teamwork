# PaddleOCR Backend

PaddleOCR-specific Python runtime code lives in this package area.

The adapter lazily imports and initializes `paddleocr.PaddleOCR`, then supports
both the PaddleOCR 3.x `predict` interface and the older `ocr` interface. It
extracts text from common PaddleOCR result shapes and returns normalized parsed
document data to the service layer.

PaddleOCR is an optional dependency in local development:

```bash
uv sync --group dev --extra paddleocr
```

The runtime Dockerfile installs the `paddleocr` extra by default.
