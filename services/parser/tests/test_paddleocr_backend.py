from parser_service.backends.paddleocr.backend import _suffix_for, extract_texts


def test_extract_texts_from_paddleocr_v3_result_shape():
    result = [
        {
            "res": {
                "rec_texts": ["助力双方交往", "搭建友谊桥梁"],
            }
        }
    ]

    assert extract_texts(result) == ["助力双方交往", "搭建友谊桥梁"]


def test_extract_texts_from_paddleocr_v2_result_shape():
    result = [
        [
            [[[0, 0], [1, 0], [1, 1], [0, 1]], ("Breaker OCR", 0.98)],
            [[[0, 2], [1, 2], [1, 3], [0, 3]], ("Relay cabinet", 0.97)],
        ]
    ]

    assert extract_texts(result) == ["Breaker OCR", "Relay cabinet"]


def test_suffix_prefers_safe_document_extension():
    assert _suffix_for("scan.PDF", "application/octet-stream") == ".pdf"
    assert _suffix_for("scan", "image/jpeg") == ".jpg"
    assert (
        _suffix_for(
            "manual.docx",
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        )
        == ".bin"
    )
