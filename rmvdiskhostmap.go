package svc

import (
	"encoding/json"
	"fmt"
)

// Rmvdiskhostmap removes the SCSI/NVMe mapping between a specific volume and a host.
// It does NOT delete the volume or the data on it.
func (c *Client) Rmvdiskhostmap(host string, vdisk string) error {
	// The IBM REST API expects the target object (vdisk) in the URI path,
	// and the command flags (like -host) in the JSON payload.
	endpoint := fmt.Sprintf("rmvdiskhostmap/%s", vdisk)
	
	payload := map[string]interface{}{
		"host": host,
	}

	// Send the POST request to execute the unmap
	_, err := c.post(endpoint, payload)
	if err != nil {
		var errResp ErrorResponse
		// Catch specific IBM API error codes
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return fmt.Errorf("failed to unmap vdisk %s from host %s: %v", vdisk, host, err)
	}

	return nil
}