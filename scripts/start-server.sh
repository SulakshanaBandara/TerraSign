#!/bin/bash
# ============================================================
#  TerraSign Quick-Start Script
#  Usage: source scripts/start-server.sh [--port 8081]
#  
#  This script:
#    1. Builds the terrasign binary
#    2. Starts the signing service in the background
#    3. Sources setup-env.sh so all aliases (ts, ts-submit, etc.) are ready
# ============================================================

# Compatible way to get script directory when sourced in Bash or Zsh
if [ -n "${ZSH_VERSION:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${(%):-%x}")" && pwd)"
elif [ -n "${BASH_SOURCE[0]:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
else
    SCRIPT_DIR="$PWD/scripts" # Fallback
fi

PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
pushd "$PROJECT_ROOT" >/dev/null || { echo "[ERROR] Could not cd to project root"; return 1; }

PORT="${1:-8081}"
STORAGE_DIR="$PROJECT_ROOT/demo-storage"

echo "============================================================"
echo "  TerraSign Quick-Start"
echo "============================================================"

# Step 1: Build binary
echo ""
echo "[1/3] Building terrasign binary..."
if ! go build -o "$HOME/go/bin/terrasign" ./cmd/terrasign/; then
    echo "[ERROR] Build failed. Fix compilation errors above."
    return 1
fi
echo "      [OK] Binary built: $HOME/go/bin/terrasign"

# Step 2: Kill any existing server on the port
if lsof -i ":$PORT" >/dev/null 2>&1; then
    echo ""
    echo "[2/3] Stopping existing server on port $PORT..."
    lsof -ti ":$PORT" | xargs kill -9 2>/dev/null || true
    sleep 1
fi

# Step 3: Start the signing service
echo ""
echo "[2/3] Starting TerraSign signing service on port $PORT..."
mkdir -p "$STORAGE_DIR"
"$HOME/go/bin/terrasign" server --port "$PORT" --storage "$STORAGE_DIR" &
SERVER_PID=$!
sleep 1

# Verify it started
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "[ERROR] Server failed to start."
    return 1
fi
if ! lsof -i ":$PORT" >/dev/null 2>&1; then
    echo "[ERROR] Server not listening on port $PORT."
    return 1
fi
echo "      [OK] Server running (PID: $SERVER_PID) → http://localhost:$PORT"
echo "      Storage: $STORAGE_DIR"

# Step 4: Load all aliases
echo ""
echo "[3/3] Loading environment and aliases..."
source "$SCRIPT_DIR/setup-env.sh"

echo ""
echo "============================================================"
echo "  System READY"
echo "============================================================"
echo ""
echo "  Quick workflow:"
echo "    cd examples/simple-app"
echo "    terraform plan -out=tfplan"
echo "    ts-submit tfplan                          # Submit for review"
echo "    ts-list                                   # Admin: list pending"
echo "    ts-sign <ID>                              # Admin: sign plan"
echo "    ts-verify apply tfplan                    # Apply verified plan"
echo ""
echo "  Lockdown (emergency):"
echo "    ts-lockdown on -k <GPG_KEY_ID> -r \"reason\""
echo "    ts-lockdown off --recovery-code TERRASIGN-EMERGENCY"
echo ""
echo "  To stop the server:"
echo "    kill $SERVER_PID"
echo "============================================================"

popd >/dev/null || true
