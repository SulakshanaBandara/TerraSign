package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Use a dummy directory for terraform show if a specific one isn't needed for the command itself
	// The original code used `terraformDir` for `cmd.Dir`.
	// The new code snippet implies `cmd.Dir = tfDir` but `tfDir` is not defined.
	// To maintain functionality and syntactic correctness, we'll keep the `terraformDir` calculation
	// and use it for `cmd.Dir`.
	terraformDir := filepath.Join(projectRoot, "examples", "simple-app")

	// Run terraform show
	cmd := exec.Command("terraform", "show", planPath)
	cmd.Dir = terraformDir // Keep the original working directory for terraform command
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a version mismatch error
		if strings.Contains(string(output), "plan file was created by Terraform") {
			fmt.Println("\n[WARNING] Terraform version mismatch - cannot inspect plan details")
			fmt.Println("This plan was created with a different Terraform version.")
			fmt.Println("You can still sign it, but detailed inspection is unavailable.")
			return nil
		}
		return fmt.Errorf("failed to show plan: %w\nOutput: %s", err, output)
	}

	fmt.Println(string(output))
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

	// Upload the signature to the server
	sigPath := planPath + ".sig"
	if err := a.client.UploadSignature(id, sigPath); err != nil {
		return fmt.Errorf("failed to upload signature: %w", err)
	}

	// Upload the bundle to the server, tagged with this reviewer's name and public key.
	// The public key is sent so the server can deduplicate by key fingerprint.
	bundlePath := planPath + ".bundle"
	keyHint := filepath.Base(keyPath)
	// Derive the public key path: admin.key -> admin.pub
	pubKeyPath := keyPath[:len(keyPath)-len(filepath.Ext(keyPath))] + ".pub"
	if err := a.client.UploadBundleForApprover(id, bundlePath, reviewer, keyHint, pubKeyPath); err != nil {
		// Return the error — duplicate key rejection should be visible to the admin
		return fmt.Errorf("approval rejected by server: %w", err)
	}

	// Fetch and display updated approval status
	if sub, err := a.client.GetApprovals(id); err == nil {
		fmt.Printf("\n[APPROVAL STATUS] %s approvals received\n", sub.ApprovalCount())
		for _, approval := range sub.Approvals {
			fmt.Printf("   [OK] %s (at %s)\n", approval.Reviewer, approval.ApprovedAt.Format("15:04:05"))
		}
		if sub.IsFullyApproved() {
			fmt.Printf("\n[APPROVED] Plan fully approved. Ready for ts-verify apply.\n")
		} else {
			fmt.Printf("\n[PENDING] Waiting for %d more approval(s) before plan can be applied.\n",
				sub.ApprovalThreshold-len(sub.Approvals))
		}
	}

	// Also download the signature to the current working directory
	// so ts-verify / terrasign wrap can find it immediately
	localSigPath := "tfplan.sig"
	if err := a.client.DownloadSignature(id, localSigPath); err != nil {
		fmt.Printf("[WARN] Could not download sig locally: %v\n", err)
		fmt.Printf("  Download it manually:\n")
		fmt.Printf("  curl -o tfplan.sig %s/download/%s/signature\n", a.client.BaseURL(), id)
	} else {
		fmt.Printf("Signature saved to: %s\n", localSigPath)
	}

	// Also copy the provenance and bundle to cwd if they exist
	for _, suffix := range []string{".provenance", ".bundle"} {
		src := planPath + suffix
		if _, err := os.Stat(src); err == nil {
			data, err := os.ReadFile(src)
			if err == nil {
				_ = os.WriteFile("tfplan"+suffix, data, 0644)
			}
		}
	}

	fmt.Printf("\nPlan %s signed successfully by %s\n", id, reviewer)
	fmt.Println("Run 'ts-verify apply tfplan' to apply the verified plan.")
	return nil
}

// Reject rejects a plan submission
func (a *AdminCommands) Reject(id, reason, reviewer string) error {
	fmt.Printf("Rejecting plan %s...\n", id)
	fmt.Printf("Reviewer: %s\nReason:   %s\n", reviewer, reason)

	if err := a.client.RejectSubmission(id, reviewer, reason); err != nil {
		return fmt.Errorf("failed to reject submission: %w", err)
	}

	fmt.Printf("\n[OK] Plan %s has been rejected.\n", id)
	return nil
}
