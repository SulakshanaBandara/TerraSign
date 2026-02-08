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
alias ts='terrasign'
alias ts-submit='terrasign submit-for-review --service http://localhost:8081'
alias ts-list='terrasign admin list-pending --service http://localhost:8081'
alias ts-inspect='terrasign admin inspect --service http://localhost:8081'
alias ts-monitor='terrasign monitor --service http://localhost:8081'

# Use function for lockdown to properly pass the on/off argument
ts-lockdown() {
    local mode="$1"
    shift
    
    if [ "$mode" = "off" ]; then
        # For lockdown off, use default key path if not specified
        if [[ ! "$@" =~ "--key" ]] && [[ ! "$@" =~ "--recovery-code" ]]; then
            # Find project root by looking for go.mod
            local current_dir="$PWD"
            local project_root="$current_dir"
            
            while [ "$project_root" != "/" ]; do
                if [ -f "$project_root/go.mod" ]; then
                    break
                fi
                project_root="$(dirname "$project_root")"
            done
            
            # If we didn't find go.mod, use current directory
            if [ ! -f "$project_root/go.mod" ]; then
                project_root="$current_dir"
            fi
            
            local default_key="$project_root/examples/simple-app/admin.key"
            terrasign lockdown off --service http://localhost:8081 --key "$default_key" "$@"
        else
            terrasign lockdown off --service http://localhost:8081 "$@"
        fi
    else
        # For lockdown on, no key required
        terrasign lockdown "$mode" --service http://localhost:8081 "$@"
    fi
}

# Get absolute path to project root (for other functions)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"


# Use function for sign to run in initialized directory (subshell)
# It tries to find the directory relative to current location OR project root
ts-sign() {
    local target_dir="$PROJECT_ROOT/examples/simple-app"
    (
        if [ -d "$target_dir" ]; then
            cd "$target_dir"
        fi
        # If we are already in the directory, the above might effectively do nothing or cd to same place.
        # If the user is somewhere else, it jumps to the right place.
        
        # We also need to fix the key path to be absolute or relative to the new dir
        # The simplest way is to assume the key is in the target_dir
        terrasign admin sign --service http://localhost:8081 "$@"
    )
}

# Use function for verify to run in initialized directory
ts-verify() {
    local target_dir="$PROJECT_ROOT/examples/simple-app"
    # Change to target directory if it exists and we're not already there
    if [ -d "$target_dir" ]; then
        cd "$target_dir" || return 1
    fi
    terrasign wrap --key admin.pub -- "$@"
}

echo "[OK] Environment configured!"
echo ""
echo "Available aliases:"
echo "  ts          - terrasign command"
echo "  ts-submit   - Submit plan for review"
echo "  ts-list     - List pending submissions"
echo "  ts-inspect  - Inspect plan changes (usage: ts-inspect <ID>)"
echo "  ts-sign     - Sign a plan (usage: ts-sign <ID>)"
echo "  ts-monitor  - Live security dashboard"
echo "  ts-lockdown - Emergency lockdown control"
echo "  ts-verify   - Wrapper to verify & apply (usage: ts-verify apply tfplan)"
echo ""
echo "Example workflow:"
echo "  1. ts-submit --wait tfplan"
echo "  2. ts-sign <ID>"
echo "  3. ts-verify apply tfplan"
