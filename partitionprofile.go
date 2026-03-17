package hmc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// GetLogicalPartitionProfiles retrieves the logical partition profiles for a specific partition by UUID.
func (c *HmcRestClient) GetLogicalPartitionProfiles(partitionUUID string, verbose bool) ([]*etree.Element, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile", c.hmcIP, partitionUUID)
    if verbose {
        hmcLogger.Printf("Fetching logical partition profiles for UUID %s, URL: %s", partitionUUID, url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/atom+xml")

    ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel()
    req = req.WithContext(ctx)

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %v", err)
    }
    defer resp.Body.Close()

    if verbose {
        hmcLogger.Printf("GetLogicalPartitionProfiles response status: %s", resp.Status)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    if verbose {
        hmcLogger.Printf("GetLogicalPartitionProfiles response body:\n%s", string(body))
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    profiles := doc.FindElements("//LogicalPartitionProfile")
    if verbose {
        hmcLogger.Printf("Found %d logical partition profiles for partition %s", len(profiles), partitionUUID)
    }

    return profiles, nil
}
// UpdateLogicalPartitionProfile updates a logical partition profile by its UUID with the provided XML payload.
func (c *HmcRestClient) UpdateLogicalPartitionProfile(partitionUUID string, profileName string, updatedProfileXML string, verbose bool) error {
    // STEP 1: GET the full LogicalPartition entry
    url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile", c.hmcIP, partitionUUID)

    if verbose {
        hmcLogger.Printf("Fetching LogicalPartition entry from URL: %s", url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return fmt.Errorf("GET request creation failed: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=LogicalPartitionProfile")
	req.Header.Set("Accept", "*/*")

    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("GET request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("GET failed: status %d body: %s", resp.StatusCode, string(body))
    }

    originalXML, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("failed reading GET response: %v", err)
    }

    // STEP 2: Replace the target LogicalPartitionProfile in the XML
    replacedXML := strings.Replace(
        string(originalXML),
        fmt.Sprintf(`<LogicalPartitionProfile href="LogicalPartitionProfile/%s">`, profileName),
        updatedProfileXML,
        1,
    )

    if verbose {
        hmcLogger.Printf("Replaced profile XML for %s", profileName)
    }

    // STEP 3: PUT the full updated LogicalPartition entry back
    postReq, err := http.NewRequest("PUT", url, strings.NewReader(replacedXML))
    if err != nil {
        return fmt.Errorf("failed to create PUT request: %v", err)
    }

    postReq.Header.Set("X-API-Session", c.session)
    postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartitionProfile")
    postReq.Header.Set("Accept", "*/*")

    postResp, err := c.client.Do(postReq)
    if err != nil {
        return fmt.Errorf("POST failed: %v", err)
    }
    defer postResp.Body.Close()

    if postResp.StatusCode != 200 && postResp.StatusCode != 201 && postResp.StatusCode != 204 {
        body, _ := io.ReadAll(postResp.Body)
        return fmt.Errorf("POST failed with status %d: %s", postResp.StatusCode, string(body))
    }

    if verbose {
        hmcLogger.Printf("Successfully updated profile %s for partition %s", profileName, partitionUUID)
    }
    return nil
}

// GetPartitionProfile retrieves the UUID of a partition profile for a logical partition
func (c *HmcRestClient) GetPartitionProfile(lparUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/quick/All", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching partition profile for partition UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=LogicalPartitionProfile")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfile response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfile response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var profiles []PartitionProfileQuick
	if err := json.Unmarshal(body, &profiles); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Check if any profiles were found
	if len(profiles) == 0 {
		return "", fmt.Errorf("no partition profiles found for partition UUID %s", lparUUID)
	}

	// Return the UUID of the first profile
	profileUUID := profiles[0].UUID
	if profileUUID == "" {
		return "", fmt.Errorf("profile UUID not found in response for partition UUID %s", lparUUID)
	}

	if verbose {
		hmcLogger.Printf("Found partition profile %s with UUID %s", profiles[0].ProfileName, profileUUID)
	}

	return profileUUID, nil
}