package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyConsistGroupID represents the body for the prestartfcconsistgrp API call
type FlashCopyConsistGroupID struct {
	ID string `json:"id"` // ID or name of the consistency group
}

// Prestartfcconsistgrp sends a POST request to /prestartfcconsistgrp to prepare a FlashCopy consistency group
func (c *Client) Prestartfcconsistgrp(reqBody FlashCopyConsistGroupID) error {
	// Validate required fields
	if reqBody.ID == "" {
		return fmt.Errorf("id is required")
	}

	// Convert FlashCopyConsistGroupID to a map for the post method
	payload := make(map[string]interface{})
	payload["id"] = reqBody.ID

	_, err := c.post("prestartfcconsistgrp", payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to prepare FlashCopy consistency group: %v", err)
	}

	return nil
}
