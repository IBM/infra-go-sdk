package svc

import (
	"fmt"
	"strings"
)

// Host represents the parameters for creating a new host
type Host struct {
	Name     string
	Fcwwpn   []string
	Type     string
	Force    bool
	Protocol string
}

func (c *Client) Mkhost(host Host) error {
	if host.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(host.Fcwwpn) == 0 {
		return fmt.Errorf("Validation failed for Mkhost, missing fcwwpn host_name, %s", host.Name)
	}

	fcwwpn := strings.Join(host.Fcwwpn, ":")

	params := map[string]interface{}{
		"name":     host.Name,
		"fcwwpn":   fcwwpn,
		"type":     host.Type,
		"protocol": host.Protocol,
	}
	if host.Force {
		params["force"] = true
	}

	_, err := c.post("mkhost", params)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to create host: %w", decodedErr)
	}

	return nil
}