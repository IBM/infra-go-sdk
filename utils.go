package svc

import (
	"fmt"
)

// GetHostByWWPN searches for a WWPN in the fabric logins and returns the associated host name if found
func (c *Client) GetHostByWWPN(wwpn string) (string, error) {
	logins, err := c.Lsfabric()
	if err != nil {
		return "", err
	}

	for _, login := range logins {
		if login.RemoteWWPN == wwpn {
			//fmt.Printf("HOSTNAMES: %v", login)
			return login.HostName, nil
		}
	}

	return "", fmt.Errorf("WWPN %s not found in any host", wwpn)
}
