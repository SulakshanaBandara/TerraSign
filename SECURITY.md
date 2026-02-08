# TerraSign Security Documentation

## Emergency Recovery Code

**Recovery Code:** `TERRASIGN-EMERGENCY`

This code can be used to deactivate lockdown mode if the admin key is unavailable:

```bash
terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY
```

### Production Deployment

In production environments:
- Store recovery code in a secure vault (e.g., HashiCorp Vault, AWS Secrets Manager)
- Implement code rotation policy (monthly/quarterly)
- Require multi-factor authentication for code retrieval
- Audit all recovery code usage
- Use cryptographically secure random codes (not predictable patterns)

### Access Control

Recovery code should only be accessible to:
- Senior security team members
- On-call incident responders
- Break-glass emergency procedures

**Never commit recovery codes to version control or expose in error messages.**
