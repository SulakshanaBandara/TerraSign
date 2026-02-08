#!/bin/bash
# Complete TerraSign Demo Workflow
# This script demonstrates the full secure workflow

set -e

echo "========================================="
echo "TerraSign Secure Workflow Demo"
echo "========================================="
echo ""

# Source environment
source scripts/setup-env.sh

# Navigate to example
cd examples/simple-app

echo "Step 1: Creating Terraform Plan..."
terraform plan -out=tfplan
echo "[OK] Plan created"
echo ""

echo "Step 2: Submitting plan for review..."
SUBMISSION_OUTPUT=$(terrasign submit-for-review --service $TERRASIGN_SERVICE tfplan)
echo "$SUBMISSION_OUTPUT"

# Extract submission ID
PLAN_ID=$(echo "$SUBMISSION_OUTPUT" | grep "Submission ID:" | awk '{print $3}')
echo ""
echo "Submission ID: $PLAN_ID"
echo ""

echo "Step 3: Admin reviews pending submissions..."
terrasign admin list-pending --service $TERRASIGN_SERVICE
echo ""

echo "Step 4: Admin signs the plan..."
echo "Running: terrasign admin sign $PLAN_ID --key admin.key"
terrasign admin sign $PLAN_ID --key admin.key --service $TERRASIGN_SERVICE
echo "[OK] Plan signed"
echo ""

echo "Step 5: Downloading signature..."
curl -s -o tfplan.sig http://localhost:8081/download/$PLAN_ID/signature
echo "[OK] Signature downloaded"
echo ""

echo "Step 6: Verifying and applying plan..."
terrasign wrap --key admin.pub -- apply tfplan
echo ""

echo "========================================="
echo "Demo Complete!"
echo "========================================="
