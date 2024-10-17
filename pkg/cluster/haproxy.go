package cluster

import (
	"embed"
	"fmt"
	"os"
	"text/template"

	"openshift-qemu/pkg/dns"
	"openshift-qemu/pkg/libvirt"
)

//go:embed templates/haproxy.cfg.tmpl
var haproxyTemplate embed.FS

// HAProxyConfig holds the data to be passed into the template.
type HAProxyConfig struct {
	ClusterName string
	BaseDomain  string
	MasterNodes []string
}

// GenerateHAProxyConfig generates the haproxy.cfg using a template.
func GenerateHAProxyConfig(clusterName, baseDomain string, nMast int) error {
	masterNodes := make([]string, nMast)
	for i := 1; i <= nMast; i++ {
		masterNodes[i-1] = fmt.Sprintf("master-%d.%s.%s", i, clusterName, baseDomain)
	}

	data := HAProxyConfig{
		ClusterName: clusterName,
		BaseDomain:  baseDomain,
		MasterNodes: masterNodes,
	}

	return executeTemplate("haproxy.cfg", data)
}

// executeTemplate is a helper function to parse and execute templates
func executeTemplate(outputPath string, data interface{}) error {
	tmpl, err := template.ParseFS(haproxyTemplate, "templates/haproxy.cfg.tmpl")
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// LBVMParams holds the parameters for creating the load balancer VM.
type LBVMParams struct {
	ClusterName       string
	CPU               int
	MEM               int
	VirNet            string
	VMDiskPath        string
	SSHPubKey         string
	BaseDomain        string
	LibguestfsBackend string
}

// ConfigureLBVM customizes and configures the load balancer VM.
func ConfigureLBVM(clusterName, sshPubKey string) (string, error) {
	vmDiskPath := fmt.Sprintf("/var/lib/libvirt/images/%s-lb.qcow2", clusterName)
	params := libvirt.VirtCustomizeParams{
		ImagePath:      vmDiskPath,
		SSHPubKeyFile:  sshPubKey,
		Packages:       []string{"haproxy", "bind-utils"},
		Uninstall:      []string{"cloud-init"},
		CopyInFiles:    []string{"haproxy.cfg:/etc/haproxy", "bootstrap.ign:/opt/"},
		RunCommands:    []string{"systemctl daemon-reload", "systemctl enable haproxy"},
		RelabelSELinux: true,
	}

	if err := libvirt.VirtCustomize(params); err != nil {
		return "", fmt.Errorf("failed to customize VM: %v", err)
	}

	return vmDiskPath, nil
}

// CreateLBVM creates, starts, and configures networking for the Load Balancer VM.
func CreateLBVM(params LBVMParams, dnsDir, dnsSvc, gatewayIP string) error {
	conn, err := libvirt.NewLibvirtConnection(params.LibguestfsBackend)
	if err != nil {
		return fmt.Errorf("failed to connect to libvirt: %v", err)
	}
	defer conn.Close()

	if err = createAndStartLBVM(conn, params); err != nil {
		return err
	}

	lbIP, lbMAC, err := waitForVMIP(conn, params.ClusterName)
	if err != nil {
		return err
	}

	if err = libvirt.AddDHCPReservation(conn, params.VirNet, lbMAC, lbIP); err != nil {
		return fmt.Errorf("failed to add DHCP reservation: %v", err)
	}

	if err = updateClusterDNS(lbIP, params.ClusterName, params.BaseDomain); err != nil {
		return err
	}

	if err = dns.ReloadDNS(dns.DNSConfig{
		ClusterName: params.ClusterName,
		BaseDomain:  params.BaseDomain,
		DNSDir:      dnsDir,
		DNSSvc:      dnsSvc,
		LibvirtGwIP: gatewayIP,
	}); err != nil {
		return fmt.Errorf("failed to restart DNS service: %v", err)
	}

	return libvirt.WaitForSSHAccess(lbIP, fmt.Sprintf("lb.%s.%s", params.ClusterName, params.BaseDomain), "sshkey", "root")
}

// createAndStartLBVM handles the VM creation and startup.
func createAndStartLBVM(conn libvirt.VirtConnection, params LBVMParams) error {
	vmParams := libvirt.VMParams{
		Name:      fmt.Sprintf("%s-lb", params.ClusterName),
		Memory:    uint(params.MEM),
		CPUs:      uint(params.CPU),
		DiskPath:  params.VMDiskPath,
		OSVariant: osVariant,
		Network:   params.VirNet,
	}

	if err := libvirt.CreateVM(conn, vmParams); err != nil {
		return fmt.Errorf("failed to create load balancer VM: %v", err)
	}

	if err := libvirt.StartVM(conn, vmParams.Name); err != nil {
		return fmt.Errorf("failed to start load balancer VM: %v", err)
	}

	return nil
}

// updateClusterDNS adds the IP and hostname to the appropriate /etc/hosts file.
func updateClusterDNS(ip, clusterName, baseDomain string) error {
	filePath := fmt.Sprintf("/etc/hosts.%s", clusterName)
	entry := fmt.Sprintf("%s lb.%s.%s api.%s.%s api-int.%s.%s", ip, clusterName, baseDomain, clusterName, baseDomain, clusterName, baseDomain)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open hosts file: %v", err)
	}
	defer f.Close()

	if _, err = f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write to hosts file: %v", err)
	}
	return nil
}
