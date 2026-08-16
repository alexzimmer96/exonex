package document

import "github.com/google/uuid"

type Document struct {
	ID            uuid.UUID `db:"id" json:"id"`
	PublisherID   uuid.UUID `db:"publisher_id" json:"publisher_id"`
	MimeType      string    `db:"mime_type" json:"mime_type"`
	SizeBytes     int64     `db:"size_bytes" json:"size_bytes"`
	StorageVolume string    `db:"storage_volume" json:"storage_volume"`
	StorageKey    string    `db:"storage_key" json:"storage_key"`
	SourceURL     *string   `db:"source_url" json:"source_url"`
}
