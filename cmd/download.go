package cmd

import (
	"fmt"
	"path/filepath"

	"openshift-qemu/pkg/logging"
	"openshift-qemu/pkg/utils"

	"github.com/spf13/cobra"
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download and prepare OpenShift 4 installation",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Access flags defined in root.go
		setupDir, _ := cmd.Flags().GetString("setup-dir")
		cacheDir, _ := cmd.Flags().GetString("cache-dir")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		dnsDir, _ := cmd.Flags().GetString("dns-dir")
		sshPubKeyFile, _ := cmd.Flags().GetString("ssh-pub-key-file")
		// pullSecFile, _ := cmd.Flags().GetString("pull-secret")
		logging.Title("DOWNLOAD AND PREPARE OPENSHIFT 4 INSTALLATION")
		logging.Info("Starting the download and preparation process...")

		// Version checks (OpenShift and RHCOS)
		logging.Step("Step 3: Running OpenShift and RHCOS Version Checks...")
		cfg := utils.Check(ocpVersion, rhcosVersion, lbImageURL, yesFlag)

		// Step 1: Create and navigate to setup directory
		logging.Info(fmt.Sprintf("Creating and using directory %s", setupDir))
		err := utils.CreateDirectory(setupDir)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to create or use directory %s", setupDir), err)
			return err
		}

		// Step 2: Create hosts file for the cluster
		hostsFile := filepath.Join("/etc", "hosts."+clusterName)
		logging.Info(fmt.Sprintf("Creating a hosts file for this cluster: %s", hostsFile))
		err = utils.CreateHostsAndDNSConfig(clusterName, dnsDir)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to configure host DNS %s", hostsFile), err)
			return err
		}

		// Step 3: Check and use SSH public key
		logging.Info("Checking SSH public key...")
		err = utils.HandleSSHKey(sshPubKeyFile)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to check SSH public key %s", sshPubKeyFile), err)
			return err
		}

		// Step 4: Download OCP and RHCOS images
		logging.Info("Downloading OpenShift Client and Installer...")
		err = utils.OpenShiftTools(cfg.Client, cfg.ClientURL, cfg.Installer, cfg.InstallerURL, cacheDir)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to download OpenShift Client/Installer %s/%s", cfg.Client, cfg.Installer), err)
			return err
		}

		// Step 5: Download RHCOS images and prepare installation files
		logging.Info("Downloading RHCOS images...")
		if err := utils.DownloadRHCOSFiles(cfg.Image, cfg.ImageURL, cfg.Kernel, cfg.RHCOSKernelURL, cfg.Initramfs, cfg.InitramfsURL, cacheDir); err != nil {
			return err
		}
		if err := utils.PrepareRHCOSInstall(cfg.Kernel, cfg.Initramfs, cfg.OCPVersion); err != nil {
			return err
		}

		// Further steps for preparation...
		logging.Ok("Download and preparation process completed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
