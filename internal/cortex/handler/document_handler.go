package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/alexzimmer96/exonex/internal/cortex/domain/document"
	"github.com/alexzimmer96/exonex/pkg"
	v1 "github.com/alexzimmer96/exonex/pkg/api/exonex/api/v1"
	v1alpha1 "github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1"
	"github.com/alexzimmer96/exonex/pkg/grpc"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DocumentHandler struct {
	documentSvc *document.Service
}

func NewDocumentHandler(documentSvc *document.Service) *DocumentHandler {
	return &DocumentHandler{
		documentSvc: documentSvc,
	}
}

// =====================================================================================================================

func (h *DocumentHandler) ListDocuments(ctx context.Context, c *connect.Request[v1alpha1.ListDocumentsRequest]) (*connect.Response[v1alpha1.ListDocumentsResponse], error) {
	action := document.ListDocumentsAction{
		Filter:   c.Msg.GetFilter(),
		ReadMask: grpc.ExtractReadMask(c),
	}
	docs, err := h.documentSvc.ListDocuments(ctx, action)
	if err != nil {
		return nil, err
	}
	mappedDocs, err := pkg.MapAllErr(docs, h.mapToApiDocument)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1alpha1.ListDocumentsResponse{
		Documents:     mappedDocs,
		NextPageToken: nil,
	}), nil
}

// =====================================================================================================================

func (h *DocumentHandler) GetDocument(ctx context.Context, c *connect.Request[v1alpha1.GetDocumentRequest]) (*connect.Response[v1alpha1.GetDocumentResponse], error) {
	documentId, err := uuid.Parse(c.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id field is malformed"))
	}
	doc, err := h.documentSvc.GetDocument(ctx, documentId, grpc.ExtractReadMask(c))
	if err != nil {
		return nil, err
	}
	mappedDoc, err := h.mapToApiDocument(*doc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	return connect.NewResponse(&v1alpha1.GetDocumentResponse{Document: mappedDoc}), nil
}

// =====================================================================================================================

func (h *DocumentHandler) CreateDocument(ctx context.Context, c *connect.Request[v1alpha1.CreateDocumentRequest]) (*connect.Response[v1alpha1.CreateDocumentResponse], error) {
	publisherID, err := uuid.Parse(c.Msg.GetPublisher())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("publisher_id field is malformed"))
	}

	action := document.CreateDocumentAction{
		Annotations:   c.Msg.Annotations.AsMap(),
		PublisherID:   publisherID,
		FileMimeType:  c.Msg.GetFileMimeType(),
		FileSizeBytes: c.Msg.GetFileSizeBytes(),
		FileExtension: c.Msg.GetFileExtension(),
	}

	doc, err := h.documentSvc.CreateDocument(ctx, action)
	if err != nil {
		return nil, err
	}

	mappedDoc, err := h.mapToApiDocument(*doc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	return connect.NewResponse(&v1alpha1.CreateDocumentResponse{Document: mappedDoc}), nil
}

// =====================================================================================================================

func (h *DocumentHandler) CreateDocumentUploadUrl(ctx context.Context, c *connect.Request[v1alpha1.CreateDocumentUploadUrlRequest]) (*connect.Response[v1alpha1.CreateDocumentUploadUrlResponse], error) {
	docID, err := uuid.Parse(c.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id field is malformed"))
	}
	url, expires, err := h.documentSvc.CreateDocumentUploadURL(ctx, docID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1alpha1.CreateDocumentUploadUrlResponse{
		UploadUrl: url,
		ExpiresAt: timestamppb.New(expires),
	}), nil
}

// =====================================================================================================================

func (h *DocumentHandler) UpdateDocument(ctx context.Context, c *connect.Request[v1alpha1.UpdateDocumentRequest]) (*connect.Response[v1alpha1.UpdateDocumentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}

// =====================================================================================================================

func (h *DocumentHandler) mapToApiDocument(in document.Document) (*v1alpha1.Document, error) {
	annotations, err := structpb.NewStruct(in.Annotations)
	if err != nil {
		return nil, err
	}
	var deletedAt *timestamppb.Timestamp
	if in.DeletedAt != nil {
		deletedAt = timestamppb.New(*in.DeletedAt)
	}
	return &v1alpha1.Document{
		Meta: &v1.ResourceMeta{
			Annotations: annotations,
			Finalizers:  in.Finalizers,
			Version:     in.Version,
			CreatedAt:   timestamppb.New(in.CreatedAt),
			UpdatedAt:   timestamppb.New(in.UpdatedAt),
			DeletedAt:   deletedAt,
		},
		Id:                  in.ID.String(),
		Publisher:           in.PublisherID.String(),
		FileUploadCompleted: in.FileUploadCompleted,
		FileMimeType:        in.FileMimeType,
		FileSizeBytes:       in.FileSizeBytes,
		FileStorageVolume:   in.FileStorageVolume,
		FileStorageKey:      in.FileStorageKey,
	}, nil
}
