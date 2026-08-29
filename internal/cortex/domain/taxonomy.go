package domain

import (
	"bytes"
	"errors"
	"io"

	"github.com/alexzimmer96/exonex/seed"
	"gopkg.in/yaml.v3"
)

// An AnnotationType defines the allowed data structure that can be used as value to an annotation.
type AnnotationType string

const (
	TypeString      AnnotationType = "STRING"
	TypeNumber      AnnotationType = "NUMBER"
	TypeBoolean     AnnotationType = "BOOLEAN"
	TypeStringArray AnnotationType = "STRING_ARRAY"
	TypeNumberArray AnnotationType = "NUMBER_ARRAY"
)

// =====================================================================================================================

type AnnotationDefinition struct {
	// The ID of the Annotation. This is also the key used in annotation fields.
	// This is typically in URL format.
	ID string `json:"id"`
	// A short description of the Annotation.
	Description string `json:"description"`
	// The value type that is accepted for the annotation.
	Type AnnotationType `json:"type"`
	// List of resources the annotation can be used on.
	// If left empty, there will be no restriction.
	Resources []string `json:"resources"`
	// Validation ruleset to verify the Annotation value against.
	Validation *AnnotationValidation `json:"validation,omitempty"`
}

type AnnotationValidation struct {
	AllowedValues []any   `json:"in"`
	MinLength     int     `json:"minLength"`
	MaxLength     int     `json:"MaxLength"`
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
}

type TaxonomyService struct {
	annotations map[string]AnnotationDefinition
}

func NewTaxonomyService() *TaxonomyService {
	return &TaxonomyService{
		annotations: mustLoadStaticAnnotations(),
	}
}

func mustLoadStaticAnnotations() map[string]AnnotationDefinition {
	decoder := yaml.NewDecoder(bytes.NewBufferString(seed.StaticAnnotations))
	var definitions map[string]AnnotationDefinition

	for {
		var def AnnotationDefinition
		err := decoder.Decode(&def)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		definitions[def.ID] = def
	}

	return definitions
}
