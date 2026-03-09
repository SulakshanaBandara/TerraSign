#!/bin/bash
# High-Impact Demo Script: "Emergency Lockdown"
# NOTE: This script requires a GPG key ID to activate lockdown.
#       Find yours with: gpg --list-secret-keys --keyid-format LONG
#       Then set: export DEMO_GPG_KEY_ID="YOUR_KEY_ID_HERE"
#       Or pass it as first argument: ./demo-lockdown.sh <your-gpg-key-id>

# Do NOT use set -e here — we intentionally show failure scenarios mid-demo
source scripts/setup-env.sh

# Accept GPG key as argument or env variable
DEMO_GPG_KEY_ID="${1:-${DEMO_GPG_KEY_ID:-}}"

echo "==================================================="
echo "🎬 SCENARIO: EMERGENCY LOCKDOWN DEMO"
echo "==================================================="
echo ""
echo "1. Start server (if not running)"
# Check if server running
if ! lsof -i :8081 >/dev/null 2>&1; then
    echo "Starting server on 8081..."
    terrasign server --port 8081 --storage ./demo-storage &
    SERVER_PID=$!
    sleep 2
else
    echo "Server already running on 8081"
fi

echo ""
echo "2. Create a normal plan (Everything OK)"
cd examples/simple-app
terraform plan -out=tfplan >/dev/null

echo "Step 2.1: Open Dashboard (Optional)"
echo "Run 'terrasign monitor' in a separate terminal to see pending plans live!"
echo ""

terrasign submit-for-review --service http://localhost:8081 tfplan
echo "[OK] Normal submission succeeded"

echo ""
echo "==================================================="
echo "🚨 SCENARIO: ACTIVE ATTACK DETECTED!"
echo "==================================================="
echo "Invigilator asks: 'What if you detect an intruder?'"
echo ""

if [ -z "$DEMO_GPG_KEY_ID" ]; then
    echo "[DEMO NOTE] Set DEMO_GPG_KEY_ID or pass your GPG key ID as argument to activate lockdown."
    echo "  Example: ./scripts/demo-lockdown.sh A1B2C3D4E5F6"
    echo ""
    echo "  Skipping lockdown activation for this run. To find your key:"
    echo "  gpg --list-secret-keys --keyid-format LONG"
else
    echo "YOU RUN: terrasign lockdown on -k $DEMO_GPG_KEY_ID"
    terrasign lockdown on -k "$DEMO_GPG_KEY_ID" -r "Demo: intruder detected" --service http://localhost:8081

    echo ""
    echo "3. Attacker tries to submit a malicious plan (Should FAIL)"
    if terrasign submit-for-review --service http://localhost:8081 tfplan; then
        echo "[FAIL] Submission should have been rejected!"
    else
        echo "[SUCCESS] Submission rejected by Lockdown Mode!"
    fi

    echo ""
    echo "==================================================="
    echo "[SCENARIO: THREAT NEUTRALIZED]"
    echo "==================================================="
    echo "Invigilator is impressed."
    echo ""
    echo "4. Lifting lockdown..."
    terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY --service http://localhost:8081
    echo "[OK] System normal again."
fi
