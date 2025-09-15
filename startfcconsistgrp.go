package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyConsistGroupStart represents the body for the startfcconsistgrp API call
type FlashCopyConsistGroupStart struct {
	ID      string `json:"id"`                // ID or name of the FlashCopy mapping
	Prep    bool   `json:"prep,omitempty"`    // true to prepare the group before starting
	Restore bool   `json:"restore,omitempty"` // true to force start if target is in use
}

// Startfcconsistgrp sends a POST request to /startfcconsistgrp/<name> to start a FlashCopy consistency group
func (c *Client) Startfcconsistgrp(reqBody FlashCopyConsistGroupStart) error {
	// Validate required fields
	if reqBody.ID == "" {
		return fmt.Errorf("name is required: CMMVC5701E No object ID was specified")
	}
	fmt.Printf("Name: %s\n", reqBody.ID)

	// Convert FlashCopyConsistGroupStart to a map for the post method
	payload := make(map[string]interface{})
	if reqBody.Prep {
		payload["prep"] = reqBody.Prep
	}
	if reqBody.Restore {
		payload["restore"] = reqBody.Restore
	}
	endpoint := fmt.Sprintf("startfcconsistgrp/%s", reqBody.ID)
	// Debug: Log the payload for inspection
	fmt.Printf("Startfcconsistgrp payload: %v\n", payload)

	_, err := c.post(endpoint, payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to start FlashCopy consistency group: %v", err)
	}

	return nil
}
