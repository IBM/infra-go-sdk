package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func (c *Client) Lsfabric(ctx context.Context) ([]FabricLoginInfo, error) {
	tempClient := &http.Client{
		Transport: c.HTTPClient.Transport,
		Timeout:   fabricTimeout,
	}

	data, err := c.postWithHTTPClient(ctx,tempClient, "lsfabric", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list fabric logins: %w", decodeIBMError(err))
	}

	var logins []FabricLoginInfo
	if err := json.Unmarshal(data, &logins); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return logins, nil
}