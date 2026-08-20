package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexzimmer96/exonex/internal/cortex/domain"
	. "github.com/alexzimmer96/exonex/internal/dbschema/exonex/cortex/table"
	"github.com/alexzimmer96/exonex/pkg/sql"
	. "github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	InvalidDocumentFilterError = errors.New("invalid filter")
	UnknownDocumentFieldError  = errors.New("unknown field")
	DocumentNotFoundError      = errors.New("document not found")
)

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

var UserFieldMap = sql.FieldMap{
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
	"deletedAt":           Documents.DeletedAt,
}

// =====================================================================================================================

func (s *DocumentRepository) ListDocuments(ctx context.Context, filter string, fields []string) ([]domain.Document, error) {
	projection := UserFieldMap.BuildProjection(fields, Documents.AllColumns)
	stmt := Documents.SELECT(projection[0], projection[1:]...)

	if filter != "" {
		whereExpr, err := s.filterBuilder.BuildExpression(filter, UserFieldMap)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", InvalidDocumentFilterError, err)
		}
		stmt = stmt.WHERE(whereExpr)
	}

	query, args := stmt.Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[domain.Document])
}

// =====================================================================================================================

func (s *DocumentRepository) GetDocument(ctx context.Context, id uuid.UUID, fields []string) (*domain.Document, error) {
	projection := UserFieldMap.BuildProjection(fields, Documents.AllColumns)
	query, args := Documents.SELECT(projection[0], projection[1:]...).WHERE(Documents.ID.EQ(UUID(id))).Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}

// =====================================================================================================================

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
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}

// =====================================================================================================================

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
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}
