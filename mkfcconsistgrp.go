package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyConsistGroup represents the body for the mkfcconsistgrp API call
type FlashCopyConsistGroup struct {
	Name       string `json:"name,omitempty"`       // Optional name for the consistency group
	AutoDelete bool   `json:"autodelete,omitempty"` // Optional name for the consistency group
}

// Mkfcconsistgrp sends a POST request to /mkfcconsistgrp to create a FlashCopy consistency group
func (c *Client) Mkfcconsistgrp(reqBody FlashCopyConsistGroup) error {
	// Convert FlashCopyConsistGroup to a map for the post method
	payload := make(map[string]interface{})
	if reqBody.Name != "" {
		payload["name"] = reqBody.Name
	}
	payload["autodelete"] = reqBody.AutoDelete
	_, err := c.post("mkfcconsistgrp", payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to create FlashCopy consistency group: %v", err)
	}

	return nil
}
