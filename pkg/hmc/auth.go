package hmc

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Login performs the logon operation to the HMC REST API
func (c *HmcRestClient) Login(username, password string, verbose bool) error {
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

	if verbose {
		hmcLogger.Printf("Sending logon request to https://%s/rest/api/web/Logon", c.hmcIP)
		hmcLogger.Printf("Logon request payload:\n%s", string(xmlData))
	}

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Logon response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Logon response body:\n%s", string(body))
	}

	var logonResp LogonResponse
	if err := xml.Unmarshal(body, &logonResp); err != nil {
		return fmt.Errorf("XML unmarshal failed: %v", err)
	}

	c.session = logonResp.Session
	return nil
}

// Logoff performs the logoff operation from the HMC REST API
func (c *HmcRestClient) Logoff() error {
	if c.session == "" {
		return nil // No session to log off
	}
	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("logoff failed with status: %s", resp.Status)
	}
	c.session = ""
	return nil
}
