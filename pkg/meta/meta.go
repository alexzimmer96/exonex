package meta

import "time"

type Meta struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	LogEvents   []LogEvent        `json:"log_events,omitempty"`
	LeaseHolder *Lease            `json:"lease,omitempty"`
}

type LogEvent struct {
	Time          time.Time `json:"time"`
	Author        string    `json:"author,omitempty"`
	Message       string    `json:"message"`
	RetentionTime time.Time `json:"retention_time,omitempty"`
}

type Lease struct {
	Holder    string    `json:"holder"`
	StartedAt time.Time `json:"created_at"`
	RenewedAt time.Time `json:"renewed_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
