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

// PowerOnPartition powers on a logical partition and returns the job response
func (c *HmcRestClient) PowerOnPartition(systemUUID, lparUUID, profileUUID, keylock, iIPLsource, osType string, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/do/PowerOn", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering on partition UUID %s on system UUID %s, URL: %s", lparUUID, systemUUID, url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "PowerOn",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters
	jobParams := map[string]string{
		"force":    "false",
		"novsi":    "true",
		"bootmode": "norm",
	}

	if profileUUID != "" {
		jobParams["LogicalPartitionProfile"] = profileUUID
	}

	if keylock != "" {
		if keylock == "normal" {
			keylock = "norm"
		}
		jobParams["keylock"] = keylock
	}

	if osType == "OS400" && iIPLsource != "" {
		jobParams["iIPLsource"] = iIPLsource
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("PowerOn job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

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
		hmcLogger.Printf("PowerOnPartition response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("PowerOnPartition response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response (assuming XML response despite Accept: application/json)
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract job ID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Monitor job status
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// PowerOffPartition powers off a logical partition and returns the job response
func (c *HmcRestClient) PowerOffPartition(systemUUID, lparUUID, shutdownOption string, restart bool, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/do/PowerOff", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering off partition UUID %s on system UUID %s, URL: %s", lparUUID, systemUUID, url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "PowerOff",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Determine immediate and operation based on shutdownOption
	var immediate, operation string
	switch shutdownOption {
	case "Delayed":
		immediate = "false"
		operation = "shutdown"
	case "Immediate":
		immediate = "true"
		operation = "shutdown"
	case "OperatingSystem":
		immediate = "false"
		operation = "osshutdown"
	case "OSImmediate":
		immediate = "true"
		operation = "osshutdown"
	case "Dump":
		immediate = "false"
		operation = "dumprestart"
		restart = false // Override restart as per Python logic
	case "DumpRetry":
		immediate = "false"
		operation = "retrydump"
		restart = false // Override restart as per Python logic
	default:
		return nil, fmt.Errorf("invalid shutdownOption: %s, must be one of Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry", shutdownOption)
	}

	// Build job parameters
	jobParams := map[string]string{
		"immediate": immediate,
		"operation": operation,
		"restart":   fmt.Sprintf("%t", restart),
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("PowerOff job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

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
		hmcLogger.Printf("PowerOffPartition response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("PowerOffPartition response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract job ID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Monitor job status
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// GetLogicalPartitions retrieves the advanced list of logical partitions for a managed system as a slice of XML elements.
func (c *HmcRestClient) GetLogicalPartitions(systemUUID string, verbose bool) ([]*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition?group=Advanced", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching advanced logical partitions for system UUID %s, URL: %s", systemUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	// Set a slightly longer timeout for Advanced XML as it can be heavy
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartitions response status: %s", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse XML response and strip namespaces to make querying easier
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// FIX: Use FindElements (plural) to capture all partitions in the Atom feed
	logicalPartitions := doc.FindElements("//LogicalPartition")
	if len(logicalPartitions) == 0 {
		if verbose {
			hmcLogger.Printf("No LogicalPartition elements found in the response feed.")
		}
		return []*etree.Element{}, nil // Return empty slice instead of error if none exist
	}

	if verbose {
		hmcLogger.Printf("Successfully parsed %d partitions from Advanced XML.", len(logicalPartitions))
	}

	return logicalPartitions, nil
}

// GetLogicalPartitionQuick retrieves quick details of a specific logical partition by UUID
func (c *HmcRestClient) GetLogicalPartitionQuick(partitionUUID string, verbose bool) (*LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, partitionUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick logical partition details for UUID %s, URL: %s", partitionUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

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
		hmcLogger.Printf("GetLogicalPartitionQuick response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetLogicalPartitionQuick response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Logical Partition failed. Response code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// Parse JSON response
	var partition LogicalPartitionQuick
	if err := json.Unmarshal(body, &partition); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Manually set the UUID since it's not in the JSON response
	partition.UUID = partitionUUID

	if verbose {
		hmcLogger.Printf("Found logical partition: Name=%s, UUID=%s", partition.PartitionName, partition.UUID)
	}

	return &partition, nil
}

// GetPartitionQuick retrieves quick properties of a partition as a map
func (c *HmcRestClient) GetPartitionQuick(lparUUID string, verbose bool) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick partition properties for UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
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
		hmcLogger.Printf("GetPartitionQuick response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionQuick response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var partitionProps map[string]interface{}
	if err := json.Unmarshal(body, &partitionProps); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return partitionProps, nil
}

// GetLogicalPartitionsQuickAll retrieves the quick list of logical partitions for a system
func (c *HmcRestClient) GetLogicalPartitionsQuickAll(systemUUID string, verbose bool) ([]LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/quick/All", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick logical partitions for system UUID %s, URL: %s", systemUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuickAll response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuickAll response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil // No partitions found
		}
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	var lparList []LogicalPartitionQuick
	if err := json.Unmarshal(body, &lparList); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return lparList, nil
}

// GetClientNetworkAdapter retrieves the ClientNetworkAdapter details for a partition
func (c *HmcRestClient) GetClientNetworkAdapter(systemUUID, lparUUID string, verbose bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching ClientNetworkAdapter for system UUID %s, partition UUID %s, URL: %s", systemUUID, lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml;type=ClientNetworkAdapter")
	req.Header.Set("Accept", "*/*")

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
		hmcLogger.Printf("GetClientNetworkAdapter response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetClientNetworkAdapter response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	clientNetworkAdapter := doc.FindElement("//ClientNetworkAdapter")
	if clientNetworkAdapter == nil {
		return nil, fmt.Errorf("ClientNetworkAdapter element not found in response")
	}

	return clientNetworkAdapter, nil
}

// DeleteLogicalPartition deletes a logical partition by its UUID.
func (c *HmcRestClient) DeleteLogicalPartition(partitionUUID string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, partitionUUID)
	if verbose {
		hmcLogger.Printf("Deleting logical partition UUID %s, URL: %s", partitionUUID, url)
	}

	// Create and configure the DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")

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
		hmcLogger.Printf("DeleteLogicalPartition response status: %s", resp.Status)
	}

	// Read the response body (if any)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose && len(body) > 0 {
		hmcLogger.Printf("DeleteLogicalPartition response body:\n%s", string(body))
	}

	// Check for success (204 No Content)
	if resp.StatusCode != http.StatusNoContent {
		// Attempt to parse error from body
		doc, err := xmlStripNamespace(body)
		if err == nil {
			errorMsgs := doc.FindElements("//Message")
			if len(errorMsgs) > 0 {
				return fmt.Errorf("delete failed: %s, status: %s", errorMsgs[0].Text(), resp.Status)
			}
		}
		return fmt.Errorf("delete failed with status %s: %s", resp.Status, string(body))
	}

	if verbose {
		hmcLogger.Printf("Logical partition %s deleted successfully", partitionUUID)
	}

	return nil
}

// GetLogicalPartitionsAdv retrieves the advanced list of logical partitions for a managed system as a slice of XML elements.
func (c *HmcRestClient) GetLogicalPartitionsAdv(systemUUID string, verbose bool) ([]*etree.Element, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition?group=Advanced", c.hmcIP, systemUUID)
    if verbose {
        hmcLogger.Printf("Fetching advanced logical partitions for system UUID %s, URL: %s", systemUUID, url)
    }

    // Create and configure the GET request
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

    // Set a slightly longer timeout for Advanced XML as payloads can be heavy
    ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel()
    req = req.WithContext(ctx)

    // Send the request
    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %v", err)
    }
    defer resp.Body.Close()

    if verbose {
        hmcLogger.Printf("GetLogicalPartitionsAdv response status: %s", resp.Status)
    }

    // Check for non-200 status codes
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
    }

    // Read the response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    // Parse XML response and strip namespaces to make querying easier
    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    // Use FindElements (plural) to capture ALL partitions in the Atom feed
    logicalPartitions := doc.FindElements("//LogicalPartition")
    if len(logicalPartitions) == 0 {
        if verbose {
            hmcLogger.Printf("No LogicalPartition elements found in the response feed.")
        }
        return []*etree.Element{}, nil // Return empty slice instead of error if none exist
    }

    if verbose {
        hmcLogger.Printf("Successfully parsed %d partitions from Advanced XML.", len(logicalPartitions))
    }

    return logicalPartitions, nil
}