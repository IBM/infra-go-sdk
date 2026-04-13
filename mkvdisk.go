package svc

import (
	"fmt"
)

// Volume represents the body for the mkvdisk API call
type Volume struct {
	Name       string `json:"name"`
	MdiskGrp   string `json:"mdiskgrp"`
	Size       int    `json:"size"`
	Unit       string `json:"unit,omitempty"` // e.g., "gb"
	IOGrp      string `json:"iogrp,omitempty"`
	RSize      string `json:"rsize,omitempty"`      // For thin provisioning, e.g., "2%"
	Warning    string `json:"warning,omitempty"`    // e.g., "80%"
	AutoExpand bool   `json:"autoexpand,omitempty"` // true for autoexpand
	GrainSize  *int   `json:"grainsize,omitempty"`  // e.g., 256
}

// Mkvdisk sends a POST request to /mkvdisk to create a volume
func (c *Client) Mkvdisk(reqBody Volume) error {
	// Validate required fields
	if reqBody.Name == "" || reqBody.MdiskGrp == "" || reqBody.Size <= 0 {
		return fmt.Errorf("name, mdiskgrp, and size are required")
	}

	// Convert Volume to a map for the post method
	payload := make(map[string]interface{})
	payload["name"] = reqBody.Name
	payload["mdiskgrp"] = reqBody.MdiskGrp
	payload["size"] = reqBody.Size
	if reqBody.Unit != "" {
		payload["unit"] = reqBody.Unit
	}
	if reqBody.IOGrp != "" {
		payload["iogrp"] = reqBody.IOGrp
	}
	if reqBody.RSize != "" {
		payload["rsize"] = reqBody.RSize
	}
	if reqBody.Warning != "" {
		payload["warning"] = reqBody.Warning
	}
	if reqBody.AutoExpand {
		payload["autoexpand"] = reqBody.AutoExpand
	}
	if reqBody.GrainSize != nil {
		payload["grainsize"] = *reqBody.GrainSize
	}

	_, err := c.post("mkvdisk", payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to create volume: %w", decodedErr)
	}

	return nil
}