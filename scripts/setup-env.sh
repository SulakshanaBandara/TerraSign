#!/bin/bash
# TerraSign Environment Setup Script
# Run: source scripts/setup-env.sh

echo "Setting up TerraSign environment..."

# Add terrasign to PATH
export PATH=$PATH:$HOME/go/bin

# Set service URL
export TERRASIGN_SERVICE="http://localhost:8081"

# Set empty password for demo keys (NEVER do this in production!)
export COSIGN_PASSWORD=""

# Set key paths
export TERRASIGN_ADMIN_KEY="./examples/simple-app/admin.key"
export TERRASIGN_PUBLIC_KEY="./examples/simple-app/admin.pub"

# Create helpful aliases
alias ts='terrasign'
alias ts-submit='terrasign submit-for-review --service $TERRASIGN_SERVICE'
alias ts-sign='cd examples/simple-app && terrasign admin sign --service $TERRASIGN_SERVICE --key admin.key'
alias ts-verify='cd examples/simple-app && terrasign wrap --key admin.pub --'
alias ts-list='terrasign admin list-pending --service $TERRASIGN_SERVICE'

echo "✓ Environment configured!"
echo ""
echo "Available aliases:"
echo "  ts          - terrasign command"
echo "  ts-submit   - Submit plan for review"
echo "  ts-sign     - Sign a plan (usage: ts-sign <ID>)"
echo "  ts-verify   - Verify and apply (usage: ts-verify apply tfplan)"
echo "  ts-list     - List pending submissions"
echo ""
echo "Example workflow:"
echo "  1. ts-submit --wait tfplan"
echo "  2. ts-sign <ID>"
echo "  3. ts-verify apply tfplan"
