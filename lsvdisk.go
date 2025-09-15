package svc

import (
	"encoding/json"
	"fmt"
)

// VolumeInfo represents the detailed information for a volume from lsvdisk API response
type VolumeInfo struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	IOGroupID                string `json:"IO_group_id"`
	IOGroupName              string `json:"IO_group_name"`
	Status                   string `json:"status"`
	MDiskGrpID               string `json:"mdisk_grp_id"`
	MDiskGrpName             string `json:"mdisk_grp_name"`
	Capacity                 string `json:"capacity"`
	Type                     string `json:"type"`
	FCID                     string `json:"FC_id"`
	FCName                   string `json:"FC_name"`
	RCID                     string `json:"RC_id"`
	RCName                   string `json:"RC_name"`
	VdiskUID                 string `json:"vdisk_UID"`
	FCMapCount               string `json:"fc_map_count"` // Changed to string
	CopyCount                string `json:"copy_count"`   // Changed to string
	FastWriteState           string `json:"fast_write_state"`
	SECopyCount              string `json:"se_copy_count"` // Changed to string
	RCChange                 string `json:"RC_change"`
	CompressedCopyCount      string `json:"compressed_copy_count"` // Changed to string
	ParentMDiskGrpID         string `json:"parent_mdisk_grp_id"`
	ParentMDiskGrpName       string `json:"parent_mdisk_grp_name"`
	OwnerID                  string `json:"owner_id"`
	OwnerName                string `json:"owner_name"`
	Formatting               string `json:"formatting"`
	Encrypt                  string `json:"encrypt"`
	VolumeID                 string `json:"volume_id"`
	VolumeName               string `json:"volume_name"`
	Function                 string `json:"function"`
	VolumeGroupID            string `json:"volume_group_id"`
	VolumeGroupName          string `json:"volume_group_name"`
	Protocol                 string `json:"protocol"`
	IsSnapshot               string `json:"is_snapshot"`
	SnapshotCount            string `json:"snapshot_count"` // Changed to string
	VolumeType               string `json:"volume_type"`
	ReplicationMode          string `json:"replication_mode"`
	IsSafeguardedSnapshot    string `json:"is_safeguarded_snapshot"`
	SafeguardedSnapshotCount string `json:"safeguarded_snapshot_count"` // Changed to string
}

// LsVdisk retrieves the complete list of volumes using the lsvdisk API endpoint
func (c *Client) LsVdisk() ([]VolumeInfo, error) {
	data, err := c.post("lsvdisk", nil)
	if err != nil {
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return nil, fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return nil, fmt.Errorf("failed to list volumes: %v", err)
	}

	var volumes []VolumeInfo
	if err := json.Unmarshal(data, &volumes); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return volumes, nil
}
