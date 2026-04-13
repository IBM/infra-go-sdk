package svc

import (
	"fmt"
)

// VolumeRemove represents the body for the rmvdisk API call
type VolumeRemove struct {
	Force              bool `json:"force,omitempty"`              // true to force deletion if mappings exist
	RemoveHostMappings bool `json:"removehostmappings,omitempty"` // true to remove host mappings before deletion
}

// Rmvdisk sends a POST request to /rmvdisk/<name> to delete a volume
func (c *Client) Rmvdisk(name string, reqBody VolumeRemove) error {
	if name == "" {
		return fmt.Errorf("name is required: CMMVC5701E No object ID was specified")
	}

	if reqBody.Force && reqBody.RemoveHostMappings {
		return fmt.Errorf("parameters 'force' and 'removehostmappings' cannot be used together")
	}

	endpoint := fmt.Sprintf("rmvdisk/%s", name)
	payload := make(map[string]interface{})

	if reqBody.Force {
		payload["force"] = true
	} else if reqBody.RemoveHostMappings {
		payload["removehostmappings"] = true
	}

	_, err := c.post(endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to delete volume %s: %w", name, decodedErr)
	}

	return nil
}