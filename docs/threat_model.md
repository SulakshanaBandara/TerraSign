# Threat Model: TerraSign

## 1. System Overview
TerraSign intercepts Terraform operations to ensure that the infrastructure plan being applied matches the one that was approved and signed.

## 2. Trust Boundaries
-   **Developer Workstation**: Trusted to generate valid plans.
-   **CI/CD Pipeline**: Semi-trusted. Can generate plans, but could be compromised to inject malicious code.
-   **Terraform State Backend**: Trusted storage for state.
-   **Sigstore (Fulcio/Rekor)**: Trusted external authority for identities and transparency logging.
-   **Verification Environment**: The environment where `terraform apply` runs. Must be trusted to enforce verification.

## 3. Threat Analysis (STRIDE)

### Spoofing
-   **Threat**: Attacker manages to sign a malicious plan with a valid key.
-   **Mitigation**: Use short-lived certificates via OIDC (Fulcio) instead of long-lived keys. Multi-factor authentication for identity providers.

### Tampering
-   **Threat**: `tfplan` file is modified after signing but before applying.
-   **Mitigation**: Cryptographic signature covers the hash of the plan file. Any modification invalidates the signature.

### Repudiation
-   **Threat**: A user denies deploying a specific plan.
-   **Mitigation**: All signatures are properly logged in the transparency log (Rekor).

### Information Disclosure
-   **Threat**: The `tfplan` contains sensitive variables.
-   **Mitigation**: TerraSign only signs the hash of the plan. The plan content itself is not uploaded to Rekor. Users must ensure `tfplan` is stored securely.

### Denial of Service
-   **Threat**: Sigstore services are down, preventing verification.
-   **Mitigation**: Implement offline verification capabilities (optional, if public keys are cached) or fail-closed (security over availability).

### Elevation of Privilege
-   **Threat**: Attacker bypasses the verification hook.
-   **Mitigation**: Integrate TerraSign deeply into the CI/CD pipeline or as a wrapper script that *must* be used to run Terraform.
