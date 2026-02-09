#!/bin/bash
# Install TerraSign Git hooks for commit signature enforcement

set -e

echo "========================================="
echo "Installing TerraSign Git Hooks"
echo "========================================="
echo ""

# Get the repository root
REPO_ROOT=$(git rev-parse --show-toplevel)
HOOKS_DIR="$REPO_ROOT/.git/hooks"

# Create pre-push hook
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
# Pre-push hook to enforce GPG signed commits

echo "========================================="
echo "TerraSign: Verifying Commit Signatures"
echo "========================================="

# Get the range of commits being pushed
while read local_ref local_sha remote_ref remote_sha
do
    if [ "$local_sha" = "0000000000000000000000000000000000000000" ]; then
        # Branch is being deleted, skip
        continue
    fi
    
    if [ "$remote_sha" = "0000000000000000000000000000000000000000" ]; then
        # New branch, check all commits
        range="$local_sha"
    else
        # Existing branch, check new commits
        range="$remote_sha..$local_sha"
    fi
    
    # Check each commit in the range
    commits=$(git rev-list "$range")
    
    for commit in $commits; do
        # Get commit info
        author=$(git log --format='%an <%ae>' -n 1 "$commit")
        subject=$(git log --format='%s' -n 1 "$commit")
        
        echo ""
        echo "Checking commit: ${commit:0:8}"
        echo "  Author: $author"
        echo "  Subject: $subject"
        
        # Verify the commit signature
        if git verify-commit "$commit" 2>&1 | grep -q "Good signature"; then
            echo "  Status: [VERIFIED] GPG signature valid"
        else
            echo ""
            echo "========================================="
            echo "[ERROR] UNSIGNED COMMIT DETECTED"
            echo "========================================="
            echo ""
            echo "Commit $commit is not signed with a GPG key."
            echo ""
            echo "TerraSign requires all commits to be cryptographically signed"
            echo "to ensure code authenticity and prevent impersonation attacks."
            echo ""
            echo "To sign this commit:"
            echo "  git commit --amend -S --no-edit"
            echo ""
            echo "To configure automatic signing:"
            echo "  git config --global commit.gpgsign true"
            echo ""
            echo "For GPG setup instructions:"
            echo "  See docs/commit_signing_guide.md"
            echo ""
            echo "Push rejected for security reasons."
            echo "========================================="
            exit 1
        fi
    done
done

echo ""
echo "========================================="
echo "[OK] All commits verified"
echo "========================================="
echo "Proceeding with push..."
echo ""

exit 0
EOF

# Make hook executable
chmod +x "$HOOKS_DIR/pre-push"

echo "[OK] Pre-push hook installed"
echo ""
echo "The hook will now verify GPG signatures before every push."
echo "Unsigned commits will be rejected automatically."
echo ""
echo "To test:"
echo "  1. Make an unsigned commit: git commit --no-gpg-sign -m 'test'"
echo "  2. Try to push: git push"
echo "  3. Push will be rejected"
echo ""
echo "========================================="
