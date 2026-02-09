#!/bin/bash
# GPG Key Setup Script for TerraSign Developers

set -e

echo "========================================="
echo "TerraSign Developer GPG Key Setup"
echo "========================================="
echo ""

# Check if GPG is installed
if ! command -v gpg &> /dev/null; then
    echo "[ERROR] GPG is not installed!"
    echo "Install with: brew install gnupg (macOS) or apt-get install gnupg (Linux)"
    exit 1
fi

echo "[1/5] Checking for existing GPG keys..."
existing_keys=$(gpg --list-secret-keys --keyid-format=long 2>/dev/null | grep -c "^sec" || echo "0")

if [ "$existing_keys" -gt 0 ]; then
    echo "Found $existing_keys existing GPG key(s)"
    gpg --list-secret-keys --keyid-format=long
    echo ""
    read -p "Do you want to use an existing key? (y/n): " use_existing
    
    if [ "$use_existing" = "y" ]; then
        read -p "Enter the key ID (e.g., 3AA5C34371567BD2): " key_id
    else
        echo "[2/5] Generating new GPG key..."
        echo "Please enter your details when prompted:"
        gpg --full-generate-key
        
        # Get the newly created key ID
        key_id=$(gpg --list-secret-keys --keyid-format=long | grep "^sec" | tail -1 | awk '{print $2}' | cut -d'/' -f2)
    fi
else
    echo "No existing GPG keys found"
    echo "[2/5] Generating new GPG key..."
    echo "Please enter your details when prompted:"
    echo "  - Key type: (1) RSA and RSA (default)"
    echo "  - Key size: 4096"
    echo "  - Expiration: 0 (does not expire) or 1y (1 year)"
    echo "  - Real name: Your full name"
    echo "  - Email: Your email (must match Git config)"
    echo ""
    
    gpg --full-generate-key
    
    # Get the newly created key ID
    key_id=$(gpg --list-secret-keys --keyid-format=long | grep "^sec" | tail -1 | awk '{print $2}' | cut -d'/' -f2)
fi

echo ""
echo "[3/5] Configuring Git to use GPG key: $key_id"
git config --global user.signingkey "$key_id"
git config --global commit.gpgsign true
git config --global tag.gpgSign true

# Configure GPG TTY for terminal
echo 'export GPG_TTY=$(tty)' >> ~/.bashrc
echo 'export GPG_TTY=$(tty)' >> ~/.zshrc
export GPG_TTY=$(tty)

echo ""
echo "[4/5] Exporting public key for GitHub..."
public_key_file="$HOME/gpg-public-key.txt"
gpg --armor --export "$key_id" > "$public_key_file"

echo ""
echo "========================================="
echo "[5/5] Setup Complete!"
echo "========================================="
echo ""
echo "Your GPG public key has been saved to: $public_key_file"
echo ""
echo "Next steps:"
echo "1. Copy your public key:"
echo "   cat $public_key_file | pbcopy  # macOS"
echo "   cat $public_key_file | xclip -selection clipboard  # Linux"
echo ""
echo "2. Add to GitHub:"
echo "   - Go to: https://github.com/settings/keys"
echo "   - Click 'New GPG key'"
echo "   - Paste your public key"
echo ""
echo "3. Test signing:"
echo "   git commit -S -m 'Test signed commit'"
echo "   git log --show-signature -1"
echo ""
echo "Your commits will now be automatically signed!"
echo "========================================="
