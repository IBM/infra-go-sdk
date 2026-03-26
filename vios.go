package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// / ConfigDevice submits a job request to configure a device on a Virtual I/O Server.
// If devName is empty, it attempts to configure all devices.
// It waits for the job to complete and checks for success.
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
	payload, err := createJobRequestPayload(operation, params, schemaVersion, verbose, includeJobParamSchema)
	if err != nil {
		return fmt.Errorf("failed to create JobRequest payload: %v", err)
	}

	if verbose {
		hmcLogger.Printf("JobRequest XML:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

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

	// Strip namespaces from the response XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// Check for error messages in the response
	errorMsgs := doc.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	// Extract the JobID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if verbose {
		hmcLogger.Printf("ConfigDevice job response: Status=%s, PercentComplete=%d%%",
			jobResp.Status, jobResp.PercentComplete)
	}

	// Check job status
	if jobResp.Status != "COMPLETED_OK" {
		if jobResp.ErrorMessage != "" {
			return fmt.Errorf("job failed: status %s, message: %s", jobResp.Status, jobResp.ErrorMessage)
		}
		return fmt.Errorf("job failed: status %s", jobResp.Status)
	}

	// Check for StdError in results
	if stdError, ok := jobResp.Results["StdError"]; ok && stdError != "" {
		return fmt.Errorf("config device error: %s", stdError)
	}

	return nil
}

// GetVirtualIOServersQuick retrieves the list of Virtual I/O Servers for a given managed system UUID
func (c *HmcRestClient) GetVirtualIOServersQuick(systemUUID string, verbose bool) ([]VIOS, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer/quick/All", c.hmcIP, systemUUID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("GetVirtualIOServersQuick failed with status: %s", resp.Status)
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetVirtualIOServersQuick response body:\n%s", string(body))
	}

	var viosList []VIOS
	if err := json.Unmarshal(body, &viosList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %v", err)
	}

	return viosList, nil
}

// GetFreePhyVolume retrieves free physical volumes for a given VIOS UUID
func (c *HmcRestClient) GetFreePhyVolume(viosUUID string, verbose bool) ([]PhysicalVolume, error) {
	if verbose {
		hmcLogger.Printf("VIOS UUID: %s", viosUUID)
	}
	// Optionally test with FibreChannelBackedOnly
	/* jobParams := map[string]string{
		"FibreChannelBackedOnly": "false",
	} */
	jobParams := map[string]string{}
	// Operation details for the job request
	reqdOperation := map[string]string{
		"OperationName": "GetFreePhysicalVolumes",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}
	// Create the XML payload for the job request
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_3_0", verbose, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Job request payload:\n%s", payload)
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetFreePhysicalVolumes", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Requesting free physical volumes for VIOS UUID %s, URL: %s", viosUUID, url)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	// Enable basic auth to match Postman's Authorization: Basic
	req.SetBasicAuth("", "") // Credentials handled by session token
	if verbose {
		hmcLogger.Printf("Request headers: %+v", req.Header)
	}

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log the response status and body
	if verbose {
		hmcLogger.Printf("GetFreePhyVolume response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if verbose {
		hmcLogger.Printf("GetFreePhyVolume response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse the response to check for specific error messages
		doc, err := xmlStripNamespace(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %v, status: %s, body: %s", err, resp.Status, string(body))
		}
		errorMsgs := doc.FindElements("//Message")
		if len(errorMsgs) > 0 {
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Strip namespaces from the response XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// Check for error messages in the response
	errorMsgs := doc.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return nil, fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	// Extract the JobID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if verbose {
		hmcLogger.Printf("Free Physical Volume job response: Status=%s", jobResp.Status)
	}

	// Extract the result XML from the job response Results map
	pvXML, ok := jobResp.Results["result"]
	if !ok {
		if verbose {
			hmcLogger.Printf("result parameter not found in job response")
		}
		return nil, fmt.Errorf("result not found in job response")
	}
	
	if verbose {
		hmcLogger.Printf("resultElem content: %s", pvXML)
	}

	// Strip namespaces from the physical volumes XML
	strippedDoc, err := xmlStripNamespace([]byte(pvXML))
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from physical volumes XML: %v", err)
	}

	// Serialize the stripped document to bytes
	strippedXML, err := strippedDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize stripped XML: %v", err)
	}

	var pvCollection PhysicalVolumeCollection
	if err := xml.Unmarshal(strippedXML, &pvCollection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal physical volumes XML: %v", err)
	}

	listPv := pvCollection.PhysicalVolumes
	if len(listPv) == 0 {
		if verbose {
			hmcLogger.Printf("No free physical volumes found for VIOS UUID %s", viosUUID)
		}
		// Return an empty list instead of an error, as no volumes is a valid case
		return listPv, nil
	}
	if verbose {
		hmcLogger.Printf("Found %d free physical volumes for VIOS UUID %s", len(listPv), viosUUID)
		for i, pv := range listPv {
			hmcLogger.Printf("Physical Volume %d: Name=%s, VolumeUniqueID=%s, UniqueDeviceID=%s, StorageLabel=%s, Capacity=%d, LocationCode=%s, State=%s", i+1, pv.VolumeName, pv.VolumeUniqueID, pv.UniqueDeviceID, pv.StorageLabel, pv.VolumeCapacity, pv.LocationCode, pv.VolumeState)
		}
	}
	return listPv, nil
}

// RemoveVolumeLPARMapping deletes the LPAR Client Adapter (unmaps the disk from the partition) via the REST API.
// RemoveVolumeLPARMapping removes one or more volume mappings between a VIOS and an LPAR.
// It accepts a slice of volume names and returns a slice of VolumeMappingInfo for each successfully processed volume.
// Each VolumeMappingInfo contains the VTD name and Server Adapter Delete URL for further cleanup orchestration.
func (c *HmcRestClient) RemoveVolumeLPARMapping(viosUUID, lparUUID string, volumeNames []string, verbose bool) ([]VolumeMappingInfo, error) {
	// =====================================================================
	// STEP 1: Find the Client Slot, Server Slot, and VTD from the Mapping
	// =====================================================================
	mappingsURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching VSCSI mappings for VIOS UUID %s, URL: %s", viosUUID, mappingsURL)
	}
	
	mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, verbose)
	if err != nil {
		return nil, err
	}

	if verbose && mappingsDoc != nil {
		docStr, _ := mappingsDoc.WriteToString()
		hmcLogger.Printf("ViosSCSIMapping XML response body:\n%s", docStr)
	}

	targetLparLower := strings.ToLower(lparUUID)
	
	// Create a map to track which volumes we need to find
	volumesToFind := make(map[string]bool)
	for _, vol := range volumeNames {
		volumesToFind[vol] = true
	}
	
	// Store mapping information for each volume
	type mappingDetails struct {
		volumeName       string
		clientSlotNum    string
		serverSlotNum    string
		vtdName          string
	}
	foundMappings := make(map[string]mappingDetails)

	// Scan all mappings once and collect info for requested volumes
	for _, mapping := range mappingsDoc.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assocLpar == nil { continue }

		href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
		
		// Only process mappings for our target LPAR
		if !strings.HasSuffix(href, targetLparLower) {
			continue
		}
		
		backingDevElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='BackingDeviceName']")
		storageVolElem := mapping.FindElement(".//*[local-name()='Storage']/*[local-name()='PhysicalVolume']/*[local-name()='VolumeName']")

		vName := ""
		if backingDevElem != nil && backingDevElem.Text() != "" {
			vName = backingDevElem.Text()
		} else if storageVolElem != nil && storageVolElem.Text() != "" {
			vName = storageVolElem.Text()
		}

		clientSlotElem := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='VirtualSlotNumber']")
		cSlot := ""
		if clientSlotElem != nil { cSlot = clientSlotElem.Text() }

		serverSlotElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='VirtualSlotNumber']")
		sSlot := ""
		if serverSlotElem != nil { sSlot = serverSlotElem.Text() }

		if verbose {
			hmcLogger.Printf("Evaluating Mapping -> LPAR href: %s | VolumeName: %s | ClientSlot: %s | ServerSlot: %s", href, vName, cSlot, sSlot)
		}

		// Check if this volume is in our list to process
		for volName := range volumesToFind {
			isMatch := false
			if vName != "" && vName == volName {
				isMatch = true
			} else if vName == "" && volName == ("EMPTY_VSCSI_SLOT_" + cSlot) {
				isMatch = true
			}

			if isMatch {
				vtdName := ""
				// Extract the Virtual Target Device (VTD) name (e.g., vtscsi0 or vtopt4)
				if targetElem := mapping.FindElement(".//*[local-name()='TargetDevice']//*[local-name()='TargetName']"); targetElem != nil {
					vtdName = targetElem.Text()
				}
				
				foundMappings[volName] = mappingDetails{
					volumeName:    volName,
					clientSlotNum: cSlot,
					serverSlotNum: sSlot,
					vtdName:       vtdName,
				}
				
				if verbose {
					hmcLogger.Printf("--> MATCH FOUND! Volume: %s | Client Slot: %s | Server Slot: %s | VTD: %s", volName, cSlot, sSlot, vtdName)
				}
				break
			}
		}
	}

	if len(foundMappings) == 0 {
		return nil, fmt.Errorf("could not find any mappings for volumes %v on VIOS %s", volumeNames, viosUUID)
	}

	// =====================================================================
	// STEP 2: Resolve the DELETE URLs for Client and Server Adapters
	// =====================================================================
	
	// Build slot-to-URL maps for efficient lookup
	clientSlotToURL := make(map[string]string)
	serverSlotToURL := make(map[string]string)
	
	// Fetch client adapters once
	clientAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)
	clientAdaptersDoc, _ := c.fetchAndParseHMCXML(clientAdaptersURL, verbose)
	if clientAdaptersDoc != nil {
		for _, entry := range clientAdaptersDoc.FindElements("//entry") {
			if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil {
				slotNum := slot.Text()
				for _, link := range entry.FindElements("./link") {
					if link.SelectAttrValue("rel", "") == "SELF" {
						clientSlotToURL[slotNum] = link.SelectAttrValue("href", "")
						break
					}
				}
			}
		}
	}
	
	// Fetch server adapters once
	serverAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
	serverAdaptersDoc, _ := c.fetchAndParseHMCXML(serverAdaptersURL, verbose)
	if serverAdaptersDoc != nil {
		for _, entry := range serverAdaptersDoc.FindElements("//entry") {
			if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil {
				slotNum := slot.Text()
				for _, link := range entry.FindElements("./link") {
					if link.SelectAttrValue("rel", "") == "SELF" {
						serverSlotToURL[slotNum] = link.SelectAttrValue("href", "")
						break
					}
				}
			}
		}
	}

	// =====================================================================
	// STEP 3: Delete Client Adapters and Collect Results
	// =====================================================================
	var results []VolumeMappingInfo
	
	for _, details := range foundMappings {
		clientAdapterDeleteURL := clientSlotToURL[details.clientSlotNum]
		serverAdapterDeleteURL := serverSlotToURL[details.serverSlotNum]
		
		// Delete client adapter
		if clientAdapterDeleteURL != "" {
			if verbose {
				hmcLogger.Printf("Executing DELETE on LPAR Client Adapter for volume %s (Slot %s)...", details.volumeName, details.clientSlotNum)
			}
			reqDelClient, _ := http.NewRequest("DELETE", clientAdapterDeleteURL, nil)
			reqDelClient.Header.Set("X-API-Session", c.session)
			
			respDelClient, err := c.client.Do(reqDelClient)
			if err != nil {
				hmcLogger.Printf("⚠️ Warning: Failed to delete client adapter for volume %s: %v", details.volumeName, err)
				continue
			}
			
			clientBody, _ := io.ReadAll(respDelClient.Body)
			respDelClient.Body.Close()
			
			if verbose {
				hmcLogger.Printf("Client Adapter DELETE Status for %s: %s", details.volumeName, respDelClient.Status)
				if len(clientBody) > 0 {
					hmcLogger.Printf("Client Adapter DELETE Response:\n%s", string(clientBody))
				}
			}

			if respDelClient.StatusCode >= 400 {
				hmcLogger.Printf("⚠️ Warning: Failed to delete client adapter for volume %s. Status: %s", details.volumeName, respDelClient.Status)
				continue
			}
		}
		
		// Add to results
		results = append(results, VolumeMappingInfo{
			VolumeName:             details.volumeName,
			VTDName:                details.vtdName,
			ServerAdapterDeleteURL: serverAdapterDeleteURL,
			ClientSlotNumber:       details.clientSlotNum,
			ServerSlotNumber:       details.serverSlotNum,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to delete any client adapters for volumes %v", volumeNames)
	}

	// Return the details so caller can finish the cleanup sequence
	return results, nil
}


// removeDeviceViaJob removes a specific device from VIOS using the RemoveDevice job operation
func (c *HmcRestClient) removeDeviceViaJob(viosUUID, deviceName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	// devName MUST be the vtscsi mapping name, not the hdisk!
	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", verbose, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice job submission failed with status %s", resp.Status)
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, verbose)
		return err
	}
	return fmt.Errorf("JobID not found")
}
// RemoveVIOSDevice executes the RemoveDevice job on the VIOS to delete a physical volume (e.g., hdisk3)
func (c *HmcRestClient) RemoveVIOSDevice(viosUUID, deviceName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", verbose, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice failed with status %s: %s", resp.Status, string(body))
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		if verbose { hmcLogger.Printf("Removing device %s from VIOS (Job ID: %s)...", deviceName, jobIDElem.Text()) }
		// Wait for the VIOS to finish deleting the disk
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, verbose)
		return err
	}
	
	return fmt.Errorf("JobID not found in RemoveDevice response")
}




// =====================================================================
// HELPER FUNCTIONS
// =====================================================================

// getElementText safely returns the text of an XML element if it exists, otherwise an empty string.
func getElementText(root *etree.Element, path string) string {
	if root == nil {
		return ""
	}
	elem := root.FindElement(path)
	if elem != nil {
		return elem.Text()
	}
	return ""
}

// =====================================================================
// VIOS METHODS
// =====================================================================

/// GetVirtualIOServers retrieves detailed, comprehensive information for all Virtual I/O Servers of a managed system.
func (c *HmcRestClient) GetVirtualIOServers(systemUUID string, verbose bool) ([]VirtualIOServerDetails, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching comprehensive VIOS details for system UUID %s, URL: %s", systemUUID, url)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetVirtualIOServers HTTP response status: %s", resp.Status)
	}
	if verbose {
		hmcLogger.Printf("GetVirtualIOServers HTTP response status: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip XML namespaces to make path searching easier with etree
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var viosList []VirtualIOServerDetails

	// Find all VirtualIOServer elements in the stripped XML
	entries := doc.FindElements("//VirtualIOServer")
	if verbose {
		hmcLogger.Printf("Found %d potential VirtualIOServer XML elements", len(entries))
	}

	for _, entry := range entries {
		// Ensure it is a primary VIOS element by checking for PartitionID
		partitionID := getElementText(entry, "PartitionID")
		if partitionID == "" {
			continue
		}

		partitionName := getElementText(entry, "PartitionName")
		if verbose {
			hmcLogger.Printf("\n--- Processing VIOS: %s (Partition ID: %s) ---", partitionName, partitionID)
		}

		vios := VirtualIOServerDetails{
			UUID:                        getElementText(entry, "PartitionUUID"),
			PartitionID:                 partitionID,
			PartitionName:               partitionName,
			PartitionState:              getElementText(entry, "PartitionState"),
			PartitionType:               getElementText(entry, "PartitionType"),
			SystemName:                  getElementText(entry, "SystemName"),
			OperatingSystemVersion:      getElementText(entry, "OperatingSystemVersion"),
			ResourceMonitoringIPAddress: getElementText(entry, "ResourceMonitoringIPAddress"),
			LogicalSerialNumber:         getElementText(entry, "LogicalSerialNumber"),
			IsBootable:                  getElementText(entry, "IsBootable"),
			Uptime:                      getElementText(entry, "Uptime"),
		}

		// 1. Parse Memory Configuration
		memConfig := entry.FindElement("PartitionMemoryConfiguration")
		if memConfig != nil {
			if verbose {
				hmcLogger.Printf("  [%s] Parsing Memory Configuration", partitionName)
			}
			vios.Memory = VIOSMemoryConfig{
				DesiredMemory: getElementText(memConfig, "DesiredMemory"),
				MaximumMemory: getElementText(memConfig, "MaximumMemory"),
				MinimumMemory: getElementText(memConfig, "MinimumMemory"),
			}
		}

		// 2. Parse Processor Configuration
		procConfig := entry.FindElement("PartitionProcessorConfiguration")
		if procConfig != nil {
			if verbose {
				hmcLogger.Printf("  [%s] Parsing Processor Configuration", partitionName)
			}
			vios.Processor = VIOSProcessorConfig{
				HasDedicatedProcessors: getElementText(procConfig, "HasDedicatedProcessors"),
				SharingMode:            getElementText(procConfig, "SharingMode"),
			}

			if dedProc := procConfig.FindElement("DedicatedProcessorConfiguration"); dedProc != nil {
				vios.Processor.DesiredProcessors = getElementText(dedProc, "DesiredProcessors")
				vios.Processor.MaximumProcessors = getElementText(dedProc, "MaximumProcessors")
				vios.Processor.MinimumProcessors = getElementText(dedProc, "MinimumProcessors")
			}
		}

		// 3. Parse Storage Configuration (PVs, VFC Mappings, and Physical FC Ports)
		pvs := entry.FindElements("PhysicalVolumes/PhysicalVolume")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Physical Volumes", partitionName, len(pvs))
		}
		for _, pv := range pvs {
			vios.Storage.PhysicalVolumes = append(vios.Storage.PhysicalVolumes, VIOSPhysicalVolume{
				VolumeName:     getElementText(pv, "VolumeName"),
				VolumeCapacity: getElementText(pv, "VolumeCapacity"),
				VolumeState:    getElementText(pv, "VolumeState"),
				UniqueDeviceID: getElementText(pv, "UniqueDeviceID"),
				LocationCode:   getElementText(pv, "LocationCode"),
			})
		}
		
		vfcs := entry.FindElements("VirtualFibreChannelMappings/VirtualFibreChannelMapping")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Virtual Fibre Channel Mappings", partitionName, len(vfcs))
		}
		for _, vfc := range vfcs {
			vios.Storage.VFCMappings = append(vios.Storage.VFCMappings, VIOSVFCMapping{
				ServerAdapterSlot: getElementText(vfc, "ServerAdapter/VirtualSlotNumber"),
				ClientPartitionID: getElementText(vfc, "ServerAdapter/ConnectingPartitionID"),
				ClientAdapterSlot: getElementText(vfc, "ServerAdapter/ConnectingVirtualSlotNumber"),
				MapPort:           getElementText(vfc, "ServerAdapter/MapPort"),
				PortWWPN:          getElementText(vfc, "Port/WWPN"),
				PortWWNN:          getElementText(vfc, "Port/WWNN"),
			})
		}

		// Use a deep search to find all PhysicalFibreChannelPort nodes regardless of exact PCI slot nesting
		fcPorts := entry.FindElements(".//PhysicalFibreChannelPort")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Physical Fibre Channel Ports", partitionName, len(fcPorts))
		}
		for _, fcPort := range fcPorts {
			portName := getElementText(fcPort, "PortName")
			wwpn := getElementText(fcPort, "WWPN")
			
			if verbose {
				hmcLogger.Printf("    -> Extracted Port: %-5s | WWPN: %s", portName, wwpn)
			}
			
			vios.Storage.FibreChannelPorts = append(vios.Storage.FibreChannelPorts, VIOSFibreChannelPort{
				PortName:     portName,
				LocationCode: getElementText(fcPort, "LocationCode"),
				WWPN:         wwpn,
				WWNN:         getElementText(fcPort, "WWNN"),
			})
		}

		// 4. Parse Network Configuration (Shared Ethernet Adapters and Trunks)
		seas := entry.FindElements("SharedEthernetAdapters/SharedEthernetAdapter")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Shared Ethernet Adapters", partitionName, len(seas))
		}
		for _, sea := range seas {
			vios.Network.SharedEthernetAdapters = append(vios.Network.SharedEthernetAdapters, VIOSSharedEthernetAdapter{
				DeviceName:         getElementText(sea, "DeviceName"),
				HighAvailability:   getElementText(sea, "HighAvailabilityMode"),
				PortVLANID:         getElementText(sea, "PortVLANID"),
				BackingDevice:      getElementText(sea, "BackingDeviceChoice/EthernetBackingDevice/DeviceName"),
				ConfigurationState: getElementText(sea, "ConfigurationState"),
			})
		}
		
		trunks := entry.FindElements("TrunkAdapters/TrunkAdapter")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Trunk Adapters", partitionName, len(trunks))
		}
		for _, trunk := range trunks {
			vios.Network.TrunkAdapters = append(vios.Network.TrunkAdapters, VIOSTrunkAdapter{
				DeviceName:        getElementText(trunk, "DeviceName"),
				MACAddress:        getElementText(trunk, "MACAddress"),
				PortVLANID:        getElementText(trunk, "PortVLANID"),
				VirtualSlotNumber: getElementText(trunk, "VirtualSlotNumber"),
			})
		}

		viosList = append(viosList, vios)
	}

	if verbose {
		hmcLogger.Printf("Successfully parsed %d valid Virtual I/O Servers with complete details", len(viosList))
	}

	return viosList, nil
}
// GetVirtualIOServer retrieves detailed information for a specific Virtual I/O Server using its UUID.
func (c *HmcRestClient) GetVirtualIOServer(viosUUID string, verbose bool) (*VirtualIOServerDetails, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching VIOS details for UUID %s, URL: %s", viosUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip XML namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// When querying a single item, the root or an entry contains the VirtualIOServer node
	entry := doc.FindElement("//VirtualIOServer")
	if entry == nil {
		return nil, fmt.Errorf("could not find VirtualIOServer in response")
	}

	vios := &VirtualIOServerDetails{
		UUID:                        getElementText(entry, "PartitionUUID"),
		PartitionID:                 getElementText(entry, "PartitionID"),
		PartitionName:               getElementText(entry, "PartitionName"),
		PartitionState:              getElementText(entry, "PartitionState"),
		PartitionType:               getElementText(entry, "PartitionType"),
		SystemName:                  getElementText(entry, "SystemName"),
		OperatingSystemVersion:      getElementText(entry, "OperatingSystemVersion"),
		ResourceMonitoringIPAddress: getElementText(entry, "ResourceMonitoringIPAddress"),
		LogicalSerialNumber:         getElementText(entry, "LogicalSerialNumber"),
		IsBootable:                  getElementText(entry, "IsBootable"),
		Uptime:                      getElementText(entry, "Uptime"),
	}

	// 1. Parse Memory Configuration
	if memConfig := entry.FindElement("PartitionMemoryConfiguration"); memConfig != nil {
		vios.Memory = VIOSMemoryConfig{
			DesiredMemory: getElementText(memConfig, "DesiredMemory"),
			MaximumMemory: getElementText(memConfig, "MaximumMemory"),
			MinimumMemory: getElementText(memConfig, "MinimumMemory"),
		}
	}

	// 2. Parse Processor Configuration
	if procConfig := entry.FindElement("PartitionProcessorConfiguration"); procConfig != nil {
		vios.Processor = VIOSProcessorConfig{
			HasDedicatedProcessors: getElementText(procConfig, "HasDedicatedProcessors"),
			SharingMode:            getElementText(procConfig, "SharingMode"),
		}
		if dedProc := procConfig.FindElement("DedicatedProcessorConfiguration"); dedProc != nil {
			vios.Processor.DesiredProcessors = getElementText(dedProc, "DesiredProcessors")
			vios.Processor.MaximumProcessors = getElementText(dedProc, "MaximumProcessors")
			vios.Processor.MinimumProcessors = getElementText(dedProc, "MinimumProcessors")
		}
	}

	// 3. Parse Storage Configuration
	for _, pv := range entry.FindElements("PhysicalVolumes/PhysicalVolume") {
		vios.Storage.PhysicalVolumes = append(vios.Storage.PhysicalVolumes, VIOSPhysicalVolume{
			VolumeName:     getElementText(pv, "VolumeName"),
			VolumeCapacity: getElementText(pv, "VolumeCapacity"),
			VolumeState:    getElementText(pv, "VolumeState"),
			UniqueDeviceID: getElementText(pv, "UniqueDeviceID"),
			LocationCode:   getElementText(pv, "LocationCode"),
		})
	}
	for _, vfc := range entry.FindElements("VirtualFibreChannelMappings/VirtualFibreChannelMapping") {
		vios.Storage.VFCMappings = append(vios.Storage.VFCMappings, VIOSVFCMapping{
			ServerAdapterSlot: getElementText(vfc, "ServerAdapter/VirtualSlotNumber"),
			ClientPartitionID: getElementText(vfc, "ServerAdapter/ConnectingPartitionID"),
			ClientAdapterSlot: getElementText(vfc, "ServerAdapter/ConnectingVirtualSlotNumber"),
			MapPort:           getElementText(vfc, "ServerAdapter/MapPort"),
			PortWWPN:          getElementText(vfc, "Port/WWPN"),
			PortWWNN:          getElementText(vfc, "Port/WWNN"),
		})
	}
	for _, fcPort := range entry.FindElements(".//PhysicalFibreChannelPort") {
		vios.Storage.FibreChannelPorts = append(vios.Storage.FibreChannelPorts, VIOSFibreChannelPort{
			PortName:     getElementText(fcPort, "PortName"),
			LocationCode: getElementText(fcPort, "LocationCode"),
			WWPN:         getElementText(fcPort, "WWPN"),
			WWNN:         getElementText(fcPort, "WWNN"),
		})
	}

	// 4. Parse Network Configuration
	for _, sea := range entry.FindElements("SharedEthernetAdapters/SharedEthernetAdapter") {
		vios.Network.SharedEthernetAdapters = append(vios.Network.SharedEthernetAdapters, VIOSSharedEthernetAdapter{
			DeviceName:         getElementText(sea, "DeviceName"),
			HighAvailability:   getElementText(sea, "HighAvailabilityMode"),
			PortVLANID:         getElementText(sea, "PortVLANID"),
			BackingDevice:      getElementText(sea, "BackingDeviceChoice/EthernetBackingDevice/DeviceName"),
			ConfigurationState: getElementText(sea, "ConfigurationState"),
		})
	}
	for _, trunk := range entry.FindElements("TrunkAdapters/TrunkAdapter") {
		vios.Network.TrunkAdapters = append(vios.Network.TrunkAdapters, VIOSTrunkAdapter{
			DeviceName:        getElementText(trunk, "DeviceName"),
			MACAddress:        getElementText(trunk, "MACAddress"),
			PortVLANID:        getElementText(trunk, "PortVLANID"),
			VirtualSlotNumber: getElementText(trunk, "VirtualSlotNumber"),
		})
	}

	return vios, nil
}

// GetVirtualSCSIServerAdapters retrieves a list of all Virtual SCSI Server Adapters (vhost) configured on a specific VIOS.
func (c *HmcRestClient) GetVirtualSCSIServerAdapters(viosUUID string, verbose bool) ([]VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching Virtual SCSI Server Adapters for VIOS UUID %s, URL: %s", viosUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	if verbose {
		hmcLogger.Printf("GetVirtualSCSIServerAdapters HTTP response status: %s", resp.Status)
		hmcLogger.Printf("GetVirtualSCSIServerAdapters response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip XML namespaces to make path searching easier with etree
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var adapters []VirtualSCSIServerAdapter

	// Iterate through each Atom entry
	entries := doc.FindElements("//entry")
	for _, entry := range entries {
		// Find the core adapter payload within the entry
		adapterElem := entry.FindElement(".//VirtualSCSIServerAdapter")
		if adapterElem == nil {
			continue
		}

		adapter := VirtualSCSIServerAdapter{
			UUID:                                getElementText(adapterElem, "Metadata/Atom/AtomID"),
			AdapterType:                         getElementText(adapterElem, "AdapterType"),
			DynamicReconfigurationConnectorName: getElementText(adapterElem, "DynamicReconfigurationConnectorName"),
			LocationCode:                        getElementText(adapterElem, "LocationCode"),
			LocalPartitionID:                    getElementText(adapterElem, "LocalPartitionID"),
			RequiredAdapter:                     getElementText(adapterElem, "RequiredAdapter"),
			VariedOn:                            getElementText(adapterElem, "VariedOn"),
			VirtualSlotNumber:                   getElementText(adapterElem, "VirtualSlotNumber"),
			RemoteLogicalPartitionID:            getElementText(adapterElem, "RemoteLogicalPartitionID"),
			RemoteSlotNumber:                    getElementText(adapterElem, "RemoteSlotNumber"),
		}

		// Extract the direct URI for this specific adapter from the Atom <link rel="SELF">
		for _, link := range entry.FindElements("./link") {
			if link.SelectAttrValue("rel", "") == "SELF" {
				adapter.AdapterURI = link.SelectAttrValue("href", "")
				break
			}
		}

		adapters = append(adapters, adapter)
	}

	if verbose {
		hmcLogger.Printf("Successfully parsed %d Virtual SCSI Server Adapters", len(adapters))
	}

	return adapters, nil
}

// GetVirtualSCSIServerAdapter retrieves the details of a specific Virtual SCSI Server Adapter (vhost) using its UUID.
func (c *HmcRestClient) GetVirtualSCSIServerAdapter(viosUUID, adapterUUID string, verbose bool) (*VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching specific Virtual SCSI Server Adapter, URL: %s", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	if verbose {
		hmcLogger.Printf("GetVirtualSCSIServerAdapter Status: %s", resp.Status)
		hmcLogger.Printf("GetVirtualSCSIServerAdapter Body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	adapterElem := doc.FindElement("//VirtualSCSIServerAdapter")
	if adapterElem == nil {
		return nil, fmt.Errorf("VirtualSCSIServerAdapter node not found in response")
	}

	adapter := &VirtualSCSIServerAdapter{
		UUID:                                getElementText(adapterElem, "Metadata/Atom/AtomID"),
		AdapterType:                         getElementText(adapterElem, "AdapterType"),
		DynamicReconfigurationConnectorName: getElementText(adapterElem, "DynamicReconfigurationConnectorName"),
		LocationCode:                        getElementText(adapterElem, "LocationCode"),
		LocalPartitionID:                    getElementText(adapterElem, "LocalPartitionID"),
		RequiredAdapter:                     getElementText(adapterElem, "RequiredAdapter"),
		VariedOn:                            getElementText(adapterElem, "VariedOn"),
		VirtualSlotNumber:                   getElementText(adapterElem, "VirtualSlotNumber"),
		RemoteLogicalPartitionID:            getElementText(adapterElem, "RemoteLogicalPartitionID"),
		RemoteSlotNumber:                    getElementText(adapterElem, "RemoteSlotNumber"),
		AdapterURI:                          url, // We already know the direct URL
	}

	return adapter, nil
}

// DeleteVirtualSCSIServerAdapter removes a specific Virtual SCSI Server Adapter (vhost) from a VIOS.
func (c *HmcRestClient) DeleteVirtualSCSIServerAdapter(viosUUID, adapterUUID string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if verbose {
		hmcLogger.Printf("Deleting Virtual SCSI Server Adapter, URL: %s", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	
	// Read the body even if it's empty to ensure the connection is freed properly
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if verbose {
		hmcLogger.Printf("DeleteVirtualSCSIServerAdapter Status: %s", resp.Status)
		if len(body) > 0 {
			hmcLogger.Printf("DeleteVirtualSCSIServerAdapter Body: %s", string(body))
		}
	}

	// Accept both 200 OK and 204 No Content as successful deletions
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete VirtualSCSIServerAdapter. Status: %s, Response: %s", resp.Status, string(body))
	}

	return nil
}

// GetViosSCSIMappings retrieves and fully parses all VSCSI mappings for a specific VIOS.
func (c *HmcRestClient) GetViosSCSIMappings(viosUUID string, verbose bool) ([]ViosSCSIMappingDetails, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if verbose {
		hmcLogger.Printf("Fetching VSCSI Mappings for VIOS UUID %s, URL: %s", viosUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var mappingsList []ViosSCSIMappingDetails

	mappings := doc.FindElements("//VirtualSCSIMapping")
	for _, mapping := range mappings {
		detail := ViosSCSIMappingDetails{}

		// 1. Associated LPAR
		if assocLpar := mapping.FindElement("AssociatedLogicalPartition"); assocLpar != nil {
			detail.AssociatedLparURI = assocLpar.SelectAttrValue("href", "")
		}

		// 2. Client Adapter
		if client := mapping.FindElement("ClientAdapter"); client != nil {
			detail.ClientAdapter = VSCSIClientAdapter{
				AdapterType:                         getElementText(client, "AdapterType"),
				DynamicReconfigurationConnectorName: getElementText(client, "DynamicReconfigurationConnectorName"),
				LocationCode:                        getElementText(client, "LocationCode"),
				LocalPartitionID:                    getElementText(client, "LocalPartitionID"),
				RequiredAdapter:                     getElementText(client, "RequiredAdapter"),
				VariedOn:                            getElementText(client, "VariedOn"),
				VirtualSlotNumber:                   getElementText(client, "VirtualSlotNumber"),
				RemoteLogicalPartitionID:            getElementText(client, "RemoteLogicalPartitionID"),
				RemoteSlotNumber:                    getElementText(client, "RemoteSlotNumber"),
				ServerLocationCode:                  getElementText(client, "ServerLocationCode"),
			}
		}

		// 3. Server Adapter
		if server := mapping.FindElement("ServerAdapter"); server != nil {
			detail.ServerAdapter = VSCSIServerAdapter{
				AdapterType:                         getElementText(server, "AdapterType"),
				DynamicReconfigurationConnectorName: getElementText(server, "DynamicReconfigurationConnectorName"),
				LocationCode:                        getElementText(server, "LocationCode"),
				LocalPartitionID:                    getElementText(server, "LocalPartitionID"),
				RequiredAdapter:                     getElementText(server, "RequiredAdapter"),
				VariedOn:                            getElementText(server, "VariedOn"),
				VirtualSlotNumber:                   getElementText(server, "VirtualSlotNumber"),
				AdapterName:                         getElementText(server, "AdapterName"),
				BackingDeviceName:                   getElementText(server, "BackingDeviceName"),
				RemoteLogicalPartitionID:            getElementText(server, "RemoteLogicalPartitionID"),
				RemoteSlotNumber:                    getElementText(server, "RemoteSlotNumber"),
				ServerLocationCode:                  getElementText(server, "ServerLocationCode"),
				UniqueDeviceID:                      getElementText(server, "UniqueDeviceID"),
			}
		}

		// 4. Storage Details
		if storage := mapping.FindElement("Storage"); storage != nil {
			if pv := storage.FindElement("PhysicalVolume"); pv != nil {
				detail.Storage.StorageType = "PhysicalVolume"
				detail.Storage.Description = getElementText(pv, "Description")
				detail.Storage.LocationCode = getElementText(pv, "LocationCode")
				detail.Storage.PersistentReserveKeyValue = getElementText(pv, "PersistentReserveKeyValue")
				detail.Storage.ReservePolicy = getElementText(pv, "ReservePolicy")
				detail.Storage.ReservePolicyAlgorithm = getElementText(pv, "ReservePolicyAlgorithm")
				detail.Storage.UniqueDeviceID = getElementText(pv, "UniqueDeviceID")
				detail.Storage.AvailableForUsage = getElementText(pv, "AvailableForUsage")
				detail.Storage.VolumeCapacity = getElementText(pv, "VolumeCapacity")
				detail.Storage.VolumeName = getElementText(pv, "VolumeName")
				detail.Storage.VolumeState = getElementText(pv, "VolumeState")
				detail.Storage.VolumeUniqueID = getElementText(pv, "VolumeUniqueID")
				detail.Storage.IsFibreChannelBacked = getElementText(pv, "IsFibreChannelBacked")
				detail.Storage.IsISCSIBacked = getElementText(pv, "IsISCSIBacked")
				detail.Storage.StorageLabel = getElementText(pv, "StorageLabel")
				detail.Storage.DescriptorPage83 = getElementText(pv, "DescriptorPage83")
			} else if vd := storage.FindElement("VirtualDisk"); vd != nil {
				detail.Storage.StorageType = "VirtualDisk"
				detail.Storage.VolumeName = getElementText(vd, "DiskName")
			} else if opt := storage.FindElement("VirtualOpticalMedia"); opt != nil {
				detail.Storage.StorageType = "VirtualOpticalMedia"
				detail.Storage.MediaName = getElementText(opt, "MediaName")
				detail.Storage.MediaUDID = getElementText(opt, "MediaUDID")
				detail.Storage.MountType = getElementText(opt, "MountType")
				detail.Storage.Size = getElementText(opt, "Size")
			}
		}

		// 5. Target Device
		if target := mapping.FindElement("TargetDevice"); target != nil {
			if vOpt := target.FindElement("VirtualOpticalTargetDevice"); vOpt != nil {
				detail.TargetDevice.DeviceType = "VirtualOpticalTargetDevice"
				detail.TargetDevice.LogicalUnitAddress = getElementText(vOpt, "LogicalUnitAddress")
				detail.TargetDevice.TargetName = getElementText(vOpt, "TargetName")
				detail.TargetDevice.UniqueDeviceID = getElementText(vOpt, "UniqueDeviceID")
			} else if lvVtd := target.FindElement("LogicalVolumeVirtualTargetDevice"); lvVtd != nil {
				detail.TargetDevice.DeviceType = "LogicalVolumeVirtualTargetDevice"
				detail.TargetDevice.LogicalUnitAddress = getElementText(lvVtd, "LogicalUnitAddress")
				detail.TargetDevice.TargetName = getElementText(lvVtd, "TargetName")
				detail.TargetDevice.UniqueDeviceID = getElementText(lvVtd, "UniqueDeviceID")
			} else if pVtd := target.FindElement("PhysicalVolumeVirtualTargetDevice"); pVtd != nil {
				detail.TargetDevice.DeviceType = "PhysicalVolumeVirtualTargetDevice"
				detail.TargetDevice.LogicalUnitAddress = getElementText(pVtd, "LogicalUnitAddress")
				detail.TargetDevice.TargetName = getElementText(pVtd, "TargetName")
				detail.TargetDevice.UniqueDeviceID = getElementText(pVtd, "UniqueDeviceID")
			}
		}

		mappingsList = append(mappingsList, detail)
	}

	if verbose {
		hmcLogger.Printf("Successfully fully parsed %d VSCSI Mappings", len(mappingsList))
	}

	return mappingsList, nil
}

// =====================================================================
// VOLUME GROUP API METHODS
// =====================================================================

// GetVolumeGroups retrieves a list of all Volume Groups configured on a specific VIOS.
func (c *HmcRestClient) GetVolumeGroups(viosUUID string, verbose bool) ([]VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

    if verbose {
        hmcLogger.Printf("Fetching Volume Groups for VIOS UUID %s, URL: %s", viosUUID, url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/atom+xml")

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
        return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    var volumeGroups []VolumeGroup

    entries := doc.FindElements("//entry")
    for _, entry := range entries {
        vgElem := entry.FindElement(".//VolumeGroup")
        if vgElem == nil {
            continue
        }

        vg := VolumeGroup{
            UUID:                  getElementText(vgElem, "Metadata/Atom/AtomID"),
            GroupName:             getElementText(vgElem, "GroupName"),
            AvailableSize:         getElementText(vgElem, "AvailableSize"),
            FreeSpace:             getElementText(vgElem, "FreeSpace"),
            GroupCapacity:         getElementText(vgElem, "GroupCapacity"),
            GroupSerialID:         getElementText(vgElem, "GroupSerialID"),
            MaximumLogicalVolumes: getElementText(vgElem, "MaximumLogicalVolumes"),
            UniqueDeviceID:        getElementText(vgElem, "UniqueDeviceID"),
            HasMediaRepository:    vgElem.FindElement(".//VirtualMediaRepository") != nil,
            MediaRepositoryName:   getElementText(vgElem, ".//VirtualMediaRepository/RepositoryName"), // NEW
            MediaRepositorySize:   getElementText(vgElem, ".//VirtualMediaRepository/RepositorySize"), // NEW
        }

	// Extract Physical Volumes with enhanced metadata
	for _, pvElem := range vgElem.FindElements(".//PhysicalVolumes/PhysicalVolume") {
		vg.PhysicalVolumes = append(vg.PhysicalVolumes, VGPhysicalVolume{
			VolumeName:             getElementText(pvElem, "VolumeName"),
			VolumeCapacity:       getElementText(pvElem, "VolumeCapacity"),
			VolumeState:          getElementText(pvElem, "VolumeState"),
			UniqueDeviceID:       getElementText(pvElem, "UniqueDeviceID"),
			VolumeUniqueID:       getElementText(pvElem, "VolumeUniqueID"),
			LocationCode:         getElementText(pvElem, "LocationCode"),
			Description:          getElementText(pvElem, "Description"),
			IsFibreChannelBacked: getElementText(pvElem, "IsFibreChannelBacked"),
			ReservePolicy:          getElementText(pvElem, "ReservePolicy"),
			ReservePolicyAlgorithm: getElementText(pvElem, "ReservePolicyAlgorithm"),
			AvailableForUsage:      getElementText(pvElem, "AvailableForUsage"),
			IsISCSIBacked:          getElementText(pvElem, "IsISCSIBacked"),
			StorageLabel:           getElementText(pvElem, "StorageLabel"),
			DescriptorPage83:       getElementText(pvElem, "DescriptorPage83"),
		})
	}

        // Extract Virtual Optical Media
        for _, optElem := range vgElem.FindElements(".//VirtualOpticalMedia") {
            vg.OpticalMedia = append(vg.OpticalMedia, VirtualOpticalMedia{
                MediaName: getElementText(optElem, "MediaName"),
                MediaUDID: getElementText(optElem, "MediaUDID"),
                MountType: getElementText(optElem, "MountType"),
                Size:      getElementText(optElem, "Size"),
            })
        }

        // NEW: Extract Virtual Disks (Logical Volumes)
        for _, vdElem := range vgElem.FindElements(".//VirtualDisks/VirtualDisk") {
            vg.VirtualDisks = append(vg.VirtualDisks, VirtualDisk{
                DiskName:       getElementText(vdElem, "DiskName"),
                DiskCapacity:   getElementText(vdElem, "DiskCapacity"),
                DiskLabel:      getElementText(vdElem, "DiskLabel"),
                UniqueDeviceID: getElementText(vdElem, "UniqueDeviceID"),
            })
        }

        volumeGroups = append(volumeGroups, vg)
    }

    return volumeGroups, nil
}

// GetVolumeGroup retrieves the details of a specific Volume Group on a Virtual I/O Server.
func (c *HmcRestClient) GetVolumeGroup(viosUUID, vgUUID string, verbose bool) (*VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup/%s", c.hmcIP, viosUUID, vgUUID)

    if verbose {
        hmcLogger.Printf("Fetching Volume Group %s for VIOS %s, URL: %s", vgUUID, viosUUID, url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/atom+xml")

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
        return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    vgElem := doc.FindElement("//VolumeGroup")
    if vgElem == nil {
        return nil, fmt.Errorf("VolumeGroup node not found in response")
    }

    vg := &VolumeGroup{
            UUID:                  getElementText(vgElem, "Metadata/Atom/AtomID"),
            GroupName:             getElementText(vgElem, "GroupName"),
            AvailableSize:         getElementText(vgElem, "AvailableSize"),
            FreeSpace:             getElementText(vgElem, "FreeSpace"),
            GroupCapacity:         getElementText(vgElem, "GroupCapacity"),
            GroupSerialID:         getElementText(vgElem, "GroupSerialID"),
            MaximumLogicalVolumes: getElementText(vgElem, "MaximumLogicalVolumes"),
            UniqueDeviceID:        getElementText(vgElem, "UniqueDeviceID"),
            HasMediaRepository:    vgElem.FindElement(".//VirtualMediaRepository") != nil,
            MediaRepositoryName:   getElementText(vgElem, ".//VirtualMediaRepository/RepositoryName"), // NEW
            MediaRepositorySize:   getElementText(vgElem, ".//VirtualMediaRepository/RepositorySize"), // NEW
        }

	// Extract Physical Volumes with enhanced metadata
	for _, pvElem := range vgElem.FindElements(".//PhysicalVolumes/PhysicalVolume") {
		vg.PhysicalVolumes = append(vg.PhysicalVolumes, VGPhysicalVolume{
			VolumeName:             getElementText(pvElem, "VolumeName"),
			VolumeCapacity:       getElementText(pvElem, "VolumeCapacity"),
			VolumeState:          getElementText(pvElem, "VolumeState"),
			UniqueDeviceID:       getElementText(pvElem, "UniqueDeviceID"),
			VolumeUniqueID:       getElementText(pvElem, "VolumeUniqueID"),
			LocationCode:         getElementText(pvElem, "LocationCode"),
			Description:          getElementText(pvElem, "Description"),
			IsFibreChannelBacked: getElementText(pvElem, "IsFibreChannelBacked"),
			ReservePolicy:          getElementText(pvElem, "ReservePolicy"),
			ReservePolicyAlgorithm: getElementText(pvElem, "ReservePolicyAlgorithm"),
			AvailableForUsage:      getElementText(pvElem, "AvailableForUsage"),
			IsISCSIBacked:          getElementText(pvElem, "IsISCSIBacked"),
			StorageLabel:           getElementText(pvElem, "StorageLabel"),
			DescriptorPage83:       getElementText(pvElem, "DescriptorPage83"),
		})
	}

    // Extract Virtual Optical Media
    for _, optElem := range vgElem.FindElements(".//VirtualOpticalMedia") {
        vg.OpticalMedia = append(vg.OpticalMedia, VirtualOpticalMedia{
            MediaName: getElementText(optElem, "MediaName"),
            MediaUDID: getElementText(optElem, "MediaUDID"),
            MountType: getElementText(optElem, "MountType"),
            Size:      getElementText(optElem, "Size"),
        })
    }

    // Extract Virtual Disks (Logical Volumes)
    for _, vdElem := range vgElem.FindElements(".//VirtualDisks/VirtualDisk") {
        vg.VirtualDisks = append(vg.VirtualDisks, VirtualDisk{
            DiskName:       getElementText(vdElem, "DiskName"),
            DiskCapacity:   getElementText(vdElem, "DiskCapacity"),
            DiskLabel:      getElementText(vdElem, "DiskLabel"),
            UniqueDeviceID: getElementText(vdElem, "UniqueDeviceID"),
        })
    }

    return vg, nil
}
// =====================================================================
// CREATE VOLUME GROUP
// =====================================================================

// CreateVolumeGroup creates a new Volume Group on a VIOS using the specified physical volumes.
func (c *HmcRestClient) CreateVolumeGroup(viosUUID, vgName string, physicalVolumes []string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

	if verbose {
		hmcLogger.Printf("Creating Volume Group '%s' on VIOS %s with disks: %v", vgName, viosUUID, physicalVolumes)
	}

	// 1. Build the PhysicalVolumes XML blocks
	var pvBuilder strings.Builder
	for _, pv := range physicalVolumes {
		pvBuilder.WriteString(fmt.Sprintf(`
			<PhysicalVolume schemaVersion="V1_0">
				<VolumeName kxe="false" kb="CUR">%s</VolumeName>
			</PhysicalVolume>`, strings.TrimSpace(pv)))
	}

	// 2. Build the full VolumeGroup XML Payload
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VolumeGroup:VolumeGroup xmlns:VolumeGroup="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" schemaVersion="V1_0">
	<GroupName kxe="false" kb="CUR">%s</GroupName>
	<PhysicalVolumes kxe="false" kb="CUD" schemaVersion="V1_0">%s
	</PhysicalVolumes>
</VolumeGroup:VolumeGroup>`, vgName, pvBuilder.String())

	if verbose {
		hmcLogger.Printf("CreateVolumeGroup Payload:\n%s", payload)
	}

	// 3. Create the HTTP Request
	// Note: While standard REST uses POST for creation, some HMC versions expect PUT here. 
	// If you get a 405 Method Not Allowed, switch this to "POST".
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VolumeGroup")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// 4. Execute the Request
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
		hmcLogger.Printf("CreateVolumeGroup HTTP response status: %s", resp.Status)
	}

	// 5. Check for Success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("CreateVolumeGroup failed with status %s: %s", resp.Status, string(body))
	}

	// 6. Monitor the Job (If the HMC kicked off an asynchronous job)
	doc, err := xmlStripNamespace(body)
	if err == nil {
		jobIDElem := doc.FindElement("//JobID")
		if jobIDElem != nil {
			jobID := jobIDElem.Text()
			if verbose {
				hmcLogger.Printf("CreateVolumeGroup Job submitted (Job ID: %s), waiting for completion...", jobID)
			}
			_, err = c.FetchJobStatus(jobID, false, 10, verbose)
			if err != nil {
				return fmt.Errorf("CreateVolumeGroup job failed: %v", err)
			}
			if verbose {
				hmcLogger.Printf("✅ CreateVolumeGroup job completed successfully.")
			}
		}
	}

	return nil
}

// =====================================================================
// CREATE VIRTUAL DISK (LOGICAL VOLUME) - SMART CLI METHOD
// =====================================================================

// CreateVirtualDisk safely creates a Logical Volume (Virtual Disk) inside a standard Volume Group.
// It verifies the host Volume Group has enough free space and checks for naming collisions before executing.
func (c *HmcRestClient) CreateVirtualDisk(sysName, viosUUID, viosName, vgName, diskName string, capacityMB int, verbose bool) error {
	requiredGB := float64(capacityMB) / 1024.0

	if verbose {
		hmcLogger.Printf("Pre-flight check: Verifying capacity and naming for new Virtual Disk '%s' (%d MB) on VIOS '%s'...", diskName, capacityMB, viosName)
	}

	// 1. Fetch Volume Groups to verify capacity and check for naming collisions
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var foundVgFreeSpace string
	var foundVgName string

	for _, vg := range vgList {
		// Collision Check: Ensure no disk with this name already exists on this VIOS
		for _, vd := range vg.VirtualDisks {
			if strings.EqualFold(vd.DiskName, diskName) {
				return fmt.Errorf("ABORT: A Virtual Disk named '%s' already exists in VG '%s'", diskName, vg.GroupName)
			}
		}

		// Locate our target Volume Group to check its capacity
		if strings.EqualFold(vg.GroupName, vgName) {
			foundVgName = vg.GroupName
			foundVgFreeSpace = vg.FreeSpace
			// We don't break here because we still want to finish scanning the other VGs for naming collisions
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("ABORT: Target Volume Group '%s' was not found on VIOS '%s'", vgName, viosName)
	}

	// Capacity Check
	freeSpaceGB, parseErr := strconv.ParseFloat(foundVgFreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", foundVgName, parseErr)
	}

	if verbose {
		hmcLogger.Printf("-> VG '%s' has %.2f GB available. Required: %.2f GB", foundVgName, freeSpaceGB, requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, capacityMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if verbose {
		hmcLogger.Printf("✅ Pre-flight checks passed. Executing creation via CLI...")
	}

	// Syntax: mklv -lv <diskName> <vgName> <Size>M
	mklvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mklv -lv %s %s %dM"`, sysName, viosName, diskName, vgName, capacityMB)

	output, err := c.CliRunner(mklvCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to create Virtual Disk via mklv: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Virtual Disk '%s' created successfully. VIOS returned: %s", diskName, strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE VIRTUAL DISK (LOGICAL VOLUME) - CLI METHOD
// =====================================================================

// DeleteVirtualDisk safely removes a Logical Volume (Virtual Disk) from a VIOS.
// It uses the native VIOS rmlv command with the -f flag to bypass confirmation prompts.
func (c *HmcRestClient) DeleteVirtualDisk(sysName, viosName, diskName string, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Safely deleting Virtual Disk '%s' on VIOS '%s' via CLI...", diskName, viosName)
	}

	// Syntax: rmlv -f <diskName>
	// The -f flag is required for automation so the OS does not wait for user input.
	rmlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmlv -f %s"`, sysName, viosName, diskName)
	
	if verbose {
		hmcLogger.Printf("Executing: %s", rmlvCmd)
	}

	output, err := c.CliRunner(rmlvCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to delete Virtual Disk via rmlv: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Virtual Disk '%s' deleted successfully. VIOS returned: %s", diskName, strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// EXTEND VIRTUAL DISK (LOGICAL VOLUME) - SMART CLI METHOD
// =====================================================================

// ExtendVirtualDisk safely increases the size of an existing Logical Volume (Virtual Disk).
// It automatically queries the HMC to verify the host Volume Group has enough free space before executing.
func (c *HmcRestClient) ExtendVirtualDisk(sysName, viosUUID, viosName, diskName string, additionalMB int, verbose bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if verbose {
		hmcLogger.Printf("Pre-flight check: Verifying capacity for Virtual Disk '%s' on VIOS '%s'...", diskName, viosName)
	}

	// 1. Fetch Volume Groups to find the disk and check capacity
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for capacity check: %v", err)
	}

	var foundVgName string
	for _, vg := range vgList {
		for _, vd := range vg.VirtualDisks {
			if strings.EqualFold(vd.DiskName, diskName) {
				foundVgName = vg.GroupName
				
				freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
				if parseErr != nil {
					return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", vg.GroupName, parseErr)
				}

				if verbose {
					hmcLogger.Printf("-> Found disk '%s' inside VG '%s'. Available space: %.2f GB", diskName, vg.GroupName, freeSpaceGB)
				}

				// Check if there is enough space to satisfy the request
				if freeSpaceGB < requiredGB {
					return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, additionalMB, vg.GroupName, freeSpaceGB)
				}
				break
			}
		}
		if foundVgName != "" {
			break
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("Virtual Disk '%s' was not found on VIOS '%s'", diskName, viosName)
	}

	// 2. Execute the extension via CLI
	if verbose {
		hmcLogger.Printf("✅ Capacity check passed. Executing extension via CLI...")
	}

	extendlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "extendlv %s %dM"`, sysName, viosName, diskName, additionalMB)
	
	output, err := c.CliRunner(extendlvCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to extend Virtual Disk via extendlv: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Virtual Disk '%s' extended successfully. VIOS returned: %s", diskName, strings.TrimSpace(output))
	}

	return nil
}
// =====================================================================
// CREATE VIRTUAL OPTICAL MEDIA - SMART CLI METHOD
// =====================================================================

// CreateVirtualOpticalMedia creates a Virtual Optical Media (ISO) in the VIOS Media Repository.
// If sourceFile is provided, it imports an existing ISO. 
// If nfsLink is true (only valid with sourceFile), it links to the file instead of copying it.
// If readOnly is true, the media is created with the -ro flag to prevent accidental overwrites.
func (c *HmcRestClient) CreateVirtualOpticalMedia(sysName, viosUUID, viosName, mediaName, sourceFile string, sizeMB int, readOnly, nfsLink, verbose bool) error {
	if nfsLink && sourceFile == "" {
		return fmt.Errorf("ABORT: The -nfslink flag can only be used when providing a sourceFile")
	}

	if verbose {
		hmcLogger.Printf("Pre-flight check: Verifying naming for Virtual Optical Media '%s' on VIOS '%s'...", mediaName, viosName)
	}

	// 1. Fetch Volume Groups to check for naming collisions in the Media Repository
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	for _, vg := range vgList {
		for _, opt := range vg.OpticalMedia {
			if strings.EqualFold(opt.MediaName, mediaName) {
				return fmt.Errorf("ABORT: A Virtual Optical Media named '%s' already exists in the repository on VIOS '%s'", mediaName, viosName)
			}
		}
	}

	// 2. Build the appropriate mkvopt command
	roFlag := ""
	if readOnly {
		roFlag = " -ro"
	}

	var mkvoptCmd string
	if sourceFile != "" {
		nfsFlag := ""
		if nfsLink {
			nfsFlag = " -nfslink"
		}
		
		if verbose {
			hmcLogger.Printf("✅ Pre-flight passed. Importing ISO from file: %s (ReadOnly: %v, NFSLink: %v)", sourceFile, readOnly, nfsLink)
		}
		// Syntax: mkvopt -name <mediaName> -file <SourceFile> [-nfslink] [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -file %s%s%s"`, sysName, viosName, mediaName, sourceFile, nfsFlag, roFlag)
	} else {
		if verbose {
			hmcLogger.Printf("✅ Pre-flight passed. Creating blank media of %d MB... (ReadOnly: %v)", sizeMB, readOnly)
		}
		// Syntax: mkvopt -name <mediaName> -size <Size>M [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -size %dM%s"`, sysName, viosName, mediaName, sizeMB, roFlag)
	}

	// 3. Execute the creation/import via CLI
	output, err := c.CliRunner(mkvoptCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to create/import Virtual Optical Media: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Virtual Optical Media '%s' created successfully. VIOS returned: %s", mediaName, strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// CREATE MEDIA REPOSITORY - SMART CLI METHOD
// =====================================================================

// CreateMediaRepository safely creates the Virtual Media Repository on a VIOS.
// It verifies that no repository currently exists on the VIOS, and that the target VG has enough space.
func (c *HmcRestClient) CreateMediaRepository(sysName, viosUUID, viosName, vgName string, sizeMB int, verbose bool) error {
	requiredGB := float64(sizeMB) / 1024.0

	if verbose {
		hmcLogger.Printf("Pre-flight check: Verifying capacity and existing repositories for VG '%s' on VIOS '%s'...", vgName, viosName)
	}

	// 1. Fetch Volume Groups to check for existing repositories and verify capacity
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var foundVgFreeSpace string
	var foundVgName string

	for _, vg := range vgList {
		// COLLISION CHECK: A VIOS can only have one repository globally.
		if vg.HasMediaRepository {
			return fmt.Errorf("ABORT: A Virtual Media Repository already exists on this VIOS (hosted in VG '%s')", vg.GroupName)
		}

		// Locate our target Volume Group to check its capacity
		if strings.EqualFold(vg.GroupName, vgName) {
			foundVgName = vg.GroupName
			foundVgFreeSpace = vg.FreeSpace
			// We do not break here because we must finish scanning the other VGs to ensure no repository exists elsewhere!
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("ABORT: Target Volume Group '%s' was not found on VIOS '%s'", vgName, viosName)
	}

	// Capacity Check
	freeSpaceGB, parseErr := strconv.ParseFloat(foundVgFreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", foundVgName, parseErr)
	}

	if verbose {
		hmcLogger.Printf("-> VG '%s' has %.2f GB available. Required: %.2f GB", foundVgName, freeSpaceGB, requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, sizeMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if verbose {
		hmcLogger.Printf("✅ Pre-flight checks passed. Executing mkrep via CLI...")
	}

	// Syntax: mkrep -sp <vgName> -size <sizeMB>M
	mkrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkrep -sp %s -size %dM"`, sysName, viosName, vgName, sizeMB)

	output, err := c.CliRunner(mkrepCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to create Media Repository via mkrep: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Media Repository created successfully. VIOS returned: %s", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE MEDIA REPOSITORY - SMART CLI METHOD (ENHANCED)
// =====================================================================

// DeleteMediaRepository removes the Virtual Media Repository from a VIOS.
// It verifies the repo exists, checks for media if force is false, and warns if force is true.
func (c *HmcRestClient) DeleteMediaRepository(sysName, viosUUID, viosName, repoName string, force, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Pre-flight check: Looking for Media Repository '%s' on VIOS '%s'...", repoName, viosName)
	}

	// 1. Fetch Volume Groups to find the existing repository
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var targetVG *VolumeGroup
	for i := range vgList {
		if vgList[i].HasMediaRepository {
			targetVG = &vgList[i]
			break
		}
	}

	// CHECK 1: Does the repository even exist?
	if targetVG == nil {
		return fmt.Errorf("ABORT: No Virtual Media Repository found on VIOS '%s'", viosName)
	}

	// CHECK 2: Does the name match?
	if !strings.EqualFold(targetVG.MediaRepositoryName, repoName) {
		return fmt.Errorf("ABORT: Found repository '%s', but you requested to delete '%s'", targetVG.MediaRepositoryName, repoName)
	}

	// CHECK 3: Safety check for existing media
	mediaCount := len(targetVG.OpticalMedia)
	if force {
		// Print the specific warning requested
		fmt.Printf("\n⚠️  WARNING: Force flag is ENABLED. Deleting repository '%s' will also PERMANENTLY DELETE %d ISO file(s) inside it.\n", repoName, mediaCount)
	} else if mediaCount > 0 {
		// Fail WITHOUT calling rmrep if media exists and force is false
		return fmt.Errorf("ABORT: Repository '%s' contains %d ISO file(s). Use the 'force' flag to delete anyway", repoName, mediaCount)
	}

	// 2. Execute the deletion via CLI
	if verbose {
		hmcLogger.Printf("✅ Pre-flight passed. Executing rmrep on VIOS...")
	}

	forceFlag := ""
	if force {
		forceFlag = " -f"
	}

	rmrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmrep%s"`, sysName, viosName, forceFlag)
	
	output, err := c.CliRunner(rmrepCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to delete Media Repository via CLI: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Media Repository '%s' deleted successfully.", repoName)
	}

	return nil
}
// =====================================================================
// CHANGE MEDIA REPOSITORY (EXTEND) - SMART CLI METHOD
// =====================================================================

// ChangeMediaRepository increases the size of the Virtual Media Repository.
// additionalMB is the amount of NEW space to add (incremental).
// It identifies the hosting Volume Group and verifies free space before executing.
func (c *HmcRestClient) ChangeMediaRepository(sysName, viosUUID, viosName string, additionalMB int, verbose bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if verbose {
		hmcLogger.Printf("Pre-flight check: Verifying VG capacity for repository expansion on VIOS '%s'...", viosName)
	}

	// 1. Fetch Volume Groups to find the repository's location
	vgList, err := c.GetVolumeGroups(viosUUID, verbose)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var hostingVG *VolumeGroup
	for i := range vgList {
		if vgList[i].HasMediaRepository {
			hostingVG = &vgList[i]
			break
		}
	}

	// CHECK 1: Does the repository even exist?
	if hostingVG == nil {
		return fmt.Errorf("ABORT: No Virtual Media Repository found on VIOS '%s'. Use CreateMediaRepository first", viosName)
	}

	// CHECK 2: Does the hosting VG have enough space?
	freeSpaceGB, parseErr := strconv.ParseFloat(hostingVG.FreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", hostingVG.GroupName, parseErr)
	}

	if verbose {
		hmcLogger.Printf("-> Repository found in VG '%s'. Available: %.2f GB | Requested: %.2f GB", 
			hostingVG.GroupName, freeSpaceGB, requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: VG '%s' only has %.2f GB free, cannot add %.2f GB", 
			hostingVG.GroupName, freeSpaceGB, requiredGB)
	}

	// 2. Execute the expansion via CLI
	if verbose {
		hmcLogger.Printf("✅ Pre-flight passed. Extending repository '%s' by %d MB...", 
			hostingVG.MediaRepositoryName, additionalMB)
	}

	// Syntax: chrep -size <Size>M
	chrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "chrep -size %dM"`, sysName, viosName, additionalMB)
	
	output, err := c.CliRunner(chrepCmd, verbose)
	if err != nil {
		return fmt.Errorf("failed to extend Media Repository via chrep: %v\nOutput: %s", err, output)
	}

	if verbose {
		hmcLogger.Printf("✅ Media Repository expanded successfully. VIOS returned: %s", strings.TrimSpace(output))
	}

	return nil
}

// CreatePhysicalVolumeMap maps one or more physical disks on the VIOS to a target LPAR using the GET-Modify-POST pattern.
// Supports batch operations - pass multiple disk names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreatePhysicalVolumeMap(sysUUID, viosUUID, lparUUID string, diskNames []string, verbose bool) (string, error) {
	if len(diskNames) == 0 {
		return "", fmt.Errorf("at least one disk name is required")
	}
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

	// 4. Construct and add mappings for each disk (Schema-compliant)
	for _, diskName := range diskNames {
		newMappingXML := fmt.Sprintf(`
	       <VirtualSCSIMapping schemaVersion="V1_0">
	           <AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
	           <Storage>
	               <PhysicalVolume schemaVersion="V1_0">
	                   <VolumeName>%s</VolumeName>
	               </PhysicalVolume>
	           </Storage>
	       </VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, diskName)

		if verbose {
			hmcLogger.Printf("Creating mapping XML for disk '%s':\n%s", diskName, newMappingXML)
		}

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", diskName, err)
		}

		// 5. Append the new mapping safely to the list
		mappingsList.AddChild(newMappingDoc.Root())
		
		if verbose {
			hmcLogger.Printf("Added mapping for disk: %s", diskName)
		}
	}

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
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", xmlStr)
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
// CreateVirtualDiskMap maps one or more Logical Volumes (Virtual Disks) on the VIOS to a target LPAR using a pristine GET-Modify-POST.
// Supports batch operations - pass multiple disk names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, verbose bool) (string, error) {
	if len(diskNames) == 0 {
		return "", fmt.Errorf("at least one disk name is required")
	}
	// 1. Raw GET - We DO NOT use fetchAndParseHMCXML because we MUST preserve all namespaces and kxe/kb attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching pristine VIOS %s XML for Virtual Disk mapping...", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		return "", fmt.Errorf("GET failed: %s", string(rawXML))
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract ONLY the VirtualIOServer element from the <entry> wrapper
	// We use local-name() so etree finds it regardless of namespace prefixes
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Find or create the VirtualSCSIMappings list
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// 5. Construct and add mappings for each virtual disk exactly as the schema requires
	for _, diskName := range diskNames {
		newMappingXML := fmt.Sprintf(`
	       <VirtualSCSIMapping schemaVersion="V1_0">
	           <AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
	           <Storage>
	               <VirtualDisk schemaVersion="V1_0">
	                   <DiskName>%s</DiskName>
	               </VirtualDisk>
	           </Storage>
	       </VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, diskName)

	 if verbose {
	  hmcLogger.Printf("Creating mapping XML for virtual disk '%s':\n%s", diskName, newMappingXML)
	 }

	 newMappingDoc := etree.NewDocument()
	 if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
	  return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", diskName, err)
	 }

	 // 6. Inject the new mapping
	 mappingsList.AddChild(newMappingDoc.Root())
	 
	 if verbose {
	  hmcLogger.Printf("Added mapping for virtual disk: %s", diskName)
	 }
	}

	// 7. Extract the VIOS document to POST
	// Because viosElem was cloned from pristine XML, it retains all original namespaces and attributes
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	if verbose {
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", postXML)
		hmcLogger.Printf("POSTing pristine modified XML back to HMC...")
	}

	// 8. Execute POST
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// 9. Graceful error handling
	// We still catch the HSCL2957 warning in case our TARGET LPAR is powered off!
	/* if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "HSCL2957") {
			if verbose {
				hmcLogger.Printf("⚠️ WARNING: Mapping saved, but target DLPAR dynamic injection failed (Expected if your target LPAR is powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
	} */

	// 10. Fetch Job Status
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose { hmcLogger.Printf("Update job triggered: %s", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}

// AddVirtualOpticalMedia natively imports an ISO file into the VIOS Media Repository using the AddOpticalMedia job.
// Note: This requires HMC V10.3.1061.0 or later.
func (c *HmcRestClient) AddVirtualOpticalMedia(viosUUID, mediaName, fileName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/AddOpticalMedia", c.hmcIP, viosUUID)
	
	if verbose {
		hmcLogger.Printf("Natively adding Virtual Optical Media '%s' from file '%s' to VIOS %s...", mediaName, fileName, viosUUID)
	}

	// 1. Define operation details for the JobRequest
	operation := map[string]string{
		"OperationName": "AddOpticalMedia",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	// 2. Build job parameters matching the HMC schema you provided
	params := map[string]string{
		"MediaName": mediaName,
		"FileName":  fileName,
	}

	// 3. Generate the XML payload using your existing helper
	payload, err := createJobRequestPayload(operation, params, "V1_0", verbose, true)
	if err != nil {
		return fmt.Errorf("failed to create job request payload: %v", err)
	}

	// 4. Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
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
		hmcLogger.Printf("AddOpticalMedia Response Status: %s", resp.Status)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("AddOpticalMedia job failed with status %s: %s", resp.Status, string(body))
	}

	// 6. Strip namespaces to find the JobID
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
		hmcLogger.Printf("Extracted JobID: %s. Waiting for ISO import to complete...", jobID)
	}

	// 7. Wait for the background job to finish
	_, err = c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return fmt.Errorf("failed during AddOpticalMedia job execution: %v", err)
	}

	return nil
}
// CreateVirtualOpticalMap creates one or more Virtual Optical Devices (CD-ROM) on the target LPAR and loads the specified ISO media.
// Supports batch operations - pass multiple media names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreateVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, verbose bool) (string, error) {
	if len(mediaNames) == 0 {
		return "", fmt.Errorf("at least one media name is required")
	}
	// 1. Raw GET - Fetch pristine VIOS XML to preserve namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching pristine VIOS %s XML for Virtual Optical mapping...", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		return "", fmt.Errorf("GET failed: %s", string(rawXML))
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract ONLY the VirtualIOServer element from the <entry> wrapper
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Find or create the VirtualSCSIMappings list
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// 5. Construct and add mappings for each optical media
	for _, mediaName := range mediaNames {
		newMappingXML := fmt.Sprintf(`
	       <VirtualSCSIMapping schemaVersion="V1_0">
	           <AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
	           <Storage>
	               <VirtualOpticalMedia schemaVersion="V1_0">
	                   <MediaName>%s</MediaName>
	               </VirtualOpticalMedia>
	           </Storage>
	       </VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, mediaName)

	 if verbose {
	  hmcLogger.Printf("Creating mapping XML for optical media '%s':\n%s", mediaName, newMappingXML)
	 }

	 newMappingDoc := etree.NewDocument()
	 if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
	  return "", fmt.Errorf("failed to parse mapping XML for media %s: %v", mediaName, err)
	 }

	 // 6. Inject the new mapping
	 mappingsList.AddChild(newMappingDoc.Root())
	 
	 if verbose {
	  hmcLogger.Printf("Added mapping for optical media: %s", mediaName)
	 }
	}

	// 7. Extract the VIOS document to POST
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	if verbose {
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", postXML)
		hmcLogger.Printf("POSTing pristine modified XML back to HMC...")
	}

	// 8. Execute POST
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)


	// 10. Fetch Job Status
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose { hmcLogger.Printf("Optical mapping job triggered: %s", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}
// DeletePhysicalVolumeMaps removes multiple physical disk mappings from a VIOS to a target LPAR in a single transaction.
// Note: Strict HTTP error checking is disabled (Option 2 behavior).
func (c *HmcRestClient) DeletePhysicalVolumeMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, verbose bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if verbose {
		hmcLogger.Printf("Fetching VIOS %s to locate and remove storage mappings...", viosUUID)
	}

	doc, err := c.fetchAndParseHMCXML(url, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VIOS mappings: %v", err)
	}

	// 2. EXTRACT the actual VirtualIOServer element
	viosElem := doc.FindElement("//VirtualIOServer")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in VIOS XML")
	}

	// 3. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//VirtualSCSIMappings")
	if mappingsList == nil {
		return "", fmt.Errorf("no VirtualSCSIMappings collection found on VIOS %s", viosUUID)
	}

	// Create a fast lookup map for the disks we want to delete
	targetDisks := make(map[string]bool)
	for _, d := range diskNames {
		targetDisks[d] = true
	}

	// 4. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements("VirtualSCSIMapping") {
		// Check if it belongs to our target LPAR
		lparRef := mapping.FindElement("AssociatedLogicalPartition")
		if lparRef == nil {
			continue
		}
		href := lparRef.SelectAttrValue("href", "")
		if !strings.Contains(href, lparUUID) {
			continue // Not our LPAR
		}

		// Check if the mapped volume is one of our target disks
		volNameElem := mapping.FindElement("Storage/PhysicalVolume/VolumeName")
		if volNameElem == nil {
			continue
		}
		
		if targetDisks[volNameElem.Text()] {
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if verbose {
			hmcLogger.Printf("No mappings found for the specified disks to LPAR '%s'. Nothing to delete.", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if verbose {
		hmcLogger.Printf("Found %d mapping(s) to remove. Removing from XML tree...", len(mappingsToRemove))
	}
	for _, mapping := range mappingsToRemove {
		mappingsList.RemoveChild(mapping)
	}

	// 6. Natively set the correct namespace and tag on the root element
	viosElem.Tag = "VirtualIOServer:VirtualIOServer"
	viosElem.CreateAttr("xmlns:VirtualIOServer", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns:ns2", "http://www.w3.org/XML/1998/namespace/k2")

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
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", xmlStr)
		hmcLogger.Printf("POSTing updated VIOS XML back to HMC to apply bulk deletion...")
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

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose {
				hmcLogger.Printf("Deletion job triggered: %s", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}
// DeleteVirtualDiskMaps removes multiple virtual disk mappings from a VIOS to an LPAR in a single operation.
// This function follows the GET-Modify-POST pattern to delete virtual disk (logical volume) mappings.
//
// Parameters:
//   - sysUUID: The UUID of the managed system
//   - viosUUID: The UUID of the Virtual I/O Server
//   - lparUUID: The UUID of the logical partition
//   - diskNames: Slice of virtual disk names to unmap (e.g., ["lv01", "lv02"])
//   - verbose: Enable detailed logging
//
// Returns:
//   - "SUCCESS" if mappings were deleted
//   - "NOT_FOUND" if no matching mappings exist (idempotent)
//   - error if operation fails
//
// Note: Strict HTTP error checking is disabled (Option 2 behavior).
func (c *HmcRestClient) DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, verbose bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if verbose {
		hmcLogger.Printf("Fetching VIOS %s to locate and remove virtual disk mappings for disks: %v", viosUUID, diskNames)
	}

	doc, err := c.fetchAndParseHMCXML(url, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VIOS mappings: %v", err)
	}

	// 2. EXTRACT the actual VirtualIOServer element
	viosElem := doc.FindElement("//VirtualIOServer")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in VIOS XML")
	}

	// 3. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//VirtualSCSIMappings")
	if mappingsList == nil {
		return "", fmt.Errorf("no VirtualSCSIMappings collection found on VIOS %s", viosUUID)
	}

	// Create a fast lookup map for the disks we want to delete
	targetDisks := make(map[string]bool)
	for _, d := range diskNames {
		targetDisks[d] = true
	}

	// 4. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements("VirtualSCSIMapping") {
		// Check if it belongs to our target LPAR
		lparRef := mapping.FindElement("AssociatedLogicalPartition")
		if lparRef == nil {
			continue
		}
		href := lparRef.SelectAttrValue("href", "")
		if !strings.Contains(href, lparUUID) {
			continue // Not our LPAR
		}

		// Check if the mapped volume is one of our target virtual disks
		diskNameElem := mapping.FindElement("Storage/VirtualDisk/DiskName")
		if diskNameElem == nil {
			continue
		}
		
		if targetDisks[diskNameElem.Text()] {
			if verbose {
				hmcLogger.Printf("Found mapping for virtual disk '%s' to LPAR %s", diskNameElem.Text(), lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if verbose {
			hmcLogger.Printf("No virtual disk mappings found for the specified disks to LPAR '%s'. Nothing to delete.", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if verbose {
		hmcLogger.Printf("Found %d virtual disk mapping(s) to remove. Removing from XML tree...", len(mappingsToRemove))
	}
	for i, mapping := range mappingsToRemove {
		diskNameElem := mapping.FindElement("Storage/VirtualDisk/DiskName")
		if diskNameElem != nil && verbose {
			hmcLogger.Printf("Removing mapping %d/%d: virtual disk '%s'", i+1, len(mappingsToRemove), diskNameElem.Text())
		}
		mappingsList.RemoveChild(mapping)
	}

	// 6. Natively set the correct namespace and tag on the root element
	viosElem.Tag = "VirtualIOServer:VirtualIOServer"
	viosElem.CreateAttr("xmlns:VirtualIOServer", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns:ns2", "http://www.w3.org/XML/1998/namespace/k2")

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
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", xmlStr)
		hmcLogger.Printf("POSTing updated VIOS XML back to HMC to apply virtual disk deletion...")
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

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose {
				hmcLogger.Printf("Virtual disk deletion job triggered: %s", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}

// DeleteVirtualOpticalMaps removes multiple virtual optical media mappings from a VIOS to an LPAR in a single operation.
// This function follows the GET-Modify-POST pattern to delete virtual optical (ISO/CD-ROM) mappings.
//
// Parameters:
//   - sysUUID: The UUID of the managed system
//   - viosUUID: The UUID of the Virtual I/O Server
//   - lparUUID: The UUID of the logical partition
//   - mediaNames: Slice of virtual optical media names to unmap (e.g., ["rhel9.iso", "aix73.iso"])
//   - verbose: Enable detailed logging
//
// Returns:
//   - "SUCCESS" if mappings were deleted
//   - "NOT_FOUND" if no matching mappings exist (idempotent)
//   - error if operation fails
//
// Note: Strict HTTP error checking is disabled (Option 2 behavior).
func (c *HmcRestClient) DeleteVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, verbose bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if verbose {
		hmcLogger.Printf("Fetching VIOS %s to locate and remove virtual optical mappings for media: %v", viosUUID, mediaNames)
	}

	doc, err := c.fetchAndParseHMCXML(url, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VIOS mappings: %v", err)
	}

	// 2. EXTRACT the actual VirtualIOServer element
	viosElem := doc.FindElement("//VirtualIOServer")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in VIOS XML")
	}

	// 3. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//VirtualSCSIMappings")
	if mappingsList == nil {
		return "", fmt.Errorf("no VirtualSCSIMappings collection found on VIOS %s", viosUUID)
	}

	// Create a fast lookup map for the media we want to delete
	targetMedia := make(map[string]bool)
	for _, m := range mediaNames {
		targetMedia[m] = true
	}

	// 4. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements("VirtualSCSIMapping") {
		// Check if it belongs to our target LPAR
		lparRef := mapping.FindElement("AssociatedLogicalPartition")
		if lparRef == nil {
			continue
		}
		href := lparRef.SelectAttrValue("href", "")
		if !strings.Contains(href, lparUUID) {
			continue // Not our LPAR
		}

		// Check if the mapped media is one of our target virtual optical media
		mediaNameElem := mapping.FindElement("Storage/VirtualOpticalMedia/MediaName")
		if mediaNameElem == nil {
			continue
		}
		
		if targetMedia[mediaNameElem.Text()] {
			if verbose {
				hmcLogger.Printf("Found mapping for optical media '%s' to LPAR %s", mediaNameElem.Text(), lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if verbose {
			hmcLogger.Printf("No virtual optical mappings found for the specified media to LPAR '%s'. Nothing to delete.", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if verbose {
		hmcLogger.Printf("Found %d virtual optical mapping(s) to remove. Removing from XML tree...", len(mappingsToRemove))
	}
	for i, mapping := range mappingsToRemove {
		mediaNameElem := mapping.FindElement("Storage/VirtualOpticalMedia/MediaName")
		if mediaNameElem != nil && verbose {
			hmcLogger.Printf("Removing mapping %d/%d: optical media '%s'", i+1, len(mappingsToRemove), mediaNameElem.Text())
		}
		mappingsList.RemoveChild(mapping)
	}

	// 6. Natively set the correct namespace and tag on the root element
	viosElem.Tag = "VirtualIOServer:VirtualIOServer"
	viosElem.CreateAttr("xmlns:VirtualIOServer", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns", "http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/")
	viosElem.CreateAttr("xmlns:ns2", "http://www.w3.org/XML/1998/namespace/k2")

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
		hmcLogger.Printf("Complete XML payload to be POSTed:\n%s", xmlStr)
		hmcLogger.Printf("POSTing updated VIOS XML back to HMC to apply virtual optical deletion...")
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

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if verbose {
				hmcLogger.Printf("Virtual optical deletion job triggered: %s", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, verbose)
		}
	}

	return "SUCCESS", nil
}
