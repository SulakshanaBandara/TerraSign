package signer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/policy"
	"github.com/sulakshanakarunarathne/terrasign/pkg/provenance"
)

// Sign signs a Terraform plan using Cosign
func Sign(planPath, keyPath string) error {
	fmt.Printf("Signing plan at %s\n", planPath)

	// Step 1: Evaluate policies
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
	// We must use --bundle to avoid interactive prompts or config errors.
	// We will extract the signature from the bundle afterwards.
	bundleFile := planPath + ".bundle"
	sigFile := planPath + ".sig"

	// Minimal args to generate bundle without Tlog upload (for key-based)
	// Note: --tlog-upload=false might require bundle in recent versions
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

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign signing failed: %w", err)
	}

	// Extract signature from bundle
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
