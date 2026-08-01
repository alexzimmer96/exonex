-- +goose Up
CREATE TYPE system_entity_type AS ENUM (
    'analysis', 'crawl_order', 'document', 'publisher', 'scrape_order'
);

CREATE TABLE meta_leases (
    entity_id UUID,
    entity_type system_entity_type,
    holder TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    renewed_at TIMESTAMP NOT NULL,
    PRIMARY KEY (entity_id, entity_type)
);

CREATE TABLE meta_log_events (
    entity_id UUID,
    entity_type system_entity_type,
    time TIMESTAMP NOT NULL,
    author TEXT NULL,
    message TEXT NOT NULL,
    retention_time TIMESTAMP NOT NULL,
    PRIMARY KEY (entity_id, entity_type)
);

-- +goose Down
DROP TABLE IF EXISTS log_events CASCADE;
DROP TABLE IF EXISTS leases CASCADE;
DROP TYPE IF EXISTS system_entity_type CASCADE;
