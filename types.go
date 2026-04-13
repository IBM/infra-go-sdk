package svc

import (
	"encoding/json"
	"fmt"
)

// ErrorResponse represents a possible error response from the API
type ErrorResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

func decodeIBMError(err error) error {
	if err == nil {
		return nil
	}

	var errResp ErrorResponse
	if json.Unmarshal([]byte(err.Error()), &errResp) == nil && (errResp.Code != "" || errResp.Description != "") {
		return fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
	}

	return err
}
