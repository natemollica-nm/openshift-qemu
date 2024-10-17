package cmd

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"openshift-qemu/pkg/logging"
	"openshift-qemu/pkg/utils"

	"github.com/spf13/cobra"

	"openshift-qemu/pkg/dns"
	"openshift-qemu/pkg/libvirt"
)

// Define default values
var (
	ocpVersion    string
	rhcosVersion  string
	nMasters      int
	nWorkers      int
	masCPU        int
	masMem        int
	worCPU        int
	worMem        int
	btsCPU        int
	btsMem        int
	lbImageURL    string
	lbCPU         int
	lbMem         int
	wsPort        int
	defLibvirtNet string
	virNetOct     string
	clusterName   string
	baseDom       string
	dnsDir        string
	vmDir         string
	setupDir      string
	cacheDir      string
	pullSecFile   string
	sshPubKeyFile string
	autostartVMs  bool
	keepBootstrap bool
	freshDownload bool
	destroy       bool
	yesFlag       bool

	startTS    time.Time
	invocation string
	exeDir     string
)

const (
	LibguestfsBackend       = "LIBGUESTFS_BACKEND"
	LibguestfsBackendDirect = "direct" // "libvirt:qemu:///system"
	dnsSvc                  = "NetworkManager"
)

// Initialize the default values and Cobra flags
func init() {
	rootCmd.PersistentFlags().StringVarP(&ocpVersion, "ocp-version", "O", "4.17", "OpenShift version")
	rootCmd.PersistentFlags().StringVarP(&rhcosVersion, "rhcos-version", "R", "", "RHCOS version")
	rootCmd.PersistentFlags().StringVarP(&lbImageURL, "lb-image", "l", "https://cloud.centos.org/centos/9-stream/x86_64/images/CentOS-Stream-GenericCloud-9.qcow2", "CentOS cloud image URL")
	rootCmd.PersistentFlags().IntVarP(&nMasters, "masters", "m", 3, "Number of master nodes")
	rootCmd.PersistentFlags().IntVarP(&nWorkers, "workers", "w", 2, "Number of worker nodes")
	rootCmd.PersistentFlags().IntVar(&masCPU, "master-cpu", 4, "Number of vCPUs for master nodes")
	rootCmd.PersistentFlags().IntVar(&masMem, "master-mem", 16000, "Memory size for master nodes in MB")
	rootCmd.PersistentFlags().IntVar(&worCPU, "worker-cpu", 2, "Number of vCPUs for worker nodes")
	rootCmd.PersistentFlags().IntVar(&worMem, "worker-mem", 8000, "Memory size for worker nodes in MB")
	rootCmd.PersistentFlags().IntVar(&btsCPU, "bootstrap-cpu", 4, "Number of vCPUs for bootstrap node")
	rootCmd.PersistentFlags().IntVar(&btsMem, "bootstrap-mem", 16000, "Memory size for bootstrap node in MB")
	rootCmd.PersistentFlags().IntVar(&lbCPU, "lb-cpu", 4, "Number of vCPUs for load balancer VM")
	rootCmd.PersistentFlags().IntVar(&lbMem, "lb-mem", 1536, "Memory size for load balancer VM in MB")
	rootCmd.PersistentFlags().IntVar(&wsPort, "ws-port", 1234, "Web server port for load balancer VM")
	rootCmd.PersistentFlags().StringVarP(&defLibvirtNet, "libvirt-network", "n", "default", "Libvirt network")
	rootCmd.PersistentFlags().StringVarP(&virNetOct, "libvirt-oct", "N", "100", "Libvirt network octet")
	rootCmd.PersistentFlags().StringVarP(&clusterName, "cluster-name", "c", "ocp4", "Cluster name")
	rootCmd.PersistentFlags().StringVarP(&baseDom, "cluster-domain", "d", "local", "Cluster domain")
	rootCmd.PersistentFlags().StringVarP(&dnsDir, "dns-dir", "z", "/etc/NetworkManager/dnsmasq.d", "DNS configuration directory")
	rootCmd.PersistentFlags().StringVarP(&vmDir, "vm-dir", "v", "/var/lib/libvirt/images", "VM directory")
	rootCmd.PersistentFlags().StringVarP(&setupDir, "setup-dir", "s", "", "Setup directory")
	rootCmd.PersistentFlags().StringVarP(&cacheDir, "cache-dir", "x", "/root/ocp4_downloads", "Cache directory")
	rootCmd.PersistentFlags().StringVarP(&pullSecFile, "pull-secret", "p", "/root/pull-secret", "Path to pull secret file")
	rootCmd.PersistentFlags().StringVar(&sshPubKeyFile, "ssh-pub-key-file", "", "Path to SSH public key file")
	rootCmd.PersistentFlags().BoolVar(&autostartVMs, "autostart-vms", false, "Autostart VMs after creation")
	rootCmd.PersistentFlags().BoolVar(&keepBootstrap, "keep-bootstrap", false, "Keep the bootstrap VM after installation")
	rootCmd.PersistentFlags().BoolVar(&freshDownload, "fresh-download", false, "Force fresh download of OCP and RHCOS images")
	rootCmd.PersistentFlags().BoolVar(&destroy, "destroy", false, "Destroy the cluster")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Automatically approve all prompts")
	startTS = time.Now()                                       // Equivalent to START_TS
	invocation = fmt.Sprintf("%s %v", os.Args[0], os.Args[1:]) // Equivalent to SINV
	exeDir, _ = os.Getwd()                                     // Equivalent to SDIR (current directory)

	// Set LIBGUESTFS_BACKEND
	err := os.Setenv(LibguestfsBackend, LibguestfsBackendDirect)
	if err != nil {
		logging.Fatal("Failed to set LIBGUESTFS_BACKEND environment variable:", err)
	}
	logging.Ok(fmt.Sprintf("LIBGUESTFS_BACKEND=%s", os.Getenv(LibguestfsBackend)))

	// Required Parameter Validation
	if nMasters <= 0 {
		logging.Fatal("Invalid number of masters: ", fmt.Errorf("masters must be > 0"))
	}
	if nWorkers < 0 {
		logging.Fatal("Invalid number of workers: ", fmt.Errorf("workers cannot be < 0"))
	}
	if masMem < 0 {
		logging.Fatal("Invalid value for --master-mem: %d", fmt.Errorf("%d", masMem))
	}
	if masCPU < 0 {
		logging.Fatal("Invalid value for --master-cpu: %d", fmt.Errorf("%d", masCPU))
	}
	if worMem < 0 {
		logging.Fatal("Invalid value for --worker-mem: %d", fmt.Errorf("%d", worMem))
	}
	if worCPU < 0 {
		logging.Fatal("Invalid value for --worker-cpu: %d", fmt.Errorf("%d", worCPU))
	}
	if btsMem < 0 {
		logging.Fatal("Invalid value for --bootstrap-mem: %d", fmt.Errorf("%d", btsMem))
	}
	if btsCPU < 0 {
		logging.Fatal("Invalid value for --bootstrap-cpu: %d", fmt.Errorf("%d", btsCPU))
	}
	if lbMem < 0 {
		logging.Fatal("Invalid value for --lb-mem: %d", fmt.Errorf("%d", lbMem))
	}
	if lbCPU < 0 {
		logging.Fatal("Invalid value for --lb-cpu: %d", fmt.Errorf("%d", lbCPU))
	}
	netOct, err := strconv.Atoi(virNetOct)
	if err != nil {
		logging.Fatal("Failed to convert --lib-virt-oct to string for validation", fmt.Errorf("value=%s | err=%v", virNetOct, err))
	}
	if netOct < 0 || netOct > 255 {
		logging.Fatal("Invalid value for --lib-virt-oct", fmt.Errorf("value=%s", virNetOct))
	}
	if _, err = os.Stat(pullSecFile); err != nil {
		logging.Fatal(fmt.Sprintf("Pull secret file not found: %s", pullSecFile), err)
	}
	if sshPubKeyFile != "" {
		if _, err = os.Stat(sshPubKeyFile); err != nil {
			logging.Fatal(fmt.Sprintf("SSH Public key file not found: %s", pullSecFile), err)
		}
	}
}

// checkIfRoot checks if the current user is root
func checkIfRoot() {
	currentUser, err := user.Current()
	if err != nil {
		logging.Error("Error fetching user information:", err)
		os.Exit(1)
	}
	if currentUser.Uid != "0" {
		logging.Error("Error: Not running as root", fmt.Errorf("current user UID: %s", currentUser.Uid))
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "openshift-qemu",
	Short: "CLI tool to set up OpenShift 4 on KVM via libvirt",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// If no arguments, show help
			cmd.Help()
			return
		}
		logging.InfoMessage("Starting OpenShift 4 UPI KVM Setup", map[string]interface{}{
			"Time":              startTS,
			"Invocation":        invocation,
			"Working Directory": exeDir,
		})
		checkIfRoot() // Checking if user is root

		// Processing VM directory
		absVMDir, err := filepath.Abs(vmDir)
		if err != nil {
			logging.Fatal("failed to resolve VM directory, %v", err)
		}
		logging.Info(fmt.Sprintf("%s VM Directory: %s", clusterName, absVMDir))

		// Conditional logic based on flags
		if defLibvirtNet != "" && virNetOct != "" {
			logging.Fatal("invalid parameter (mutually-exclusive):", fmt.Errorf("specify either --libvirt-network (-n) or --libvirt-oct (-N), not both"))
		}
		if defLibvirtNet == "" && virNetOct == "" {
			defLibvirtNet = "default"
		}
		// Pre-flight Checks
		utils.CheckDependencies(setupDir, pullSecFile, dnsDir, clusterName, baseDom, LibguestfsBackendDirect)

		logging.Title("OPENSHIFT SETUP INITIALIZATION")
		// Print some values to ensure everything is processed
		logging.InfoMessage("Cluster Information:", map[string]interface{}{
			"OpenShift version":      ocpVersion,
			"Number of master nodes": nMasters,
			"Number of worker nodes": nWorkers,
			"Cluster name":           clusterName,
		})

		// Step 1: Ensure libvirt network setup
		logging.Step("Setting up Libvirt Network...")
		bridgeName, gatewayIP, err := libvirt.EnsureLibvirtNetwork(virNetOct, defLibvirtNet, LibguestfsBackendDirect)
		if err != nil {
			log.Fatalf("Failed to set up libvirt network: %v", err)
		}
		// Proceed with the rest of the setup
		logging.Info(fmt.Sprintf("Libvirt bridge: %s, Gateway IP: %s", bridgeName, gatewayIP))

		// Step 2: Run DNS checks
		logging.Step("Step 2: Running DNS Checks...")
		err = dns.TestDNS(dns.DNSConfig{
			ClusterName: clusterName,
			BaseDomain:  baseDom,
			DNSDir:      dnsDir,
			DNSSvc:      dnsSvc,
			LibvirtGwIP: gatewayIP,
		})
		if err != nil {
			log.Fatalf("Failed to run DNS checks: %v", err)
		}
	},
}

func Execute() {
	// Initialize the logger with the default log level
	logging.InitLogger(logrus.InfoLevel)

	// Set the log level if specified through a command-line flag (optional)
	if logLevel, err := rootCmd.Flags().GetString("log-level"); err == nil {
		logging.SetLogLevel(logLevel)
	}

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		logging.Error("failed command run", err)
		os.Exit(1) // Ensures we exit with a failure code if the command fails
	}
}
