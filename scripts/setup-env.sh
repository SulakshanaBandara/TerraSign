#!/bin/bash
# TerraSign Environment Setup Script
# Run: source scripts/setup-env.sh

echo "Setting up TerraSign environment..."

# Add terrasign to PATH
export PATH=$PATH:$HOME/go/bin

# Set# Service URL (Local)
export TERRASIGN_SERVICE="http://localhost:8081"

# Set empty password for demo keys (NEVER do this in production!)
export COSIGN_PASSWORD=""

# Set key paths
export TERRASIGN_ADMIN_KEY="./examples/simple-app/admin.key"
export TERRASIGN_PUBLIC_KEY="./examples/simple-app/admin.pub"

# Helpful Aliases (forced to port 8081 to avoid Jenkins conflict)
alias ts='terrasign'
alias ts-submit='terrasign submit-for-review --service http://localhost:8081'
alias ts-list='terrasign admin list-pending --service http://localhost:8081'
alias ts-monitor='terrasign monitor --service http://localhost:8081'
alias ts-lockdown='terrasign lockdown --service http://localhost:8081'

# Use function for sign to run in initialized directory (subshell)
ts-sign() {
    (cd examples/simple-app && terrasign admin sign --service http://localhost:8081 "$@")
}

# Use function for verify to run in initialized directory (but keep user there)
ts-verify() {
    cd examples/simple-app && terrasign wrap --key admin.pub -- "$@"
}

echo "[OK] Environment configured!"
echo ""
echo "Available aliases:"
echo "  ts          - terrasign command"
echo "  ts-submit   - Submit plan for review"
echo "  ts-sign     - Sign a plan (usage: ts-sign <ID>)"
echo "  ts-monitor  - Live security dashboard"
echo "  ts-lockdown - Emergency lockdown control"
echo "  ts-verify   - Wrapper to verify & apply" (usage: ts-verify apply tfplan)"
echo "  ts-list     - List pending submissions"
echo ""
echo "Example workflow:"
echo "  1. ts-submit --wait tfplan"
echo "  2. ts-sign <ID>"
echo "  3. ts-verify apply tfplan"
