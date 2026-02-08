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
func (s *Storage) StorePlan(planData io.Reader, submitter string) (*PlanSubmission, error) {
	id := uuid.New().String()
	
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
		ID:          id,
		Submitter:   submitter,
		SubmittedAt: time.Now(),
		Status:      "pending",
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
