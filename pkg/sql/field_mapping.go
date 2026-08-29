package sql

import (
	"fmt"
	"strings"

	jet "github.com/go-jet/jet/v2/postgres"
)

type UnknownFieldsError struct {
	Fields []string `json:"fields"`
}

func (e UnknownFieldsError) Error() string {
	return fmt.Sprintf("fields are not found: %s", strings.Join(e.Fields, ","))
}

// =====================================================================================================================

type FieldMap map[string]jet.Column

func (fm FieldMap) BuildProjection(fields []string, fallback jet.Projection) ([]jet.Projection, error) {
	if fields == nil {
		return []jet.Projection{fallback}, nil
	}

	var unknownFields []string
	projections := make([]jet.Projection, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		if col, ok := fm[field]; ok {
			if !seen[field] {
				projections = append(projections, col)
				seen[field] = true
			}
		}
		unknownFields = append(unknownFields, field)
	}

	if len(unknownFields) != 0 {
		return nil, UnknownFieldsError{Fields: unknownFields}
	}

	return projections, nil
}
