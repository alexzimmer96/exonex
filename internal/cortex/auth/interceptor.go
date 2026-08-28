package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	apiv1 "github.com/alexzimmer96/exonex/pkg/api/exonex/api/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func CreateAuthContextInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Spec().IsClient {
				slog.WarnContext(ctx, "AuthContextInterceptor should not be applied to clients")
				return next(ctx, req)
			}
			ctx = NewContext(ctx, WithRoles([]Role{RoleUnauthenticated}))
			return next(ctx, req)
		}
	}
}

func CreateMethodPermissionInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Spec().IsClient {
				slog.WarnContext(ctx, "MethodPermissionInterceptor should not be applied to clients")
				return next(ctx, req)
			}
			proc := req.Spec().Procedure
			fullName := protoreflect.FullName(strings.ReplaceAll(strings.TrimPrefix(proc, "/"), "/", "."))
			desc, err := protoregistry.GlobalFiles.FindDescriptorByName(fullName)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("descriptor not found: %w", err))
			}

			methodDesc, ok := desc.(protoreflect.MethodDescriptor)
			if !ok {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("descriptor is not a method"))
			}

			opts := methodDesc.Options().(*descriptorpb.MethodOptions)
			if proto.HasExtension(opts, apiv1.E_MethodPermission) {
				requiredPerm := proto.GetExtension(opts, apiv1.E_MethodPermission).(string)
				authCtx, err := GetAuthContext(ctx)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, errors.New(""))
				}
				if !authCtx.HasPermission(requiredPerm) {
					return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission '%s' required to perform this action", requiredPerm))
				}
			}

			return next(ctx, req)
		}
	}
}
