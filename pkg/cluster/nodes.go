package cluster

import (
	"fmt"
	"os"
	"time"

	"openshift-qemu/pkg/libvirt"
	"openshift-qemu/pkg/logging"
)

// NodeParams holds the configuration for creating bootstrap, master, and worker nodes.
type NodeParams struct {
	ClusterName       string
	BaseDomain        string
	VMDir             string
	LBIP              string
	WSPort            int
	Image             string
	VirNet            string
	BtsMem            int
	BtsCPU            int
	MasMem            int
	MasCPU            int
	WorMem            int
	WorCPU            int
	NMaster           int
	NWorker           int
	RHCOSArg          string
	LibguestfsBackend string
}

// CreateNodes handles the creation of bootstrap, master, and worker nodes using libvirt.
func CreateNodes(params NodeParams) error {
	logging.Info("Creating Bootstrap, Master, and Worker nodes...")

	conn, err := libvirt.NewLibvirtConnection(params.LibguestfsBackend)
	if err != nil {
		logging.Fatal("Failed to connect to libvirt", err)
		return err
	}
	defer conn.Close()

	// Create the Bootstrap VM
	err = createBootstrapNode(conn, params)
	if err != nil {
		logging.Fatal("Failed to create bootstrap node", err)
		return err
	}

	// Create the Master VMs
	err = createMasterNodes(conn, params)
	if err != nil {
		logging.Fatal("Failed to create master nodes", err)
		return err
	}

	// Create the Worker VMs
	err = createWorkerNodes(conn, params)
	if err != nil {
		logging.Fatal("Failed to create worker nodes", err)
		return err
	}

	// Start the VMs and wait for IPs
	err = waitForVMIPs(conn, params)
	if err != nil {
		return err
	}
	bootstrapIP, _, err := libvirt.GetVMIP(conn, fmt.Sprintf("%s-bootstrap", params.ClusterName))
	return libvirt.WaitForSSHAccess(bootstrapIP, fmt.Sprintf("bootstrap.%s.%s", params.ClusterName, params.BaseDomain), "sshkey", "core")
}

// createBootstrapNode creates the bootstrap node VM.
func createBootstrapNode(conn libvirt.VirtConnection, params NodeParams) error {
	logging.Info("Creating Bootstrap VM")

	bootstrapParams := libvirt.VMParams{
		Name:      fmt.Sprintf("%s-bootstrap", params.ClusterName),
		Memory:    uint(params.BtsMem),
		CPUs:      uint(params.BtsCPU),
		DiskPath:  fmt.Sprintf("%s/%s-bootstrap.qcow2", params.VMDir, params.ClusterName),
		OSVariant: osVariant,
		Location:  "rhcos-install/",
		ExtraArgs: fmt.Sprintf("nomodeset rd.neednet=1 coreos.inst=yes coreos.inst.install_dev=vda %s=http://%s:%d/%s coreos.inst.ignition_url=http://%s:%d/bootstrap.ign", params.RHCOSArg, params.LBIP, params.WSPort, params.Image, params.LBIP, params.WSPort),
		Network:   params.VirNet,
	}

	return libvirt.CreateVM(conn, bootstrapParams)
}

// createMasterNodes creates the master node VMs.
func createMasterNodes(conn libvirt.VirtConnection, params NodeParams) error {
	for i := 1; i <= params.NMaster; i++ {
		masterName := fmt.Sprintf("%s-master-%d", params.ClusterName, i)
		logging.Info(fmt.Sprintf("Creating Master-%d VM", i))

		masterParams := libvirt.VMParams{
			Name:      masterName,
			Memory:    uint(params.MasMem),
			CPUs:      uint(params.MasCPU),
			DiskPath:  fmt.Sprintf("%s/%s-master-%d.qcow2", params.VMDir, params.ClusterName, i),
			OSVariant: osVariant,
			Location:  "rhcos-install/",
			ExtraArgs: fmt.Sprintf("nomodeset rd.neednet=1 coreos.inst=yes coreos.inst.install_dev=vda %s=http://%s:%d/%s coreos.inst.ignition_url=http://%s:%d/master.ign", params.RHCOSArg, params.LBIP, params.WSPort, params.Image, params.LBIP, params.WSPort),
			Network:   params.VirNet,
		}

		err := libvirt.CreateVM(conn, masterParams)
		if err != nil {
			return err
		}
	}
	return nil
}

// createWorkerNodes creates the worker node VMs.
func createWorkerNodes(conn libvirt.VirtConnection, params NodeParams) error {
	for i := 1; i <= params.NWorker; i++ {
		workerName := fmt.Sprintf("%s-worker-%d", params.ClusterName, i)
		logging.Info(fmt.Sprintf("Creating Worker-%d VM", i))

		workerParams := libvirt.VMParams{
			Name:      workerName,
			Memory:    uint(params.WorMem),
			CPUs:      uint(params.WorCPU),
			DiskPath:  fmt.Sprintf("%s/%s-worker-%d.qcow2", params.VMDir, params.ClusterName, i),
			OSVariant: osVariant,
			Location:  "rhcos-install/",
			ExtraArgs: fmt.Sprintf("nomodeset rd.neednet=1 coreos.inst=yes coreos.inst.install_dev=vda %s=http://%s:%d/%s coreos.inst.ignition_url=http://%s:%d/worker.ign", params.RHCOSArg, params.LBIP, params.WSPort, params.Image, params.LBIP, params.WSPort),
			Network:   params.VirNet,
		}

		err := libvirt.CreateVM(conn, workerParams)
		if err != nil {
			return err
		}
	}
	return nil
}

// waitForVMIPs waits for VMs to obtain IP addresses and configures DHCP reservations.
func waitForVMIPs(conn libvirt.VirtConnection, params NodeParams) error {
	logging.Info("Waiting for VMs to obtain IP addresses")

	roles := []string{"bootstrap", "master", "worker"}
	for _, role := range roles {
		for i := 1; i <= getRoleCount(params, role); i++ {
			vmName := fmt.Sprintf("%s-%s-%d", params.ClusterName, role, i)
			ip, mac, err := waitForVMIP(conn, vmName)
			if err != nil {
				logging.Fatal(fmt.Sprintf("Failed to get IP for %s", vmName), err)
			}
			if err = libvirt.AddDHCPReservation(conn, ip, mac, params.VirNet); err != nil {
				return err
			}
			updateHostDNS(params, ip, vmName)
		}
	}
	return nil
}

// getRoleCount returns the number of VMs for a given role (bootstrap, master, or worker).
func getRoleCount(params NodeParams, role string) int {
	switch role {
	case "bootstrap":
		return 1
	case "master":
		return params.NMaster
	case "worker":
		return params.NWorker
	default:
		return 0
	}
}

// waitForVMIP waits for a VM to obtain an IP address.
func waitForVMIP(conn libvirt.VirtConnection, vmName string) (string, string, error) {
	var ip, mac string
	for {
		var err error
		time.Sleep(5 * time.Second)
		ip, mac, err = libvirt.GetVMIP(conn, vmName) // Retrieves the IP and MAC using libvirt API
		if err == nil && ip != "" && mac != "" {
			logging.Info(fmt.Sprintf("Obtained IP: %s for VM: %s", ip, vmName))
			return ip, mac, nil
		}
		return "", "", err
	}
}

// updateHostDNS adds a /etc/hosts entry for the VM.
func updateHostDNS(params NodeParams, ip, vmName string) {
	hostsEntry := fmt.Sprintf("%s %s.%s.%s", ip, vmName, params.ClusterName, params.BaseDomain)
	err := os.WriteFile(fmt.Sprintf("/etc/hosts.%s", params.ClusterName), []byte(hostsEntry), 0o644)
	if err != nil {
		logging.Fatal(fmt.Sprintf("Failed to add hosts entry for %s", vmName), err)
	}
}
