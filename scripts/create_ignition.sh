#!/usr/bin/env bash

# Script to create Ignition files for OpenShift IPI installation

# Check if the openshift-install command is available
if ! command -v openshift-install &> /dev/null; then
    echo "Error: openshift-install command not found."
    echo "Please install the OpenShift installer before running this script."
    exit 1
fi

# Directory where the OpenShift manifests are stored
INSTALL_DIR="./config/openshift-install"

# Ensure the manifest directory exists before proceeding
if [ ! -d "$INSTALL_DIR" ]; then
    echo "Error: Manifests not found. Please generate the manifests first."
    exit 1
fi

# Create the Ignition files
echo "Running openshift-install to create Ignition files..."
openshift-install create ignition-configs --dir="$INSTALL_DIR"

# Move the Ignition files to the 'ignition' directory
IGNITION_DIR="./ignition"
mkdir -p "$IGNITION_DIR"
mv "$INSTALL_DIR"/*.ign "$IGNITION_DIR/"

echo "Ignition files created successfully and moved to $IGNITION_DIR."
