#!/usr/bin/env bash

# Script to start the VMs using Lima

# Start the master node
echo "Starting the master VM..."
limactl start config/lima-master.yaml

# Start the worker node
echo "Starting the worker VM..."
limactl start config/lima-worker.yaml

echo "VMs started."
