package auth

type Permission string

const (
	PermissionDocumentsRead            = Permission("documents.read")
	PermissionDocumentsCreate          = Permission("documents.create")
	PermissionDocumentsUploadUrlCreate = Permission("documents.upload_url.create")
	PermissionDocumentsUpdate          = Permission("documents.update")
	PermissionDocumentsDelete          = Permission("documents.delete")
)

const (
	PermissionDocumentUpdateFileUploadCompletedField = Permission("documents.update/file_upload_completed")
	PermissionDocumentUpdateFileSizeBytesField       = Permission("documents.update/file_size_bytes")
)
