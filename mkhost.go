package svc

import (
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
	return err
}
