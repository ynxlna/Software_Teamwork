# File Service Migrations

No production PostgreSQL repository is implemented in this MVP.

When PostgreSQL metadata persistence is added, create forward-only migrations here. The expected first table should store base file metadata only:

- internal file id
- display filename
- content type
- size bytes
- checksum
- server-generated object key
- created and deleted timestamps

Do not store raw file contents in PostgreSQL and do not expose object keys through API responses.

Do not store knowledge-base IDs, knowledge document IDs, report IDs, template IDs, material IDs, report file IDs, business tags, processing status, or ACLs here. Those belong to the owner service that references the base file object.
