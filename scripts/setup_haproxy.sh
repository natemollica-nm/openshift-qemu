#!/usr/bin/env bash

# Script to configure HAProxy load balancer

# Install HAProxy if not already installed
if ! command -v haproxy &> /dev/null; then
    echo "Installing HAProxy..."
    brew install haproxy
fi

# Copy HAProxy configuration
echo "Configuring HAProxy..."
cp config/haproxy.cfg /opt/homebrew/etc/haproxy.cfg

# Restart HAProxy service
echo "Restarting HAProxy service..."
sudo brew services restart haproxy

echo "HAProxy configuration complete."
