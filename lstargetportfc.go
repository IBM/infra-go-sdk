package svc

import (
	"encoding/json"
	"fmt"
)

// TargetPortFC represents a Virtual NPIV Fibre Channel port object
// from the lstargetportfc API response.
type TargetPortFC struct {
	ID              string `json:"id"`
	FC_IO_Port_ID   string `json:"fc_io_port_id"`
	PortID          string `json:"port_id"`
	NodeID          string `json:"node_id"`
	NodeName        string `json:"node_name"`
	WWPN            string `json:"WWPN"`
	PortSpeed       string `json:"port_speed"`
	NPortID         string `json:"nportid"`
	Status          string `json:"status"`
	HostIOPermitted string `json:"host_io_permitted"` 
}

// Lstargetportfc retrieves a list of all Virtual (NPIV) Fibre Channel target ports on the system.
func (c *Client) Lstargetportfc() ([]TargetPortFC, error) {
	// Send the POST request to execute the lstargetportfc command
	data, err := c.post("lstargetportfc", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual FC target ports: %w", decodeIBMError(err))
	}

	var targetPorts []TargetPortFC
	if err := json.Unmarshal(data, &targetPorts); err != nil {
		// Log the unmarshal failure with structured logging
		return nil, fmt.Errorf("failed to parse lstargetportfc response: %v", err)
	}

	return targetPorts, nil
}