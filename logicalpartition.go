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

// PowerOnPartition powers on a logical partition directly by its UUID and returns the job response.
func (c *HmcRestClient) PowerOnPartition(lparUUID, profileUUID, keylock, iIPLsource, osType string, verbose bool) (*etree.Document, error) {
	// Updated URL to target the LogicalPartition UUID directly
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOn", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering on partition UUID %s, URL: %s", lparUUID, url)
	}

	// Define operation details for the JobRequest 
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
			keylock = "norm" // Normalize keylock string 
		}
		jobParams["keylock"] = keylock
	}

	if osType == "OS400" && iIPLsource != "" {
		jobParams["iIPLsource"] = iIPLsource
	}

	// Create XML payload using existing helper [cite: 16, 207]
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}

	// Configure and execute the PUT request [cite: 208]
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response and extract JobID [cite: 34, 209]
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	// Monitor job status [cite: 763, 209]
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// PowerOffPartition powers off a logical partition directly by its UUID and returns the job response.
func (c *HmcRestClient) PowerOffPartition(lparUUID, shutdownOption string, restart bool, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOff", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering off partition UUID %s, URL: %s", lparUUID, url)
	}

	// Define operation details for the JobRequest 
	reqdOperation := map[string]string{
		"OperationName": "PowerOff",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Determine immediate flag and operation string based on shutdownOption [cite: 210, 211]
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
		restart = false 
	case "DumpRetry":
		immediate = "false"
		operation = "retrydump"
		restart = false 
	default:
		return nil, fmt.Errorf("invalid shutdownOption: %s, must be one of Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry", shutdownOption)
	}

	// Build job parameters for the XML payload [cite: 211]
	jobParams := map[string]string{
		"immediate": immediate,
		"operation": operation,
		"restart":   fmt.Sprintf("%t", restart),
	}

	// Create XML payload using the existing job request helper [cite: 211]
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}

	// Configure and execute the PUT request [cite: 211, 212]
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Extract the JobID from the XML response [cite: 212, 213]
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	// Monitor the background job for completion [cite: 213]
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
func (c *HmcRestClient) GetLogicalPartitionQuickAll(systemUUID string, verbose bool) ([]LogicalPartitionQuick, error) {
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




// CreateLogicalPartition creates a new LPAR using the direct UOM PUT method.
func (c *HmcRestClient) CreateLogicalPartition(sysUUID string, req CreateLparRequest, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition", c.hmcIP, sysUUID)

	// The HMC demands STRICT ALPHABETICAL ORDERING for elements.
	// We have rearranged the Processing Units and Virtual Processors 
	// to perfectly match this alphabetical requirement (D -> M -> M).
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<uom:LogicalPartition xmlns:uom="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                      schemaVersion="V1_0">
    <uom:Metadata><uom:Atom/></uom:Metadata>
    <uom:PartitionMemoryConfiguration schemaVersion="V1_0">
        <uom:Metadata><uom:Atom/></uom:Metadata>
        <uom:DesiredMemory>%d</uom:DesiredMemory>
        <uom:MaximumMemory>%d</uom:MaximumMemory>
        <uom:MinimumMemory>%d</uom:MinimumMemory>
    </uom:PartitionMemoryConfiguration>
    <uom:PartitionName>%s</uom:PartitionName>
    <uom:PartitionProcessorConfiguration schemaVersion="V1_0">
        <uom:Metadata><uom:Atom/></uom:Metadata>
        <uom:HasDedicatedProcessors>false</uom:HasDedicatedProcessors>
        <uom:SharedProcessorConfiguration schemaVersion="V1_0">
            <uom:Metadata><uom:Atom/></uom:Metadata>
            <uom:DesiredProcessingUnits>%.1f</uom:DesiredProcessingUnits>
            <uom:DesiredVirtualProcessors>%d</uom:DesiredVirtualProcessors>
            <uom:MaximumProcessingUnits>%.1f</uom:MaximumProcessingUnits>
            <uom:MaximumVirtualProcessors>%d</uom:MaximumVirtualProcessors>
            <uom:MinimumProcessingUnits>%.1f</uom:MinimumProcessingUnits>
            <uom:MinimumVirtualProcessors>%d</uom:MinimumVirtualProcessors>
            <uom:UncappedWeight>128</uom:UncappedWeight>
        </uom:SharedProcessorConfiguration>
        <uom:SharingMode>%s</uom:SharingMode>
    </uom:PartitionProcessorConfiguration>
    <uom:PartitionType>AIX/Linux</uom:PartitionType>
</uom:LogicalPartition>`,
		req.DesiredMem, req.MaxMem, req.MinMem, 
		req.Name, 
		req.DesiredProcUnits, req.DesiredVcpus, // Desired (D)
		req.MaxProcUnits, req.MaxVcpus,         // Maximum (M)
		req.MinProcUnits, req.MinVcpus,         // Minimum (Mi)
		req.SharingMode)

	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("UOM creation failed (%s): %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("LPAR created, but failed to extract UUID from response")
	}

	newUUID := atomID.Text()
	if verbose {
		hmcLogger.Printf("🚀 LPAR Created! UUID: %s", newUUID)
	}

	return newUUID, nil
}

// CreateVirtualSCSIClientAdapter adds a vSCSI Client Adapter and strictly maps it to a VIOS.
func (c *HmcRestClient) CreateVirtualSCSIClientAdapter(lparUUID string, viosID, viosSlot int, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)

	if verbose {
		hmcLogger.Printf("Adding vSCSI Client Adapter mapping to VIOS ID: %d, VIOS Slot: %d...", viosID, viosSlot)
	}

	// We provide the Base Adapter properties (AdapterType, RequiredAdapter)
	// followed exactly by the vSCSI explicit mapping (Remote... properties)
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VirtualSCSIClientAdapter:VirtualSCSIClientAdapter xmlns:VirtualSCSIClientAdapter="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                                   xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                                   schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <AdapterType>Client</AdapterType>
    <RequiredAdapter>false</RequiredAdapter>
    <RemoteLogicalPartitionID>%d</RemoteLogicalPartitionID>
    <RemoteSlotNumber>%d</RemoteSlotNumber>
</VirtualSCSIClientAdapter:VirtualSCSIClientAdapter>`, viosID, viosSlot)

	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualSCSIClientAdapter")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vSCSI Adapter creation failed (%s): %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("Adapter created successfully, but failed to extract new UUID")
	}

	return atomID.Text(), nil
}

// GetLogicalPartitionDetailed fetches the exhaustive XML details of a specific logical partition by its UUID.
func (c *HmcRestClient) GetLogicalPartitionDetailed(lparUUID string, verbose bool) (*LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching exhaustive logical partition details for UUID %s, URL: %s", lparUUID, url)
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
		hmcLogger.Printf("GetLogicalPartitionDetailed response status: %s", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
    }

	// 1. Strip the namespaces using the existing helper to make unmarshaling clean
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// 2. Extract ONLY the core LogicalPartition element (bypassing the <entry> atom wrapper)
	lparElem := doc.FindElement("//LogicalPartition")
	if lparElem == nil {
		return nil, fmt.Errorf("LogicalPartition root element not found in XML response")
	}

	// 3. Serialize the isolated element back to bytes
	lparDoc := etree.NewDocument()
	lparDoc.SetRoot(lparElem.Copy())
	lparBytes, err := lparDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize isolated LogicalPartition element: %v", err)
	}

	// 4. Unmarshal directly into our comprehensive Go struct
	var detailedLpar LogicalPartitionDetailed
	if err := xml.Unmarshal(lparBytes, &detailedLpar); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML into LogicalPartitionDetailed struct: %v", err)
	}

	if verbose {
		hmcLogger.Printf("✅ Successfully parsed exhaustive details for LPAR: %s", detailedLpar.PartitionName)
	}

	return &detailedLpar, nil
}

// MapPhysicalIOAdapters dynamically assigns multiple Physical I/O Adapters to an LPAR.
// It accepts an optional inventory (*ManagedSystemDetailed) to perform a local safety check 
// for ownership conflicts before talking to the HMC.
func (c *HmcRestClient) MapPhysicalIOAdapters(sysUUID, lparUUID string, adapterIDs []string, inventory *ManagedSystemDetailed, verbose bool) (string, error) {
	
	// 1. PRE-FLIGHT SAFETY CHECK (Only if inventory is provided)
	if inventory != nil {
		if verbose { hmcLogger.Printf("Performing pre-flight ownership check for %d adapters...", len(adapterIDs)) }
		for _, adapterID := range adapterIDs {
			for _, bus := range inventory.IOConfig.IOBuses {
				for _, slot := range bus.IOSlots {
					if slot.RelatedIOAdapter.AdapterID == adapterID {
						// PartitionID > 0 means the slot is assigned to someone 
						// We check if that someone is NOT our target LPAR
						if slot.PartitionID > 0 && slot.PartitionUUID != lparUUID {
							return "", fmt.Errorf("ABORT: Adapter %s is currently owned by LPAR '%s' (ID: %d)", 
								adapterID, slot.PartitionName, slot.PartitionID)
						}
					}
				}
			}
		}
	}

	// 2. FETCH PRISTINE LPAR XML
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil { return "", err }
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil { return "", err }
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		return "", fmt.Errorf("GET failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil { return "", err }
	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil { return "", fmt.Errorf("LogicalPartition element not found") }

	// 3. TARGET THE IO CONFIGURATION
	ioConfig := lparElem.FindElement(".//*[local-name()='PartitionIOConfiguration']")
	if ioConfig == nil { return "", fmt.Errorf("PartitionIOConfiguration not found") }

	profileSlots := ioConfig.FindElement(".//*[local-name()='ProfileIOSlots']")
	if profileSlots == nil {
		profileSlots = ioConfig.CreateElement("ProfileIOSlots")
		profileSlots.CreateAttr("schemaVersion", "V1_0")
	}

	// 4. INJECT ALL ADAPTERS
	var newlyAdded int
	for _, adapterID := range adapterIDs {
		// Duplicate Check (Don't map what's already in this specific LPAR's profile)
		duplicate := false
		for _, slot := range profileSlots.FindElements(".//*[local-name()='AssociatedIOSlot']/*[local-name()='SlotDynamicReconfigurationConnectorIndex']") {
			if slot.Text() == adapterID {
				duplicate = true
				break
			}
		}
		if duplicate { continue }

		newSlotXML := fmt.Sprintf(`
			<ProfileIOSlot schemaVersion="V1_0">
				<AssociatedIOSlot schemaVersion="V1_0">
					<SlotDynamicReconfigurationConnectorIndex>%s</SlotDynamicReconfigurationConnectorIndex>
				</AssociatedIOSlot>
				<IsRequired>false</IsRequired>
			</ProfileIOSlot>`, adapterID)
		
		slotDoc := etree.NewDocument()
		slotDoc.ReadFromString(newSlotXML)
		profileSlots.AddChild(slotDoc.Root())
		newlyAdded++
	}

	if newlyAdded == 0 { return "ALREADY_MAPPED", nil }

	// 5. POST UPDATED XML
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, _ := http.NewRequest("POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil { return "", err }
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	if postResp.StatusCode >= 400 {
		return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	// 6. MONITOR DLPAR JOB
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose { hmcLogger.Printf("I/O Batch job triggered: %s", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}