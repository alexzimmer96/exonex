package grpc

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

func CreateFieldTypeValidationInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Spec().IsClient {
				slog.WarnContext(ctx, "FieldTypeValidationInterceptor should not be applied to clients")
				return next(ctx, req)
			}
			if msg, ok := req.Any().(proto.Message); ok && msg != nil {
				val := reflect.ValueOf(msg)
				if val.Kind() == reflect.Pointer && val.IsNil() {
					return next(ctx, req)
				}
				validationResult := validateFieldTypes(ctx, msg)
				if validationResult == nil {
					return next(ctx, req)
				}
				respErr := connect.NewError(
					connect.CodeInvalidArgument,
					errors.New("invalid arguments provided in request body"),
				)
				detail, err := connect.NewErrorDetail(validationResult)
				if err != nil {
					slog.ErrorContext(ctx, "failed to create error detail", slog.String("error", err.Error()))
				} else {
					respErr.AddDetail(detail)
				}
				return nil, respErr
			}
			return next(ctx, req)
		}
	}
}
