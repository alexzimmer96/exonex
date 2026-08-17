package sql

import (
	"fmt"
	"time"

	jet "github.com/go-jet/jet/v2/postgres"
	"github.com/google/cel-go/cel"
	"github.com/google/uuid"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

type CELFieldMap map[string]jet.Column

type FilterBuilder struct {
	env *cel.Env
}

type ColumnUUID interface {
	jet.ColumnString
	IsUUID() bool
}

type columnUUID struct {
	jet.ColumnString
}

func (c columnUUID) IsUUID() bool {
	return true
}

func AsUUID(c jet.ColumnString) ColumnUUID {
	return columnUUID{c}
}

func NewFilterBuilder() (*FilterBuilder, error) {
	env, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	return &FilterBuilder{env: env}, nil
}

func (b *FilterBuilder) BuildExpression(filterStr string, mapping CELFieldMap) (jet.BoolExpression, error) {
	if filterStr == "" {
		return nil, nil
	}

	ast, iss := b.env.Parse(filterStr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("invalid CEL expression: %w", iss.Err())
	}

	return walkAST(ast.Expr(), mapping)
}

func walkAST(e *exprpb.Expr, mapping CELFieldMap) (jet.BoolExpression, error) {
	switch kind := e.ExprKind.(type) {
	case *exprpb.Expr_CallExpr:
		return parseCall(kind.CallExpr, mapping)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", kind)
	}
}

func parseCall(call *exprpb.Expr_Call, mapping CELFieldMap) (jet.BoolExpression, error) {
	switch call.Function {
	case "_&&_":
		left, err := walkAST(call.Args[0], mapping)
		if err != nil {
			return nil, err
		}
		right, err := walkAST(call.Args[1], mapping)
		if err != nil {
			return nil, err
		}
		return left.AND(right), nil

	case "_||_":
		left, err := walkAST(call.Args[0], mapping)
		if err != nil {
			return nil, err
		}
		right, err := walkAST(call.Args[1], mapping)
		if err != nil {
			return nil, err
		}
		return left.OR(right), nil

	case "@in":
		return parseInOperator(call.Args, mapping)

	case "_==_", "_!=_", "_>_", "_>=_", "_<_", "_<=_":
		return createComparison(call.Function, call.Args, mapping)

	default:
		return nil, fmt.Errorf("unsupported operator: %s", call.Function)
	}
}

func parseInOperator(args []*exprpb.Expr, mapping CELFieldMap) (jet.BoolExpression, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("invalid @in arguments")
	}

	left := args[0]
	right := args[1]

	if listExpr := right.GetListExpr(); listExpr != nil {
		return createSQLInList(left, listExpr, mapping)
	}

	if selectExpr := right.GetSelectExpr(); selectExpr != nil {
		return createJSONContains(selectExpr, left, mapping)
	}

	if identExpr := right.GetIdentExpr(); identExpr != nil {
		col, ok := mapping[identExpr.Name]
		if !ok {
			return nil, fmt.Errorf("unknown field: %s", identExpr.Name)
		}
		return createNativeArrayContains(left, col)
	}

	return nil, fmt.Errorf("unsupported right-hand expression for 'in' operator")
}

func createSQLInList(left *exprpb.Expr, listExpr *exprpb.Expr_CreateList, mapping CELFieldMap) (jet.BoolExpression, error) {
	ident := left.GetIdentExpr().Name
	col, ok := mapping[ident]
	if !ok {
		return nil, fmt.Errorf("unknown field: %s", ident)
	}

	switch c := col.(type) {
	case ColumnUUID:
		var expressions []jet.Expression
		for _, elem := range listExpr.Elements {
			val := elem.GetConstExpr().GetStringValue()
			parsed, err := uuid.Parse(val)
			if err != nil {
				return nil, fmt.Errorf("invalid UUID in list for field %s: %w", ident, err)
			}
			expressions = append(expressions, jet.UUID(parsed))
		}
		return c.IN(expressions...), nil
	case jet.ColumnString:
		var expressions []jet.Expression
		for _, elem := range listExpr.Elements {
			expressions = append(expressions, jet.String(elem.GetConstExpr().GetStringValue()))
		}
		return c.IN(expressions...), nil

	case jet.ColumnInteger:
		var expressions []jet.Expression
		for _, elem := range listExpr.Elements {
			expressions = append(expressions, jet.Int(elem.GetConstExpr().GetInt64Value()))
		}
		return c.IN(expressions...), nil

	default:
		return nil, fmt.Errorf("IN list not supported for column type %T", col)
	}
}

func createNativeArrayContains(left *exprpb.Expr, arrayCol jet.Column) (jet.BoolExpression, error) {
	constVal := left.GetConstExpr()
	if constVal == nil {
		return nil, fmt.Errorf("left side of array search must be a constant")
	}

	var valExpr jet.Expression
	switch v := constVal.ConstantKind.(type) {
	case *exprpb.Constant_StringValue:
		valExpr = jet.String(v.StringValue)
	case *exprpb.Constant_Int64Value:
		valExpr = jet.Int(v.Int64Value)
	case *exprpb.Constant_BoolValue:
		valExpr = jet.Bool(v.BoolValue)
	default:
		return nil, fmt.Errorf("unsupported array element type: %T", v)
	}

	return jet.RawBool("#val# = ANY(#col#)", jet.RawArgs{
		"#val#": valExpr,
		"#col#": arrayCol,
	}), nil
}

func createJSONContains(selectExpr *exprpb.Expr_Select, valueExpr *exprpb.Expr, mapping CELFieldMap) (jet.BoolExpression, error) {
	baseField := selectExpr.Operand.GetIdentExpr().Name
	jsonKey := selectExpr.Field

	baseCol, ok := mapping[baseField]
	if !ok {
		return nil, fmt.Errorf("unknown base field: %s", baseField)
	}

	constVal := valueExpr.GetConstExpr()
	var jsonQuery string

	switch v := constVal.ConstantKind.(type) {
	case *exprpb.Constant_StringValue:
		jsonQuery = fmt.Sprintf(`{"%s": ["%s"]}`, jsonKey, v.StringValue)
	case *exprpb.Constant_Int64Value:
		jsonQuery = fmt.Sprintf(`{"%s": [%d]}`, jsonKey, v.Int64Value)
	case *exprpb.Constant_BoolValue:
		jsonQuery = fmt.Sprintf(`{"%s": [%t]}`, jsonKey, v.BoolValue)
	default:
		return nil, fmt.Errorf("unsupported type for json array contains")
	}

	return jet.RawBool("#baseCol# @> #jsonQuery#::jsonb", jet.RawArgs{
		"#baseCol#":   baseCol,
		"#jsonQuery#": jet.String(jsonQuery),
	}), nil
}

func createComparison(op string, args []*exprpb.Expr, mapping CELFieldMap) (jet.BoolExpression, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("invalid operator arguments")
	}

	leftExpr := args[0]
	rightExpr := args[1]

	if selectExpr := leftExpr.GetSelectExpr(); selectExpr != nil {
		baseField := selectExpr.Operand.GetIdentExpr().Name
		jsonKey := selectExpr.Field
		baseCol, ok := mapping[baseField]
		if !ok {
			return nil, fmt.Errorf("unknown base field: %s", baseField)
		}
		return createJSONComparison(op, baseCol, jsonKey, rightExpr)
	}

	ident := args[0].GetIdentExpr().Name
	col, ok := mapping[ident]
	if !ok {
		return nil, fmt.Errorf("unknown field: %s", ident)
	}

	valExpr := args[1].GetConstExpr()

	switch c := col.(type) {
	case ColumnUUID:
		val := valExpr.GetStringValue()
		parsedUUID, err := uuid.Parse(val)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID format for field %s: %w", ident, err)
		}
		uuidLiteral := jet.UUID(parsedUUID)
		switch op {
		case "_==_":
			return c.EQ(uuidLiteral), nil
		case "_!=_":
			return c.NOT_EQ(uuidLiteral), nil
		}
	case jet.ColumnString:
		val := valExpr.GetStringValue()
		switch op {
		case "_==_":
			return c.EQ(jet.String(val)), nil
		case "_!=_":
			return c.NOT_EQ(jet.String(val)), nil
		case "_>_":
			return c.GT(jet.String(val)), nil
		case "_<_":
			return c.LT(jet.String(val)), nil
		}

	case jet.ColumnInteger:
		val := valExpr.GetInt64Value()
		switch op {
		case "_==_":
			return c.EQ(jet.Int(val)), nil
		case "_!=_":
			return c.NOT_EQ(jet.Int(val)), nil
		case "_>_":
			return c.GT(jet.Int(val)), nil
		case "_>=_":
			return c.GT_EQ(jet.Int(val)), nil
		case "_<_":
			return c.LT(jet.Int(val)), nil
		case "_<=_":
			return c.LT_EQ(jet.Int(val)), nil
		}

	case jet.ColumnBool:
		val := valExpr.GetBoolValue()
		switch op {
		case "_==_":
			return c.EQ(jet.Bool(val)), nil
		case "_!=_":
			return c.NOT_EQ(jet.Bool(val)), nil
		}

	case jet.ColumnTimestamp:
		t, err := time.Parse(time.RFC3339, valExpr.GetStringValue())
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp format for %s: %w", ident, err)
		}
		tsVal := jet.TimestampT(t)
		switch op {
		case "_==_":
			return c.EQ(tsVal), nil
		case "_!=_":
			return c.NOT_EQ(tsVal), nil
		case "_>_":
			return c.GT(tsVal), nil
		case "_>=_":
			return c.GT_EQ(tsVal), nil
		case "_<_":
			return c.LT(tsVal), nil
		case "_<=_":
			return c.LT_EQ(tsVal), nil
		}

	default:
		return nil, fmt.Errorf("unsupported column type %T for field %s", col, ident)
	}

	return nil, fmt.Errorf("operator %s not supported for field %s", op, ident)
}

func createJSONComparison(op string, baseCol jet.Column, jsonKey string, rightExpr *exprpb.Expr) (jet.BoolExpression, error) {
	constVal := rightExpr.GetConstExpr()
	switch v := constVal.ConstantKind.(type) {
	case *exprpb.Constant_StringValue:
		val := v.StringValue
		jsonTextVal := jet.RawString("(#baseCol# ->> #key#)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case "_==_":
			return jsonTextVal.EQ(jet.String(val)), nil
		case "_!=_":
			return jsonTextVal.NOT_EQ(jet.String(val)), nil
		}

	case *exprpb.Constant_Int64Value:
		val := v.Int64Value
		jsonNumVal := jet.RawFloat("((#baseCol# ->> #key#)::numeric)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case "_==_":
			return jsonNumVal.EQ(jet.Float(float64(val))), nil
		case "_!=_":
			return jsonNumVal.NOT_EQ(jet.Float(float64(val))), nil
		case "_>_":
			return jsonNumVal.GT(jet.Float(float64(val))), nil
		case "_>=_":
			return jsonNumVal.GT_EQ(jet.Float(float64(val))), nil
		case "_<_":
			return jsonNumVal.LT(jet.Float(float64(val))), nil
		case "_<=_":
			return jsonNumVal.LT_EQ(jet.Float(float64(val))), nil
		}

	case *exprpb.Constant_BoolValue:
		val := v.BoolValue
		jsonBoolVal := jet.RawBool("((#baseCol# ->> #key#)::boolean)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case "_==_":
			return jsonBoolVal.EQ(jet.Bool(val)), nil
		case "_!=_":
			return jsonBoolVal.NOT_EQ(jet.Bool(val)), nil
		}

	default:
		return nil, fmt.Errorf("unsupported JSON value type for key %s", jsonKey)
	}
	return nil, fmt.Errorf("operator %s not supported for JSON key %s", op, jsonKey)
}
