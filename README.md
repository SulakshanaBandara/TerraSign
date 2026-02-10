# TerraSign

**Enterprise-Grade Infrastructure Supply Chain Security**

TerraSign ensures the integrity of your Infrastructure as Code (IaC) pipelines by implementing cryptographic signing, verification, and **separation of duties** for Terraform plans.

## Why TerraSign?

Infrastructure supply chain attacks are real. TerraSign implements industry-standard security controls:

1. **Separation of Duties**: Developers cannot sign their own infrastructure changes
2. **Cryptographic Verification**: Only signed plans can be deployed
3. **Human-in-the-Loop**: Security admins review and approve all changes
4. **Audit Trail**: Immutable transparency log via Sigstore/Rekor

## Architecture

```
Developer → CI (Plan) → Signing Service → Admin Review → CI (Apply)
                            ↓                    ↓
                      Unsigned Plan        Signed Plan
```

**Key Principle:** The system that creates the plan (CI) cannot sign it. Only security admins with the private key can approve deployments.

## Multi-Layer Security Architecture

TerraSign implements defense in depth with four security layers:

1. Layer 1: Git Commit Signing - Cryptographic proof of code authorship (GPG)
2. Layer 2: Terraform Plan Signing - Admin approval for infrastructure changes (Cosign)
3. Layer 3: Policy Enforcement - Automated security rules (OPA)
4. Layer 4: SLSA Provenance - Complete audit trail and supply chain security

This multi-layer approach ensures that even if one layer is compromised, others provide protection.

## Quick Start

### For Developers

1. Setup GPG signing:
   ```bash
   ./scripts/setup-gpg.sh
   ```

2. Add your public key to GitHub (see output from setup script)

3. Make signed commits:
   ```bash
   git commit -m "Your changes"  # Automatically signed
   ```

### For Admins

### Installation

```bash
go install github.com/sulakshanakarunarathne/terrasign/cmd/terrasign@latest
```

### Basic Workflow

#### 1. Start Signing Service

```bash
terrasign server --port 8080
```

#### 2. CI: Submit Plan for Review

```bash
terraform plan -out=tfplan
terrasign submit-for-review --service http://localhost:8080 tfplan
```

#### 3. Admin: Review and Sign

```bash
# List pending plans
terrasign admin list-pending

# Download for review
terrasign admin download <plan-id>
terraform show tfplan

# Sign if approved
terrasign admin sign <plan-id> --key admin.key
```

#### 4. CI: Apply Verified Plan

```bash
terrasign wrap --key admin.pub -- apply tfplan
```

## Security Features

### Separation of Duties
- CI pipeline **cannot** sign plans (no private key)
- Security admin **must** review and sign
- Prevents auto-deployment of malicious code

### Attack Prevention

| Attack | Defense |
|--------|---------|
| Malicious developer pushes bad code | Admin reviews plan, rejects deployment |
| Compromised CI server | Cannot sign without admin key |
| Plan tampering | Cryptographic signature verification fails |
| Replay attack | Freshness checks (Phase 4) |

## Commands

### CI Commands
- `terrasign submit-for-review` - Submit plan to signing service
- `terrasign wrap` - Enforce verification before apply

### Admin Commands
- `terrasign admin list-pending` - List plans awaiting review
- `terrasign admin download <id>` - Download plan for review
- `terrasign admin sign <id>` - Sign approved plan
- `terrasign admin reject <id>` - Reject plan

### Server Commands
- `terrasign server` - Start signing service

### Local Commands (Testing)
- `terrasign sign` - Sign plan locally
- `terrasign verify` - Verify signed plan

## CI/CD Integration

### Jenkins

See [`Jenkinsfile`](file:///Users/sulakshanakarunarathne/Documents/IIT/4th-year/FYP/FYP-Demo/Jenkinsfile) for full example.

```groovy
stage('Submit for Review') {
    steps {
        sh 'terrasign submit-for-review tfplan'
    }
}

stage('Apply (After Admin Approval)') {
    steps {
        sh 'terrasign wrap --key admin.pub -- apply tfplan'
    }
}
```

### GitHub Actions

See [`.github/workflows/terrasign.yml`](file:///Users/sulakshanakarunarathne/Documents/IIT/4th-year/FYP/FYP-Demo/.github/workflows/terrasign.yml) for full example.

## Documentation

- **[Walkthrough](file:///Users/sulakshanakarunarathne/.gemini/antigravity/brain/d2d1eb53-63ba-4075-822f-c7b44285921e/walkthrough.md)**: Complete demo with attack scenarios
- **[Implementation Plan](file:///Users/sulakshanakarunarathne/.gemini/antigravity/brain/d2d1eb53-63ba-4075-822f-c7b44285921e/implementation_plan.md)**: Architecture and design decisions
- **[CI/CD Integration](file:///Users/sulakshanakarunarathne/.gemini/antigravity/brain/d2d1eb53-63ba-4075-822f-c7b44285921e/cicd_integration.md)**: Jenkins and GitHub Actions setup

## Roadmap

- [x] **Phase 1**: Separation of Duties
- [ ] **Phase 2**: Policy-as-Code (OPA integration)
- [ ] **Phase 3**: SLSA Provenance Attestation
- [ ] **Phase 4**: Enhanced Verification (freshness, replay protection)

## License

MIT
# Security update
# Test change
# Backdoor code
