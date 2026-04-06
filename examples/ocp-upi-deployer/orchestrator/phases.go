package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/infrastructure"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/services"
	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/validation"
)

// ============================================================================
// PHASE EXECUTION
// ============================================================================

// executePhase routes to the appropriate phase handler based on phase name
func (o *Orchestrator) executePhase(ctx *ClusterContext, phase string) error {
	switch phase {
	case "validate", "validate_config":
		return o.phaseValidate(ctx)
	case "check_resources":
		return o.phaseCheckResources(ctx)
	case "setup_helper_services":
		return o.phaseSetupHelperServices(ctx)
	case "create_lpars":
		return o.phaseCreateLPARs(ctx)
	case "setup_dns":
		return o.phaseSetupDNS(ctx)
	case "setup_dhcp":
		return o.phaseSetupDHCP(ctx)
	case "setup_pxe":
		return o.phaseSetupPXE(ctx)
	case "setup_http":
		return o.phaseSetupHTTP(ctx)
	case "setup_haproxy":
		return o.phaseSetupHAProxy(ctx)
	case "download_images":
		return o.phaseDownloadImages(ctx)
	case "generate_ignition":
		return o.phaseGenerateIgnition(ctx)
	case "power_on":
		return o.phasePowerOn(ctx)
	case "wait_bootstrap":
		return o.phaseWaitBootstrap(ctx)
	case "wait_installation":
		return o.phaseWaitInstallation(ctx)
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}
}

// ============================================================================
// VALIDATION PHASES
// ============================================================================

// phaseValidate validates the cluster configuration
func (o *Orchestrator) phaseValidate(ctx *ClusterContext) error {
	fmt.Println("Validating configuration...")

	// Create a temporary multi-cluster config for validation
	multiConfig := &MultiClusterConfig{
		HelperNode: ctx.HelperNode,
		HMC:        ctx.HMC,
	}

	validator := validation.NewValidator(multiConfig, ctx.ClusterConfig, ctx.Name, o.sshClient, o.verbose)
	validator.SetHMCClient(o.hmcClient)
	if err := validator.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Println("✓ Configuration validated")
	return nil
}

// phaseCheckResources queries the HMC to ensure physical hardware has enough capacity
func (o *Orchestrator) phaseCheckResources(ctx *ClusterContext) error {
	fmt.Println("Checking physical resources on managed systems via HMC...")

	if !ctx.ClusterConfig.Advanced.ValidateResources {
		fmt.Println("  ℹ Resource validation disabled in advanced config. Skipping.")
		return nil
	}

	// 1. Calculate REQUIRED resources per physical system based on user's node config
	reqMem := make(map[string]int)     // in MB
	reqCpu := make(map[string]float64) // in Units

	nodes := ctx.ClusterConfig.GetAllNodes()
	for _, node := range nodes {
		reqMem[node.SystemName] += node.LPAR.Memory.DesiredMB
		reqCpu[node.SystemName] += node.LPAR.Processor.Units
	}

	// 2. Query the HMC for ACTUAL available resources
	for i, ps := range ctx.ClusterConfig.PowerSystems {
		fmt.Printf("  Querying managed system: %s...\n", ps.Name)

		sysUUID, sys, err := o.hmcClient.GetManagedSystemByName(ps.Name, o.verbose)
		if err != nil {
			return fmt.Errorf("failed to query HMC for system %s: %w", ps.Name, err)
		}
		_ = sysUUID // UUID not needed for resource validation

		// Extract live available values from the HMC SDK response
		// (These field names match the standard IBM HMC REST API schema used by powerhmc-go)
		availMemMB := sys.MemoryConfig.CurrentAvailableSystemMemory
		availCpu := sys.ProcessorConfig.CurrentAvailableSystemProcessorUnits

		// Dynamically update the context configuration with the live data!
		ctx.ClusterConfig.PowerSystems[i].AvailableMemoryGB = int(availMemMB / 1024)
		ctx.ClusterConfig.PowerSystems[i].AvailableProcessors = int(availCpu)

		fmt.Printf("    System Capacity:  %d GB RAM, %.2f Cores available\n", ctx.ClusterConfig.PowerSystems[i].AvailableMemoryGB, availCpu)
		fmt.Printf("    Cluster Requires: %d GB RAM, %.2f Cores\n", reqMem[ps.Name]/1024, reqCpu[ps.Name])

		// 3. Mathematical Validation
		if float64(availMemMB) < float64(reqMem[ps.Name]) {
			return fmt.Errorf("insufficient memory on %s: cluster needs %d MB, but only %d MB is available physically",
				ps.Name, reqMem[ps.Name], int(availMemMB)) // <-- Added int() cast here
		}

		if availCpu < reqCpu[ps.Name] {
			return fmt.Errorf("insufficient processors on %s: cluster needs %.2f units, but only %.2f are available physically",
				ps.Name, reqCpu[ps.Name], availCpu)
		}

		fmt.Printf("    ✓ Sufficient resources confirmed on %s\n", ps.Name)
	}

	return nil
}

// ============================================================================
// HELPER NODE SETUP PHASES
// ============================================================================

// phaseSetupHelperServices performs one-time helper node setup (packages, firewall, directories)
func (o *Orchestrator) phaseSetupHelperServices(ctx *ClusterContext) error {
	fmt.Println("Setting up helper node (packages, firewall, directories)...")

	// Perform one-time helper node setup (packages, firewall, directories, etc.)
	helperSetup := services.NewHelperNodeSetup(o.sshClient, o.verbose, o.config.HelperNode.RequiredPackages)
	if err := helperSetup.PerformFullSetup(); err != nil {
		return fmt.Errorf("failed to setup helper node: %w", err)
	}

	// Setup IP aliases for VIPs
	if err := o.setupIPAliases(ctx); err != nil {
		return fmt.Errorf("failed to setup IP aliases: %w", err)
	}

	fmt.Println("✓ Helper node setup completed")
	return nil
}

// setupIPAliases configures IP alias for cluster VIP (persistent across reboots)
func (o *Orchestrator) setupIPAliases(ctx *ClusterContext) error {
	fmt.Println("Setting up persistent IP alias for VIP...")

	iface := ctx.HelperNode.NetworkInterface

	// Convert CIDR to netmask (e.g., "192.0.2.0/20" -> "255.255.240.0")
	netmask := infrastructure.CidrToNetmask(ctx.ClusterConfig.Network.NetworkCIDR)
	if netmask == "" {
		return fmt.Errorf("invalid network CIDR: %s", ctx.ClusterConfig.Network.NetworkCIDR)
	}

	// Check if VIP already exists
	exists, err := o.networkManager.CheckVIPExists(iface, ctx.VIP)
	if err != nil {
		return fmt.Errorf("failed to check VIP existence: %w", err)
	}

	if !exists {
		// Add VIP with persistent configuration
		if err := o.networkManager.AddVIPAlias(iface, ctx.VIP, netmask, ctx.Name); err != nil {
			return fmt.Errorf("failed to add VIP: %w", err)
		}
		fmt.Printf("✓ Added persistent VIP: %s (survives reboots)\n", ctx.VIP)
	} else {
		fmt.Printf("✓ VIP already configured: %s\n", ctx.VIP)

		// Verify persistence configuration exists
		persistent, _ := o.networkManager.VerifyVIPPersistence(iface, ctx.Name)
		if !persistent {
			fmt.Printf("⚠ Warning: VIP exists but persistence config missing, recreating...\n")
			if err := o.networkManager.AddVIPAlias(iface, ctx.VIP, netmask, ctx.Name); err != nil {
				return fmt.Errorf("failed to add VIP persistence: %w", err)
			}
		}
	}

	ctx.State.IPAliases = append(ctx.State.IPAliases, IPAliasState{
		Interface: iface,
		IP:        ctx.VIP,
		Purpose:   "cluster-vip",
	})

	fmt.Printf("✓ IP alias configured: VIP=%s (used for both API and Ingress)\n", ctx.VIP)
	return nil
}

// ============================================================================
// INFRASTRUCTURE SETUP PHASES
// ============================================================================

// phaseSetupDNS configures DNS records for the cluster
func (o *Orchestrator) phaseSetupDNS(ctx *ClusterContext) error {
	fmt.Println("Configuring DNS records...")

	// Generate DNS configuration
	dnsmasq := services.NewDNSmasqGenerator(ctx, o.verbose)
	dnsConfig, err := dnsmasq.GenerateDNS()
	if err != nil {
		return fmt.Errorf("failed to generate DNS config: %w", err)
	}

	// Upload DNS configuration
	dnsPath := dnsmasq.GetDNSConfigPath()
	if err := o.sshClient.UploadContent(dnsConfig, dnsPath); err != nil {
		return fmt.Errorf("failed to upload DNS config: %w", err)
	}
	fmt.Printf("✓ DNS configuration uploaded to %s\n", dnsPath)

	// Track the file for cleanup
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, dnsPath)
	o.saveState(ctx)

	// Enable and restart dnsmasq to apply DNS configuration
	fmt.Println("Enabling and restarting dnsmasq service...")
	if err := o.sshClient.SystemctlEnable("dnsmasq"); err != nil {
		fmt.Printf("Warning: Failed to enable dnsmasq (may already be enabled): %v\n", err)
	}
	if err := o.sshClient.SystemctlRestart("dnsmasq"); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w", err)
	}

	fmt.Println("✓ DNS configured successfully")
	return nil
}

// phaseSetupDHCP configures DHCP with MAC-to-IP bindings
func (o *Orchestrator) phaseSetupDHCP(ctx *ClusterContext) error {
	fmt.Println("Configuring DHCP with MAC-to-IP bindings...")

	// Generate DHCP configuration
	dnsmasq := services.NewDNSmasqGenerator(ctx, o.verbose)
	dhcpConfig, err := dnsmasq.GenerateDHCP()
	if err != nil {
		return fmt.Errorf("failed to generate DHCP config: %w", err)
	}

	// Upload DHCP configuration
	dhcpPath := dnsmasq.GetDHCPConfigPath()
	if err := o.sshClient.UploadContent(dhcpConfig, dhcpPath); err != nil {
		return fmt.Errorf("failed to upload DHCP config: %w", err)
	}
	fmt.Printf("✓ DHCP configuration uploaded to %s\n", dhcpPath)

	// Track the file for cleanup
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, dhcpPath)
	o.saveState(ctx)

	// Restart dnsmasq to apply DHCP configuration
	fmt.Println("Restarting dnsmasq service...")
	if err := o.sshClient.SystemctlRestart("dnsmasq"); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w", err)
	}

	fmt.Println("✓ DHCP configured successfully")
	return nil
}

// phaseSetupPXE configures PXE/TFTP boot for the cluster
func (o *Orchestrator) phaseSetupPXE(ctx *ClusterContext) error {
	fmt.Println("Configuring PXE/TFTP boot...")

	// Create cluster-specific TFTP directory structure
	tftpDir := fmt.Sprintf("/var/lib/tftpboot/%s", ctx.Name)
	tftpRHCOSDir := fmt.Sprintf("%s/rhcos", tftpDir)
	createDirCmd := fmt.Sprintf("sudo mkdir -p %s", tftpRHCOSDir)
	if _, err := o.sshClient.ExecuteCommand(createDirCmd); err != nil {
		return fmt.Errorf("failed to create TFTP directories: %w", err)
	}

	// Copy RHCOS boot files from HTTP to TFTP directory
	fmt.Println("Copying RHCOS boot files to TFTP directory...")
	pxeManager := services.NewPXEBootManager(ctx, o.sshClient)
	if err := pxeManager.CopyRHCOSBootFiles(); err != nil {
		return fmt.Errorf("failed to copy RHCOS boot files to TFTP: %w", err)
	}

	// Generate PXE configuration
	dnsmasq := services.NewDNSmasqGenerator(ctx, o.verbose)
	pxeConfig, err := dnsmasq.GeneratePXE()
	if err != nil {
		return fmt.Errorf("failed to generate PXE config: %w", err)
	}

	// Upload PXE configuration
	pxePath := dnsmasq.GetPXEConfigPath()
	if err := o.sshClient.UploadContent(pxeConfig, pxePath); err != nil {
		return fmt.Errorf("failed to upload PXE config: %w", err)
	}
	fmt.Printf("✓ PXE configuration uploaded to %s\n", pxePath)

	// Configure PXE boot (GRUB2 for Power systems)
	fmt.Println("Configuring PXE boot (GRUB2)...")
	if err := pxeManager.Configure(); err != nil {
		return fmt.Errorf("failed to configure PXE boot: %w", err)
	}

	// Track files for cleanup
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, tftpDir, pxePath)
	o.saveState(ctx)

	// Restart dnsmasq to apply PXE configuration
	fmt.Println("Restarting dnsmasq service...")
	if err := o.sshClient.SystemctlRestart("dnsmasq"); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w", err)
	}

	fmt.Println("✓ PXE/TFTP configured successfully")
	return nil
}

// phaseSetupHTTP configures HTTP server for serving installation files
func (o *Orchestrator) phaseSetupHTTP(ctx *ClusterContext) error {
	fmt.Println("Setting up HTTP server...")

	httpServer := services.NewHTTPServerManager(ctx, o.sshClient)
	if err := httpServer.Setup(); err != nil {
		return fmt.Errorf("failed to setup HTTP server: %w", err)
	}

	// Right before 'return nil' at the bottom of the function:
	httpDir := fmt.Sprintf("/var/www/html/%s", ctx.Name)
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, httpDir)
	o.saveState(ctx) // Persist the tracking

	fmt.Println("✓ HTTP server configured")
	return nil
}

// phaseSetupHAProxy configures HAProxy for load balancing
func (o *Orchestrator) phaseSetupHAProxy(ctx *ClusterContext) error {
	fmt.Println("Configuring HAProxy...")

	haproxy := services.NewHAProxyGenerator(ctx, o.verbose)
	haproxyConfig, err := haproxy.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate HAProxy config: %w", err)
	}

	// Create conf.d directory if it doesn't exist
	if _, err := o.sshClient.ExecuteCommand("sudo mkdir -p /etc/haproxy/conf.d"); err != nil {
		return fmt.Errorf("failed to create HAProxy conf.d directory: %w", err)
	}

	// Upload HAProxy configuration to conf.d with cluster-specific name
	haproxyPath := fmt.Sprintf("/etc/haproxy/conf.d/%s.cfg", ctx.Name)
	if err := o.sshClient.UploadContent(haproxyConfig, haproxyPath); err != nil {
		return fmt.Errorf("failed to upload HAProxy config: %w", err)
	}
	fmt.Printf("✓ HAProxy configuration created: %s\n", haproxyPath)

	// Enable and restart HAProxy to apply new configuration
	fmt.Println("Enabling and restarting HAProxy service...")
	if err := o.sshClient.SystemctlEnable("haproxy"); err != nil {
		fmt.Printf("Warning: Failed to enable HAProxy (may already be enabled): %v\n", err)
	}
	if err := o.sshClient.SystemctlRestart("haproxy"); err != nil {
		return fmt.Errorf("failed to restart HAProxy: %w", err)
	}
	fmt.Println("✓ HAProxy service enabled and restarted successfully")

	// Right before 'return nil' at the bottom of the function:
	haproxyPath = fmt.Sprintf("/etc/haproxy/conf.d/%s.cfg", ctx.Name)
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, haproxyPath)
	o.saveState(ctx) // Persist the tracking

	fmt.Println("✓ HAProxy configured")
	return nil
}

// ============================================================================
// OPENSHIFT PREPARATION PHASES
// ============================================================================

// phaseDownloadImages downloads RHCOS images and OpenShift tools
// Files are downloaded to /var/www/html/<clustername>/rhcos/ (managed by setup_http phase)
func (o *Orchestrator) phaseDownloadImages(ctx *ClusterContext) error {
	fmt.Println("Downloading RHCOS images and OpenShift tools...")

	downloader := services.NewDownloader(ctx, o.sshClient)

	// Download RHCOS images to HTTP directory
	if err := downloader.DownloadRHCOSImages(); err != nil {
		return fmt.Errorf("failed to download RHCOS images: %w", err)
	}

	// Download OpenShift tools
	if err := downloader.DownloadOpenShiftTools(); err != nil {
		return fmt.Errorf("failed to download OpenShift tools: %w", err)
	}

	fmt.Println("✓ Images and tools downloaded to HTTP directory")
	return nil
}

// phaseGenerateIgnition generates ignition configurations
func (o *Orchestrator) phaseGenerateIgnition(ctx *ClusterContext) error {
	fmt.Println("Generating ignition configurations...")

	httpServer := services.NewHTTPServerManager(ctx, o.sshClient)
	ignition := services.NewIgnitionGenerator(ctx, o.sshClient, httpServer)
	if err := ignition.Generate(); err != nil {
		return fmt.Errorf("failed to generate ignition configs: %w", err)
	}

	// Track the working directory for cleanup
	workDir := fmt.Sprintf("/root/ocp4-%s", ctx.Name)
	ctx.State.HelperFiles = append(ctx.State.HelperFiles, workDir)
	o.saveState(ctx) // Persist the tracking

	fmt.Println("✓ Ignition configurations generated")
	return nil
}

// ============================================================================
// LPAR PROVISIONING PHASES
// ============================================================================

// phaseCreateLPARs creates LPARs for cluster nodes
func (o *Orchestrator) phaseCreateLPARs(ctx *ClusterContext) error {
	fmt.Println("Creating LPARs...")

	lparProvisioner := infrastructure.NewLPARProvisioner(ctx, o.hmcClient)
	if err := lparProvisioner.ProvisionAll(); err != nil {
		return fmt.Errorf("failed to create LPARs: %w", err)
	}

	fmt.Println("✓ LPARs created")
	return nil
}

// phasePowerOn powers on all LPARs
func (o *Orchestrator) phasePowerOn(ctx *ClusterContext) error {
	fmt.Println("Powering on LPARs...")

	lparProvisioner := infrastructure.NewLPARProvisioner(ctx, o.hmcClient)
	if err := lparProvisioner.PowerOnAll(); err != nil {
		return fmt.Errorf("failed to power on LPARs: %w", err)
	}

	fmt.Println("✓ LPARs powered on")
	return nil
}

// ============================================================================
// INSTALLATION WAIT PHASES
// ============================================================================

// phaseWaitBootstrap waits for OpenShift bootstrap to complete
func (o *Orchestrator) phaseWaitBootstrap(ctx *ClusterContext) error {
	fmt.Println("Waiting for OpenShift bootstrap to complete...")

	// Working directory where ignition files and auth/kubeconfig are located
	workDir := fmt.Sprintf("/root/ocp4-%s", ctx.Name)

	// Path to openshift-install binary
	openshiftInstallPath := fmt.Sprintf("/var/www/html/%s/tools/openshift-install", ctx.Name)

	// Wait for bootstrap complete
	if err := o.waitForBootstrapComplete(ctx, workDir, openshiftInstallPath); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	fmt.Println("✓ Bootstrap phase completed successfully")
	return nil
}

// phaseWaitInstallation waits for OpenShift installation to complete
func (o *Orchestrator) phaseWaitInstallation(ctx *ClusterContext) error {
	fmt.Println("Waiting for OpenShift installation to complete...")

	// Working directory where ignition files and auth/kubeconfig are located
	workDir := fmt.Sprintf("/root/ocp4-%s", ctx.Name)

	// Path to openshift-install binary
	openshiftInstallPath := fmt.Sprintf("/var/www/html/%s/tools/openshift-install", ctx.Name)

	// Wait for installation complete
	if err := o.waitForInstallComplete(ctx, workDir, openshiftInstallPath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Save kubeconfig if configured
	if ctx.ClusterConfig.Advanced.SaveKubeconfig {
		if err := o.saveKubeconfig(ctx, workDir); err != nil {
			fmt.Printf("Warning: Failed to save kubeconfig: %v\n", err)
		}
	}

	fmt.Println("✓ OpenShift installation completed successfully")
	return nil
}

// ============================================================================
// INSTALLATION WAIT HELPERS
// ============================================================================

// waitForBootstrapComplete waits for bootstrap to complete
func (o *Orchestrator) waitForBootstrapComplete(ctx *ClusterContext, workDir, openshiftInstallPath string) error {
	fmt.Println("\n=== Waiting for Bootstrap Complete ===")

	timeoutSecs := ctx.ClusterConfig.Deployment.Timeouts.BootstrapComplete
	if timeoutSecs == 0 {
		timeoutSecs = 1800 // Default 30 minutes
	}

	fmt.Printf("Timeout: %d seconds (%d minutes)\n", timeoutSecs, timeoutSecs/60)
	fmt.Println("This may take 20-30 minutes...")

	// Create a context with the designated timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("cd %s && %s wait-for bootstrap-complete --log-level=info 2>&1",
		workDir, openshiftInstallPath)

	fmt.Println("Executing: openshift-install wait-for bootstrap-complete")
	output, err := o.sshClient.ExecuteCommandCtx(timeoutCtx, cmd)
	if err != nil {
		fmt.Printf("\n❌ Bootstrap failed:\n%s\n", output)
		return fmt.Errorf("bootstrap completion failed: %w", err)
	}

	fmt.Printf("\n✓ Bootstrap Complete!\n%s\n", output)

	if ctx.ClusterConfig.Bootstrap != nil {
		fmt.Println("\nNote: Bootstrap node can now be powered off or deleted")
		fmt.Println("  The cluster will continue installation without it")
	}

	return nil
}

// waitForInstallComplete waits for installation to complete
func (o *Orchestrator) waitForInstallComplete(ctx *ClusterContext, workDir, openshiftInstallPath string) error {
	fmt.Println("\n=== Waiting for Installation Complete ===")

	timeoutSecs := ctx.ClusterConfig.Deployment.Timeouts.InstallationComplete
	if timeoutSecs == 0 {
		timeoutSecs = 3600 // Default 60 minutes
	}

	fmt.Printf("Timeout: %d seconds (%d minutes)\n", timeoutSecs, timeoutSecs/60)

	if ctx.ClusterConfig.IsSNO() {
		fmt.Println("This may take 30-45 minutes for SNO...")
	} else {
		fmt.Println("This may take 30-60 minutes for multi-node cluster...")
	}

	// Create a context with the designated timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("cd %s && %s wait-for install-complete --log-level=info 2>&1",
		workDir, openshiftInstallPath)

	fmt.Println("Executing: openshift-install wait-for install-complete")
	output, err := o.sshClient.ExecuteCommandCtx(timeoutCtx, cmd)
	if err != nil {
		fmt.Printf("\n❌ Installation failed:\n%s\n", output)
		return fmt.Errorf("installation completion failed: %w", err)
	}

	fmt.Printf("\n✓ Installation Complete!\n%s\n", output)

	o.displayClusterInfo(ctx, output)

	return nil
}

// ============================================================================
// POST-INSTALLATION HELPERS
// ============================================================================

// saveKubeconfig saves the kubeconfig file to the specified location
func (o *Orchestrator) saveKubeconfig(ctx *ClusterContext, workDir string) error {
	fmt.Println("\nSaving kubeconfig...")

	kubeconfigPath := ctx.ClusterConfig.Advanced.KubeconfigPath
	if kubeconfigPath == "" {
		kubeconfigPath = fmt.Sprintf("./kubeconfig-%s", ctx.Name)
	}

	// Copy kubeconfig from helper node working directory
	remoteKubeconfig := fmt.Sprintf("%s/auth/kubeconfig", workDir)
	cmd := fmt.Sprintf("cat %s", remoteKubeconfig)

	kubeconfigContent, err := o.sshClient.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	// Write to local file
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	fmt.Printf("✓ Kubeconfig saved to: %s\n", kubeconfigPath)
	fmt.Printf("\nTo use the cluster:\n")
	fmt.Printf("  export KUBECONFIG=%s\n", kubeconfigPath)
	fmt.Printf("  oc get nodes\n")
	fmt.Printf("  oc get clusteroperators\n")

	return nil
}

// displayClusterInfo extracts and displays cluster information from install output
func (o *Orchestrator) displayClusterInfo(ctx *ClusterContext, output string) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("CLUSTER INFORMATION")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("Deployment Name: %s\n", ctx.Name)
	fmt.Printf("Base Domain:     %s\n", ctx.ClusterConfig.Network.BaseDomain)
	fmt.Printf("Cluster VIP:     %s\n", ctx.VIP)

	// Extract console URL and credentials from output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "console") ||
			strings.Contains(line, "kubeadmin") ||
			strings.Contains(line, "password") ||
			strings.Contains(line, "https://") {
			fmt.Println(line)
		}
	}

	fmt.Println(strings.Repeat("=", 70))

	// Display access information
	fmt.Println("\nAccess your cluster:")
	fmt.Printf("  Console URL: https://console-openshift-console.apps.%s.%s\n",
		ctx.Name, ctx.ClusterConfig.Network.BaseDomain)
	fmt.Printf("  API URL:     https://api.%s.%s:6443\n",
		ctx.Name, ctx.ClusterConfig.Network.BaseDomain)

	fmt.Println("\nUseful commands:")
	fmt.Println("  oc get nodes")
	fmt.Println("  oc get clusteroperators")
	fmt.Println("  oc get clusterversion")
	fmt.Println("  oc get pods --all-namespaces")
}

// Made with Bob
