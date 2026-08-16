package repository

import (
	"context"

	"github.com/alexzimmer96/exonex/internal/cortex/domain/document"
	. "github.com/alexzimmer96/exonex/internal/db/exonex/cortex/table"
	"github.com/alexzimmer96/exonex/pkg/sql"
	. "github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentRepository struct {
	pool *pgxpool.Pool
}

func (s *DocumentRepository) ListDocuments(ctx context.Context, limit, offset int64) ([]document.Document, error) {
	driver := sql.TxFromContext(ctx, s.pool)
	query, args := SELECT(Documents.AllColumns).
		FROM(Documents).
		ORDER_BY(Documents.CreatedAt).
		LIMIT(limit).
		OFFSET(offset).
		Sql()

	rows, err := driver.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[document.Document])
}

func (s *DocumentRepository) CreateDocument(ctx context.Context, doc document.Document) (*document.Document, error) {
	driver := sql.TxFromContext(ctx, s.pool)
	query, args := Documents.INSERT(
		Documents.ID,
		Documents.PublisherID,
		Documents.MimeType,
		Documents.SizeBytes,
		Documents.StorageVolume,
		Documents.StorageKey,
		Documents.SourceURL,
	).VALUES(
		doc.ID,
		doc.PublisherID,
		doc.MimeType,
		doc.SizeBytes,
		doc.StorageVolume,
		doc.StorageKey,
		doc.SourceURL,
	).Sql()

	rows, err := driver.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[*document.Document])
}
