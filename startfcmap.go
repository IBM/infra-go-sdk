package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyMappingStart represents the body for the startfcmap API call
type FlashCopyMappingStart struct {
	ID      string `json:"id"`                // ID or name of the FlashCopy mapping
	Prep    bool   `json:"prep,omitempty"`    // true to prepare the mapping before starting
	Restore bool   `json:"restore,omitempty"` // true to force start if target is in use
}

// Startfcmap sends a POST request to /startfcmap to start a FlashCopy mapping
func (c *Client) Startfcmap(reqBody FlashCopyMappingStart) error {
	// Validate required fields
	if reqBody.ID == "" {
		return fmt.Errorf("id is required: CMMVC5701E No object ID was specified")
	}
	fmt.Printf("ID: %s\n", reqBody.ID)
	// Convert FlashCopyMappingStart to a map for the post method
	payload := make(map[string]interface{})
	if reqBody.Prep {
		payload["prep"] = reqBody.Prep
	}
	if reqBody.Restore {
		payload["restore"] = reqBody.Restore
	}
	endpoint := fmt.Sprintf("startfcmap/%s", reqBody.ID)

	_, err := c.post(endpoint, payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to start FlashCopy mapping: %v", err)
	}

	return nil
}
