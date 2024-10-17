package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"openshift-qemu/pkg/systemd"

	"openshift-qemu/pkg/libvirt"
	"openshift-qemu/pkg/logging"
)

// libvirt module daemons
const (
	qemu     = "qemu"
	virtint  = "interface"
	network  = "network"
	nodedev  = "nodedev"
	nwfilter = "nwfilter"
	secret   = "secret"
	storage  = "storage"
)

// Linux Packages
const (
	virsh                = "virsh"
	virtInstall          = "virt-install"
	virtCustomize        = "virt-customize"
	systemctl            = "systemctl"
	dig                  = "dig"
	wget                 = "wget"
	libvirtNetworkDriver = "libvirt_driver_network.so"
)

type Dependencies struct {
	Executables []string
	Drivers     []string
	Files       []string
	Directories []string
}

// CheckDependencies if a command exists on the system
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// Run a system command and return the output
func runCommand(cmd string, args ...string) (string, error) {
	output, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		logging.Error(fmt.Sprintf("%s", output), err)
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// CheckDependencies performs all dependency and environment checks
func CheckDependencies(setupDir, pullSecFile, dnsDir, clusterName, baseDom, libguestfsBackendDirect string) {
	logging.Title("DEPENDENCIES & SANITY CHECKS")
	commandRunDeps := Dependencies{
		Executables: []string{virsh, virtInstall, virtCustomize, systemctl, dig, wget},
		Drivers:     []string{libvirtNetworkDriver},
		Files:       []string{pullSecFile},
		Directories: []string{setupDir},
	}
	commandRunDeps.checkExecutables()
	commandRunDeps.checkLibvirtNetworkDriver()
	commandRunDeps.checkSetupDirectory()
	commandRunDeps.checkFile()
	checkVirtDaemons()
	checkExistingVMs(clusterName, libguestfsBackendDirect)
	checkDNSService(dnsDir)
	checkConflictingDNSRecords(clusterName, baseDom)
}

// checkExecutables verifies that all required dependencies are installed
func (c *Dependencies) checkExecutables() {
	logging.Info("Checking if we have all the dependencies:")
	for _, dep := range c.Executables {
		if !commandExists(dep) {
			logging.Fatal(fmt.Sprintf("Executable '%s' not found and is required! Please fix missing dependency...", dep), nil)
		}
	}
	logging.Ok("Dependencies found")
}

// checkLibvirtNetworkDriver verifies the presence of the libvirt network driver
func (c *Dependencies) checkLibvirtNetworkDriver() {
	for _, driver := range c.Drivers {
		if _, err := filepath.Glob(fmt.Sprintf("/usr/**/%s", driver)); err != nil {
			logging.Fatal(fmt.Sprintf("%s not found", driver), err)
		}
		logging.Ok("libvirt_driver_network.so found")
	}
}

// checkSetupDirectory verifies if the setup directory already exists
func (c *Dependencies) checkSetupDirectory() {
	for _, dir := range c.Directories {
		logging.Info(fmt.Sprintf("Checking if the %s directory already exists:", dir))
		if _, err := os.Stat(dir); err == nil {
			logging.Fatal(fmt.Sprintf("Directory %s already exists\n"+
				"You can use --destroy to remove your existing installation\n"+
				"You can also use --setup-dir to specify a different directory for this installation", dir), err)
		}
		logging.Ok()
	}
}

// checkPullSecret verifies the existence of the pull secret file and prints part of its content
func (c *Dependencies) checkFile() {
	for _, file := range c.Files {
		logging.Info(fmt.Sprintf("Checking for file (%s):", file))
		if _, err := os.Stat(file); err == nil {
			// Simulate the export behavior by reading the file
			var content []byte
			content, err = os.ReadFile(file)
			if err != nil {
				logging.Fatal("Error reading file", err)
			}
			logging.Info(fmt.Sprintf("File found: %s ...", string(content[:50]))) // Show a small part
		} else {
			logging.Fatal("Pull secret not found! Please specify the pull secret file using -p or --pull-secret", err)
		}
		logging.Ok()
	}
}

// checkVirtDaemons ensures that all necessary virt daemons are running or enabled
func checkVirtDaemons() {
	virtDrivers := []string{qemu, virtint, network, nodedev, nwfilter, secret, storage}
	for _, drv := range virtDrivers {
		service := systemd.Systemd{Name: "virt" + drv + "d"}
		err := service.CheckStatus()
		if err != nil {
			logging.Fatal(fmt.Sprintf("Failed to check status of %s", service.Name), err)
		}

		// Start the service if it's not active
		if service.Status != systemd.StatusActive {
			err = service.Start()
			if err != nil {
				logging.Fatal(fmt.Sprintf("Failed to start %s", service.Name), err)
			}
		}
		logging.Ok(fmt.Sprintf("%s is active", service.Name))
	}
}

// checkExistingVMs checks if there are existing VMs with the given cluster name
func checkExistingVMs(clusterName, libguestfsBackendDirect string) {
	logging.Info("Checking if we have any existing leftover VMs:")

	// Use libvirt.NewLibvirtConnection from the libvirt package
	conn, err := libvirt.NewLibvirtConnection(libguestfsBackendDirect)
	if err != nil {
		logging.Fatal("Failed to connect to libvirt", err)
	}
	defer conn.Close()

	// Get VMs by cluster name
	vms, err := libvirt.GetVMsByName(conn, clusterName)
	if err != nil {
		logging.Fatal("Failed to list VMs", err)
	}

	if len(vms) > 0 {
		logging.Fatal(fmt.Sprintf("Found existing VM(s): %v", vms), nil)
	}
	logging.Ok("No leftover VMs found")
}

// checkDNSService verifies the DNS service (dnsmasq or NetworkManager) is active and reloads it
func checkDNSService(dnsDir string) {
	logging.Info("Checking if DNS service (dnsmasq or NetworkManager) is active:")
	if _, err := os.Stat("/etc/NetworkManager/dnsmasq.d"); os.IsNotExist(err) {
		if _, err = os.Stat("/etc/dnsmasq.d"); os.IsNotExist(err) {
			logging.Fatal("No dnsmasq found", err)
		}
	}

	dnsSvc := determineDNSSvc(dnsDir)
	err := reloadDNSService(dnsSvc)
	if err != nil {
		logging.Fatal("Failed to reload DNS service", err)
	}

	// NetworkManager-specific check
	if dnsSvc == "NetworkManager" {
		err = checkNetworkManagerDnsmasq()
		if err != nil {
			logging.Fatal("Failed to check DNS service network manager", err)
		}
	}
}

// determineDNSSvc determines which DNS service is being used based on the directory
func determineDNSSvc(dnsDir string) string {
	if dnsDir == "/etc/NetworkManager/dnsmasq.d" {
		return "NetworkManager"
	}
	return "dnsmasq"
}

// reloadDNSService reloads the DNS service (NetworkManager or dnsmasq)
func reloadDNSService(dnsSvc string) error {
	dnsCmd := "restart"
	if dnsSvc == "NetworkManager" {
		dnsCmd = "reload"
	}
	service := systemd.Systemd{Name: "NetworkManager"}
	logging.Info(fmt.Sprintf("Testing dnsmasq %s (systemctl %s %s):", dnsCmd, dnsCmd, dnsSvc))
	switch dnsCmd {
	case "restart":
		err := service.Restart()
		if err != nil {
			return err
		}
	case "reload":
		err := service.Reload()
		if err != nil {
			return err
		}
	}
	logging.Ok()
	return nil
}

// checkNetworkManagerDnsmasq verifies if dnsmasq is enabled in NetworkManager
func checkNetworkManagerDnsmasq() error {
	logging.Info("Checking if dnsmasq is enabled in NetworkManager")
	err := filepath.Walk("/etc/NetworkManager/", func(path string, info os.FileInfo, err error) error {
		if err == nil && filepath.Ext(path) == ".conf" {
			file, _ := os.Open(path)
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if match, _ := regexp.MatchString(`^(?!#).*dnsmasq`, line); match {
					fmt.Println(line)
				}
			}
		}
		return nil
	})
	if err != nil {
		logging.Error("DNS Directory is set to NetworkManager but dnsmasq is not enabled in NetworkManager", fmt.Errorf("see: https://github.com/kxr/ocp4_setup_upi_kvm/wiki/Setting-Up-DNS"))
		return err
	}
	logging.Ok()
	return nil
}

// checkConflictingDNSRecords checks for leftover/conflicting DNS records
func checkConflictingDNSRecords(clusterName, baseDom string) {
	logging.Info("Checking for any leftover/conflicting DNS records:")
	hosts := []string{"api", "api-int", "bootstrap", "master-1", "master-2", "master-3", "etcd-0", "etcd-1", "etcd-2", "worker-1", "worker-2", "test.apps"}
	for _, host := range hosts {
		var res string
		res, err := runCommand("dig", "+short", fmt.Sprintf("%s.%s.%s", host, clusterName, baseDom), "@127.0.0.1")
		if err != nil || res != "" {
			logging.Fatal(fmt.Sprintf("Found existing DNS record for %s.%s.%s: %s", host, clusterName, baseDom, res), err)
		}
	}

	// CheckDependencies /etc/hosts for conflicts
	existingHosts, err := runCommand("grep", "-v", "^#", "/etc/hosts")
	if err == nil && strings.Contains(existingHosts, clusterName+"."+baseDom) {
		logging.Fatal(fmt.Sprintf("Found existing /etc/hosts records: %s", existingHosts), err)
	}
	logging.Ok()
}
