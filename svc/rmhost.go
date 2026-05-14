package svc

import (
	"context"
	"fmt"
)

// HostRemove represents the body for the rmhost API call
type HostRemove struct {
	Force bool `json:"force,omitempty"` // true to force deletion even if volume mappings exist
}

// Rmhost sends a POST request to /rmhost/<name> to delete a host from the storage
func (c *Client) Rmhost(ctx context.Context, name string, reqBody HostRemove) error {
	if name == "" {
		return fmt.Errorf("host name or id is required for deletion")
	}

	endpoint := fmt.Sprintf("rmhost/%s", name)
	
	// Create payload map if Force is true, otherwise pass nil
	var payload map[string]interface{}
	if reqBody.Force {
		payload = map[string]interface{}{
			"force": true,
		}
	}

	_, err := c.post(ctx, endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to delete host %s: %w", name, decodedErr)
	}

	return nil
}