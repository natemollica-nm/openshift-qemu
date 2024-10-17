#!/usr/bin/env bash

# Script to bootstrap OpenShift IPI cluster

# Step 1: Configure DNS
echo "Configuring DNS..."
bash scripts/configure_dns.sh

# Step 2: Configure Load Balancer (HAProxy)
echo "Setting up HAProxy..."
bash scripts/setup_haproxy.sh

# Step 3: Generate OpenShift Install Manifests
echo "Generating OpenShift manifests..."
bash scripts/generate_manifests.sh

# Step 4: Create Ignition files
echo "Creating Ignition files..."
bash scripts/create_ignition.sh

# Step 5: Generate and Apply Lima VM Configurations
echo "Generating and applying Lima VM configurations..."
bash scripts/generate_lima_configs.sh

# Step 6: Validate DNS configuration
echo "Validating DNS setup..."
bash scripts/validate_dns.sh

echo "Bootstrap process completed!"
