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
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	var jobDocStr string
	if verbose {
		jobDocStr, _ = jobDoc.WriteToString()
		hmcLogger.Printf("ConfigDevice job response:\n%s", jobDocStr)
	}

	// Check job status
	statusElem := jobDoc.FindElement("//Status")
	if statusElem == nil || statusElem.Text() != "COMPLETED_OK" {
		messageElem := jobDoc.FindElement("//Message")
		msg := ""
		if messageElem != nil {
			msg = messageElem.Text()
		}
		return fmt.Errorf("job failed: status %s, message: %s", statusElem.Text(), msg)
	}

	// Optionally check StdError
	stdErrorElem := jobDoc.FindElement("//StdError")
	if stdErrorElem != nil && stdErrorElem.Text() != "" {
		return fmt.Errorf("config device error: %s", stdErrorElem.Text())
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
	pvDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	var pvDocStr string
	if verbose {
		pvDocStr, _ = pvDoc.WriteToString()
		hmcLogger.Printf("Free Physical Volume job response:\n%s", pvDocStr)
	}
	// Extract the result XML from the job response
	resultElem := pvDoc.FindElement("//Results/JobParameter/ParameterValue")
	if verbose {
		if resultElem != nil {
			hmcLogger.Printf("resultElem content: %s", resultElem.Text())
		} else {
			hmcLogger.Printf("resultElem is nil: no ParameterValue found for ParameterName 'result'")
		}
	}
	if resultElem == nil {
		return nil, fmt.Errorf("result not found in job response: %s", pvDocStr)
	}
	pvXML := resultElem.Text()

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

// GetViosSCSIMappings retrieves the VSCSI mappings for a VIOS using the extended group.
func (c *HmcRestClient) GetViosSCSIMappings(viosUUID string, verbose bool) ([]*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching VSCSI mappings for VIOS UUID %s, URL: %s", viosUUID, url)
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
		hmcLogger.Printf("GetViosSCSIMappings response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetViosSCSIMappings response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	mappings := doc.FindElements("//VirtualSCSIMapping")
	if verbose {
		hmcLogger.Printf("Found %d VSCSI mappings for VIOS %s", len(mappings), viosUUID)
	}

	return mappings, nil
}
// RemoveVolumeLPARMapping deletes the LPAR Client Adapter (unmaps the disk from the partition) via the REST API.
// It returns the Virtual Target Device name (VTD) and the Server Adapter Delete URL so the caller can orchestrate the rest of the cleanup.
func (c *HmcRestClient) RemoveVolumeLPARMapping(viosUUID, lparUUID, volumeName string, verbose bool) (string, string, error) {
	// =====================================================================
	// STEP 1: Find the Client Slot, Server Slot, and VTD from the Mapping
	// =====================================================================
	mappingsURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching VSCSI mappings for VIOS UUID %s, URL: %s", viosUUID, mappingsURL)
	}
	
	mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, verbose)
	if err != nil {
		return "", "", err
	}

	if verbose && mappingsDoc != nil {
		docStr, _ := mappingsDoc.WriteToString()
		hmcLogger.Printf("ViosSCSIMapping XML response body:\n%s", docStr)
	}

	var clientSlotNum, serverSlotNum, vtdName string
	targetLparLower := strings.ToLower(lparUUID)

	for _, mapping := range mappingsDoc.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assocLpar == nil { continue }

		href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
		
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

		if strings.HasSuffix(href, targetLparLower) {
			isMatch := false
			if vName != "" && vName == volumeName {
				isMatch = true
			} else if vName == "" && volumeName == ("EMPTY_VSCSI_SLOT_" + cSlot) {
				isMatch = true
			}

			if isMatch {
				clientSlotNum = cSlot
				serverSlotNum = sSlot
				
				// Extract the Virtual Target Device (VTD) name (e.g., vtscsi0 or vtopt4)
				if targetElem := mapping.FindElement(".//*[local-name()='TargetDevice']//*[local-name()='TargetName']"); targetElem != nil {
					vtdName = targetElem.Text()
				}
				
				if verbose {
					hmcLogger.Printf("--> MATCH FOUND! Client Slot: %s | Server Slot: %s | VTD: %s", clientSlotNum, serverSlotNum, vtdName)
				}
				break
			}
		}
	}

	if clientSlotNum == "" {
		return "", "", fmt.Errorf("could not find mapping for %s on VIOS %s", volumeName, viosUUID)
	}

	// =====================================================================
	// STEP 2: Resolve the DELETE URLs for both Adapters
	// =====================================================================
	var clientAdapterDeleteURL, serverAdapterDeleteURL string

	clientAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)
	clientAdaptersDoc, _ := c.fetchAndParseHMCXML(clientAdaptersURL, verbose)
	if clientAdaptersDoc != nil {
		for _, entry := range clientAdaptersDoc.FindElements("//entry") {
			if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil && slot.Text() == clientSlotNum {
				for _, link := range entry.FindElements("./link") {
					if link.SelectAttrValue("rel", "") == "SELF" {
						clientAdapterDeleteURL = link.SelectAttrValue("href", "")
						break
					}
				}
				break
			}
		}
	}

	if serverSlotNum != "" {
		serverAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
		serverAdaptersDoc, _ := c.fetchAndParseHMCXML(serverAdaptersURL, verbose)
		if serverAdaptersDoc != nil {
			for _, entry := range serverAdaptersDoc.FindElements("//entry") {
				if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil && slot.Text() == serverSlotNum {
					for _, link := range entry.FindElements("./link") {
						if link.SelectAttrValue("rel", "") == "SELF" {
							serverAdapterDeleteURL = link.SelectAttrValue("href", "")
							break
						}
					}
					break
				}
			}
		}
	}

	// =====================================================================
	// STEP 3: Delete Client Adapter (Removes LPAR-side connection)
	// =====================================================================
	if clientAdapterDeleteURL != "" {
		if verbose { hmcLogger.Printf("Executing DELETE on LPAR Client Adapter (Slot %s)...", clientSlotNum) }
		reqDelClient, _ := http.NewRequest("DELETE", clientAdapterDeleteURL, nil)
		reqDelClient.Header.Set("X-API-Session", c.session)
		
		respDelClient, err := c.client.Do(reqDelClient)
		if err != nil {
			return "", "", fmt.Errorf("HTTP request failed while deleting client adapter: %v", err)
		}
		
		clientBody, _ := io.ReadAll(respDelClient.Body)
		respDelClient.Body.Close()
		
		if verbose {
			hmcLogger.Printf("Client Adapter DELETE Status: %s", respDelClient.Status)
			if len(clientBody) > 0 {
				hmcLogger.Printf("Client Adapter DELETE Response:\n%s", string(clientBody))
			}
		}

		if respDelClient.StatusCode >= 400 {
			return "", "", fmt.Errorf("failed to delete client adapter. Status: %s, Response: %s", respDelClient.Status, string(clientBody))
		}
	}

	// Return the details so main.go can finish the sequence
	return vtdName, serverAdapterDeleteURL, nil
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

// RunVIOSCommand executes an OS-level command by tunneling it through the HMC CLIRunner job via viosvrcmd.
func (c *HmcRestClient) RunVIOSCommand(cmdString string, verbose bool) error {
	// 1. Fetch the Management Console UUID
	mcURL := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole", c.hmcIP)
	
	req, err := http.NewRequest("GET", mcURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")
	req.Header.Set("Accept", "*/*")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to fetch Management Console: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get Management Console (HTTP %d): %s", resp.StatusCode, string(body))
	}
	
	mcDoc, err := xmlStripNamespace(body)
	if err != nil {
		return fmt.Errorf("failed to parse Management Console XML: %v", err)
	}
	
	// Extract the Management Console UUID from the entry element
	// The feed has its own <id>, but we need the <entry><id> which is the actual MC UUID
	var mcUUID string
	if entryElem := mcDoc.FindElement("//entry/id"); entryElem != nil {
		mcUUID = entryElem.Text()
	} else if uuidElem := mcDoc.FindElement("//ManagementConsole/Metadata/Atom/AtomID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	} else if uuidElem := mcDoc.FindElement("//UUID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	}

	if mcUUID == "" {
		if verbose {
			hmcLogger.Printf("Management Console response:\n%s", string(body))
		}
		return fmt.Errorf("could not resolve Management Console UUID from response")
	}
	
	if verbose {
		hmcLogger.Printf("Resolved Management Console UUID: %s", mcUUID)
	}

	// 2. Target the Management Console's CLIRunner Job endpoint
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole/%s/do/CLIRunner", c.hmcIP, mcUUID)

	// 3. Construct the exact viosvrcmd string with proper quoting
	//cmdString := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s "`, sysName, viosName, diskName)

	if verbose {
		hmcLogger.Printf("Executing HMC CLI Command: %s", cmdString)
	}

	// 4. Create the CLIRunner Job Payload with required acknowledgeThisAPIMayGoAwayInTheFuture parameter
	// Using schemaVersion="V1_0" as confirmed working in Postman
	payload := fmt.Sprintf(`<JobRequest:JobRequest xmlns:JobRequest="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
	   <Metadata>
	       <Atom/>
	   </Metadata>
	   <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
	       <Metadata>
	           <Atom/>
	       </Metadata>
	       <OperationName kxe="false" kb="ROR">CLIRunner</OperationName>
	       <GroupName kxe="false" kb="ROR">ManagementConsole</GroupName>
	   </RequestedOperation>
	   <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
	       <Metadata>
	           <Atom/>
	       </Metadata>
	       <JobParameter schemaVersion="V1_0">
	           <Metadata>
	               <Atom/>
	           </Metadata>
	           <ParameterName kxe="false" kb="ROR">cmd</ParameterName>
	           <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
	       </JobParameter>
	       <JobParameter schemaVersion="V1_0">
	           <Metadata>
	               <Atom/>
	           </Metadata>
	           <ParameterName kxe="false" kb="ROR">acknowledgeThisAPIMayGoAwayInTheFuture</ParameterName>
	           <ParameterValue kxe="false" kb="CUR">true</ParameterValue>
	       </JobParameter>
	   </JobParameters>
</JobRequest:JobRequest>`, cmdString)

	if verbose {
		hmcLogger.Printf("CLIRunner payload:\n%s", payload)
		hmcLogger.Printf("CLIRunner URL: %s", url)
	}

	// 5. Submit the JobRequest via PUT (Jobs in HMC REST API use PUT)
	req2, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create CLIRunner request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req2.Header.Set("X-API-Session", c.session)
	req2.Header.Set("Accept", "application/atom+xml")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()

	resp2, err := c.client.Do(req2.WithContext(ctx2))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return fmt.Errorf("failed to read CLIRunner response: %v", err)
	}

	// Look for success status codes
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusAccepted && resp2.StatusCode != http.StatusCreated {
		return fmt.Errorf("CLIRunner failed with status %s: %s", resp2.Status, string(body2))
	}

	if verbose {
		hmcLogger.Printf("CLIRunner response (HTTP %d):\n%s", resp2.StatusCode, string(body2))
	}

	// Wait for the job to complete
	doc2, err := xmlStripNamespace(body2)
	if err != nil {
		return fmt.Errorf("failed to parse CLIRunner response XML: %v", err)
	}

	jobIDElem := doc2.FindElement("//JobID")
	if jobIDElem == nil {
		// Try alternative path
		jobIDElem = doc2.FindElement("//JobResponse/JobID")
	}
	
	if jobIDElem != nil {
		jobID := jobIDElem.Text()
		if verbose {
			hmcLogger.Printf("CLIRunner Job submitted (Job ID: %s), waiting for completion...", jobID)
		}
		_, err = c.FetchJobStatus(jobID, false, 10, verbose)
		if err != nil {
			return fmt.Errorf("CLIRunner job failed: %v", err)
		}
		if verbose {
			hmcLogger.Printf("✅ CLIRunner job completed successfully")
		}
	} else {
		return fmt.Errorf("JobID not found in CLIRunner response: %s", string(body2))
	}

	return nil
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

// GetViosSCSIMapping filters all VSCSI mappings on a VIOS to return only those associated with a specific LPAR UUID.
func (c *HmcRestClient) GetViosSCSIMapping(viosUUID, lparUUID string, verbose bool) ([]*etree.Element, error) {
	if verbose {
		hmcLogger.Printf("Filtering VSCSI mappings on VIOS %s for LPAR UUID %s", viosUUID, lparUUID)
	}

	// 1. Get all mappings for the VIOS using your existing SDK function
	allMappings, err := c.GetViosSCSIMappings(viosUUID, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all mappings: %v", err)
	}
	if verbose {
		hmcLogger.Printf("GetViosSCSIMappings response body:\n%v", allMappings)
	}
	var filteredMappings []*etree.Element
	targetLparLower := strings.ToLower(lparUUID)

	// 2. Iterate and filter based on AssociatedLogicalPartition href
	for _, mapping := range allMappings {
		assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assocLpar == nil {
			continue
		}

		href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
		// Check if the href (URL) ends with our LPAR UUID
		if strings.HasSuffix(href, targetLparLower) {
			filteredMappings = append(filteredMappings, mapping)
		}
	}

	if verbose {
		hmcLogger.Printf("Found %d mappings for LPAR %s on VIOS %s", len(filteredMappings), lparUUID, viosUUID)
	}

	return filteredMappings, nil
}