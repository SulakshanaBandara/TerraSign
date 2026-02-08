package remote

import "time"

// PlanSubmission represents a plan submitted for review
type PlanSubmission struct {
	ID          string    `json:"id"`
	PlanHash    string    `json:"plan_hash"`
	Submitter   string    `json:"submitter"`
	SubmittedAt time.Time `json:"submitted_at"`
	Status      string    `json:"status"` // pending, approved, rejected
	ReviewedBy  string    `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	SignedAt    *time.Time `json:"signed_at,omitempty"`
}

// SigningServiceConfig holds configuration for the signing service
type SigningServiceConfig struct {
	StorageDir string
	Port       int
	AdminKey   string // Path to admin public key for verification
}
