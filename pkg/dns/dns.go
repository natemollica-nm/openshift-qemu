package dns

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"openshift-qemu/pkg/logging"
	"openshift-qemu/pkg/systemd"
)

// DNSConfig holds relevant information for DNS setup and checks.
type DNSConfig struct {
	ClusterName string
	BaseDomain  string
	DNSDir      string
	DNSSvc      string
	LibvirtGwIP string
}

// ReloadDNS reloads the DNS and virtnetworkd services using systemd.
func ReloadDNS(dnsConfig DNSConfig) error {
	dnsService := &systemd.Systemd{Name: dnsConfig.DNSSvc}
	if err := dnsService.Restart(); err != nil {
		return fmt.Errorf("failed to restart DNS service %s: %w", dnsConfig.DNSSvc, err)
	}

	// Wait before restarting virtnetworkd service.
	time.Sleep(5 * time.Second)

	virtService := &systemd.Systemd{Name: "virtnetworkd"}
	if err := virtService.Restart(); err != nil {
		return fmt.Errorf("failed to restart virtnetworkd service: %w", err)
	}

	return nil
}

// Cleanup removes temporary DNS test files and reloads DNS services.
func Cleanup(dnsDir string) error {
	filesToRemove := []string{
		"/etc/hosts.dnstest",
		filepath.Join(dnsDir, "dnstest.conf"),
	}

	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil {
			logging.Warn(fmt.Sprintf("Failed to remove file %s: %v", file, err))
		}
	}

	if err := ReloadDNS(DNSConfig{DNSSvc: "dnsmasq"}); err != nil {
		return fmt.Errorf("failed to reload dnsmasq: %w", err)
	}
	return nil
}

// TestDNS performs the DNS setup, configuration, and testing.
func TestDNS(config DNSConfig) error {
	logging.Title("DNS CHECK")

	// Check if the first nameserver in /etc/resolv.conf points to localhost.
	if err := checkFirstNameserver(); err != nil {
		return fmt.Errorf("DNS test failed: %w", err)
	}

	// Create a test hosts file for dnsmasq.
	if err := createTestHostsFile(config.BaseDomain); err != nil {
		return fmt.Errorf("failed to create hosts file: %w", err)
	}

	// Create a test dnsmasq configuration file.
	if err := createDNSConfigFile(config); err != nil {
		return fmt.Errorf("failed to create dnsmasq config file: %w", err)
	}

	// Reload DNS and libvirt network services.
	if err := ReloadDNS(config); err != nil {
		return fmt.Errorf("failed to reload DNS: %w", err)
	}

	// Perform DNS tests (forward, reverse, wildcard).
	if err := runDNSTests(config); err != nil {
		return fmt.Errorf("DNS tests failed: %w", err)
	}

	// Clean up test files.
	if err := Cleanup(config.DNSDir); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

// checkFirstNameserver ensures the first nameserver in /etc/resolv.conf is localhost.
func checkFirstNameserver() error {
	// Read /etc/resolv.conf
	resolvConf, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("failed to read /etc/resolv.conf: %w", err)
	}

	// Check if the first nameserver is pointing locally (127.x.x.x).
	for _, line := range strings.Split(string(resolvConf), "\n") {
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) > 1 && strings.HasPrefix(fields[1], "127.") {
				logging.Ok()
				return nil
			}
			break
		}
	}

	return fmt.Errorf("first nameserver is not pointing to localhost")
}

// createTestHostsFile creates a test hosts file for DNS testing.
func createTestHostsFile(baseDomain string) error {
	hostsContent := fmt.Sprintf("1.2.3.4 xxxtestxxx.%s\n", baseDomain)
	return os.WriteFile("/etc/hosts.dnstest", []byte(hostsContent), 0o644)
}

// createDNSConfigFile creates a dnsmasq configuration file.
func createDNSConfigFile(config DNSConfig) error {
	dnsConfigContent := fmt.Sprintf(`
local=/%s.%s/
addn-hosts=/etc/hosts.dnstest
address=/test-wild-card.%s.%s/5.6.7.8
`, config.ClusterName, config.BaseDomain, config.ClusterName, config.BaseDomain)

	dnsConfigFile := filepath.Join(config.DNSDir, "dnstest.conf")
	return os.WriteFile(dnsConfigFile, []byte(dnsConfigContent), 0o644)
}

// runDNSTests runs forward, reverse, and wildcard DNS tests.
func runDNSTests(config DNSConfig) error {
	digTargets := []string{config.LibvirtGwIP, ""}

	for _, dnsHost := range digTargets {
		var digDest string
		if dnsHost != "" {
			digDest = fmt.Sprintf("@%s", dnsHost)
		}

		// Forward DNS test
		if err := testDNSLookup(fmt.Sprintf("xxxtestxxx.%s", config.BaseDomain), digDest, "1.2.3.4"); err != nil {
			return err
		}

		// Reverse DNS test
		if err := testReverseDNSLookup("1.2.3.4", digDest, fmt.Sprintf("xxxtestxxx.%s.", config.BaseDomain)); err != nil {
			return err
		}

		// Wildcard DNS test
		if err := testDNSLookup(fmt.Sprintf("blah.test-wild-card.%s.%s", config.ClusterName, config.BaseDomain), digDest, "5.6.7.8"); err != nil {
			return err
		}
	}

	return nil
}

// testDNSLookup performs a forward DNS lookup test.
func testDNSLookup(host, digDest, expected string) error {
	ipRecords, err := net.LookupHost(host)
	if err != nil || len(ipRecords) == 0 || ipRecords[0] != expected {
		return fmt.Errorf("forward DNS lookup failed for host %s via %s", host, digDest)
	}
	logging.Ok()
	return nil
}

// testReverseDNSLookup performs a reverse DNS lookup test.
func testReverseDNSLookup(ip, digDest, expected string) error {
	hosts, err := net.LookupAddr(ip)
	if err != nil || len(hosts) == 0 || hosts[0] != expected {
		return fmt.Errorf("reverse DNS lookup failed for IP %s via %s", ip, digDest)
	}
	logging.Ok()
	return nil
}
