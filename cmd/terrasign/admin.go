package main

import (
	"fmt"
	"os"
	"os/exec"
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

// Inspect shows what changes are in a plan
func (a *AdminCommands) Inspect(id string) error {
	fmt.Printf("Inspecting plan %s...\n\n", id)

	// Download the plan to a temp location
	tempDir := filepath.Join(os.TempDir(), "terrasign-inspect", id)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after inspection

	planPath := filepath.Join(tempDir, "tfplan")
	if err := a.client.DownloadPlan(id, planPath); err != nil {
		return fmt.Errorf("failed to download plan: %w", err)
	}

	// Run terraform show to display the changes
	// We need to run this from an initialized terraform directory
	// Get absolute path to examples/simple-app
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	
	// Try to find the project root by looking for go.mod
	projectRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			// Reached filesystem root, use cwd
			projectRoot = cwd
			break
		}
		projectRoot = parent
	}
	
	terraformDir := filepath.Join(projectRoot, "examples", "simple-app")
	
	cmd := exec.Command("terraform", "show", planPath)
	cmd.Dir = terraformDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to show plan: %w", err)
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

	// Sign the plan (skip policy check since it was done during submission)
	if err := signer.SignWithOptions(planPath, keyPath, true); err != nil {
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
