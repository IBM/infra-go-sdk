package svc

import (
	"encoding/json"
	"fmt"
	"time"
)

// FabricLoginInfo represents an entry from lsfabric API response
type FabricLoginInfo struct {
	RemoteWWPN    string `json:"remote_wwpn"`
	RemoteNPortID string `json:"remote_nportid"`
	ID            string `json:"id"`
	NodeName      string `json:"node_name"`
	LocalWWPN     string `json:"local_wwpn"`
	LocalPort     string `json:"local_port"`
	LocalNPortID  string `json:"local_nportid"`
	State         string `json:"state"`
	HostName      string `json:"name"`
	ClusterName   string `json:"cluster_name"`
	RemoteType    string `json:"type"`
}

// Lsfabric retrieves information about Fibre Channel fabric logins using the lsfabric API endpoint
// This operation can take longer in large fabric environments, so we use an extended timeout
func (c *Client) Lsfabric() ([]FabricLoginInfo, error) {
	// Save original timeout and restore it after
	originalTimeout := c.HTTPClient.Timeout
	c.HTTPClient.Timeout = 300 * time.Second // 5 minutes for fabric operations
	defer func() {
		c.HTTPClient.Timeout = originalTimeout
	}()

	data, err := c.post("lsfabric", nil)
	if err != nil {
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return nil, fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return nil, fmt.Errorf("failed to list fabric logins: %v", err)
	}

	var logins []FabricLoginInfo
	if err := json.Unmarshal(data, &logins); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return logins, nil
}
