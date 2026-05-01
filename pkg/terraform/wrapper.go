package terraform

import (
	"crypto/sha256"
	"encoding/hex"
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

				fmt.Printf("  Downloading bundle for approver: %s...\n", approval.Reviewer)
				if err := client.DownloadBundleForApprover(sub.ID, approval.Reviewer, bundlePath); err != nil {
					return fmt.Errorf("failed to download bundle for %s: %w", approval.Reviewer, err)
				}

				defer os.Remove(bundlePath)
			}

			// 4. Download policy attestation and SLSA provenance so Steps 2 & 3 of
			//    verifier.Verify() can read them (they are stored server-side after admin sign).
			policyPath := planFile + ".policy"
			provenancePath := planFile + ".provenance"
			if err := client.DownloadPolicyAttestation(sub.ID, policyPath); err != nil {
				fmt.Printf("  [INFO] Policy attestation not available on server: %v\n", err)
			} else {
				fmt.Println("  [OK] Policy attestation downloaded")
				defer os.Remove(policyPath)
			}
			if err := client.DownloadProvenance(sub.ID, provenancePath); err != nil {
				fmt.Printf("  [INFO] SLSA provenance not available on server: %v\n", err)
			} else {
				fmt.Println("  [OK] SLSA provenance downloaded")
				defer os.Remove(provenancePath)
			}

			// Verify each approver's bundle.
			// We rename each approver's bundle to planFile+".bundle" (the path verifier.Verify expects),
			// run verification, then clean up.

			validApprovals := 0
			for _, approval := range sub.Approvals {
				bundlePath := fmt.Sprintf("%s-%s.bundle", planFile, approval.Reviewer)

				// Move bundle to the expected location (planFile+".bundle") for verifier.Verify
				os.Rename(bundlePath, planFile+".bundle")

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
