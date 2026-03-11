package svc

import (
	"encoding/json"
	"fmt"
)

// VolumeRemove represents the body for the rmvdisk API call
type VolumeRemove struct {
	Force              bool `json:"force,omitempty"`              // true to force deletion if mappings exist
	RemoveHostMappings bool `json:"removehostmappings,omitempty"` // true to remove host mappings before deletion
}

// Rmvdisk sends a POST request to /rmvdisk/<name> to delete a volume
func (c *Client) Rmvdisk(name string, reqBody VolumeRemove) error {
	// Validate required fields
	if name == "" {
		return fmt.Errorf("name is required: CMMVC5701E No object ID was specified")
	}

	if reqBody.Force && reqBody.RemoveHostMappings {
		return fmt.Errorf("parameters 'force' and 'removehostmappings' cannot be used together")
	}

	endpoint := "rmvdisk"
	if name != "" {
		endpoint = fmt.Sprintf("rmvdisk/%s", name)
	}
	// Convert VolumeRemove to a map for the post method
	payload := make(map[string]interface{})

	if reqBody.Force {
		payload["force"] = true
	} else if reqBody.RemoveHostMappings {
		payload["removehostmappings"] = true
	}

	_, err := c.post(endpoint, payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to delete volume: %v", err)
	}

	return nil
}
