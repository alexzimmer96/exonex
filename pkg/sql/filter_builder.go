// Package sql provides helpers for translating user supplied query strings into
// SQL expressions understood by the go-jet query builder.
//
// FilterBuilder converts a CEL (Common Expression Language) filter string into a
// jet.BoolExpression that can be plugged into a WHERE clause. The overall flow is:
//
//	filter string --(CEL parser)--> AST --(walkAST/parseCall)--> jet.BoolExpression
//
// A FieldMap tells the builder which CEL identifiers are allowed and which SQL
// column each identifier maps to. Identifiers that are not present in the map are
// rejected, which keeps the generated SQL restricted to an explicit allow-list of
// columns.
package sql

import (
	"fmt"
	"time"

	jet "github.com/go-jet/jet/v2/postgres"
	"github.com/google/cel-go/cel"
	"github.com/google/uuid"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// CEL represents binary/relational operators as synthetic function names in the
// parsed AST (e.g. "a == b" becomes a call to "_==_"). These constants name those
// functions so the switch statements below read intentionally instead of relying on
// scattered magic strings.
const (
	celFuncAnd = "_&&_"
	celFuncOr  = "_||_"
	celFuncIn  = "@in"

	celFuncEq  = "_==_"
	celFuncNeq = "_!=_"
	celFuncGt  = "_>_"
	celFuncGte = "_>=_"
	celFuncLt  = "_<_"
	celFuncLte = "_<=_"
)

// ColumnUUID marks a jet string column that actually stores a UUID value. It lets
// the builder parse/validate UUID literals instead of treating them as plain text.
type ColumnUUID interface {
	jet.ColumnString
	IsUUID() bool
}

// AsUUID wraps a jet string column so the filter builder treats its values as UUIDs.
func AsUUID(c jet.ColumnString) ColumnUUID {
	return columnUUID{c}
}

type columnUUID struct {
	jet.ColumnString
}

func (c columnUUID) IsUUID() bool {
	return true
}

// =====================================================================================================================

// FilterBuilder builds SQL boolean expressions from CEL filter strings.
type FilterBuilder struct {
	env *cel.Env
}

// NewFilterBuilder creates a FilterBuilder backed by a default CEL environment.
func NewFilterBuilder() (*FilterBuilder, error) {
	env, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	return &FilterBuilder{env: env}, nil
}

// BuildExpression parses filterStr as a CEL expression and translates it into a
// jet.BoolExpression using mapping to resolve field names to columns.
//
// An empty filter string is not an error: it returns (nil, nil), letting callers
// omit the WHERE clause entirely.
func (b *FilterBuilder) BuildExpression(filterStr string, mapping FieldMap) (jet.BoolExpression, error) {
	if filterStr == "" {
		return nil, nil
	}

	ast, iss := b.env.Parse(filterStr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("invalid CEL expression: %w", iss.Err())
	}

	parsedExpr, err := cel.AstToParsedExpr(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to convert AST: %w", err)
	}

	return walkAST(parsedExpr.Expr, mapping)
}

// walkAST recursively translates a CEL expression node into a SQL boolean
// expression. Only call expressions (operators and functions) can form a boolean
// predicate, so any other node kind is rejected.
func walkAST(e *exprpb.Expr, mapping FieldMap) (jet.BoolExpression, error) {
	switch kind := e.ExprKind.(type) {
	case *exprpb.Expr_CallExpr:
		return parseCall(kind.CallExpr, mapping)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", kind)
	}
}

// parseCall dispatches a CEL call node to the handler for its operator.
func parseCall(call *exprpb.Expr_Call, mapping FieldMap) (jet.BoolExpression, error) {
	switch call.Function {
	case celFuncAnd, celFuncOr:
		return parseLogical(call, mapping)

	case celFuncIn:
		return parseInOperator(call.Args, mapping)

	case celFuncEq, celFuncNeq, celFuncGt, celFuncGte, celFuncLt, celFuncLte:
		return createComparison(call.Function, call.Args, mapping)

	default:
		return nil, fmt.Errorf("unsupported operator: %s", call.Function)
	}
}

// parseLogical handles the binary "&&" and "||" operators by recursively building
// the left and right sub-expressions and combining them.
func parseLogical(call *exprpb.Expr_Call, mapping FieldMap) (jet.BoolExpression, error) {
	if len(call.Args) < 2 {
		return nil, fmt.Errorf("invalid arguments for logical operator %s", call.Function)
	}

	left, err := walkAST(call.Args[0], mapping)
	if err != nil {
		return nil, err
	}
	right, err := walkAST(call.Args[1], mapping)
	if err != nil {
		return nil, err
	}

	if call.Function == celFuncAnd {
		return left.AND(right), nil
	}
	return left.OR(right), nil
}

// parseInOperator handles the "in" operator, whose meaning depends on the shape of
// its right-hand side:
//
//	field in [a, b, c]   -> SQL "field IN (...)"          (createSQLInList)
//	value in field.key   -> JSONB containment "@>"         (createJSONContains)
//	value in field       -> Postgres array "= ANY(field)"  (createNativeArrayContains)
func parseInOperator(args []*exprpb.Expr, mapping FieldMap) (jet.BoolExpression, error) {
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

// createSQLInList builds "column IN (values...)" from a CEL list literal. The
// element type is derived from the column type so literals are converted correctly
// (UUID validation, string, integer).
func createSQLInList(left *exprpb.Expr, listExpr *exprpb.Expr_CreateList, mapping FieldMap) (jet.BoolExpression, error) {
	ident, ok := extractPath(left)
	if !ok {
		return nil, fmt.Errorf("invalid left-hand expression for IN operator")
	}

	col, ok := mapping[ident]
	if !ok {
		return nil, fmt.Errorf("unknown field: %s", ident)
	}

	switch c := col.(type) {
	case ColumnUUID:
		expressions := make([]jet.Expression, 0, len(listExpr.Elements))
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
		expressions := make([]jet.Expression, 0, len(listExpr.Elements))
		for _, elem := range listExpr.Elements {
			expressions = append(expressions, jet.String(elem.GetConstExpr().GetStringValue()))
		}
		return c.IN(expressions...), nil

	case jet.ColumnInteger:
		expressions := make([]jet.Expression, 0, len(listExpr.Elements))
		for _, elem := range listExpr.Elements {
			expressions = append(expressions, jet.Int(elem.GetConstExpr().GetInt64Value()))
		}
		return c.IN(expressions...), nil

	default:
		return nil, fmt.Errorf("IN list not supported for column type %T", col)
	}
}

// createNativeArrayContains builds "value = ANY(column)" for a Postgres array
// column. The value must be a scalar constant.
func createNativeArrayContains(left *exprpb.Expr, arrayCol jet.Column) (jet.BoolExpression, error) {
	constVal := left.GetConstExpr()
	if constVal == nil {
		return nil, fmt.Errorf("left side of array search must be a constant")
	}

	valExpr, err := constToExpr(constVal)
	if err != nil {
		return nil, err
	}

	return jet.RawBool("#val# = ANY(#col#)", jet.RawArgs{
		"#val#": valExpr,
		"#col#": arrayCol,
	}), nil
}

// createJSONContains builds a JSONB containment check "column @> '{...}'" used to
// test whether a JSON array stored under column.key contains the given value.
func createJSONContains(selectExpr *exprpb.Expr_Select, valueExpr *exprpb.Expr, mapping FieldMap) (jet.BoolExpression, error) {
	baseField, ok := extractPath(selectExpr.Operand)
	if !ok {
		return nil, fmt.Errorf("invalid base field for json array contains")
	}
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

// createComparison handles the relational operators (==, !=, <, <=, >, >=).
//
// The left-hand side is resolved in two ways:
//  1. As a direct column when its full dotted path is a key in the mapping.
//  2. As a JSON field access "base.key" when only the base is a known column, in
//     which case the comparison is applied to the extracted JSON value.
func createComparison(op string, args []*exprpb.Expr, mapping FieldMap) (jet.BoolExpression, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("invalid operator arguments")
	}

	leftExpr := args[0]
	rightExpr := args[1]

	fullPath, ok := extractPath(leftExpr)
	if !ok {
		return nil, fmt.Errorf("invalid left-hand expression in comparison")
	}

	// Case 1: the whole path is a known column -> direct column comparison.
	if col, exists := mapping[fullPath]; exists {
		valExpr := rightExpr.GetConstExpr()
		if valExpr == nil {
			return nil, fmt.Errorf("right-hand side of comparison for %s must be a constant", fullPath)
		}
		return mapColumnComparison(fullPath, valExpr, col, op)
	}

	// Case 2: "base.key" where base is a known column -> JSON field comparison.
	if selectExpr := leftExpr.GetSelectExpr(); selectExpr != nil {
		if baseField, baseOk := extractPath(selectExpr.Operand); baseOk {
			if baseCol, exists := mapping[baseField]; exists {
				return createJSONComparison(op, baseCol, selectExpr.Field, rightExpr)
			}
		}
	}

	return nil, fmt.Errorf("unknown field: %s", fullPath)
}

// mapColumnComparison dispatches a comparison to the type-specific handler based on
// the concrete column type. ColumnUUID must be checked before jet.ColumnString
// because it embeds it.
func mapColumnComparison(ident string, valExpr *exprpb.Constant, col jet.Column, op string) (jet.BoolExpression, error) {
	switch c := col.(type) {
	case ColumnUUID:
		return mapUUIDOperations(ident, valExpr, c, op)
	case jet.ColumnString:
		return mapStringOperations(ident, valExpr, c, op)
	case jet.ColumnInteger:
		return mapIntOperations(ident, valExpr, c, op)
	case jet.ColumnBool:
		return mapBoolOperations(ident, valExpr, c, op)
	case jet.ColumnTimestampz:
		return mapTimestampOperations(ident, valExpr, c, op)
	default:
		return nil, fmt.Errorf("unsupported column type %T for field %s", col, ident)
	}
}

func mapUUIDOperations(ident string, valExpr *exprpb.Constant, c ColumnUUID, op string) (jet.BoolExpression, error) {
	val := valExpr.GetStringValue()
	parsedUUID, err := uuid.Parse(val)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID format for field %s: %w", ident, err)
	}
	uuidLiteral := jet.UUID(parsedUUID)
	switch op {
	case celFuncEq:
		return c.EQ(uuidLiteral), nil
	case celFuncNeq:
		return c.NOT_EQ(uuidLiteral), nil
	}
	return nil, fmt.Errorf("unsupported operator '%s' for UUID field '%s'", op, ident)
}

func mapStringOperations(ident string, valExpr *exprpb.Constant, c jet.ColumnString, op string) (jet.BoolExpression, error) {
	val := valExpr.GetStringValue()
	switch op {
	case celFuncEq:
		return c.EQ(jet.String(val)), nil
	case celFuncNeq:
		return c.NOT_EQ(jet.String(val)), nil
	case celFuncGt:
		return c.GT(jet.String(val)), nil
	case celFuncLt:
		return c.LT(jet.String(val)), nil
	}
	return nil, fmt.Errorf("unsupported operator '%s' for string field '%s'", op, ident)
}

func mapIntOperations(ident string, valExpr *exprpb.Constant, c jet.ColumnInteger, op string) (jet.BoolExpression, error) {
	val := valExpr.GetInt64Value()
	switch op {
	case celFuncEq:
		return c.EQ(jet.Int(val)), nil
	case celFuncNeq:
		return c.NOT_EQ(jet.Int(val)), nil
	case celFuncGt:
		return c.GT(jet.Int(val)), nil
	case celFuncGte:
		return c.GT_EQ(jet.Int(val)), nil
	case celFuncLt:
		return c.LT(jet.Int(val)), nil
	case celFuncLte:
		return c.LT_EQ(jet.Int(val)), nil
	}
	return nil, fmt.Errorf("unsupported operator '%s' for integer field '%s'", op, ident)
}

func mapBoolOperations(ident string, valExpr *exprpb.Constant, c jet.ColumnBool, op string) (jet.BoolExpression, error) {
	val := valExpr.GetBoolValue()
	switch op {
	case celFuncEq:
		return c.EQ(jet.Bool(val)), nil
	case celFuncNeq:
		return c.NOT_EQ(jet.Bool(val)), nil
	}
	return nil, fmt.Errorf("unsupported operator '%s' for bool field '%s'", op, ident)
}

func mapTimestampOperations(ident string, valExpr *exprpb.Constant, c jet.ColumnTimestampz, op string) (jet.BoolExpression, error) {
	t, err := extractTime(valExpr, ident)
	if err != nil {
		return nil, err
	}
	tsVal := jet.TimestampzT(t)
	switch op {
	case celFuncEq:
		return c.EQ(tsVal), nil
	case celFuncNeq:
		return c.NOT_EQ(tsVal), nil
	case celFuncGt:
		return c.GT(tsVal), nil
	case celFuncGte:
		return c.GT_EQ(tsVal), nil
	case celFuncLt:
		return c.LT(tsVal), nil
	case celFuncLte:
		return c.LT_EQ(tsVal), nil
	}
	return nil, fmt.Errorf("unsupported operator '%s' for timestamp field '%s'", op, ident)
}

// createJSONComparison compares a value extracted from a JSON object (base ->> key)
// against a constant. The extracted text is cast to the appropriate SQL type based
// on the constant kind (text, numeric, boolean).
func createJSONComparison(op string, baseCol jet.Column, jsonKey string, rightExpr *exprpb.Expr) (jet.BoolExpression, error) {
	constVal := rightExpr.GetConstExpr()
	switch v := constVal.ConstantKind.(type) {
	case *exprpb.Constant_StringValue:
		val := v.StringValue
		jsonTextVal := jet.RawString("(#baseCol# ->> #key#)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case celFuncEq:
			return jsonTextVal.EQ(jet.String(val)), nil
		case celFuncNeq:
			return jsonTextVal.NOT_EQ(jet.String(val)), nil
		}

	case *exprpb.Constant_Int64Value:
		val := v.Int64Value
		jsonNumVal := jet.RawFloat("((#baseCol# ->> #key#)::numeric)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case celFuncEq:
			return jsonNumVal.EQ(jet.Float(float64(val))), nil
		case celFuncNeq:
			return jsonNumVal.NOT_EQ(jet.Float(float64(val))), nil
		case celFuncGt:
			return jsonNumVal.GT(jet.Float(float64(val))), nil
		case celFuncGte:
			return jsonNumVal.GT_EQ(jet.Float(float64(val))), nil
		case celFuncLt:
			return jsonNumVal.LT(jet.Float(float64(val))), nil
		case celFuncLte:
			return jsonNumVal.LT_EQ(jet.Float(float64(val))), nil
		}

	case *exprpb.Constant_BoolValue:
		val := v.BoolValue
		jsonBoolVal := jet.RawBool("((#baseCol# ->> #key#)::boolean)",
			jet.RawArgs{"#baseCol#": baseCol, "#key#": jsonKey})

		switch op {
		case celFuncEq:
			return jsonBoolVal.EQ(jet.Bool(val)), nil
		case celFuncNeq:
			return jsonBoolVal.NOT_EQ(jet.Bool(val)), nil
		}

	default:
		return nil, fmt.Errorf("unsupported JSON value type for key %s", jsonKey)
	}
	return nil, fmt.Errorf("operator %s not supported for JSON key %s", op, jsonKey)
}

// constToExpr converts a scalar CEL constant into a jet literal expression.
func constToExpr(constVal *exprpb.Constant) (jet.Expression, error) {
	switch v := constVal.ConstantKind.(type) {
	case *exprpb.Constant_StringValue:
		return jet.String(v.StringValue), nil
	case *exprpb.Constant_Int64Value:
		return jet.Int(v.Int64Value), nil
	case *exprpb.Constant_BoolValue:
		return jet.Bool(v.BoolValue), nil
	default:
		return nil, fmt.Errorf("unsupported array element type: %T", v)
	}
}

// extractTime parses a timestamp constant. It accepts an RFC3339 string or a Unix
// epoch integer (seconds) and always returns the time in UTC.
func extractTime(valExpr *exprpb.Constant, ident string) (time.Time, error) {
	if strVal, ok := valExpr.ConstantKind.(*exprpb.Constant_StringValue); ok {
		t, err := time.Parse(time.RFC3339, strVal.StringValue)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid RFC3339 timestamp format for %s: %w", ident, err)
		}
		return t.UTC(), nil
	}
	if intVal, ok := valExpr.ConstantKind.(*exprpb.Constant_Int64Value); ok {
		return time.Unix(intVal.Int64Value, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 string for timestamp comparison on field %s", ident)
}

// extractPath flattens an identifier or select chain into a dotted field path,
// e.g. the expression "a.b.c" becomes the string "a.b.c". It returns false for any
// other expression kind.
func extractPath(e *exprpb.Expr) (string, bool) {
	if ident := e.GetIdentExpr(); ident != nil {
		return ident.Name, true
	}
	if sel := e.GetSelectExpr(); sel != nil {
		base, ok := extractPath(sel.Operand)
		if !ok {
			return "", false
		}
		return base + "." + sel.Field, true
	}
	return "", false
}
