-- +goose Up
BEGIN;

-- load pg_jsonschema extension.
CREATE EXTENSION IF NOT EXISTS pg_jsonschema;

-- setting up partman for automatic partition management.
CREATE SCHEMA IF NOT EXISTS partman;

-- it is best practice to keep it in its own schema.
CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;

-- create schema for cortex.
CREATE SCHEMA IF NOT EXISTS cortex;

-- is_flat_annotation checks if the given JSONB data structure is a valid annotation object.
-- annotation objects must not contain nested JSON objects and only rely on primitive types and arrays.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cortex.is_flat_annotation (data JSONB)
  RETURNS BOOLEAN
  AS $func$
  SELECT
    jsonb_matches_schema ('{
           "type": "object",
           "additionalProperties": {
             "oneOf": [
               { "type": ["string", "number", "boolean", "null"] },
               {
                 "type": "array",
                 "items": {
                   "type": ["string", "number", "boolean", "null"]
                 }
               }
             ]
           }
         }'::JSON, data);
$func$
LANGUAGE sql
IMMUTABLE STRICT;

-- +goose StatementEnd
-- Publishers Table
CREATE TABLE cortex.publishers (
  id UUID PRIMARY KEY,
  annotations JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (cortex.is_flat_annotation (annotations)),
  finalizers TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  canonical_name TEXT NOT NULL,
  version INT8 NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL DEFAULT NULL
);

CREATE TABLE cortex.documents (
  id UUID PRIMARY KEY,
  annotations JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (cortex.is_flat_annotation (annotations)),
  finalizers TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  publisher_id UUID NOT NULL REFERENCES cortex.publishers (id) ON DELETE CASCADE,
  file_upload_completed BOOL NOT NULL DEFAULT FALSE,
  file_mime_type TEXT NOT NULL,
  file_size_bytes INT8,
  file_storage_volume TEXT NOT NULL,
  file_storage_key TEXT NOT NULL,
  version INT8 NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL DEFAULT NULL
);

CREATE TABLE cortex.artifact_types (
  id TEXT NOT NULL PRIMARY KEY,
  annotations JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (cortex.is_flat_annotation (annotations)),
  finalizers TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  api_group TEXT NOT NULL,
  api_kind TEXT NOT NULL,
  api_version TEXT NOT NULL,
  deprecated BOOL NOT NULL DEFAULT FALSE,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  schema JSON NOT NULL CHECK (jsonschema_is_valid (schema)),
  version INT8 NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL DEFAULT NULL
);

CREATE TABLE cortex.artifacts (
  id UUID,
  annotations JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (cortex.is_flat_annotation (annotations)),
  finalizers TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  artifact_type TEXT NOT NULL REFERENCES cortex.artifact_types (id) ON DELETE CASCADE,
  document_id UUID NOT NULL REFERENCES cortex.documents (id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT 'default',
  payload JSONB NOT NULL,
  version INT8 NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL DEFAULT NULL,
  PRIMARY KEY (id, created_at),
  CONSTRAINT unique_artifact_per_doc UNIQUE (document_id, artifact_type, NAME, created_at)
);

COMMIT;

-- +goose Down
DROP TABLE cortex.artifacts CASCADE;

DROP TABLE cortex.artifact_types CASCADE;

DROP TABLE cortex.documents CASCADE;

DROP TABLE cortex.publishers CASCADE;

DROP FUNCTION cortex.is_flat_annotation;

DROP SCHEMA IF EXISTS cortex CASCADE;

