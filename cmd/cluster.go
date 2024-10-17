package cmd

import (
	"github.com/spf13/cobra"
	"openshift-qemu/pkg/cluster"
	"openshift-qemu/pkg/libvirt"
	"openshift-qemu/pkg/logging"
)

// Create the 'create' subcommand
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manages OpenShift cluster lifecycle",
}

// Create the 'create-lb' subcommand to create the load balancer VM
var createLBCmd = &cobra.Command{
	Use:   "create-lb",
	Short: "Create the load balancer VM for the OpenShift cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.Info("Creating Load Balancer VM")

		// Generate HAProxy config
		err := cluster.GenerateHAProxyConfig(clusterName, baseDom, nMasters)
		if err != nil {
			logging.Fatal("Failed to generate HAProxy config", err)
		}

		// Create the Load Balancer VM
		vmDiskPath, err := cluster.ConfigureLBVM(clusterName, sshPubKeyFile)
		if err != nil {
			return err
		}
		logging.Info("Load Balancer VM successfully configured (virt-customize)")
		_, gatewayIP, err := libvirt.EnsureLibvirtNetwork(virNetOct, defLibvirtNet, LibguestfsBackendDirect)
		err = cluster.CreateLBVM(cluster.LBVMParams{
			ClusterName: clusterName,
			CPU:         lbCPU,
			MEM:         lbMem,
			VirNet:      defLibvirtNet,
			VMDiskPath:  vmDiskPath,
			SSHPubKey:   sshPubKeyFile,
			BaseDomain:  baseDom,
		}, dnsDir, dnsSvc, gatewayIP)

		return nil
	},
}

func init() {
	// Add 'create-lb' as a subcommand under 'cluster'
	clusterCmd.AddCommand(createLBCmd)

	// Add the main cluster command to the root command
	rootCmd.AddCommand(clusterCmd)
}
