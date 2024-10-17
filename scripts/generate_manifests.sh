#!/usr/bin/env bash

# Script to generate OpenShift manifests

# Check if the openshift-install command is available
if ! command -v openshift-install &> /dev/null; then
    echo "Error: openshift-install command not found."
    echo "Please install the OpenShift installer before running this script."
    exit 1
fi

# Directory to store the OpenShift installation files
INSTALL_DIR=config/openshift-install

# Create the directory if it doesn't exist
test -d "${INSTALL_DIR}" || {
    mkdir -p "${INSTALL_DIR}"
}

if ! test -f "${INSTALL_DIR}"/install-config.yaml; then
    cp config/install-config.yaml "${INSTALL_DIR}"/install-config.yaml
fi

# Generate the manifests
echo "Running openshift-install to create manifests..."
openshift-install create manifests --dir="${INSTALL_DIR}"

echo "Manifests generated successfully in ${INSTALL_DIR}."

# Optional: Modify manifests if needed
# You can include custom modifications to the manifests here if required
