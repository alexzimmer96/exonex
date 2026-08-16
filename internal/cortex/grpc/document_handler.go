package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/alexzimmer96/exonex/internal/cortex/domain/document"
	v1alpha1 "github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1"
)

type DocumentHandler struct {
	documentSvc *document.Service
}

func NewDocumentHandler(documentSvc *document.Service) *DocumentHandler {
	return &DocumentHandler{
		documentSvc: documentSvc,
	}
}

func (d *DocumentHandler) ListDocuments(ctx context.Context, c *connect.Request[v1alpha1.ListDocumentsRequest]) (*connect.Response[v1alpha1.ListDocumentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}

func (d *DocumentHandler) GetDocument(ctx context.Context, c *connect.Request[v1alpha1.GetDocumentRequest]) (*connect.Response[v1alpha1.GetDocumentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}

func (d *DocumentHandler) CreateDocument(ctx context.Context, c *connect.Request[v1alpha1.CreateDocumentRequest]) (*connect.Response[v1alpha1.CreateDocumentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}

func (d *DocumentHandler) CreateDocumentUploadUrl(ctx context.Context, c *connect.Request[v1alpha1.CreateDocumentUploadUrlRequest]) (*connect.Response[v1alpha1.CreateDocumentUploadUrlResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}

func (d *DocumentHandler) UpdateDocument(ctx context.Context, c *connect.Request[v1alpha1.UpdateDocumentRequest]) (*connect.Response[v1alpha1.UpdateDocumentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("method is unimplemented"))
}
