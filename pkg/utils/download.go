package utils

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"text/template"

	"openshift-qemu/pkg/logging"
)

// OpenShiftTools handles the download and preparation steps for the OpenShift 4 installation
func OpenShiftTools(client, clientURL, installer, installerURL, cacheDir string) error {
	if err := download(client, clientURL, cacheDir, false); err != nil {
		logging.Error("Failed to download OpenShift (oc) client", err)
	}
	err := extractFile(client, cacheDir)
	if err != nil {
		logging.Error("Failed to extract OpenShift (oc) client", err)
		return err
	}
	if err = download(installer, installerURL, cacheDir, false); err != nil {
		logging.Error("Failed to download OpenShift installer", err)
	}
	err = extractFile(installer, cacheDir)
	if err != nil {
		logging.Error("Failed to extract OpenShift installer", err)
		return err
	}
	return nil
}

// DownloadRHCOSFiles Download RHCOS live image, kernel, and initramfs to download cache
func DownloadRHCOSFiles(image, imageURL, kernel, kernelURL, initramfs, initramfsURL, cacheDir string) error {
	if err := download(image, imageURL, cacheDir, false); err != nil {
		logging.Error("Failed to download RHCOS image", err)
		return err
	}
	if err := download(kernel, kernelURL, cacheDir, false); err != nil {
		logging.Error("Failed to download RHCOS kernel", err)
		return err
	}
	if err := download(initramfs, initramfsURL, cacheDir, false); err != nil {
		logging.Error("Failed to download RHCOS initramfs", err)
		return err
	}
	return nil
}

// CreateHostsAndDNSConfig generates the hosts file and dnsmasq config for the cluster
func CreateHostsAndDNSConfig(clusterName, dnsDir string) error {
	// Create hosts file
	hostsFile := fmt.Sprintf("/etc/hosts.%s", clusterName)
	err := touchFile(hostsFile)
	if err != nil {
		return fmt.Errorf("failed to create hosts file: %v", err)
	}

	// Create dnsmasq config
	dnsConfigFile := fmt.Sprintf("%s/%s.conf", dnsDir, clusterName)
	err = writeFile(dnsConfigFile, fmt.Sprintf("addn-hosts=/etc/hosts.%s", clusterName))
	if err != nil {
		return fmt.Errorf("failed to create dnsmasq config file: %v", err)
	}
	return nil
}

// HandleSSHKey handles SSH key generation or reuses an existing key
func HandleSSHKey(sshPubKeyFile string) error {
	// Step 2: Handle SSH key generation or reuse
	logging.Info("SSH key to be injected in all VMs: ")
	if sshPubKeyFile == "" {
		cmd := exec.Command("ssh-keygen", "-f", "sshkey", "-q", "-N", "")
		err := cmd.Run()
		if err != nil {
			logging.Error("Failed to generate SSH key", err)
		}
		sshPubKeyFile = "sshkey.pub"
		logging.Ok("generated new ssh key")
	} else if _, err := os.Stat(sshPubKeyFile); err == nil {
		logging.Ok(fmt.Sprintf("using existing %s", sshPubKeyFile))
	} else {
		logging.Error("Unable to select SSH public key", err)
	}
	return nil
}

type RHCOSTemplateData struct {
	OCPVersion string
}

//go:embed templates/treeinfo.tmpl
var treeinfoTemplate embed.FS

// PrepareRHCOSInstall prepares the RHCOS install files using embedded templating
func PrepareRHCOSInstall(kernel, initramfs, ocpVer string) error {
	logging.Info("Preparing RHCOS installation files")

	// Create directory if not exists
	err := os.Mkdir("rhcos-install", 0o755)
	if err != nil && !os.IsExist(err) {
		logging.Error("Failed to create RHCOS install directory", err)
		return err
	}

	// Copy kernel and initramfs
	err = copyFile(kernel, "rhcos-install/vmlinuz")
	if err != nil {
		logging.Error("Failed to copy kernel", err)
		return err
	}

	err = copyFile(initramfs, "rhcos-install/initramfs.img")
	if err != nil {
		logging.Error("Failed to copy initramfs", err)
		return err
	}

	// Parse the embedded template
	tmpl, err := template.ParseFS(treeinfoTemplate, "templates/treeinfo.tmpl")
	if err != nil {
		logging.Error("Failed to parse embedded treeinfo template", err)
		return err
	}

	// Create the .treeinfo file
	treeinfoFile, err := os.Create("rhcos-install/.treeinfo")
	if err != nil {
		logging.Error("Failed to create .treeinfo file", err)
		return err
	}
	defer treeinfoFile.Close()

	// Execute the template with data
	data := RHCOSTemplateData{OCPVersion: ocpVer}
	err = tmpl.Execute(treeinfoFile, data)
	if err != nil {
		logging.Error("Failed to execute template for .treeinfo", err)
		return err
	}

	logging.Ok("RHCOS installation files prepared successfully")
	return nil
}

//go:embed templates/install-config.yaml.tmpl
var installConfigTemplate embed.FS

// InstallConfig holds the data to be passed into the template for generating install-config.yaml.
type InstallConfig struct {
	ClusterName        string
	ClusterNetworkCIDR string
	NMaster            int
	PullSecret         string
	SSHPublicKey       string
}

// CreateInstallConfig generates the install-config.yaml using an embedded template.
func CreateInstallConfig(setupDir, clusterName string, nMast int, pullSec, sshPubKeyFile string) {
	logging.Info("Creating install-config.yaml: ")

	// Parse the embedded template
	tmpl, err := template.ParseFS(installConfigTemplate, "templates/install-config.yaml.tmpl")
	if err != nil {
		logging.Error("Error parsing template", err)
	}

	// Prepare the data for the template
	data := InstallConfig{
		ClusterName:  clusterName,
		NMaster:      nMast,
		PullSecret:   pullSec,
		SSHPublicKey: readFileContent(sshPubKeyFile),
	}

	// Create install_dir if it doesn't exist
	err = os.MkdirAll("install_dir", 0o755)
	if err != nil {
		logging.Error("Failed to create install_dir", err)
	}

	// Open the output file for writing
	f, err := os.Create("install_dir/install-config.yaml")
	if err != nil {
		logging.Error("Failed to create install-config.yaml", err)
	}
	defer f.Close()

	// Execute the template and write to the file
	err = tmpl.Execute(f, data)
	if err != nil {
		logging.Error("Failed to execute template for install-config.yaml", err)
	}

	logging.Ok()
}

//go:embed templates/tmpws.service.tmpl
var tmpwsTemplate embed.FS

type TmpwsServiceData struct {
	WSPort int
}

// PrepareTmpwsService prepares the tmpws.service file using embedded templating
func PrepareTmpwsService(wsPort int) {
	logging.Info("Creating tmpws.service")

	// Parse the embedded template
	tmpl, err := template.ParseFS(tmpwsTemplate, "templates/tmpws.service.tmpl")
	if err != nil {
		logging.Error("Failed to parse embedded tmpws.service template", err)
		return
	}

	// Create the tmpws.service file
	serviceFile, err := os.Create("tmpws.service")
	if err != nil {
		logging.Error("Failed to create tmpws.service", err)
		return
	}
	defer serviceFile.Close()

	// Execute the template with data
	data := TmpwsServiceData{WSPort: wsPort}
	err = tmpl.Execute(serviceFile, data)
	if err != nil {
		logging.Error("Failed to execute template for tmpws.service", err)
		return
	}

	logging.Ok("tmpws.service created successfully")
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, input, 0o644)
	if err != nil {
		return err
	}

	return nil
}

func readFileContent(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		logging.Error(fmt.Sprintf("Failed to read file: %s", filePath), err)
	}
	return string(data)
}
