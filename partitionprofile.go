package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// GetLogicalPartitionProfiles retrieves the logical partition profiles for a specific partition by UUID.
// Returns structured profile data using the same pattern as GetAllLogicalPartitionsInHmc.
func (c *HmcRestClient) GetLogicalPartitionProfiles(partitionUUID string, verbose bool) ([]LogicalPartitionProfile, error) {
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

    // Strip namespaces from XML
    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    // Find all LogicalPartitionProfile elements
    profileElements := doc.FindElements("//LogicalPartitionProfile")
    if verbose {
        hmcLogger.Printf("Found %d logical partition profiles for partition %s", len(profileElements), partitionUUID)
    }

    // Parse each profile element into structured data
    var profiles []LogicalPartitionProfile
    for _, profileElem := range profileElements {
        // Isolate the element XML
        profileDoc := etree.NewDocument()
        profileDoc.SetRoot(profileElem.Copy())
        profileBytes, err := profileDoc.WriteToBytes()
        if err != nil {
            if verbose {
                hmcLogger.Printf("Skipping profile due to XML serialization error: %v", err)
            }
            continue
        }

        // Unmarshal into the struct
        var profile LogicalPartitionProfile
        if err := xml.Unmarshal(profileBytes, &profile); err != nil {
            if verbose {
                hmcLogger.Printf("Skipping profile due to unmarshal error: %v", err)
            }
            continue
        }
        profiles = append(profiles, profile)
    }

    if verbose {
        hmcLogger.Printf("Successfully parsed %d profiles for partition %s", len(profiles), partitionUUID)
    }

    return profiles, nil
}

// GetLogicalPartitionProfile retrieves a single logical partition profile by its UUID.
func (c *HmcRestClient) GetLogicalPartitionProfile(partitionUUID string, profileUUID string, verbose bool) (*LogicalPartitionProfile, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/%s", c.hmcIP, partitionUUID, profileUUID)
	if verbose {
		hmcLogger.Printf("Fetching logical partition profile %s for partition UUID %s, URL: %s", profileUUID, partitionUUID, url)
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
		hmcLogger.Printf("GetLogicalPartitionProfile response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionProfile response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip namespaces from XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Find the LogicalPartitionProfile element
	profileElement := doc.FindElement("//LogicalPartitionProfile")
	if profileElement == nil {
		return nil, fmt.Errorf("LogicalPartitionProfile element not found in response")
	}

	// Isolate the element XML
	profileDoc := etree.NewDocument()
	profileDoc.SetRoot(profileElement.Copy())
	profileBytes, err := profileDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize profile XML: %v", err)
	}

	// Unmarshal into the struct
	var profile LogicalPartitionProfile
	if err := xml.Unmarshal(profileBytes, &profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Successfully retrieved profile %s (%s) for partition %s", profile.ProfileName, profileUUID, partitionUUID)
	}

	return &profile, nil
}
// DeleteLogicalPartitionProfile deletes a logical partition profile by its UUID.
// This permanently removes the profile from the partition.
// Note: You cannot delete the profile that is currently in use by a running partition.
func (c *HmcRestClient) DeleteLogicalPartitionProfile(partitionUUID string, profileUUID string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/%s", c.hmcIP, partitionUUID, profileUUID)
	if verbose {
		hmcLogger.Printf("Deleting logical partition profile %s for partition UUID %s, URL: %s", profileUUID, partitionUUID, url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("DeleteLogicalPartitionProfile response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose && len(body) > 0 {
		hmcLogger.Printf("DeleteLogicalPartitionProfile response body:\n%s", string(body))
	}

	// DELETE typically returns 204 No Content on success, but may also return 200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	if verbose {
		hmcLogger.Printf("Successfully deleted profile %s for partition %s", profileUUID, partitionUUID)
	}

	return nil
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

// GetPartitionProfiles retrieves all partition profiles (UUID and name) for a logical partition
func (c *HmcRestClient) GetPartitionProfiles(lparUUID string, verbose bool) ([]PartitionProfileQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/quick/All", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching partition profiles for partition UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=LogicalPartitionProfile")
	req.Header.Set("Accept", "application/json")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfiles response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfiles response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var profiles []PartitionProfileQuick
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Found %d partition profiles for partition UUID %s", len(profiles), lparUUID)
	}

	return profiles, nil
}

// SaveCurrentLparConfig saves the current active configuration of a Logical Partition to a profile.
// If force is true, it will overwrite an existing profile with the same name.
func (c *HmcRestClient) SaveCurrentLparConfig(lparUUID, profileName string, force, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/SaveCurrentConfig", c.hmcIP, lparUUID)
	
	if verbose {
		hmcLogger.Printf("Saving current config for LPAR %s to profile '%s' (Force: %t)...", lparUUID, profileName, force)
	}

	// 1. Define operation details for the JobRequest
	reqdOperation := map[string]string{
		"OperationName": "SaveCurrentConfig",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// 2. Build job parameters matching the HMC schema
	jobParams := map[string]string{
		"PartitionProfileName": profileName,
		"force":                fmt.Sprintf("%t", force),
	}

	// 3. Generate the XML payload using your existing helper
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return fmt.Errorf("failed to create job request payload: %v", err)
	}

	// 4. Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// 5. Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("SaveCurrentConfig Response Status: %s", resp.Status)
	}

	// 6. Check for non-success status codes (Usually 200, 201, or 202 for Jobs)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("SaveCurrentConfig job submission failed with status %s: %s", resp.Status, string(body))
	}

	// 7. Strip namespaces to find the JobID
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s. Waiting for completion...", jobID)
	}

	// 8. Wait for the background job to finish
	_, err = c.FetchJobStatus(jobID, false, 5, verbose)
	if err != nil {
		return fmt.Errorf("failed during SaveCurrentConfig job execution: %v", err)
	}

	return nil
}