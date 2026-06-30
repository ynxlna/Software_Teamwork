from pydantic import BaseModel, ConfigDict, Field


class CreateParsedDocumentRequest(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    document_name: str | None = Field(default=None, alias="documentName", max_length=512)
    content_type: str | None = Field(default=None, alias="contentType", max_length=255)
    size_bytes: int | None = Field(default=None, alias="sizeBytes", ge=0)
    data_base64: str = Field(alias="dataBase64", min_length=1)
