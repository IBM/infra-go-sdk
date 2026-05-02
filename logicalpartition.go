package hmc

import (
	"bytes"
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

// PowerOnPartition powers on a logical partition using options struct.
func (c *HmcRestClient) PowerOnPartition(ctx context.Context, lparUUID string, options *PowerOnOptions, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOn", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Powering on partition", "partitionUUID", lparUUID, "bootMode", options.BootMode, "url", url)
	}

	reqdOperation := map[string]string{
		"OperationName": "PowerOn",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	jobParams := make(map[string]string)
	schemaVersion := "V1_0"

	// Apply defaults
	bootMode := options.BootMode
	if bootMode == "" {
		bootMode = "norm"
	}
	
	keylock := options.Keylock
	if keylock == "" {
		keylock = "normal"
	}

	// Apply Netboot Logic & IP Parameters
	if bootMode == "netboot" {
		jobParams["OperationType"] = "netboot"
		schemaVersion = "V1_2_0"

		// The Hypervisor strictly requires these for netboot:
		jobParams["LogicalPartitionProfileUUID"] = options.ProfileUUID
		
		// Use provided values or defaults
		if options.ConnectionSpeed != "" {
			jobParams["ConnectionSpeed"] = options.ConnectionSpeed
		} else {
			jobParams["ConnectionSpeed"] = "auto"
		}
		
		if options.DuplexMode != "" {
			jobParams["DuplexMode"] = options.DuplexMode
		} else {
			jobParams["DuplexMode"] = "auto"
		}

		if options.LocationCode != "" {
			jobParams["SlotPhysicalLocationCode"] = options.LocationCode
		}
		if options.ClientIP != "" {
			jobParams["IPAddress"] = options.ClientIP
		}
		if options.ServerIP != "" {
			jobParams["ServerIPAddress"] = options.ServerIP
		}
		if options.Gateway != "" {
			jobParams["Gateway"] = options.Gateway
		}
		if options.Netmask != "" {
			jobParams["SubnetMask"] = options.Netmask
		}

	} else {
		// Normal boot parameters
		jobParams["bootmode"] = bootMode
		jobParams["force"] = "false"
		jobParams["novsi"] = "true"

		if options.ProfileUUID != "" {
			jobParams["LogicalPartitionProfile"] = options.ProfileUUID
		}
		
		if keylock == "normal" {
			keylock = "norm"
		}
		jobParams["keylock"] = keylock
		
		if options.OSType == "OS400" && options.IIPLSource != "" {
			jobParams["iIPLsource"] = options.IIPLSource
		}
	}

	payload, err := createJobRequestPayload(reqdOperation, jobParams, schemaVersion, debug, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("X-HMC-Schema-Version", schemaVersion)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(respBody))
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return "", fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return "", fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	if debug {
		c.Logger.Debug("PowerOnPartition job submitted", "jobID", jobID)
	}

	// Network boot operations take significantly longer than normal boots
	// Use 30 minutes for netboot, 15 minutes for normal boot
	timeout := 15
	if bootMode == "netboot" {
		timeout = 30
		if debug {
			c.Logger.Debug("Using extended timeout for network boot operation", "timeoutMinutes", timeout)
		}
	}

	jobResp, err := c.FetchJobStatus(ctx, jobID, false, timeout, debug)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobResp.Status, nil
}

// PowerOffPartition powers off a logical partition directly by its UUID and returns the job status string.
func (c *HmcRestClient) PowerOffPartition(ctx context.Context, lparUUID, shutdownOption string, restart bool, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOff", c.hmcIP, lparUUID)
	if debug {
		c.Logger.Debug("Powering off partition", "partitionUUID", lparUUID, "url", url)
	}

	// Define operation details for the JobRequest 
	reqdOperation := map[string]string{
		"OperationName": "PowerOff",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Determine immediate flag and operation string based on shutdownOption
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
		return "", fmt.Errorf("invalid shutdownOption: %s, must be one of Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry", shutdownOption)
	}

	// Build job parameters for the XML payload
	jobParams := map[string]string{
		"immediate": immediate,
		"operation": operation,
		"restart":   fmt.Sprintf("%t", restart),
	}

	// Create XML payload using the existing job request helper
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Created JobRequest Payload", "payload", payload)
	}
	
	// Configure and execute the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(respBody))
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}
	if debug {
		c.Logger.Debug("Job Response", "body", string(respBody))
	}

	// Extract the JobID from the XML response
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return "", fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return "", fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	// Monitor the background job for completion
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 10, debug)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	// Return the status from the structured response
	return jobResp.Status, nil
}

// GetLogicalPartitionsInSystem retrieves the advanced list of logical partitions for a managed system as a slice of deeply parsed Go structs.
func (c *HmcRestClient) GetLogicalPartitionsInSystem(systemUUID string, debug bool) ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition", c.hmcIP, systemUUID)
	if debug {
		c.Logger.Debug("Fetching advanced logical partitions", "systemUUID", systemUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if debug {
		c.Logger.Debug("GetLogicalPartitions response status", "status", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Logger.Error("Request failed", "status", resp.StatusCode)
		if debug {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("request failed with status %d. Enable debug mode to see full response", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// Parse XML response and strip namespaces to make querying easier
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Use FindElements (plural) to capture all partitions in the Atom feed
	logicalPartitions := doc.FindElements("//LogicalPartition")
	if len(logicalPartitions) == 0 {
		if debug {
			c.Logger.Debug("No LogicalPartition elements found in the response feed.")
		}
		return []LogicalPartitionDetailed{}, nil // Return empty slice instead of error if none exist
	}

	if debug {
		c.Logger.Info("Successfully parsed partitions from Advanced XML", "count", len(logicalPartitions))
	}

	// Natively convert the XML nodes to Go Structs!
	return parseLogicalPartitionElements(logicalPartitions, debug)
}

// GetLogicalPartitionQuick retrieves quick details of a specific logical partition by UUID
func (c *HmcRestClient) GetLogicalPartitionQuick(partitionUUID string, debug bool) (*LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, partitionUUID)
	if debug {
		c.Logger.Debug("Fetching quick logical partition details", "partitionUUID", partitionUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetLogicalPartitionQuick response", "status", resp.Status, "body", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		if debug {
			c.Logger.Warn("Get of Logical Partition failed", "statusCode", resp.StatusCode)
		}
		return nil, nil
	}

	// Parse JSON response
	var partition LogicalPartitionQuick
	if err := json.Unmarshal(body, &partition); err != nil {
		c.Logger.Error("Failed to parse JSON response", "error", err)
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Manually set the UUID since it's not in the JSON response
	partition.UUID = partitionUUID

	if debug {
		c.Logger.Info("Found logical partition", "partitionName", partition.PartitionName, "partitionUUID", partition.UUID)
	}

	return &partition, nil
}

// GetLogicalPartitionsQuickAll retrieves the quick list of logical partitions for a system
func (c *HmcRestClient) GetLogicalPartitionsQuickAll(ctx context.Context, systemUUID string, debug bool) ([]LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/quick/All", c.hmcIP, systemUUID)
	if debug {
		c.Logger.Debug("Fetching quick logical partitions", "systemUUID", systemUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)

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
		c.Logger.Debug("GetLogicalPartitionsQuickAll response", "status", resp.Status, "body", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil // No partitions found
		}
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status: %s. Enable debug mode to see full response", resp.Status)
	}

	var lparList []LogicalPartitionQuick
	if err := json.Unmarshal(body, &lparList); err != nil {
		c.Logger.Error("Failed to parse JSON response", "error", err)
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully retrieved logical partitions", "count", len(lparList))
	}

	return lparList, nil
}

// DeleteLogicalPartition deletes a logical partition by its UUID.
func (c *HmcRestClient) DeleteLogicalPartition(partitionUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, partitionUUID)
	if debug {
		c.Logger.Debug("Deleting logical partition", "partitionUUID", partitionUUID, "url", url)
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

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body (if any)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("DeleteLogicalPartition response", "status", resp.Status)
		if len(body) > 0 {
			c.Logger.Debug("DeleteLogicalPartition response body", "body", string(body))
		}
	}

	// Check for success (204 No Content)
	if resp.StatusCode != http.StatusNoContent {
		// Attempt to parse error from body
		doc, err := xmlStripNamespace(body)
		if err == nil {
			errorMsgs := doc.FindElements("//Message")
			if len(errorMsgs) > 0 {
				c.Logger.Error("Delete failed", "status", resp.Status, "message", errorMsgs[0].Text())
				return fmt.Errorf("delete failed: %s, status: %s", errorMsgs[0].Text(), resp.Status)
			}
		}
		c.Logger.Error("Delete failed", "status", resp.Status)
		if debug {
			return fmt.Errorf("delete failed with status %s: %s", resp.Status, string(body))
		}
		return fmt.Errorf("delete failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	if debug {
		c.Logger.Info("Logical partition deleted successfully", "partitionUUID", partitionUUID)
	}

	return nil
}

func (c *HmcRestClient) GetLogicalPartitionsAdv(systemUUID string, debug bool) ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition?group=Advanced", c.hmcIP, systemUUID)
	
	if debug {
		c.Logger.Debug("Fetching Advanced logical partitions", "systemUUID", systemUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("failed: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed: %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	// Capture the raw LogicalPartition elements from inside the Atom feed
	elements := doc.FindElements("//LogicalPartition")
	
	var detailedPartitions []LogicalPartitionDetailed
	for _, lparElem := range elements {
		// Serialize the isolated element
		lparDoc := etree.NewDocument()
		lparDoc.SetRoot(lparElem.Copy())
		lparBytes, _ := lparDoc.WriteToBytes()

		var detailedLpar LogicalPartitionDetailed
		if err := xml.Unmarshal(lparBytes, &detailedLpar); err != nil {
			if debug {
				c.Logger.Warn("Unmarshal warning for LPAR", "error", err)
			}
			continue
		}
		detailedPartitions = append(detailedPartitions, detailedLpar)
	}

	if debug {
		c.Logger.Info("Successfully parsed Advanced logical partitions", "count", len(detailedPartitions))
	}

	return detailedPartitions, nil
}



// CreateLogicalPartition creates a new LPAR using the direct UOM PUT method and returns the complete LPAR details.
func (c *HmcRestClient) CreateLogicalPartition(sysUUID string, req CreateLparRequest, debug bool) (*LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition", c.hmcIP, sysUUID)

	if debug {
		c.Logger.Debug("Creating Logical Partition", "systemUUID", sysUUID, "lparName", req.Name)
	}

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
    <uom:PartitionType>%s</uom:PartitionType>
</uom:LogicalPartition>`,
		req.DesiredMem, req.MaxMem, req.MinMem, 
		req.Name, 
		req.DesiredProcUnits, req.DesiredVcpus, // Desired (D)
		req.MaxProcUnits, req.MaxVcpus,         // Maximum (M)
		req.MinProcUnits, req.MinVcpus,         // Minimum (Mi)
		req.SharingMode,
		req.OsType)

	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.Logger.Error("UOM creation failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("UOM creation failed (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("UOM creation failed (%s). Enable debug mode to see full response", resp.Status)
	}

	if debug {
		c.Logger.Debug("CreateLogicalPartition response status", "status", resp.Status)
	}

	// Parse the complete LogicalPartition response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	// Extract the LogicalPartition element
	lparElem := doc.FindElement("//LogicalPartition")
	if lparElem == nil {
		return nil, fmt.Errorf("LogicalPartition element not found in response")
	}

	// Convert etree element back to XML bytes for unmarshaling
	lparDoc := etree.NewDocument()
	lparDoc.SetRoot(lparElem.Copy())
	lparXML, err := lparDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize LogicalPartition element: %v", err)
	}

	// Unmarshal into LogicalPartitionDetailed struct
	var lpar LogicalPartitionDetailed
	if err := xml.Unmarshal(lparXML, &lpar); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LogicalPartition: %v", err)
	}

	if debug {
		c.Logger.Info("LPAR Created!", "uuid", lpar.MetadataID, "name", lpar.PartitionName, "profile", lpar.DefaultProfileName)
	}

	return &lpar, nil
}

// CreateVirtualSCSIClientAdapter adds a vSCSI Client Adapter and strictly maps it to a VIOS.
func (c *HmcRestClient) CreateVirtualSCSIClientAdapter(lparUUID string, viosID, viosSlot int, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Adding vSCSI Client Adapter mapping", "viosID", viosID, "viosSlot", viosSlot)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.Logger.Error("vSCSI Adapter creation failed", "status", resp.Status)
		if debug {
			return "", fmt.Errorf("vSCSI Adapter creation failed (%s): %s", resp.Status, string(body))
		}
		return "", fmt.Errorf("vSCSI Adapter creation failed (%s). Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("Adapter created successfully, but failed to extract new UUID")
	}

	if debug {
		c.Logger.Info("Successfully created Virtual SCSI Client Adapter", "uuid", atomID.Text())
	}

	return atomID.Text(), nil
}

// GetLogicalPartitionDetailed fetches the exhaustive XML details of a specific logical partition by its UUID.
func (c *HmcRestClient) GetLogicalPartitionDetailed(ctx context.Context, lparUUID string, debug bool) (*LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	if debug {
		c.Logger.Debug("Fetching exhaustive logical partition details", "lparUUID", lparUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(ctxWithTimeout)

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
		c.Logger.Debug("GetLogicalPartitionDetailed response status", "status", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.StatusCode)
		if debug {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("request failed with status %d. Enable debug mode to see full response", resp.StatusCode)
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

	if debug {
		c.Logger.Info("Successfully parsed exhaustive details for LPAR", "partitionName", detailedLpar.PartitionName)
	}

	return &detailedLpar, nil
}

// MapPhysicalIOAdapters dynamically assigns Physical I/O Adapters to an LPAR.
// It rigorously validates ownership against the system inventory.
// Returns a list of successfully attached adapters, a list of skipped adapters, and any errors.
func (c *HmcRestClient) MapPhysicalIOAdapters(ctx context.Context, sysUUID, lparUUID, lparName string, targets []string, inventory *ManagedSystemDetailed, debug bool) ([]string, []string, error) {
	if inventory == nil {
		return nil, nil, fmt.Errorf("inventory is required for pre-flight validation")
	}

	type SlotInjection struct {
		DRCIndex     string
		LocationCode string
		TargetName   string
	}
	var slotsToInject []SlotInjection
	var attached []string
	var skipped []string

	// =====================================================================
	// 1. PRE-FLIGHT VALIDATION
	// =====================================================================
	for _, target := range targets {
		found := false
		cleanTarget := strings.ToLower(strings.TrimSpace(target))

		for _, bus := range inventory.IOConfig.IOBuses {
			for _, slot := range bus.IOSlots {
				actualLocCode := slot.PhysicalLocationCode
				if slot.RelatedIOAdapter.DeviceName != "" {
					actualLocCode = slot.RelatedIOAdapter.DeviceName
				}

				if strings.ToLower(actualLocCode) == cleanTarget || strings.ToLower(slot.PCAdapterID) == cleanTarget || strings.ToLower(slot.ConnectorIndex) == cleanTarget {
					found = true
					
					if slot.PartitionID > 0 {
						// It is owned by someone. Is it us?
						if !strings.EqualFold(slot.PartitionName, lparName) {
							return nil, nil, fmt.Errorf("ABORT: Target '%s' is currently assigned to a DIFFERENT LPAR ('%s', ID: %d)", target, slot.PartitionName, slot.PartitionID)
						}
						// It's already ours! Skip it.
						skipped = append(skipped, target)
					} else {
						// It's unassigned. Safe to map.
						slotsToInject = append(slotsToInject, SlotInjection{
							DRCIndex:     slot.ConnectorIndex,
							LocationCode: actualLocCode,
							TargetName:   target,
						})
					}
					break
				}
			}
			if found { break }
		}
		if !found {
			return nil, nil, fmt.Errorf("ABORT: Target '%s' does not exist on the Managed System", target)
		}
	}

	// If everything was already mapped, exit early!
	if len(slotsToInject) == 0 {
		return attached, skipped, nil
	}

	// =====================================================================
	// 2. FETCH PRISTINE LPAR XML
	// =====================================================================
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil { return nil, nil, err }
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil { return nil, nil, err }
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != 200 { return nil, nil, fmt.Errorf("GET failed: %s", string(rawXML)) }

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil { return nil, nil, err }
	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil { return nil, nil, fmt.Errorf("LogicalPartition element not found") }

	ioConfig := lparElem.FindElement(".//*[local-name()='PartitionIOConfiguration']")
	if ioConfig == nil { return nil, nil, fmt.Errorf("PartitionIOConfiguration not found") }

	profileSlots := ioConfig.FindElement(".//*[local-name()='ProfileIOSlots']")
	if profileSlots == nil {
		profileSlots = ioConfig.CreateElement("ProfileIOSlots")
		profileSlots.CreateAttr("schemaVersion", "V1_0")
	}

	// =====================================================================
	// 3. INJECT ADAPTERS
	// =====================================================================
	for _, slotInfo := range slotsToInject {
		// Strict XML Deduplication Check (in case inventory was stale)
		duplicate := false
		for _, existingSlot := range profileSlots.FindElements(".//*[local-name()='AssociatedIOSlot']/*[local-name()='SlotDynamicReconfigurationConnectorIndex']") {
			if existingSlot.Text() == slotInfo.DRCIndex {
				duplicate = true
				break
			}
		}
		
		if duplicate {
			skipped = append(skipped, slotInfo.TargetName)
			continue
		}

		newSlotXML := fmt.Sprintf(`
			<ProfileIOSlot schemaVersion="V1_0">
				<AssociatedIOSlot schemaVersion="V1_0">
					<SlotDynamicReconfigurationConnectorIndex>%s</SlotDynamicReconfigurationConnectorIndex>
					<SlotPhysicalLocationCode>%s</SlotPhysicalLocationCode>
				</AssociatedIOSlot>
				<IsRequired>false</IsRequired>
			</ProfileIOSlot>`, slotInfo.DRCIndex, slotInfo.LocationCode)
		
		slotDoc := etree.NewDocument()
		slotDoc.ReadFromString(newSlotXML)
		profileSlots.AddChild(slotDoc.Root())
		
		attached = append(attached, slotInfo.TargetName)
	}

	if len(attached) == 0 {
		return attached, skipped, nil
	}

	// =====================================================================
	// 4. POST UPDATED XML
	// =====================================================================
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	if err != nil { return nil, nil, err }
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil { return nil, nil, err }
	defer postResp.Body.Close()
	body, _ := io.ReadAll(postResp.Body)

	if postResp.StatusCode >= 400 { return nil, nil, fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body)) }

	return attached, skipped, nil
}

// UnmapPhysicalIOAdapters dynamically detaches Physical I/O Adapters from an LPAR.
func (c *HmcRestClient) UnmapPhysicalIOAdapters(ctx context.Context, sysUUID, lparUUID, lparName string, targets []string, inventory *ManagedSystemDetailed, debug bool) ([]string, []string, error) {
	if inventory == nil {
		return nil, nil, fmt.Errorf("inventory is required for pre-flight validation")
	}

	var drcIndexesToRemove = make(map[string]string) // Map DRC Index -> Original Target Name
	var detached []string
	var skipped []string

	// =====================================================================
	// 1. PRE-FLIGHT VALIDATION
	// =====================================================================
	for _, target := range targets {
		foundAndAttached := false
		cleanTarget := strings.ToLower(strings.TrimSpace(target))

		for _, bus := range inventory.IOConfig.IOBuses {
			for _, slot := range bus.IOSlots {
				actualLocCode := slot.PhysicalLocationCode
				if slot.RelatedIOAdapter.DeviceName != "" { actualLocCode = slot.RelatedIOAdapter.DeviceName }

				if strings.ToLower(actualLocCode) == cleanTarget || strings.ToLower(slot.PCAdapterID) == cleanTarget || strings.ToLower(slot.ConnectorIndex) == cleanTarget {
					if strings.EqualFold(slot.PartitionName, lparName) {
						// It is attached to our LPAR! Mark it for removal.
						drcIndexesToRemove[slot.ConnectorIndex] = target
						foundAndAttached = true
					} else if slot.PartitionID == 0 || slot.PartitionName == "" {
						// ✨ THE FIX: Target is completely unassigned (owned by hypervisor). 
						// This means it is already detached from our perspective. Break to add to 'skipped' list.
						break
					} else {
						// It is attached to a DIFFERENT LPAR! Abort to prevent taking down another VM.
						return nil, nil, fmt.Errorf("ABORT: Target '%s' is not attached to LPAR '%s'. (Currently assigned to '%s')", target, lparName, slot.PartitionName)
					}
					break
				}
			}
			if foundAndAttached { break }
		}
		
		if !foundAndAttached {
			// It's not attached to us (or doesn't exist). We skip it!
			skipped = append(skipped, target)
		}
	}

	if len(drcIndexesToRemove) == 0 {
		return detached, skipped, nil
	}

	// =====================================================================
	// 2. FETCH LPAR XML
	// =====================================================================
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil { return nil, nil, err }
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil { return nil, nil, err }
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != 200 { return nil, nil, fmt.Errorf("GET failed: %s", string(rawXML)) }

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil { return nil, nil, err }
	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil { return nil, nil, fmt.Errorf("LogicalPartition element not found") }

	ioConfig := lparElem.FindElement(".//*[local-name()='PartitionIOConfiguration']")
	if ioConfig == nil { return detached, skipped, nil }

	profileSlots := ioConfig.FindElement(".//*[local-name()='ProfileIOSlots']")
	if profileSlots == nil { return detached, skipped, nil }

	// =====================================================================
	// 3. REMOVE NODES
	// =====================================================================
	for _, slot := range profileSlots.FindElements(".//*[local-name()='ProfileIOSlot']") {
		drcElem := slot.FindElement(".//*[local-name()='SlotDynamicReconfigurationConnectorIndex']")
		if drcElem != nil {
			if targetName, exists := drcIndexesToRemove[drcElem.Text()]; exists {
				profileSlots.RemoveChild(slot)
				detached = append(detached, targetName)
			}
		}
	}

	if len(detached) == 0 {
		return detached, skipped, nil
	}

	// =====================================================================
	// 4. POST UPDATED XML
	// =====================================================================
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	if err != nil { return nil, nil, err }
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil { return nil, nil, err }
	defer postResp.Body.Close()
	body, _ := io.ReadAll(postResp.Body)

	if postResp.StatusCode >= 400 { return nil, nil, fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body)) }

	return detached, skipped, nil
}


// GetAllLogicalPartitionsInHmc retrieves the Go structures for all logical partitions managed by the HMC across all systems.
func (c *HmcRestClient) GetAllLogicalPartitionsInHmc(debug bool) ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition", c.hmcIP)
	if debug {
		c.Logger.Debug("Fetching ALL logical partitions across the HMC", "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

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

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.StatusCode)
		if debug {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("request failed with status %d. Enable debug mode to see full response", resp.StatusCode)
	}

	// 1. Strip namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	// 2. Locate all partition blocks
	elements := doc.FindElements("//LogicalPartition")
	
	var allPartitions []LogicalPartitionDetailed
	for _, lparElem := range elements {
		// 3. Isolate the element XML
		lparDoc := etree.NewDocument()
		lparDoc.SetRoot(lparElem.Copy())
		lparBytes, err := lparDoc.WriteToBytes()
		if err != nil {
			continue 
		}

		// 4. Unmarshal into your big struct
		var detailedLpar LogicalPartitionDetailed
		if err := xml.Unmarshal(lparBytes, &detailedLpar); err != nil {
			if debug {
				c.Logger.Warn("Skipping partition due to unmarshal error", "error", err)
			}
			continue
		}
		allPartitions = append(allPartitions, detailedLpar)
	}

	if debug {
		c.Logger.Info("Successfully parsed partitions from HMC global inventory", "count", len(allPartitions))
	}

	return allPartitions, nil
}
// GetLogicalPartitionQuickProperty retrieves a specific quick property of a logical partition.
// This endpoint provides a fast way to poll specific states without downloading the entire LPAR XML.
//
// Supported property names include:
//   - IsVirtualServiceAttentionLEDOn : The virtual service attention LED state.
//   - MigrationState                 : The state of the partition's migration operation.
//   - ProgressState                  : The progress state of the partition's hibernation operation.
//   - PartitionType                  : The partition environment (e.g., 'AIX/Linux', 'OS400', or 'Virtual IO Server').
//   - PartitionName                  : The name of the partition.
//   - PartitionID                    : The integer ID of the partition.
//   - PartitionState                 : The state of the partition (e.g., 'running', 'not activated').
//   - RemoteRestartState             : The state of the partition's Remote Restart operation.
//   - AssociatedManagedSystem        : The REST URI of the partition's parent managed system.
//   - RMCState                       : The state of the partition's Resource Monitoring Control (RMC) connection.
//   - PowerManagementMode            : The power management mode.
//
// Returns the cleaned value as a string (quotes and whitespace removed).
func (c *HmcRestClient) GetLogicalPartitionQuickProperty(lparUUID, propertyName string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick/%s", c.hmcIP, lparUUID, propertyName)
	if debug {
		c.Logger.Debug("Fetching quick property for LPAR", "propertyName", propertyName, "lparUUID", lparUUID, "url", url)
	}

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Requesting JSON format

	// Set a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	// Execute the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return "", fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return "", fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// The HMC often returns single properties wrapped in JSON quotes (e.g., "running").
	// We trim the whitespace and the quotes to return a clean Go string.
	cleanValue := strings.TrimSpace(string(body))
	cleanValue = strings.Trim(cleanValue, "\"")

	if debug {
		c.Logger.Info("Property retrieved", "propertyName", propertyName, "value", cleanValue)
	}

	return cleanValue, nil
}
// SearchLogicalPartitions queries the HMC for logical partitions matching a specific property and value.
// Example: SearchLogicalPartitions("PartitionState", "running", true)
func (c *HmcRestClient) SearchLogicalPartitions(propertyName, propertyValue string, debug bool) ([]*etree.Element, error) {
	// Construct the HMC search string: (Property==Value)
	// Note: If your value contains spaces or special characters (like AIX/Linux), 
	// the HMC usually expects single quotes around it (e.g., 'AIX/Linux').
	searchQuery := fmt.Sprintf("(%s==%s)", propertyName, propertyValue)
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/search/%s", c.hmcIP, searchQuery)

	if debug {
		c.Logger.Debug("Searching for Logical Partitions", "propertyName", propertyName, "propertyValue", propertyValue, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	// Set a reasonable timeout
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

	if debug {
		c.Logger.Debug("SearchLogicalPartitions response status", "status", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Logger.Error("Request failed", "status", resp.StatusCode)
		if debug {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("request failed with status %d. Enable debug mode to see full response", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// Strip namespaces for easy querying
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract all LogicalPartition blocks returned by the search
	logicalPartitions := doc.FindElements("//LogicalPartition")
	if len(logicalPartitions) == 0 {
		if debug {
			c.Logger.Debug("No matching LogicalPartition elements found", "searchQuery", searchQuery)
		}
		return []*etree.Element{}, nil
	}

	if debug {
		c.Logger.Info("Successfully found partitions matching criteria", "count", len(logicalPartitions))
	}

	return logicalPartitions, nil
}

// ChangeDefaultProfileName changes the default profile of a logical partition.
// This job is used to assign another profile of a logical partition as the default profile.
//
// Parameters:
//   - lparUUID: The UUID of the logical partition
//   - profileName: The name of the profile to be assigned as the default profile
//   - debug: If true, logs detailed information
//
// Returns:
//   - jobID: The job ID for tracking the operation
//   - error: Error if the operation fails, nil on success
//
// Reference: https://www.ibm.com/docs/en/power10/7063-CR1?topic=jobs-changedefaultprofilename-logicalpartition-job
func (c *HmcRestClient) ChangeDefaultProfileName(lparUUID, profileName string, debug bool) (string, error) {
	// Construct the URL for the ChangeDefaultProfileName operation
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/ChangeDefaultProfileName", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Changing default profile", "lparUUID", lparUUID, "profileName", profileName, "url", url)
	}

	// Define operation details for the JobRequest
	reqdOperation := map[string]string{
		"OperationName": "ChangeDefaultProfileName",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters with the profile name
	jobParams := map[string]string{
		"DefaultProfileName": profileName,
	}

	// Create XML payload using existing helper
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}

	// Configure and execute the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if debug {
		c.Logger.Debug("Response status", "status", resp.Status)
		c.Logger.Debug("Response body", "body", string(respBody))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(respBody))
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response and extract JobID
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return "", fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return "", fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	if debug {
		c.Logger.Info("ChangeDefaultProfileName job submitted successfully", "jobID", jobID)
	}

	return jobID, nil
}

// CreateVirtualFibreChannelClientAdapter adds a vFC Client Adapter to an LPAR and maps it to a VIOS.
// NOTE: Unlike vSCSI, vFC strictly requires an explicit viosSlot number. It cannot be auto-assigned.
func (c *HmcRestClient) CreateVirtualFibreChannelClientAdapter(lparUUID string, viosID, viosSlot int, debug bool) (string, error) {
	
	// FAIL-FAST: IBM Hypervisor strictly requires a target slot for NPIV mapping.
	if viosSlot <= 0 {
		return "", fmt.Errorf("viosSlot must be greater than 0. The HMC does not support auto-assigning remote slots for Virtual Fibre Channel adapters")
	}

	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualFibreChannelClientAdapter", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Provisioning vFC Client Adapter", "viosID", viosID, "viosSlot", viosSlot)
	}

	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VirtualFibreChannelClientAdapter:VirtualFibreChannelClientAdapter xmlns:VirtualFibreChannelClientAdapter="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                                                   xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                                                   schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <AdapterType>Client</AdapterType>
    <RequiredAdapter>false</RequiredAdapter>
    <ConnectingPartitionID>%d</ConnectingPartitionID>
    <ConnectingVirtualSlotNumber>%d</ConnectingVirtualSlotNumber>
</VirtualFibreChannelClientAdapter:VirtualFibreChannelClientAdapter>`, viosID, viosSlot)


	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualFibreChannelClientAdapter")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.Logger.Error("vFC Adapter creation failed", "status", resp.Status)
		if debug {
			return "", fmt.Errorf("vFC Adapter creation failed (%s):\n%s", resp.Status, string(body))
		}
		return "", fmt.Errorf("vFC Adapter creation failed (%s). Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("vFC Adapter created successfully, but failed to extract new UUID")
	}

	if debug {
		c.Logger.Info("vFC Adapter created successfully", "uuid", atomID.Text())
	}

	return atomID.Text(), nil
}
// GetVirtualFibreChannelClientAdapters retrieves all vFC Client Adapters attached to a specific LPAR.
func (c *HmcRestClient) GetVirtualFibreChannelClientAdapters(lparUUID string, debug bool) ([]VirtualFibreChannelClientAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualFibreChannelClientAdapter", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Fetching Virtual Fibre Channel Client Adapters", "lparUUID", lparUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// LOG THE REQUEST (GET requests have no payload, so we pass an empty string)
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

	// LOG THE RAW RESPONSE
	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		// Handle the case where the LPAR simply has zero vFC adapters attached
		if resp.StatusCode == http.StatusNoContent {
			if debug {
				c.Logger.Debug("No vFC adapters found on LPAR (204 No Content)")
			}
			return []VirtualFibreChannelClientAdapter{}, nil
		}
		c.Logger.Error("Failed to fetch vFC adapters", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	// Parse XML response and strip namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var adapters []VirtualFibreChannelClientAdapter

	// Extract the core elements and natively unmarshal them into structs
	adapterElements := doc.FindElements("//VirtualFibreChannelClientAdapter")
	
	for _, elem := range adapterElements {
		adapterDoc := etree.NewDocument()
		adapterDoc.SetRoot(elem.Copy())
		adapterBytes, err := adapterDoc.WriteToBytes()
		if err != nil {
			c.Logger.Warn("Failed to serialize adapter element", "error", err)
			continue
		}

		var adapter VirtualFibreChannelClientAdapter
		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			c.Logger.Warn("Failed to unmarshal adapter XML", "error", err)
			continue
		}
		
		adapters = append(adapters, adapter)
	}

	if debug {
		c.Logger.Info("Successfully retrieved vFC adapters", "count", len(adapters), "lparUUID", lparUUID)
	}

	return adapters, nil
}
// SetPartitionBootString updates the LPAR's PendingBootString (e.g., "cd/dvd-all") 
// to force a specific boot device priority on the next power-on.
func (c *HmcRestClient) SetPartitionBootString(ctx context.Context, lparUUID, bootString string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Setting partition boot string", "lparUUID", lparUUID, "bootString", bootString)
	}

	// 1. Raw GET - Fetch pristine LPAR XML to preserve namespaces and attributes
	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	getReq = getReq.WithContext(ctxWithTimeout)

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != 200 {
		return fmt.Errorf("GET failed: %s", string(rawXML))
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil {
		return fmt.Errorf("LogicalPartition element not found in response")
	}

	// 3. Find or create BootListInformation block
	bootListInfo := lparElem.FindElement(".//*[local-name()='BootListInformation']")
	if bootListInfo == nil {
		bootListInfo = lparElem.CreateElement("BootListInformation")
	}

	// 4. Find or create PendingBootString and inject our target string
	pendingBootStr := bootListInfo.FindElement(".//*[local-name()='PendingBootString']")
	if pendingBootStr == nil {
		pendingBootStr = bootListInfo.CreateElement("PendingBootString")
	}
	pendingBootStr.SetText(bootString)

	// 5. Extract the modified document to POST
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	// 6. Execute POST
	postReq, _ := http.NewRequest("POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")
	postReq = postReq.WithContext(ctxWithTimeout)

	c.logRawTraffic("REQUEST (POST)", url, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", url, string(body))

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	return nil
}

// GetDedicatedVirtualNICs fetches the detailed configurations of all Dedicated vNICs attached to an LPAR.
func (c *HmcRestClient) GetDedicatedVirtualNICs(ctx context.Context, lparUUID string, debug bool) ([]VirtualNICDedicated, error) {
	// Query the Dedicated Virtual NIC child collection
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualNICDedicated", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Fetching Dedicated Virtual NICs for LPAR", "lparUUID", lparUUID)
	}

	// 1. Fetch and strip namespaces into an etree Document
	doc, err := c.fetchAndParseHMCXML(ctx, url, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VirtualNICDedicated collection: %v", err)
	}

	var vnics []VirtualNICDedicated

	// 2. Bypass the Atom <feed> and <entry> wrappers, slice out the actual vNIC elements
	vnicElements := doc.FindElements("//VirtualNICDedicated")
	
	for _, vnicElem := range vnicElements {
		tempDoc := etree.NewDocument()
		tempDoc.SetRoot(vnicElem.Copy())
		vnicBytes, _ := tempDoc.WriteToBytes()

		var vnicInfo VirtualNICDedicated
		if err := xml.Unmarshal(vnicBytes, &vnicInfo); err != nil {
			if debug {
				c.Logger.Warn("Failed to unmarshal VirtualNICDedicated XML", "error", err)
			}
			continue
		}

		vnics = append(vnics, vnicInfo)
	}

	if debug {
		c.Logger.Debug("Successfully extracted Dedicated Virtual NICs", "lparUUID", lparUUID, "count", len(vnics))
	}

	return vnics, nil
}
// GetSRIOVLogicalPorts fetches all SR-IOV Ethernet Logical Ports provisioned to a specific Logical Partition.
func (c *HmcRestClient) GetSRIOVLogicalPorts(ctx context.Context, lparUUID string, debug bool) ([]SRIOVLogicalPort, error) {
	// Query the child collection directly for this LPAR
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/SRIOVEthernetLogicalPort", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Fetching SR-IOV Logical Ports for LPAR", "lparUUID", lparUUID)
	}

	// 1. Fetch and strip namespaces into an etree Document
	doc, err := c.fetchAndParseHMCXML(ctx, url, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SR-IOV Logical Ports: %v", err)
	}

	// 2. Bypass the <feed><entry> Atom wrappers and find the actual port elements
	portElements := doc.FindElements("//SRIOVEthernetLogicalPort")
	var logicalPorts []SRIOVLogicalPort

	// 3. Unmarshal each element directly into our Go struct
	for _, portElem := range portElements {
		tempDoc := etree.NewDocument()
		tempDoc.SetRoot(portElem.Copy())
		portBytes, _ := tempDoc.WriteToBytes()

		var port SRIOVLogicalPort
		if err := xml.Unmarshal(portBytes, &port); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SRIOVLogicalPort XML: %v", err)
		}
		logicalPorts = append(logicalPorts, port)
	}

	if debug {
		c.Logger.Debug("Successfully extracted SR-IOV Logical Ports", "lparUUID", lparUUID, "count", len(logicalPorts))
	}

	return logicalPorts, nil
}

// CreateSRIOVLogicalPort provisions a new SR-IOV Ethernet Logical Port to a specific Logical Partition.
func (c *HmcRestClient) CreateSRIOVLogicalPort(lparUUID string, adapterID string, physicalPortID string, opts SRIOVPortCreateOptions, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/SRIOVEthernetLogicalPort", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Provisioning SR-IOV Logical Port", "lparUUID", lparUUID, "adapterID", adapterID, "physicalPortID", physicalPortID)
	}

	// 1. Construct the base payload natively
	reqPayload := SRIOVLogicalPortRequest{
		SchemaVersion:  "V1_3_0",
		XMLNS:          "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/",
		AdapterID:      adapterID,
		PhysicalPortID: physicalPortID,
	}

	// 2. Apply Advanced Options
	if opts.Capacity != "" {
		cap := opts.Capacity
		if !strings.HasSuffix(cap, "%") {
			cap += "%"
		}
		reqPayload.ConfiguredCapacity = &cap
	}

	if opts.PortVLANID != "" {
		reqPayload.PortVLANID = &opts.PortVLANID
	}

	// Handle IBM's boolean logic as a string
	promiscuousStr := "false"
	if opts.IsPromiscuous {
		promiscuousStr = "true"
	}
	reqPayload.IsPromiscous = &promiscuousStr

	if opts.AllowedMACAddresses != "" {
		reqPayload.AllowedMACAddresses = &opts.AllowedMACAddresses
	}

	if opts.AllowedVLANs != "" {
		reqPayload.AllowedVLANs = &opts.AllowedVLANs
	}

	if opts.Allowed8021QPriorities != "" {
		reqPayload.IEEE8021QAllowablePriorities = &opts.Allowed8021QPriorities
	}

	// 3. Marshal into XML
	payloadBytes, err := xml.MarshalIndent(reqPayload, "", "  ")
	if err != nil {
		return  "FAILED",fmt.Errorf("failed to marshal SR-IOV Logical Port request: %v", err)
	}

	if debug {
		c.Logger.Debug(fmt.Sprintf("Create Payload:\n%s", string(payloadBytes)))
	}

	// 4. Execute PUT Request
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return  "FAILED",err
	}

	// Exact Header Matching
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml;type=SRIOVEthernetLogicalPort")
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml, */*")
	req.Header.Set("X-API-Session", c.session)

	resp, err := c.client.Do(req)
	if err != nil {
		return  "FAILED",fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "FAILED",fmt.Errorf("failed to read response body: %v", err)
	}

	// 5. Evaluate the HMC Response
	if resp.StatusCode >= 400 {
		return  "FAILED",fmt.Errorf("failed to create SR-IOV Logical Port (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return "SUCCESS", nil
}

// DeleteSRIOVLogicalPorts intelligently removes one or more SR-IOV Ethernet Logical Ports.
// It accepts a list of targets which can be either the LogicalPortID or the LocationCode.
func (c *HmcRestClient) DeleteSRIOVLogicalPorts(ctx context.Context, lparUUID string, targets []string, debug bool) error {
	if len(targets) == 0 {
		return nil // Nothing to delete
	}

	if debug {
		c.Logger.Debug("Initiating Smart Deletion for SR-IOV Logical Ports", "lparUUID", lparUUID, "targets", targets)
	}

	// 1. Fetch current ports on the LPAR so we can resolve IDs/Locations to UUIDs
	currentPorts, err := c.GetSRIOVLogicalPorts(ctx, lparUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to fetch current SR-IOV logical ports for resolution: %v", err)
	}

	// 2. Resolve the user's targets into actual Atom UUIDs
	var uuidsToDelete []string
	for _, target := range targets {
		cleanTarget := strings.TrimSpace(target)
		if cleanTarget == "" {
			continue
		}

		found := false
		for _, port := range currentPorts {
			// Match against Logical Port ID, Location Code, OR the UUID itself
			if strings.EqualFold(port.LogicalPortID, cleanTarget) ||
				strings.EqualFold(port.LocationCode, cleanTarget) ||
				strings.EqualFold(port.UUID, cleanTarget) {
				
				uuidsToDelete = append(uuidsToDelete, port.UUID)
				found = true
				if debug {
					c.Logger.Debug("Resolved Target", "input", cleanTarget, "uuid", port.UUID)
				}
				break
			}
		}

		if !found && debug {
			c.Logger.Warn("⚠️ Deletion target not found on LPAR (Skipping)", "target", cleanTarget)
		}
	}

	if len(uuidsToDelete) == 0 {
		return fmt.Errorf("none of the specified targets were found on this LPAR")
	}

	// 3. Execute the DELETE request for each resolved UUID
	for _, portUUID := range uuidsToDelete {
		url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/SRIOVEthernetLogicalPort/%s", c.hmcIP, lparUUID, portUUID)

		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create delete request for %s: %v", portUUID, err)
		}

		req.Header.Set("X-API-Session", c.session)

		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("HTTP request failed for %s: %v", portUUID, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("failed to delete SR-IOV Logical Port %s (HTTP %d): %s", portUUID, resp.StatusCode, string(body))
		}

		if debug {
			c.Logger.Debug("Successfully deleted SR-IOV Logical Port", "portUUID", portUUID)
		}
	}

	return nil
}
// GetRawLparXML fetches the raw XML payload for a Logical Partition (Useful for diffs and backups)
func (c *HmcRestClient) GetRawLparXML(sysUUID, lparUUID string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Fetching raw LPAR XML", "lparUUID", lparUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch LPAR XML (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}