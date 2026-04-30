package svc

import (
	"context"
	"fmt"
)

type FlashCopyMappingRemove struct {
	Force bool `json:"force,omitempty"`
}

func (c *Client) Rmfcmap(ctx context.Context,name string, reqBody FlashCopyMappingRemove) error {
	if name == "" {
		return fmt.Errorf("name is required: CMMVC5701E No object ID was specified")
	}

	endpoint := fmt.Sprintf("rmfcmap/%s", name)
	payload := make(map[string]interface{})
	if reqBody.Force {
		payload["force"] = reqBody.Force
	}

	_, err := c.post(ctx,endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to delete FlashCopy mapping: %w , name: %s", decodedErr, name)
	}

	return nil
}