FROM postgres:18.4

COPY --from=ghcr.io/pglayers/pgx-pg_jsonschema:18-v0.3.4 / /extensions/pg_jsonschema/
COPY --from=ghcr.io/pglayers/pgx-pg_partman:18-5.5.0    / /extensions/pg_partman/
COPY --from=ghcr.io/pglayers/pgx-pgvector:18-0.8.5      / /extensions/pgvector/

CMD ["postgres", \
     "-c", "extension_control_path=/extensions/pg_jsonschema/share:/extensions/pg_partman/share:/extensions/pgvector/share:$system", \
     "-c", "dynamic_library_path=/extensions/pg_jsonschema/lib:/extensions/pg_partman/lib:/extensions/pgvector/lib:$libdir"]