package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Storage handles plan storage and retrieval
type Storage struct {
	baseDir string
}

// NewStorage creates a new storage instance
func NewStorage(baseDir string) (*Storage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &Storage{baseDir: baseDir}, nil
}

// StorePlan saves a plan file and creates a submission record
func (s *Storage) StorePlan(planData io.Reader, submitter string, threshold int) (*PlanSubmission, error) {
	id := uuid.New().String()

	if threshold < 1 {
		threshold = 1 // Minimum 1 approval required
	}

	// Create directory for this submission
	submissionDir := filepath.Join(s.baseDir, id)
	if err := os.MkdirAll(submissionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create submission directory: %w", err)
	}

	// Save plan file
	planPath := filepath.Join(submissionDir, "tfplan")
	planFile, err := os.Create(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan file: %w", err)
	}
	defer planFile.Close()

	if _, err := io.Copy(planFile, planData); err != nil {
		return nil, fmt.Errorf("failed to write plan data: %w", err)
	}

	// Create submission metadata
	submission := &PlanSubmission{
		ID:                id,
		Submitter:         submitter,
		CreatedAt:         time.Now(),
		Status:            "pending",
		ApprovalThreshold: threshold,
		Approvals:         []Approval{},
	}

	// Save metadata
	if err := s.saveMetadata(submission); err != nil {
		return nil, err
	}

	return submission, nil
}

// GetSubmission retrieves a submission by ID
func (s *Storage) GetSubmission(id string) (*PlanSubmission, error) {
	metadataPath := filepath.Join(s.baseDir, id, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	var submission PlanSubmission
	if err := json.Unmarshal(data, &submission); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &submission, nil
}

// ListPending returns all pending submissions
func (s *Storage) ListPending() ([]*PlanSubmission, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var pending []*PlanSubmission
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		submission, err := s.GetSubmission(entry.Name())
		if err != nil {
			continue
		}

		if submission.Status == "pending" {
			pending = append(pending, submission)
		}
	}

	return pending, nil
}

// GetPlanPath returns the path to the plan file
func (s *Storage) GetPlanPath(id string) string {
	return filepath.Join(s.baseDir, id, "tfplan")
}

// GetSignaturePath returns the path to the signature file
func (s *Storage) GetSignaturePath(id string) string {
	return filepath.Join(s.baseDir, id, "tfplan.sig")
}

// GetBundlePath returns the path to the primary cosign bundle file
func (s *Storage) GetBundlePath(id string) string {
	return filepath.Join(s.baseDir, id, "tfplan.bundle")
}

// GetBundlePathForApprover returns the bundle path for a specific approver
func (s *Storage) GetBundlePathForApprover(id, approver string) string {
	// Sanitize approver name for use as filename
	safe := ""
	for _, c := range approver {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			safe += string(c)
		} else {
			safe += "_"
		}
	}
	if safe == "" {
		safe = "unknown"
	}
	return filepath.Join(s.baseDir, id, "tfplan-"+safe+".bundle")
}

// AddApproval registers one admin's approval and auto-promotes the plan when threshold is reached
func (s *Storage) AddApproval(id string, approval Approval) (*PlanSubmission, error) {
	submission, err := s.GetSubmission(id)
	if err != nil {
		return nil, err
	}

	if submission.Status == "rejected" {
		return nil, fmt.Errorf("plan %s has been rejected and cannot be approved", id)
	}

	// Prevent duplicate approval from same reviewer
	if submission.HasApproval(approval.Reviewer) {
		return nil, fmt.Errorf("reviewer %q has already approved this plan", approval.Reviewer)
	}

	submission.Approvals = append(submission.Approvals, approval)

	// Auto-promote if threshold is reached
	if submission.IsFullyApproved() {
		now := time.Now()
		submission.Status = "approved"
		submission.SignedAt = &now
		submission.ReviewedBy = approval.Reviewer // Final approver
		submission.ReviewedAt = &now
		fmt.Printf("[APPROVED] Plan %s reached %d/%d approvals — now approved for apply.\n",
			id, len(submission.Approvals), submission.ApprovalThreshold)
	} else {
		fmt.Printf("[APPROVAL %d/%d] Plan %s approved by %s.\n",
			len(submission.Approvals), submission.ApprovalThreshold, id, approval.Reviewer)
	}

	if err := s.saveMetadata(submission); err != nil {
		return nil, err
	}
	return submission, nil
}

// UpdateSubmission updates submission metadata
func (s *Storage) UpdateSubmission(submission *PlanSubmission) error {
	return s.saveMetadata(submission)
}

// saveMetadata saves submission metadata to disk
func (s *Storage) saveMetadata(submission *PlanSubmission) error {
	metadataPath := filepath.Join(s.baseDir, submission.ID, "metadata.json")
	data, err := json.MarshalIndent(submission, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// GlobalPolicy holds server-wide approval policy
type GlobalPolicy struct {
	ApprovalThreshold int    `json:"approval_threshold"` // 0 means not set (use per-submission default)
	SetBy             string `json:"set_by,omitempty"`
	SetAt             string `json:"set_at,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

// policyPath returns the path to the global policy file
func (s *Storage) policyPath() string {
	return filepath.Join(s.baseDir, "policy.json")
}

// GetGlobalPolicy returns the current global approval policy
func (s *Storage) GetGlobalPolicy() (*GlobalPolicy, error) {
	data, err := os.ReadFile(s.policyPath())
	if os.IsNotExist(err) {
		return &GlobalPolicy{ApprovalThreshold: 1}, nil // default: 1 approval
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read policy: %w", err)
	}
	var policy GlobalPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}
	if policy.ApprovalThreshold < 1 {
		policy.ApprovalThreshold = 1
	}
	return &policy, nil
}

// SetGlobalPolicy persists a new global approval policy
func (s *Storage) SetGlobalPolicy(policy *GlobalPolicy) error {
	if policy.ApprovalThreshold < 1 {
		policy.ApprovalThreshold = 1
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}
	if err := os.WriteFile(s.policyPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write policy: %w", err)
	}
	fmt.Printf("[POLICY] Global approval threshold set to %d by %s\n",
		policy.ApprovalThreshold, policy.SetBy)
	return nil
}
