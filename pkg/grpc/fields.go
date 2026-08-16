package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alexzimmer96/exonex/pkg"
	apiv1 "github.com/alexzimmer96/exonex/pkg/api/exonex/api/v1"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	annotationsSchema = jsonschema.MustCompileString("annotations.json", pkg.AnnotationsSchema)
)

func VerifyFieldTypes(ctx context.Context, msg proto.Message) []errdetails.BadRequest_FieldViolation {
	var violations []errdetails.BadRequest_FieldViolation
	msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		opts := fd.Options()
		if proto.HasExtension(opts, apiv1.E_FieldType) {
			fieldType, ok := proto.GetExtension(opts, apiv1.E_FieldType).(apiv1.FieldType)
			if !ok {
				slog.WarnContext(ctx, "could not parse value of 'exonex.api.v1.field_type' option to value")
				return true
			}
			if fieldType == apiv1.FieldType_FIELD_TYPE_ANNOTATIONS {
				violation := verifyAnnotations(value)
				if violation == nil {
					return true
				}
				violations = append(violations, errdetails.BadRequest_FieldViolation{
					Field:       string(fd.Name()),
					Description: violation.Error(),
				})
			}
		}
		return true
	})
	return nil
}

func verifyAnnotations(value protoreflect.Value) error {
	jsonValue, err := protojson.Marshal(value.Interface().(*structpb.Value))
	if err != nil {
		return fmt.Errorf("could not parse value to annotation object: %w", err)
	}
	return annotationsSchema.Validate(jsonValue)
}
