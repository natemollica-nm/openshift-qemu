package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"openshift-qemu/pkg/config"
	"openshift-qemu/pkg/logging"
)

// Constants for OpenShift and RHCOS mirrors
const (
	OCP_MIRROR   string = "https://mirror.openshift.com/pub/openshift-v4/clients/ocp"
	RHCOS_MIRROR string = "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos"
)

// Check runs the OpenShift and RHCOS version checks and downloads
func Check(ocpVersion, rhcosVersion, lbImgURL string, yes bool) config.OpenShiftConfig {
	logging.Title("OPENSHIFT/RHCOS VERSION/URL CHECK")

	cfg := config.OpenShiftConfig{
		OCPVersion:   ocpVersion,
		RHCOSVersion: rhcosVersion,
		LBImageURL:   lbImgURL,
	}

	// Step 1: OpenShift Version CheckDependencies
	cfg.InstallerURL, cfg.ClientURL, cfg.Installer, cfg.Client, cfg.OCPVersion = checkOpenShift(ocpVersion)

	// Step 2: RHCOS Version CheckDependencies
	cfg.Image, cfg.Kernel, cfg.RHCOSKernelURL, cfg.Initramfs, cfg.RHCOSInitramfs = checkRHCOS(rhcosVersion, cfg.OCPVersion)

	// Step 3: Validate CentOS Cloud Image (for Load Balancer)
	validateCentOSImage(cfg.LBImageURL)

	// Ask user if they want to continue, passing the version info
	versionInfo := fmt.Sprintf("\n\nRed Hat OpenShift Version = %s\nRed Hat CoreOS Version = %s\nCentOS Image:%s\n\n", ocpVersion, rhcosVersion, cfg.LBImageURL)
	VerifyContinue(yes, versionInfo)

	return cfg
}

// checkOpenShift checks and returns the OpenShift client and installer URLs
func checkOpenShift(ocpVersion string) (string, string, string, string, string) {
	ocpVer, client, clientURL, installer, installerURL, err := checkOpenShiftVersion(ocpVersion)
	if err != nil {
		logging.Fatal("Failed to obtain OCP URL information", err)
	}
	err = ValidateURL(clientURL)
	if err != nil {
		logging.Fatal("URL Validation failed for OCP client", err)
	}
	err = ValidateURL(installerURL)
	if err != nil {
		logging.Fatal("URL Validation failed for OCP installer", err)
	}

	return installerURL, clientURL, installer, client, ocpVer
}

// checkRHCOS checks and returns the RHCOS kernel, initramfs, and image URLs
func checkRHCOS(rhcosVersion, ocpVer string) (string, string, string, string, string) {
	image, kernel, rhcosKernelURL, initramfs, rhcosInitramfsURL, rhcosImageURL := checkRHCOSVersion(ocpVer, rhcosVersion)
	err := ValidateURL(rhcosKernelURL)
	if err != nil {
		logging.Fatal("URL Validation failed for RHCOS kernel", err)
	}
	err = ValidateURL(rhcosInitramfsURL)
	if err != nil {
		logging.Fatal("URL Validation failed for RHCOS initramfs", err)
	}
	err = ValidateURL(rhcosImageURL)
	if err != nil {
		logging.Fatal("URL Validation failed for RHCOS image", err)
	}

	return image, kernel, rhcosKernelURL, initramfs, rhcosInitramfsURL
}

// validateCentOSImage checks the validity of the CentOS cloud image URL
func validateCentOSImage(lbImgURL string) {
	err := ValidateURL(lbImgURL)
	if err != nil {
		logging.Fatal("URL Validation failed for CentOS image", err)
	}
}

// checkOpenShiftVersion checks and normalizes the OpenShift version, returning the client and installer URLs
func checkOpenShiftVersion(ocpVersion string) (string, string, string, string, string, error) {
	var urldir, installer, client, clientURL, installerURL string

	// Normalize version
	if ocpVersion == "latest" || ocpVersion == "stable" {
		urldir = ocpVersion
	} else {
		parts := strings.Split(ocpVersion, ".")
		if parts[0] != "4" {
			return installer, client, clientURL, installer, installerURL, fmt.Errorf("invalid OpenShift version %s", ocpVersion)
		}
		ocpVer := strings.Join(parts[:2], ".")
		ocpMinor := parts[2]
		if ocpMinor == "" || ocpMinor == "latest" || ocpMinor == "stable" {
			urldir = fmt.Sprintf("%s-%s", ocpMinor, ocpVer)
		} else {
			urldir = fmt.Sprintf("%s.%s", ocpVer, ocpMinor)
		}
	}

	logging.Info(fmt.Sprintf("Looking up OCP4 client for release %s: ", urldir))
	client = findInURL(OCP_MIRROR+"/"+urldir, "client-linux")
	if client == "" {
		return installer, client, clientURL, installer, installerURL, fmt.Errorf("no client found in %s/%s", OCP_MIRROR, urldir)
	}
	clientURL = fmt.Sprintf("%s/%s/%s", OCP_MIRROR, urldir, client)
	logging.Info(client)

	logging.Info(fmt.Sprintf("Looking up OCP4 installer for release %s: ", urldir))
	installer = findInURL(OCP_MIRROR+"/"+urldir, "install-linux")
	if installer == "" {
		return installer, client, clientURL, installer, installerURL, fmt.Errorf("no installer found in %s/%s", OCP_MIRROR, urldir)
	}
	installerURL = fmt.Sprintf("%s/%s/%s", OCP_MIRROR, urldir, installer)
	logging.Info(installer)

	return installer, client, clientURL, installer, installerURL, nil
}

// checkRHCOSVersion checks and normalizes the RHCOS version, returning the kernel, initramfs, and image URLs
func checkRHCOSVersion(ocpVer, rhcosVersion string) (string, string, string, string, string, string) {
	var path string

	// Normalize version
	if rhcosVersion == "" {
		rhcosVersion = ocpVer[:len(ocpVer)-2] // Match RHCOS to OpenShift version
		path = "latest"
	} else {
		parts := strings.Split(rhcosVersion, ".")
		rhcosVer := strings.Join(parts[:2], ".")
		rhcosMinor := parts[2]
		if rhcosMinor == "" || rhcosMinor == "latest" {
			path = "latest"
		} else {
			path = fmt.Sprintf("%s.%s", rhcosVer, rhcosMinor)
		}
	}

	// Kernel
	logging.Info(fmt.Sprintf("Looking up RHCOS kernel for release %s/%s: ", rhcosVersion, path))
	kernel := findInURL(RHCOS_MIRROR+"/"+rhcosVersion+"/"+path, "installer-kernel|live-kernel")
	if kernel == "" {
		logging.Error(fmt.Sprintf("No kernel found in %s/%s", RHCOS_MIRROR, path), nil)
	}
	kernelURL := fmt.Sprintf("%s/%s/%s", RHCOS_MIRROR, rhcosVersion, kernel)
	logging.Info(kernel)

	// Initramfs
	logging.Info(fmt.Sprintf("Looking up RHCOS initramfs for release %s/%s: ", rhcosVersion, path))
	initramfs := findInURL(RHCOS_MIRROR+"/"+rhcosVersion+"/"+path, "installer-initramfs|live-initramfs")
	if initramfs == "" {
		logging.Error(fmt.Sprintf("No initramfs found in %s/%s", RHCOS_MIRROR, path), nil)
	}
	initramfsURL := fmt.Sprintf("%s/%s/%s", RHCOS_MIRROR, rhcosVersion, initramfs)
	logging.Info(initramfs)

	// Image
	logging.Info(fmt.Sprintf("Looking up RHCOS image for release %s/%s: ", rhcosVersion, path))
	image := findInURL(RHCOS_MIRROR+"/"+rhcosVersion+"/"+path, "metal|live-rootfs")
	if image == "" {
		logging.Error(fmt.Sprintf("No image found in %s/%s", RHCOS_MIRROR, path), nil)
	}
	imageURL := fmt.Sprintf("%s/%s/%s", RHCOS_MIRROR, rhcosVersion, image)
	logging.Info(image)

	return image, kernel, kernelURL, initramfs, initramfsURL, imageURL
}

// findInURL searches for a pattern in a given URL
func findInURL(url, pattern string) string {
	resp, err := http.Get(url)
	if err != nil {
		logging.Error(fmt.Sprintf("Failed to fetch URL: %s", url), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Error("Failed to read response body", err)
	}

	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(string(body))
	if len(match) == 0 {
		return ""
	}
	return match[0]
}
