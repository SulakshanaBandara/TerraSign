package signer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/policy"
	"github.com/sulakshanakarunarathne/terrasign/pkg/provenance"
)

// Sign signs a Terraform plan using Cosign
func Sign(planPath, keyPath string) error {
	return SignWithOptions(planPath, keyPath, false)
}

// SignWithOptions signs a Terraform plan with additional options
func SignWithOptions(planPath, keyPath string, skipPolicy bool) error {
	fmt.Printf("Signing plan at %s\n", planPath)

	// Step 1: Evaluate policies (skip if already done during submission)
	if !skipPolicy {
		fmt.Println("Evaluating security policies...")
		policyEngine := policy.NewPolicyEngine("./policies")
		policyResult, err := policyEngine.Evaluate(planPath)
		if err != nil {
			return fmt.Errorf("policy evaluation failed: %w", err)
		}

		if !policyResult.Passed {
			fmt.Println("\n[ERROR] POLICY VIOLATIONS DETECTED:")
			for _, violation := range policyResult.Violations {
				fmt.Printf("  - [%s] %s\n", violation.Policy, violation.Message)
			}
			return fmt.Errorf("plan failed %d policy check(s) - signing aborted", len(policyResult.Violations))
		}
		fmt.Println("[OK] All policy checks passed")

		// Save policy attestation
		if err := policyEngine.SaveAttestation(planPath, policyResult); err != nil {
			return fmt.Errorf("failed to save policy attestation: %w", err)
		}
	} else {
		fmt.Println("[SKIP] Policy evaluation (already done during submission)")
	}

	// Step 2: Generate SLSA provenance
	fmt.Println("Generating SLSA provenance...")
	builderID := "https://github.com/actions/runner/v2" // Or detect from environment
	if os.Getenv("JENKINS_URL") != "" {
		builderID = os.Getenv("JENKINS_URL")
	}

	provenanceGen := provenance.NewProvenanceGenerator(builderID)
	buildStartTime := time.Now().Add(-5 * time.Minute) // Approximate
	slsaProvenance, err := provenanceGen.Generate(planPath, buildStartTime)
	if err != nil {
		return fmt.Errorf("provenance generation failed: %w", err)
	}

	if err := provenanceGen.Save(slsaProvenance, planPath); err != nil {
		return fmt.Errorf("failed to save provenance: %w", err)
	}
	fmt.Println("[OK] Provenance generated")

	// Step 3: Sign with Cosign
	fmt.Println("Signing with cryptographic signature...")
	if keyPath != "" {
		fmt.Printf("Signing with key: %s\n", keyPath)
	} else {
		fmt.Println("Signing with keyless (OIDC)")
	}

	// Construct the cosign command
	// We MUST generate a bundle because cosign v3.x verify-blob requires it
	// to avoid ASN.1 / IEEE P1363 encoding errors with this specific key.
	bundleFile := planPath + ".bundle"
	sigFile := planPath + ".sig"

	args := []string{"sign-blob", "--yes",
		"--bundle", bundleFile,
		"--tlog-upload=false",
	}

	if keyPath != "" {
		args = append(args, "--key", keyPath)
	}

	args = append(args, planPath)

	cmd := exec.Command("cosign", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass through COSIGN_PASSWORD to avoid interactive prompt
	// Default to empty string for demo keys if not set
	env := os.Environ()
	hasPass := false
	for _, e := range env {
		if strings.HasPrefix(e, "COSIGN_PASSWORD=") {
			hasPass = true
			break
		}
	}
	if !hasPass {
		env = append(env, "COSIGN_PASSWORD=")
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign signing failed: %w", err)
	}

	// Extract signature from bundle (server upload endpoint expects raw file)
	if err := extractSignatureFromBundle(bundleFile, sigFile); err != nil {
		return fmt.Errorf("failed to extract signature from bundle: %w", err)
	}

	fmt.Printf("Successfully signed plan.\nSignature: %s\nBundle: %s\n", sigFile, bundleFile)
	fmt.Printf("Policy Attestation: %s\n", planPath+".policy")
	fmt.Printf("SLSA Provenance: %s\n", planPath+".provenance")
	return nil
}

// extractSignatureFromBundle parses the bundle and writes the signature to a file
func extractSignatureFromBundle(bundlePath, sigPath string) error {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}

	var bundle struct {
		MessageSignature struct {
			Signature string `json:"signature"`
		} `json:"messageSignature"`
	}

	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("failed to parse bundle JSON: %w", err)
	}

	if bundle.MessageSignature.Signature == "" {
		return fmt.Errorf("signature not found in bundle")
	}

	// Write signature to file (it is already base64 encoded in JSON)
	if err := os.WriteFile(sigPath, []byte(bundle.MessageSignature.Signature), 0644); err != nil {
		return fmt.Errorf("failed to write signature file: %w", err)
	}

	return nil
}
