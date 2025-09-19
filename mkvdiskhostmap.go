package svc

import (
	"encoding/json"
	"fmt"
)

// VolumeHostMap represents the body for the mkvdiskhostmap API call
type VolumeHostMap struct {
	Host  string `json:"host"`            // Host ID or name
	SCSI  string `json:"scsi,omitempty"`  // SCSI LUN ID
	Force bool   `json:"force,omitempty"` // true to force multiple assignments
	VDisk string `json:"vdisk"`           // Volume ID or name
}

// Mkvdiskhostmap sends a POST request to /mkvdiskhostmap to create a volume to host mapping
func (c *Client) Mkvdiskhostmap(reqBody VolumeHostMap) error {
	// Validate required fields
	if reqBody.Host == "" || reqBody.VDisk == "" {
		return fmt.Errorf("host and vdisk are required")
	}

	// Convert VolumeHostMap to a map for the post method
	payload := make(map[string]interface{})
	payload["host"] = reqBody.Host
	if reqBody.SCSI != "" {
		payload["scsi"] = reqBody.SCSI
	}
	payload["force"] = reqBody.Force
	endpoint := fmt.Sprintf("mkvdiskhostmap/%s", reqBody.VDisk)
	_, err := c.post(endpoint, payload)
	if err != nil {
		// Attempt to parse error response for more details
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to create volume host mapping: %v", err)
	}

	return nil
}
