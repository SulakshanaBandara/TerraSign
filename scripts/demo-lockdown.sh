#!/bin/bash
# High-Impact Demo Script: "Emergency Lockdown"

set -e
source scripts/setup-env.sh

echo "==================================================="
echo "🎬 SCENARIO: EMERGENCY LOCKDOWN DEMO"
echo "==================================================="
echo ""
echo "1. Start server (if not running)"
# Check if server running
if ! lsof -i :8081 >/dev/null; then
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
terrasign submit-for-review --service http://localhost:8081 tfplan
echo "[OK] Normal submission succeeded"

echo ""
echo "==================================================="
echo "🚨 SCENARIO: ACTIVE ATTACK DETECTED!"
echo "==================================================="
echo "Invigilator asks: 'What if you detect an intruder?'"
echo ""
echo "YOU RUN: terrasign lockdown on"
terrasign lockdown on --service http://localhost:8081

echo ""
echo "3. Attacker tries to submit a malicious plan (Should FAIL)"
if terrasign submit-for-review --service http://localhost:8081 tfplan; then
    echo "[FAIL] Submission should have been rejected!"
    exit 1
else
    echo "[SUCCESS] Submission rejected by Lockdown Mode!"
fi

echo ""
echo "==================================================="
echo "✅ SCENARIO: THREAT NEUTRALIZED"
echo "==================================================="
echo "Invigilator is impressed."
echo ""
echo "4. Lifting lockdown..."
terrasign lockdown off --service http://localhost:8081
echo "[OK] System normal again."
