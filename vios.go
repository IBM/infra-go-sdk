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
func (c *HmcRestClient) ConfigDevice(viosID string, devName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/ConfigDevice", c.hmcIP, viosID)
	if debug {
		c.Logger.Debug("Submitting ConfigDevice job", "viosID", viosID, "url", url)
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
	payload, err := createJobRequestPayload(operation, params, schemaVersion, debug, includeJobParamSchema)
	if err != nil {
		return fmt.Errorf("failed to create JobRequest payload: %v", err)
	}

	if debug {
		c.Logger.Debug("JobRequest XML generated", "payload", payload)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// Log response status if debug
	if debug {
		c.Logger.Debug("ConfigDevice response status", "status", resp.Status)
		c.Logger.Debug("ConfigDevice response body", "body", string(body))
	}

	// Check for non-success status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		c.Logger.Error("ConfigDevice job submission failed", "status", resp.Status)
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
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(jobID, false, 10, debug)
	if err != nil {
		return fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if debug {
		c.Logger.Info("ConfigDevice job response", "status", jobResp.Status)
	}

	// Check job status
	if jobResp.Status != "COMPLETED_OK" {
		return fmt.Errorf("job failed: status %s", jobResp.Status)
	}

	// Check for StdError in results
	var stdError string
	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "StdError" {
			stdError = param.ParameterValue
			break
		}
	}
	if stdError != "" {
		return fmt.Errorf("config device error: %s", stdError)
	}

	return nil
}

// GetVirtualIOServersQuick retrieves the exhaustive quick list of Virtual I/O Servers for a given managed system UUID.
func (c *HmcRestClient) GetVirtualIOServersQuick(systemUUID string, debug bool) ([]VIOSQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer/quick/All", c.hmcIP, systemUUID)
	
	if debug {
		c.Logger.Debug("Fetching quick VIOS list", "systemUUID", systemUUID)
	}

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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if debug {
			c.Logger.Warn("GetVirtualIOServersQuick failed", "status", resp.Status)
		}
		// Sometimes the HMC returns 204 No Content if there are literally zero VIOSes on the system.
		// Handling that cleanly prevents an unmarshal error on an empty body.
		if resp.StatusCode == http.StatusNoContent {
			return []VIOSQuick{}, nil
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualIOServersQuick response", "bodyLengthBytes", len(body))
	}

	var viosList []VIOSQuick
	if err := json.Unmarshal(body, &viosList); err != nil {
		c.Logger.Error("Failed to unmarshal JSON response", "error", err)
		return nil, fmt.Errorf("failed to unmarshal JSON response: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed VIOS entries from quick feed", "count", len(viosList))
	}

	return viosList, nil
}

// GetFreePhyVolume retrieves free physical volumes for a given VIOS UUID
func (c *HmcRestClient) GetFreePhyVolume(viosUUID string, debug bool) ([]PhysicalVolume, error) {
	if debug {
		c.Logger.Debug("Fetching free physical volumes", "viosUUID", viosUUID)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_3_0", debug, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Job request payload generated", "payload", payload)
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetFreePhysicalVolumes", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Requesting free physical volumes", "viosUUID", viosUUID, "url", url)
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
	if debug {
		c.Logger.Debug("Request headers prepared")
	}

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

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

	// Log the response status and body
	if debug {
		c.Logger.Debug("GetFreePhyVolume response status", "status", resp.Status)
		c.Logger.Debug("GetFreePhyVolume response body", "body", string(body))
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
			c.Logger.Error("HMC error encountered", "status", resp.Status, "message", errorMsgs[0].Text())
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(body))
		}
		c.Logger.Error("Request failed", "status", resp.Status)
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
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(jobID, false, 10, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if debug {
		c.Logger.Info("Free Physical Volume job response", "status", jobResp.Status)
	}

	// Extract the result XML from the job response Results parameters
	var pvXML string
	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "result" {
			pvXML = param.ParameterValue
			break
		}
	}
	if pvXML == "" {
		if debug {
			c.Logger.Warn("result parameter not found in job response")
		}
		return nil, fmt.Errorf("result not found in job response")
	}
	
	if debug {
		c.Logger.Debug("resultElem content parsed", "content", pvXML)
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
		if debug {
			c.Logger.Info("No free physical volumes found", "viosUUID", viosUUID)
		}
		// Return an empty list instead of an error, as no volumes is a valid case
		return listPv, nil
	}
	if debug {
		c.Logger.Info("Found free physical volumes", "count", len(listPv), "viosUUID", viosUUID)
		for i, pv := range listPv {
			c.Logger.Debug(fmt.Sprintf("Physical Volume %d", i+1), 
				"Name", pv.VolumeName, 
				"VolumeUniqueID", pv.VolumeUniqueID, 
				"UniqueDeviceID", pv.UniqueDeviceID, 
				"StorageLabel", pv.StorageLabel, 
				"Capacity", pv.VolumeCapacity, 
				"LocationCode", pv.LocationCode, 
				"State", pv.VolumeState)
		}
	}
	return listPv, nil
}

// RemoveVolumeLPARMapping deletes the LPAR Client Adapter (unmaps the disk from the partition) via the REST API.
// RemoveVolumeLPARMapping removes one or more volume mappings between a VIOS and an LPAR.
// It accepts a slice of volume names and returns a slice of VolumeMappingInfo for each successfully processed volume.
// Each VolumeMappingInfo contains the VTD name and Server Adapter Delete URL for further cleanup orchestration.
func (c *HmcRestClient) RemoveVolumeLPARMapping(viosUUID, lparUUID string, volumeNames []string, debug bool) ([]VolumeMappingInfo, error) {
	// =====================================================================
	// STEP 1: Find the Client Slot, Server Slot, and VTD from the Mapping
	// =====================================================================
	mappingsURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching VSCSI mappings", "viosUUID", viosUUID, "url", mappingsURL)
	}
	
	mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, debug)
	if err != nil {
		return nil, err
	}

	if debug && mappingsDoc != nil {
		docStr, _ := mappingsDoc.WriteToString()
		c.Logger.Debug("ViosSCSIMapping XML response body", "body", docStr)
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

		if debug {
			c.Logger.Debug("Evaluating Mapping", "href", href, "volumeName", vName, "clientSlot", cSlot, "serverSlot", sSlot)
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
				
				if debug {
					c.Logger.Debug("MATCH FOUND!", "volume", volName, "clientSlot", cSlot, "serverSlot", sSlot, "vtd", vtdName)
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
	clientAdaptersDoc, _ := c.fetchAndParseHMCXML(clientAdaptersURL, debug)
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
	serverAdaptersDoc, _ := c.fetchAndParseHMCXML(serverAdaptersURL, debug)
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
			if debug {
				c.Logger.Debug("Executing DELETE on LPAR Client Adapter", "volume", details.volumeName, "slot", details.clientSlotNum)
			}
			reqDelClient, _ := http.NewRequest("DELETE", clientAdapterDeleteURL, nil)
			reqDelClient.Header.Set("X-API-Session", c.session)
			
			c.logRawTraffic("REQUEST (DELETE)", clientAdapterDeleteURL, "")

			respDelClient, err := c.client.Do(reqDelClient)
			if err != nil {
				c.Logger.Warn("Failed to delete client adapter", "volume", details.volumeName, "error", err)
				continue
			}
			
			clientBody, _ := io.ReadAll(respDelClient.Body)
			respDelClient.Body.Close()
			
			c.logRawTraffic("RESPONSE", clientAdapterDeleteURL, string(clientBody))

			if debug {
				c.Logger.Debug("Client Adapter DELETE Status", "volume", details.volumeName, "status", respDelClient.Status)
				if len(clientBody) > 0 {
					c.Logger.Debug("Client Adapter DELETE Response", "body", string(clientBody))
				}
			}

			if respDelClient.StatusCode >= 400 {
				c.Logger.Warn("Failed to delete client adapter", "volume", details.volumeName, "status", respDelClient.Status)
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
func (c *HmcRestClient) removeDeviceViaJob(viosUUID, deviceName string, debug bool) error {
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

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", debug, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice job submission failed with status %s", resp.Status)
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, debug)
		return err
	}
	return fmt.Errorf("JobID not found")
}
// RemoveVIOSDevice executes the RemoveDevice job on the VIOS to delete a physical volume (e.g., hdisk3)
func (c *HmcRestClient) RemoveVIOSDevice(viosUUID, deviceName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", debug, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice failed with status %s: %s", resp.Status, string(body))
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		if debug { c.Logger.Info("Removing device from VIOS", "deviceName", deviceName, "jobID", jobIDElem.Text()) }
		// Wait for the VIOS to finish deleting the disk
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, debug)
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

// GetVirtualIOServers retrieves detailed, comprehensive information for all Virtual I/O Servers 
// of a managed system using exhaustive Go struct unmarshaling.
func (c *HmcRestClient) GetVirtualIOServers(systemUUID string, debug bool) ([]VirtualIOServerDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer", c.hmcIP, systemUUID)
	
	if debug {
		c.Logger.Debug("Fetching exhaustive VIOS details", "systemUUID", systemUUID)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// 1. Strip XML namespaces to prevent Unmarshal tag conflicts
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// 2. Convert the clean etree Document back to a byte array
	strippedXML, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to write stripped XML to bytes: %v", err)
	}

	// 3. Define an inline wrapper to catch the Atom Feed hierarchy
	var feed struct {
		Entries []struct {
			VIOS VirtualIOServerDetailed `xml:"content>VirtualIOServer"`
		} `xml:"entry"`
	}

	// 4. Elegantly unmarshal the entire payload into our exhaustive structs
	if err := xml.Unmarshal(strippedXML, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exhaustive VIOS data: %v", err)
	}

	// Extract from the feed wrapper into a clean slice
	var viosList []VirtualIOServerDetailed
	for _, entry := range feed.Entries {
		viosList = append(viosList, entry.VIOS)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual I/O Servers comprehensively", "count", len(viosList))
	}

	return viosList, nil
}
// GetVirtualIOServer retrieves detailed information for a specific Virtual I/O Server using its UUID.
func (c *HmcRestClient) GetVirtualIOServer(viosUUID string, debug bool) (*VirtualIOServerDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Fetching VIOS details", "viosUUID", viosUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip XML namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Convert the clean etree Document back to a byte array
	strippedXML, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to write stripped XML to bytes: %v", err)
	}

	// Define wrapper to catch the Atom entry hierarchy
	var entry struct {
		VIOS VirtualIOServerDetailed `xml:"content>VirtualIOServer"`
	}

	// Unmarshal the entire payload into our exhaustive struct
	if err := xml.Unmarshal(strippedXML, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VIOS data: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual I/O Server", "partitionName", entry.VIOS.PartitionName)
	}

	return &entry.VIOS, nil
}

// GetVirtualSCSIServerAdapters retrieves a list of all Virtual SCSI Server Adapters (vhost) configured on a specific VIOS.
func (c *HmcRestClient) GetVirtualSCSIServerAdapters(viosUUID string, debug bool) ([]VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching Virtual SCSI Server Adapters", "viosUUID", viosUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualSCSIServerAdapters HTTP response status", "status", resp.Status)
		c.Logger.Debug("GetVirtualSCSIServerAdapters response body", "body", string(body))
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

		// Use XML unmarshaling to automatically populate the struct
		var adapter VirtualSCSIServerAdapter
		
		// Create a new document with the VirtualSCSIServerAdapter element as root
		adapterDoc := etree.NewDocument()
		adapterDoc.SetRoot(adapterElem.Copy())
		
		adapterBytes, err := adapterDoc.WriteToBytes()
		if err != nil {
			if debug {
				c.Logger.Warn("Warning: failed to serialize VirtualSCSIServerAdapter element", "error", err)
			}
			continue
		}

		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			if debug {
				c.Logger.Warn("Warning: failed to unmarshal VirtualSCSIServerAdapter", "error", err)
			}
			continue
		}

		// Extract the direct URI for this specific adapter from the Atom <link rel="SELF">
		for _, link := range entry.FindElements("./link") {
			if link.SelectAttrValue("rel", "") == "SELF" {
				adapter.Link = link.SelectAttrValue("href", "")
				break
			}
		}

		// Populate deprecated ID field for backward compatibility
		adapter.ID = adapter.UUID

		adapters = append(adapters, adapter)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual SCSI Server Adapters", "count", len(adapters))
	}

	return adapters, nil
}

// GetVirtualSCSIServerAdapter retrieves the details of a specific Virtual SCSI Server Adapter (vhost) using its UUID.
func (c *HmcRestClient) GetVirtualSCSIServerAdapter(viosUUID, adapterUUID string, debug bool) (*VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if debug {
		c.Logger.Debug("Fetching specific Virtual SCSI Server Adapter", "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualSCSIServerAdapter Status", "status", resp.Status)
		c.Logger.Debug("GetVirtualSCSIServerAdapter Body", "body", string(body))
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

	// Use XML unmarshaling to automatically populate the struct
	var adapter VirtualSCSIServerAdapter
	
	// Create a new document with the VirtualSCSIServerAdapter element as root
	adapterDoc := etree.NewDocument()
	adapterDoc.SetRoot(adapterElem.Copy())
	
	adapterBytes, err := adapterDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize VirtualSCSIServerAdapter element: %v", err)
	}

	if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VirtualSCSIServerAdapter: %v", err)
	}

	// Set the Link field (we already know the direct URL)
	adapter.Link = url
	
	// Populate deprecated ID field for backward compatibility
	adapter.ID = adapter.UUID

	return &adapter, nil
}

// DeleteVirtualSCSIServerAdapter removes a specific Virtual SCSI Server Adapter (vhost) from a VIOS.
func (c *HmcRestClient) DeleteVirtualSCSIServerAdapter(viosUUID, adapterUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if debug {
		c.Logger.Debug("Deleting Virtual SCSI Server Adapter", "url", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	
	// Read the body even if it's empty to ensure the connection is freed properly
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("DeleteVirtualSCSIServerAdapter Status", "status", resp.Status)
		if len(body) > 0 {
			c.Logger.Debug("DeleteVirtualSCSIServerAdapter Body", "body", string(body))
		}
	}

	// Accept both 200 OK and 204 No Content as successful deletions
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete VirtualSCSIServerAdapter. Status: %s, Response: %s", resp.Status, string(body))
	}

	return nil
}

// GetViosSCSIMappings retrieves and fully parses all VSCSI mappings for a specific VIOS.
func (c *HmcRestClient) GetViosSCSIMappings(viosUUID string, debug bool) ([]VirtualSCSIMapping, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VSCSI Mappings", "viosUUID", viosUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var mappingsList []VirtualSCSIMapping

	mappingElems := doc.FindElements("//VirtualSCSIMapping")
	for _, mappingElem := range mappingElems {
		// Create a new document with mappingElem as root for serialization
		mappingDoc := etree.NewDocument()
		mappingDoc.SetRoot(mappingElem.Copy())
		
		// Serialize to bytes
		mappingBytes, err := mappingDoc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize VirtualSCSIMapping element: %v", err)
		}
		
		// Unmarshal into VirtualSCSIMapping struct
		var mapping VirtualSCSIMapping
		if err := xml.Unmarshal(mappingBytes, &mapping); err != nil {
			return nil, fmt.Errorf("failed to unmarshal VirtualSCSIMapping: %v", err)
		}
		
		mappingsList = append(mappingsList, mapping)
	}

	if debug {
		c.Logger.Info("Successfully parsed VSCSI Mappings", "count", len(mappingsList))
	}

	return mappingsList, nil
}

// =====================================================================
// VOLUME GROUP API METHODS
// =====================================================================

// GetVolumeGroups retrieves a list of all Volume Groups configured on a specific VIOS.
func (c *HmcRestClient) GetVolumeGroups(viosUUID string, debug bool) ([]VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

    if debug {
        c.Logger.Debug("Fetching Volume Groups", "viosUUID", viosUUID, "url", url)
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

    c.logRawTraffic("REQUEST (GET)", url, "")

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    c.logRawTraffic("RESPONSE", url, string(body))

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

        // Use XML unmarshaling to automatically populate the struct
        var vg VolumeGroup
        
        // Create a new document with the VolumeGroup element as root
        vgDoc := etree.NewDocument()
        vgDoc.SetRoot(vgElem.Copy())
        
        vgBytes, err := vgDoc.WriteToBytes()
        if err != nil {
            if debug {
                c.Logger.Warn("Warning: failed to serialize VolumeGroup element", "error", err)
            }
            continue
        }

        if err := xml.Unmarshal(vgBytes, &vg); err != nil {
            if debug {
                c.Logger.Warn("Warning: failed to unmarshal VolumeGroup", "error", err)
            }
            continue
        }

        volumeGroups = append(volumeGroups, vg)
    }

    return volumeGroups, nil
}

// GetVolumeGroup retrieves the details of a specific Volume Group on a Virtual I/O Server.
func (c *HmcRestClient) GetVolumeGroup(viosUUID, vgUUID string, debug bool) (*VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup/%s", c.hmcIP, viosUUID, vgUUID)

    if debug {
        c.Logger.Debug("Fetching Volume Group", "vgUUID", vgUUID, "viosUUID", viosUUID, "url", url)
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

    c.logRawTraffic("REQUEST (GET)", url, "")

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    c.logRawTraffic("RESPONSE", url, string(body))

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

    // Use XML unmarshaling to automatically populate the struct
    var vg VolumeGroup
    
    // Create a new document with the VolumeGroup element as root
    vgDoc := etree.NewDocument()
    vgDoc.SetRoot(vgElem.Copy())
    
    vgBytes, err := vgDoc.WriteToBytes()
    if err != nil {
        return nil, fmt.Errorf("failed to serialize VolumeGroup element: %v", err)
    }
    
    if err := xml.Unmarshal(vgBytes, &vg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal VolumeGroup: %v", err)
    }

    if debug {
        c.Logger.Info("Successfully parsed Volume Group", "groupName", vg.GroupName, "uuid", vg.UUID)
        c.Logger.Debug("Volume Group Contents", "VirtualDisks", len(vg.VirtualDisks), "OpticalMedia", len(vg.OpticalMedia), "PhysicalVolumes", len(vg.PhysicalVolumes))
    }

    return &vg, nil
}
// =====================================================================
// CREATE VOLUME GROUP
// =====================================================================

// CreateVolumeGroup creates a new Volume Group on a VIOS using the specified physical volumes.
func (c *HmcRestClient) CreateVolumeGroup(viosUUID, vgName string, physicalVolumes []string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Creating Volume Group", "vgName", vgName, "viosUUID", viosUUID, "disks", physicalVolumes)
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

	if debug {
		c.Logger.Debug("CreateVolumeGroup Payload", "payload", payload)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

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

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("CreateVolumeGroup HTTP response status", "status", resp.Status)
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
			if debug {
				c.Logger.Info("CreateVolumeGroup Job submitted, waiting for completion...", "jobID", jobID)
			}
			_, err = c.FetchJobStatus(jobID, false, 10, debug)
			if err != nil {
				return fmt.Errorf("CreateVolumeGroup job failed: %v", err)
			}
			if debug {
				c.Logger.Info("CreateVolumeGroup job completed successfully.")
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
func (c *HmcRestClient) CreateVirtualDisk(sysName, viosUUID, viosName, vgName, diskName string, capacityMB int, debug bool) error {
	requiredGB := float64(capacityMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity and naming for new Virtual Disk", "diskName", diskName, "capacityMB", capacityMB, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to verify capacity and check for naming collisions
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
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

	if debug {
		c.Logger.Debug("VG Capacity Check", "vgName", foundVgName, "availableGB", freeSpaceGB, "requiredGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, capacityMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if debug {
		c.Logger.Info("Pre-flight checks passed. Executing creation via CLI...")
	}

	// Syntax: mklv -lv <diskName> <vgName> <Size>M
	mklvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mklv -lv %s %s %dM"`, sysName, viosName, diskName, vgName, capacityMB)

	output, err := c.CliRunner(mklvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create Virtual Disk via mklv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk created successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE VIRTUAL DISK (LOGICAL VOLUME) - CLI METHOD
// =====================================================================

// DeleteVirtualDisk safely removes a Logical Volume (Virtual Disk) from a VIOS.
// It uses the native VIOS rmlv command with the -f flag to bypass confirmation prompts.
func (c *HmcRestClient) DeleteVirtualDisk(sysName, viosName, diskName string, debug bool) error {
	if debug {
		c.Logger.Debug("Safely deleting Virtual Disk via CLI", "diskName", diskName, "viosName", viosName)
	}

	// Syntax: rmlv -f <diskName>
	// The -f flag is required for automation so the OS does not wait for user input.
	rmlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmlv -f %s"`, sysName, viosName, diskName)
	
	if debug {
		c.Logger.Debug("Executing", "command", rmlvCmd)
	}

	output, err := c.CliRunner(rmlvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to delete Virtual Disk via rmlv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk deleted successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// EXTEND VIRTUAL DISK (LOGICAL VOLUME) - SMART CLI METHOD
// =====================================================================

// ExtendVirtualDisk safely increases the size of an existing Logical Volume (Virtual Disk).
// It automatically queries the HMC to verify the host Volume Group has enough free space before executing.
func (c *HmcRestClient) ExtendVirtualDisk(sysName, viosUUID, viosName, diskName string, additionalMB int, debug bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity for Virtual Disk extension", "diskName", diskName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the disk and check capacity
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
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

				if debug {
					c.Logger.Debug("Found disk inside VG", "diskName", diskName, "vgName", vg.GroupName, "availableSpaceGB", freeSpaceGB)
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
	if debug {
		c.Logger.Info("Capacity check passed. Executing extension via CLI...")
	}

	extendlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "extendlv %s %dM"`, sysName, viosName, diskName, additionalMB)
	
	output, err := c.CliRunner(extendlvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to extend Virtual Disk via extendlv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk extended successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
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
func (c *HmcRestClient) CreateVirtualOpticalMedia(sysName, viosUUID, viosName, mediaName, sourceFile string, sizeMB int, readOnly, nfsLink, debug bool) error {
	if nfsLink && sourceFile == "" {
		return fmt.Errorf("ABORT: The -nfslink flag can only be used when providing a sourceFile")
	}

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying naming for Virtual Optical Media", "mediaName", mediaName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to check for naming collisions in the Media Repository
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
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
		
		if debug {
			c.Logger.Info("Pre-flight passed. Importing ISO from file", "sourceFile", sourceFile, "readOnly", readOnly, "nfsLink", nfsLink)
		}
		// Syntax: mkvopt -name <mediaName> -file <SourceFile> [-nfslink] [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -file %s%s%s"`, sysName, viosName, mediaName, sourceFile, nfsFlag, roFlag)
	} else {
		if debug {
			c.Logger.Info("Pre-flight passed. Creating blank media", "sizeMB", sizeMB, "readOnly", readOnly)
		}
		// Syntax: mkvopt -name <mediaName> -size <Size>M [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -size %dM%s"`, sysName, viosName, mediaName, sizeMB, roFlag)
	}

	// 3. Execute the creation/import via CLI
	output, err := c.CliRunner(mkvoptCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create/import Virtual Optical Media: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Optical Media created successfully", "mediaName", mediaName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// CREATE MEDIA REPOSITORY - SMART CLI METHOD
// =====================================================================

// CreateMediaRepository safely creates the Virtual Media Repository on a VIOS.
// It verifies that no repository currently exists on the VIOS, and that the target VG has enough space.
func (c *HmcRestClient) CreateMediaRepository(sysName, viosUUID, viosName, vgName string, sizeMB int, debug bool) error {
	requiredGB := float64(sizeMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity and existing repositories for VG", "vgName", vgName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to check for existing repositories and verify capacity
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var foundVgFreeSpace string
	var foundVgName string

	for _, vg := range vgList {
		// COLLISION CHECK: A VIOS can only have one repository globally.
		if vg.MediaRepositoryName != "" {
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

	if debug {
		c.Logger.Debug("VG Capacity Check", "vgName", foundVgName, "availableGB", freeSpaceGB, "requiredGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, sizeMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if debug {
		c.Logger.Info("Pre-flight checks passed. Executing mkrep via CLI...")
	}

	// Syntax: mkrep -sp <vgName> -size <sizeMB>M
	mkrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkrep -sp %s -size %dM"`, sysName, viosName, vgName, sizeMB)

	output, err := c.CliRunner(mkrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create Media Repository via mkrep: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository created successfully", "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE MEDIA REPOSITORY - SMART CLI METHOD (ENHANCED)
// =====================================================================

// DeleteMediaRepository removes the Virtual Media Repository from a VIOS.
// It verifies the repo exists, checks for media if force is false, and warns if force is true.
func (c *HmcRestClient) DeleteMediaRepository(sysName, viosUUID, viosName, repoName string, force, debug bool) error {
	if debug {
		c.Logger.Debug("Pre-flight check: Looking for Media Repository", "repoName", repoName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the existing repository
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var targetVG *VolumeGroup
	for i := range vgList {
		if vgList[i].MediaRepositoryName != "" {
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
		c.Logger.Warn("Force flag is ENABLED. Deleting repository will also PERMANENTLY DELETE ISO file(s) inside it.", "repoName", repoName, "mediaCount", mediaCount)
	} else if mediaCount > 0 {
		// Fail WITHOUT calling rmrep if media exists and force is false
		return fmt.Errorf("ABORT: Repository '%s' contains %d ISO file(s). Use the 'force' flag to delete anyway", repoName, mediaCount)
	}

	// 2. Execute the deletion via CLI
	if debug {
		c.Logger.Info("Pre-flight passed. Executing rmrep on VIOS...")
	}

	forceFlag := ""
	if force {
		forceFlag = " -f"
	}

	rmrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmrep%s"`, sysName, viosName, forceFlag)
	
	output, err := c.CliRunner(rmrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to delete Media Repository via CLI: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository deleted successfully", "repoName", repoName)
	}

	return nil
}
// =====================================================================
// CHANGE MEDIA REPOSITORY (EXTEND) - SMART CLI METHOD
// =====================================================================

// ChangeMediaRepository increases the size of the Virtual Media Repository.
// additionalMB is the amount of NEW space to add (incremental).
// It identifies the hosting Volume Group and verifies free space before executing.
func (c *HmcRestClient) ChangeMediaRepository(sysName, viosUUID, viosName string, additionalMB int, debug bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying VG capacity for repository expansion on VIOS", "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the repository's location
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var hostingVG *VolumeGroup
	for i := range vgList {
		if vgList[i].MediaRepositoryName != "" {
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

	if debug {
		c.Logger.Debug("Repository capacity check", "vgName", hostingVG.GroupName, "availableGB", freeSpaceGB, "requestedGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: VG '%s' only has %.2f GB free, cannot add %.2f GB", 
			hostingVG.GroupName, freeSpaceGB, requiredGB)
	}

	// 2. Execute the expansion via CLI
	if debug {
		c.Logger.Info("Pre-flight passed. Extending repository", "repoName", hostingVG.MediaRepositoryName, "additionalMB", additionalMB)
	}

	// Syntax: chrep -size <Size>M
	chrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "chrep -size %dM"`, sysName, viosName, additionalMB)
	
	output, err := c.CliRunner(chrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to extend Media Repository via chrep: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository expanded successfully", "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// CreatePhysicalVolumeMap maps one or more physical disks on the VIOS to a target LPAR using the GET-Modify-POST pattern.
// Supports batch operations - pass multiple disk names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreatePhysicalVolumeMap(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	if len(diskNames) == 0 {
		return "", fmt.Errorf("at least one disk name is required")
	}
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching VIOS to inject new storage mapping...", "viosUUID", viosUUID)
	}

	doc, err := c.fetchAndParseHMCXML(url, debug)
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
		if debug { c.Logger.Debug("No existing mappings found. Creating VirtualSCSIMappings group...") }
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

		if debug {
			c.Logger.Debug("Creating mapping XML for disk", "diskName", diskName, "xml", newMappingXML)
		}

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", diskName, err)
		}

		// 5. Append the new mapping safely to the list
		mappingsList.AddChild(newMappingDoc.Root())
		
		if debug {
			c.Logger.Debug("Added mapping for disk", "diskName", diskName)
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

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", xmlStr)
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
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

	c.logRawTraffic("REQUEST (POST)", postURL, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug { c.Logger.Info("Update job triggered", "jobID", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// CreateVirtualDiskMap maps one or more Logical Volumes (Virtual Disks) on the VIOS to a target LPAR using a pristine GET-Modify-POST.
// Supports batch operations - pass multiple disk names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	if len(diskNames) == 0 {
		return "", fmt.Errorf("at least one disk name is required")
	}
	// 1. Raw GET - We DO NOT use fetchAndParseHMCXML because we MUST preserve all namespaces and kxe/kb attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML for Virtual Disk mapping...", "viosUUID", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(rawXML))

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

	 if debug {
	  c.Logger.Debug("Creating mapping XML for virtual disk", "diskName", diskName, "xml", newMappingXML)
	 }

	 newMappingDoc := etree.NewDocument()
	 if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
	  return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", diskName, err)
	 }

	 // 6. Inject the new mapping
	 mappingsList.AddChild(newMappingDoc.Root())
	 
	 if debug {
	  c.Logger.Debug("Added mapping for virtual disk", "diskName", diskName)
	 }
	}

	// 7. Extract the VIOS document to POST
	// Because viosElem was cloned from pristine XML, it retains all original namespaces and attributes
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", postXML)
		c.Logger.Info("POSTing pristine modified XML back to HMC...")
	}

	// 8. Execute POST
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 9. Graceful error handling
	// We still catch the HSCL2957 warning in case our TARGET LPAR is powered off!
	/* if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "HSCL2957") {
			if debug {
				c.Logger.Warn("Mapping saved, but target DLPAR dynamic injection failed (Expected if your target LPAR is powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
	} */

	// 10. Fetch Job Status
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug { c.Logger.Info("Update job triggered", "jobID", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// AddVirtualOpticalMedia natively imports an ISO file into the VIOS Media Repository using the AddOpticalMedia job.
// AddVirtualOpticalMedia adds one or more Virtual Optical Media (ISO files) to a VIOS Media Repository.
// mediaFiles is a map where keys are media names and values are file paths.
// Returns a map of results for each media (nil for success, error for failure).
// Note: This requires HMC V10.3.1061.0 or later.
func (c *HmcRestClient) AddVirtualOpticalMedia(viosUUID string, mediaFiles map[string]string, debug bool) (map[string]error, error) {
	if len(mediaFiles) == 0 {
		return nil, fmt.Errorf("at least one media file is required")
	}

	results := make(map[string]error)
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/AddOpticalMedia", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Info("Adding Virtual Optical Media to VIOS", "count", len(mediaFiles), "viosUUID", viosUUID)
	}

	// Process each media file sequentially
	for mediaName, fileName := range mediaFiles {
		if debug {
			c.Logger.Debug("Processing media", "mediaName", mediaName, "fileName", fileName)
		}

		// 1. Define operation details for the JobRequest
		operation := map[string]string{
			"OperationName": "AddOpticalMedia",
			"GroupName":     "VirtualIOServer",
			"ProgressType":  "DISCRETE",
		}

		// 2. Build job parameters
		params := map[string]string{
			"MediaName": mediaName,
			"FileName":  fileName,
		}

		// 3. Generate the XML payload
		payload, err := createJobRequestPayload(operation, params, "V1_0", debug, true)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to create job request payload: %v", err)
			continue
		}

		// 4. Create and configure the PUT request
		req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to create request: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
		req.Header.Set("X-API-Session", c.session)
		req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		req = req.WithContext(ctx)

		c.logRawTraffic("REQUEST (PUT)", url, payload)

		// 5. Send the request
		resp, err := c.client.Do(req)
		if err != nil {
			cancel()
			results[mediaName] = fmt.Errorf("HTTP request failed: %v", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		
		c.logRawTraffic("RESPONSE", url, string(body))

		if err != nil {
			results[mediaName] = fmt.Errorf("failed to read response body: %v", err)
			continue
		}

		if debug {
			c.Logger.Debug("AddOpticalMedia Response Status", "mediaName", mediaName, "status", resp.Status)
		}

		if resp.StatusCode >= 400 {
			results[mediaName] = fmt.Errorf("AddOpticalMedia job failed with status %s: %s", resp.Status, string(body))
			continue
		}

		// 6. Strip namespaces to find the JobID
		doc, err := xmlStripNamespace(body)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to strip namespaces from XML response: %v", err)
			continue
		}

		jobIDElem := doc.FindElement("//JobID")
		if jobIDElem == nil {
			results[mediaName] = fmt.Errorf("JobID not found in response: %s", string(body))
			continue
		}
		jobID := jobIDElem.Text()
		
		if debug {
			c.Logger.Debug("Extracted JobID. Waiting for ISO import to complete...", "jobID", jobID, "mediaName", mediaName)
		}

		// 7. Wait for the background job to finish
		_, err = c.FetchJobStatus(jobID, false, 10, debug)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed during AddOpticalMedia job execution: %v", err)
			continue
		}

		// Success
		results[mediaName] = nil
		if debug {
			c.Logger.Info("Successfully added media", "mediaName", mediaName)
		}
	}

	// Check if all operations failed
	allFailed := true
	for _, err := range results {
		if err == nil {
			allFailed = false
			break
		}
	}

	if allFailed {
		return results, fmt.Errorf("all media additions failed")
	}

	return results, nil
}
// CreateVirtualOpticalMap creates one or more Virtual Optical Devices (CD-ROM) on the target LPAR and loads the specified ISO media.
// Supports batch operations - pass multiple media names to create multiple mappings in a single transaction.
func (c *HmcRestClient) CreateVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, debug bool) (string, error) {
	if len(mediaNames) == 0 {
		return "", fmt.Errorf("at least one media name is required")
	}
	// 1. Raw GET - Fetch pristine VIOS XML to preserve namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML for Virtual Optical mapping...", "viosUUID", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(rawXML))

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

	 if debug {
	  c.Logger.Debug("Creating mapping XML for optical media", "mediaName", mediaName, "xml", newMappingXML)
	 }

	 newMappingDoc := etree.NewDocument()
	 if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
	  return "", fmt.Errorf("failed to parse mapping XML for media %s: %v", mediaName, err)
	 }

	 // 6. Inject the new mapping
	 mappingsList.AddChild(newMappingDoc.Root())
	 
	 if debug {
	  c.Logger.Debug("Added mapping for optical media", "mediaName", mediaName)
	 }
	}

	// 7. Extract the VIOS document to POST
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", postXML)
		c.Logger.Info("POSTing pristine modified XML back to HMC...")
	}

	// 8. Execute POST
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 10. Fetch Job Status
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug { c.Logger.Info("Optical mapping job triggered", "jobID", jobIDElem.Text()) }
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// DeletePhysicalVolumeMaps removes multiple physical disk mappings from a VIOS to a target LPAR in a single transaction.
// Note: Strict HTTP error checking is disabled (Option 2 behavior).
func (c *HmcRestClient) DeletePhysicalVolumeMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VIOS to locate and remove storage mappings...", "viosUUID", viosUUID)
	}

	doc, err := c.fetchAndParseHMCXML(url, debug)
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
		if debug {
			c.Logger.Info("No mappings found for the specified disks. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing mapping(s) from XML tree", "count", len(mappingsToRemove))
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

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", xmlStr)
		c.Logger.Info("POSTing updated VIOS XML back to HMC to apply bulk deletion...")
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

	c.logRawTraffic("REQUEST (POST)", postURL, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", postURL, string(body))

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// DeleteVirtualDiskMaps removes multiple virtual disk mappings from a VIOS to an LPAR in a single operation.
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
func (c *HmcRestClient) DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VIOS to locate and remove virtual disk mappings", "viosUUID", viosUUID, "disks", diskNames)
	}

	doc, err := c.fetchAndParseHMCXML(url, debug)
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
			if debug {
				c.Logger.Debug("Found mapping for virtual disk", "diskName", diskNameElem.Text(), "lparUUID", lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No virtual disk mappings found for the specified disks. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing virtual disk mapping(s) from XML tree", "count", len(mappingsToRemove))
	}
	for i, mapping := range mappingsToRemove {
		diskNameElem := mapping.FindElement("Storage/VirtualDisk/DiskName")
		if diskNameElem != nil && debug {
			c.Logger.Debug("Removing mapping", "index", i+1, "total", len(mappingsToRemove), "diskName", diskNameElem.Text())
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

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", xmlStr)
		c.Logger.Info("POSTing updated VIOS XML back to HMC to apply virtual disk deletion...")
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

	c.logRawTraffic("REQUEST (POST)", postURL, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", postURL, string(body))

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Virtual disk deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// DeleteVirtualOpticalMaps removes multiple virtual optical media mappings from a VIOS to an LPAR in a single operation.
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
func (c *HmcRestClient) DeleteVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, debug bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VIOS to locate and remove virtual optical mappings", "viosUUID", viosUUID, "media", mediaNames)
	}

	doc, err := c.fetchAndParseHMCXML(url, debug)
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
			if debug {
				c.Logger.Debug("Found mapping for optical media", "mediaName", mediaNameElem.Text(), "lparUUID", lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No virtual optical mappings found for the specified media. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success if they are already gone
	}

	// 5. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing virtual optical mapping(s) from XML tree", "count", len(mappingsToRemove))
	}
	for i, mapping := range mappingsToRemove {
		mediaNameElem := mapping.FindElement("Storage/VirtualOpticalMedia/MediaName")
		if mediaNameElem != nil && debug {
			c.Logger.Debug("Removing mapping", "index", i+1, "total", len(mappingsToRemove), "mediaName", mediaNameElem.Text())
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

	if debug {
		c.Logger.Debug("Complete XML payload to be POSTed", "payload", xmlStr)
		c.Logger.Info("POSTing updated VIOS XML back to HMC to apply virtual optical deletion...")
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

	c.logRawTraffic("REQUEST (POST)", postURL, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", postURL, string(body))

	// OPTION 2: HTTP Status check intentionally omitted to match Create behavior

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Virtual optical deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// CreateVirtualFibreChannelMappings creates multiple Virtual Fibre Channel (NPIV) mappings between a VIOS and an LPAR in a single operation.
// By mapping directly to the physical FC ports, the HMC acts as an orchestrator and automatically generates the required Client and Server virtual adapters.
//
// Parameters:
//   - sysUUID: The UUID of the managed system
//   - viosUUID: The UUID of the Virtual I/O Server
//   - lparUUID: The UUID of the logical partition
//   - fcPortNames: Slice of physical FC port names to map to the LPAR (e.g., ["fcs0", "fcs1"])
//   - verbose: Enable detailed logging
//
// Returns:
//   - "SUCCESS" if mappings were successfully created and pushed to the OS
//   - "SUCCESS_WITH_RMC_WARNING" if mappings were saved to the HMC profile but the dynamic LPAR push timed out (common for powered-off LPARs or SAN fabric delays)
//   - error if the operation fails
func (c *HmcRestClient) CreateVirtualFibreChannelMappings(sysUUID, viosUUID, lparUUID string, fcPortNames []string, debug bool) (string, error) {
	if len(fcPortNames) == 0 {
		return "", fmt.Errorf("at least one Fibre Channel port name (e.g., 'fcs0') is required")
	}

	// 1. Raw GET - Fetch pristine VIOS XML to preserve namespaces and attributes
	// We use 'group=ViosFCMapping' to target the Fibre Channel mapping configurations
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosFCMapping", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML for vFC mappings", "viosUUID", viosUUID, "fcPorts", fcPortNames)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	// LOG THE GET REQUEST (No payload)
	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	
	// LOG THE GET RESPONSE (Pristine XML)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != http.StatusOK {
		c.Logger.Error("GET failed", "status", getResp.Status, "body", string(rawXML))
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

	// 4. Find or create the VirtualFibreChannelMappings list
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualFibreChannelMappings']")
	if mappingsList == nil {
		if debug {
			c.Logger.Debug("No existing vFC mappings found. Creating VirtualFibreChannelMappings group...")
		}
		mappingsList = viosElem.CreateElement("VirtualFibreChannelMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_3_0")
	}

	// 5. Construct and inject the new mapping XML blocks for EACH port
	for _, fcPortName := range fcPortNames {
		newMappingXML := fmt.Sprintf(`
        <VirtualFibreChannelMapping schemaVersion="V1_3_0">
            <AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
            <Port schemaVersion="V1_3_0">
                <PortName>%s</PortName>
            </Port>
        </VirtualFibreChannelMapping>`, c.hmcIP, sysUUID, lparUUID, fcPortName)

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for port %s: %v", fcPortName, err)
		}

		// 6. Inject the new mapping into the DOM
		mappingsList.AddChild(newMappingDoc.Root())
		
		if debug {
			c.Logger.Debug("Injected new vFC mapping into XML payload", "fcPort", fcPortName)
		}
	}

	// 7. Extract the VIOS document to POST
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	// 8. Execute POST
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	// LOG THE POST REQUEST (Modified XML)
	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// LOG THE POST RESPONSE (HMC Job Status or Error)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// =========================================================================
	// GRACEFUL RMC ERROR HANDLING
	// =========================================================================
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR/RMC timeout warnings (like HSCL294D)
		if strings.Contains(bodyStr, "HSCL294D") || strings.Contains(bodyStr, "HSCL2957") {
			if debug {
				c.Logger.Warn("Mapping(s) saved to HMC, but dynamic DLPAR push timed out (Common with SAN fabric delays or offline LPARs).", 
					"status", postResp.Status)
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		
		c.Logger.Error("POST failed", "status", postResp.Status, "body", bodyStr)
		return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
	}

	// 9. Fetch Job Status
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Mapping job triggered", "jobID", jobIDElem.Text())
			}
			_, jobErr := c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
			if jobErr != nil {
				return "", fmt.Errorf("background job failed: %v", jobErr)
			}
		}
	}

	if debug {
		c.Logger.Info("Virtual Fibre Channel mapping(s) created successfully", "lparUUID", lparUUID, "fcPorts", fcPortNames)
	}

	return "SUCCESS", nil
}

// DeleteVirtualFibreChannelMappings removes multiple Virtual Fibre Channel (NPIV) mappings from a VIOS to an LPAR in a single operation.
//
// Parameters:
//   - sysUUID: The UUID of the managed system
//   - viosUUID: The UUID of the Virtual I/O Server
//   - lparUUID: The UUID of the logical partition
//   - fcPortNames: Slice of physical FC port names to unmap (e.g., ["fcs0", "fcs1"])
//   - verbose: Enable detailed logging
//
// Returns:
//   - "SUCCESS" if mappings were deleted
//   - "SUCCESS_WITH_RMC_WARNING" if mappings were deleted but dynamic LPAR push timed out
//   - "NOT_FOUND" if no matching mappings exist (idempotent)
//   - error if operation fails
func (c *HmcRestClient) DeleteVirtualFibreChannelMappings(sysUUID, viosUUID, lparUUID string, fcPortNames []string, debug bool) (string, error) {
	// 1. GET the VIOS with its mappings extended group
	// IBM strictly requires 'group=ViosFCMapping' for Fibre Channel topologies
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosFCMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VIOS to locate and remove vFC mappings", "viosUUID", viosUUID, "fcPorts", fcPortNames)
	}

	// Fetch and strip namespaces so we can easily query the DOM
	doc, err := c.fetchAndParseHMCXML(url, debug)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VIOS vFC mappings: %v", err)
	}

	// 2. EXTRACT the actual VirtualIOServer element
	viosElem := doc.FindElement("//VirtualIOServer")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in VIOS XML")
	}

	// 3. Locate the VirtualFibreChannelMappings collection
	mappingsList := viosElem.FindElement(".//VirtualFibreChannelMappings")
	if mappingsList == nil {
		if debug {
			c.Logger.Info("No VirtualFibreChannelMappings collection found. Nothing to delete.", "viosUUID", viosUUID)
		}
		return "NOT_FOUND", nil
	}

	// Create a fast lookup map for the ports we want to unmap
	targetPorts := make(map[string]bool)
	for _, port := range fcPortNames {
		targetPorts[port] = true
	}

	// 4. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements("VirtualFibreChannelMapping") {
		// Check if it belongs to our target LPAR
		lparRef := mapping.FindElement("AssociatedLogicalPartition")
		if lparRef == nil {
			continue
		}
		href := lparRef.SelectAttrValue("href", "")
		if !strings.Contains(href, lparUUID) {
			continue // Not our LPAR
		}

		// Check if the mapped port is one of our target ports
		portNameElem := mapping.FindElement("Port/PortName")
		if portNameElem == nil {
			continue
		}
		
		if targetPorts[portNameElem.Text()] {
			if debug {
				c.Logger.Debug("Found mapping targeted for deletion", "fcPort", portNameElem.Text(), "lparUUID", lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No matching vFC mappings found for the specified ports. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success
	}

	// 5. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing vFC mappings from XML tree", "count", len(mappingsToRemove))
	}
	
	for i, mapping := range mappingsToRemove {
		portNameElem := mapping.FindElement("Port/PortName")
		if portNameElem != nil && debug {
			c.Logger.Debug("Removing mapping", "index", i+1, "total", len(mappingsToRemove), "fcPort", portNameElem.Text())
		}
		mappingsList.RemoveChild(mapping)
	}

	// 6. Natively set the correct namespace and tag on the root element
	// Because fetchAndParseHMCXML strips namespaces, we MUST add them back before POSTing
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

	// 8. POST the complete update back to the VIOS API
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("POSTing updated VIOS XML back to HMC to apply vFC deletion")
	}

	req, err := http.NewRequest("POST", postURL, strings.NewReader(xmlStr))
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	req.Header.Set("Accept", "application/atom+xml")

	// Wire logging to capture the exact payload sent
	c.logRawTraffic("REQUEST (POST)", postURL, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	// Wire logging to capture the HMC response
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// =========================================================================
	// GRACEFUL RMC ERROR HANDLING
	// =========================================================================
	if resp.StatusCode >= 400 {
		bodyStr := string(body)
		
		// Catch known IBM DLPAR/RMC timeout warnings (like HSCL294D)
		// Deleting a mapping triggers OS-level changes on the LPAR and VIOS
		if strings.Contains(bodyStr, "HSCL294D") || strings.Contains(bodyStr, "HSCL2957") {
			if debug {
				c.Logger.Warn("Mapping deleted from HMC, but dynamic DLPAR push timed out (Common with SAN fabric delays or offline LPARs).", 
					"status", resp.Status)
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		
		c.Logger.Error("POST failed", "status", resp.Status, "body", bodyStr)
		return "", fmt.Errorf("POST failed (%s): %s", resp.Status, bodyStr)
	}

	// 9. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("vFC deletion job triggered", "jobID", jobIDElem.Text())
			}
			_, jobErr := c.FetchJobStatus(jobIDElem.Text(), false, 10, debug)
			if jobErr != nil {
				return "", fmt.Errorf("background job failed: %v", jobErr)
			}
		}
	}

	if debug {
		c.Logger.Info("Virtual Fibre Channel mapping(s) deleted successfully")
	}

	return "SUCCESS", nil
}