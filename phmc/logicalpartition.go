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
func (c *RestClient) PowerOnPartition(ctx context.Context, lparUUID string, options *PowerOnOptions) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOn", c.hmcIP, lparUUID)


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

	payload, err := createJobRequestPayload(reqdOperation, jobParams, schemaVersion, true)
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


	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
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


	// Network boot operations take significantly longer than normal boots
	// Use 30 minutes for netboot, 15 minutes for normal boot
	timeout := 15
	if bootMode == "netboot" {
		timeout = 30
	}

	jobResp, err := c.FetchJobStatus(ctx, jobID, false, timeout)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobResp.Status, nil
}

// PowerOffPartition powers off a logical partition directly by its UUID and returns the job status string.
func (c *RestClient) PowerOffPartition(ctx context.Context, lparUUID, shutdownOption string, restart bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/PowerOff", c.hmcIP, lparUUID)

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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", true)
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


	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
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
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 10)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	// Return the status from the structured response
	return jobResp.Status, nil
}

// GetLogicalPartitionsInSystem retrieves the advanced list of logical partitions for a managed system as a slice of deeply parsed Go structs.
func (c *RestClient) GetLogicalPartitionsInSystem(systemUUID string) ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition", c.hmcIP, systemUUID)

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


	// Handle 204 No Content - system has no logical partitions
	if resp.StatusCode == http.StatusNoContent {
		return []LogicalPartitionDetailed{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
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

	// Use FindElements (plural) to capture all partitions in the Atom feed
	logicalPartitions := doc.FindElements("//LogicalPartition")
	if len(logicalPartitions) == 0 {
		return []LogicalPartitionDetailed{}, nil // Return empty slice instead of error if none exist
	}


	// Natively convert the XML nodes to Go Structs!
	return parseLogicalPartitionElements(logicalPartitions)
}

// GetLogicalPartitionQuick retrieves quick details of a specific logical partition by UUID
func (c *RestClient) GetLogicalPartitionQuick(partitionUUID string) (*LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, partitionUUID)

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)


	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}



	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	// Parse JSON response
	var partition LogicalPartitionQuick
	if err := json.Unmarshal(body, &partition); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Manually set the UUID since it's not in the JSON response
	partition.UUID = partitionUUID


	return &partition, nil
}

// GetLogicalPartitionsQuickAll retrieves the quick list of logical partitions for a system
func (c *RestClient) GetLogicalPartitionsQuickAll(ctx context.Context, systemUUID string) ([]LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/quick/All", c.hmcIP, systemUUID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json, application/atom+xml, application/xml, */*")

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}



	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil // No partitions found
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var lparList []LogicalPartitionQuick

	// Try JSON first (preferred format)
	if err := json.Unmarshal(body, &lparList); err != nil {
		// If JSON fails, try XML format (fallback for older HMC versions)

		// Parse XML response using Atom feed format
		type AtomEntry struct {
			ID      string `xml:"id"`
			Content struct {
				LPARQuick struct {
					PartitionName string `xml:"PartitionName"`
					UUID          string `xml:"Metadata>Atom>AtomID"`
				} `xml:"LogicalPartition"`
			} `xml:"content"`
		}

		type AtomFeed struct {
			XMLName xml.Name    `xml:"feed"`
			Entries []AtomEntry `xml:"entry"`
		}

		var feed AtomFeed
		if xmlErr := xml.Unmarshal(body, &feed); xmlErr != nil {
			return nil, fmt.Errorf("failed to parse response (tried JSON and XML): json error: %v, xml error: %v", err, xmlErr)
		}

		// Convert XML entries to LogicalPartitionQuick structs
		for _, entry := range feed.Entries {
			lpar := LogicalPartitionQuick{
				PartitionName: entry.Content.LPARQuick.PartitionName,
				UUID:          entry.Content.LPARQuick.UUID,
			}
			lparList = append(lparList, lpar)
		}

	} else {
	}

	return lparList, nil
}

// DeleteLogicalPartition deletes a logical partition by its UUID.
func (c *RestClient) DeleteLogicalPartition(partitionUUID string) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, partitionUUID)

	// Create and configure the DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)


	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body (if any)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
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
		return fmt.Errorf("delete failed with status %s", resp.Status)
	}


	return nil
}

// GetLogicalPartitionsAdv retrieves logical partitions for a managed system with Advanced group attributes as a slice of deeply parsed Go structs.
func (c *RestClient) GetLogicalPartitionsAdv(systemUUID string) ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition?group=Advanced", c.hmcIP, systemUUID)


	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed: %s", resp.Status)
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
			continue
		}
		detailedPartitions = append(detailedPartitions, detailedLpar)
	}


	return detailedPartitions, nil
}

// CreateLogicalPartition creates a new LPAR using the direct UOM PUT method and returns the complete LPAR details.
// Supports both shared and dedicated processor configurations based on req.DedicatedProc flag.
func (c *RestClient) CreateLogicalPartition(sysUUID string, req CreateLparRequest) (*LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition", c.hmcIP, sysUUID)

	// Transparently handle IBM's schema typos for sharing modes
	if req.SharingMode == "share idle procs" {
		req.SharingMode = "sre idle proces"
	} else if req.SharingMode == "share idle procs active" {
		req.SharingMode = "sre idle procs active"
	} else if req.SharingMode == "share idle procs always" {
		req.SharingMode = "sre idle procs always"
	}

	// PRE-PROCESSING: Assign safe defaults for the new fields
	resGroupID := req.ResourceGroupID
	if resGroupID == "" {
		resGroupID = "0" // 0 is the universal HMC ID for the "Default Resource Group"
	}

	maxVirtualSlots := req.MaxVirtualSlots
	if maxVirtualSlots == 0 {
		maxVirtualSlots = 200 // Safe default to allow for high I/O mapping capacity
	}

	var payload string

	if req.DedicatedProc {
		// Dedicated Processor Configuration
		// The HMC demands STRICT ALPHABETICAL ORDERING for elements.
		payload = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<uom:LogicalPartition xmlns:uom="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
		                    schemaVersion="V1_0">
		  <uom:Metadata><uom:Atom/></uom:Metadata>
		  <uom:PartitionIOConfiguration schemaVersion="V1_0">
		      <uom:Metadata><uom:Atom/></uom:Metadata>
		      <uom:MaximumVirtualIOSlots>%d</uom:MaximumVirtualIOSlots>
		  </uom:PartitionIOConfiguration>
		  <uom:PartitionMemoryConfiguration schemaVersion="V1_0">
		      <uom:Metadata><uom:Atom/></uom:Metadata>
		      <uom:DesiredMemory>%d</uom:DesiredMemory>
		      <uom:MaximumMemory>%d</uom:MaximumMemory>
		      <uom:MinimumMemory>%d</uom:MinimumMemory>
		  </uom:PartitionMemoryConfiguration>
		  <uom:PartitionName>%s</uom:PartitionName>
		  <uom:PartitionProcessorConfiguration schemaVersion="V1_0">
		      <uom:Metadata><uom:Atom/></uom:Metadata>
		      <uom:DedicatedProcessorConfiguration schemaVersion="V1_0">
		          <uom:Metadata><uom:Atom/></uom:Metadata>
		          <uom:DesiredProcessors>%.0f</uom:DesiredProcessors>
		          <uom:MaximumProcessors>%.0f</uom:MaximumProcessors>
		          <uom:MinimumProcessors>%.0f</uom:MinimumProcessors>
		      </uom:DedicatedProcessorConfiguration>
		      <uom:HasDedicatedProcessors>true</uom:HasDedicatedProcessors>
		      <uom:SharingMode>%s</uom:SharingMode>
		  </uom:PartitionProcessorConfiguration>
		  <uom:PartitionType>%s</uom:PartitionType>
</uom:LogicalPartition>`,
			maxVirtualSlots,
			req.DesiredMem, req.MaxMem, req.MinMem,
			req.Name,
			req.DesiredProcUnits, req.MaxProcUnits, req.MinProcUnits,
			req.SharingMode,
			req.OsType)
	} else {
		// Shared Processor Configuration

		// CONDITIONAL WEIGHT: Only inject the tag if the mode is actually uncapped
		uncappedWeightXML := ""
		if strings.ToLower(req.SharingMode) == "uncapped" {
			weight := req.UncappedWeight
			if weight == 0 {
				weight = 128 // Standard default weight
			}
			uncappedWeightXML = fmt.Sprintf("\n            <uom:UncappedWeight>%d</uom:UncappedWeight>", weight)
		}

		// The HMC demands STRICT ALPHABETICAL ORDERING for elements.
		payload = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<uom:LogicalPartition xmlns:uom="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
		                    schemaVersion="V1_0">
		  <uom:Metadata><uom:Atom/></uom:Metadata>
		  <uom:PartitionIOConfiguration schemaVersion="V1_0">
		      <uom:Metadata><uom:Atom/></uom:Metadata>
		      <uom:MaximumVirtualIOSlots>%d</uom:MaximumVirtualIOSlots>
		  </uom:PartitionIOConfiguration>
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
		          <uom:MinimumVirtualProcessors>%d</uom:MinimumVirtualProcessors>%s
		      </uom:SharedProcessorConfiguration>
		      <uom:SharingMode>%s</uom:SharingMode>
		  </uom:PartitionProcessorConfiguration>
		  <uom:PartitionType>%s</uom:PartitionType>
</uom:LogicalPartition>`,
			maxVirtualSlots,
			req.DesiredMem, req.MaxMem, req.MinMem,
			req.Name,
			req.DesiredProcUnits, req.DesiredVcpus,
			req.MaxProcUnits, req.MaxVcpus,
			req.MinProcUnits, req.MinVcpus,
			uncappedWeightXML,
			req.SharingMode,
			req.OsType)
	}

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


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("UOM creation failed (%s): %s", resp.Status, string(body))
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


	return &lpar, nil
}

// CreateVirtualSCSIClientAdapter adds a vSCSI Client Adapter and strictly maps it to a VIOS.
func (c *RestClient) CreateVirtualSCSIClientAdapter(lparUUID string, viosID, viosSlot int) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)


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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

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
func (c *RestClient) GetLogicalPartitionDetailed(ctx context.Context, lparUUID string) (*LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(ctxWithTimeout)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}



	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
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


	return &detailedLpar, nil
}

// MapPhysicalIOAdapters dynamically assigns Physical I/O Adapters to an LPAR.
// It rigorously validates ownership against the system inventory.
// Returns a list of successfully attached adapters, a list of skipped adapters, and any errors.
func (c *RestClient) MapPhysicalIOAdapters(ctx context.Context, sysUUID, lparUUID, lparName string, targets []string, inventory *ManagedSystemDetailed) ([]string, []string, error) {
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
			if found {
				break
			}
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
	if err != nil {
		return nil, nil, err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return nil, nil, err
	}
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("GET failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return nil, nil, err
	}
	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil {
		return nil, nil, fmt.Errorf("LogicalPartition element not found")
	}

	ioConfig := lparElem.FindElement(".//*[local-name()='PartitionIOConfiguration']")
	if ioConfig == nil {
		return nil, nil, fmt.Errorf("PartitionIOConfiguration not found")
	}

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
	if err != nil {
		return nil, nil, err
	}
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return nil, nil, err
	}
	defer postResp.Body.Close()

	body, err := io.ReadAll(postResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if postResp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	return attached, skipped, nil
}

// UnmapPhysicalIOAdapters dynamically detaches Physical I/O Adapters from an LPAR.
func (c *RestClient) UnmapPhysicalIOAdapters(ctx context.Context, sysUUID, lparUUID, lparName string, targets []string, inventory *ManagedSystemDetailed) ([]string, []string, error) {
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
				if slot.RelatedIOAdapter.DeviceName != "" {
					actualLocCode = slot.RelatedIOAdapter.DeviceName
				}

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
			if foundAndAttached {
				break
			}
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
	if err != nil {
		return nil, nil, err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return nil, nil, err
	}
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("GET failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return nil, nil, err
	}
	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil {
		return nil, nil, fmt.Errorf("LogicalPartition element not found")
	}

	ioConfig := lparElem.FindElement(".//*[local-name()='PartitionIOConfiguration']")
	if ioConfig == nil {
		return detached, skipped, nil
	}

	profileSlots := ioConfig.FindElement(".//*[local-name()='ProfileIOSlots']")
	if profileSlots == nil {
		return detached, skipped, nil
	}

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
	if err != nil {
		return nil, nil, err
	}
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return nil, nil, err
	}
	defer postResp.Body.Close()

	body, err := io.ReadAll(postResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if postResp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	return detached, skipped, nil
}

// GetAllLogicalPartitionsInHmc retrieves the Go structures for all logical partitions managed by the HMC across all systems.
func (c *RestClient) GetAllLogicalPartitionsInHmc() ([]LogicalPartitionDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition", c.hmcIP)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
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
			continue
		}
		allPartitions = append(allPartitions, detailedLpar)
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
func (c *RestClient) GetLogicalPartitionQuickProperty(lparUUID, propertyName string) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick/%s", c.hmcIP, lparUUID, propertyName)

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


	// Execute the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status %s", resp.Status)
	}

	// The HMC often returns single properties wrapped in JSON quotes (e.g., "running").
	// We trim the whitespace and the quotes to return a clean Go string.
	cleanValue := strings.TrimSpace(string(body))
	cleanValue = strings.Trim(cleanValue, "\"")


	return cleanValue, nil
}

// SearchLogicalPartitions queries the HMC for logical partitions matching a specific property and value.
// Example: SearchLogicalPartitions("PartitionState", "running", true)
func (c *RestClient) SearchLogicalPartitions(propertyName, propertyValue string) ([]*etree.Element, error) {
	// Construct the HMC search string: (Property==Value)
	// Note: If your value contains spaces or special characters (like AIX/Linux),
	// the HMC usually expects single quotes around it (e.g., 'AIX/Linux').
	searchQuery := fmt.Sprintf("(%s==%s)", propertyName, propertyValue)
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/search/%s", c.hmcIP, searchQuery)


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


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	// Strip namespaces for easy querying
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract all LogicalPartition blocks returned by the search
	logicalPartitions := doc.FindElements("//LogicalPartition")
	if len(logicalPartitions) == 0 {
		return []*etree.Element{}, nil
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
func (c *RestClient) ChangeDefaultProfileName(lparUUID, profileName string) (string, error) {
	// Construct the URL for the ChangeDefaultProfileName operation
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/ChangeDefaultProfileName", c.hmcIP, lparUUID)


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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", true)
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


	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}



	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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


	return jobID, nil
}

// CreateVirtualFibreChannelClientAdapter adds a vFC Client Adapter to an LPAR and maps it to a VIOS.
// NOTE: Unlike vSCSI, vFC strictly requires an explicit viosSlot number. It cannot be auto-assigned.
func (c *RestClient) CreateVirtualFibreChannelClientAdapter(lparUUID string, viosID, viosSlot int) (string, error) {

	// FAIL-FAST: IBM Hypervisor strictly requires a target slot for NPIV mapping.
	if viosSlot <= 0 {
		return "", fmt.Errorf("viosSlot must be greater than 0. The HMC does not support auto-assigning remote slots for Virtual Fibre Channel adapters")
	}

	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualFibreChannelClientAdapter", c.hmcIP, lparUUID)


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


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vFC Adapter creation failed (%s): %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("vFC Adapter created successfully, but failed to extract new UUID")
	}


	return atomID.Text(), nil
}

// GetVirtualFibreChannelClientAdapters retrieves all vFC Client Adapters attached to a specific LPAR.
func (c *RestClient) GetVirtualFibreChannelClientAdapters(lparUUID string) ([]VirtualFibreChannelClientAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualFibreChannelClientAdapter", c.hmcIP, lparUUID)


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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// LOG THE RAW RESPONSE

	if resp.StatusCode != http.StatusOK {
		// Handle the case where the LPAR simply has zero vFC adapters attached
		if resp.StatusCode == http.StatusNoContent {
			return []VirtualFibreChannelClientAdapter{}, nil
		}
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
			continue
		}

		var adapter VirtualFibreChannelClientAdapter
		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			continue
		}

		adapters = append(adapters, adapter)
	}


	return adapters, nil
}

// SetPartitionBootString updates the LPAR's PendingBootString (e.g., "cd/dvd-all")
// to force a specific boot device priority on the next power-on.
func (c *RestClient) SetPartitionBootString(ctx context.Context, lparUUID, bootString string) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)


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


	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)

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


	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, err := io.ReadAll(postResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	return nil
}

// GetDedicatedVirtualNICs fetches the detailed configurations of all Dedicated vNICs attached to an LPAR.
func (c *RestClient) GetDedicatedVirtualNICs(ctx context.Context, lparUUID string) ([]VirtualNICDedicated, error) {
	// Query the Dedicated Virtual NIC child collection
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualNICDedicated", c.hmcIP, lparUUID)


	// 1. Fetch and strip namespaces into an etree Document
	doc, err := c.fetchAndParseHMCXML(ctx, url)
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
			continue
		}

		vnics = append(vnics, vnicInfo)
	}


	return vnics, nil
}

// GetSRIOVLogicalPorts fetches all SR-IOV Ethernet Logical Ports provisioned to a specific Logical Partition.
func (c *RestClient) GetSRIOVLogicalPorts(ctx context.Context, lparUUID string) ([]SRIOVLogicalPort, error) {
	// Query the child collection directly for this LPAR
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/SRIOVEthernetLogicalPort", c.hmcIP, lparUUID)


	// 1. Fetch and strip namespaces into an etree Document
	doc, err := c.fetchAndParseHMCXML(ctx, url)
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


	return logicalPorts, nil
}

// CreateSRIOVLogicalPort provisions a new SR-IOV Ethernet Logical Port to a specific Logical Partition.
func (c *RestClient) CreateSRIOVLogicalPort(lparUUID string, adapterID string, physicalPortID string, opts SRIOVPortCreateOptions) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/SRIOVEthernetLogicalPort", c.hmcIP, lparUUID)


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
		return "FAILED", fmt.Errorf("failed to marshal SR-IOV Logical Port request: %v", err)
	}


	// 4. Execute PUT Request
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "FAILED", err
	}

	// Exact Header Matching
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml;type=SRIOVEthernetLogicalPort")
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml, */*")
	req.Header.Set("X-API-Session", c.session)

	resp, err := c.client.Do(req)
	if err != nil {
		return "FAILED", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "FAILED", fmt.Errorf("failed to read response body: %v", err)
	}

	// 5. Evaluate the HMC Response
	if resp.StatusCode >= 400 {
		return "FAILED", fmt.Errorf("failed to create SR-IOV Logical Port (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return "SUCCESS", nil
}

// DeleteSRIOVLogicalPorts intelligently removes one or more SR-IOV Ethernet Logical Ports.
// It accepts a list of targets which can be either the LogicalPortID or the LocationCode.
func (c *RestClient) DeleteSRIOVLogicalPorts(ctx context.Context, lparUUID string, targets []string) error {
	if len(targets) == 0 {
		return nil // Nothing to delete
	}


	// 1. Fetch current ports on the LPAR so we can resolve IDs/Locations to UUIDs
	currentPorts, err := c.GetSRIOVLogicalPorts(ctx, lparUUID)
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
				break
			}
		}
		if !found {
			return fmt.Errorf("target %q not found on this LPAR", cleanTarget)
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

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body for %s: %v", portUUID, err)
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("failed to delete SR-IOV Logical Port %s (HTTP %d): %s", portUUID, resp.StatusCode, string(body))
		}

	}

	return nil
}

// GetRawLparXML fetches the raw XML payload for a Logical Partition (Useful for diffs and backups)
func (c *RestClient) GetRawLparXML(sysUUID, lparUUID string) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)


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
// EnableKvmCapability enables the KVM capability flag on a specific LPAR
// using the pristine GET-Modify-POST pattern required by the HMC.
func (c *RestClient) EnableKvmCapability(ctx context.Context, lparUUID string) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)


	// 1. Pristine GET to fetch the current LPAR XML state
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		return fmt.Errorf("GET failed with status %d: %s", getResp.StatusCode, string(rawXML))
	}

	// 2. Parse the XML Document
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return err
	}

	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil {
		return fmt.Errorf("LogicalPartition element not found in response")
	}

	// 3. Find or Create the KvmCapable element
	kvmTag := lparElem.FindElement(".//*[local-name()='KvmCapable']")
	if kvmTag != nil {
		if kvmTag.Text() == "true" {
			return nil // Already enabled, idempotent exit
		}
		kvmTag.SetText("true")
	} else {
		newTag := lparElem.CreateElement("KvmCapable")
		newTag.CreateAttr("kb", "UOD")
		newTag.CreateAttr("kxe", "false")
		newTag.SetText("true")
	}


	// 4. POST the updated XML back to the HMC
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	if err != nil {
		return err
	}

	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, err := io.ReadAll(postResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Graceful Error Handling
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout/warnings just like other partition updates
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			return nil
		}
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
	}


	return nil
}
