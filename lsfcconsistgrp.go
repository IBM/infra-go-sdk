package svc

import (
	"context"
	"encoding/json"
	"fmt"
)

// FlashCopyConsistGroupMappingInfo represents a FlashCopy mapping within a consistency group
type FlashCopyConsistGroupMappingInfo struct {
	FCMappingID   string `json:"FC_mapping_id"`
	FCMappingName string `json:"FC_mapping_name"`
}

// FlashCopyConsistGroupInfo represents the detailed information for a FlashCopy consistency group from lsfcconsistgrp API response
type FlashCopyConsistGroupInfo struct {
	ID           string                             `json:"id"`
	Name         string                             `json:"name"`
	Status       string                             `json:"status"`
	StartTime    string                             `json:"start_time"`
	OwnerID      string                             `json:"owner_id"`
	OwnerName    string                             `json:"owner_name"`
	SizeMismatch string                             `json:"size_mismatch"`
	Mappings     []FlashCopyConsistGroupMappingInfo `json:"-"` // Populated manually from response
}

// Lsfcconsistgrp retrieves information about FlashCopy consistency groups using the lsfcconsistgrp API endpoint
func (c *Client) Lsfcconsistgrp(ctx context.Context,name string) ([]FlashCopyConsistGroupInfo, error) {
	// Construct endpoint with or without name
	endpoint := "lsfcconsistgrp"
	if name != "" {
		endpoint = fmt.Sprintf("lsfcconsistgrp/%s", name)
	}

	data, err := c.post(ctx,endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list FlashCopy consistency groups: %w", decodeIBMError(err))
	}

	var result []FlashCopyConsistGroupInfo
	if name != "" {
		// Try unmarshaling as a single object first (group without mappings)
		var group FlashCopyConsistGroupInfo
		if err := json.Unmarshal(data, &group); err == nil && group.ID != "" {
			// Successful single object response
			result = []FlashCopyConsistGroupInfo{group}
		} else {
			// Fall back to array response (group with mappings)
			c.Logger.Debug("Falling back to array parsing for consistency group mappings")
			var rawResponse []map[string]interface{}
			if err := json.Unmarshal(data, &rawResponse); err != nil {
				return nil, fmt.Errorf("failed to parse response: %v", err)
			}

			if len(rawResponse) == 0 {
				return nil, fmt.Errorf("no FlashCopy consistency group found with name: %s", name)
			}

			var mappings []FlashCopyConsistGroupMappingInfo
			var foundGroup bool

			for _, item := range rawResponse {
				if _, ok := item["id"]; ok {
					// It's a consistency group
					b, _ := json.Marshal(item)
					if err := json.Unmarshal(b, &group); err != nil {
						return nil, fmt.Errorf("failed to parse group data: %v", err)
					}
					foundGroup = true
				} else if _, ok := item["FC_mapping_id"]; ok {
					// It's a mapping
					var m FlashCopyConsistGroupMappingInfo
					b, _ := json.Marshal(item)
					if err := json.Unmarshal(b, &m); err != nil {
						return nil, fmt.Errorf("failed to parse mapping data: %v", err)
					}
					mappings = append(mappings, m)
				}
			}

			if !foundGroup {
				return nil, fmt.Errorf("no FlashCopy consistency group found with name: %s", name)
			}

			// Assign mappings to the group (may be empty if no mappings exist)
			group.Mappings = mappings
			result = []FlashCopyConsistGroupInfo{group}
		}
	} else {
		// For all groups, expect an array of group objects without mappings
		var groups []FlashCopyConsistGroupInfo
		if err := json.Unmarshal(data, &groups); err != nil {
			return nil, fmt.Errorf("failed to parse response: %v", err)
		}
		if len(groups) == 0 {
			return nil, fmt.Errorf("no FlashCopy consistency groups found")
		}
		result = groups
	}

	return result, nil
}