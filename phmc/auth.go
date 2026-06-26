package hmc

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Login performs the logon operation to the HMC REST API
func (c *RestClient) Login(ctx context.Context, username, password string, debug bool) error {
	payload := LogonRequest{
		SchemaVersion: "V1_0",
		XMLNS:         "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		XMLNSMC:       "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		UserID:        username,
		Password:      password,
	}

	xmlData, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("XML marshal failed: %v", err)
	}

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response failed: %v", err)
	}

	var logonResp LogonResponse
	if err := xml.Unmarshal(body, &logonResp); err != nil {
		return fmt.Errorf("XML unmarshal failed: %v", err)
	}

	c.session = logonResp.Session
	return nil
}

// Logoff performs the logoff operation from the HMC REST API
func (c *RestClient) Logoff(ctx context.Context) error {
	if c.session == "" {
		return nil // No session to log off
	}

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)

	// Flush stale keep-alive connections — long-running scripts cause the HMC to
	// silently drop the TCP connection, so force a fresh one for Logoff.
	c.client.CloseIdleConnections()

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("X-API-Session", c.session)

	// Tell Go not to reuse this connection
	req.Close = true

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := c.client.Do(req)
	if err != nil {
		// If the HMC aggressively killed the session while we were waiting, ignore it.
		if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection reset by peer") {
			c.session = "" // Clear local session state
			return nil
		}
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if _, err := io.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("reading response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("logoff failed with status: %s", resp.Status)
	}

	c.session = ""
	return nil
}
