package svc

import (
	"encoding/json"
	"fmt"
)

// Host represents a host object from the lshost API response
type Hosts struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	PortCount         string `json:"port_count"`
	Type              string `json:"type"`
	IOGrpCount        string `json:"iogrp_count"`
	Status            string `json:"status"`
	SiteID            string `json:"site_id"`
	SiteName          string `json:"site_name"`
	HostClusterID     string `json:"host_cluster_id"`
	HostClusterName   string `json:"host_cluster_name"`
	Protocol          string `json:"protocol"`
	StatusPolicy      string `json:"status_policy"`
	StatusSite        string `json:"status_site"`
	NodeLoggedInCount string `json:"node_logged_in_count"`
	State             string `json:"state"`
	PortsetID         string `json:"portset_id"`
	PortsetName       string `json:"portset_name"`
	OwnerID           string `json:"owner_id"`
	OwnerName         string `json:"owner_name"`
}

// Lshost retrieves a list of all defined hosts
func (c *Client) Lshost() ([]Hosts, error) {
	data, err := c.post("lshost", nil)
	if err != nil {
		var errResp ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return nil, fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
		}
		return nil, fmt.Errorf("failed to list hosts: %v", err)
	}

	var hosts []Hosts
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return hosts, nil
}

// LshostByTarget retrieves details for a specific host.
// Note: The API returns a single object for specific resource requests.
func (c *Client) LshostByTarget(target string) (*Hosts, error) {
    endpoint := fmt.Sprintf("lshost/%s", target)
    data, err := c.post(endpoint, nil)
    if err != nil {
        var errResp ErrorResponse
        if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
            return nil, fmt.Errorf("error %s: %s", errResp.Code, errResp.Description)
        }
        return nil, fmt.Errorf("failed to get host details for %s: %v", target, err)
    }

    // Changed from []Host to a single Host pointer
    var host Hosts
    if err := json.Unmarshal(data, &host); err != nil {
        return nil, fmt.Errorf("failed to parse response: %v", err)
    }

    return &host, nil
}