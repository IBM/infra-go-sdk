package svc

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tier struct {
	Tier                   string `json:"tier"`
	TierCapacity           string `json:"tier_capacity"`
	TierFreeCapacity       string `json:"tier_free_capacity"`
	TierCompressedDataUsed string `json:"tier_compressed_data_used,omitempty"`
}

type SystemInfo struct {
	ID                              string `json:"id"`
	Name                            string `json:"name"`
	Location                        string `json:"location"`
	Partnership                     string `json:"partnership"`
	TotalMDiskCapacity              string `json:"total_mdisk_capacity"`
	SpaceInMDiskGrps                string `json:"space_in_mdisk_grps"`
	SpaceAllocatedToVDisks          string `json:"space_allocated_to_vdisks"`
	TotalFreeSpace                  string `json:"total_free_space"`
	TotalVDiskCopyCapacity          string `json:"total_vdiskcopy_capacity"`
	TotalUsedCapacity               string `json:"total_used_capacity"`
	TotalOverallocation             string `json:"total_overallocation"`
	TotalVDiskCapacity              string `json:"total_vdisk_capacity"`
	TotalAllocatedExtentCapacity    string `json:"total_allocated_extent_capacity"`
	StatisticsStatus                string `json:"statistics_status"`
	StatisticsFrequency             string `json:"statistics_frequency"`
	ClusterLocale                   string `json:"cluster_locale"`
	TimeZone                        string `json:"time_zone"`
	CodeLevel                       string `json:"code_level"`
	ConsoleIP                       string `json:"console_IP"`
	IDAlias                         string `json:"id_alias"`
	GMLinkTolerance                 string `json:"gm_link_tolerance"`
	GMInterClusterDelaySimulation   string `json:"gm_inter_cluster_delay_simulation"`
	GMIntraClusterDelaySimulation   string `json:"gm_intra_cluster_delay_simulation"`
	GMMaxHostDelay                  string `json:"gm_max_host_delay"`
	EmailReply                      string `json:"email_reply"`
	EmailContact                    string `json:"email_contact"`
	EmailContactPrimary             string `json:"email_contact_primary"`
	EmailContactAlternate           string `json:"email_contact_alternate"`
	EmailContactLocation            string `json:"email_contact_location"`
	EmailContact2                   string `json:"email_contact2"`
	EmailContact2Primary            string `json:"email_contact2_primary"`
	EmailContact2Alternate          string `json:"email_contact2_alternate"`
	EmailState                      string `json:"email_state"`
	InventoryMailInterval           string `json:"inventory_mail_interval"`
	ClusterNTPIPAddress             string `json:"cluster_ntp_IP_address"`
	ClusterISNSIPAddress            string `json:"cluster_isns_IP_address"`
	ISCSIAuthMethod                 string `json:"iscsi_auth_method"`
	ISCSIChapSecret                 string `json:"iscsi_chap_secret"`
	AuthServiceConfigured           string `json:"auth_service_configured"`
	AuthServiceEnabled              string `json:"auth_service_enabled"`
	AuthServiceURL                  string `json:"auth_service_url"`
	AuthServiceUserName             string `json:"auth_service_user_name"`
	AuthServicePwdSet               string `json:"auth_service_pwd_set"`
	AuthServiceCertSet              string `json:"auth_service_cert_set"`
	AuthServiceType                 string `json:"auth_service_type"`
	RelationshipBandwidthLimit      string `json:"relationship_bandwidth_limit"`
	Tiers                           []Tier `json:"tiers"`
	EasyTierAcceleration            string `json:"easy_tier_acceleration"`
	HasNASKey                       string `json:"has_nas_key"`
	Layer                           string `json:"layer"`
	RCBufferSize                    string `json:"rc_buffer_size"`
	CompressionActive               string `json:"compression_active"`
	CompressionVirtualCapacity      string `json:"compression_virtual_capacity"`
	CompressionCompressedCapacity   string `json:"compression_compressed_capacity"`
	CompressionUncompressedCapacity string `json:"compression_uncompressed_capacity"`
	CachePrefetch                   string `json:"cache_prefetch"`
	EmailOrganization               string `json:"email_organization"`
	EmailMachineAddress             string `json:"email_machine_address"`
	EmailMachineCity                string `json:"email_machine_city"`
	EmailMachineState               string `json:"email_machine_state"`
	EmailMachineZip                 string `json:"email_machine_zip"`
	EmailMachineCountry             string `json:"email_machine_country"`
	TotalDriveRawCapacity           string `json:"total_drive_raw_capacity"`
	CompressionDestageMode          string `json:"compression_destage_mode"`
	LocalFCPortMask                 string `json:"local_fc_port_mask"`
	PartnerFCPortMask               string `json:"partner_fc_port_mask"`
	HighTempMode                    string `json:"high_temp_mode"`
	Topology                        string `json:"topology"`
	TopologyStatus                  string `json:"topology_status"`
	RCAuthMethod                    string `json:"rc_auth_method"`
	VDiskProtectionTime             string `json:"vdisk_protection_time"`
	VDiskProtectionEnabled          string `json:"vdisk_protection_enabled"`
	ProductName                     string `json:"product_name"`
	ODX                             string `json:"odx"`
	MaxReplicationDelay             string `json:"max_replication_delay"`
	PartnershipExclusionThreshold   string `json:"partnership_exclusion_threshold"`
	Gen1CompatibilityModeEnabled    string `json:"gen1_compatibility_mode_enabled"`
	IBMCustomer                     string `json:"ibm_customer"`
	IBMComponent                    string `json:"ibm_component"`
	IBMCountry                      string `json:"ibm_country"`
	TotalReclaimableCapacity        string `json:"total_reclaimable_capacity"`
	PhysicalCapacity                string `json:"physical_capacity"`
	PhysicalFreeCapacity            string `json:"physical_free_capacity"`
	UsedCapacityBeforeReduction     string `json:"used_capacity_before_reduction"`
	UsedCapacityAfterReduction      string `json:"used_capacity_after_reduction"`
	OverheadCapacity                string `json:"overhead_capacity"`
	DeduplicationCapacitySaving     string `json:"deduplication_capacity_saving"`
	EnhancedCallhome                string `json:"enhanced_callhome"`
	CensorCallhome                  string `json:"censor_callhome"`
	HostUnmap                       string `json:"host_unmap"`
	BackendUnmap                    string `json:"backend_unmap"`
	QuorumMode                      string `json:"quorum_mode"`
	QuorumSiteID                    string `json:"quorum_site_id"`
	QuorumSiteName                  string `json:"quorum_site_name"`
	QuorumLease                     string `json:"quorum_lease"`
	AutomaticVDiskAnalysisEnabled   string `json:"automatic_vdisk_analysis_enabled"`
	CallhomeAcceptedUsage           string `json:"callhome_accepted_usage"`
	SafeguardedCopySuspended        string `json:"safeguarded_copy_suspended"`
}

func (c *Client) Lssystem(ctx context.Context) (*SystemInfo, error) {
	data, err := c.post(ctx, "lssystem", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", decodeIBMError(err))
	}

	var info SystemInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse lssystem response: %w", err)
	}

	return &info, nil
}