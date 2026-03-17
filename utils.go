package svc

import (
	"fmt"
	"strings"
)

// GetHostByWWPN searches for any of the provided WWPNs in the fabric logins
// and returns the associated host name and matching WWPN if found
func (c *Client) GetHostByWWPN(wwpns []string) (string, string, error) {
	if len(wwpns) == 0 {
		return "", "", fmt.Errorf("no WWPNs provided")
	}

	logins, err := c.Lsfabric()
	if err != nil {
		return "", "", err
	}

	// Normalize WWPNs to uppercase for comparison
	normalizedWWPNs := make(map[string]string)
	for _, wwpn := range wwpns {
		normalizedWWPNs[strings.ToUpper(wwpn)] = wwpn
	}

	// Search for any matching WWPN in fabric logins
	for _, login := range logins {
		upperRemoteWWPN := strings.ToUpper(login.RemoteWWPN)
		if originalWWPN, found := normalizedWWPNs[upperRemoteWWPN]; found {
			return login.HostName, originalWWPN, nil
		}
	}

	return "", "", fmt.Errorf("none of the provided WWPNs found in fabric: %v", wwpns)
}
