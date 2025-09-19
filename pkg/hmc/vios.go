package hmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConfigDevice submits a job request to configure a device on a Virtual I/O Server.
// If devName is empty, it attempts to configure all devices.
func (c *HmcRestClient) ConfigDevice(viosID string, devName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/ConfigDevice", c.hmcIP, viosID)
	if verbose {
		hmcLogger.Printf("Submitting ConfigDevice job for VIOS ID %s, URL: %s", viosID, url)
	}

	// Prepare operation map
	operation := map[string]string{
		"OperationName": "ConfigDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	// Prepare params map
	params := make(map[string]string)
	if devName != "" {
		params["devName"] = devName
	}

	// Schema version
	schemaVersion := "V1_1_0"

	// Include job param schema
	includeJobParamSchema := true

	// Generate payload using createJobRequestPayload
	xmlString, err := createJobRequestPayload(operation, params, schemaVersion, verbose, includeJobParamSchema)
	if err != nil {
		return fmt.Errorf("failed to create JobRequest payload: %v", err)
	}

	if verbose {
		hmcLogger.Printf("JobRequest XML:\n%s", xmlString)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(xmlString))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=JobResponse")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("ConfigDevice response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("ConfigDevice response body:\n%s", string(body))
	}

	// Check for non-success status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("ConfigDevice job submission failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}
