package remote

import (
	"fmt"
	"time"
)

// Approval represents a single admin's cryptographic approval of a plan
type Approval struct {
	Reviewer       string    `json:"reviewer"`
	ApprovedAt     time.Time `json:"approved_at"`
	KeyHint        string    `json:"key_hint,omitempty"`        // Key filename for audit display
	KeyFingerprint string    `json:"key_fingerprint,omitempty"` // SHA-256 of the public key bytes
	BundleFile     string    `json:"bundle_file"`               // Filename of this approver's bundle
}

// PlanSubmission represents a plan submitted for review
type PlanSubmission struct {
	ID                string     `json:"id"`
	PlanHash          string     `json:"plan_hash"`
	Submitter         string     `json:"submitter"`
	CreatedAt         time.Time  `json:"created_at"`
	Status            string     `json:"status"`             // pending, approved, rejected
	ApprovalThreshold int        `json:"approval_threshold"` // How many approvals needed
	Approvals         []Approval `json:"approvals"`          // List of received approvals
	ReviewedBy        string     `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	SignedAt          *time.Time `json:"signed_at,omitempty"`
	RejectionReason   string     `json:"rejection_reason,omitempty"`
}

// ApprovalCount returns "N/M" formatted approval progress
func (p *PlanSubmission) ApprovalCount() string {
	return fmt.Sprintf("%d/%d", len(p.Approvals), p.ApprovalThreshold)
}

// IsFullyApproved returns true when enough approvals have been collected
func (p *PlanSubmission) IsFullyApproved() bool {
	return p.ApprovalThreshold > 0 && len(p.Approvals) >= p.ApprovalThreshold
}

// HasApproval returns true if the given reviewer name has already signed
func (p *PlanSubmission) HasApproval(reviewer string) bool {
	for _, a := range p.Approvals {
		if a.Reviewer == reviewer {
			return true
		}
	}
	return false
}

// HasApprovalFromKey returns true if the given key fingerprint has already signed.
// This prevents the same physical key from counting twice even with a different reviewer name.
func (p *PlanSubmission) HasApprovalFromKey(fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	for _, a := range p.Approvals {
		if a.KeyFingerprint == fingerprint {
			return true
		}
	}
	return false
}

// SigningServiceConfig holds configuration for the signing service
type SigningServiceConfig struct {
	StorageDir string
	Port       int
	AdminKey   string // Path to admin public key for verification
	APIToken   string // Optional Bearer token for authenticating API requests
}
