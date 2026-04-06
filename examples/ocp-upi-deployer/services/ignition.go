package services

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

const installConfigTemplate = `apiVersion: v1
baseDomain: {{.BaseDomain}}
compute:
{{- if .IsSNO}}
- name: worker
  replicas: {{.WorkerReplicas}}
{{- else}}
- hyperthreading: Enabled
  name: worker
  replicas: {{.WorkerReplicas}}
  architecture: ppc64le
{{- end}}
controlPlane:
{{- if .IsSNO}}
  name: master
  replicas: {{.MasterReplicas}}
{{- else}}
  hyperthreading: Enabled
  name: master
  replicas: {{.MasterReplicas}}
  architecture: ppc64le
{{- end}}
metadata:
  name: {{.ClusterName}}
networking:
  clusterNetwork:
  - cidr: {{.ClusterNetworkCIDR}}
    hostPrefix: {{.ClusterNetworkHostPrefix}}
  machineNetwork:
  - cidr: {{.MachineNetwork}}
  networkType: OVNKubernetes
  serviceNetwork:
  - {{.ServiceNetwork}}
platform:
  none: {}
{{- if .IsSNO}}
bootstrapInPlace:
  installationDisk: {{.DiskDevice}}
{{- else}}
fips: false
{{- end}}
pullSecret: '{{.PullSecret}}'
sshKey: '{{.SSHKey}}'
`

// IgnitionGenerator handles OpenShift ignition file generation
type IgnitionGenerator struct {
	ctx        *ClusterContext
	ssh        *communication.SSHClient
	httpServer *HTTPServerManager
}

// NewIgnitionGenerator creates a new ignition generator
func NewIgnitionGenerator(ctx *ClusterContext, ssh *communication.SSHClient, httpServer *HTTPServerManager) *IgnitionGenerator {
	return &IgnitionGenerator{
		ctx:        ctx,
		ssh:        ssh,
		httpServer: httpServer,
	}
}

// Generate creates ignition files for the cluster
func (ig *IgnitionGenerator) Generate() error {
	fmt.Printf("Generating ignition files for deployment %s...\n", ig.ctx.Name)

	// Create working directory on helper node using deployment name for isolation
	workDir := fmt.Sprintf("/root/ocp4-%s", ig.ctx.Name)
	if err := ig.createWorkingDirectory(workDir); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Create install-config.yaml
	if err := ig.createInstallConfig(workDir); err != nil {
		return fmt.Errorf("failed to create install-config.yaml: %w", err)
	}

	// Generate manifests
	if err := ig.generateManifests(workDir); err != nil {
		return fmt.Errorf("failed to generate manifests: %w", err)
	}

	// Configure scheduler (for multi-node only)
	if !ig.ctx.ClusterConfig.IsSNO() {
		if err := ig.configureScheduler(workDir); err != nil {
			return fmt.Errorf("failed to configure scheduler: %w", err)
		}
	}

	// Generate ignition configs
	if err := ig.generateIgnitionConfigs(workDir); err != nil {
		return fmt.Errorf("failed to generate ignition configs: %w", err)
	}

	// Copy ignition files to HTTP directory
	if err := ig.copyIgnitionFiles(workDir); err != nil {
		return fmt.Errorf("failed to copy ignition files: %w", err)
	}

	fmt.Printf("Ignition files generated successfully\n")
	return nil
}

// createWorkingDirectory creates the working directory for ignition generation
func (ig *IgnitionGenerator) createWorkingDirectory(workDir string) error {
	cmd := fmt.Sprintf("mkdir -p %s", workDir)
	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("  ✓ Created working directory: %s\n", workDir)
	return nil
}

// createInstallConfig creates the install-config.yaml file
func (ig *IgnitionGenerator) createInstallConfig(workDir string) error {
	fmt.Printf("  Creating install-config.yaml...\n")

	// Read pull secret
	pullSecretPath := ig.ctx.ClusterConfig.OpenShift.PullSecretFile
	pullSecret, err := os.ReadFile(pullSecretPath)
	if err != nil {
		return fmt.Errorf("failed to read pull secret from %s: %w", pullSecretPath, err)
	}

	// Read SSH public key from helper node
	sshKeyPath := os.ExpandEnv(ig.ctx.ClusterConfig.OpenShift.SSHPublicKeyFile)
	sshKeyContent, err := ig.ssh.ExecuteCommand(fmt.Sprintf("cat %s", sshKeyPath))
	if err != nil {
		return fmt.Errorf("failed to read SSH key from helper node %s: %w", sshKeyPath, err)
	}
	sshKey := []byte(sshKeyContent)

	// Generate install-config.yaml content using the template
	installConfig, err := ig.generateInstallConfigYAML(string(pullSecret), string(sshKey))
	if err != nil {
		return fmt.Errorf("failed to generate install-config.yaml: %w", err)
	}

	// Upload to helper node
	configPath := filepath.Join(workDir, "install-config.yaml")
	if err := ig.ssh.UploadContent(installConfig, configPath); err != nil {
		return fmt.Errorf("failed to upload install-config.yaml: %w", err)
	}

	// Also save a backup copy
	backupPath := filepath.Join(workDir, "install-config.yaml.bak")
	if _, err := ig.ssh.ExecuteCommand(fmt.Sprintf("cp %s %s", configPath, backupPath)); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("  ✓ Created install-config.yaml\n")
	return nil
}

// generateInstallConfigYAML generates the install-config.yaml content
func (ig *IgnitionGenerator) generateInstallConfigYAML(pullSecret, sshKey string) (string, error) {
	tmpl, err := template.New("installConfig").Parse(installConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse install-config template: %w", err)
	}

	cfg := ig.ctx.ClusterConfig

	// Determine worker replicas (0 for SNO, actual count for multi-node)
	workerReplicas := 0
	if !cfg.IsSNO() && cfg.Workers != nil {
		workerReplicas = len(cfg.Workers.Nodes)
	}

	// Determine master replicas (1 for SNO, 3 for multi-node)
	masterReplicas := 1
	if !cfg.IsSNO() {
		masterReplicas = 3
	}

	// Clean up pull secret and ssh key strings to ensure valid YAML
	cleanPullSecret := strings.TrimSpace(pullSecret)
	cleanSSHKey := strings.TrimSpace(sshKey)

	data := struct {
		BaseDomain               string
		WorkerReplicas           int
		MasterReplicas           int
		ClusterName              string
		ClusterNetworkCIDR       string
		ClusterNetworkHostPrefix int
		ServiceNetwork           string
		MachineNetwork           string
		IsSNO                    bool
		DiskDevice               string
		PullSecret               string
		SSHKey                   string
	}{
		BaseDomain:               cfg.Network.BaseDomain,
		WorkerReplicas:           workerReplicas,
		MasterReplicas:           masterReplicas,
		ClusterName:              ig.ctx.Name,
		ClusterNetworkCIDR:       cfg.OpenShift.ClusterNetworkCIDR,
		ClusterNetworkHostPrefix: cfg.OpenShift.ClusterNetworkHostPrefix,
		ServiceNetwork:           cfg.OpenShift.ServiceNetwork,
		MachineNetwork:           cfg.OpenShift.MachineNetwork,
		IsSNO:                    cfg.IsSNO(),
		DiskDevice:               cfg.OpenShift.DiskDevice,
		PullSecret:               cleanPullSecret,
		SSHKey:                   cleanSSHKey,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute install-config template: %w", err)
	}

	return buf.String(), nil
}

// generateManifests generates OpenShift manifests
func (ig *IgnitionGenerator) generateManifests(workDir string) error {
	fmt.Printf("  Generating manifests...\n")

	openshiftInstall := ig.httpServer.GetOpenShiftInstallPath()
	cmd := fmt.Sprintf("cd %s && %s create manifests --dir=.", workDir, openshiftInstall)

	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to create manifests: %w", err)
	}

	fmt.Printf("  ✓ Manifests generated\n")
	return nil
}

// configureScheduler configures the cluster scheduler for multi-node clusters
func (ig *IgnitionGenerator) configureScheduler(workDir string) error {
	fmt.Printf("  Configuring scheduler (mastersSchedulable=false)...\n")

	schedulerPath := filepath.Join(workDir, "manifests", "cluster-scheduler-02-config.yml")

	// Use sed to replace mastersSchedulable: true with mastersSchedulable: false
	cmd := fmt.Sprintf("sed -i 's/mastersSchedulable: true/mastersSchedulable: false/g' %s", schedulerPath)

	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to configure scheduler: %w", err)
	}

	fmt.Printf("  ✓ Scheduler configured\n")
	return nil
}

// generateIgnitionConfigs generates ignition configuration files
func (ig *IgnitionGenerator) generateIgnitionConfigs(workDir string) error {
	fmt.Printf("  Generating ignition configs...\n")

	openshiftInstall := ig.httpServer.GetOpenShiftInstallPath()

	// SNO uses different command than multi-node
	var cmd string
	if ig.ctx.ClusterConfig.IsSNO() {
		// For SNO, use create single-node-ignition-config
		// This creates bootstrap-in-place ignition config
		cmd = fmt.Sprintf("cd %s && %s create single-node-ignition-config --dir=.", workDir, openshiftInstall)
		fmt.Printf("  Using SNO command: create single-node-ignition-config\n")
	} else {
		// For multi-node, use create ignition-configs
		cmd = fmt.Sprintf("cd %s && %s create ignition-configs --dir=.", workDir, openshiftInstall)
		fmt.Printf("  Using multi-node command: create ignition-configs\n")
	}

	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to create ignition configs: %w", err)
	}

	fmt.Printf("  ✓ Ignition configs generated\n")
	return nil
}

// copyIgnitionFiles copies ignition files to the HTTP directory
func (ig *IgnitionGenerator) copyIgnitionFiles(workDir string) error {
	fmt.Printf("  Copying ignition files to HTTP directory...\n")

	ignitionDir := filepath.Join(ig.httpServer.GetClusterHTTPDir(), "ignition")

	if ig.ctx.ClusterConfig.IsSNO() {
		// For SNO, create single-node-ignition-config generates:
		// - bootstrap-in-place-for-live-iso.ign
		// We copy it as bootstrap.ign for PXE boot
		srcPath := filepath.Join(workDir, "bootstrap-in-place-for-live-iso.ign")
		destPath := filepath.Join(ignitionDir, "bootstrap.ign")

		// Copy file
		cmd := fmt.Sprintf("cp %s %s", srcPath, destPath)
		if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to copy bootstrap-in-place-for-live-iso.ign: %w", err)
		}

		// Set permissions (readable by all)
		cmd = fmt.Sprintf("chmod 644 %s", destPath)
		if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to set permissions on bootstrap.ign: %w", err)
		}

		fmt.Printf("  ✓ Copied bootstrap-in-place-for-live-iso.ign as bootstrap.ign\n")
	} else {
		// Multi-node needs all three: bootstrap.ign, master.ign, worker.ign
		ignitionFiles := []string{"bootstrap.ign", "master.ign", "worker.ign"}

		for _, file := range ignitionFiles {
			srcPath := filepath.Join(workDir, file)
			destPath := filepath.Join(ignitionDir, file)

			// Copy file
			cmd := fmt.Sprintf("cp %s %s", srcPath, destPath)
			if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
				return fmt.Errorf("failed to copy %s: %w", file, err)
			}

			// Set permissions (readable by all)
			cmd = fmt.Sprintf("chmod 644 %s", destPath)
			if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", file, err)
			}

			fmt.Printf("  ✓ Copied %s\n", file)
		}
	}

	// Restore SELinux contexts
	cmd := fmt.Sprintf("restorecon -vR %s", ignitionDir)
	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		fmt.Printf("  Warning: Could not restore SELinux contexts: %v\n", err)
	}

	return nil
}

// GetKubeconfigPath returns the path to the kubeconfig file
func (ig *IgnitionGenerator) GetKubeconfigPath() string {
	workDir := fmt.Sprintf("/root/ocp4-%s", ig.ctx.Name)
	return filepath.Join(workDir, "auth", "kubeconfig")
}

// GetKubeadminPasswordPath returns the path to the kubeadmin password file
func (ig *IgnitionGenerator) GetKubeadminPasswordPath() string {
	workDir := fmt.Sprintf("/root/ocp4-%s", ig.ctx.Name)
	return filepath.Join(workDir, "auth", "kubeadmin-password")
}

// DownloadKubeconfig downloads the kubeconfig file from the helper node
func (ig *IgnitionGenerator) DownloadKubeconfig(localPath string) error {
	fmt.Printf("Downloading kubeconfig...\n")

	remotePath := ig.GetKubeconfigPath()

	// Check if file exists
	checkCmd := fmt.Sprintf("test -f %s", remotePath)
	if _, err := ig.ssh.ExecuteCommand(checkCmd); err != nil {
		return fmt.Errorf("kubeconfig not found at %s", remotePath)
	}

	// Download file
	if err := ig.ssh.DownloadFile(remotePath, localPath); err != nil {
		return fmt.Errorf("failed to download kubeconfig: %w", err)
	}

	fmt.Printf("  ✓ Kubeconfig downloaded to %s\n", localPath)
	return nil
}

// Cleanup removes the working directory
func (ig *IgnitionGenerator) Cleanup() error {
	fmt.Printf("Cleaning up ignition working directory...\n")

	workDir := fmt.Sprintf("/root/ocp4-%s", ig.ctx.Name)

	cmd := fmt.Sprintf("rm -rf %s", workDir)
	if _, err := ig.ssh.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to remove working directory: %w", err)
	}

	fmt.Printf("  ✓ Working directory removed\n")
	return nil
}
