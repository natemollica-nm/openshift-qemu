#!/usr/bin/env bash

# Script to generate/update and apply Lima YAML configs for OpenShift nodes

# Directory for Lima configurations
LIMA_CONFIG_DIR=$(readlink -f config)

# Ignition files directory
IGNITION_DIR=$(readlink -f ignition)

# Generate or update Lima config for master node
echo "Generating Lima config for master node..."
cat > "${LIMA_CONFIG_DIR}/lima-master.yaml" <<EOF
arch: "aarch64"
cpus: 4
memory: "8192MB"
disk: "40GB"
images:
  - location: "./rhcos-4.17.0-aarch64-qemu.aarch64.qcow2"
networks:
  - lima: bridged
    interface: "eth0"
    macaddress: "52:54:00:1e:56:ac"
qemu:
  args:
    - "-fw_cfg"
    - "name=opt/com.coreos/config,file=${IGNITION_DIR}/master.ign"
ssh:
  localPort: 60023
EOF

# Generate or update Lima config for bootstrap node
echo "Generating Lima config for bootstrap node..."
cat > "${LIMA_CONFIG_DIR}/lima-bootstrap.yaml" <<EOF
arch: "aarch64"
cpus: 4
memory: "8192MB"
disk: "40GB"
images:
  - location: "./rhcos-4.17.0-aarch64-qemu.aarch64.qcow2"
networks:
  - lima: bridged
    interface: "eth0"
    macaddress: "52:54:00:1e:56:ad"
qemu:
  args:
    - "-fw_cfg"
    - "name=opt/com.coreos/config,file=${IGNITION_DIR}/bootstrap.ign"
ssh:
  localPort: 60024
EOF

# Generate or update Lima config for worker node
echo "Generating Lima config for worker node..."
cat > "${LIMA_CONFIG_DIR}/lima-worker.yaml" <<EOF
arch: "aarch64"
cpus: 4
memory: "8192MB"
disk: "40GB"
images:
  - location: "./rhcos-4.17.0-aarch64-qemu.aarch64.qcow2"
networks:
  - lima: bridged
    interface: "eth0"
    macaddress: "52:54:00:1e:56:ae"
qemu:
  args:
    - "-fw_cfg"
    - "name=opt/com.coreos/config,file=${IGNITION_DIR}/worker.ign"
ssh:
  localPort: 60025
EOF

# Apply Lima configurations
echo "Starting the master VM..."
limactl start "${LIMA_CONFIG_DIR}/lima-master.yaml"

echo "Starting the bootstrap VM..."
limactl start "${LIMA_CONFIG_DIR}/lima-bootstrap.yaml"

echo "Starting the worker VM..."
limactl start "${LIMA_CONFIG_DIR}/lima-worker.yaml"

echo "Lima VMs started successfully."
