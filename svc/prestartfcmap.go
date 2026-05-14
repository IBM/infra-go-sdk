package svc

import (
	"context"
	"fmt"
)

type FlashCopyMappingID struct {
	ID string `json:"id"`
}

func (c *Client) Prestartfcmap(ctx context.Context, reqBody FlashCopyMappingID) error {
	if reqBody.ID == "" {
		return fmt.Errorf("id is required")
	}

	endpoint := fmt.Sprintf("prestartfcmap/%s", reqBody.ID)
	
	_, err := c.post(ctx,endpoint, nil) // IBM API expects empty body for this endpoint
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to prepare FlashCopy mapping %s: %w", reqBody.ID, decodedErr)
	}

	return nil
}