package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyMappingID represents the body for the prestartfcmap API call
type FlashCopyMappingID struct {
	ID string `json:"id"` // ID or name of the FlashCopy mapping
}

// Prestartfcmap sends a POST request to /prestartfcmap to prepare a FlashCopy mapping
func (c *Client) Prestartfcmap(reqBody FlashCopyMappingID) error {
	// Validate required fields
	if reqBody.ID == "" {
		return fmt.Errorf("id is required")
	}

	// Convert FlashCopyMappingID to a map for the post method
	payload := make(map[string]interface{})
	payload["id"] = reqBody.ID
	endpoint := fmt.Sprintf("prestartfcmap/%s", reqBody.ID)
	_, err := c.post(endpoint, nil)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to prepare FlashCopy mapping: %v", err)
	}

	return nil
}
