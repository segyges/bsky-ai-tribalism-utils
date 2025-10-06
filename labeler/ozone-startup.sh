#!/bin/bash

set -e

echo "============================================"
echo "Ozone Deployment Script"
echo "============================================"
echo ""
echo "This script will download the Docker compose file"
echo "and set up the systemd service for Ozone."
echo ""

# Check if we're running on the server
if [ ! -d /ozone ]; then
    echo "Error: /ozone directory not found."
    echo "This script should be run on the Ozone server after running the setup script."
    exit 1
fi

# Check if config files exist
if [ ! -f /ozone/ozone.env ] || [ ! -f /ozone/postgres.env ]; then
    echo "Error: Configuration files not found."
    echo "Please run the setup script first to generate configuration files."
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed."
    echo "Please install Docker first (see documentation)."
    exit 1
fi

# Check if compose.yaml already exists
if [ -f /ozone/compose.yaml ]; then
    echo "⚠️  WARNING: /ozone/compose.yaml already exists"
    echo ""
    read -p "Do you want to re-download it? (y/N): " REDOWNLOAD
    if [[ "$REDOWNLOAD" =~ ^[Yy]$ ]]; then
        sudo rm /ozone/compose.yaml
        echo "✓ Removed existing compose.yaml"
        echo ""
    else
        echo "Using existing compose.yaml"
        echo ""
    fi
fi

# Download compose.yaml if it doesn't exist
if [ ! -f /ozone/compose.yaml ]; then
    echo "Downloading Docker compose file..."
    curl -fsSL https://raw.githubusercontent.com/bluesky-social/ozone/main/service/compose.yaml | sudo tee /ozone/compose.yaml > /dev/null
    echo "✓ Downloaded compose.yaml"
    echo ""
fi

# Check if systemd service already exists
if [ -f /etc/systemd/system/ozone.service ]; then
    echo "⚠️  WARNING: Systemd service already exists"
    echo ""
    read -p "Do you want to recreate it? (y/N): " RECREATE
    if [[ "$RECREATE" =~ ^[Yy]$ ]]; then
        echo "Stopping existing service..."
        sudo systemctl stop ozone 2>/dev/null || true
        sudo systemctl disable ozone 2>/dev/null || true
        sudo rm /etc/systemd/system/ozone.service
        echo "✓ Removed existing service"
        echo ""
    else
        echo "Using existing systemd service"
        echo ""
    fi
fi

# Create systemd service if it doesn't exist
if [ ! -f /etc/systemd/system/ozone.service ]; then
    echo "Creating systemd service..."
    cat <<SYSTEMD_UNIT_FILE | sudo tee /etc/systemd/system/ozone.service > /dev/null
[Unit]
Description=Bluesky Ozone Service
Documentation=https://github.com/bluesky-social/ozone
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/ozone
ExecStart=/usr/bin/docker compose --file /ozone/compose.yaml up --detach
ExecStop=/usr/bin/docker compose --file /ozone/compose.yaml down

[Install]
WantedBy=default.target
SYSTEMD_UNIT_FILE
    echo "✓ Created systemd service"
    echo ""
fi

# Reload systemd daemon
echo "Reloading systemd daemon..."
sudo systemctl daemon-reload
echo "✓ Systemd daemon reloaded"
echo ""

# Enable service
echo "Enabling Ozone service..."
sudo systemctl enable ozone
echo "✓ Service enabled (will start on boot)"
echo ""

# Ask before starting
read -p "Do you want to start the Ozone service now? (Y/n): " START_NOW
if [[ ! "$START_NOW" =~ ^[Nn]$ ]]; then
    echo ""
    echo "Starting Ozone service..."
    sudo systemctl start ozone
    echo "✓ Service started"
    echo ""
    
    # Wait a moment for containers to start
    echo "Waiting for containers to start..."
    sleep 5
    
    # Check status
    echo ""
    echo "Service status:"
    sudo systemctl status ozone --no-pager || true
    echo ""
    
    echo "Docker containers:"
    sudo docker ps
    echo ""
else
    echo ""
    echo "Service not started. You can start it later with:"
    echo "  sudo systemctl start ozone"
    echo ""
fi

# Get the hostname from ozone.env
OZONE_PUBLIC_URL=$(sudo grep OZONE_PUBLIC_URL /ozone/ozone.env | cut -d= -f2)

# Summary
echo "============================================"
echo "Deployment Complete!"
echo "============================================"
echo ""
echo "Files created:"
echo "  • /ozone/compose.yaml"
echo "  • /etc/systemd/system/ozone.service"
echo ""
echo "Useful commands:"
echo "  sudo systemctl status ozone    # Check service status"
echo "  sudo systemctl start ozone     # Start service"
echo "  sudo systemctl stop ozone      # Stop service"
echo "  sudo systemctl restart ozone   # Restart service"
echo "  sudo docker ps                 # View running containers"
echo "  sudo docker logs ozone-ozone-1 # View Ozone logs"
echo ""
echo "Next steps:"
echo "  1. Verify Ozone is online:"
echo "     curl ${OZONE_PUBLIC_URL}/xrpc/_health"
echo ""
echo "  2. Test WebSocket connection:"
echo "     wsdump \"${OZONE_PUBLIC_URL/https/wss}/xrpc/com.atproto.label.subscribeLabels?cursor=0\""
echo ""
echo "  3. Access the Ozone UI at:"
echo "     ${OZONE_PUBLIC_URL}"
echo ""
echo "  4. Login and announce your service to the network (see documentation)"
echo ""