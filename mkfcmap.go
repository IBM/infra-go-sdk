package svc

import (
	"encoding/json"
	"fmt"
)

// FlashCopyMapping represents the body for the mkfcmap API call
type FlashCopyMapping struct {
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	ConsistGrp  string `json:"consistgrp,omitempty"`
	AutoDelete  bool   `json:"autodelete,omitempty"`
	GrainSize   int    `json:"grainsize,omitempty"` // e.g., 64 or 256
	Incremental bool   `json:"incremental,omitempty"`
	CopyRate    int    `json:"copyrate,omitempty"`  // 0-150
	CleanRate   int    `json:"cleanrate,omitempty"` // 0-150
	KeepTarget  bool   `json:"keeptarget,omitempty"`
	IOGrp       string `json:"iogrp,omitempty"`
}

// Mkfcmap sends a POST request to /mkfcmap to create a FlashCopy mapping
func (c *Client) Mkfcmap(reqBody FlashCopyMapping) error {
	// Validate required fields
	if reqBody.Source == "" || reqBody.Target == "" {
		return fmt.Errorf("source and target are required")
	}

	// Convert FlashCopyMapping to a map for the post method
	payload := make(map[string]interface{})
	if reqBody.Name != "" {
		payload["name"] = reqBody.Name
	}
	payload["source"] = reqBody.Source
	payload["target"] = reqBody.Target
	if reqBody.ConsistGrp != "" {
		payload["consistgrp"] = reqBody.ConsistGrp
	}
	payload["autodelete"] = reqBody.AutoDelete
	if reqBody.GrainSize != 0 {
		payload["grainsize"] = reqBody.GrainSize
	}
	payload["incremental"] = reqBody.Incremental
	if reqBody.CopyRate != 0 {
		payload["copyrate"] = reqBody.CopyRate
	}
	if reqBody.CleanRate != 0 {
		payload["cleanrate"] = reqBody.CleanRate
	}
	payload["keeptarget"] = reqBody.KeepTarget
	if reqBody.IOGrp != "" {
		payload["iogrp"] = reqBody.IOGrp
	}

	_, err := c.post("mkfcmap", payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to create FlashCopy mapping: %v", err)
	}

	return nil
}
