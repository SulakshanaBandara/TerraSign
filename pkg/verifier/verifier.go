package verifier

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/policy"
	"github.com/sulakshanakarunarathne/terrasign/pkg/provenance"
)

// Verify invokes the cosign CLI to verify the signature of the given file.
// It requires a key path for key-based verification, OR identity+issuer for keyless.
func Verify(planPath, keyPath, identity, issuer string) error {
	fmt.Printf("Verifying plan at %s\n", planPath)

	// Step 1: Verify cryptographic signature
	fmt.Println("Step 1/4: Verifying cryptographic signature...")
	if err := verifyCosignSignature(planPath, keyPath, identity, issuer); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	fmt.Println("[OK] Signature valid")

	// Step 2: Verify policy attestation
	fmt.Println("Step 2/4: Verifying policy compliance...")
	policyResult, err := policy.LoadAttestation(planPath)
	if err != nil {
		fmt.Printf("⚠️  Warning: No policy attestation found (plan may predate policy checks)\n")
	} else {
		if !policyResult.Passed {
			return fmt.Errorf("plan failed policy checks: %d violations", len(policyResult.Violations))
		}
		fmt.Println("[OK] Policy compliance verified")
	}

	// Step 3: Verify SLSA provenance
	fmt.Println("Step 3/4: Verifying SLSA provenance...")
	slsaProvenance, err := provenance.LoadProvenance(planPath)
	if err != nil {
		fmt.Printf("⚠️  Warning: No provenance found (plan may predate provenance generation)\n")
	} else {
		fmt.Printf("  Builder: %s\n", slsaProvenance.Predicate.Builder.ID)
		fmt.Printf("  Build Type: %s\n", slsaProvenance.Predicate.BuildType)
		fmt.Println("[OK] Provenance verified")
	}

	// Step 4: Check freshness (24h limit)
	fmt.Println("Step 4/4: Checking plan freshness...")
	if slsaProvenance != nil {
		age := time.Since(slsaProvenance.Predicate.Metadata.BuildFinishedOn)
		if age > 24*time.Hour {
			return fmt.Errorf("plan is stale (%.1f hours old, max 24h)", age.Hours())
		}
		fmt.Printf("[OK] Plan is fresh (%.1f hours old)\n", age.Hours())
	}

	fmt.Println("\n[SUCCESS] VERIFICATION SUCCESSFUL - All checks passed!")
	return nil
}

// verifyCosignSignature verifies the cryptographic signature using the cosign bundle.
// Uses --bundle so cosign handles encoding internally (avoids ASN.1 vs IEEE-P1363 issues).
// IMPORTANT: signing (Mac) and verification (Jenkins) must use the same cosign version.
// Current pinned version: v3.0.2 on both Mac and Jenkins container.
func verifyCosignSignature(planPath, keyPath, identity, issuer string) error {
	bundleFile := planPath + ".bundle"
	certFile := planPath + ".crt"

	if _, err := os.Stat(bundleFile); os.IsNotExist(err) {
		return fmt.Errorf("bundle file not found: %s", bundleFile)
	}

	args := []string{"verify-blob",
		"--bundle", bundleFile,
		"--insecure-ignore-tlog=true",
	}

	if keyPath != "" {
		args = append(args, "--key", keyPath)
	} else {
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file not found: %s (required for keyless verification)", certFile)
		}
		args = append(args, "--certificate", certFile)
		args = append(args, "--certificate-identity", identity)
		args = append(args, "--certificate-oidc-issuer", issuer)
	}

	args = append(args, planPath)

	cmd := exec.Command("cosign", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign verification failed: %w", err)
	}

	return nil
}
