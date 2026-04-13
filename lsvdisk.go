package svc

import (
	"encoding/json"
	"fmt"
)

// Vdisk represents a comprehensive volume object from the lsvdisk API response.
// All fields are strings to accommodate the IBM Spectrum Virtualize REST API,
// which returns percentages, sizes (e.g., "500.00GB"), and booleans as strings.
type Vdisk struct {
	// General Volume Properties
	ID                  string `json:"id"`
	Name                string `json:"name"`
	IOGroupID           string `json:"IO_group_id"`
	IOGroupName         string `json:"IO_group_name"`
	Status              string `json:"status"`
	MdiskGrpID          string `json:"mdisk_grp_id"`
	MdiskGrpName        string `json:"mdisk_grp_name"`
	Capacity            string `json:"capacity"`
	Type                string `json:"type"`
	Formatted           string `json:"formatted"`
	Formatting          string `json:"formatting"`
	VdiskUID            string `json:"vdisk_UID"`
	PreferredNodeID     string `json:"preferred_node_id"`
	PreferredNodeName   string `json:"preferred_node_name"`
	FastWriteState      string `json:"fast_write_state"`
	Cache               string `json:"cache"`
	FCMapCount          string `json:"fc_map_count"`
	SyncRate            string `json:"sync_rate"`
	CopyCount           string `json:"copy_count"`
	SECopyCount         string `json:"se_copy_count"`
	MirrorWritePriority string `json:"mirror_write_priority"`
	RCChange            string `json:"RC_change"`
	CompressedCopyCount string `json:"compressed_copy_count"`
	AccessIOGroupCount  string `json:"access_IO_group_count"`
	ParentMdiskGrpID    string `json:"parent_mdisk_grp_id"`
	ParentMdiskGrpName  string `json:"parent_mdisk_grp_name"`
	Encrypt             string `json:"encrypt"`
	VolumeID            string `json:"volume_id"`
	VolumeName          string `json:"volume_name"`
	Protocol            string `json:"protocol"`

	// Cloud & Snapshot Capabilities
	CloudBackupEnabled            string `json:"cloud_backup_enabled"`
	DeduplicatedCopyCount         string `json:"deduplicated_copy_count"`
	IsSnapshot                    string `json:"is_snapshot"`
	SnapshotCount                 string `json:"snapshot_count"`
	ProtectionProvisionedCapacity string `json:"protection_provisioned_capacity"`
	ProtectionWrittenCapacity     string `json:"protection_written_capacity"`
	IsSafeguardedSnapshot         string `json:"is_safeguarded_snapshot"`
	SafeguardedSnapshotCount      string `json:"safeguarded_snapshot_count"`

	// Copy Specifics (Thin Provisioning & Tiering)
	CopyID                   string `json:"copy_id"`
	Sync                     string `json:"sync"`
	Primary                  string `json:"primary"`
	UsedCapacity             string `json:"used_capacity"`
	RealCapacity             string `json:"real_capacity"`
	FreeCapacity             string `json:"free_capacity"`
	Overallocation           string `json:"overallocation"`
	Autoexpand               string `json:"autoexpand"`
	Warning                  string `json:"warning"`
	Grainsize                string `json:"grainsize"`
	SECopy                   string `json:"se_copy"`
	EasyTier                 string `json:"easy_tier"`
	EasyTierStatus           string `json:"easy_tier_status"`
	CompressedCopy           string `json:"compressed_copy"`
	UncompressedUsedCapacity string `json:"uncompressed_used_capacity"`
	DeduplicatedCopy         string `json:"deduplicated_copy"`
}

// LsVdisk retrieves a list of all volumes on the FlashSystem.
// The REST API returns an array of objects for this endpoint.
func (c *Client) LsVdisk() ([]Vdisk, error) {
	data, err := c.post("lsvdisk", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list vdisks: %w", decodeIBMError(err))
	}

	var vdisks []Vdisk
	if err := json.Unmarshal(data, &vdisks); err != nil {
		// Log the unmarshal failure with structured logging
		return nil, fmt.Errorf("failed to parse lsvdisk response: %v", err)
	}

	return vdisks, nil
}

// LsVdiskByName retrieves detailed information for a specific volume by its name or ID.
func (c *Client) LsVdiskByName(target string) (*Vdisk, error) {
	endpoint := fmt.Sprintf("lsvdisk/%s", target)
	data, err := c.post(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get vdisk details for %s: %w", target, decodeIBMError(err))
	}

	// The API returns an array even for a single target, so we unmarshal into a slice
	var vdisks []Vdisk
	if err := json.Unmarshal(data, &vdisks); err != nil {
		// Log the unmarshal failure and include the target name for context
		return nil, fmt.Errorf("failed to parse lsvdisk/%s response: %v", target, err)
	}

	// Safety check: Ensure the array isn't empty
	if len(vdisks) == 0 {
		return nil, fmt.Errorf("volume %s not found (empty response array)", target)
	}

	// Return a pointer to the first (and only) item in the array
	return &vdisks[0], nil
}