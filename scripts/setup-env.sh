#!/bin/bash
# TerraSign Environment Setup Script
# Run:        source scripts/setup-env.sh         (verbose, for interactive use)
# Quiet load: TERRASIGN_QUIET=1 source scripts/setup-env.sh  (no output)

_ts_echo() { [ "${TERRASIGN_QUIET:-0}" = "1" ] || echo "$@"; }

_ts_echo "Setting up TerraSign environment..."

# Add terrasign to PATH
export PATH=$PATH:$HOME/go/bin

# Set Service URL (Local)
# Set Service URL (Local)
export TERRASIGN_SERVICE="http://localhost:8081"

# API Token for authenticating to the service
export TERRASIGN_TOKEN="demo-secret-token"

# Set empty password for demo keys (NEVER do this in production!)
export COSIGN_PASSWORD=""

# Set key paths
export TERRASIGN_ADMIN_KEY="./examples/simple-app/admin.key"
export TERRASIGN_PUBLIC_KEY="./examples/simple-app/admin.pub"

# Unalias ALL potentially conflicting aliases FIRST, before defining anything
unalias ts 2>/dev/null || true
unalias ts-submit 2>/dev/null || true
unalias ts-list 2>/dev/null || true
unalias ts-inspect 2>/dev/null || true
unalias ts-monitor 2>/dev/null || true
unalias ts-lockdown 2>/dev/null || true
unalias ts-sign 2>/dev/null || true
unalias ts-verify 2>/dev/null || true

# Helpful Aliases (forced to port 8081 to avoid Jenkins conflict)
alias ts="$HOME/go/bin/terrasign"
alias ts-admin="$HOME/go/bin/terrasign admin"
alias ts-list="$HOME/go/bin/terrasign admin list-pending --service http://localhost:8081"
alias ts-inspect="$HOME/go/bin/terrasign admin inspect --service http://localhost:8081"
alias ts-monitor="$HOME/go/bin/terrasign monitor --service http://localhost:8081"
alias ts-server="$HOME/go/bin/terrasign server --port 8081 --storage $PROJECT_ROOT/demo-storage"

# Use function for lockdown to properly pass the on/off argument
# SECURITY: lockdown on requires --gpg-key for identity verification
ts-lockdown() {
    local mode="$1"
    shift

    if [ -z "$mode" ]; then
        echo "Usage: ts-lockdown <on|off> [-k <gpg-key-id>] [-r \"<reason>\"]"
        echo ""
        echo "  on   : Activate emergency lockdown (requires -k <gpg-key-id>)"
        echo "  off  : Lift lockdown  (requires -k <admin.key> or --recovery-code TERRASIGN-EMERGENCY)"
        return 1
    fi

    if [ "$mode" = "on" ]; then
        # SECURITY: key is MANDATORY for lockdown activation
        if [[ ! "$*" =~ "-k" ]] && [[ ! "$*" =~ "--key" ]]; then
            echo ""
            echo "[ERROR] Lockdown activation requires your GPG key."
            echo ""
            echo "Usage: ts-lockdown on -k <gpg-key-id> -r \"<reason>\""
            echo ""
            echo "To find your key ID:"
            echo "  gpg --list-secret-keys --keyid-format LONG"
            echo ""
            echo "Example:"
            echo "  ts-lockdown on -k A1B2C3D4E5F6 -r \"Suspected breach\""
            return 1
        fi
        $HOME/go/bin/terrasign lockdown on --service http://localhost:8081 "$@"

    elif [ "$mode" = "off" ]; then
        # Allow key file OR recovery code for deactivation
        if [[ ! "$*" =~ "-k" ]] && [[ ! "$*" =~ "--key" ]] && [[ ! "$*" =~ "--recovery-code" ]]; then
            echo ""
            echo "[ERROR] Lockdown deactivation requires authentication."
            echo ""
            echo "Options:"
            echo "  ts-lockdown off -k <path/to/admin.key>"
            echo "  ts-lockdown off --recovery-code TERRASIGN-EMERGENCY"
            return 1
        fi
        $HOME/go/bin/terrasign lockdown off --service http://localhost:8081 "$@"

    else
        echo "[ERROR] Unknown mode: $mode. Must be 'on' or 'off'."
        return 1
    fi
}

# Get absolute path to project root (for other functions)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"


# Use function for sign to run in initialized directory (subshell)
# It tries to find the directory relative to current location OR project root
ts-submit() {
    local target_dir="$PROJECT_ROOT/examples/simple-app"
    # Change to target directory if it exists and we're not already there
    if [ -d "$target_dir" ]; then
        cd "$target_dir" || return 1
    fi
    $HOME/go/bin/terrasign submit-for-review --service http://localhost:8081 "$@"
}

ts-sign() {
    local target_dir="$PROJECT_ROOT/examples/simple-app"
    # Change to target directory if it exists and we're not already there
    if [ -d "$target_dir" ]; then
        cd "$target_dir" || return 1
    fi
    (
        # We also need to fix the key path to be absolute or relative to the new dir
        # The simplest way is to assume the key is in the target_dir
        $HOME/go/bin/terrasign admin sign --service http://localhost:8081 "$@"
    )
}

# Use function for verify to run in initialized directory
ts-verify() {
    local target_dir="$PROJECT_ROOT/examples/simple-app"
    # Change to target directory if it exists and we're not already there
    if [ -d "$target_dir" ]; then
        cd "$target_dir" || return 1
    fi
    $HOME/go/bin/terrasign wrap --key admin.pub -- "$@"
}

# Download signature for a submission
ts-download-sig() {
    local id="$1"
    if [ -z "$id" ]; then
        echo "Usage: ts-download-sig <submission-id>"
        echo "  Downloads the signature file to ./tfplan.sig"
        return 1
    fi
    curl -f -o tfplan.sig "http://localhost:8081/download/$id/signature" && \
        echo "[OK] Signature downloaded to tfplan.sig" || \
        echo "[ERROR] Failed to download signature (is the plan signed?)"
}

_ts_echo "[OK] Environment configured!"
_ts_echo ""
_ts_echo "Available aliases:"
_ts_echo "  ts          - terrasign command"
_ts_echo "  ts-submit   - Submit plan for review"
_ts_echo "  ts-list     - List pending submissions"
_ts_echo "  ts-inspect  - Inspect plan changes (usage: ts-inspect <ID>)"
_ts_echo "  ts-sign     - Sign a plan (usage: ts-sign <ID>)"
_ts_echo "  ts-monitor  - Live security dashboard"
_ts_echo "  ts-lockdown - Emergency lockdown control"
_ts_echo "  ts-verify   - Wrapper to verify & apply (usage: ts-verify apply tfplan)"
_ts_echo ""
_ts_echo "Example workflow:"
_ts_echo "  1. ts-submit tfplan"
_ts_echo "  2. ts-sign <ID>"
_ts_echo "  3. ts-verify apply tfplan"
