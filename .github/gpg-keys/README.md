# Trusted GPG Keys

This directory contains public GPG keys of trusted developers.

## Adding Your Key

After generating your GPG key:

```bash
# Export your public key
gpg --armor --export your.email@example.com > .github/gpg-keys/yourname.asc

# Commit and push
git add .github/gpg-keys/yourname.asc
git commit -S -m "Add GPG public key for yourname"
git push
```

## Key Format

Keys should be in ASCII-armored format (`.asc` extension).

## Verification

CI/CD will import all keys from this directory to verify commit signatures.
