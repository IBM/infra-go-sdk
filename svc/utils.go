package svc

import (
	"context"
	"fmt"
	"strings"
)

// validateContext checks if the provided context is valid (not nil).
// Returns an error if the context is nil, otherwise returns nil.
func validateContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	return nil
}

// GetHostByWWPN searches for any of the provided WWPNs in the fabric logins
// and returns the associated host name and matching WWPN if found
func (c *Client) GetHostByWWPN(ctx context.Context, wwpns []string) (string, string, error) {
	if len(wwpns) == 0 {
		return "", "", fmt.Errorf("no WWPNs provided")
	}

	logins, err := c.Lsfabric(ctx)
	if err != nil {
		return "", "", err
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
			return login.HostName, originalWWPN, nil
		}
	}

	return "", "", fmt.Errorf("none of the provided WWPNs found in fabric: %v", wwpns)
}
