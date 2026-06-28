# Knowledge Service Migrations

PostgreSQL migrations for the Go Knowledge service will live in this directory.

The current baseline does not create durable tables yet. Add forward-only
migration files such as `0001_create_knowledge_bases.sql` when metadata
persistence is implemented.
