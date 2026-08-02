-- +goose Up
BEGIN;
CREATE SCHEMA IF NOT EXISTS partman;
CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;

CREATE TABLE publisher_types (
    id TEXT PRIMARY KEY
);

CREATE TABLE publishers (
    id UUID PRIMARY KEY,
    tags JSONB DEFAULT '{}'::JSONB,
    log_events JSONB DEFAULT '[]'::JSONB,
    lease_holder TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    publisher_type TEXT NOT NULL REFERENCES publisher_types (id),
    name TEXT NOT NULL,
    description TEXT,
    web_url TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_publishers_reconcile ON publishers (lease_expires_at ASC NULLS FIRST);


CREATE TABLE feed_types (
    id TEXT PRIMARY KEY
);

CREATE TABLE feeds (
    id UUID PRIMARY KEY,
    tags JSONB DEFAULT '{}'::JSONB,
    log_events JSONB DEFAULT '[]'::JSONB,
    lease_holder TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    publisher_id UUID NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
    feed_type TEXT NOT NULL REFERENCES feed_types(id),
    url TEXT NOT NULL,
    poll_interval INTERVAL NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_feeds_reconcile ON feeds (lease_expires_at ASC NULLS FIRST);

CREATE TABLE feed_polls (
    feed_id UUID PRIMARY KEY REFERENCES feeds (id) ON DELETE CASCADE,
    result_last_modified_at TIMESTAMPTZ NOT NULL,
    result_etag TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE document_artifact_formats (
    id TEXT PRIMARY KEY
);

CREATE TABLE documents (
    id UUID NOT NULL,
    tags JSONB DEFAULT '{}'::JSONB,
    log_events JSONB DEFAULT '[]'::JSONB,
    lease_holder TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    publisher_id UUID NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
    parent_document_id UUID NULL DEFAULT NULL REFERENCES documents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    published_at TIMESTAMPTZ NULL DEFAULT NULL,
    PRIMARY KEY (id)
) PARTITION BY RANGE (id);
CREATE INDEX idx_documents_reconcile ON documents (lease_expires_at ASC NULLS FIRST);

SELECT partman.create_partition(
    p_parent_table := 'public.documents',
    p_control      := 'id',
    p_interval     := '1 mon',
    p_premake      := 3,
    p_time_encoder := 'partman.uuid7_time_encoder',
    p_time_decoder := 'partman.uuid7_time_decoder'
);

CREATE TABLE document_artifacts (
    id UUID NOT NULL,
    tags JSONB DEFAULT '{}'::JSONB,
    log_events JSONB DEFAULT '[]'::JSONB,
    format TEXT NOT NULL REFERENCES document_artifact_formats (id) ON DELETE CASCADE,
    storage_url TEXT NOT NULL,
    size_bytes int8 NOT NULL,
    mime_type TEXT NOT NULL,
    checksum_sha26 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE scrape_orders (
    id UUID PRIMARY KEY,
    tags JSONB DEFAULT '{}'::JSONB,
    log_events JSONB DEFAULT '[]'::JSONB,
    lease_holder TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    publisher_id UUID NOT NULL REFERENCES publishers (id) ON DELETE CASCADE,
    scrape_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PLANNED' CHECK (status IN ('PLANNED', 'SUCCEEDED', 'FAILED', 'ABORTED')),
    created_at TIMESTAMPTZ DEFAULT now(),
    finished_at TIMESTAMPTZ NULL DEFAULT NULL
);
CREATE INDEX idx_scrape_orders_reconcile ON scrape_orders (lease_expires_at ASC NULLS FIRST);

COMMIT;
-- +goose Down
DROP TABLE IF EXISTS feed_polls CASCADE;
DROP TABLE IF EXISTS feeds CASCADE;
DROP TABLE IF EXISTS feed_types CASCADE;
DROP TABLE IF EXISTS publishers CASCADE;
DROP TABLE IF EXISTS publisher_types CASCADE;
DROP TYPE IF EXISTS system_entity_type CASCADE;