package libvirt

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"libvirt.org/libvirt-go"
	"openshift-qemu/pkg/logging"
)

type VM struct {
	Name   string
	Status string
}
type VirtConnection *libvirt.Connect

// NewLibvirtConnection initializes a new libvirt connection
func NewLibvirtConnection(uri string) (*libvirt.Connect, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %v", err)
	}
	return conn, nil
}

// GetVMsByName lists all VMs (domains) that match a given name
func GetVMsByName(conn *libvirt.Connect, clusterName string) ([]VM, error) {
	var vms []VM

	// List all VMs (domains)
	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_PERSISTENT)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %v", err)
	}

	// Filter VMs by the cluster name
	for _, domain := range domains {
		var name string
		name, err = domain.GetName()
		if err != nil {
			return nil, fmt.Errorf("failed to get domain name: %v", err)
		}
		if err = domain.Free(); err != nil {
			return nil, fmt.Errorf("failed to free domain: %v", err)
		}

		if nameContainsCluster(name, clusterName) {
			vms = append(vms, VM{Name: name})
		}
	}
	return vms, nil
}

// Helper function to check if a VM name contains the cluster name
func nameContainsCluster(name, clusterName string) bool {
	return name == clusterName || name == fmt.Sprintf("%s-bootstrap", clusterName) || name == fmt.Sprintf("%s-master", clusterName)
}

type VMParams struct {
	Name      string
	Memory    uint
	CPUs      uint
	DiskPath  string
	OSVariant string
	Location  string
	ExtraArgs string
	Network   string
}

// CreateVM creates a new VM based on the provided parameters
func CreateVM(conn *libvirt.Connect, params VMParams) error {
	// Updated domain XML with additional features and metadata
	domainXML := fmt.Sprintf(`
<domain type='kvm'>
  <name>%s</name>
  <metadata>
    <libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0">
      <libosinfo:os id="http://redhat.com/rhel/9.0"/>
    </libosinfo:libosinfo>
  </metadata>
  <memory unit='MiB'>%d</memory>
  <vcpu placement='static'>%d</vcpu>
  <cpu mode='host-passthrough'>
    <model fallback='allow'/>
  </cpu>
  <os>
    <type arch='x86_64' machine='pc-q35-rhel9.4.0'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <interface type='network'>
      <source network='%s'/>
      <model type='virtio'/>
    </interface>
    <graphics type='vnc' autoport='yes'/>
  </devices>
</domain>`, params.Name, params.Memory, params.CPUs, params.DiskPath, params.Network)

	// Create and define the domain with the updated XML
	domain, err := conn.DomainCreateXML(domainXML, 0)
	if err != nil {
		return fmt.Errorf("failed to create domain: %v", err)
	}
	defer domain.Free()

	fmt.Printf("VM %s created successfully.\n", params.Name)
	return nil
}

// StartVM starts a VM by name
func StartVM(conn *libvirt.Connect, vmName string) error {
	dom, err := conn.LookupDomainByName(vmName)
	if err != nil {
		return fmt.Errorf("failed to find VM %s: %v", vmName, err)
	}
	defer dom.Free()

	err = dom.Create()
	if err != nil {
		return fmt.Errorf("failed to start VM %s: %v", vmName, err)
	}
	return nil
}

// StopVM stops a VM by name
func StopVM(conn *libvirt.Connect, vmName string) error {
	dom, err := conn.LookupDomainByName(vmName)
	if err != nil {
		return fmt.Errorf("failed to find VM %s: %v", vmName, err)
	}
	defer dom.Free()

	err = dom.Destroy()
	if err != nil {
		return fmt.Errorf("failed to stop VM %s: %v", vmName, err)
	}
	return nil
}

// DestroyVM destroys a VM by name
func DestroyVM(conn *libvirt.Connect, vmName string) error {
	dom, err := conn.LookupDomainByName(vmName)
	if err != nil {
		return fmt.Errorf("failed to find VM %s: %v", vmName, err)
	}
	defer dom.Free()

	err = dom.Undefine()
	if err != nil {
		return fmt.Errorf("failed to destroy VM %s: %v", vmName, err)
	}
	return nil
}

// GetVMIP retrieves the IP address and MAC address of a VM by querying its network interfaces.
func GetVMIP(conn *libvirt.Connect, vmName string) (string, string, error) {
	// Lookup the domain (VM) by its name
	dom, err := conn.LookupDomainByName(vmName)
	if err != nil {
		fmt.Printf("Failed to find VM %s: %v\n", vmName, err)
		return "", "", err
	}
	defer dom.Free()

	// Get domain interfaces (network information)
	ifaces, err := dom.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
	if err != nil {
		fmt.Printf("Failed to list network interfaces for VM %s: %v\n", vmName, err)
		return "", "", err
	}

	// Loop through interfaces to find the IP and MAC addresses
	for _, iface := range ifaces {
		if iface.Hwaddr != "" {
			for _, addr := range iface.Addrs {
				if addr.Type == libvirt.IP_ADDR_TYPE_IPV4 && strings.Contains(addr.Addr, ".") {
					return addr.Addr, iface.Hwaddr, nil
				}
			}
		}
	}
	return "", "", nil
}

// AddDHCPReservation adds a DHCP reservation for a VM by specifying its MAC address and IP address
func AddDHCPReservation(conn *libvirt.Connect, networkName string, macAddress string, ipAddress string) error {
	// Find the network by its name
	network, err := conn.LookupNetworkByName(networkName)
	if err != nil {
		return fmt.Errorf("failed to find network %s: %v", networkName, err)
	}
	defer network.Free()

	// DHCP host XML to be added
	dhcpHostXML := fmt.Sprintf("<host mac='%s' ip='%s'/>", macAddress, ipAddress)

	// Add the DHCP reservation to the network
	err = network.Update(
		libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST,
		libvirt.NETWORK_SECTION_IP_DHCP_HOST,
		-1,
		dhcpHostXML,
		libvirt.NETWORK_UPDATE_AFFECT_LIVE|libvirt.NETWORK_UPDATE_AFFECT_CONFIG,
	)
	if err != nil {
		return fmt.Errorf("failed to add DHCP reservation for MAC %s and IP %s: %v", macAddress, ipAddress, err)
	}

	fmt.Printf("Successfully added DHCP reservation: MAC=%s, IP=%s\n", macAddress, ipAddress)
	return nil
}

// WaitForSSHAccess continuously checks if SSH access to the specified VM is available
func WaitForSSHAccess(vmIP, host, sshKeyPath, sshUser string) error {
	// Use ssh-keygen to remove any previous host key for the VM
	err := removeOldHostKey(vmIP)
	if err != nil {
		return err
	}
	err = removeOldHostKey(host)
	if err != nil {
		return err
	}

	// Loop to wait for SSH access to become available
	for {
		time.Sleep(5 * time.Second)
		logging.Info(fmt.Sprintf("Trying to establish SSH connection to %s (%s)", host, vmIP))

		cmd := exec.Command("ssh", "-i", sshKeyPath, "-o", "StrictHostKeyChecking=no", fmt.Sprintf("%s@%s", sshUser, vmIP), "true")
		err := cmd.Run()
		if err == nil {
			logging.Info(fmt.Sprintf("SSH access to %s established", vmIP))
			return nil
		}

		logging.Info("SSH access not available yet, retrying...")
	}
}

// removeOldHostKey removes an old SSH host key for the given host/IP from known_hosts
func removeOldHostKey(host string) error {
	logging.Info(fmt.Sprintf("Removing old SSH host key for %s", host))
	err := exec.Command("ssh-keygen", "-R", host).Run()
	if err != nil {
		return err
	}
	return nil
}

// VirtCustomizeParams holds the parameters needed for customizing the VM
type VirtCustomizeParams struct {
	ImagePath      string   // The VM disk image to customize
	SSHPubKeyFile  string   // SSH public key to inject
	Packages       []string // List of packages to install
	Uninstall      []string // List of packages to uninstall
	CopyInFiles    []string // List of files to copy-in with destination paths (format: "source:destination")
	RunCommands    []string // List of commands to run inside the VM
	RelabelSELinux bool     // Whether to run --selinux-relabel
}

// VirtCustomize customizes a VM disk image with the given options using virt-customize
func VirtCustomize(params VirtCustomizeParams) error {
	logging.Info(fmt.Sprintf("Customizing VM image at %s", params.ImagePath))

	// Base virt-customize command
	args := []string{"-a", params.ImagePath}

	// SSH key injection
	args = append(args, "--ssh-inject", fmt.Sprintf("root:file:%s", params.SSHPubKeyFile))

	// Install packages
	if len(params.Packages) > 0 {
		args = append(args, "--install", joinPackages(params.Packages))
	}

	// Uninstall packages
	if len(params.Uninstall) > 0 {
		args = append(args, "--uninstall", joinPackages(params.Uninstall))
	}

	// Copy-in files
	for _, file := range params.CopyInFiles {
		args = append(args, "--copy-in", file)
	}

	// SELinux relabeling (optional based on bool flag)
	if params.RelabelSELinux {
		args = append(args, "--selinux-relabel")
	}

	// Run commands
	for _, cmd := range params.RunCommands {
		args = append(args, "--run-command", cmd)
	}

	// Execute virt-customize command
	cmd := exec.Command("virt-customize", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("virt-customize failed: %v\nOutput: %s", err, string(output))
	}

	logging.Ok("VM customization completed successfully")
	return nil
}

// joinPackages combines a list of packages into a single comma-separated string for --install option
func joinPackages(packages []string) string {
	return fmt.Sprintf("%s", stringJoin(packages, ","))
}

// stringJoin joins a string array with a delimiter
func stringJoin(arr []string, delimiter string) string {
	return fmt.Sprintf("%s", fmt.Sprintf(strings.Join(arr, delimiter)))
}
