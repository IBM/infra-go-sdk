package svc

import (
	"fmt"
)

type FlashCopyMappingStart struct {
	ID      string `json:"id"`
	Prep    bool   `json:"prep,omitempty"`
	Restore bool   `json:"restore,omitempty"`
}

func (c *Client) Startfcmap(reqBody FlashCopyMappingStart) error {
	if reqBody.ID == "" {
		return fmt.Errorf("id is required: CMMVC5701E No object ID was specified")
	}

	payload := make(map[string]interface{})
	if reqBody.Prep {
		payload["prep"] = reqBody.Prep
	}
	if reqBody.Restore {
		payload["restore"] = reqBody.Restore
	}
	
	endpoint := fmt.Sprintf("startfcmap/%s", reqBody.ID)
	_, err := c.post(endpoint, payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to start FlashCopy mapping:%s, %w", reqBody.ID,decodedErr)
	}

	return nil
}