package services

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

// HTTPServerManager manages HTTP server directory structure for OpenShift installation
type HTTPServerManager struct {
	ctx        *ClusterContext
	ssh        *communication.SSHClient
	downloader *Downloader
}

// NewHTTPServerManager creates a new HTTP server manager
func NewHTTPServerManager(ctx *ClusterContext, ssh *communication.SSHClient) *HTTPServerManager {
	return &HTTPServerManager{
		ctx:        ctx,
		ssh:        ssh,
		downloader: NewDownloader(ctx, ssh),
	}
}

// Setup creates the HTTP directory structure and downloads required files
func (h *HTTPServerManager) Setup() error {
	fmt.Printf("Setting up HTTP server for deployment %s...\n", h.ctx.Name)

	// Create base cluster directory
	if err := h.createClusterDirectory(); err != nil {
		return fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// Create subdirectories
	if err := h.createSubdirectories(); err != nil {
		return fmt.Errorf("failed to create subdirectories: %w", err)
	}

	// Download RHCOS images and OpenShift tools
	if err := h.downloader.DownloadAll(); err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}

	// Set proper permissions
	if err := h.setPermissions(); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Restore SELinux contexts
	if err := h.restoreSELinuxContexts(); err != nil {
		return fmt.Errorf("failed to restore SELinux contexts: %w", err)
	}

	// Create helper script
	if err := h.createHelperScript(); err != nil {
		return fmt.Errorf("failed to create helper script: %w", err)
	}

	fmt.Printf("HTTP server setup completed successfully\n")
	return nil
}

// createClusterDirectory creates the main cluster directory
func (h *HTTPServerManager) createClusterDirectory() error {
	clusterDir := h.GetClusterHTTPDir()

	// Check if directory already exists
	checkCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", clusterDir)
	output, err := h.ssh.ExecuteCommand(checkCmd)

	if err == nil && strings.TrimSpace(output) == "exists" {
		fmt.Printf("  ℹ Cluster directory already exists: %s\n", clusterDir)

		// Check if it contains files from a previous deployment
		listCmd := fmt.Sprintf("ls -A %s 2>/dev/null | wc -l", clusterDir)
		countOutput, _ := h.ssh.ExecuteCommand(listCmd)
		fileCount := strings.TrimSpace(countOutput)

		if fileCount != "0" {
			fmt.Printf("  ⚠ Warning: Directory contains %s files from previous deployment\n", fileCount)
			fmt.Printf("  ℹ Existing files will be overwritten during setup\n")
		}
		return nil
	}

	// Create directory if it doesn't exist
	cmd := fmt.Sprintf("mkdir -p %s", clusterDir)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to create cluster directory %s: %w", clusterDir, err)
	}

	fmt.Printf("  ✓ Created cluster directory: %s\n", clusterDir)
	return nil
}

// createSubdirectories creates all required subdirectories
func (h *HTTPServerManager) createSubdirectories() error {
	clusterDir := h.GetClusterHTTPDir()

	subdirs := []string{
		"ignition", // Ignition files (bootstrap.ign, master.ign, worker.ign)
		"rhcos",    // RHCOS images (kernel, initramfs, rootfs)
		"tools",    // OpenShift installer and client tools
		"scripts",  // Helper scripts
	}

	for _, subdir := range subdirs {
		path := filepath.Join(clusterDir, subdir)

		// Check if subdirectory exists
		checkCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", path)
		output, err := h.ssh.ExecuteCommand(checkCmd)

		if err == nil && strings.TrimSpace(output) == "exists" {
			fmt.Printf("  ℹ Subdirectory already exists: %s\n", path)
			continue
		}

		// Create subdirectory
		cmd := fmt.Sprintf("mkdir -p %s", path)
		if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to create subdirectory %s: %w", path, err)
		}
		fmt.Printf("  ✓ Created subdirectory: %s\n", path)
	}

	return nil
}

// setPermissions sets proper permissions on HTTP directories
func (h *HTTPServerManager) setPermissions() error {
	clusterDir := h.GetClusterHTTPDir()

	// Set directory permissions to 755 (rwxr-xr-x)
	cmd := fmt.Sprintf("chmod -R 755 %s", clusterDir)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Set ownership to apache user (typically apache:apache or httpd:httpd)
	cmd = fmt.Sprintf("chown -R apache:apache %s 2>/dev/null || chown -R httpd:httpd %s 2>/dev/null || true",
		clusterDir, clusterDir)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		fmt.Printf("  Warning: Could not set apache/httpd ownership: %v\n", err)
	}

	fmt.Printf("  ✓ Set permissions on %s\n", clusterDir)
	return nil
}

// restoreSELinuxContexts restores SELinux contexts for HTTP directories
func (h *HTTPServerManager) restoreSELinuxContexts() error {
	clusterDir := h.GetClusterHTTPDir()

	if err := h.ssh.RestoreconRecursive(clusterDir); err != nil {
		fmt.Printf("  Warning: Could not restore SELinux contexts: %v\n", err)
		return nil
	}

	fmt.Printf("  ✓ Restored SELinux contexts for %s\n", clusterDir)
	return nil
}

// createHelperScript creates the install helper script
func (h *HTTPServerManager) createHelperScript() error {
	script := GenerateHelperScript(h.ctx)

	scriptPath := filepath.Join(h.GetClusterHTTPDir(), "scripts", "install-helper.sh")

	if err := h.ssh.UploadContent(script, scriptPath); err != nil {
		return fmt.Errorf("failed to upload helper script: %w", err)
	}

	// Make executable
	cmd := fmt.Sprintf("chmod 755 %s", scriptPath)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	fmt.Printf("  ✓ Created install helper script\n")
	return nil
}

// UploadIgnitionFile uploads an ignition file to the cluster's ignition directory
func (h *HTTPServerManager) UploadIgnitionFile(filename string, content []byte) error {
	destPath := filepath.Join(h.GetClusterHTTPDir(), "ignition", filename)

	if err := h.ssh.UploadContent(string(content), destPath); err != nil {
		return fmt.Errorf("failed to upload ignition file %s: %w", filename, err)
	}

	// Set proper permissions
	cmd := fmt.Sprintf("chmod 644 %s", destPath)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", destPath, err)
	}

	fmt.Printf("  ✓ Uploaded ignition file: %s\n", filename)
	return nil
}

// GetIgnitionURL returns the HTTP URL for an ignition file
func (h *HTTPServerManager) GetIgnitionURL(filename string) string {
	return fmt.Sprintf("http://%s:8080/%s/ignition/%s",
		h.ctx.VIP,
		h.ctx.Name,
		filename)
}

// GetRHCOSImageURL returns the HTTP URL for an RHCOS image
func (h *HTTPServerManager) GetRHCOSImageURL(filename string) string {
	return fmt.Sprintf("http://%s:8080/%s/rhcos/%s",
		h.ctx.VIP,
		h.ctx.Name,
		filename)
}

// GetKernelURL returns the URL for the RHCOS kernel
func (h *HTTPServerManager) GetKernelURL() string {
	return h.GetRHCOSImageURL("rhcos-live-kernel-ppc64le")
}

// GetInitramfsURL returns the URL for the RHCOS initramfs
func (h *HTTPServerManager) GetInitramfsURL() string {
	return h.GetRHCOSImageURL("rhcos-live-initramfs.ppc64le.img")
}

// GetRootfsURL returns the URL for the RHCOS rootfs
func (h *HTTPServerManager) GetRootfsURL() string {
	return h.GetRHCOSImageURL("rhcos-live-rootfs.ppc64le.img")
}

// GetOpenShiftInstallPath returns the path to openshift-install binary
func (h *HTTPServerManager) GetOpenShiftInstallPath() string {
	return filepath.Join(h.GetClusterHTTPDir(), "tools", "openshift-install")
}

// GetClusterHTTPDir returns the cluster's HTTP directory path
// Uses ctx.Name (from multi-cluster config) for the directory name
func (h *HTTPServerManager) GetClusterHTTPDir() string {
	return filepath.Join("/var/www/html", h.ctx.Name)
}

// Cleanup removes the cluster's HTTP directory
func (h *HTTPServerManager) Cleanup() error {
	fmt.Printf("Cleaning up HTTP directories for cluster %s...\n", h.ctx.Name)

	clusterDir := h.GetClusterHTTPDir()

	cmd := fmt.Sprintf("rm -rf %s", clusterDir)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to remove cluster directory: %w", err)
	}

	fmt.Printf("  ✓ HTTP directories removed successfully\n")
	return nil
}

// VerifySetup verifies that the HTTP directory structure is correct
func (h *HTTPServerManager) VerifySetup() error {
	fmt.Printf("Verifying HTTP server setup for deployment %s...\n", h.ctx.Name)

	clusterDir := h.GetClusterHTTPDir()

	// Check main directory
	cmd := fmt.Sprintf("test -d %s", clusterDir)
	if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("cluster directory does not exist: %s", clusterDir)
	}

	// Check subdirectories
	subdirs := []string{"ignition", "rhcos", "tools", "scripts"}
	for _, subdir := range subdirs {
		path := filepath.Join(clusterDir, subdir)
		cmd := fmt.Sprintf("test -d %s", path)
		if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("subdirectory does not exist: %s", path)
		}
	}

	// Check RHCOS images
	rhcosImages := []string{
		"rhcos-live-kernel-ppc64le",
		"rhcos-live-initramfs.ppc64le.img",
		"rhcos-live-rootfs.ppc64le.img",
	}
	for _, image := range rhcosImages {
		path := filepath.Join(clusterDir, "rhcos", image)
		cmd := fmt.Sprintf("test -f %s", path)
		if _, err := h.ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("RHCOS image missing: %s", image)
		}
	}

	fmt.Printf("  ✓ HTTP server setup verified successfully\n")
	return nil
}

// GetDiskUsage returns the disk usage of the cluster's HTTP directory
func (h *HTTPServerManager) GetDiskUsage() (string, error) {
	clusterDir := h.GetClusterHTTPDir()

	cmd := fmt.Sprintf("du -sh %s 2>/dev/null | awk '{print $1}'", clusterDir)
	output, err := h.ssh.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get disk usage: %w", err)
	}

	return output, nil
}

// Made with Bob
