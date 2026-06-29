package svc

import (
	"context"
	"fmt"
)

// Rmvdiskhostmap removes the SCSI/NVMe mapping between a specific volume and a host.
// It does NOT delete the volume or the data on it.
func (c *Client) Rmvdiskhostmap(ctx context.Context, host string, vdisk string) error {
	if host == "" || vdisk == "" {
		return fmt.Errorf("host and vdisk are required, host:%s, vdisk:%s", host, vdisk)
	}

	endpoint := fmt.Sprintf("rmvdiskhostmap/%s", vdisk)
	
	payload := map[string]interface{}{
		"host": host,
	}

	_, err := c.post(ctx,endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to unmap vdisk %s from host %s: %w", vdisk, host, decodedErr)
	}

	return nil
}
