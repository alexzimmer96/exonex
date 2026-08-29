package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexzimmer96/exonex/pkg"
	"github.com/alexzimmer96/exonex/pkg/sql"
	"github.com/alexzimmer96/exonex/pkg/storage"
	"github.com/google/uuid"
)

// =====================================================================================================================

// Document is the domain model for a stored document, including its file metadata
// and lifecycle state.
type Document struct {
	ID                  uuid.UUID      `db:"documents.id" json:"id"`
	Annotations         map[string]any `db:"documents.annotations" json:"annotations"`
	Finalizers          []string       `db:"documents.finalizers" json:"finalizers"`
	PublisherID         uuid.UUID      `db:"documents.publisher_id" json:"publisherID"`
	FileUploadCompleted bool           `db:"documents.file_upload_completed" json:"fileUploadCompleted"`
	FileMimeType        string         `db:"documents.file_mime_type" json:"fileMimeType"`
	FileSizeBytes       int64          `db:"documents.file_size_bytes" json:"fileSizeBytes"`
	FileStorageVolume   string         `db:"documents.file_storage_volume" json:"fileStorageVolume"`
	FileStorageKey      string         `db:"documents.file_storage_key" json:"fileStorageKey"`
	Version             int64          `db:"documents.version" json:"version"`
	CreatedAt           time.Time      `db:"documents.created_at" json:"createdAt"`
	UpdatedAt           time.Time      `db:"documents.updated_at" json:"updatedAt"`
	DeletedAt           *time.Time     `db:"documents.deleted_at" json:"deletedAt,omitempty"`
}

// =====================================================================================================================

// DocumentRepository is the persistence boundary for Documents, implemented by
// the repository layer.
type DocumentRepository interface {
	// ListDocuments returns documents matching the CEL filter expression,
	// projecting only the requested fields.
	ListDocuments(ctx context.Context, filter string, fields []string) ([]Document, error)
	// GetDocument returns a single document by ID, projecting only the requested fields.
	GetDocument(ctx context.Context, id uuid.UUID, fields []string) (*Document, error)
	// CreateDocument inserts a new document and returns the stored row.
	CreateDocument(ctx context.Context, doc Document) (*Document, error)
	// UpdateDocument persists the given document and returns the stored row.
	UpdateDocument(ctx context.Context, doc Document) (*Document, error)
}

// =====================================================================================================================

// DocumentService implements the business rules for documents on top of a DocumentRepository.
type DocumentService struct {
	documentRepo  DocumentRepository
	volumeManager *storage.VolumeManager
}

// NewDocumentService creates a DocumentService backed by the given repository.
func NewDocumentService(documentRepo DocumentRepository, volumeManager *storage.VolumeManager) *DocumentService {
	return &DocumentService{
		documentRepo:  documentRepo,
		volumeManager: volumeManager,
	}
}

// =====================================================================================================================

// ListDocumentsAction carries the inputs for listing documents.
//   - Filter is a CEL expression that represents the query to filter for relevant Documents.
//   - ReadMask is the list of fields that the caller expects to be returned.
type ListDocumentsAction struct {
	Filter   string   `json:"filter"`
	ReadMask []string `json:"readMask"`
}

// ListDocuments returns all documents matching the action's filter, projecting
// the requested fields.
func (svc *DocumentService) ListDocuments(ctx context.Context, action ListDocumentsAction) ([]Document, error) {
	docs, err := svc.documentRepo.ListDocuments(ctx, action.Filter, action.ReadMask)
	if err != nil {
		if fieldsErr, ok := errors.AsType[sql.UnknownFieldsError](err); ok {
			return nil, Error{
				Kind:    ErrorKindInvalidArgument,
				Message: fmt.Sprintf("field selector had unknown fields: %s", strings.Join(fieldsErr.Fields, ", ")),
				Wrapped: fieldsErr,
			}
		}
		slog.ErrorContext(ctx, "failed to list Documents from database", slog.String("error", err.Error()))
		return nil, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to list Documents from database",
			Wrapped: err,
		}
	}
	return docs, nil
}

// =====================================================================================================================

// GetDocument returns a single document by ID, projecting the given fields.
func (svc *DocumentService) GetDocument(ctx context.Context, id uuid.UUID, readMask []string) (*Document, error) {
	doc, err := svc.documentRepo.GetDocument(ctx, id, readMask)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Document from database", slog.String("error", err.Error()))
		return nil, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to get Document from database",
			Wrapped: err,
		}
	}
	return doc, nil
}

// =====================================================================================================================

// CreateDocumentAction carries the inputs for creating a document.
type CreateDocumentAction struct {
	Annotations   map[string]any `json:"annotations"`
	PublisherID   uuid.UUID      `json:"publisherID"`
	FileMimeType  string         `json:"fileMimeType"`
	FileSizeBytes int64          `json:"FileSizeBytes"`
	FileExtension string         `json:"FileExtension"`
}

// CreateDocument validates the file metadata, allocates a UUIDv7 ID, derives the
// storage location, and persists the new document.
func (svc *DocumentService) CreateDocument(ctx context.Context, action CreateDocumentAction) (*Document, error) {
	match, err := pkg.ExtensionMatchesMimeType(action.FileExtension, action.FileMimeType)
	if err != nil {
		return nil, Error{
			Kind:    ErrorKindInvalidArgument,
			Message: "invalid file MIME type",
			Wrapped: err,
		}
	}
	if !match {
		return nil, Error{
			Kind:    ErrorKindInvalidArgument,
			Message: "fileExtension does not match MIME type",
		}
	}

	docID, err := uuid.NewV7()
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to generate new UUID for document creation",
			slog.String("error", err.Error()),
		)
		return nil, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to generate UUID",
			Wrapped: err,
		}
	}

	newDoc := Document{
		ID:                docID,
		Annotations:       action.Annotations,
		PublisherID:       action.PublisherID,
		FileMimeType:      action.FileMimeType,
		FileSizeBytes:     action.FileSizeBytes,
		FileStorageVolume: "default",
		FileStorageKey:    svc.buildStorageKey(action.PublisherID, docID, action.FileExtension),
	}

	doc, err := svc.documentRepo.CreateDocument(ctx, newDoc)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to create new Document in database",
			slog.String("error", err.Error()),
		)
		return nil, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to create new Document in database",
			Wrapped: err,
		}
	}

	return doc, nil
}

// buildStorageKey derives the object-storage key for a document, partitioned by
// publisher and year to spread keys across prefixes.
func (svc *DocumentService) buildStorageKey(publisherId, documentId uuid.UUID, fileExtension string) string {
	return fmt.Sprintf("%s/%d/%s%s", publisherId.String(), time.Now().Year(), documentId, fileExtension)
}

// =====================================================================================================================

// CreateDocumentUploadURL creates a new Presigned Upload URL for the Document that can be used
// to upload the File directly to S3.
func (svc *DocumentService) CreateDocumentUploadURL(ctx context.Context, id uuid.UUID) (string, time.Time, error) {
	doc, err := svc.documentRepo.GetDocument(ctx, id, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Document from database", slog.String("error", err.Error()))
		return "", time.Time{}, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to get Document from database",
			Wrapped: err,
		}
	}

	if doc.FileUploadCompleted {
		return "", time.Time{}, Error{
			Kind:    ErrorKindFailedPrecondition,
			Message: "File upload for Document has already been completed",
		}
	}

	adapter, err := svc.volumeManager.GetAdapter(doc.FileStorageVolume)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to get storage Adapter for volume",
			slog.String("volume", doc.FileStorageVolume),
			slog.String("error", err.Error()),
		)
		return "", time.Time{}, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to generate upload URL for document",
			Wrapped: err,
		}
	}

	expiration := time.Now().Add(time.Hour)
	url, err := adapter.CreatePresignedUploadURL(ctx, storage.CreatePresignedUploadURLAction{
		Key:                 doc.FileStorageKey,
		Expiration:          expiration,
		ExpectedContentType: new(doc.FileMimeType),
		ExpectedFileSize:    new(doc.FileSizeBytes),
		IfNoneMatch:         new("*"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate upload URL for Document", slog.String("error", err.Error()))
		return "", time.Time{}, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to generate upload URL for document",
			Wrapped: err,
		}
	}
	return url, expiration, nil
}
