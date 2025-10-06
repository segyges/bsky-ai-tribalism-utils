#!/bin/bash

set -e

echo "============================================"
echo "Ozone Configuration Setup Script (Local)"
echo "============================================"
echo ""
echo "This script will generate configuration files"
echo "locally and push them to your remote server."
echo ""

# Check for required commands locally
for cmd in openssl curl jq xxd ssh scp; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: $cmd is not installed locally. Please install it first."
        exit 1
    fi
done

# Prompt for remote server details
echo "Step 1: Remote Server Configuration"
echo "------------------------------------"
read -p "Enter remote server address (e.g., user@server.com): " REMOTE_SERVER
if [ -z "$REMOTE_SERVER" ]; then
    echo "Error: Remote server address is required"
    exit 1
fi

# Test SSH connection
echo ""
echo "Testing SSH connection..."
if ! ssh -o ConnectTimeout=5 "$REMOTE_SERVER" "echo 'Connection successful'" &> /dev/null; then
    echo "Error: Could not connect to $REMOTE_SERVER"
    echo "Please check your SSH configuration and try again."
    exit 1
fi
echo "✓ SSH connection successful"

# Check if configuration files already exist on remote
echo ""
echo "Checking for existing configuration files on remote server..."
echo ""

EXISTING_FILES=$(ssh "$REMOTE_SERVER" "sudo bash -c '
FILES=()
[ -f /ozone/caddy/etc/caddy/Caddyfile ] && FILES+=(\"/ozone/caddy/etc/caddy/Caddyfile\")
[ -f /ozone/postgres.env ] && FILES+=(\"/ozone/postgres.env\")
[ -f /ozone/ozone.env ] && FILES+=(\"/ozone/ozone.env\")
[ -f /ozone/CREDENTIALS.txt ] && FILES+=(\"/ozone/CREDENTIALS.txt\")
if [ \${#FILES[@]} -gt 0 ]; then
    for file in \"\${FILES[@]}\"; do
        echo \"\$file\"
    done
fi
'")

if [ -n "$EXISTING_FILES" ]; then
    echo "⚠️  WARNING: The following configuration files already exist on remote:"
    echo "$EXISTING_FILES" | while read -r file; do
        echo "  • $file"
    done
    echo ""
    read -p "Do you want to overwrite these files? (y/N): " OVERWRITE
    if [[ ! "$OVERWRITE" =~ ^[Yy]$ ]]; then
        echo "Aborting setup. No files were modified."
        exit 0
    fi
    echo ""
    echo "Will overwrite existing files..."
    echo ""
else
    echo "✓ No existing configuration files found on remote"
    echo ""
fi

# Prompt for domain name
echo "Step 2: Domain Configuration"
echo "-----------------------------"
read -p "Enter your Ozone domain (e.g., ozone.example.com): " OZONE_HOSTNAME
if [ -z "$OZONE_HOSTNAME" ]; then
    echo "Error: Domain name is required"
    exit 1
fi

# Prompt for service account handle
echo ""
echo "Step 3: Bluesky Service Account"
echo "--------------------------------"
read -p "Enter your labeler service account handle (e.g., mylabeler.bsky.social): " OZONE_SERVICE_ACCOUNT_HANDLE
if [ -z "$OZONE_SERVICE_ACCOUNT_HANDLE" ]; then
    echo "Error: Service account handle is required"
    exit 1
fi

# Prompt for contact email
echo ""
echo "Step 4: Contact Email"
echo "---------------------"
read -p "Enter your technical contact email (for Let's Encrypt): " CONTACT_EMAIL
if [ -z "$CONTACT_EMAIL" ]; then
    echo "Error: Contact email is required"
    exit 1
fi

echo ""
echo "Resolving service account DID..."
OZONE_SERVER_DID=$(curl --fail --silent --show-error "https://api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=${OZONE_SERVICE_ACCOUNT_HANDLE}" | jq --raw-output .did)

if [ -z "$OZONE_SERVER_DID" ] || [ "$OZONE_SERVER_DID" == "null" ]; then
    echo "Error: Could not resolve DID for handle ${OZONE_SERVICE_ACCOUNT_HANDLE}"
    echo "Please make sure the account exists on Bluesky"
    exit 1
fi

echo "✓ Resolved DID: ${OZONE_SERVER_DID}"

# Generate passwords and keys locally
echo ""
echo "Generating secure credentials locally..."
OZONE_ADMIN_PASSWORD=$(openssl rand --hex 16)
OZONE_SIGNING_KEY_HEX=$(openssl ecparam --name secp256k1 --genkey --noout --outform DER | tail --bytes=+8 | head --bytes=32 | xxd --plain --cols 32)
POSTGRES_PASSWORD=$(openssl rand --hex 16)

echo "✓ Generated admin password"
echo "✓ Generated signing key"
echo "✓ Generated database password"

# Create temporary directory for config files
TEMP_DIR=$(mktemp -d)
echo ""
echo "Creating configuration files locally..."

# Create Caddyfile
cat > "$TEMP_DIR/Caddyfile" <<CADDYFILE
${OZONE_HOSTNAME} {
  tls ${CONTACT_EMAIL}
  reverse_proxy http://localhost:3000
}
CADDYFILE

echo "✓ Caddyfile created"

# Create Postgres env file
cat > "$TEMP_DIR/postgres.env" <<POSTGRES_CONFIG
POSTGRES_USER=postgres
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=ozone
POSTGRES_CONFIG

echo "✓ Postgres config created"

# Create Ozone env file
cat > "$TEMP_DIR/ozone.env" <<OZONE_CONFIG
OZONE_SERVER_DID=${OZONE_SERVER_DID}
OZONE_PUBLIC_URL=https://${OZONE_HOSTNAME}
OZONE_ADMIN_DIDS=${OZONE_SERVER_DID}
OZONE_ADMIN_PASSWORD=${OZONE_ADMIN_PASSWORD}
OZONE_SIGNING_KEY_HEX=${OZONE_SIGNING_KEY_HEX}
OZONE_DB_POSTGRES_URL=postgresql://postgres:${POSTGRES_PASSWORD}@localhost:5432/ozone
OZONE_DB_MIGRATE=1
OZONE_DID_PLC_URL=https://plc.directory
OZONE_APPVIEW_URL=https://api.bsky.app
OZONE_APPVIEW_DID=did:web:api.bsky.app
LOG_ENABLED=1
OZONE_CONFIG

echo "✓ Ozone config created"

# Create credentials file
cat > "$TEMP_DIR/CREDENTIALS.txt" <<CREDENTIALS
============================================
OZONE CREDENTIALS - KEEP THIS FILE SECURE!
============================================

Domain: ${OZONE_HOSTNAME}
Service Account: ${OZONE_SERVICE_ACCOUNT_HANDLE}
Service DID: ${OZONE_SERVER_DID}
Contact Email: ${CONTACT_EMAIL}

Ozone Admin Password: ${OZONE_ADMIN_PASSWORD}
Postgres Password: ${POSTGRES_PASSWORD}

Ozone URL: https://${OZONE_HOSTNAME}

IMPORTANT: Store these credentials securely!
You will need the admin password to access Ozone.
CREDENTIALS

echo "✓ Credentials file created"

# Also save credentials locally
cp "$TEMP_DIR/CREDENTIALS.txt" ./ozone-credentials.txt
chmod 600 ./ozone-credentials.txt

# Create directories on remote and upload files
echo ""
echo "Creating directories on remote server..."
ssh "$REMOTE_SERVER" "sudo mkdir -p /ozone/postgres /ozone/caddy/data /ozone/caddy/etc/caddy"
echo "✓ Directories created on remote"

echo ""
echo "Uploading configuration files to remote server..."

# Upload files using a temporary location first, then move with sudo
scp "$TEMP_DIR/Caddyfile" "$REMOTE_SERVER:/tmp/Caddyfile"
ssh "$REMOTE_SERVER" "sudo mv /tmp/Caddyfile /ozone/caddy/etc/caddy/Caddyfile"
echo "✓ Uploaded Caddyfile"

scp "$TEMP_DIR/postgres.env" "$REMOTE_SERVER:/tmp/postgres.env"
ssh "$REMOTE_SERVER" "sudo mv /tmp/postgres.env /ozone/postgres.env"
echo "✓ Uploaded postgres.env"

scp "$TEMP_DIR/ozone.env" "$REMOTE_SERVER:/tmp/ozone.env"
ssh "$REMOTE_SERVER" "sudo mv /tmp/ozone.env /ozone/ozone.env"
echo "✓ Uploaded ozone.env"

scp "$TEMP_DIR/CREDENTIALS.txt" "$REMOTE_SERVER:/tmp/CREDENTIALS.txt"
ssh "$REMOTE_SERVER" "sudo mv /tmp/CREDENTIALS.txt /ozone/CREDENTIALS.txt && sudo chmod 600 /ozone/CREDENTIALS.txt"
echo "✓ Uploaded CREDENTIALS.txt"

# Clean up temporary directory
rm -rf "$TEMP_DIR"

# Summary
echo ""
echo "============================================"
echo "Configuration Complete!"
echo "============================================"
echo ""
echo "Files created on remote server:"
echo "  • /ozone/caddy/etc/caddy/Caddyfile"
echo "  • /ozone/postgres.env"
echo "  • /ozone/ozone.env"
echo "  • /ozone/CREDENTIALS.txt"
echo ""
echo "Credentials saved locally at:"
echo "  ./ozone-credentials.txt"
echo ""
echo "To retrieve credentials from server later:"
echo "  ssh $REMOTE_SERVER \"sudo cat /ozone/CREDENTIALS.txt\" > ~/ozone-credentials.txt"
echo ""
echo "Next steps on remote server:"
echo "  1. Download the Docker compose file:"
echo "     curl https://raw.githubusercontent.com/bluesky-social/ozone/main/service/compose.yaml | sudo tee /ozone/compose.yaml"
echo ""
echo "  2. Create and start the systemd service (see documentation)"
echo ""