# TerraSign Quick Start Guide

## For Demo/Development

### 1. One-Time Setup

```bash
# Install TerraSign
go install ./cmd/terrasign

# Setup environment (creates aliases and sets env vars)
source scripts/setup-env.sh
```

### 2. Start Server

```bash
# Terminal 1
terrasign server --port 8081 --storage ./demo-storage
```

### 3. Run Demo Workflow

**Option A: Automated Script**
```bash
# Runs complete workflow automatically
./scripts/demo-workflow.sh
```

**Option B: Manual (for presentation)**

**Terminal 2 (Developer/CI):**
```bash
cd examples/simple-app
terraform plan -out=tfplan
ts-submit tfplan  # Note the ID
```

**Terminal 3 (Admin):**
```bash
ts-list           # See pending plans
ts-sign <ID>      # Sign the plan
```

**Terminal 2 (Developer/CI):**
```bash
# Download signature manually
curl -o tfplan.sig http://localhost:8081/download/<ID>/signature

# Verify and apply
ts-verify apply tfplan
```

## For Production

### Jenkins Setup

1. Add credentials in Jenkins:
   - `terrasign-service-url`: `http://terrasign-server:8081`
   - `cosign-password`: (empty for demo keys)
   - `admin-public-key-path`: Path to `admin.pub`

2. Use the provided `Jenkinsfile`

3. Admin signs plans via:
   ```bash
   terrasign admin sign <ID> --key /secure/path/admin.key
   ```

### GitHub Actions Setup

1. Add secrets in repository settings:
   - `TERRASIGN_SERVICE_URL`
   - `COSIGN_PASSWORD`
   - `ADMIN_PUBLIC_KEY` (content of admin.pub)

2. Enable environment protection:
   - Settings → Environments → Create "production"
   - Add required reviewers

3. Workflow runs automatically on push

## Security Best Practices

### DO:
- Store `admin.key` in secure vault (HashiCorp Vault, AWS Secrets Manager)
- Use environment variables for secrets
- Restrict terraform binary access in CI
- Enable audit logging

### DON'T:
- Commit private keys to git
- Use passwords in command line
- Allow direct `terraform apply` in CI
- Share admin keys across teams

## Troubleshooting

**"command not found: terrasign"**
```bash
export PATH=$PATH:$HOME/go/bin
# Or run: source scripts/setup-env.sh
```

**"timeout waiting for signature"**
- Ensure you're signing the correct submission ID
- Check server is running on correct port
- Verify network connectivity

**"signature verification failed"**
- Ensure plan hasn't been modified
- Check you're using matching key pair
- Verify signature was downloaded correctly

## Configuration

Create `.terrasign.yaml` in project root:
```yaml
service:
  url: http://localhost:8081
  timeout: 5m

keys:
  admin_private: examples/simple-app/admin.key
  admin_public: examples/simple-app/admin.pub
```

Then commands become shorter:
```bash
terrasign submit tfplan
terrasign admin sign <ID>
```
