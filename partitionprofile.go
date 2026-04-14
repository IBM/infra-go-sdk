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
func (c *HmcRestClient) GetLogicalPartitionProfiles(partitionUUID string, debug bool) ([]LogicalPartitionProfile, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile", c.hmcIP, partitionUUID)
	if debug {
		c.Logger.Debug("Fetching logical partition profiles", "partitionUUID", partitionUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetLogicalPartitionProfiles response status", "status", resp.Status)
		c.Logger.Debug("GetLogicalPartitionProfiles response body", "body", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip namespaces from XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Find all LogicalPartitionProfile elements
	profileElements := doc.FindElements("//LogicalPartitionProfile")
	if debug {
		c.Logger.Debug("Found logical partition profiles", "count", len(profileElements), "partitionUUID", partitionUUID)
	}

	// Parse each profile element into structured data
	var profiles []LogicalPartitionProfile
	for _, profileElem := range profileElements {
		// Isolate the element XML
		profileDoc := etree.NewDocument()
		profileDoc.SetRoot(profileElem.Copy())
		profileBytes, err := profileDoc.WriteToBytes()
		if err != nil {
			if debug {
				c.Logger.Warn("Skipping profile due to XML serialization error", "error", err)
			}
			continue
		}

		// Unmarshal into the struct
		var profile LogicalPartitionProfile
		if err := xml.Unmarshal(profileBytes, &profile); err != nil {
			if debug {
				c.Logger.Warn("Skipping profile due to unmarshal error", "error", err)
			}
			continue
		}
		profiles = append(profiles, profile)
	}

	if debug {
		c.Logger.Info("Successfully parsed profiles", "count", len(profiles), "partitionUUID", partitionUUID)
	}

	return profiles, nil
}

// GetLogicalPartitionProfile retrieves a single logical partition profile by its UUID.
func (c *HmcRestClient) GetLogicalPartitionProfile(partitionUUID string, profileUUID string, debug bool) (*LogicalPartitionProfile, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/%s", c.hmcIP, partitionUUID, profileUUID)
	if debug {
		c.Logger.Debug("Fetching logical partition profile", "profileUUID", profileUUID, "partitionUUID", partitionUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetLogicalPartitionProfile response status", "status", resp.Status)
		c.Logger.Debug("GetLogicalPartitionProfile response body", "body", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(body))
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

	if debug {
		c.Logger.Info("Successfully retrieved profile", "profileName", profile.ProfileName, "profileUUID", profileUUID, "partitionUUID", partitionUUID)
	}

	return &profile, nil
}

// DeleteLogicalPartitionProfile deletes a logical partition profile by its UUID.
// This permanently removes the profile from the partition.
// Note: You cannot delete the profile that is currently in use by a running partition.
func (c *HmcRestClient) DeleteLogicalPartitionProfile(partitionUUID string, profileUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/%s", c.hmcIP, partitionUUID, profileUUID)
	if debug {
		c.Logger.Debug("Deleting logical partition profile", "profileUUID", profileUUID, "partitionUUID", partitionUUID, "url", url)
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

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("DeleteLogicalPartitionProfile response status", "status", resp.Status)
		if len(body) > 0 {
			c.Logger.Debug("DeleteLogicalPartitionProfile response body", "body", string(body))
		}
	}

	// DELETE typically returns 204 No Content on success, but may also return 200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(body))
		return fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	if debug {
		c.Logger.Info("Successfully deleted profile", "profileUUID", profileUUID, "partitionUUID", partitionUUID)
	}

	return nil
}


// UpdateLogicalPartitionProfile updates a logical partition profile by its UUID with the provided XML payload.
func (c *HmcRestClient) UpdateLogicalPartitionProfile(partitionUUID string, profileName string, updatedProfileXML string, debug bool) error {
	// STEP 1: GET the full LogicalPartition entry
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile", c.hmcIP, partitionUUID)

	if debug {
		c.Logger.Debug("Fetching LogicalPartition entry", "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("GET request creation failed: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=LogicalPartitionProfile")
	req.Header.Set("Accept", "*/*")

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("GET request failed", "error", err)
		return fmt.Errorf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	originalXML, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed reading GET response: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(originalXML))

	if resp.StatusCode != 200 {
		c.Logger.Error("GET failed", "status", resp.StatusCode, "body", string(originalXML))
		return fmt.Errorf("GET failed: status %d body: %s", resp.StatusCode, string(originalXML))
	}

	// STEP 2: Replace the target LogicalPartitionProfile in the XML
	replacedXML := strings.Replace(
		string(originalXML),
		fmt.Sprintf(`<LogicalPartitionProfile href="LogicalPartitionProfile/%s">`, profileName),
		updatedProfileXML,
		1,
	)

	if debug {
		c.Logger.Debug("Replaced profile XML", "profileName", profileName)
	}

	// STEP 3: PUT the full updated LogicalPartition entry back
	postReq, err := http.NewRequest("PUT", url, strings.NewReader(replacedXML))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartitionProfile")
	postReq.Header.Set("Accept", "*/*")

	c.logRawTraffic("REQUEST (PUT)", url, replacedXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		c.Logger.Error("POST failed", "error", err)
		return fmt.Errorf("POST failed: %v", err)
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(postBody))

	if postResp.StatusCode != 200 && postResp.StatusCode != 201 && postResp.StatusCode != 204 {
		c.Logger.Error("POST failed", "status", postResp.StatusCode, "body", string(postBody))
		return fmt.Errorf("POST failed with status %d: %s", postResp.StatusCode, string(postBody))
	}

	if debug {
		c.Logger.Info("Successfully updated profile", "profileName", profileName, "partitionUUID", partitionUUID)
	}
	return nil
}

// GetPartitionProfiles retrieves all partition profiles (UUID and name) for a logical partition
func (c *HmcRestClient) GetPartitionProfiles(lparUUID string, debug bool) ([]PartitionProfileQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/quick/All", c.hmcIP, lparUUID)
	if debug {
		c.Logger.Debug("Fetching partition profiles", "lparUUID", lparUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetPartitionProfiles response status", "status", resp.Status)
		c.Logger.Debug("GetPartitionProfiles response body", "body", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var profiles []PartitionProfileQuick
	if err := json.Unmarshal(body, &profiles); err != nil {
		c.Logger.Error("Failed to parse JSON response", "error", err)
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if debug {
		c.Logger.Info("Found partition profiles", "count", len(profiles), "lparUUID", lparUUID)
	}

	return profiles, nil
}

// SaveCurrentLparConfig saves the current active configuration of a Logical Partition to a profile.
// If force is true, it will overwrite an existing profile with the same name.
func (c *HmcRestClient) SaveCurrentLparConfig(lparUUID, profileName string, force, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/SaveCurrentConfig", c.hmcIP, lparUUID)
	
	if debug {
		c.Logger.Debug("Saving current config for LPAR", "lparUUID", lparUUID, "profileName", profileName, "force", force)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// 5. Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("SaveCurrentConfig Response Status", "status", resp.Status)
	}

	// 6. Check for non-success status codes (Usually 200, 201, or 202 for Jobs)
	if resp.StatusCode >= 400 {
		c.Logger.Error("SaveCurrentConfig job submission failed", "status", resp.Status, "body", string(body))
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
	
	if debug {
		c.Logger.Info("Extracted JobID. Waiting for completion...", "jobID", jobID)
	}

	// 8. Wait for the background job to finish
	_, err = c.FetchJobStatus(jobID, false, 5, debug)
	if err != nil {
		return fmt.Errorf("failed during SaveCurrentConfig job execution: %v", err)
	}

	return nil
}