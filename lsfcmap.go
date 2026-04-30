package svc

import (
	"context"
	"encoding/json"
	"fmt"
)

// FlashCopyMappingInfo represents the detailed information for a FlashCopy mapping from lsfcmap API response
type FlashCopyMappingInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	SourceVDiskID    string `json:"source_vdisk_id"`
	SourceVDiskName  string `json:"source_vdisk_name"`
	TargetVDiskID    string `json:"target_vdisk_id"`
	TargetVDiskName  string `json:"target_vdisk_name"`
	ConsistGrpID     string `json:"group_id"`
	ConsistGrpName   string `json:"group_name"`
	Status           string `json:"status"`
	Progress         string `json:"progress"`
	CopyRate         string `json:"copy_rate"`
	CleanProgress    string `json:"clean_progress"`
	Incremental      string `json:"incremental"`
	PartnerFCMapID   string `json:"partner_FC_id"`
	PartnerFCMapName string `json:"partner_FC_name"`
	Restoring        string `json:"restoring"`
	StartTime        string `json:"start_time"`
	RCControlled     string `json:"rc_controlled"`
	SizeMismatch     string `json:"size_mismatch"`
	IsSnapshot       string `json:"is_snapshot"`
	SnapshotID       string `json:"snapshot_id"`
}

// Lsfcmap retrieves information about FlashCopy mappings using the lsfcmap API endpoint
func (c *Client) Lsfcmap(ctx context.Context,name string) ([]FlashCopyMappingInfo, error) {
	endpoint := "lsfcmap"
	if name != "" {
		endpoint = fmt.Sprintf("lsfcmap/%s", name)
	}

	data, err := c.post(ctx,endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list FlashCopy mappings: %w", decodeIBMError(err))
	}

	var result []FlashCopyMappingInfo
	if name != "" {
		// Try unmarshaling as a single object first (specific mapping)
		var mapping FlashCopyMappingInfo
		if err := json.Unmarshal(data, &mapping); err == nil && mapping.ID != "" {
			// Successful single object response
			result = []FlashCopyMappingInfo{mapping}
		} else {
			// Fall back to array response (for consistency or unexpected cases)
			c.Logger.Debug("Falling back to array parsing for FlashCopy mapping")
			var mappings []FlashCopyMappingInfo
			if err := json.Unmarshal(data, &mappings); err != nil {
				return nil, fmt.Errorf("failed to parse response: %v", err)
			}
			if len(mappings) == 0 {
				return nil, fmt.Errorf("no FlashCopy mapping found with name: %s", name)
			}
			// Filter mappings client-side to ensure exact match
			for _, m := range mappings {
				if m.Name == name {
					result = []FlashCopyMappingInfo{m}
					break
				}
			}
			if len(result) == 0 {
				return nil, fmt.Errorf("no FlashCopy mapping found with name: %s", name)
			}
		}
	} else {
		// For all mappings, expect an array of mapping objects
		var mappings []FlashCopyMappingInfo
		if err := json.Unmarshal(data, &mappings); err != nil {
			return nil, fmt.Errorf("failed to parse response: %v", err)
		}
		if len(mappings) == 0 {
			return nil, fmt.Errorf("no FlashCopy mappings found")
		}
		result = mappings
	}

	return result, nil
}