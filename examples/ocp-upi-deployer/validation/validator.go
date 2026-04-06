package validation

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

// ============================================================================
// VALIDATOR STRUCT AND INITIALIZATION
// ============================================================================

// Validator validates configuration in three phases:
// 1. Config Validation (static, fast) - validates YAML structure and values
// 2. Helper Node Validation (SSH) - validates helper node infrastructure
// 3. HMC Validation (HMC API) - validates Power systems and VIOS configuration
type Validator struct {
	multiConfig   *MultiClusterConfig
	clusterConfig *ClusterConfig
	clusterName   string
	sshClient     *communication.SSHClient
	hmcClient     interface{} // HMC client for active validation (can be nil for static validation)
	verbose       bool
	errors        []string
	warnings      []string
}

// NewValidator creates a new validator
func NewValidator(multiConfig *MultiClusterConfig, clusterConfig *ClusterConfig, clusterName string, sshClient *communication.SSHClient, verbose bool) *Validator {
	return &Validator{
		multiConfig:   multiConfig,
		clusterConfig: clusterConfig,
		clusterName:   clusterName,
		sshClient:     sshClient,
		hmcClient:     nil, // Will be set separately if needed
		verbose:       verbose,
		errors:        []string{},
		warnings:      []string{},
	}
}

// SetHMCClient sets the HMC client for active validation
func (v *Validator) SetHMCClient(client interface{}) {
	v.hmcClient = client
}

// Validate performs comprehensive validation in three phases
func (v *Validator) Validate() error {
	// Phase 1: Config Validation (static, fast)
	if v.verbose {
		fmt.Println("  Phase 1: Validating configuration...")
	}
	v.validateHelperNode()
	v.validateHMC()
	v.validateVIPs()

	if v.clusterConfig != nil {
		v.validatePowerSystems()
		v.validateStorage()
		v.validateNetwork()
		v.validateOpenShift()
		v.validateNodes()
		v.validateDeployment()
	}
	if v.verbose {
		fmt.Println("  ✓ Configuration valid")
	}

	// Phase 2: Helper Node Validation (SSH-based)
	if v.sshClient != nil && v.clusterName != "" {
		if v.verbose {
			fmt.Println("  Phase 2: Validating helper node...")
		}
		v.validateRemoteEnvironment()
		if v.verbose {
			fmt.Println("  ✓ Helper node validated")
		}
	}

	// Phase 3: HMC Validation (HMC API-based)
	if v.hmcClient != nil && v.clusterConfig != nil && v.clusterConfig.Storage.Type == "vios" {
		if v.verbose {
			fmt.Println("  Phase 3: Validating HMC infrastructure...")
		}
		v.validateVIOSConfiguration()
		if v.verbose {
			fmt.Println("  ✓ HMC infrastructure validated")
		}
	}

	// Print warnings
	if len(v.warnings) > 0 {
		fmt.Println("\n⚠️  Warnings:")
		for _, w := range v.warnings {
			fmt.Printf("   - %s\n", w)
		}
	}

	// Return errors if any
	if len(v.errors) > 0 {
		errMsg := "\n❌ Validation Errors:\n"
		for _, e := range v.errors {
			errMsg += fmt.Sprintf("   - %s\n", e)
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// ============================================================================
// PHASE 1: CONFIG VALIDATION (Static, Fast)
// ============================================================================
// These methods validate configuration structure and values without making
// any external connections. They run in milliseconds and catch common errors
// like missing fields, invalid IPs, or malformed configuration.
// ============================================================================

// validateVIPs ensures no two clusters are assigned the same VIP and VIPs are valid
func (v *Validator) validateVIPs() {
	usedVIPs := make(map[string]string)
	for _, cluster := range v.multiConfig.Clusters {
		if cluster.VIP == "" {
			v.errors = append(v.errors, fmt.Sprintf("cluster '%s' is missing a VIP assignment", cluster.Name))
			continue
		}

		if !v.isValidIP(cluster.VIP) {
			v.errors = append(v.errors, fmt.Sprintf("VIP '%s' for cluster '%s' is not a valid IP", cluster.VIP, cluster.Name))
		}

		if existingCluster, exists := usedVIPs[cluster.VIP]; exists {
			v.errors = append(v.errors, fmt.Sprintf("VIP conflict: IP %s is assigned to both '%s' and '%s'", cluster.VIP, existingCluster, cluster.Name))
		} else {
			usedVIPs[cluster.VIP] = cluster.Name
		}
	}
}

// validateHelperNode validates helper node configuration
func (v *Validator) validateHelperNode() {
	h := v.multiConfig.HelperNode

	if h.Hostname == "" {
		v.errors = append(v.errors, "helper_node.hostname is required")
	}

	if h.IP == "" {
		v.errors = append(v.errors, "helper_node.ip is required")
	} else if !v.isValidIP(h.IP) {
		v.errors = append(v.errors, fmt.Sprintf("helper_node.ip '%s' is not a valid IP address", h.IP))
	}

	if h.SSHUser == "" {
		v.errors = append(v.errors, "helper_node.ssh_user is required")
	}

	if h.SSHKeyFile == "" {
		v.errors = append(v.errors, "helper_node.ssh_key_file is required")
	} else {
		expandedPath := os.ExpandEnv(strings.ReplaceAll(h.SSHKeyFile, "~", "$HOME"))
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			v.errors = append(v.errors, fmt.Sprintf("helper_node.ssh_key_file '%s' does not exist", h.SSHKeyFile))
		}
	}

	if h.NetworkInterface == "" {
		v.errors = append(v.errors, "helper_node.network_interface is required")
	}
}

// validateHMC validates HMC configuration
func (v *Validator) validateHMC() {
	h := v.multiConfig.HMC

	if h.IP == "" {
		v.errors = append(v.errors, "hmc.ip is required")
	} else if !v.isValidIP(h.IP) {
		v.errors = append(v.errors, fmt.Sprintf("hmc.ip '%s' is not a valid IP address", h.IP))
	}

	if h.Username == "" {
		v.errors = append(v.errors, "hmc.username is required")
	}

	if h.Password == "" {
		v.errors = append(v.errors, "hmc.password is required")
	}
}

// validatePowerSystems validates Power systems configuration
func (v *Validator) validatePowerSystems() {
	if len(v.clusterConfig.PowerSystems) == 0 {
		v.errors = append(v.errors, "at least one power_system is required")
		return
	}

	for i, ps := range v.clusterConfig.PowerSystems {
		if ps.Name == "" {
			v.errors = append(v.errors, fmt.Sprintf("power_systems[%d].name is required", i))
		}
		if ps.VswitchName == "" {
			v.errors = append(v.errors, fmt.Sprintf("power_systems[%d].vswitch_name is required", i))
		}
	}
}

// validateStorage validates storage configuration
func (v *Validator) validateStorage() {
	s := v.clusterConfig.Storage

	if s.Type == "" {
		v.errors = append(v.errors, "storage.type is required (vios or svc)")
		return
	}

	if s.Type != "vios" && s.Type != "svc" {
		v.errors = append(v.errors, fmt.Sprintf("storage.type must be 'vios' or 'svc', got '%s'", s.Type))
	}

	if s.Type == "vios" && len(s.VIOS) == 0 {
		v.errors = append(v.errors, "storage.vios is required when storage.type is 'vios'")
	}

	if s.Type == "svc" && s.SVC == nil {
		v.errors = append(v.errors, "storage.svc is required when storage.type is 'svc'")
	}

	// Check for explicit rootvg rejection (config validation)
	if s.Type == "vios" && len(s.VIOS) > 0 {
		viosConfig := s.VIOS[0]
		if strings.ToLower(viosConfig.VolumeGroup) == "rootvg" {
			v.errors = append(v.errors,
				"INVALID CONFIGURATION: 'rootvg' cannot be used for client LPAR storage.\n"+
					"   'rootvg' is reserved for VIOS system use only.\n"+
					"   Please specify a different volume group or leave it empty for auto-discovery.\n"+
					"   To create a dedicated volume group, run on the VIOS:\n"+
					"   mkvg -f -s 256 <physical_volume_name>")
		}
	}

	// Note: Active VIOS validation is performed in Phase 3 if HMC client is available
}

// validateAutoDiscovery validates that at least one active VIOS has non-rootvg volume groups
func (v *Validator) validateAutoDiscovery(hmcClient *hmc.HmcRestClient, systemUUID string) error {
	viosServers, err := hmcClient.GetVirtualIOServersQuick(systemUUID, v.verbose)
	if err != nil {
		return fmt.Errorf("failed to get VIOS list: %w", err)
	}

	if len(viosServers) == 0 {
		return fmt.Errorf("no VIOS found on system")
	}

	// Check each active VIOS for non-rootvg volume groups
	for _, vios := range viosServers {
		if vios.PartitionState != "running" || vios.RMCState != "active" {
			continue
		}

		// Check volume groups for this VIOS
		vgs, err := hmcClient.GetVolumeGroups(vios.UUID, v.verbose)
		if err != nil {
			continue
		}

		hasNonRootvg := false
		for _, vg := range vgs {
			if strings.ToLower(vg.GroupName) != "rootvg" {
				hasNonRootvg = true
				break
			}
		}

		if hasNonRootvg {
			// Found at least one VIOS with non-rootvg VGs
			return nil
		}
	}

	return fmt.Errorf("VIOS CONFIGURATION ERROR: No active VIOS found with non-rootvg volume groups.\n" +
		"   All discovered VIOS only have 'rootvg', which cannot be used for client LPAR storage.\n" +
		"   Please create a dedicated volume group on at least one VIOS:\n" +
		"   mkvg -f -s 256 <physical_volume_name>")
}

// validateSpecificVIOS validates that a specific VIOS has non-rootvg volume groups
func (v *Validator) validateSpecificVIOS(hmcClient *hmc.HmcRestClient, systemUUID, viosName string) error {
	viosServers, err := hmcClient.GetVirtualIOServersQuick(systemUUID, v.verbose)
	if err != nil {
		return fmt.Errorf("failed to get VIOS list: %w", err)
	}

	var targetVIOSUUID string
	for _, vios := range viosServers {
		if vios.PartitionName == viosName {
			targetVIOSUUID = vios.UUID
			break
		}
	}

	if targetVIOSUUID == "" {
		return fmt.Errorf("specified VIOS '%s' not found on system", viosName)
	}

	// Get volume groups for this VIOS
	vgs, err := hmcClient.GetVolumeGroups(targetVIOSUUID, v.verbose)
	if err != nil {
		return fmt.Errorf("failed to get volume groups for VIOS '%s': %w", viosName, err)
	}

	var availableVGs []string
	hasNonRootvg := false
	for _, vg := range vgs {
		availableVGs = append(availableVGs, vg.GroupName)
		if strings.ToLower(vg.GroupName) != "rootvg" {
			hasNonRootvg = true
		}
	}

	if !hasNonRootvg {
		return fmt.Errorf("VIOS CONFIGURATION ERROR: VIOS '%s' only has 'rootvg' volume group.\n"+
			"   'rootvg' cannot be used for client LPAR storage.\n"+
			"   Available volume groups: %v\n"+
			"   Please create a dedicated volume group on this VIOS:\n"+
			"   mkvg -f -s 256 <physical_volume_name>",
			viosName, availableVGs)
	}

	return nil
}

// validateNetwork validates network configuration
func (v *Validator) validateNetwork() {
	n := v.clusterConfig.Network

	if n.Domain == "" {
		v.errors = append(v.errors, "network.domain is required")
	}

	// Note: ClusterName is now derived from the deployment name, no longer a separate field

	if n.BaseDomain == "" {
		v.errors = append(v.errors, "network.base_domain is required")
	}

	if n.NetworkCIDR == "" {
		v.errors = append(v.errors, "network.network_cidr is required")
	} else if !v.isValidCIDR(n.NetworkCIDR) {
		v.errors = append(v.errors, fmt.Sprintf("network.network_cidr '%s' is not a valid CIDR", n.NetworkCIDR))
	}

	if n.Gateway == "" {
		v.errors = append(v.errors, "network.gateway is required")
	} else if !v.isValidIP(n.Gateway) {
		v.errors = append(v.errors, fmt.Sprintf("network.gateway '%s' is not a valid IP address", n.Gateway))
	}

	if n.Nameserver == "" {
		v.errors = append(v.errors, "network.nameserver is required")
	} else if !v.isValidIP(n.Nameserver) {
		v.errors = append(v.errors, fmt.Sprintf("network.nameserver '%s' is not a valid IP address", n.Nameserver))
	}
}

// validateOpenShift validates OpenShift configuration
func (v *Validator) validateOpenShift() {
	o := v.clusterConfig.OpenShift

	if o.Version == "" {
		v.errors = append(v.errors, "openshift.version is required")
	}

	if o.PullSecretFile == "" {
		v.errors = append(v.errors, "openshift.pull_secret_file is required")
	} else {
		if _, err := os.Stat(o.PullSecretFile); os.IsNotExist(err) {
			v.errors = append(v.errors, fmt.Sprintf("openshift.pull_secret_file '%s' does not exist", o.PullSecretFile))
		}
	}

	if o.SSHPublicKeyFile == "" {
		v.errors = append(v.errors, "openshift.ssh_public_key_file is required")
	} else {
		expandedPath := os.ExpandEnv(strings.ReplaceAll(o.SSHPublicKeyFile, "~", "$HOME"))
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			v.errors = append(v.errors, fmt.Sprintf("openshift.ssh_public_key_file '%s' does not exist", o.SSHPublicKeyFile))
		}
	}

	if o.InstallType == "" {
		v.errors = append(v.errors, "openshift.install_type is required (sno or multi-node)")
	} else if o.InstallType != "sno" && o.InstallType != "multi-node" {
		v.errors = append(v.errors, fmt.Sprintf("openshift.install_type must be 'sno' or 'multi-node', got '%s'", o.InstallType))
	}

	// Validate RHCOS URLs
	if o.RHCOSImages.KernelURL == "" {
		v.errors = append(v.errors, "openshift.rhcos_images.kernel_url is required")
	}
	if o.RHCOSImages.InitramfsURL == "" {
		v.errors = append(v.errors, "openshift.rhcos_images.initramfs_url is required")
	}
	if o.RHCOSImages.RootfsURL == "" {
		v.errors = append(v.errors, "openshift.rhcos_images.rootfs_url is required")
	}

	// Validate OCP client config
	if o.OCPClientConfig.Client == "" {
		v.errors = append(v.errors, "openshift.ocp_client_config.ocp_client is required")
	}
	if o.OCPClientConfig.Installer == "" {
		v.errors = append(v.errors, "openshift.ocp_client_config.ocp_installer is required")
	}
}

// validateNodes validates node configuration
func (v *Validator) validateNodes() {
	if v.clusterConfig.IsSNO() {
		v.validateSNONode()
	} else {
		v.validateMultiNodeCluster()
	}
}

// validateSNONode validates SNO node configuration
func (v *Validator) validateSNONode() {
	if v.clusterConfig.SNONode == nil {
		v.errors = append(v.errors, "sno_node is required for SNO deployment")
		return
	}

	sno := v.clusterConfig.SNONode

	// Auto-populate hostname from cluster name if not provided
	if sno.Hostname == "" {
		sno.Hostname = v.clusterName
		v.clusterConfig.SNONode.Hostname = v.clusterName
	}

	// Auto-populate name from hostname if not provided
	if sno.Name == "" {
		sno.Name = sno.Hostname + "-master"
		v.clusterConfig.SNONode.Name = sno.Name
	}
	if sno.IP == "" {
		v.errors = append(v.errors, "sno_node.ip is required")
	} else if !v.isValidIP(sno.IP) {
		v.errors = append(v.errors, fmt.Sprintf("sno_node.ip '%s' is not a valid IP address", sno.IP))
	}
	if sno.SystemName == "" {
		v.errors = append(v.errors, "sno_node.system_name is required")
	}

	v.validateLPARConfig("sno_node", sno.LPAR)
}

// validateMultiNodeCluster validates multi-node cluster configuration
func (v *Validator) validateMultiNodeCluster() {
	if v.clusterConfig.Masters == nil || len(v.clusterConfig.Masters.Nodes) == 0 {
		v.errors = append(v.errors, "masters.nodes is required for multi-node deployment")
		return
	}

	// Validate master count (must be odd for quorum)
	masterCount := len(v.clusterConfig.Masters.Nodes)
	if masterCount < 3 {
		v.errors = append(v.errors, fmt.Sprintf("minimum 3 master nodes required, got %d", masterCount))
	}
	if masterCount%2 == 0 {
		v.errors = append(v.errors, fmt.Sprintf("master count must be odd for quorum, got %d", masterCount))
	}

	// Validate each master node
	for i, master := range v.clusterConfig.Masters.Nodes {
		if master.Name == "" {
			v.errors = append(v.errors, fmt.Sprintf("masters.nodes[%d].name is required", i))
		}
		if master.IP == "" {
			v.errors = append(v.errors, fmt.Sprintf("masters.nodes[%d].ip is required", i))
		} else if !v.isValidIP(master.IP) {
			v.errors = append(v.errors, fmt.Sprintf("masters.nodes[%d].ip '%s' is not a valid IP", i, master.IP))
		}
	}

	v.validateLPARConfig("masters", v.clusterConfig.Masters.LPAR)

	// Validate workers if present
	if v.clusterConfig.Workers != nil && len(v.clusterConfig.Workers.Nodes) > 0 {
		for i, worker := range v.clusterConfig.Workers.Nodes {
			if worker.Name == "" {
				v.errors = append(v.errors, fmt.Sprintf("workers.nodes[%d].name is required", i))
			}
			if worker.IP == "" {
				v.errors = append(v.errors, fmt.Sprintf("workers.nodes[%d].ip is required", i))
			} else if !v.isValidIP(worker.IP) {
				v.errors = append(v.errors, fmt.Sprintf("workers.nodes[%d].ip '%s' is not a valid IP", i, worker.IP))
			}
		}
		v.validateLPARConfig("workers", v.clusterConfig.Workers.LPAR)
	}
}

// validateLPARConfig validates LPAR configuration
func (v *Validator) validateLPARConfig(prefix string, lpar LPARConfig) {
	if lpar.Processor.Units <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s.lpar.processor.units must be positive", prefix))
	}
	if lpar.Processor.VirtualProcs <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s.lpar.processor.virtual_procs must be positive", prefix))
	}
	if lpar.Memory.DesiredMB <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s.lpar.memory.desired_mb must be positive", prefix))
	}
	if lpar.Storage.BootDiskGB <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s.lpar.storage.boot_disk_gb must be positive", prefix))
	}
}

// validateDeployment validates deployment configuration
func (v *Validator) validateDeployment() {
	d := v.clusterConfig.Deployment

	if len(d.Phases) == 0 {
		v.warnings = append(v.warnings, "deployment.phases is empty, using default phases")
	}

	if d.Timeouts.LPARCreation <= 0 {
		v.warnings = append(v.warnings, "deployment.timeouts.lpar_creation should be positive")
	}
}

// ============================================================================
// PHASE 2: HELPER NODE VALIDATION (SSH-Based)
// ============================================================================
// These methods validate helper node infrastructure by making SSH connections
// to check for directory conflicts, VIP conflicts, and other environment issues.
// ============================================================================

// validateRemoteEnvironment runs active SSH checks against the helper node infrastructure
func (v *Validator) validateRemoteEnvironment() {
	if v.clusterConfig == nil {
		return
	}

	// 0. Test SSH connectivity to helper node
	if _, err := v.sshClient.ExecuteCommand("echo 'connected'"); err != nil {
		v.errors = append(v.errors, fmt.Sprintf("HELPER NODE UNREACHABLE: Unable to connect to helper node via SSH: %v", err))
		return // Skip remaining checks if helper node is unreachable
	}

	// 1. Check for sufficient disk space on helper node (must have at least 10GB free)
	v.validateHelperNodeDiskSpace()

	// 2. Check for Directory/Config Collisions on Helper Node
	httpDir := fmt.Sprintf("/var/www/html/%s", v.clusterName)
	dnsmasqPath := fmt.Sprintf("/etc/dnsmasq.d/%s.conf", v.clusterName)
	haproxyPath := fmt.Sprintf("/etc/haproxy/conf.d/%s.cfg", v.clusterName)

	checkCmd := fmt.Sprintf("if [ -d '%s' ] || [ -f '%s' ] || [ -f '%s' ]; then echo 'exists'; else echo 'missing'; fi",
		httpDir, dnsmasqPath, haproxyPath)

	if out, err := v.sshClient.ExecuteCommand(checkCmd); err == nil && strings.TrimSpace(out) == "exists" {
		v.errors = append(v.errors, fmt.Sprintf("CLUSTER COLLISION: Artifacts for '%s' already exist on the helper node. Run the 'delete' command to clean them up first to prevent accidental overwrites.", v.clusterName))
	}

	// 3. Check for VIP Conflicts on the network
	var vip string
	for _, c := range v.multiConfig.Clusters {
		if c.Name == v.clusterName {
			vip = c.VIP
			break
		}
	}

	if vip != "" && v.clusterConfig.Advanced.CheckNetworkConnectivity {
		iface := v.multiConfig.HelperNode.NetworkInterface
		checkBoundCmd := fmt.Sprintf("sudo ip addr show dev %s | grep -q '%s/'", iface, vip)
		if _, err := v.sshClient.ExecuteCommand(checkBoundCmd); err != nil {
			// Not bound to us. Ping the network.
			pingCmd := fmt.Sprintf("ping -c 2 -W 2 %s", vip)
			if _, pingErr := v.sshClient.ExecuteCommand(pingCmd); pingErr == nil {
				v.errors = append(v.errors, fmt.Sprintf("IP CONFLICT: The VIP %s is already actively responding on the network. Please choose an unused IP.", vip))
			}
		}
	}
}

// validateHelperNodeDiskSpace checks if /var/www/html has at least 10GB free space
func (v *Validator) validateHelperNodeDiskSpace() {
	// Use df command to get available space in KB for /var/www/html
	// -BK outputs in KB, --output=avail gets only available space
	dfCmd := "df -BK --output=avail /var/www/html | tail -n 1 | tr -d 'K'"

	output, err := v.sshClient.ExecuteCommand(dfCmd)
	if err != nil {
		v.warnings = append(v.warnings, fmt.Sprintf("Unable to check disk space on helper node: %v", err))
		return
	}

	// Parse available space in KB
	var availableKB int
	trimmedOutput := strings.TrimSpace(output)
	if _, err := fmt.Sscanf(trimmedOutput, "%d", &availableKB); err != nil {
		v.warnings = append(v.warnings, fmt.Sprintf("Unable to parse disk space output '%s': %v", trimmedOutput, err))
		return
	}

	// Convert to GB (1 GB = 1024 * 1024 KB)
	availableGB := float64(availableKB) / (1024 * 1024)
	requiredGB := 10.0

	if availableGB < requiredGB {
		v.errors = append(v.errors,
			fmt.Sprintf("INSUFFICIENT DISK SPACE: /var/www/html has only %.2f GB available, but at least %.0f GB is required.\n"+
				"   OpenShift installation requires significant space for RHCOS images, ignition configs, and other artifacts.\n"+
				"   Please free up disk space on the helper node before proceeding.",
				availableGB, requiredGB))
	} else if v.verbose {
		fmt.Printf("  ✓ Helper node has %.2f GB available in /var/www/html\n", availableGB)
	}
}

// ============================================================================
// PHASE 3: HMC VALIDATION (HMC API-Based)
// ============================================================================
// These methods validate HMC infrastructure by making HMC REST API calls to
// verify Power systems, VIOS servers, and volume group availability.
// ============================================================================

// validateVIOSConfiguration performs active validation of VIOS configuration
func (v *Validator) validateVIOSConfiguration() {
	if len(v.clusterConfig.Storage.VIOS) == 0 {
		return
	}

	viosConfig := v.clusterConfig.Storage.VIOS[0]

	// Scenario 1: Explicit rootvg rejection (already checked in Phase 1)
	if strings.ToLower(viosConfig.VolumeGroup) == "rootvg" {
		return // Already caught in config validation
	}

	// Type assert to HMC client
	hmcClient, ok := v.hmcClient.(*hmc.HmcRestClient)
	if !ok {
		v.warnings = append(v.warnings, "HMC client type assertion failed, skipping active VIOS validation")
		return
	}

	// Get system UUID
	systemUUID, _, err := hmcClient.GetManagedSystemByName(viosConfig.SystemName, v.verbose)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("failed to get managed system '%s': %v", viosConfig.SystemName, err))
		return
	}

	// Scenario 2 & 3: Validate VIOS and volume group configuration
	if viosConfig.VIOSName == "" && viosConfig.VolumeGroup == "" {
		// Full auto-discovery - validate at least one VIOS has non-rootvg VGs
		if err := v.validateAutoDiscovery(hmcClient, systemUUID); err != nil {
			v.errors = append(v.errors, err.Error())
		}
	} else if viosConfig.VIOSName != "" && viosConfig.VolumeGroup == "" {
		// Partial auto-discovery - validate specific VIOS has non-rootvg VGs
		if err := v.validateSpecificVIOS(hmcClient, systemUUID, viosConfig.VIOSName); err != nil {
			v.errors = append(v.errors, err.Error())
		}
	}
	// If both are specified and VG != rootvg, no additional validation needed
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// isValidIP checks if a string is a valid IP address
func (v *Validator) isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// isValidCIDR checks if a string is a valid CIDR notation
func (v *Validator) isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// isValidHostname checks if a string is a valid hostname
func (v *Validator) isValidHostname(hostname string) bool {
	// Simple hostname validation
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(hostname)
}
