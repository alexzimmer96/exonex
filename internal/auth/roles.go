package auth

type Role string

func GetRolePermissions(role Role) []Permission {
	permissions, ok := rolePermissions[role]
	if !ok {
		return []Permission{}
	}
	return permissions
}

const (
	RoleUnauthenticated = Role("unauthenticated")
)

var rolePermissions = map[Role][]Permission{
	RoleUnauthenticated: {
		PermissionDocumentsRead,
		PermissionDocumentsCreate,
		PermissionDocumentsUpdate,
		PermissionDocumentsUpdate,
		PermissionDocumentsDelete,
		PermissionDocumentsUploadUrlCreate,
		PermissionDocumentUpdateFileUploadCompletedField,
		PermissionDocumentUpdateFileSizeBytesField,
	},
}
