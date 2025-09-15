package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyMappingRemove represents the body for the rmfcmap API call
type FlashCopyMappingRemove struct {
	Force bool `json:"force,omitempty"` // true to force deletion if in stopped state
}

// Rmfcmap sends a POST request to /rmfcmap/<name> to delete a FlashCopy mapping
func (c *Client) Rmfcmap(name string, reqBody FlashCopyMappingRemove) error {
	// Validate required fields
	if name == "" {
		return fmt.Errorf("name is required: CMMVC5701E No object ID was specified")
	}
	fmt.Printf("Name: %s\n", name)
	endpoint := "rmfcmap"
	if name != "" {
		endpoint = fmt.Sprintf("rmfcmap/%s", name)
	}

	// Convert FlashCopyMappingRemove to a map for the post method
	payload := make(map[string]interface{})
	if reqBody.Force {
		payload["force"] = reqBody.Force
	}

	// Debug: Log the payload for inspection
	fmt.Printf("Rmfcmap payload: %v\n", payload)

	_, err := c.post(endpoint, payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to delete FlashCopy mapping: %v", err)
	}

	return nil
}
