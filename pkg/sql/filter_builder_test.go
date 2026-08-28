package sql

import (
	"fmt"
	"testing"

	jet "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testColumns and testFieldMap provide a small synthetic table covering every
// column type the FilterBuilder understands, so tests can exercise each branch.
var (
	idCol      = jet.StringColumn("id")
	nameCol    = jet.StringColumn("name")
	ageCol     = jet.IntegerColumn("age")
	activeCol  = jet.BoolColumn("active")
	createdCol = jet.TimestampzColumn("created_at")
	labelsCol  = jet.StringColumn("labels")
	metaCol    = jet.StringColumn("meta")

	// testTable ties the columns to a table name so the generated SQL is complete.
	testTable = jet.NewTable("public", "docs", "", idCol, nameCol, ageCol, activeCol, createdCol, labelsCol, metaCol)

	testFieldMap = FieldMap{
		"id":         AsUUID(idCol),
		"name":       nameCol,
		"age":        ageCol,
		"active":     activeCol,
		"created_at": createdCol,
		"labels":     labelsCol,
		"meta":       metaCol,
	}
)

const testUUID = "123e4567-e89b-12d3-a456-426614174000"

// buildSQL parses filter and returns the parameterized SQL for the resulting WHERE
// clause together with a string rendering of the bound arguments. This mirrors how
// the production code renders queries (via Sql()) and keeps assertions simple: the
// query string carries the SQL structure (operators, casts, column names) while the
// args string carries the concrete literal values.
func buildSQL(t *testing.T, filter string) (query, args string, err error) {
	t.Helper()

	fb, err := NewFilterBuilder()
	require.NoError(t, err)

	expr, err := fb.BuildExpression(filter, testFieldMap)
	if err != nil {
		return "", "", err
	}
	if expr == nil {
		return "", "", nil
	}

	sqlStr, sqlArgs := testTable.SELECT(idCol).WHERE(expr).Sql()
	return sqlStr, fmt.Sprintf("%v", sqlArgs), nil
}

func TestBuildExpression_Empty(t *testing.T) {
	fb, err := NewFilterBuilder()
	require.NoError(t, err)

	expr, err := fb.BuildExpression("", testFieldMap)
	require.NoError(t, err)
	assert.Nil(t, expr, "empty filter should produce no expression")
}

func TestBuildExpression_Comparisons(t *testing.T) {
	tests := []struct {
		name          string
		filter        string
		queryContains []string
		argsContains  []string
	}{
		{
			name:          "string equality",
			filter:        `name == "foo"`,
			queryContains: []string{"name", "="},
			argsContains:  []string{"foo"},
		},
		{
			name:          "string inequality",
			filter:        `name != "foo"`,
			queryContains: []string{"name", "!="},
			argsContains:  []string{"foo"},
		},
		{
			name:          "integer greater or equal",
			filter:        `age >= 18`,
			queryContains: []string{"age", ">="},
			argsContains:  []string{"18"},
		},
		{
			name:          "integer less than",
			filter:        `age < 65`,
			queryContains: []string{"age", "<"},
			argsContains:  []string{"65"},
		},
		{
			name:          "bool equality",
			filter:        `active == true`,
			queryContains: []string{"active", "="},
			argsContains:  []string{"true"},
		},
		{
			name:          "uuid equality",
			filter:        `id == "` + testUUID + `"`,
			queryContains: []string{"id", "="},
			argsContains:  []string{testUUID},
		},
		{
			name:          "timestamp comparison",
			filter:        `created_at > "2023-01-01T00:00:00Z"`,
			queryContains: []string{"created_at", ">"},
			argsContains:  []string{"2023-01-01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := buildSQL(t, tt.filter)
			require.NoError(t, err)
			for _, want := range tt.queryContains {
				assert.Contains(t, query, want)
			}
			for _, want := range tt.argsContains {
				assert.Contains(t, args, want)
			}
		})
	}
}

func TestBuildExpression_Logical(t *testing.T) {
	t.Run("AND", func(t *testing.T) {
		query, args, err := buildSQL(t, `name == "foo" && age > 1`)
		require.NoError(t, err)
		assert.Contains(t, query, "AND")
		assert.Contains(t, args, "foo")
		assert.Contains(t, args, "1")
	})

	t.Run("OR", func(t *testing.T) {
		query, args, err := buildSQL(t, `name == "foo" || name == "bar"`)
		require.NoError(t, err)
		assert.Contains(t, query, "OR")
		assert.Contains(t, args, "foo")
		assert.Contains(t, args, "bar")
	})
}

func TestBuildExpression_InOperator(t *testing.T) {
	t.Run("string list", func(t *testing.T) {
		query, args, err := buildSQL(t, `name in ["a", "b"]`)
		require.NoError(t, err)
		assert.Contains(t, query, "IN")
		assert.Contains(t, args, "a")
		assert.Contains(t, args, "b")
	})

	t.Run("integer list", func(t *testing.T) {
		query, args, err := buildSQL(t, `age in [1, 2, 3]`)
		require.NoError(t, err)
		assert.Contains(t, query, "IN")
		assert.Contains(t, args, "1")
		assert.Contains(t, args, "3")
	})

	t.Run("uuid list", func(t *testing.T) {
		_, args, err := buildSQL(t, `id in ["`+testUUID+`"]`)
		require.NoError(t, err)
		assert.Contains(t, args, testUUID)
	})

	t.Run("native array contains", func(t *testing.T) {
		// The value and column are rendered as bound sub-expressions, so only the
		// structural "= ANY(...)" part is observable in the query string.
		query, _, err := buildSQL(t, `"important" in labels`)
		require.NoError(t, err)
		assert.Contains(t, query, "= ANY(")
	})

	t.Run("json array contains", func(t *testing.T) {
		// The JSONB containment operator is what makes this branch distinctive.
		query, _, err := buildSQL(t, `"draft" in meta.tags`)
		require.NoError(t, err)
		assert.Contains(t, query, "@>")
		assert.Contains(t, query, "::jsonb")
	})
}

func TestBuildExpression_JSONComparison(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		query, args, err := buildSQL(t, `meta.status == "active"`)
		require.NoError(t, err)
		assert.Contains(t, query, "->>")
		assert.Contains(t, args, "status")
		assert.Contains(t, args, "active")
	})

	t.Run("numeric value", func(t *testing.T) {
		query, args, err := buildSQL(t, `meta.priority > 5`)
		require.NoError(t, err)
		assert.Contains(t, query, "->>")
		assert.Contains(t, query, "::numeric")
		assert.Contains(t, args, "priority")
		assert.Contains(t, args, "5")
	})

	t.Run("bool value", func(t *testing.T) {
		query, args, err := buildSQL(t, `meta.published == true`)
		require.NoError(t, err)
		assert.Contains(t, query, "->>")
		assert.Contains(t, query, "::boolean")
		assert.Contains(t, args, "published")
	})
}

func TestBuildExpression_Errors(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		errSnippet string
	}{
		{
			name:       "unknown field",
			filter:     `unknown == "x"`,
			errSnippet: "unknown field",
		},
		{
			name:       "invalid uuid",
			filter:     `id == "not-a-uuid"`,
			errSnippet: "invalid UUID",
		},
		{
			name:       "invalid uuid in list",
			filter:     `id in ["not-a-uuid"]`,
			errSnippet: "invalid UUID in list",
		},
		{
			name:       "unsupported operator for bool",
			filter:     `active > true`,
			errSnippet: "unsupported operator",
		},
		{
			name:       "unsupported operator for uuid",
			filter:     `id > "` + testUUID + `"`,
			errSnippet: "unsupported operator",
		},
		{
			name:       "invalid cel syntax",
			filter:     `name ==`,
			errSnippet: "invalid CEL expression",
		},
		{
			name:       "unsupported operator/function",
			filter:     `name.startsWith("a")`,
			errSnippet: "unsupported operator",
		},
		{
			name:       "non-field left-hand call",
			filter:     `size(name) == 1`,
			errSnippet: "invalid left-hand expression",
		},
		{
			name:       "invalid RFC3339 timestamp",
			filter:     `created_at > "not-a-date"`,
			errSnippet: "invalid RFC3339 timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := buildSQL(t, tt.filter)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSnippet)
		})
	}
}

func TestExtractPath(t *testing.T) {
	// A nested JSON path resolves against the base column ("meta"), so the JSON key
	// "a" is bound as an argument of the extraction expression.
	_, args, err := buildSQL(t, `meta.a == "x"`)
	require.NoError(t, err)
	assert.Contains(t, args, "a")
}
