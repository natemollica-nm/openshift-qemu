#!/usr/bin/env bash

# Script to configure dnsmasq for DNS resolution on macOS

# Install dnsmasq if not already installed
if ! command -v dnsmasq &> /dev/null; then
    echo "Installing dnsmasq..."
    brew install dnsmasq
fi

# Configure dnsmasq
echo "Configuring dnsmasq..."
cp config/dnsmasq.conf /opt/homebrew/etc/dnsmasq.d/dnsmasq-ocp.conf

# Restart dnsmasq service
echo "Restarting dnsmasq service..."
sudo brew services restart dnsmasq

# List all network services and prompt user to select one
echo "Available network services:"
networksetup -listallnetworkservices

read -p "Enter the network service to configure DNS (e.g., Wi-Fi, Ethernet): " selected_service

# Check if the selected service exists
if ! networksetup -listallnetworkservices | grep -q "$selected_service"; then
    echo "Invalid network service selected. Exiting."
    exit 1
fi

# Desired DNS servers: 127.0.0.1 and 8.8.8.8 and 8.8.4.4
desired_dns="127.0.0.1 8.8.8.8 8.8.4.4"

# Get the current DNS servers for the selected service
current_dns=$(networksetup -getdnsservers "$selected_service")

# Check if 127.0.0.1, 8.8.8.8, and 8.8.4.4 are already set
if [[ "$current_dns" == "There aren't any DNS Servers set on $selected_service." ]] || [[ "$current_dns" != *"127.0.0.1"* ]] || [[ "$current_dns" != *"8.8.8.8"* ]] || [[ "$current_dns" != *"8.8.4.4"* ]]; then
    echo "Updating DNS settings for the '$selected_service' interface..."
    sudo networksetup -setdnsservers "$selected_service" "$desired_dns"
else
    echo "$desired_dns are already set for the '$selected_service' interface."
fi

# Optional: Check if dnsmasq is correctly set
echo "Validating DNS resolution with dnsmasq..."
dig @127.0.0.1 api.openshift-lima.openshift-local.com +short
dig @127.0.0.1 api-int.openshift-lima.openshift-local.com +short
dig @127.0.0.1 \*.apps.openshift-lima.openshift-local.com +short
dig @127.0.0.1 bootstrap.openshift-lima.openshift-local.com +short
dig @127.0.0.1 master-0.openshift-lima.openshift-local.com +short
dig @127.0.0.1 worker-0.openshift-lima.openshift-local.com +short

echo "DNS configuration complete."
