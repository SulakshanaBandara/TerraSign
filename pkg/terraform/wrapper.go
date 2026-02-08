package terraform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sulakshanakarunarathne/terrasign/pkg/verifier"
)

// Execute wraps the terraform command, intercepting "apply" to enforce verification.
// args are the arguments intended for terraform (e.g., "apply", "tfplan").
func Execute(args []string, keyPath, identity, issuer string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided to terraform wrapper")
	}

	command := args[0]
	
	// Check if the command is 'apply' and enforce verification
	if command == "apply" {
		// Identify the plan file argument.
		// Terraform apply [options] [PLAN]
		// Simple heuristic: try to find the last argument that doesn't start with '-'
		// This is brittle but sufficient for a prototype. A real implementation would parse flags.
		var planFile string
		for i := len(args) - 1; i >= 1; i-- {
			if !strings.HasPrefix(args[i], "-") {
				planFile = args[i]
				break
			}
		}

		if planFile != "" {
			fmt.Printf("Intercepted 'apply' command. Verifying plan: %s\n", planFile)
			
			if keyPath == "" && identity == "" {
				return fmt.Errorf("either --key or --identity must be provided for verification")
			}
            
            // Verification logic:
			if err := verifier.Verify(planFile, keyPath, identity, issuer); err != nil {
				return fmt.Errorf("PLAN VERIFICATION FAILED: %v. Aborting apply.", err)
			}
		} else {
			fmt.Println("Warning: No plan file detected in arguments. Skipping verification (this might be insecure).")
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
