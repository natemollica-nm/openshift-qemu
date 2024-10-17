package libvirt

import (
	"fmt"
	"strings"

	"libvirt.org/libvirt-go"
	"openshift-qemu/pkg/logging"
)

// EnsureLibvirtNetwork checks if the network exists or creates a new one based on the given parameters.
func EnsureLibvirtNetwork(virNetOct, virNet string, libguestfsBackendDirect string) (string, string, error) {
	conn, err := NewLibvirtConnection(libguestfsBackendDirect)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to libvirt: %v", err)
	}
	defer conn.Close()

	// Check if the network exists based on the virNetOct or virNet.
	if virNetOct != "" {
		networkName := fmt.Sprintf("ocp-%s", virNetOct)
		network, err := conn.LookupNetworkByName(networkName)
		if err == nil {
			defer network.Free()
			logging.Info(fmt.Sprintf("Libvirt network %s already exists, reusing it.\n", networkName))
			virNet = networkName
		} else {
			logging.Info(fmt.Sprintf("Creating libvirt network %s...\n", networkName))
			if err := createNewLibvirtNetwork(conn, networkName, virNetOct); err != nil {
				return "", "", err
			}
			virNet = networkName
		}
	} else if virNet != "" {
		network, err := conn.LookupNetworkByName(virNet)
		if err != nil {
			return "", "", fmt.Errorf("libvirt network %s doesn't exist: %v", virNet, err)
		}
		defer network.Free()
		logging.Info(fmt.Sprintf("Using existing libvirt network: %s\n", virNet))
	} else {
		return "", "", fmt.Errorf("unhandled situation: either virNetOct or virNet must be provided")
	}

	// Get bridge and gateway IP information
	bridgeName, err := getLibvirtBridge(conn, virNet)
	if err != nil {
		return "", "", err
	}

	gatewayIP, err := getLibvirtNetworkGatewayIP(conn, virNet)
	if err != nil {
		return bridgeName, "", err
	}

	return bridgeName, gatewayIP, nil
}

// createNewLibvirtNetwork defines, autostarts, and starts a new libvirt network
func createNewLibvirtNetwork(conn *libvirt.Connect, networkName, virNetOct string) error {
	networkXML := fmt.Sprintf(`
<network>
  <name>%s</name>
  <bridge name="%s"/>
  <forward/>
  <ip address="192.168.%s.1" netmask="255.255.255.0">
    <dhcp>
      <range start="192.168.%s.2" end="192.168.%s.254"/>
    </dhcp>
  </ip>
</network>`, networkName, networkName, virNetOct, virNetOct, virNetOct)

	network, err := conn.NetworkDefineXML(networkXML)
	if err != nil {
		return fmt.Errorf("failed to define network: %v", err)
	}
	defer network.Free()

	if err := network.SetAutostart(true); err != nil {
		return fmt.Errorf("failed to set autostart: %v", err)
	}
	if err := network.Create(); err != nil {
		return fmt.Errorf("failed to start network: %v", err)
	}

	logging.Info(fmt.Sprintf("Libvirt network %s created and started successfully.\n", networkName))
	return nil
}

// getLibvirtBridge fetches the bridge name for the given libvirt network
func getLibvirtBridge(conn *libvirt.Connect, networkName string) (string, error) {
	network, err := conn.LookupNetworkByName(networkName)
	if err != nil {
		return "", fmt.Errorf("failed to lookup network %s: %v", networkName, err)
	}
	defer network.Free()

	bridgeName, err := network.GetBridgeName()
	if err != nil {
		return "", fmt.Errorf("failed to get bridge name for network %s: %v", networkName, err)
	}
	return bridgeName, nil
}

// getLibvirtNetworkGatewayIP retrieves the gateway IP for a given network
func getLibvirtNetworkGatewayIP(conn *libvirt.Connect, networkName string) (string, error) {
	network, err := conn.LookupNetworkByName(networkName)
	if err != nil {
		return "", fmt.Errorf("failed to lookup network %s: %v", networkName, err)
	}
	defer network.Free()

	xmlDesc, err := network.GetXMLDesc(0)
	if err != nil {
		return "", fmt.Errorf("failed to get network XML description for %s: %v", networkName, err)
	}

	// Parse XML to find the IP address
	ipAddrStart := strings.Index(xmlDesc, "<ip address=")
	if ipAddrStart == -1 {
		return "", fmt.Errorf("IP address not found in network XML for %s", networkName)
	}

	// Extract the IP address from the XML
	ipAddrStart += len(`<ip address="`)
	ipAddrEnd := strings.Index(xmlDesc[ipAddrStart:], `"`)
	if ipAddrEnd == -1 {
		return "", fmt.Errorf("malformed XML while parsing IP address for %s", networkName)
	}

	return xmlDesc[ipAddrStart : ipAddrStart+ipAddrEnd], nil
}
