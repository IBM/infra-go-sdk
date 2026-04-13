package svc

import (
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
	if reqBody.Host == "" || reqBody.VDisk == "" {
		return fmt.Errorf("host and vdisk are required")
	}

	payload := make(map[string]interface{})
	payload["host"] = reqBody.Host
	if reqBody.SCSI != "" {
		payload["scsi"] = reqBody.SCSI
	}
	if reqBody.Force {
		payload["force"] = true
	}
	
	endpoint := fmt.Sprintf("mkvdiskhostmap/%s", reqBody.VDisk)
	_, err := c.post(endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to create volume host mapping: %w", decodedErr)
	}

	return nil
}