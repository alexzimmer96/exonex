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
	InvalidFieldSelectorError  = errors.New("invalid field selector")
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

// =====================================================================================================================

// UserFieldMap contains the mapping between the Document attributes and database fields.
var UserFieldMap = sql.FieldMap{
	"id":                  sql.AsUUID(Documents.ID),
	"meta.annotations":    Documents.Annotations,
	"meta.finalizers":     Documents.Finalizers,
	"publisher":           sql.AsUUID(Documents.PublisherID),
	"fileUploadCompleted": Documents.FileUploadCompleted,
	"fileMimeType":        Documents.FileMimeType,
	"fileSizeBytes":       Documents.FileSizeBytes,
	"fileStorageVolume":   Documents.FileStorageVolume,
	"fileStorageKey":      Documents.FileStorageKey,
	"meta.createdAt":      Documents.CreatedAt,
	"meta.updatedAt":      Documents.UpdatedAt,
	"meta.deletedAt":      Documents.DeletedAt,
}

// buildProjection resolves the requested fields to columns, falling back to all
// columns when nothing was requested or no requested field is known.
func (s *DocumentRepository) buildProjection(fields []string) ([]Projection, error) {
	projection, err := UserFieldMap.BuildProjection(fields, Documents.AllColumns)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", InvalidFieldSelectorError, err)
	}
	if len(projection) == 0 {
		return []Projection{Documents.AllColumns}, nil
	}
	return projection, nil
}

// =====================================================================================================================

func (s *DocumentRepository) ListDocuments(ctx context.Context, filter string, fields []string) ([]domain.Document, error) {
	stmt, err := s.buildListQuery(filter, fields)
	if err != nil {
		return nil, err
	}

	query, args := stmt.Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[domain.Document])
}

// buildListQuery takes a filter and list of fields that should be returned and builds the respective SQL query.
func (s *DocumentRepository) buildListQuery(filter string, fields []string) (SelectStatement, error) {
	projection, err := s.buildProjection(fields)
	if err != nil {
		return nil, err
	}

	stmt := Documents.SELECT(projection[0], projection[1:]...)

	if filter != "" {
		whereExpr, err := s.filterBuilder.BuildExpression(filter, UserFieldMap)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", InvalidDocumentFilterError, err)
		}
		stmt = stmt.WHERE(whereExpr)
	}

	return stmt, nil
}

// =====================================================================================================================

func (s *DocumentRepository) GetDocument(ctx context.Context, id uuid.UUID, fields []string) (*domain.Document, error) {
	stmt, err := s.buildGetQuery(id, fields)
	if err != nil {
		return nil, err
	}

	query, args := stmt.Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}

func (s *DocumentRepository) buildGetQuery(id uuid.UUID, fields []string) (SelectStatement, error) {
	projection, err := s.buildProjection(fields)
	if err != nil {
		return nil, err
	}
	return Documents.SELECT(projection[0], projection[1:]...).WHERE(Documents.ID.EQ(UUID(id))), nil
}

// =====================================================================================================================

func (s *DocumentRepository) CreateDocument(ctx context.Context, doc domain.Document) (*domain.Document, error) {
	query, args := s.buildCreateQuery(doc).Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}

func (s *DocumentRepository) buildCreateQuery(doc domain.Document) InsertStatement {
	return Documents.INSERT(
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
	).RETURNING(Documents.AllColumns)
}

// =====================================================================================================================

func (s *DocumentRepository) UpdateDocument(ctx context.Context, doc domain.Document) (*domain.Document, error) {
	query, args := s.buildUpdateQuery(doc).Sql()
	rows, err := sql.TxFromContext(ctx, s.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[domain.Document])
}

func (s *DocumentRepository) buildUpdateQuery(doc domain.Document) UpdateStatement {
	return Documents.UPDATE(
		Documents.FileUploadCompleted,
		Documents.FileSizeBytes,
		Documents.UpdatedAt,
		Documents.Version,
	).SET(
		doc.FileUploadCompleted,
		doc.FileSizeBytes,
		NOW(),
		Documents.Version.ADD(Int64(1)),
	).WHERE(Documents.ID.EQ(UUID(doc.ID))).RETURNING(Documents.AllColumns)
}
