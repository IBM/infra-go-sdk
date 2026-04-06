package services

import (
	"fmt"
	"path/filepath"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

// Downloader handles downloading RHCOS images and OpenShift tools
type Downloader struct {
	ctx *ClusterContext
	ssh *communication.SSHClient
}

// NewDownloader creates a new downloader
func NewDownloader(ctx *ClusterContext, ssh *communication.SSHClient) *Downloader {
	return &Downloader{
		ctx: ctx,
		ssh: ssh,
	}
}

// DownloadAll downloads all required artifacts
func (d *Downloader) DownloadAll() error {
	if err := d.DownloadRHCOSImages(); err != nil {
		return fmt.Errorf("failed to download RHCOS images: %w", err)
	}
	if err := d.DownloadOpenShiftTools(); err != nil {
		return fmt.Errorf("failed to download OpenShift tools: %w", err)
	}
	return nil
}

// DownloadRHCOSImages downloads RHCOS images using generic filenames
func (d *Downloader) DownloadRHCOSImages() error {
	fmt.Printf("Downloading RHCOS images to HTTP directory...\n")
	rhcosDir := filepath.Join("/var/www/html", d.ctx.Name, "rhcos")
	rhcosURLs := d.ctx.ClusterConfig.OpenShift.RHCOSImages

	images := []struct {
		url      string
		filename string
		desc     string
	}{
		{rhcosURLs.KernelURL, "kernel", "RHCOS kernel"},
		{rhcosURLs.InitramfsURL, "initramfs.img", "RHCOS initramfs"},
		{rhcosURLs.RootfsURL, "rootfs.img", "RHCOS rootfs"},
	}

	for _, image := range images {
		if image.url == "" {
			return fmt.Errorf("%s URL not provided in configuration", image.desc)
		}
		destPath := filepath.Join(rhcosDir, image.filename)

		// Check if file exists and has size > 0
		checkCmd := fmt.Sprintf("test -s %s", destPath)
		if _, err := d.ssh.ExecuteCommand(checkCmd); err == nil {
			fmt.Printf("  ✓ %s already exists, skipping download\n", image.desc)
			continue
		}

		fmt.Printf("  Downloading %s...\n", image.desc)
		// Robust curl with retries and timeouts, using -sSL for SSH safety
		downloadCmd := fmt.Sprintf("curl -sSL -C - --retry 3 --retry-delay 5 --max-time 1800 -o %s '%s'", destPath, image.url)
		if _, err := d.ssh.ExecuteCommand(downloadCmd); err != nil {
			return fmt.Errorf("failed to download %s from %s: %w", image.desc, image.url, err)
		}
		fmt.Printf("  ✓ Downloaded: %s\n", image.desc)
	}

	// Store the downloaded filenames in state for use by other components
	d.ctx.State.ServiceEndpoints.RHCOSFiles = RHCOSFiles{
		Kernel:    "kernel",
		Initramfs: "initramfs.img",
		Rootfs:    "rootfs.img",
	}

	return nil
}

// DownloadOpenShiftTools downloads and extracts installer/client tools
func (d *Downloader) DownloadOpenShiftTools() error {
	fmt.Printf("Downloading OpenShift tools...\n")
	toolsDir := filepath.Join("/var/www/html", d.ctx.Name, "tools")
	ocpConfig := d.ctx.ClusterConfig.OpenShift.OCPClientConfig

	tools := []struct {
		url      string
		filename string
		desc     string
	}{
		{ocpConfig.Installer, "openshift-install-linux.tar.gz", "OpenShift installer"},
		{ocpConfig.Client, "openshift-client-linux.tar.gz", "OpenShift client"},
	}

	for _, tool := range tools {
		if tool.url == "" {
			continue
		}
		destPath := filepath.Join(toolsDir, tool.filename)
		checkCmd := fmt.Sprintf("test -s %s", destPath)
		if _, err := d.ssh.ExecuteCommand(checkCmd); err == nil {
			continue
		}

		fmt.Printf("  Downloading %s...\n", tool.desc)
		downloadCmd := fmt.Sprintf("curl -sSL -C - --retry 3 --retry-delay 5 --max-time 900 -o %s '%s'", destPath, tool.url)
		if _, err := d.ssh.ExecuteCommand(downloadCmd); err != nil {
			fmt.Printf("  Warning: Failed to download %s: %v\n", tool.desc, err)
			continue
		}
	}

	return d.extractOpenShiftTools(toolsDir)
}

func (d *Downloader) extractOpenShiftTools(toolsDir string) error {
	tools := []string{"openshift-install-linux.tar.gz", "openshift-client-linux.tar.gz"}
	for _, tool := range tools {
		tarPath := filepath.Join(toolsDir, tool)
		if _, err := d.ssh.ExecuteCommand(fmt.Sprintf("test -s %s", tarPath)); err != nil {
			continue
		}
		extractCmd := fmt.Sprintf("cd %s && tar -xzf %s", toolsDir, tool)
		if _, err := d.ssh.ExecuteCommand(extractCmd); err != nil {
			return fmt.Errorf("failed to extract %s: %w", tool, err)
		}
	}
	makeExecCmd := fmt.Sprintf("cd %s && chmod +x openshift-install oc kubectl", toolsDir)
	_, err := d.ssh.ExecuteCommand(makeExecCmd)
	return err
}
