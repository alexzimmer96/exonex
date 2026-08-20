package sql

import (
	"errors"

	jet "github.com/go-jet/jet/v2/postgres"
)

var (
	ErrFieldNotFound = errors.New("field not found")
)

type FieldMap map[string]jet.Column

func (fm FieldMap) BuildProjection(fields []string, fallback jet.Projection) []jet.Projection {
	if fields == nil {
		return []jet.Projection{fallback}
	}

	projections := make([]jet.Projection, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		if col, ok := fm[field]; ok {
			if !seen[field] {
				projections = append(projections, col)
				seen[field] = true
			}
		}
	}
	return projections
}
