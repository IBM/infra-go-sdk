package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// CONFIGURATION TYPES
// =============================================================================

// Config represents the complete deployment configuration
type Config struct {
	HMC          HMCConfig          `yaml:"hmc"`
	PowerSystems []PowerSystem      `yaml:"power_systems"`
	Storage      StorageConfig      `yaml:"storage"`
	Network      NetworkConfig      `yaml:"network"`
	OpenShift    OpenShiftConfig    `yaml:"openshift"`
	HelperNode   HelperNodeConfig   `yaml:"helper_node"`
	Bootstrap    *BootstrapConfig   `yaml:"bootstrap,omitempty"`
	Masters      *MastersConfig     `yaml:"masters,omitempty"`
	Workers      *WorkersConfig     `yaml:"workers,omitempty"`
	SNONode      *SNONodeConfig     `yaml:"sno_node,omitempty"`
	Ansible      AnsibleConfig      `yaml:"ansible"`
	Deployment   DeploymentConfig   `yaml:"deployment"`
	Advanced     AdvancedConfig     `yaml:"advanced"`
}

// HMCConfig represents HMC connection details
type HMCConfig struct {
	IP       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// PowerSystem represents a Power system configuration
type PowerSystem struct {
	Name                 string `yaml:"name"`
	VswitchName          string `yaml:"vswitch_name"`
	VlanID               int    `yaml:"vlan_id"`
	MaxLPARs             int    `yaml:"max_lpars,omitempty"`
	AvailableMemoryGB    int    `yaml:"available_memory_gb,omitempty"`
	AvailableProcessors  int    `yaml:"available_processors,omitempty"`
}

// StorageConfig represents storage backend configuration
type StorageConfig struct {
	Type string      `yaml:"type"`
	SVC  SVCConfig   `yaml:"svc,omitempty"`
	VIOS []VIOSConfig `yaml:"vios,omitempty"`
}

// SVCConfig represents SVC storage configuration
type SVCConfig struct {
	IP           string `yaml:"ip"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PoolName     string `yaml:"pool_name"`
	VolumePrefix string `yaml:"volume_prefix"`
}

// VIOSConfig represents VIOS storage configuration
type VIOSConfig struct {
	SystemName  string `yaml:"system_name"`
	VIOSName    string `yaml:"vios_name"`
	VolumeGroup string `yaml:"volume_group"`
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Domain       string   `yaml:"domain"`
	ClusterName  string   `yaml:"cluster_name"`
	BaseDomain   string   `yaml:"base_domain"`
	NetworkCIDR  string   `yaml:"network_cidr"`
	Gateway      string   `yaml:"gateway"`
	Netmask      string   `yaml:"netmask"`
	Broadcast    string   `yaml:"broadcast"`
	Nameserver   string   `yaml:"nameserver"`
	DNSForwarders []string `yaml:"dns_forwarders"`
	NTPServers   []string `yaml:"ntp_servers,omitempty"`
	MACPrefix    string   `yaml:"mac_prefix,omitempty"`
}

// OpenShiftConfig represents OpenShift cluster configuration
type OpenShiftConfig struct {
	Version                    string          `yaml:"version"`
	PullSecretFile             string          `yaml:"pull_secret_file"`
	SSHPublicKeyFile           string          `yaml:"ssh_public_key_file"`
	ClusterNetworkCIDR         string          `yaml:"cluster_network_cidr"`
	ClusterNetworkHostPrefix   int             `yaml:"cluster_network_host_prefix"`
	ServiceNetwork             string          `yaml:"service_network"`
	MachineNetwork             string          `yaml:"machine_network"`
	DiskDevice                 string          `yaml:"disk_device,omitempty"`           // Disk where RHCOS will be installed
	InstallType                string          `yaml:"install_type"`                    // Installation type: agent, assisted, sno, normal
	ForceOCPDownload           bool            `yaml:"force_ocp_download"`              // Download RHCOS and client for PXE setup
	RHCOSArch                  string          `yaml:"rhcos_arch,omitempty"`            // RHCOS architecture (default: ppc64le)
	RHCOSKernelOptions         []string        `yaml:"rhcos_kernel_options,omitempty"`  // Kernel options for RHCOS nodes
	SysctlTunedOptions         bool            `yaml:"sysctl_tuned_options,omitempty"`  // Apply sysctl options via tuned operator
	PowerVMRMC                 bool            `yaml:"powervm_rmc,omitempty"`           // Deploy RMC daemonset on ppc64le nodes
	ForceDNS                   bool            `yaml:"force_dns,omitempty"`             // Force DNS to use helper as DNS server
	RHCOSImages                RHCOSURLs       `yaml:"rhcos_images"`                    // Required for PXE boot setup
	OCPClientConfig            OCPClientConfig `yaml:"ocp_client_config"`               // OpenShift client/installer configuration
}

// RHCOSURLs holds RHCOS image URLs for PXE boot configuration
type RHCOSURLs struct {
	KernelURL    string `yaml:"kernel_url"`
	InitramfsURL string `yaml:"initramfs_url"`
	RootfsURL    string `yaml:"rootfs_url"`
}

// OCPClientConfig holds OpenShift client and installer configuration
// These match the Ansible playbook variable names exactly
type OCPClientConfig struct {
	Arch      string `yaml:"ocp_client_arch"`      // Architecture (e.g., "ppc64le")
	BaseURL   string `yaml:"ocp_base_url"`         // Base URL for OCP clients
	Base      string `yaml:"ocp_client_base"`      // Client base path (e.g., "ocp")
	Tag       string `yaml:"ocp_client_tag"`       // Version tag (e.g., "latest-4.21")
	Client    string `yaml:"ocp_client"`           // Full URL to openshift-client-linux.tar.gz
	Installer string `yaml:"ocp_installer"`        // Full URL to openshift-install-linux.tar.gz
}

// HelperNodeConfig represents helper/bastion node configuration
type HelperNodeConfig struct {
	Hostname         string         `yaml:"hostname"`
	IP               string         `yaml:"ip"`
	SSHUser          string         `yaml:"ssh_user"`
	SSHKeyFile       string         `yaml:"ssh_key_file"`
	Services         ServicesConfig `yaml:"services"`
	NetworkInterface string         `yaml:"network_interface"`
}

// BootstrapConfig represents bootstrap node configuration
type BootstrapConfig struct {
	Name       string     `yaml:"name"`
	Hostname   string     `yaml:"hostname"`
	IP         string     `yaml:"ip"`
	SystemName string     `yaml:"system_name"`
	LPAR       LPARConfig `yaml:"lpar"`
}

// MastersConfig represents master nodes configuration
type MastersConfig struct {
	Nodes []NodeConfig `yaml:"nodes"`
	LPAR  LPARConfig   `yaml:"lpar"`
}

// WorkersConfig represents worker nodes configuration
type WorkersConfig struct {
	Nodes []NodeConfig `yaml:"nodes"`
	LPAR  LPARConfig   `yaml:"lpar"`
}

// SNONodeConfig represents Single Node OpenShift configuration
type SNONodeConfig struct {
	Name       string     `yaml:"name"`
	Hostname   string     `yaml:"hostname"`
	IP         string     `yaml:"ip"`
	SystemName string     `yaml:"system_name"`
	LPAR       LPARConfig `yaml:"lpar"`
	MACAddress string     `yaml:"mac_address,omitempty"` // Populated from created LPAR state
}

// NodeConfig represents a single node configuration
type NodeConfig struct {
	Name       string `yaml:"name"`
	Hostname   string `yaml:"hostname"`
	IP         string `yaml:"ip"`
	SystemName string `yaml:"system_name"`
	MACAddress string `yaml:"mac_address,omitempty"` // Populated from created LPAR state
}

// LPARConfig represents LPAR resource configuration
type LPARConfig struct {
	OSType    string          `yaml:"os_type"`
	Processor ProcessorConfig `yaml:"processor"`
	Memory    MemoryConfig    `yaml:"memory"`
	Storage   StorageSpec     `yaml:"storage"`
}

// ProcessorConfig represents processor configuration
type ProcessorConfig struct {
	Type         string  `yaml:"type"`
	Units        float64 `yaml:"units"`
	VirtualProcs int     `yaml:"virtual_procs"`
	MinUnits     float64 `yaml:"min_units"`
	MaxUnits     float64 `yaml:"max_units"`
	MinProcs     int     `yaml:"min_procs"`
	MaxProcs     int     `yaml:"max_procs"`
}

// MemoryConfig represents memory configuration
type MemoryConfig struct {
	DesiredMB int `yaml:"desired_mb"`
	MinMB     int `yaml:"min_mb"`
	MaxMB     int `yaml:"max_mb"`
}

// StorageSpec represents storage specification
type StorageSpec struct {
	BootDiskGB          int `yaml:"boot_disk_gb"`
	DataDiskGB          int `yaml:"data_disk_gb,omitempty"`
	EtcdDiskGB          int `yaml:"etcd_disk_gb,omitempty"`
	ContainerStorageGB  int `yaml:"container_storage_gb,omitempty"`
}

// ServicesConfig represents services configuration for helper node
type ServicesConfig struct {
	Dnsmasq DnsmasqConfig `yaml:"dnsmasq"`
	HTTP    HTTPConfig    `yaml:"http"`
	HAProxy HAProxyConfig `yaml:"haproxy"`
	NFS     NFSConfig     `yaml:"nfs"`
}

// DnsmasqConfig represents dnsmasq service configuration
type DnsmasqConfig struct {
	Enabled        bool   `yaml:"enabled"`
	DHCPRangeStart string `yaml:"dhcp_range_start"`
	DHCPRangeEnd   string `yaml:"dhcp_range_end"`
	DHCPLeaseTime  string `yaml:"dhcp_lease_time"`
	TFTPRoot       string `yaml:"tftp_root"`
}

// HTTPConfig represents HTTP service configuration
type HTTPConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Port         int    `yaml:"port"`
	DocumentRoot string `yaml:"document_root"`
}

// HAProxyConfig represents HAProxy service configuration
type HAProxyConfig struct {
	Enabled bool `yaml:"enabled"`
}

// NFSConfig represents NFS service configuration
type NFSConfig struct {
	Enabled bool `yaml:"enabled"`
}

// AnsibleConfig represents ansible playbook configuration
type AnsibleConfig struct {
	PlaybookRepo   string `yaml:"playbook_repo"`
	PlaybookBranch string `yaml:"playbook_branch"`
	VarsFile       string `yaml:"vars_file"`
}

// DeploymentConfig represents deployment options
type DeploymentConfig struct {
	Phases           []string        `yaml:"phases"`
	Timeouts         TimeoutsConfig  `yaml:"timeouts"`
	Retry            RetryConfig     `yaml:"retry"`
	CleanupOnFailure bool            `yaml:"cleanup_on_failure"`
	Verbose          bool            `yaml:"verbose"`
}

// TimeoutsConfig represents timeout configuration
type TimeoutsConfig struct {
	LPARCreation         int `yaml:"lpar_creation"`
	StorageAttachment    int `yaml:"storage_attachment"`
	PowerOn              int `yaml:"power_on"`
	HelperSetup          int `yaml:"helper_setup"`
	BootstrapComplete    int `yaml:"bootstrap_complete"`
	InstallationComplete int `yaml:"installation_complete"`
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxAttempts  int `yaml:"max_attempts"`
	DelaySeconds int `yaml:"delay_seconds"`
}

// AdvancedConfig represents advanced options
type AdvancedConfig struct {
	ParallelLPARCreation      bool   `yaml:"parallel_lpar_creation"`
	MaxParallelOperations     int    `yaml:"max_parallel_operations"`
	ValidateResources         bool   `yaml:"validate_resources"`
	CheckNetworkConnectivity  bool   `yaml:"check_network_connectivity"`
	VerifyStorageCapacity     bool   `yaml:"verify_storage_capacity"`
	AutoDistributeLPARs       bool   `yaml:"auto_distribute_lpars"`
	DistributionStrategy      string `yaml:"distribution_strategy"`
	SaveKubeconfig            bool   `yaml:"save_kubeconfig"`
	KubeconfigPath            string `yaml:"kubeconfig_path"`
	EnableMonitoring          bool   `yaml:"enable_monitoring"`
	MonitoringIntervalSeconds int    `yaml:"monitoring_interval_seconds"`
	StateFile                 string `yaml:"state_file"`
	SaveStateOnEachPhase      bool   `yaml:"save_state_on_each_phase"`
	SNOMode                   bool   `yaml:"sno_mode"`
	SkipBootstrap             bool   `yaml:"skip_bootstrap"`
	SkipWorkers               bool   `yaml:"skip_workers"`
	MasterSchedulable         bool   `yaml:"master_schedulable"`
}

// =============================================================================
// DEPLOYMENT STATE TYPES
// =============================================================================

// DeploymentState represents the current state of deployment
type DeploymentState struct {
	CurrentPhase    string                 `json:"current_phase"`
	CompletedPhases []string               `json:"completed_phases"`
	CreatedLPARs    map[string]LPARState   `json:"created_lpars"`
	CreatedVolumes  map[string]VolumeState `json:"created_volumes"`
	Timestamp       string                 `json:"timestamp"`
	Status          string                 `json:"status"` // "in_progress", "completed", "failed"
	Error           string                 `json:"error,omitempty"`
}

// LPARState represents the state of a created LPAR
type LPARState struct {
	Name       string   `json:"name"`
	UUID       string   `json:"uuid"`
	SystemName string   `json:"system_name"`
	IP         string   `json:"ip"`
	MACAddress string   `json:"mac_address"`
	Status     string   `json:"status"` // "created", "powered_on", "configured"
	Volumes    []string `json:"volumes"`
}

// VolumeState represents the state of a created volume
type VolumeState struct {
	Name       string `json:"name"`
	VolumeID   string `json:"volume_id"`
	SizeGB     int    `json:"size_gb"`
	AttachedTo string `json:"attached_to"` // LPAR name
	Status     string `json:"status"`      // "created", "mapped"
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// LoadConfig loads and parses the configuration file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// GetPowerSystem returns a Power system by name
func (c *Config) GetPowerSystem(name string) (*PowerSystem, error) {
	for i := range c.PowerSystems {
		if c.PowerSystems[i].Name == name {
			return &c.PowerSystems[i], nil
		}
	}
	return nil, fmt.Errorf("power system '%s' not found", name)
}

// GetAllNodes returns all nodes (helper, bootstrap, masters, workers, or SNO)
func (c *Config) GetAllNodes() []NodeInfo {
	nodes := []NodeInfo{}

	// SNO mode - only SNO node
	if c.SNONode != nil {
		nodes = append(nodes, NodeInfo{
			Name:       c.SNONode.Name,
			Hostname:   c.SNONode.Hostname,
			IP:         c.SNONode.IP,
			SystemName: c.SNONode.SystemName,
			Role:       "sno",
			LPAR:       c.SNONode.LPAR,
		})
		return nodes
	}

	// Standard HA mode - helper, bootstrap, masters, workers
	// Bootstrap node
	if c.Bootstrap != nil {
		nodes = append(nodes, NodeInfo{
			Name:       c.Bootstrap.Name,
			Hostname:   c.Bootstrap.Hostname,
			IP:         c.Bootstrap.IP,
			SystemName: c.Bootstrap.SystemName,
			Role:       "bootstrap",
			LPAR:       c.Bootstrap.LPAR,
		})
	}

	// Master nodes
	if c.Masters != nil {
		for _, master := range c.Masters.Nodes {
			nodes = append(nodes, NodeInfo{
				Name:       master.Name,
				Hostname:   master.Hostname,
				IP:         master.IP,
				SystemName: master.SystemName,
				Role:       "master",
				LPAR:       c.Masters.LPAR,
			})
		}
	}

	// Worker nodes
	if c.Workers != nil {
		for _, worker := range c.Workers.Nodes {
			nodes = append(nodes, NodeInfo{
				Name:       worker.Name,
				Hostname:   worker.Hostname,
				IP:         worker.IP,
				SystemName: worker.SystemName,
				Role:       "worker",
				LPAR:       c.Workers.LPAR,
			})
		}
	}

	return nodes
}

// IsSNO returns true if this is a Single Node OpenShift configuration
func (c *Config) IsSNO() bool {
	return c.SNONode != nil
}

// NodeInfo represents complete node information
type NodeInfo struct {
	Name       string
	Hostname   string
	IP         string
	SystemName string
	Role       string // "helper", "bootstrap", "master", "worker", "sno"
	LPAR       LPARConfig
}

// GetNodesBySystem returns nodes grouped by Power system
func (c *Config) GetNodesBySystem() map[string][]NodeInfo {
	nodesBySystem := make(map[string][]NodeInfo)
	
	for _, node := range c.GetAllNodes() {
		nodesBySystem[node.SystemName] = append(nodesBySystem[node.SystemName], node)
	}
	
	return nodesBySystem
}

// ValidateBasic performs basic validation of the configuration
func (c *Config) ValidateBasic() error {
	// Validate HMC config
	if c.HMC.IP == "" {
		return fmt.Errorf("HMC IP is required")
	}
	if c.HMC.Username == "" {
		return fmt.Errorf("HMC username is required")
	}

	// Validate at least one Power system
	if len(c.PowerSystems) == 0 {
		return fmt.Errorf("at least one Power system is required")
	}

	// SNO mode validation
	if c.IsSNO() {
		if c.SNONode.Name == "" {
			return fmt.Errorf("SNO node name is required")
		}
		if c.SNONode.IP == "" {
			return fmt.Errorf("SNO node IP is required")
		}
		// SNO doesn't need bootstrap or workers
		return nil
	}

	// HA mode validation
	if c.Masters == nil || len(c.Masters.Nodes) < 3 {
		return fmt.Errorf("minimum 3 master nodes required for HA mode")
	}
	if len(c.Masters.Nodes)%2 == 0 {
		return fmt.Errorf("master count must be odd number for quorum, got %d", len(c.Masters.Nodes))
	}

	if c.Workers == nil || len(c.Workers.Nodes) < 2 {
		return fmt.Errorf("minimum 2 worker nodes required for HA mode")
	}

	// Validate all nodes reference valid Power systems
	for _, node := range c.GetAllNodes() {
		if _, err := c.GetPowerSystem(node.SystemName); err != nil {
			return fmt.Errorf("node %s references invalid system: %v", node.Name, err)
		}
	}

	return nil
}

// Made with Bob
