package repository

import (
	"context"
	"fmt"

	"github.com/alexzimmer96/exonex/internal/cortex/domain"
	. "github.com/alexzimmer96/exonex/internal/dbschema/exonex/cortex/table"
	"github.com/alexzimmer96/exonex/pkg/sql"
	. "github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var UserFieldMap = sql.CELFieldMap{
	"id":                  sql.AsUUID(Documents.ID),
	"annotations":         Documents.Annotations,
	"finalizers":          Documents.Finalizers,
	"publisher":           sql.AsUUID(Documents.PublisherID),
	"fileUploadCompleted": Documents.FileUploadCompleted,
	"fileMimeType":        Documents.FileMimeType,
	"fileSizeBytes":       Documents.FileSizeBytes,
	"fileStorageVolume":   Documents.FileStorageVolume,
	"fileStorageKey":      Documents.FileStorageKey,
	"createdAt":           Documents.CreatedAt,
	"updatedAt":           Documents.UpdatedAt,
}

type DocumentRepository struct {
	pool          *pgxpool.Pool
	filterBuilder *sql.FilterBuilder
}

func NewDocumentRepository(pool *pgxpool.Pool, filterBuilder *sql.FilterBuilder) *DocumentRepository {
	return &DocumentRepository{
		pool:          pool,
		filterBuilder: filterBuilder,
	}
}

func (s *DocumentRepository) ListDocuments(ctx context.Context, filter string) ([]domain.Document, error) {
	stmt := Documents.SELECT(Documents.AllColumns)

	if filter != "" {
		whereExpr, err := s.filterBuilder.BuildExpression(filter, UserFieldMap)
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		stmt = stmt.WHERE(whereExpr)
	}

	query, args := stmt.Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.Document])
}

func (s *DocumentRepository) CreateDocument(ctx context.Context, doc domain.Document) (*domain.Document, error) {
	query, args := Documents.INSERT(
		Documents.ID,
		Documents.Annotations,
		Documents.PublisherID,
		Documents.FileMimeType,
		Documents.FileSizeBytes,
		Documents.FileStorageVolume,
		Documents.FileStorageKey,
	).VALUES(
		doc.ID,
		doc.Annotations,
		doc.PublisherID,
		doc.FileMimeType,
		doc.FileSizeBytes,
		doc.FileStorageVolume,
		doc.FileStorageKey,
	).RETURNING(Documents.AllColumns).Sql()

	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Document])
}

func (s *DocumentRepository) UpdateDocument(ctx context.Context, doc domain.Document) (*domain.Document, error) {
	query, args := Documents.UPDATE(
		Documents.FileUploadCompleted,
		Documents.FileSizeBytes,
		Documents.UpdatedAt,
		Documents.Version,
	).SET(
		doc.FileUploadCompleted,
		doc.FileSizeBytes,
		NOW(),
		Documents.Version.ADD(Int64(1)),
	).WHERE(Documents.ID.EQ(UUID(doc.ID))).RETURNING(Documents.AllColumns).Sql()

	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Document])
}
