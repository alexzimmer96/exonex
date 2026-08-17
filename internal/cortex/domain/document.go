package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alexzimmer96/exonex/pkg"
	"github.com/google/uuid"
)

// =====================================================================================================================

type Document struct {
	ID                  uuid.UUID      `db:"documents.id" json:"id"`
	Annotations         map[string]any `db:"documents.annotations" json:"annotations"`
	Finalizers          []string       `db:"documents.finalizers" json:"Finalizers"`
	PublisherID         uuid.UUID      `db:"documents.publisher_id" json:"publisher_id"`
	FileUploadCompleted bool           `db:"documents.file_upload_completed" json:"file_upload_completed"`
	FileMimeType        string         `db:"documents.file_mime_type" json:"file_mime_type"`
	FileSizeBytes       int64          `db:"documents.file_size_bytes" json:"file_size_bytes"`
	FileStorageVolume   string         `db:"documents.file_storage_volume" json:"file_storage_volume"`
	FileStorageKey      string         `db:"documents.file_storage_key" json:"file_storage_key"`
	Version             int64          `db:"documents.version" json:"version"`
	CreatedAt           time.Time      `db:"documents.created_at" json:"created_at"`
	UpdatedAt           time.Time      `db:"documents.updated_at" json:"updated_at"`
	DeletedAt           *time.Time     `db:"documents.deleted_at" json:"deleted_at,omitempty"`
}

// =====================================================================================================================

type DocumentRepository interface {
	ListDocuments(ctx context.Context, filter string) ([]Document, error)
	CreateDocument(ctx context.Context, doc Document) (*Document, error)
	UpdateDocument(ctx context.Context, doc Document) (*Document, error)
}

// =====================================================================================================================

type DocumentService struct {
	documentRepo DocumentRepository
}

func NewDocumentService(documentRepo DocumentRepository) *DocumentService {
	return &DocumentService{
		documentRepo: documentRepo,
	}
}

// =====================================================================================================================

type ListDocumentsAction struct {
	Filter string `json:"filter"`
}

func (svc *DocumentService) ListDocuments(ctx context.Context, action ListDocumentsAction) ([]Document, error) {
	docs, err := svc.documentRepo.ListDocuments(ctx, action.Filter)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to list Documents from database",
			slog.String("error", err.Error()),
		)
		return nil, Error{
			Kind:    ErrorKindInternal,
			Message: "failed to list new Documents from database",
			Wrapped: err,
		}
	}
	return docs, nil
}

// =====================================================================================================================

type CreateDocumentAction struct {
	Annotations   map[string]any `json:"annotations"`
	PublisherID   uuid.UUID      `json:"publisher_id"`
	FileMimeType  string         `json:"file_mime_type"`
	FileSizeBytes int64          `json:"file_size_bytes"`
	FileExtension string         `json:"file_extension"`
}

func (svc *DocumentService) CreateDocument(ctx context.Context, action CreateDocumentAction) (*Document, error) {
	match, err := pkg.ExtensionMatchesMimeType(action.FileExtension, action.FileMimeType)
	if err != nil {
		return nil, Error{
			Kind:    ErrorKindInvalidArgument,
			Message: "failed to generate UUID",
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

func (svc *DocumentService) buildStorageKey(publisherId, documentId uuid.UUID, fileExtension string) string {
	return fmt.Sprintf("/%s/%d/%s%s", publisherId.String(), time.Now().Year(), documentId, fileExtension)
}
