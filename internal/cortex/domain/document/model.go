package document

import (
	"time"

	"github.com/google/uuid"
)

// =====================================================================================================================

type Type string

const (
	DocumentTypeUnspecified = Type("")
	DocumentTypeLegal       = Type("LEGAL")
	DocumentTypeRobotsTXT   = Type("ROBOTS_TXT")
	DocumentTypeArticle     = Type("ARTICLE")
	DocumentTypeSocial      = Type("SOCIAL")
	DocumentTypeJobPosting  = Type("JOB_POSTING")
)

// =====================================================================================================================

type TdmStatus string

const (
	TdmStatusUnspecified    = TdmStatus("")
	TdmStatusAllowed        = TdmStatus("ALLOWED")
	TdmStatusOptOutDetected = TdmStatus("OPT_OUT_DETECTED")
)

// =====================================================================================================================

type ArtifactFormat string

const (
	ArtifactFormatUnspecified   = ArtifactFormat("")
	ArtifactFormatHTML          = ArtifactFormat("HTML")
	ArtifactFormatPDF           = ArtifactFormat("PDF")
	ArtifactFormatImage         = ArtifactFormat("IMAGE")
	ArtifactFormatMarkdown      = ArtifactFormat("MARKDOWN")
	ArtifactFormatCleanMarkdown = ArtifactFormat("CLEAN_MARKDOWN")
)

// =====================================================================================================================

type Document struct {
	ID          uuid.UUID  `json:"id"`
	PublisherID uuid.UUID  `json:"publisher_id"`
	OriginalURL string     `json:"original_url"`
	Type        string     `json:"document_type"`
	Artifacts   []Artifact `json:"artifacts"`
	TdmStatus   TdmStatus  `json:"tdm_status,omitempty"`
	PublishedAt time.Time  `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Artifact struct {
	ID uuid.UUID `json:"id"`
}
