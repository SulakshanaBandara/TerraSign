# System Architecture: TerraSign

## 1. Components

### 1.1 CLI (terrasign)
-   **Role**: Entry point for users and CI/CD pipelines.
-   **Commands**:
    -   `sign`: Signs a `tfplan` file using Cosign.
    -   `verify`: Verifies the signature of a `tfplan` file against the transparency log.

### 1.2 Signer (pkg/signer)
-   **Dependencies**: `github.com/sigstore/cosign/v2`.
-   **Flow**:
    1.  Read `tfplan` binary.
    2.  Calculate SHA256 hash.
    3.  Authenticate with OIDC provider (or use static keys).
    4.  Sign the hash.
    5.  Upload signature to Rekor transparency log.
    6.  Output: Signature bundle (optional, if detached) or reference to Rekor entry.

### 1.3 Verifier (pkg/verifier)
-   **Dependencies**: `github.com/sigstore/cosign/v2`, `github.com/sigstore/rekor`.
-   **Flow**:
    1.  Read `tfplan` binary.
    2.  Calculate SHA256 hash.
    3.  Retrieve signature/entry from Rekor.
    4.  Verify signature against the public key/identity.
    5.  Verify inclusion proof in Rekor.

### 1.4 Terraform Wrapper (pkg/terraform)
-   **Role**: Enforces `terrasign verify` before `terraform apply`.
-   **Implementation**: A function that wraps `exec.Command("terraform", ...)` and injects the verification step.

## 2. Data Flow

1.  **Generate Plan**: User/CI runs `terraform plan -out=tfplan`.
2.  **Sign Plan**: User/CI runs `terrasign sign tfplan`.
    -   `tfplan` -> Hash -> Sign -> Rekor.
3.  **Distribute**: `tfplan` is passed to the apply stage.
4.  **Verify & Apply**: User/CI runs `terrasign verify tfplan && terraform apply tfplan`.
    -   `tfplan` -> Hash -> Verify against Rekor.
    -   If valid -> Execute `terraform apply`.

## 3. Integration with Sigstore
We will use Cosign as a library to handle the heavy lifting of OIDC authentication, key management, and interaction with Rekor.
