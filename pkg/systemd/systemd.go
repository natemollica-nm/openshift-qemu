package systemd

import (
	"fmt"
	"os/exec"
	"strings"

	"openshift-qemu/pkg/logging"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusFailed   Status = "failed"
)

type Systemd struct {
	Name      string
	Status    Status
	IsEnabled bool
}

// CheckStatus checks if the systemd service is active, inactive, or failed, and if it is enabled
func (s *Systemd) CheckStatus() error {
	// Check if service is active
	active, err := runCommand("systemctl", "is-active", s.Name)
	if err != nil {
		return err
	}

	switch strings.TrimSpace(active) {
	case string(StatusActive):
		s.Status = StatusActive
	case string(StatusInactive):
		s.Status = StatusInactive
	default:
		s.Status = StatusFailed
	}

	// Check if service is enabled
	enabled, err := runCommand("systemctl", "is-enabled", s.Name)
	if err != nil {
		return err
	}
	s.IsEnabled = strings.TrimSpace(enabled) == "enabled"

	return nil
}

// Start starts the systemd service
func (s *Systemd) Start() error {
	if s.Status == StatusActive {
		logging.Info(fmt.Sprintf("%s is already running\n", s.Name))
		return nil
	}
	_, err := runCommand("systemctl", "start", s.Name)
	if err != nil {
		return err
	}
	s.Status = StatusActive
	logging.Info(fmt.Sprintf("%s started successfully\n", s.Name))
	return nil
}

// Stop stops the systemd service
func (s *Systemd) Stop() error {
	if s.Status == StatusInactive {
		logging.Info(fmt.Sprintf("%s is already stopped\n", s.Name))
		return nil
	}
	_, err := runCommand("systemctl", "stop", s.Name)
	if err != nil {
		return err
	}
	s.Status = StatusInactive
	logging.Info(fmt.Sprintf("%s stopped successfully\n", s.Name))
	return nil
}

// Restart restarts the systemd service
func (s *Systemd) Restart() error {
	_, err := runCommand("systemctl", "restart", s.Name)
	if err != nil {
		return err
	}
	s.Status = StatusActive
	logging.Info(fmt.Sprintf("%s restarted successfully\n", s.Name))
	return nil
}

// Reload restarts the systemd service
func (s *Systemd) Reload() error {
	_, err := runCommand("systemctl", "reload", s.Name)
	if err != nil {
		return err
	}
	s.Status = StatusActive
	logging.Info(fmt.Sprintf("%s reloaded successfully\n", s.Name))
	return nil
}

// Enable enables the systemd service to start at boot
func (s *Systemd) Enable() error {
	if s.IsEnabled {
		logging.Info(fmt.Sprintf("%s is already enabled\n", s.Name))
		return nil
	}
	_, err := runCommand("systemctl", "enable", s.Name)
	if err != nil {
		return err
	}
	s.IsEnabled = true
	logging.Info(fmt.Sprintf("%s enabled successfully\n", s.Name))
	return nil
}

// Disable disables the systemd service from starting at boot
func (s *Systemd) Disable() error {
	if !s.IsEnabled {
		logging.Info(fmt.Sprintf("%s is already disabled\n", s.Name))
		return nil
	}
	_, err := runCommand("systemctl", "disable", s.Name)
	if err != nil {
		return err
	}
	s.IsEnabled = false
	logging.Info(fmt.Sprintf("%s disabled successfully\n", s.Name))
	return nil
}

// runCommand executes a command and returns its output
func runCommand(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	return string(out), err
}
