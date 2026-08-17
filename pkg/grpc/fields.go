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
)

var (
	annotationsSchema = jsonschema.MustCompileString("annotations.json", pkg.AnnotationsSchema)
)

func validateFieldTypes(ctx context.Context, msg proto.Message) *errdetails.BadRequest {
	if msg == nil {
		return nil
	}
	var violations []*errdetails.BadRequest_FieldViolation
	validateFieldsRecursive(ctx, msg.ProtoReflect(), "", &violations)
	if len(violations) == 0 {
		return nil
	}
	return &errdetails.BadRequest{
		FieldViolations: violations,
	}
}

func validateFieldsRecursive(
	ctx context.Context,
	pMsg protoreflect.Message,
	pathPrefix string,
	violations *[]*errdetails.BadRequest_FieldViolation,
) {
	fields := pMsg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		fieldPath := fd.JSONName()
		if pathPrefix != "" {
			fieldPath = pathPrefix + "." + fd.JSONName()
		}

		opts := fd.Options()
		if proto.HasExtension(opts, apiv1.E_FieldType) {
			fieldType, ok := proto.GetExtension(opts, apiv1.E_FieldType).(apiv1.FieldType)
			if !ok {
				slog.WarnContext(ctx, "could not parse value of 'exonex.api.v1.field_type' option", "field", fieldPath)
			} else if fieldType == apiv1.FieldType_FIELD_TYPE_ANNOTATIONS {
				if pMsg.Has(fd) {
					val := pMsg.Get(fd)
					if err := verifyAnnotations(val); err != nil {
						*violations = append(*violations, &errdetails.BadRequest_FieldViolation{
							Field:       fieldPath,
							Description: err.Error(),
						})
					}
				}
			}
		}

		if fd.Kind() == protoreflect.MessageKind {
			if !pMsg.Has(fd) {
				continue
			}
			switch {
			case fd.IsList():
				list := pMsg.Get(fd).List()
				for j := 0; j < list.Len(); j++ {
					elem := list.Get(j)
					elemPath := fmt.Sprintf("%s[%d]", fieldPath, j)
					validateFieldsRecursive(ctx, elem.Message(), elemPath, violations)
				}
			case fd.IsMap():
				if fd.MapValue().Kind() == protoreflect.MessageKind {
					mapVal := pMsg.Get(fd).Map()
					mapVal.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
						elemPath := fmt.Sprintf("%s[%v]", fieldPath, k.Interface())
						validateFieldsRecursive(ctx, v.Message(), elemPath, violations)
						return true
					})
				}
			default:
				subMsg := pMsg.Get(fd).Message()
				validateFieldsRecursive(ctx, subMsg, fieldPath, violations)
			}
		}
	}
}

func verifyAnnotations(value protoreflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	protoMsg := value.Message().Interface()
	jsonValue, err := protojson.Marshal(protoMsg)
	if err != nil {
		return fmt.Errorf("could not parse value to annotation object: %w", err)
	}
	if len(jsonValue) == 0 || string(jsonValue) == "null" {
		return nil
	}
	return annotationsSchema.Validate(jsonValue)
}
