package types

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// MULTI-CLUSTER CONFIGURATION TYPES
// =============================================================================

// MultiClusterConfig represents the top-level configuration for multiple clusters
type MultiClusterConfig struct {
	HelperNode HelperNodeConfig `yaml:"helper_node"`
	HMC        HMCConfig        `yaml:"hmc"`
	Clusters   []ClusterRef     `yaml:"clusters"`
}

// ClusterRef references an individual cluster configuration
type ClusterRef struct {
	Name          string         `yaml:"name"`
	Type          string         `yaml:"type"`                     // "sno" or "multi-node"
	OCPVersion    string         `yaml:"ocp_version"`              // e.g., "4.21"
	VIP           string         `yaml:"vip"`                      // Single VIP for both API and Ingress
	ConfigFile    string         `yaml:"config_file,omitempty"`    // Legacy: Path to external config
	ClusterConfig *ClusterConfig `yaml:"cluster_config,omitempty"` // New: Embedded config
}

// HelperNodeConfig represents the helper/bastion node configuration
type HelperNodeConfig struct {
	Hostname         string   `yaml:"hostname"`
	IP               string   `yaml:"ip"`
	SSHUser          string   `yaml:"ssh_user"`
	SSHKeyFile       string   `yaml:"ssh_key_file"`
	NetworkInterface string   `yaml:"network_interface"`
	RequiredPackages []string `yaml:"required_packages,omitempty"`
}

// HMCConfig represents HMC connection details
type HMCConfig struct {
	IP       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// =============================================================================
// CLUSTER-SPECIFIC CONFIGURATION TYPES
// =============================================================================

// ClusterConfig represents a single cluster's complete configuration
type ClusterConfig struct {
	PowerSystems []PowerSystem    `yaml:"power_systems"`
	Storage      StorageConfig    `yaml:"storage"`
	Network      NetworkConfig    `yaml:"network"`
	OpenShift    OpenShiftConfig  `yaml:"openshift"`
	SNONode      *SNONodeConfig   `yaml:"sno_node,omitempty"`
	Bootstrap    *BootstrapConfig `yaml:"bootstrap,omitempty"`
	Masters      *MastersConfig   `yaml:"masters,omitempty"`
	Workers      *WorkersConfig   `yaml:"workers,omitempty"`
	Deployment   DeploymentConfig `yaml:"deployment"`
	Advanced     AdvancedConfig   `yaml:"advanced"`
}

// PowerSystem represents a Power system configuration
type PowerSystem struct {
	Name                string `yaml:"name"`
	VswitchName         string `yaml:"vswitch_name"`
	VlanID              int    `yaml:"vlan_id"`
	MaxLPARs            int    `yaml:"max_lpars,omitempty"`
	AvailableMemoryGB   int    `yaml:"available_memory_gb,omitempty"`
	AvailableProcessors int    `yaml:"available_processors,omitempty"`
}

// StorageConfig represents storage backend configuration
type StorageConfig struct {
	Type string       `yaml:"type"` // "vios" or "svc"
	SVC  *SVCConfig   `yaml:"svc,omitempty"`
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
// Note: ClusterName is derived from ClusterContext.Name (the deployment identifier)
type NetworkConfig struct {
	Domain        string   `yaml:"domain"`
	BaseDomain    string   `yaml:"base_domain"`
	NetworkCIDR   string   `yaml:"network_cidr"`
	Gateway       string   `yaml:"gateway"`
	Netmask       string   `yaml:"netmask"`
	Broadcast     string   `yaml:"broadcast"`
	Nameserver    string   `yaml:"nameserver"`
	DNSForwarders []string `yaml:"dns_forwarders"`
	NTPServers    []string `yaml:"ntp_servers,omitempty"`
	MACPrefix     string   `yaml:"mac_prefix,omitempty"`
}

// OpenShiftConfig represents OpenShift cluster configuration
type OpenShiftConfig struct {
	Version                  string          `yaml:"version"`
	PullSecretFile           string          `yaml:"pull_secret_file"`
	SSHPublicKeyFile         string          `yaml:"ssh_public_key_file"`
	ClusterNetworkCIDR       string          `yaml:"cluster_network_cidr"`
	ClusterNetworkHostPrefix int             `yaml:"cluster_network_host_prefix"`
	ServiceNetwork           string          `yaml:"service_network"`
	MachineNetwork           string          `yaml:"machine_network"`
	DiskDevice               string          `yaml:"disk_device,omitempty"`
	InstallType              string          `yaml:"install_type"` // "sno", "multi-node"
	ForceOCPDownload         bool            `yaml:"force_ocp_download"`
	RHCOSArch                string          `yaml:"rhcos_arch,omitempty"`
	RHCOSKernelOptions       []string        `yaml:"rhcos_kernel_options,omitempty"`
	SysctlTunedOptions       bool            `yaml:"sysctl_tuned_options,omitempty"`
	PowerVMRMC               bool            `yaml:"powervm_rmc,omitempty"`
	ForceDNS                 bool            `yaml:"force_dns,omitempty"`
	RHCOSImages              RHCOSURLs       `yaml:"rhcos_images"`
	OCPClientConfig          OCPClientConfig `yaml:"ocp_client_config"`
}

// RHCOSURLs holds RHCOS image URLs
type RHCOSURLs struct {
	KernelURL    string `yaml:"kernel_url"`
	InitramfsURL string `yaml:"initramfs_url"`
	RootfsURL    string `yaml:"rootfs_url"`
}

// OCPClientConfig holds OpenShift client configuration
type OCPClientConfig struct {
	Arch      string `yaml:"ocp_client_arch"`
	BaseURL   string `yaml:"ocp_base_url"`
	Base      string `yaml:"ocp_client_base"`
	Tag       string `yaml:"ocp_client_tag"`
	Client    string `yaml:"ocp_client"`
	Installer string `yaml:"ocp_installer"`
}

// SNONodeConfig represents Single Node OpenShift configuration
// Note: Name and Hostname are optional and will be auto-populated from cluster name if not provided
type SNONodeConfig struct {
	Name       string     `yaml:"name,omitempty"`     // Optional: defaults to "{hostname}-master"
	Hostname   string     `yaml:"hostname,omitempty"` // Optional: defaults to cluster name
	IP         string     `yaml:"ip"`
	SystemName string     `yaml:"system_name"`
	LPAR       LPARConfig `yaml:"lpar"`
	MACAddress string     `yaml:"mac_address,omitempty"`
}

// BootstrapConfig represents bootstrap node configuration
type BootstrapConfig struct {
	Name       string     `yaml:"name"`
	Hostname   string     `yaml:"hostname"`
	IP         string     `yaml:"ip"`
	SystemName string     `yaml:"system_name"`
	LPAR       LPARConfig `yaml:"lpar"`
	MACAddress string     `yaml:"mac_address,omitempty"`
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

// NodeConfig represents a single node configuration
type NodeConfig struct {
	Name       string `yaml:"name"`
	Hostname   string `yaml:"hostname"`
	IP         string `yaml:"ip"`
	SystemName string `yaml:"system_name"`
	MACAddress string `yaml:"mac_address,omitempty"`
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
	BootDiskGB         int `yaml:"boot_disk_gb"`
	DataDiskGB         int `yaml:"data_disk_gb,omitempty"`
	EtcdDiskGB         int `yaml:"etcd_disk_gb,omitempty"`
	ContainerStorageGB int `yaml:"container_storage_gb,omitempty"`
}

// DeploymentConfig represents deployment options
type DeploymentConfig struct {
	Phases           []string       `yaml:"phases"`
	Timeouts         TimeoutsConfig `yaml:"timeouts"`
	Retry            RetryConfig    `yaml:"retry"`
	CleanupOnFailure bool           `yaml:"cleanup_on_failure"`
	Verbose          bool           `yaml:"verbose"`
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
	// Version and identification
	StateVersion int    `json:"state_version"`
	ClusterName  string `json:"cluster_name"`

	// Phase tracking
	CurrentPhase    string           `json:"current_phase"`
	CompletedPhases []string         `json:"completed_phases"`
	PhaseHistory    []PhaseExecution `json:"phase_history"`

	// Resource states
	CreatedLPARs   map[string]LPARState   `json:"created_lpars"`
	CreatedVolumes map[string]VolumeState `json:"created_volumes"`
	IPAliases      []IPAliasState         `json:"ip_aliases"`
	HelperFiles    []string               `json:"helper_files"` // <-- ADDED: Tracks created files/dirs

	// Service endpoints
	ServiceEndpoints ServiceEndpoints `json:"service_endpoints"`

	// Status and timing
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// PhaseExecution tracks the execution details of a deployment phase
type PhaseExecution struct {
	PhaseName   string   `json:"phase_name"`
	Status      string   `json:"status"`             // "running", "completed", "failed", "skipped"
	StartTime   string   `json:"start_time"`         // ISO 8601 format
	EndTime     string   `json:"end_time,omitempty"` // ISO 8601 format
	DurationSec float64  `json:"duration_sec"`
	Error       string   `json:"error,omitempty"`
	RetryCount  int      `json:"retry_count"`
	Artifacts   []string `json:"artifacts,omitempty"` // Files/resources created in this phase
}

// ServiceEndpoints stores cluster service URLs and access information
type ServiceEndpoints struct {
	APIURL         string            `json:"api_url,omitempty"`
	IngressURL     string            `json:"ingress_url,omitempty"`
	ConsoleURL     string            `json:"console_url,omitempty"`
	KubeconfigPath string            `json:"kubeconfig_path,omitempty"`
	IgnitionURLs   map[string]string `json:"ignition_urls,omitempty"` // role -> URL
	HTTPServerURL  string            `json:"http_server_url,omitempty"`
	TFTPServerIP   string            `json:"tftp_server_ip,omitempty"`
	RHCOSFiles     RHCOSFiles        `json:"rhcos_files"` // Downloaded RHCOS filenames (always include in JSON)
}

// RHCOSFiles stores the filenames of downloaded RHCOS boot files
type RHCOSFiles struct {
	Kernel    string `json:"kernel"`    // e.g., "kernel"
	Initramfs string `json:"initramfs"` // e.g., "initramfs.img"
	Rootfs    string `json:"rootfs"`    // e.g., "rootfs.img"
}

// LPARState represents the state of a created LPAR
type LPARState struct {
	// Basic identification
	Name         string   `json:"name"`
	UUID         string   `json:"uuid"`
	SystemName   string   `json:"system_name"`
	SystemUUID   string   `json:"system_uuid"`
	IP           string   `json:"ip"`
	MACAddress   string   `json:"mac_address"`
	LocationCode string   `json:"location_code"` // Network adapter location code for netboot
	Status       string   `json:"status"`        // "lpar_created", "storage_attached", "network_attached", "profile_saved", "powered_on"
	Volumes      []string `json:"volumes"`

	// Resource allocation
	ProcessorUnits    float64 `json:"processor_units,omitempty"`
	VirtualProcessors int     `json:"virtual_processors,omitempty"`
	MemoryMB          int     `json:"memory_mb,omitempty"`
	ProfileName       string  `json:"profile_name,omitempty"`

	// Network details
	NetworkAdapterID string `json:"network_adapter_id,omitempty"`
	VLanID           int    `json:"vlan_id,omitempty"`
	VswitchName      string `json:"vswitch_name,omitempty"`

	// Timestamps
	CreatedAt     string `json:"created_at,omitempty"`      // ISO 8601 format
	LastPoweredOn string `json:"last_powered_on,omitempty"` // ISO 8601 format
	LastModified  string `json:"last_modified,omitempty"`   // ISO 8601 format
}

// VolumeState represents the state of a created volume
type VolumeState struct {
	Name       string `json:"name"`
	VolumeID   string `json:"volume_id,omitempty"` // UDID if available
	SizeGB     int    `json:"size_gb"`
	AttachedTo string `json:"attached_to"` // LPAR name
	Status     string `json:"status"`      // "created", "mapped"

	// Deep Storage Tracking fields:
	StorageType string `json:"storage_type"` // "vios" or "svc"
	ViosName    string `json:"vios_name,omitempty"`
	VolumeGroup string `json:"volume_group,omitempty"`
	PoolName    string `json:"pool_name,omitempty"` // For SVC
}

// IPAliasState represents the state of an IP alias
type IPAliasState struct {
	Interface string `json:"interface"` // e.g., "eth0:0"
	IP        string `json:"ip"`
	Purpose   string `json:"purpose"` // "api" or "ingress"
}

// =============================================================================
// COMBINED CLUSTER CONTEXT
// =============================================================================

// ClusterContext combines all configuration for a single cluster deployment
type ClusterContext struct {
	Name          string
	Type          string // "sno" or "multi-node"
	OCPVersion    string
	VIP           string // Single VIP for both API and Ingress
	ClusterConfig *ClusterConfig
	HelperNode    HelperNodeConfig
	HMC           HMCConfig
	State         *DeploymentState
	Verbose       bool // Enable verbose output for HMC API calls
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// LoadMultiClusterConfig loads the multi-cluster configuration
func LoadMultiClusterConfig(filename string) (*MultiClusterConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config MultiClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// LoadClusterConfig loads a cluster-specific configuration
func LoadClusterConfig(filename string) (*ClusterConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config file: %v", err)
	}

	var config ClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cluster config file: %v", err)
	}

	return &config, nil
}

// IsSNO returns true if this is a Single Node OpenShift configuration
func (c *ClusterConfig) IsSNO() bool {
	return c.SNONode != nil
}

// GetAllNodes returns all nodes for the cluster
func (c *ClusterConfig) GetAllNodes() []NodeInfo {
	nodes := []NodeInfo{}

	// SNO mode - only SNO node
	if c.SNONode != nil {
		// Ensure hostname is populated (use cluster name from context if empty)
		// This is needed when config is loaded without validation
		hostname := c.SNONode.Hostname
		if hostname == "" {
			// Hostname will be populated by validator, but for safety we need a fallback
			// The actual cluster name should be passed from context, but we can't access it here
			// So we'll handle this in the caller if needed
			hostname = c.SNONode.Hostname
		}

		nodes = append(nodes, NodeInfo{
			Name:       c.SNONode.Name,
			Hostname:   hostname,
			IP:         c.SNONode.IP,
			SystemName: c.SNONode.SystemName,
			Role:       "sno",
			LPAR:       c.SNONode.LPAR,
			MACAddress: c.SNONode.MACAddress,
		})
		return nodes
	}

	// Multi-node mode
	if c.Bootstrap != nil {
		nodes = append(nodes, NodeInfo{
			Name:       c.Bootstrap.Name,
			Hostname:   c.Bootstrap.Hostname,
			IP:         c.Bootstrap.IP,
			SystemName: c.Bootstrap.SystemName,
			Role:       "bootstrap",
			LPAR:       c.Bootstrap.LPAR,
			MACAddress: c.Bootstrap.MACAddress,
		})
	}

	if c.Masters != nil {
		for _, master := range c.Masters.Nodes {
			nodes = append(nodes, NodeInfo{
				Name:       master.Name,
				Hostname:   master.Hostname,
				IP:         master.IP,
				SystemName: master.SystemName,
				Role:       "master",
				LPAR:       c.Masters.LPAR,
				MACAddress: master.MACAddress,
			})
		}
	}

	if c.Workers != nil {
		for _, worker := range c.Workers.Nodes {
			nodes = append(nodes, NodeInfo{
				Name:       worker.Name,
				Hostname:   worker.Hostname,
				IP:         worker.IP,
				SystemName: worker.SystemName,
				Role:       "worker",
				LPAR:       c.Workers.LPAR,
				MACAddress: worker.MACAddress,
			})
		}
	}

	return nodes
}

// NodeInfo represents complete node information
type NodeInfo struct {
	Name       string
	Hostname   string
	IP         string
	SystemName string
	Role       string // "sno", "bootstrap", "master", "worker"
	LPAR       LPARConfig
	MACAddress string
}

// GetPowerSystem returns a Power system by name
func (c *ClusterConfig) GetPowerSystem(name string) (*PowerSystem, error) {
	for i := range c.PowerSystems {
		if c.PowerSystems[i].Name == name {
			return &c.PowerSystems[i], nil
		}
	}
	return nil, fmt.Errorf("power system '%s' not found", name)
}

// GetClusterConfig resolves the cluster configuration, either from the embedded struct or an external file
func (c *ClusterRef) GetClusterConfig() (*ClusterConfig, error) {
	// If the user embedded the config directly in the main yaml, use it
	if c.ClusterConfig != nil {
		return c.ClusterConfig, nil
	}

	// Fallback to the legacy split-file approach
	if c.ConfigFile != "" {
		return LoadClusterConfig(c.ConfigFile)
	}

	return nil, fmt.Errorf("no cluster configuration found for '%s': must provide either 'cluster_config' block or 'config_file' path", c.Name)
}

// GetStateFilePath returns the state file path for a cluster
// Uses cluster directory structure: clusters/<cluster-name>/state.json
func GetStateFilePath(clusterName string) string {
	return fmt.Sprintf("clusters/%s/state.json", clusterName)
}

// Made with Bob
