package svc

import (
	"encoding/json"
	"fmt"
)

// PortFC represents a Fibre Channel port object from the lsportfc API response.
// Note: All fields are defined as strings to match the IBM Spectrum Virtualize REST API behavior.
type PortFC struct {
	ID              string `json:"id"`
	FC_IO_Port_ID   string `json:"fc_io_port_id"`
	PortID          string `json:"port_id"`
	Type            string `json:"type"`
	PortSpeed       string `json:"port_speed"`
	NodeID          string `json:"node_id"`
	NodeName        string `json:"node_name"`
	WWPN            string `json:"WWPN"` // IBM API often capitalizes WWPN in this response
	NPortID         string `json:"nportid"`
	Status          string `json:"status"`
	Attachment      string `json:"attachment"`
	ClusterUse      string `json:"cluster_use"`
	AdapterLocation string `json:"adapter_location"`
	AdapterPortID   string `json:"adapter_port_id"`
}

// Lsportfc retrieves a list of all physical Fibre Channel ports on the system.
func (c *Client) Lsportfc() ([]PortFC, error) {
	// Send the POST request to execute the lsportfc command
	data, err := c.post("lsportfc", nil)
	if err != nil {
		var errResp ErrorResponse
		// Catch specific IBM API error codes
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return nil, fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return nil, fmt.Errorf("failed to list FC ports: %v", err)
	}

	var ports []PortFC
	if err := json.Unmarshal(data, &ports); err != nil {
		return nil, fmt.Errorf("failed to parse lsportfc response: %v", err)
	}

	return ports, nil
}