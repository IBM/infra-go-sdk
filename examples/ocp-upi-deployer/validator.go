package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Validator performs comprehensive validation of the deployment configuration
type Validator struct {
	config *Config
	errors []string
	warnings []string
}

// NewValidator creates a new validator instance
func NewValidator(config *Config) *Validator {
	return &Validator{
		config: config,
		errors: []string{},
		warnings: []string{},
	}
}

// Validate performs all validation checks
func (v *Validator) Validate() error {
	v.validateHMC()
	v.validatePowerSystems()
	v.validateStorage()
	v.validateNetwork()
	v.validateOpenShift()
	v.validateNodes()
	v.validateResourceDistribution()
	v.validateDeployment()
	
	if len(v.errors) > 0 {
		return fmt.Errorf("validation failed with %d errors:\n%s", 
			len(v.errors), strings.Join(v.errors, "\n"))
	}
	
	if len(v.warnings) > 0 {
		fmt.Printf("⚠️  Validation completed with %d warnings:\n%s\n\n", 
			len(v.warnings), strings.Join(v.warnings, "\n"))
	}
	
	return nil
}

// validateHMC validates HMC configuration
func (v *Validator) validateHMC() {
	if v.config.HMC.IP == "" {
		v.errors = append(v.errors, "HMC IP address is required")
	} else if !v.isValidIP(v.config.HMC.IP) {
		v.errors = append(v.errors, fmt.Sprintf("HMC IP '%s' is not valid", v.config.HMC.IP))
	}
	
	if v.config.HMC.Username == "" {
		v.errors = append(v.errors, "HMC username is required")
	}
	
	if v.config.HMC.Password == "" {
		v.errors = append(v.errors, "HMC password is required")
	}
}

// validatePowerSystems validates Power systems configuration
func (v *Validator) validatePowerSystems() {
	if len(v.config.PowerSystems) == 0 {
		v.errors = append(v.errors, "at least one Power system is required")
		return
	}
	
	systemNames := make(map[string]bool)
	for i, sys := range v.config.PowerSystems {
		if sys.Name == "" {
			v.errors = append(v.errors, fmt.Sprintf("Power system #%d: name is required", i+1))
		} else if systemNames[sys.Name] {
			v.errors = append(v.errors, fmt.Sprintf("duplicate Power system name: %s", sys.Name))
		} else {
			systemNames[sys.Name] = true
		}
		
		if sys.VswitchName == "" {
			v.errors = append(v.errors, fmt.Sprintf("Power system '%s': vswitch_name is required", sys.Name))
		}
		
		if sys.VlanID <= 0 || sys.VlanID > 4094 {
			v.errors = append(v.errors, fmt.Sprintf("Power system '%s': vlan_id must be between 1 and 4094", sys.Name))
		}
	}
}

// validateStorage validates storage configuration
func (v *Validator) validateStorage() {
	switch v.config.Storage.Type {
	case "svc":
		if v.config.Storage.SVC.IP == "" {
			v.errors = append(v.errors, "SVC IP address is required")
		}
		if v.config.Storage.SVC.Username == "" {
			v.errors = append(v.errors, "SVC username is required")
		}
		if v.config.Storage.SVC.PoolName == "" {
			v.errors = append(v.errors, "SVC pool name is required")
		}
		if v.config.Storage.SVC.VolumePrefix == "" {
			v.warnings = append(v.warnings, "SVC volume prefix is empty, volumes will have no prefix")
		}
		
	case "vios":
		if len(v.config.Storage.VIOS) == 0 {
			v.errors = append(v.errors, "at least one VIOS configuration is required for VIOS storage type")
		}
		for i, vios := range v.config.Storage.VIOS {
			if vios.SystemName == "" {
				v.errors = append(v.errors, fmt.Sprintf("VIOS #%d: system_name is required", i+1))
			}
			if vios.VIOSName == "" {
				v.errors = append(v.errors, fmt.Sprintf("VIOS #%d: vios_name is required", i+1))
			}
			if vios.VolumeGroup == "" {
				v.errors = append(v.errors, fmt.Sprintf("VIOS #%d: volume_group is required", i+1))
			}
		}
		
	case "physical":
		v.warnings = append(v.warnings, "physical storage type selected - ensure physical disks are pre-allocated")
		
	default:
		v.errors = append(v.errors, fmt.Sprintf("invalid storage type '%s', must be 'svc', 'vios', or 'physical'", v.config.Storage.Type))
	}
}

// validateNetwork validates network configuration
func (v *Validator) validateNetwork() {
	if v.config.Network.Domain == "" {
		v.errors = append(v.errors, "network domain is required")
	}
	
	if v.config.Network.ClusterName == "" {
		v.errors = append(v.errors, "cluster name is required")
	}
	
	if !v.isValidCIDR(v.config.Network.NetworkCIDR) {
		v.errors = append(v.errors, fmt.Sprintf("invalid network CIDR: %s", v.config.Network.NetworkCIDR))
	}
	
	if !v.isValidIP(v.config.Network.Gateway) {
		v.errors = append(v.errors, fmt.Sprintf("invalid gateway IP: %s", v.config.Network.Gateway))
	}
	
	if !v.isValidIP(v.config.Network.Nameserver) {
		v.errors = append(v.errors, fmt.Sprintf("invalid nameserver IP: %s", v.config.Network.Nameserver))
	}
	
	// Validate MAC prefix if specified
	if v.config.Network.MACPrefix != "" {
		macPrefixRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}:){2}[0-9A-Fa-f]{2}$`)
		if !macPrefixRegex.MatchString(v.config.Network.MACPrefix) {
			v.errors = append(v.errors, fmt.Sprintf("invalid MAC prefix format: %s (expected XX:XX:XX)", v.config.Network.MACPrefix))
		}
	}
}

// validateOpenShift validates OpenShift configuration
func (v *Validator) validateOpenShift() {
	if v.config.OpenShift.Version == "" {
		v.errors = append(v.errors, "OpenShift version is required")
	}
	
	// Validate pull secret file
	v.validateFilePath("pull secret", v.config.OpenShift.PullSecretFile, true)
	
	// Validate SSH public key file
	v.validateFilePath("SSH public key", v.config.OpenShift.SSHPublicKeyFile, true)
	
	if !v.isValidCIDR(v.config.OpenShift.ClusterNetworkCIDR) {
		v.errors = append(v.errors, fmt.Sprintf("invalid cluster network CIDR: %s", v.config.OpenShift.ClusterNetworkCIDR))
	}
	
	if !v.isValidCIDR(v.config.OpenShift.ServiceNetwork) {
		v.errors = append(v.errors, fmt.Sprintf("invalid service network CIDR: %s", v.config.OpenShift.ServiceNetwork))
	}
	
	// Validate disk device
	if v.config.OpenShift.DiskDevice == "" {
		v.errors = append(v.errors, "disk device is required (e.g., /dev/sda)")
	} else if !strings.HasPrefix(v.config.OpenShift.DiskDevice, "/dev/") {
		v.errors = append(v.errors, fmt.Sprintf("disk device must start with /dev/ (got: %s)", v.config.OpenShift.DiskDevice))
	}
	
	// Validate install type
	if v.config.OpenShift.InstallType == "" {
		v.errors = append(v.errors, "install type is required")
	} else {
		validTypes := map[string]bool{"agent": true, "assisted": true, "sno": true, "normal": true}
		if !validTypes[v.config.OpenShift.InstallType] {
			v.errors = append(v.errors, fmt.Sprintf("invalid install type: %s (must be: agent, assisted, sno, or normal)", v.config.OpenShift.InstallType))
		}
	}
	
	// Validate RHCOS architecture
	if v.config.OpenShift.RHCOSArch != "" {
		validArch := map[string]bool{"ppc64le": true, "x86_64": true, "aarch64": true, "s390x": true}
		if !validArch[v.config.OpenShift.RHCOSArch] {
			v.errors = append(v.errors, fmt.Sprintf("invalid RHCOS architecture: %s (must be: ppc64le, x86_64, aarch64, or s390x)", v.config.OpenShift.RHCOSArch))
		}
	}
	
	// Validate RHCOS image URLs
	v.validateRHCOSURLs()
	
	// Validate OCP client/installer URLs
	v.validateOCPClientConfig()
}

// validateRHCOSURLs validates RHCOS image URLs
func (v *Validator) validateRHCOSURLs() {
	rhcos := v.config.OpenShift.RHCOSImages
	
	if rhcos.KernelURL == "" {
		v.errors = append(v.errors, "RHCOS kernel URL is required")
	} else if !v.isValidURL(rhcos.KernelURL) {
		v.errors = append(v.errors, fmt.Sprintf("invalid RHCOS kernel URL: %s", rhcos.KernelURL))
	}
	
	if rhcos.InitramfsURL == "" {
		v.errors = append(v.errors, "RHCOS initramfs URL is required")
	} else if !v.isValidURL(rhcos.InitramfsURL) {
		v.errors = append(v.errors, fmt.Sprintf("invalid RHCOS initramfs URL: %s", rhcos.InitramfsURL))
	}
	
	if rhcos.RootfsURL == "" {
		v.errors = append(v.errors, "RHCOS rootfs URL is required")
	} else if !v.isValidURL(rhcos.RootfsURL) {
		v.errors = append(v.errors, fmt.Sprintf("invalid RHCOS rootfs URL: %s", rhcos.RootfsURL))
	}
}

// validateOCPClientConfig validates OpenShift client/installer URLs
func (v *Validator) validateOCPClientConfig() {
	ocp := v.config.OpenShift.OCPClientConfig
	
	if ocp.Arch == "" {
		v.errors = append(v.errors, "OCP client architecture is required")
	}
	
	if ocp.BaseURL == "" {
		v.errors = append(v.errors, "OCP base URL is required")
	} else if !v.isValidURL(ocp.BaseURL) {
		v.errors = append(v.errors, fmt.Sprintf("invalid OCP base URL: %s", ocp.BaseURL))
	}
	
	if ocp.Base == "" {
		v.errors = append(v.errors, "OCP client base path is required")
	}
	
	if ocp.Tag == "" {
		v.errors = append(v.errors, "OCP client tag is required")
	}
	
	if ocp.Client == "" {
		v.errors = append(v.errors, "OCP client tarball URL is required")
	} else if !v.isValidURL(ocp.Client) {
		v.errors = append(v.errors, fmt.Sprintf("invalid OCP client URL: %s", ocp.Client))
	}
	
	if ocp.Installer == "" {
		v.errors = append(v.errors, "OCP installer tarball URL is required")
	} else if !v.isValidURL(ocp.Installer) {
		v.errors = append(v.errors, fmt.Sprintf("invalid OCP installer URL: %s", ocp.Installer))
	}
}

// validateNodes validates all node configurations
func (v *Validator) validateNodes() {
	// Validate helper node
	if v.config.HelperNode.Hostname == "" {
		v.errors = append(v.errors, "helper node: hostname is required")
	}
	if !v.isValidIP(v.config.HelperNode.IP) {
		v.errors = append(v.errors, fmt.Sprintf("helper node: invalid IP address '%s'", v.config.HelperNode.IP))
	}
	
	// SNO mode validation
	if v.config.IsSNO() {
		v.validateNode("sno", v.config.SNONode.Name, v.config.SNONode.Hostname,
			v.config.SNONode.IP, v.config.SNONode.SystemName, v.config.SNONode.LPAR)
		v.checkDuplicates()
		return
	}
	
	// HA mode validation
	// Validate bootstrap node
	if v.config.Bootstrap != nil {
		v.validateNode("bootstrap", v.config.Bootstrap.Name, v.config.Bootstrap.Hostname,
			v.config.Bootstrap.IP, v.config.Bootstrap.SystemName, v.config.Bootstrap.LPAR)
	}
	
	// Validate master nodes
	if v.config.Masters != nil {
		if len(v.config.Masters.Nodes) < 3 {
			v.errors = append(v.errors, fmt.Sprintf("minimum 3 master nodes required, got %d", len(v.config.Masters.Nodes)))
		} else if len(v.config.Masters.Nodes)%2 == 0 {
			v.errors = append(v.errors, fmt.Sprintf("master count must be odd for quorum, got %d", len(v.config.Masters.Nodes)))
		}
		
		for _, master := range v.config.Masters.Nodes {
			v.validateNode("master", master.Name, master.Hostname, master.IP, master.SystemName, v.config.Masters.LPAR)
		}
	}
	
	// Validate worker nodes
	if v.config.Workers != nil {
		if len(v.config.Workers.Nodes) < 2 {
			v.warnings = append(v.warnings, fmt.Sprintf("minimum 2 worker nodes recommended, got %d", len(v.config.Workers.Nodes)))
		}
		
		for _, worker := range v.config.Workers.Nodes {
			v.validateNode("worker", worker.Name, worker.Hostname, worker.IP, worker.SystemName, v.config.Workers.LPAR)
		}
	}
	
	// Check for duplicate IPs and names
	v.checkDuplicates()
}

// validateNode validates a single node configuration
func (v *Validator) validateNode(role, name, hostname, ip, systemName string, lpar LPARConfig) {
	if name == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s node: name is required", role))
	}
	
	if hostname == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': hostname is required", role, name))
	}
	
	if !v.isValidIP(ip) {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': invalid IP address '%s'", role, name, ip))
	}
	
	if systemName == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': system_name is required", role, name))
	} else if _, err := v.config.GetPowerSystem(systemName); err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': references non-existent system '%s'", role, name, systemName))
	}
	
	// Validate LPAR configuration
	v.validateLPAR(role, name, lpar)
}

// validateLPAR validates LPAR resource configuration
func (v *Validator) validateLPAR(role, name string, lpar LPARConfig) {
	// Validate processor
	if lpar.Processor.Type != "shared" && lpar.Processor.Type != "dedicated" {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': processor type must be 'shared' or 'dedicated'", role, name))
	}
	
	if lpar.Processor.Units <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': processor units must be > 0", role, name))
	}
	
	if lpar.Processor.VirtualProcs <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': virtual processors must be > 0", role, name))
	}
	
	if lpar.Processor.MinUnits > lpar.Processor.Units {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': min processor units cannot exceed desired units", role, name))
	}
	
	if lpar.Processor.MaxUnits < lpar.Processor.Units {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': max processor units cannot be less than desired units", role, name))
	}
	
	// Validate memory
	if lpar.Memory.DesiredMB <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': desired memory must be > 0", role, name))
	}
	
	if lpar.Memory.MinMB > lpar.Memory.DesiredMB {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': min memory cannot exceed desired memory", role, name))
	}
	
	if lpar.Memory.MaxMB < lpar.Memory.DesiredMB {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': max memory cannot be less than desired memory", role, name))
	}
	
	// Validate storage
	if lpar.Storage.BootDiskGB <= 0 {
		v.errors = append(v.errors, fmt.Sprintf("%s node '%s': boot disk size must be > 0", role, name))
	}
	
	// Minimum recommendations
	if role == "master" && lpar.Memory.DesiredMB < 16384 {
		v.warnings = append(v.warnings, fmt.Sprintf("master node '%s': recommended minimum 16GB memory, configured %dMB", name, lpar.Memory.DesiredMB))
	}
	
	if role == "worker" && lpar.Memory.DesiredMB < 16384 {
		v.warnings = append(v.warnings, fmt.Sprintf("worker node '%s': recommended minimum 16GB memory, configured %dMB", name, lpar.Memory.DesiredMB))
	}
	
	if role == "sno" && lpar.Memory.DesiredMB < 32768 {
		v.warnings = append(v.warnings, fmt.Sprintf("SNO node '%s': recommended minimum 32GB memory, configured %dMB", name, lpar.Memory.DesiredMB))
	}
}

// checkDuplicates checks for duplicate node names and IPs
func (v *Validator) checkDuplicates() {
	names := make(map[string]string)
	ips := make(map[string]string)
	
	for _, node := range v.config.GetAllNodes() {
		if existingRole, exists := names[node.Name]; exists {
			v.errors = append(v.errors, fmt.Sprintf("duplicate node name '%s' used by %s and %s", node.Name, existingRole, node.Role))
		} else {
			names[node.Name] = node.Role
		}
		
		if existingRole, exists := ips[node.IP]; exists {
			v.errors = append(v.errors, fmt.Sprintf("duplicate IP address '%s' used by %s and %s nodes", node.IP, existingRole, node.Role))
		} else {
			ips[node.IP] = node.Role
		}
	}
}

// validateResourceDistribution validates resource distribution across systems
func (v *Validator) validateResourceDistribution() {
	nodesBySystem := v.config.GetNodesBySystem()
	
	for systemName, nodes := range nodesBySystem {
		system, _ := v.config.GetPowerSystem(systemName)
		
		if system.MaxLPARs > 0 && len(nodes) > system.MaxLPARs {
			v.warnings = append(v.warnings, fmt.Sprintf("system '%s': %d LPARs configured but max_lpars is %d", 
				systemName, len(nodes), system.MaxLPARs))
		}
		
		// Calculate total resources needed
		totalMemoryMB := 0
		totalProcessorUnits := 0.0
		
		for _, node := range nodes {
			totalMemoryMB += node.LPAR.Memory.DesiredMB
			totalProcessorUnits += node.LPAR.Processor.Units
		}
		
		if system.AvailableMemoryGB > 0 {
			totalMemoryGB := totalMemoryMB / 1024
			if totalMemoryGB > system.AvailableMemoryGB {
				v.warnings = append(v.warnings, fmt.Sprintf("system '%s': %dGB memory required but only %dGB available", 
					systemName, totalMemoryGB, system.AvailableMemoryGB))
			}
		}
		
		if system.AvailableProcessors > 0 {
			if int(totalProcessorUnits) > system.AvailableProcessors {
				v.warnings = append(v.warnings, fmt.Sprintf("system '%s': %.1f processor units required but only %d available", 
					systemName, totalProcessorUnits, system.AvailableProcessors))
			}
		}
	}
}

// validateDeployment validates deployment configuration
func (v *Validator) validateDeployment() {
	if len(v.config.Deployment.Phases) == 0 {
		v.warnings = append(v.warnings, "no deployment phases configured")
	}
	
	// Validate timeouts
	if v.config.Deployment.Timeouts.LPARCreation <= 0 {
		v.warnings = append(v.warnings, "LPAR creation timeout should be > 0")
	}
	
	// Validate retry configuration
	if v.config.Deployment.Retry.MaxAttempts < 1 {
		v.warnings = append(v.warnings, "retry max_attempts should be >= 1")
	}
}

// Helper functions

func (v *Validator) isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func (v *Validator) isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func (v *Validator) isValidURL(urlStr string) bool {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	
	// Check if scheme is http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}
	
	// Check if host is present
	if parsedURL.Host == "" {
		return false
	}
	
	return true
}

// validateFilePath validates a file path with support for ~ expansion
func (v *Validator) validateFilePath(name, path string, required bool) {
	if path == "" {
		if required {
			v.errors = append(v.errors, fmt.Sprintf("%s file path is required", name))
		}
		return
	}
	
	// Expand ~ to home directory
	expandedPath := path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			v.errors = append(v.errors, fmt.Sprintf("%s: unable to expand home directory in path '%s': %v", name, path, err))
			return
		}
		expandedPath = strings.Replace(path, "~", home, 1)
	}
	
	// Check if file exists
	fileInfo, err := os.Stat(expandedPath)
	if os.IsNotExist(err) {
		v.errors = append(v.errors, fmt.Sprintf("%s file not found: %s", name, path))
		return
	}
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: error accessing file '%s': %v", name, path, err))
		return
	}
	
	// Check if it's a regular file (not a directory)
	if fileInfo.IsDir() {
		v.errors = append(v.errors, fmt.Sprintf("%s path is a directory, not a file: %s", name, path))
		return
	}
	
	// Check file permissions (readable)
	file, err := os.Open(expandedPath)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s file is not readable: %s (error: %v)", name, path, err))
		return
	}
	file.Close()
}

// validateDirectoryPath validates a directory path with support for ~ expansion
func (v *Validator) validateDirectoryPath(name, path string, required bool) {
	if path == "" {
		if required {
			v.errors = append(v.errors, fmt.Sprintf("%s directory path is required", name))
		}
		return
	}
	
	// Expand ~ to home directory
	expandedPath := path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			v.errors = append(v.errors, fmt.Sprintf("%s: unable to expand home directory in path '%s': %v", name, path, err))
			return
		}
		expandedPath = strings.Replace(path, "~", home, 1)
	}
	
	// Check if directory exists
	fileInfo, err := os.Stat(expandedPath)
	if os.IsNotExist(err) {
		v.errors = append(v.errors, fmt.Sprintf("%s directory not found: %s", name, path))
		return
	}
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: error accessing directory '%s': %v", name, path, err))
		return
	}
	
	// Check if it's actually a directory
	if !fileInfo.IsDir() {
		v.errors = append(v.errors, fmt.Sprintf("%s path is a file, not a directory: %s", name, path))
		return
	}
	
	// Check directory permissions (readable and executable)
	if fileInfo.Mode().Perm()&0400 == 0 {
		v.warnings = append(v.warnings, fmt.Sprintf("%s directory may not be readable: %s", name, path))
	}
}

// ValidateURLAccessibility checks if URLs are accessible (optional, can be slow)
// This is a separate method that can be called optionally for thorough validation
func (v *Validator) ValidateURLAccessibility() {
	urls := []struct {
		name string
		url  string
	}{
		{"RHCOS Kernel", v.config.OpenShift.RHCOSImages.KernelURL},
		{"RHCOS Initramfs", v.config.OpenShift.RHCOSImages.InitramfsURL},
		{"RHCOS Rootfs", v.config.OpenShift.RHCOSImages.RootfsURL},
		{"OCP Client", v.config.OpenShift.OCPClientConfig.Client},
		{"OCP Installer", v.config.OpenShift.OCPClientConfig.Installer},
	}
	
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow redirects
		},
	}
	
	for _, item := range urls {
		if item.url == "" {
			continue
		}
		
		resp, err := client.Head(item.url)
		if err != nil {
			v.warnings = append(v.warnings, fmt.Sprintf("%s URL may not be accessible: %s (error: %v)", item.name, item.url, err))
			continue
		}
		resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
			v.warnings = append(v.warnings, fmt.Sprintf("%s URL returned status %d: %s", item.name, resp.StatusCode, item.url))
		}
	}
}

// Made with Bob
