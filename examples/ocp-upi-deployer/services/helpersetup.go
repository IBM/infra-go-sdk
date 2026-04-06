package services

import (
	"fmt"
	"strings"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

const httpdConfigContent = `# OpenShift UPI Deployer - HTTP Server Configuration
# Listen on port 8080 to avoid conflict with HAProxy on port 80
Listen 8080

<VirtualHost *:8080>
    DocumentRoot /var/www/html
    <Directory /var/www/html>
        Options Indexes FollowSymLinks
        AllowOverride None
        Require all granted
    </Directory>
</VirtualHost>
`

// HelperNodeSetup manages helper node configuration and package installation
type HelperNodeSetup struct {
	sshClient        *communication.SSHClient
	verbose          bool
	requiredPackages []string
}

// NewHelperNodeSetup creates a new helper node setup manager
func NewHelperNodeSetup(sshClient *communication.SSHClient, verbose bool, requiredPackages []string) *HelperNodeSetup {
	// Use provided packages or default list
	packages := requiredPackages
	if len(packages) == 0 {
		packages = []string{
			"dnsmasq",
			"haproxy",
			"httpd",
			"firewalld",
		}
	}

	return &HelperNodeSetup{
		sshClient:        sshClient,
		verbose:          verbose,
		requiredPackages: packages,
	}
}

// InstallPackages installs required packages on the helper node
func (h *HelperNodeSetup) InstallPackages() error {
	fmt.Println("Checking and installing required packages...")

	packages := h.requiredPackages

	// Check installed packages
	fmt.Println("Checking installed packages...")
	checkCmd := fmt.Sprintf("rpm -q %s 2>&1", strings.Join(packages, " "))
	output, err := h.sshClient.ExecuteCommand(checkCmd)

	// Parse output to find missing packages
	missingPackages := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Catch both variations of rpm not installed messages
		if strings.Contains(line, "not installed") || strings.Contains(line, "is not installed") {
			for _, pkg := range packages {
				if strings.Contains(line, pkg) {
					missingPackages = append(missingPackages, pkg)
					break
				}
			}
		}
	}

	// If rpm -q failed entirely and we couldn't parse, assume all need installation
	if err != nil && len(missingPackages) == 0 {
		missingPackages = packages
	}

	if len(missingPackages) == 0 {
		fmt.Println("✓ All required packages are already installed")
		return nil
	}

	fmt.Printf("Installing missing packages: %s (this may take a few minutes)...\n", strings.Join(missingPackages, ", "))
	installCmd := fmt.Sprintf("sudo dnf install -y %s", strings.Join(missingPackages, " "))

	if h.verbose {
		fmt.Printf("Executing: %s\n", installCmd)
	}

	output, err = h.sshClient.ExecuteCommand(installCmd)
	if err != nil {
		return fmt.Errorf("failed to install packages: %w\nOutput: %s", err, output)
	}

	fmt.Println("✓ Required packages installed successfully")
	return nil
}

// ConfigureFirewall sets up firewall rules for OpenShift services
func (h *HelperNodeSetup) ConfigureFirewall() error {
	fmt.Println("Configuring firewall...")

	// 1. Enable and start firewalld first to ensure it's active
	if _, err := h.sshClient.ExecuteCommand("sudo systemctl enable --now firewalld"); err != nil {
		return fmt.Errorf("failed to start firewalld: %w", err)
	}

	// 2. Add Services (firewall-cmd allows multiple --add-service flags)
	services := []string{"dns", "dhcp", "http", "https", "tftp"}
	svcArgs := ""
	for _, svc := range services {
		svcArgs += fmt.Sprintf(" --add-service=%s", svc)
	}

	svcCmd := "sudo firewall-cmd --permanent" + svcArgs
	if h.verbose {
		fmt.Printf("Executing: %s\n", svcCmd)
	}
	if _, err := h.sshClient.ExecuteCommand(svcCmd); err != nil {
		return fmt.Errorf("failed to add firewall services: %w", err)
	}

	// 3. Add Ports (must be in a separate command from --add-service)
	ports := []string{"6443/tcp", "22623/tcp", "8080/tcp"}
	portArgs := ""
	for _, port := range ports {
		portArgs += fmt.Sprintf(" --add-port=%s", port)
	}

	portCmd := "sudo firewall-cmd --permanent" + portArgs
	if h.verbose {
		fmt.Printf("Executing: %s\n", portCmd)
	}
	if _, err := h.sshClient.ExecuteCommand(portCmd); err != nil {
		return fmt.Errorf("failed to add firewall ports: %w", err)
	}

	// 4. Reload to apply permanent rules
	if _, err := h.sshClient.ExecuteCommand("sudo firewall-cmd --reload"); err != nil {
		return fmt.Errorf("failed to reload firewall: %w", err)
	}

	fmt.Println("✓ Firewall configured successfully")
	return nil
}

// DisableSELinux disables SELinux (required for some OpenShift services)
func (h *HelperNodeSetup) DisableSELinux() error {
	fmt.Println("Checking SELinux status...")

	output, err := h.sshClient.ExecuteCommand("getenforce")
	if err != nil {
		return fmt.Errorf("failed to check SELinux status: %w", err)
	}

	status := strings.TrimSpace(output)
	if status == "Disabled" {
		fmt.Println("✓ SELinux is already disabled")
		return nil
	}

	fmt.Println("Disabling SELinux...")

	// Set SELinux to permissive mode immediately
	if _, err := h.sshClient.ExecuteCommand("sudo setenforce 0"); err != nil {
		return fmt.Errorf("failed to set SELinux to permissive: %w", err)
	}

	// Disable SELinux permanently. Using '.*' ensures we catch it whether it is currently 'enforcing' or 'permissive'
	sedCmd := "sudo sed -i 's/^SELINUX=.*/SELINUX=disabled/' /etc/selinux/config"
	if _, err := h.sshClient.ExecuteCommand(sedCmd); err != nil {
		return fmt.Errorf("failed to disable SELinux permanently: %w", err)
	}

	fmt.Println("✓ SELinux disabled (reboot required for permanent effect)")
	return nil
}

// SetupDirectories creates required base directories on the helper node
func (h *HelperNodeSetup) SetupDirectories() error {
	fmt.Println("Setting up required directories...")

	directories := []string{
		"/var/www/html/",
		"/etc/dnsmasq.d",
		"/etc/haproxy",
		"/etc/haproxy/conf.d",
	}

	existingDirs := []string{}
	missingDirs := []string{}

	for _, dir := range directories {
		checkCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", dir)
		output, err := h.sshClient.ExecuteCommand(checkCmd)
		if err != nil {
			missingDirs = append(missingDirs, dir)
			continue
		}

		if strings.TrimSpace(output) == "exists" {
			existingDirs = append(existingDirs, dir)
		} else {
			missingDirs = append(missingDirs, dir)
		}
	}

	if len(existingDirs) > 0 {
		fmt.Printf("  ℹ Found existing directories (shared with other clusters):\n")
		for _, dir := range existingDirs {
			fmt.Printf("    - %s\n", dir)
		}
	}

	if len(missingDirs) > 0 {
		fmt.Printf("  Creating missing directories:\n")
		for _, dir := range missingDirs {
			fmt.Printf("    - %s\n", dir)
		}

		cmd := fmt.Sprintf("sudo mkdir -p %s", strings.Join(missingDirs, " "))
		if _, err := h.sshClient.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}
	}

	fmt.Println("✓ Directory setup complete")
	return nil
}

// ConfigureHTTPD configures Apache httpd to listen on port 8080
func (h *HelperNodeSetup) ConfigureHTTPD() error {
	fmt.Println("Configuring httpd to listen on port 8080...")

	// Write configuration file safely using a heredoc to prevent shell expansion/escaping issues
	cmd := fmt.Sprintf("sudo tee /etc/httpd/conf.d/ocp-upi-deployer.conf > /dev/null << 'EOF'\n%s\nEOF", httpdConfigContent)
	if _, err := h.sshClient.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to write httpd configuration: %w", err)
	}

	// Comment out default Listen 80 in httpd.conf if it exists
	cmd = "sudo sed -i 's/^Listen 80/#Listen 80/' /etc/httpd/conf/httpd.conf 2>/dev/null || true"
	if _, err := h.sshClient.ExecuteCommand(cmd); err != nil {
		fmt.Printf("  Warning: Could not comment out default Listen 80: %v\n", err)
	}

	// Test configuration
	cmd = "sudo httpd -t"
	if _, err := h.sshClient.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("httpd configuration test failed: %w", err)
	}

	fmt.Println("✓ httpd configured to listen on port 8080")
	return nil
}

// EnableServices enables and starts required systemd services
func (h *HelperNodeSetup) EnableServices() error {
	fmt.Println("Enabling required services...")

	if err := h.ConfigureHTTPD(); err != nil {
		return fmt.Errorf("httpd configuration failed: %w", err)
	}

	// Enable httpd service (for serving ignition files and RHCOS images)
	cmd := "sudo systemctl enable --now httpd"

	if h.verbose {
		fmt.Println("Enabling service: httpd on port 8080")
		fmt.Println("Note: dnsmasq (DNS/DHCP/TFTP) will be started when configured")
	}

	if _, err := h.sshClient.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to enable httpd: %w", err)
	}

	// Verify httpd is listening on port 8080
	cmd = "sudo ss -tlnp | grep ':8080'"
	if output, err := h.sshClient.ExecuteCommand(cmd); err == nil {
		if h.verbose {
			fmt.Printf("  httpd listening on port 8080: %s\n", output)
		}
	}

	fmt.Println("✓ Required services enabled")
	return nil
}

// PerformFullSetup performs complete helper node setup
func (h *HelperNodeSetup) PerformFullSetup() error {
	if err := h.InstallPackages(); err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	if err := h.ConfigureFirewall(); err != nil {
		return fmt.Errorf("firewall configuration failed: %w", err)
	}

	if err := h.DisableSELinux(); err != nil {
		return fmt.Errorf("SELinux configuration failed: %w", err)
	}

	if err := h.SetupDirectories(); err != nil {
		return fmt.Errorf("directory setup failed: %w", err)
	}

	if err := h.EnableServices(); err != nil {
		return fmt.Errorf("service enablement failed: %w", err)
	}

	return nil
}
