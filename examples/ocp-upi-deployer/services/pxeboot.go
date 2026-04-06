package services

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

const grubTemplate = `# GRUB2 Config for {{.Hostname}} (Cluster: {{.ClusterName}})
# MAC: {{.MACAddress}}, Role: {{.Role}}
set default=0
set timeout=10

menuentry '{{.MenuLabel}}' {
  linux {{.ClusterName}}/rhcos/kernel {{.KernelParams}}
  initrd {{.ClusterName}}/rhcos/initramfs.img
}
`

// PXEBootManager handles PXE/GRUB boot configuration for cluster nodes
type PXEBootManager struct {
	ctx       *ClusterContext
	sshClient *communication.SSHClient
}

// NewPXEBootManager creates a new PXE boot configuration manager
func NewPXEBootManager(ctx *ClusterContext, sshClient *communication.SSHClient) *PXEBootManager {
	return &PXEBootManager{
		ctx:       ctx,
		sshClient: sshClient,
	}
}

// Configure sets up PXE boot configuration for all cluster nodes
func (p *PXEBootManager) Configure() error {
	fmt.Printf("Configuring PXE boot for cluster '%s'...\n", p.ctx.Name)

	// Generate GRUB2 network boot directory structure (one-time setup)
	if err := p.setupGRUB2NetBoot(); err != nil {
		return fmt.Errorf("failed to setup GRUB2 netboot: %w", err)
	}

	// Create TFTP directory structure for this cluster
	if err := p.createTFTPDirectories(); err != nil {
		return fmt.Errorf("failed to create TFTP directories: %w", err)
	}

	// Generate and upload GRUB configuration files
	if err := p.generateGRUBConfigs(); err != nil {
		return fmt.Errorf("failed to generate GRUB configs: %w", err)
	}

	// Set proper permissions
	if err := p.setPermissions(); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	fmt.Printf("PXE boot configuration completed for cluster '%s'\n", p.ctx.Name)
	return nil
}

// setupGRUB2NetBoot generates GRUB2 network boot directory structure
// This is a one-time setup that creates the necessary GRUB2 bootloader files
func (p *PXEBootManager) setupGRUB2NetBoot() error {
	fmt.Println("Setting up GRUB2 network boot structure...")

	// Check if already set up
	checkCmd := "test -d /var/lib/tftpboot/boot/grub2 && echo 'exists' || echo 'missing'"
	output, err := p.sshClient.ExecuteCommand(checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check GRUB2 netboot directory: %w", err)
	}

	if strings.TrimSpace(output) == "exists" {
		fmt.Println("  GRUB2 netboot structure already exists, skipping...")
		return nil
	}

	// Generate GRUB2 network boot directory structure
	// This creates /var/lib/tftpboot/boot/grub2/ with necessary bootloader files
	cmd := "sudo grub2-mknetdir --net-directory=/var/lib/tftpboot"
	if _, err := p.sshClient.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to generate GRUB2 netboot structure: %w", err)
	}

	fmt.Println("  ✓ GRUB2 netboot structure created")
	return nil
}

// createTFTPDirectories creates the TFTP directory structure for the cluster
func (p *PXEBootManager) createTFTPDirectories() error {
	fmt.Println("Creating TFTP directory structure...")

	// For Power systems with cluster-specific TFTP root:
	// - core.elf goes in /var/lib/tftpboot/{cluster}/
	// - GRUB configs go in /var/lib/tftpboot/{cluster}/ (GRUB looks relative to tftp-root)
	// - RHCOS files go in /var/lib/tftpboot/{cluster}/rhcos/
	dirs := []string{
		filepath.Join("/var/lib/tftpboot", p.ctx.Name),
		filepath.Join("/var/lib/tftpboot", p.ctx.Name, "rhcos"),
	}

	for _, dir := range dirs {
		cmd := fmt.Sprintf("sudo mkdir -p %s", dir)
		if _, err := p.sshClient.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy core.elf to cluster directory
	// core.elf is the GRUB2 network bootloader for Power systems
	fmt.Printf("  Copying core.elf to cluster directory...\n")
	copyCmd := fmt.Sprintf("sudo cp /var/lib/tftpboot/boot/grub2/powerpc-ieee1275/core.elf /var/lib/tftpboot/%s/", p.ctx.Name)
	if _, err := p.sshClient.ExecuteCommand(copyCmd); err != nil {
		return fmt.Errorf("failed to copy core.elf: %w", err)
	}

	fmt.Printf("  ✓ TFTP directory structure created\n")
	fmt.Printf("  Note: RHCOS boot files will be copied after image download\n")
	return nil
}

// generateGRUBConfigs generates GRUB configuration files for all nodes
func (p *PXEBootManager) generateGRUBConfigs() error {
	fmt.Println("Generating GRUB2 configuration files for Power systems...")

	// Get all nodes for the cluster
	nodes := p.ctx.ClusterConfig.GetAllNodes()

	for _, node := range nodes {
		// Get MAC address from state if available (after LPAR creation)
		macAddress := node.MACAddress
		if macAddress == "" && p.ctx.State != nil {
			if lparState, exists := p.ctx.State.CreatedLPARs[node.Hostname]; exists {
				macAddress = lparState.MACAddress
			}
		}

		// Skip nodes without MAC addresses (not yet provisioned)
		if macAddress == "" {
			fmt.Printf("  Skipping %s - no MAC address assigned yet\n", node.Hostname)
			continue
		}

		// Create a node info with the MAC address for config generation
		nodeWithMAC := node
		nodeWithMAC.MACAddress = macAddress

		// Generate node-specific GRUB config
		grubConfig, err := p.generateNodeGRUBConfig(nodeWithMAC)
		if err != nil {
			return fmt.Errorf("failed to generate GRUB config for node %s: %w", node.Hostname, err)
		}

		// Upload to cluster-specific directory with MAC-based filename
		// GRUB2 uses format: grub.cfg-01-aa-bb-cc-dd-ee-ff
		// Since dhcp-boot points to {cluster}/core.elf, GRUB looks for configs in {cluster}/
		macFormatted := p.formatMACForGRUB(macAddress)
		remotePath := filepath.Join("/var/lib/tftpboot", p.ctx.Name, fmt.Sprintf("grub.cfg-%s", macFormatted))

		if err := p.sshClient.UploadContent(grubConfig, remotePath); err != nil {
			return fmt.Errorf("failed to upload GRUB config for node %s: %w", node.Hostname, err)
		}

		fmt.Printf("  Generated GRUB2 config for %s (MAC: %s) -> %s\n", node.Hostname, macAddress, remotePath)
	}

	fmt.Println("GRUB2 configuration completed for all nodes")
	return nil
}

// generateNodeGRUBConfig generates GRUB2 configuration for a specific node (Power systems)
func (p *PXEBootManager) generateNodeGRUBConfig(node NodeInfo) (string, error) {
	tmpl, err := template.New("grub").Parse(grubTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse grub template: %w", err)
	}

	httpServer := p.ctx.HelperNode.IP
	clusterName := p.ctx.Name

	// Determine ignition file based on node role
	var ignitionFile string
	var menuLabel string
	switch node.Role {
	case "sno":
		ignitionFile = "bootstrap.ign"
		menuLabel = "Install Single Node OpenShift"
	case "bootstrap":
		ignitionFile = "bootstrap.ign"
		menuLabel = "Install Bootstrap Node"
	case "master", "control-plane":
		ignitionFile = "master.ign"
		menuLabel = "Install Master Node"
	case "worker":
		ignitionFile = "worker.ign"
		menuLabel = "Install Worker Node"
	default:
		ignitionFile = "master.ign"
		menuLabel = "Install Master Node"
	}

	// Get install device from config or use default
	installDevice := p.ctx.ClusterConfig.OpenShift.DiskDevice
	if installDevice == "" {
		installDevice = "/dev/sda"
	}

	// Get rootfs filename from state (set by downloader)
	rootfsFilename := p.ctx.State.ServiceEndpoints.RHCOSFiles.Rootfs
	if rootfsFilename == "" {
		rootfsFilename = "rootfs.img" // Fallback to default
	}

	// Get initramfs filename from state
	initramfsFilename := p.ctx.State.ServiceEndpoints.RHCOSFiles.Initramfs
	if initramfsFilename == "" {
		initramfsFilename = "initramfs.img" // Fallback to default
	}

	// Build kernel parameters - SNO uses live boot, others use installer
	var kernelParamsList []string
	if node.Role == "sno" {
		// SNO: Live boot with ignition (no installation)
		kernelParamsList = []string{
			"ip=dhcp",
			"rd.neednet=1",
			"ignition.platform.id=metal",
			"ignition.firstboot",
			fmt.Sprintf("coreos.live.rootfs_url=http://%s:8080/%s/rhcos/%s", httpServer, clusterName, rootfsFilename),
			fmt.Sprintf("ignition.config.url=http://%s:8080/%s/ignition/%s", httpServer, clusterName, ignitionFile),
		}
	} else {
		// Multi-node: Traditional installer boot
		kernelParamsList = []string{
			fmt.Sprintf("initrd=%s/rhcos/%s", clusterName, initramfsFilename),
			"nomodeset",
			"rd.neednet=1",
			"ip=dhcp",
			"coreos.inst=yes",
			fmt.Sprintf("coreos.inst.install_dev=%s", installDevice),
			fmt.Sprintf("coreos.live.rootfs_url=http://%s:8080/%s/rhcos/%s", httpServer, clusterName, rootfsFilename),
			fmt.Sprintf("coreos.inst.ignition_url=http://%s:8080/%s/ignition/%s", httpServer, clusterName, ignitionFile),
		}
	}

	data := struct {
		Hostname     string
		ClusterName  string
		MACAddress   string
		Role         string
		MenuLabel    string
		KernelParams string
	}{
		Hostname:     node.Hostname,
		ClusterName:  clusterName,
		MACAddress:   node.MACAddress,
		Role:         node.Role,
		MenuLabel:    menuLabel,
		KernelParams: strings.Join(kernelParamsList, " "),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute grub template: %w", err)
	}

	return buf.String(), nil
}

// extractFilenameFromURL extracts the filename from a URL
func (p *PXEBootManager) extractFilenameFromURL(url string) string {
	if url == "" {
		return ""
	}

	lastSlash := strings.LastIndex(url, "/")
	if lastSlash == -1 || lastSlash == len(url)-1 {
		return ""
	}

	filename := url[lastSlash+1:]

	if queryStart := strings.Index(filename, "?"); queryStart != -1 {
		filename = filename[:queryStart]
	}

	return filename
}

// formatMACForGRUB converts MAC address to GRUB format
func (p *PXEBootManager) formatMACForGRUB(mac string) string {
	mac = strings.ToLower(strings.ReplaceAll(mac, ":", "-"))
	return "01-" + mac
}

// CopyRHCOSBootFiles copies kernel and initramfs from HTTP directory to TFTP directory
func (p *PXEBootManager) CopyRHCOSBootFiles() error {
	httpRHCOSDir := filepath.Join("/var/www/html", p.ctx.Name, "rhcos")
	tftpRHCOSDir := filepath.Join("/var/lib/tftpboot", p.ctx.Name, "rhcos")

	// Get the filenames from state (populated by downloader)
	// Fall back to default names if state doesn't have them (for backward compatibility)
	rhcosFiles := p.ctx.State.ServiceEndpoints.RHCOSFiles
	kernelSrc := rhcosFiles.Kernel
	initramfsSrc := rhcosFiles.Initramfs

	// Fallback to default filenames if not set in state
	if kernelSrc == "" {
		kernelSrc = "kernel"
	}
	if initramfsSrc == "" {
		initramfsSrc = "initramfs.img"
	}

	// Use the same names for destination
	kernelDest := kernelSrc
	initramfsDest := initramfsSrc

	// Verify files exist
	checkCmd := fmt.Sprintf("test -f %s/%s && test -f %s/%s && echo 'exists' || echo 'missing'",
		httpRHCOSDir, kernelSrc, httpRHCOSDir, initramfsSrc)
	output, err := p.sshClient.ExecuteCommand(checkCmd)
	if err != nil || strings.TrimSpace(output) != "exists" {
		return fmt.Errorf("RHCOS boot files not found in %s. Expected files: %s, %s. Please ensure download_images phase completed successfully",
			httpRHCOSDir, kernelSrc, initramfsSrc)
	}

	// Copy kernel
	copyKernelCmd := fmt.Sprintf("sudo cp %s/%s %s/%s", httpRHCOSDir, kernelSrc, tftpRHCOSDir, kernelDest)
	if _, err := p.sshClient.ExecuteCommand(copyKernelCmd); err != nil {
		return fmt.Errorf("failed to copy kernel: %w", err)
	}
	fmt.Printf("    ✓ Copied %s to %s/%s\n", kernelSrc, tftpRHCOSDir, kernelDest)

	// Copy initramfs
	copyInitramfsCmd := fmt.Sprintf("sudo cp %s/%s %s/%s", httpRHCOSDir, initramfsSrc, tftpRHCOSDir, initramfsDest)
	if _, err := p.sshClient.ExecuteCommand(copyInitramfsCmd); err != nil {
		return fmt.Errorf("failed to copy initramfs: %w", err)
	}
	fmt.Printf("    ✓ Copied %s to %s/%s\n", initramfsSrc, tftpRHCOSDir, initramfsDest)

	return nil
}

// setPermissions sets proper permissions on TFTP directories and files
func (p *PXEBootManager) setPermissions() error {
	fmt.Println("Setting TFTP permissions...")

	clusterTFTPPath := filepath.Join("/var/lib/tftpboot", p.ctx.Name)

	commands := []string{
		"sudo chown -R nobody:nobody /var/lib/tftpboot/boot",
		"sudo chmod -R 755 /var/lib/tftpboot/boot",
		fmt.Sprintf("sudo chown -R nobody:nobody %s", clusterTFTPPath),
		fmt.Sprintf("sudo chmod -R 755 %s", clusterTFTPPath),
		"sudo restorecon -Rv /var/lib/tftpboot/boot",
		fmt.Sprintf("sudo restorecon -Rv %s", clusterTFTPPath),
	}

	for _, cmd := range commands {
		if _, err := p.sshClient.ExecuteCommand(cmd); err != nil {
			if !strings.Contains(cmd, "restorecon") {
				return fmt.Errorf("failed to execute: %s: %w", cmd, err)
			}
		}
	}

	return nil
}

// Cleanup removes PXE boot configuration for the cluster
func (p *PXEBootManager) Cleanup() error {
	fmt.Printf("Cleaning up GRUB2 boot configuration for cluster '%s'...\n", p.ctx.Name)

	clusterTFTPPath := filepath.Join("/var/lib/tftpboot", p.ctx.Name)
	cmd := fmt.Sprintf("sudo rm -rf %s", clusterTFTPPath)
	if _, err := p.sshClient.ExecuteCommand(cmd); err != nil {
		fmt.Printf("Warning: failed to remove cluster TFTP directory %s: %v\n", clusterTFTPPath, err)
	}

	fmt.Printf("GRUB2 boot configuration cleaned up for cluster '%s'\n", p.ctx.Name)
	return nil
}

// GetBootInfo returns boot information for a specific node
func (p *PXEBootManager) GetBootInfo(hostname string) (string, error) {
	nodes := p.ctx.ClusterConfig.GetAllNodes()

	installDevice := p.ctx.ClusterConfig.OpenShift.DiskDevice
	if installDevice == "" {
		installDevice = "/dev/sda"
	}

	for _, node := range nodes {
		if node.Hostname == hostname {
			info := fmt.Sprintf(`Boot Information for %s:
	 Cluster:        %s
	 Role:           %s
	 MAC Address:    %s
	 Install Device: %s
	 GRUB2 Config:   /var/lib/tftpboot/%s/grub.cfg-%s
	 Kernel:         /var/lib/tftpboot/%s/rhcos/kernel
	 Initramfs:      /var/lib/tftpboot/%s/rhcos/initramfs.img
	 Ignition URL:   http://%s:8080/%s/ignition/%s
	 Rootfs URL:     http://%s:8080/%s/install/rootfs.img
`,
				node.Hostname,
				p.ctx.Name,
				node.Role,
				node.MACAddress,
				installDevice,
				p.ctx.Name,
				p.formatMACForGRUB(node.MACAddress),
				p.ctx.Name,
				p.ctx.Name,
				p.ctx.HelperNode.IP,
				p.ctx.Name,
				p.getIgnitionFileName(node.Role),
				p.ctx.HelperNode.IP,
				p.ctx.Name,
			)
			return info, nil
		}
	}

	return "", fmt.Errorf("node %s not found in cluster %s", hostname, p.ctx.Name)
}

// getIgnitionFileName returns the ignition file name based on node role
func (p *PXEBootManager) getIgnitionFileName(role string) string {
	switch role {
	case "sno":
		return "bootstrap.ign"
	case "bootstrap":
		return "bootstrap.ign"
	case "master", "control-plane":
		return "master.ign"
	case "worker":
		return "worker.ign"
	default:
		return "master.ign"
	}
}

// VerifyConfiguration verifies that GRUB2 boot configuration is properly set up
func (p *PXEBootManager) VerifyConfiguration() error {
	fmt.Printf("Verifying GRUB2 boot configuration for cluster '%s'...\n", p.ctx.Name)

	cmd := "test -d /var/lib/tftpboot/boot/grub2 && echo 'exists' || echo 'missing'"
	output, err := p.sshClient.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to check GRUB2 directory: %w", err)
	}

	if strings.TrimSpace(output) != "exists" {
		return fmt.Errorf("GRUB2 directory does not exist: /var/lib/tftpboot/boot/grub2")
	}

	nodes := p.ctx.ClusterConfig.GetAllNodes()
	for _, node := range nodes {
		if node.MACAddress == "" {
			fmt.Printf("  ⚠ Skipping %s - no MAC address assigned\n", node.Hostname)
			continue
		}
		macFormatted := p.formatMACForGRUB(node.MACAddress)
		grubPath := filepath.Join("/var/lib/tftpboot", p.ctx.Name, fmt.Sprintf("grub.cfg-%s", macFormatted))

		cmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'missing'", grubPath)
		output, err := p.sshClient.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to check GRUB2 config for %s: %w", node.Hostname, err)
		}

		if strings.TrimSpace(output) != "exists" {
			return fmt.Errorf("GRUB2 config missing for node %s: %s", node.Hostname, grubPath)
		}

		fmt.Printf("  ✓ GRUB2 config exists for %s\n", node.Hostname)
	}

	fmt.Printf("GRUB2 boot configuration verified successfully for cluster '%s'\n", p.ctx.Name)
	return nil
}

// ListConfigurations lists all GRUB2 configurations for the cluster
func (p *PXEBootManager) ListConfigurations() (string, error) {
	clusterPath := filepath.Join("/var/lib/tftpboot", p.ctx.Name)
	cmd := fmt.Sprintf("ls -lh %s/grub.cfg* 2>/dev/null || echo 'No configurations found'", clusterPath)
	output, err := p.sshClient.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to list GRUB2 configurations: %w", err)
	}

	return output, nil
}
