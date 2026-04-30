package svc

import (
	"context"
	"fmt"
)

type FlashCopyConsistGroup struct {
	Name       string `json:"name,omitempty"`
	AutoDelete bool   `json:"autodelete,omitempty"`
}

func (c *Client) Mkfcconsistgrp(ctx context.Context,reqBody FlashCopyConsistGroup) error {
	payload := make(map[string]interface{})
	if reqBody.Name != "" {
		payload["name"] = reqBody.Name
	}
	if reqBody.AutoDelete {
		payload["autodelete"] = true
	}
	
	_, err := c.post(ctx,"mkfcconsistgrp", payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to create FlashCopy consistency group: %w", decodedErr)
	}

	return nil
}