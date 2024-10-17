#!/usr/bin/env bash

# Script to validate DNS resolution

# Test DNS resolution for API, master, and worker nodes
echo "Validating DNS resolution..."

echo "Testing API DNS resolution..."
dig api.openshift-local.com

echo "Testing master node DNS resolution..."
dig master-0.openshift-local.com

echo "Testing worker node DNS resolution..."
dig worker-0.openshift-local.com

echo "Testing Ingress wildcard DNS resolution..."
dig apps.openshift-local.com

echo "DNS validation complete."
