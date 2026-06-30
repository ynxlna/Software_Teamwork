import base64
import io
import zipfile

import pytest

from parser_service.backends.document import DocumentParserBackend
from parser_service.service import BackendHealth, ParsedDocument, ParseRequest
from parser_service.service.errors import AppError


class FakeOCRBackend:
    name = "paddleocr"

    def __init__(self) -> None:
        self.requests = []

    def health(self) -> BackendHealth:
        return BackendHealth(ready=True, status="ready")

    def warm_up(self) -> None:
        return None

    def parse(self, request):
        self.requests.append(request)
        return ParsedDocument(content=" OCR result ", title="OCR", backend=self.name)


def test_document_backend_parses_text_without_ocr():
    ocr = FakeOCRBackend()
    backend = DocumentParserBackend(ocr_backend=ocr)

    parsed = backend.parse(
        ParseRequest(
            document_name="notes.md",
            content_type="text/markdown",
            size_bytes=None,
            data=b"# Title\n\nbody",
        )
    )

    assert parsed.content == "# Title\nbody"
    assert parsed.title == "Title"
    assert parsed.backend == "text"
    assert ocr.requests == []


def test_document_backend_parses_docx_text():
    backend = DocumentParserBackend(ocr_backend=FakeOCRBackend())

    parsed = backend.parse(
        ParseRequest(
            document_name="manual.docx",
            content_type="application/vnd.openxmlformats-officedocument.wordprocessingml.document",
            size_bytes=None,
            data=_docx(["Breaker Manual", "Install steps"]),
        )
    )

    assert parsed.content == "Breaker Manual\nInstall steps"
    assert parsed.title == "Breaker Manual"
    assert parsed.backend == "docx"


def test_document_backend_routes_pdf_to_paddleocr():
    ocr = FakeOCRBackend()
    backend = DocumentParserBackend(ocr_backend=ocr)

    parsed = backend.parse(
        ParseRequest(
            document_name="scan.pdf",
            content_type="application/pdf",
            size_bytes=None,
            data=b"%PDF-1.7\n",
        )
    )

    assert parsed.content == " OCR result "
    assert parsed.backend == "paddleocr"
    assert len(ocr.requests) == 1


def test_document_backend_rejects_unsupported_binary():
    backend = DocumentParserBackend(ocr_backend=FakeOCRBackend())

    with pytest.raises(AppError) as raised:
        backend.parse(
            ParseRequest(
                document_name="archive.bin",
                content_type="application/octet-stream",
                size_bytes=None,
                data=base64.b64decode("AAECAwQF"),
            )
        )

    assert raised.value.status_code == 400
    assert raised.value.code == "validation_error"


def _docx(paragraphs: list[str]) -> bytes:
    parts = []
    for paragraph in paragraphs:
        parts.append(f"<w:p><w:r><w:t>{paragraph}</w:t></w:r></w:p>")
    xml = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
        "<w:body>"
        + "".join(parts)
        + "</w:body></w:document>"
    )
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        archive.writestr("word/document.xml", xml)
    return buffer.getvalue()
