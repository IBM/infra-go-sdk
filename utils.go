package svc

import (
	"fmt"
	"strings"
)

// GetHostByWWPN searches for any of the provided WWPNs in the fabric logins
// and returns the associated host name and matching WWPN if found
func (c *Client) GetHostByWWPN(wwpns []string) (string, string, error) {
	if len(wwpns) == 0 {
		if c.Logger != nil {
			c.Logger.Warn("GetHostByWWPN called with no WWPNs")
		}
		return "", "", fmt.Errorf("no WWPNs provided")
	}

	if c.Logger != nil {
		c.Logger.Debug("Looking up host by WWPNs", "wwpn_count", len(wwpns))
	}

	logins, err := c.Lsfabric()
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Failed to retrieve fabric logins", "error", err)
		}
		return "", "", err
	}

	if c.Logger != nil {
		c.Logger.Debug("Retrieved fabric logins", "login_count", len(logins))
	}

	// Normalize WWPNs to uppercase for comparison
	normalizedWWPNs := make(map[string]string, len(wwpns))
	for _, wwpn := range wwpns {
		normalizedWWPNs[strings.ToUpper(strings.TrimSpace(wwpn))] = wwpn
	}

	// Search for any matching WWPN in fabric logins
	for _, login := range logins {
		upperRemoteWWPN := strings.ToUpper(strings.TrimSpace(login.RemoteWWPN))
		if originalWWPN, found := normalizedWWPNs[upperRemoteWWPN]; found {
			if c.Logger != nil {
				c.Logger.Info("Matched WWPN to host", "host", login.HostName, "wwpn", originalWWPN)
			}
			return login.HostName, originalWWPN, nil
		}
	}

	if c.Logger != nil {
		c.Logger.Warn("No matching WWPN found in fabric", "requested_wwpns", wwpns)
	}

	return "", "", fmt.Errorf("none of the provided WWPNs found in fabric: %v", wwpns)
}
