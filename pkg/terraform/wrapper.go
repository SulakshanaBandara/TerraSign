package terraform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
	"github.com/sulakshanakarunarathne/terrasign/pkg/verifier"
)

// Execute wraps the terraform command, intercepting "apply" to enforce verification.
func Execute(args []string, keyDir, identity, issuer, serviceURL string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided to terraform wrapper")
	}

	command := args[0]

	if command == "apply" {
		var planFile string
		for i := len(args) - 1; i >= 1; i-- {
			if !strings.HasPrefix(args[i], "-") {
				planFile = args[i]
				break
			}
		}

		if planFile != "" {
			fmt.Printf("Intercepted 'apply' command. Verifying plan: %s\n", planFile)

			if keyDir == "" && identity == "" {
				return fmt.Errorf("either --key-dir or --identity must be provided for verification")
			}

			// 1. Hash the local plan file
			f, err := os.Open(planFile)
			if err != nil {
				return fmt.Errorf("failed to open plan file: %w", err)
			}
			h := sha256.New()
			io.Copy(h, f)
			f.Close()
			planHash := hex.EncodeToString(h.Sum(nil))

			// 2. Fetch submission status by hash
			client := remote.NewClient(serviceURL, os.Getenv("TERRASIGN_TOKEN"))
			fmt.Printf("Looking up plan hash on server: %s...\n", planHash[:8])

			sub, err := client.GetStatusByHash(planHash)
			if err != nil {
				return fmt.Errorf("server lookup failed: %w", err)
			}

			if sub.Status != "approved" {
				return fmt.Errorf("PLAN NOT FULLY APPROVED. Status is %q. %d/%d approvals", sub.Status, len(sub.Approvals), sub.ApprovalThreshold)
			}

			fmt.Printf("Plan is marked approved (%d/%d approvals). Downloading multi-party bundles...\n", len(sub.Approvals), sub.ApprovalThreshold)

			// 3. Download bundles for all approvers
			for _, approval := range sub.Approvals {
				bundlePath := fmt.Sprintf("%s-%s.bundle", planFile, approval.Reviewer)
				sigPath := fmt.Sprintf("%s-%s.sig", planFile, approval.Reviewer) // We must extract the sig to avoid ASN.1 errors

				fmt.Printf("  Downloading bundle for approver: %s...\n", approval.Reviewer)
				if err := client.DownloadBundleForApprover(sub.ID, approval.Reviewer, bundlePath); err != nil {
					return fmt.Errorf("failed to download bundle for %s: %w", approval.Reviewer, err)
				}

				// Optional: Extract signature if using the standard verifier
				// By convention, verifier.Verify() expects the exact .bundle and .sig extensions
				// For the prototype we rename the file temporarily for the verifier, then delete
				defer os.Remove(bundlePath)
				defer os.Remove(sigPath)
			}

			// We need to run verifier.Verify on the plan for each signed bundle.
			// Currently verifier.Verify assumes fixed filenames `planFile.bundle` and `planFile.sig`.
			// We temporarily rename each approver's bundle to the expected name, verify it, then restore.
			validApprovals := 0
			for _, approval := range sub.Approvals {
				bundlePath := fmt.Sprintf("%s-%s.bundle", planFile, approval.Reviewer)

				// Move it to the target location expected by verifier
				os.Rename(bundlePath, planFile+".bundle")

				// Extract the raw signature directly out of the bundle JSON to avoid ASN.1 encoding issues
				bundleData, err := os.ReadFile(planFile + ".bundle")
				if err == nil {
					var bundleObj struct {
						MessageSignature struct {
							Signature string `json:"signature"`
						} `json:"messageSignature"`
					}
					if err := json.Unmarshal(bundleData, &bundleObj); err == nil && bundleObj.MessageSignature.Signature != "" {
						os.WriteFile(planFile+".sig", []byte(bundleObj.MessageSignature.Signature), 0644)
					}
				}

				fmt.Printf("\n--- Verifying signature from %s ---\n", approval.Reviewer)

				// If we have a key directory, try all public keys in it until one works
				var verified bool
				if keyDir != "" {
					entries, _ := os.ReadDir(keyDir)
					for _, entry := range entries {
						if strings.HasSuffix(entry.Name(), ".pub") {
							keyPath := filepath.Join(keyDir, entry.Name())
							if err := verifier.Verify(planFile, keyPath, identity, issuer); err == nil {
								verified = true
								validApprovals++
								break
							}
						}
					}
				} else {
					// Keyless
					if err := verifier.Verify(planFile, "", identity, issuer); err == nil {
						verified = true
						validApprovals++
					}
				}

				os.Remove(planFile + ".bundle")
				os.Remove(planFile + ".sig")

				if !verified {
					return fmt.Errorf("cryptographic verification failed for approver %s bundle", approval.Reviewer)
				}
			}

			if validApprovals < sub.ApprovalThreshold {
				return fmt.Errorf("only cryptographically verified %d/%d approvals locally. Aborting apply.", validApprovals, sub.ApprovalThreshold)
			}

			fmt.Println("\n[SUCCESS] MULTI-PARTY ZERO-TRUST VERIFICATION PASSED. Executing apply...")
		} else {
			fmt.Println("Warning: No plan file detected in arguments. Skipping verification.")
		}
	}

	// execute terraform command
	cmd := exec.Command("terraform", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform execution failed: %w", err)
	}

	return nil
}
