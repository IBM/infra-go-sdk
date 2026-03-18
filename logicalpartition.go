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

/// CreatePhysicalVolumeMap maps a physical disk on the VIOS to a target LPAR using the GET-Modify-POST pattern.
func (c *HmcRestClient) CreatePhysicalVolumeMap(sysUUID, viosUUID, lparUUID, diskName string, verbose bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching VIOS %s to inject new storage mapping...", viosUUID)
	}

	doc, err := c.fetchAndParseHMCXML(url, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VIOS mappings: %v", err)
	}

	// 2. EXTRACT the actual VirtualIOServer element (ignoring the <entry> Atom wrapper)
	viosElem := doc.FindElement("//VirtualIOServer")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in VIOS XML")
	}

	// 3. Locate or create the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//VirtualSCSIMappings")
	if mappingsList == nil {
		if verbose { hmcLogger.Printf("No existing mappings found. Creating VirtualSCSIMappings group...") }
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// 4. Construct the new mapping (Schema-compliant)
	newMappingXML := fmt.Sprintf(`
        <VirtualSCSIMapping schemaVersion="V1_0">
            <AssociatedLogicalPartition href="https://%s:443/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
            <Storage>
                <PhysicalVolume schemaVersion="V1_0">
                    <VolumeName>%s</VolumeName>
                </PhysicalVolume>
            </Storage>
        </VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, diskName)

	newMappingDoc := etree.NewDocument()
	if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
		return "", fmt.Errorf("failed to parse new mapping XML: %v", err)
	}

	// 5. Append the new mapping safely to the list
	mappingsList.AddChild(newMappingDoc.Root())

	// 6. Natively set the correct namespace and tag on the root element using etree
	viosElem.Tag = "VirtualIOServer:VirtualIOServer"
	viosElem.CreateAttr("xmlns:VirtualIOServer", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns:ns2", "http://www.w3.org/XML/1998/namespace/k2")
	
	// Ensure schemaVersion exists
	if viosElem.SelectAttrValue("schemaVersion", "") == "" {
		viosElem.CreateAttr("schemaVersion", "V1_0")
	}

	// 7. Serialize ONLY the VIOS element back to a string
	payloadDoc := etree.NewDocument()
	payloadDoc.SetRoot(viosElem.Copy())
	xmlStr, err := payloadDoc.WriteToString()
	if err != nil {
		return "", err
	}

	if verbose {
		hmcLogger.Printf("POSTing updated VIOS XML back to HMC...")
	}

	// 8. POST the complete update back to the VIOS API
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	req, err := http.NewRequest("POST", postURL, strings.NewReader(xmlStr))
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	req.Header.Set("Accept", "application/atom+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose { hmcLogger.Printf("Update job triggered: %s", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}