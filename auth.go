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
// Login performs the logon operation to the HMC REST API
func (c *HmcRestClient) Login(username, password string, debug bool) error {
	// Optional: If you still want to pass 'verbose' through the function signature,
	// you can toggle the logger level right here.
	if debug {
		c.EnableVerboseLogging()
	}

	payload := LogonRequest{
		SchemaVersion: "V1_0",
		XMLNS:         "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		XMLNSMC:       "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		UserID:        username,
		Password:      password,
	}
	
	xmlData, err := xml.Marshal(payload)
	if err != nil {
		c.Logger.Error("XML marshal failed", "error", err)
		return fmt.Errorf("XML marshal failed: %v", err)
	}

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	
	// LOOK HOW CLEAN THIS IS! No more "if verbose { ... }" blocks!
	// We pass the URL and Username as structured key-value pairs.
	c.Logger.Debug("Sending logon request", 
		"url", url, 
		"user", username,
	)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, string(xmlData))

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err, "url", url)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Structured logging makes tracking status codes super easy
	c.Logger.Debug("Logon response received", "status", resp.Status, "code", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response failed: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// For massive XML payloads, you can still use Debugf if you want formatted strings
	if debug {
		c.Logger.Debugf("Logon response body:\n%s", string(body))
	}

	var logonResp LogonResponse
	if err := xml.Unmarshal(body, &logonResp); err != nil {
		c.Logger.Error("XML unmarshal failed", "error", err)
		return fmt.Errorf("XML unmarshal failed: %v", err)
	}

	c.session = logonResp.Session
	c.Logger.Info("Successfully authenticated with HMC", "user", username)
	return nil
}

// Logoff performs the logoff operation from the HMC REST API
func (c *HmcRestClient) Logoff() error {
	if c.session == "" {
		c.Logger.Debug("No active session to log off")
		return nil // No session to log off
	}
	
	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	
	c.Logger.Debug("Sending logoff request", "url", url)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		c.Logger.Error("Request creation failed", "error", err)
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err == nil {
		c.logRawTraffic("RESPONSE", url, string(body))
	}

	c.Logger.Debug("Logoff response received", "status", resp.Status, "code", resp.StatusCode)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.Logger.Error("Logoff failed", "status", resp.Status)
		return fmt.Errorf("logoff failed with status: %s", resp.Status)
	}
	
	c.session = ""
	c.Logger.Info("Successfully logged off from HMC")
	
	return nil
}