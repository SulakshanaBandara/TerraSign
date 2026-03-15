#!/bin/bash
export COSIGN_PASSWORD=""
cd examples/simple-app
$HOME/go/bin/terrasign sign tfplan --key admin.key >/dev/null 2>&1
echo "=== 1. Direct verify ==="
$HOME/go/bin/terrasign verify tfplan --key admin.pub
echo "=== 2. Wrapper apply ==="
$HOME/go/bin/terrasign wrap --key admin.pub -- apply tfplan
