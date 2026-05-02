package svc

import (
	"context"
	"fmt"
)

type FlashCopyConsistGroupID struct {
	ID string `json:"id"`
}

func (c *Client) Prestartfcconsistgrp(ctx context.Context, reqBody FlashCopyConsistGroupID) error {
	if reqBody.ID == "" {
		return fmt.Errorf("id is required")
	}

	payload := make(map[string]interface{})
	payload["id"] = reqBody.ID

	_, err := c.post(ctx,"prestartfcconsistgrp", payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to prepare FlashCopy consistency group %s: %w", reqBody.ID, decodedErr)
	}

	return nil
}