package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
	"github.com/sulakshanakarunarathne/terrasign/pkg/signer"
)

// AdminCommands handles admin-specific operations
type AdminCommands struct {
	client *remote.Client
}

// NewAdminCommands creates admin command handler
func NewAdminCommands(serviceURL string) *AdminCommands {
	return &AdminCommands{
		client: remote.NewClient(serviceURL),
	}
}

// ListPending lists all pending plan submissions
func (a *AdminCommands) ListPending() error {
	submissions, err := a.client.ListPending()
	if err != nil {
		return fmt.Errorf("failed to list pending submissions: %w", err)
	}

	if len(submissions) == 0 {
		fmt.Println("No pending submissions")
		return nil
	}

	fmt.Printf("Found %d pending submission(s):\n\n", len(submissions))
	for _, sub := range submissions {
		fmt.Printf("ID: %s\n", sub.ID)
		fmt.Printf("  Submitter: %s\n", sub.Submitter)
		fmt.Printf("  Created:   %s\n", sub.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  Status:    %s\n", sub.Status)
		fmt.Println()
	}

	return nil
}

// Download downloads a plan for review
func (a *AdminCommands) Download(id, outputDir string) error {
	fmt.Printf("Downloading plan %s...\n", id)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	planPath := filepath.Join(outputDir, "tfplan")
	if err := a.client.DownloadPlan(id, planPath); err != nil {
		return fmt.Errorf("failed to download plan: %w", err)
	}

	fmt.Printf("Plan downloaded to: %s\n", planPath)
	fmt.Println("\nReview the plan with:")
	fmt.Printf("  terraform show %s\n", planPath)
	fmt.Println("\nIf approved, sign with:")
	fmt.Printf("  terrasign admin sign %s --key <admin-key>\n", id)

	return nil
}

// Sign signs an approved plan
func (a *AdminCommands) Sign(id, keyPath, reviewer string) error {
	fmt.Printf("Signing plan %s...\n", id)

	// Download the plan first
	tempDir := filepath.Join(os.TempDir(), "terrasign-admin", id)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	planPath := filepath.Join(tempDir, "tfplan")
	if err := a.client.DownloadPlan(id, planPath); err != nil {
		return fmt.Errorf("failed to download plan: %w", err)
	}

	// Sign the plan
	if err := signer.Sign(planPath, keyPath); err != nil {
		return fmt.Errorf("failed to sign plan: %w", err)
	}

	// Upload the signature
	sigPath := planPath + ".sig"
	if err := a.client.UploadSignature(id, sigPath); err != nil {
		return fmt.Errorf("failed to upload signature: %w", err)
	}

	fmt.Printf("Plan %s signed successfully by %s\n", id, reviewer)
	return nil
}

// Reject rejects a plan submission
func (a *AdminCommands) Reject(id, reason string) error {
	fmt.Printf("Rejecting plan %s...\n", id)
	fmt.Printf("Reason: %s\n", reason)
	
	// In a full implementation, this would call an API endpoint
	// For now, we just print the message
	fmt.Println("Note: Rejection functionality requires server-side implementation")
	
	return nil
}
