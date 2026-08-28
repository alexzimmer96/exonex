package auth

import (
	"context"
	"errors"
	"maps"
	"slices"
)

type contextKey string

const authContextKey = "authContext"

var (
	ErrAuthContextNotFound = errors.New("AuthContext not found in Context")
)

type ContextOpt func(authCtx *contextImpl)

type Context interface {
	GetRoles() []Role
	GetPermissions() []Permission
	HasPermission(string) bool
}

// =====================================================================================================================

type contextImpl struct {
	roles       []Role
	permissions map[Permission]struct{}
}

func (a contextImpl) GetRoles() []Role {
	return a.roles
}

func (a contextImpl) GetPermissions() []Permission {
	return slices.Collect(maps.Keys(a.permissions))
}

func (a contextImpl) HasPermission(perm string) bool {
	_, ok := a.permissions[Permission(perm)]
	return ok
}

// =====================================================================================================================

func WithRoles(roles []Role) ContextOpt {
	return func(ctx *contextImpl) {
		ctx.roles = append(ctx.roles, roles...)
	}
}

func NewContext(ctx context.Context, opts ...ContextOpt) context.Context {
	authCtx := &contextImpl{}
	for _, opt := range opts {
		opt(authCtx)
	}

	permissions := map[Permission]struct{}{}
	for _, role := range authCtx.roles {
		for _, perm := range GetRolePermissions(role) {
			permissions[perm] = struct{}{}
		}
	}
	authCtx.permissions = permissions

	return context.WithValue(ctx, authContextKey, *authCtx)
}

func GetAuthContext(ctx context.Context) (Context, error) {
	authCtx, ok := ctx.Value(authContextKey).(contextImpl)
	if !ok {
		return nil, ErrAuthContextNotFound
	}
	return authCtx, nil
}
