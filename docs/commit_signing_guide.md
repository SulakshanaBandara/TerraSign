# Git Commit Signing Guide

## Overview

TerraSign uses **multi-layer security** to protect infrastructure deployments:

- **Layer 1:** Git commit signing (Developer identity)
- **Layer 2:** Terraform plan signing (Deployment approval)
- **Layer 3:** Policy enforcement (Security rules)
- **Layer 4:** SLSA provenance (Audit trail)

This guide covers **Layer 1: Git Commit Signing**.

## Why Sign Commits?

Without signing:
- Anyone can impersonate you by setting `git config user.name "Your Name"`
- No proof of who actually wrote the code
- Attackers can inject malicious code

With signing:
- Cryptographic proof of authorship
- GitHub shows "Verified" badge
- CI/CD can reject unsigned commits
- Complete audit trail

## Quick Setup

Run the automated setup script:

```bash
./scripts/setup-gpg.sh
```

This will:
1. Generate a GPG key (or use existing)
2. Configure Git to sign commits automatically
3. Export your public key for GitHub

## Manual Setup

### 1. Generate GPG Key

```bash
gpg --full-generate-key
```

Choose:
- Key type: `(1) RSA and RSA`
- Key size: `4096`
- Expiration: `0` (never) or `1y` (1 year)
- Name: Your full name
- Email: **Must match your Git email**

### 2. Get Key ID

```bash
gpg --list-secret-keys --keyid-format=long
```

Output:
```
sec   rsa4096/3AA5C34371567BD2 2024-01-01 [SC]
      ^^^^^^^^^^^^^^^^
      This is your KEY_ID
```

### 3. Configure Git

```bash
git config --global user.signingkey 3AA5C34371567BD2
git config --global commit.gpgsign true
git config --global tag.gpgSign true
```

### 4. Export Public Key

```bash
gpg --armor --export 3AA5C34371567BD2 > gpg-public-key.txt
```

### 5. Add to GitHub

1. Go to https://github.com/settings/keys
2. Click "New GPG key"
3. Paste contents of `gpg-public-key.txt`
4. Click "Add GPG key"

## Usage

### Signing Commits

Commits are now signed automatically:

```bash
git commit -m "Add new feature"
```

To explicitly sign:

```bash
git commit -S -m "Add new feature"
```

### Verifying Signatures

```bash
# Verify last commit
git log --show-signature -1

# Verify specific commit
git verify-commit <commit-hash>
```

### Signing Tags

```bash
git tag -s v1.0.0 -m "Release v1.0.0"
```

## Troubleshooting

### "gpg: signing failed: Inappropriate ioctl for device"

Add to your shell profile (`~/.bashrc` or `~/.zshrc`):

```bash
export GPG_TTY=$(tty)
```

### "gpg: signing failed: No secret key"

Your key ID is not configured:

```bash
git config --global user.signingkey <YOUR_KEY_ID>
```

### "error: gpg failed to sign the data"

Check if GPG agent is running:

```bash
gpgconf --kill gpg-agent
gpg-agent --daemon
```

### Commits not showing "Verified" on GitHub

1. Ensure your GPG key email matches your GitHub email
2. Add the public key to GitHub (Settings → GPG keys)
3. Verify the key is trusted: `gpg --edit-key <KEY_ID>` → `trust` → `5` (ultimate)

## CI/CD Integration

### GitHub Actions

Commit signatures are automatically verified in `.github/workflows/verify-commits.yml`

### Jenkins

Commit verification runs in the "Verify Commit Signatures" stage

## Security Best Practices

1. **Protect your private key** - Never share or commit it
2. **Use a strong passphrase** - Protect key with password
3. **Backup your key** - Store securely offline
4. **Set expiration** - Rotate keys annually
5. **Revoke compromised keys** - Immediately if leaked

## For Admins: Enforcing Signed Commits

### GitHub Branch Protection

1. Repository Settings → Branches
2. Add rule for `main` branch
3. Enable "Require signed commits"
4. Unsigned commits will be rejected

### Local Git Hooks

Create `.git/hooks/pre-push`:

```bash
#!/bin/bash
git verify-commit HEAD || {
    echo "Error: Commit is not signed!"
    exit 1
}
```

## Multi-Layer Security in Action

```
Developer writes code
    ↓
[Layer 1] Signs commit with GPG key
    ↓
Pushes to GitHub
    ↓
CI/CD verifies commit signature
    ↓
Generates Terraform plan
    ↓
[Layer 2] Admin signs plan with Cosign
    ↓
[Layer 3] Policy engine validates changes
    ↓
[Layer 4] SLSA provenance recorded
    ↓
Infrastructure deployed
```

## References

- [GitHub: Signing commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)
- [GPG documentation](https://gnupg.org/documentation/)
- [Git commit signing](https://git-scm.com/book/en/v2/Git-Tools-Signing-Your-Work)
