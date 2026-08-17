package handler

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"github.com/alexzimmer96/exonex/internal/auth"
	"github.com/alexzimmer96/exonex/pkg"
	v1alpha1 "github.com/alexzimmer96/exonex/pkg/api/exonex/cortex/v1alpha1"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (a *AuthHandler) WhoAmI(ctx context.Context, c *connect.Request[v1alpha1.WhoAmIRequest]) (*connect.Response[v1alpha1.WhoAmIResponse], error) {
	authCtx, err := auth.GetAuthContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	activeRoles := pkg.MapAll(authCtx.GetRoles(), func(s auth.Role) string {
		return string(s)
	})
	slices.Sort(activeRoles)

	activePermissions := pkg.MapAll(authCtx.GetPermissions(), func(s auth.Permission) string {
		return string(s)
	})
	slices.Sort(activePermissions)

	return connect.NewResponse(&v1alpha1.WhoAmIResponse{
		ActiveRoles:       activeRoles,
		ActivePermissions: activePermissions,
	}), nil
}
